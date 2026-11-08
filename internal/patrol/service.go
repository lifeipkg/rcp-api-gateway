package patrol

import (
	"context"
	"sync"

	"github.com/lifeipkg/rcp-api-gateway/internal/model"
)

// Service 是平台巡查编排：
//
//  1. 消费幂等：event_id + version 作为唯一键，同键旧版本重放直接拒绝；
//  2. 状态机单调前进：决策秩只升不降，回跳被拦截并置 stateRollbackProtected；
//  3. 分级处置：同一决策落同一语义的动作；
//  4. 巡查线降级：不可用时转人工加审（宁可人审，不可漏审）。
//
// 幂等与状态机当前是进程内表（与 Java 版等价）：重启即失效、无 TTL。
// 生产形态应落在 PostgreSQL（经 dataway），届时键与版本语义不变。
type Service struct {
	client Client

	mu       sync.Mutex
	versions map[string]int64
	ranks    map[string]int
}

func NewService(client Client) *Service {
	return &Service{
		client:   client,
		versions: make(map[string]int64),
		ranks:    make(map[string]int),
	}
}

// Dispatch 受理一个事件：先幂等判定，再巡查，最后状态机与处置。
func (s *Service) Dispatch(ctx context.Context, req model.EventRequest) model.EventResponse {
	version := *req.Version
	if stale := s.acceptVersion(req.EventID, version); stale {
		reason := model.RejectStaleVersion
		return model.EventResponse{Accepted: false, RejectReason: &reason}
	}

	verdict := s.judge(ctx, req)
	disposition := model.Dispositions[verdict.Decision]

	return model.EventResponse{
		Accepted:               true,
		Verdict:                &verdict,
		Disposition:            &disposition,
		StateRollbackProtected: s.advance(req.EventID, verdict.Decision.Rank()),
	}
}

// acceptVersion 返回 true 表示该版本是重放，应拒绝受理。
//
// 一次加锁内完成"读旧值 + 比较 + 写入"，避免并发同键请求各读各写导致的假放行。
func (s *Service) acceptVersion(eventID string, version int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen, ok := s.versions[eventID]
	if ok && version <= seen {
		return true
	}
	s.versions[eventID] = version
	return false
}

// judge 调巡查线；任何异常（不可达、非 200、响应不合法、未声明决策）都降级为加审。
func (s *Service) judge(ctx context.Context, req model.EventRequest) model.Verdict {
	payload := make(map[string]any, len(req.Payload)+1)
	for key, value := range req.Payload {
		payload[key] = value
	}
	if _, exists := payload["eventId"]; !exists {
		payload["eventId"] = req.EventID
	}

	verdict, err := s.client.Check(ctx, req.Domain, payload)
	if err != nil {
		return model.Verdict{
			EventID:  req.EventID,
			Decision: model.DecisionExtraReview,
			Score:    model.DegradedScore,
			Reason:   model.ReasonPatrolDegraded,
			Detail:   map[string]any{"domain": req.Domain},
		}
	}
	return verdict
}

// advance 只前进不回退；返回是否拦截了一次状态回跳。
func (s *Service) advance(eventID string, rank int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, ok := s.ranks[eventID]
	if ok && rank < previous {
		return true
	}
	s.ranks[eventID] = rank
	return false
}
