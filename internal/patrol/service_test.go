package patrol

import (
	"context"
	"errors"
	"testing"

	"github.com/lifeipkg/rcp-api-gateway/internal/model"
)

type stubClient struct {
	verdict    model.Verdict
	err        error
	gotDomain  string
	gotPayload map[string]any
	calls      int
}

func (s *stubClient) Check(_ context.Context, domain string, payload map[string]any) (model.Verdict, error) {
	s.calls++
	s.gotDomain = domain
	s.gotPayload = payload
	if s.err != nil {
		return model.Verdict{}, s.err
	}
	return s.verdict, nil
}

func request(eventID string, version int64, domain string) model.EventRequest {
	return model.EventRequest{EventID: eventID, Domain: domain, Version: &version}
}

func verdictFor(eventID string, decision model.Decision) model.Verdict {
	return model.Verdict{EventID: eventID, Decision: decision, Score: 10, Reason: "rule:test"}
}

func TestStaleVersionIsRejected(t *testing.T) {
	client := &stubClient{verdict: verdictFor("job:1", model.DecisionPass)}
	service := NewService(client)

	if got := service.Dispatch(context.Background(), request("job:1", 3, "content")); !got.Accepted {
		t.Fatalf("首个版本应受理，实际 accepted=false")
	}
	// 反向断言：同键同版本必须被拒，否则重放攻击成立。
	stale := service.Dispatch(context.Background(), request("job:1", 3, "content"))
	if stale.Accepted || stale.RejectReason == nil || *stale.RejectReason != model.RejectStaleVersion {
		t.Fatalf("同键同版本应被幂等拒绝，实际 %+v", stale)
	}
	// 旧版本同样拒绝。
	older := service.Dispatch(context.Background(), request("job:1", 2, "content"))
	if older.Accepted {
		t.Fatalf("旧版本应被拒绝，实际 %+v", older)
	}
	// 更高版本必须受理。
	if newer := service.Dispatch(context.Background(), request("job:1", 4, "content")); !newer.Accepted {
		t.Fatalf("更高版本应受理，实际 %+v", newer)
	}
}

func TestStaleReplayDoesNotCallPatrol(t *testing.T) {
	client := &stubClient{verdict: verdictFor("job:2", model.DecisionPass)}
	service := NewService(client)
	service.Dispatch(context.Background(), request("job:2", 1, "content"))
	service.Dispatch(context.Background(), request("job:2", 1, "content"))
	if client.calls != 1 {
		t.Fatalf("幂等拒绝不应触发巡查，实际调用 %d 次", client.calls)
	}
}

func TestPatrolFailureDegradesToExtraReview(t *testing.T) {
	client := &stubClient{err: errors.New("connection refused")}
	service := NewService(client)

	got := service.Dispatch(context.Background(), request("job:3", 1, "fund"))
	if !got.Accepted || got.Verdict == nil {
		t.Fatalf("降级也应受理并给出裁决，实际 %+v", got)
	}
	if got.Verdict.Decision != model.DecisionExtraReview {
		t.Fatalf("巡查线不可用必须转加审，实际 %s", got.Verdict.Decision)
	}
	if got.Verdict.Reason != model.ReasonPatrolDegraded || got.Verdict.Detail["domain"] != "fund" {
		t.Fatalf("降级裁决缺少原因/域信息，实际 %+v", got.Verdict)
	}
	if got.Disposition == nil || got.Disposition.Action != "MANUAL_REVIEW" {
		t.Fatalf("降级处置应为人工复核，实际 %+v", got.Disposition)
	}
}

func TestStateRollbackIsProtected(t *testing.T) {
	client := &stubClient{verdict: verdictFor("job:4", model.DecisionIntercept)}
	service := NewService(client)

	first := service.Dispatch(context.Background(), request("job:4", 1, "message"))
	if first.StateRollbackProtected {
		t.Fatalf("首次落地不应触发回跳保护")
	}
	if first.Verdict.Decision != model.DecisionIntercept {
		t.Fatalf("首次裁决应为 INTERCEPT，实际 %s", first.Verdict.Decision)
	}

	// 反向断言：版本更高的低分裁决不得把状态拉回 PASS。
	client.verdict = verdictFor("job:4", model.DecisionPass)
	second := service.Dispatch(context.Background(), request("job:4", 2, "message"))
	if !second.StateRollbackProtected {
		t.Fatalf("状态回跳必须被拦截并置 stateRollbackProtected=true")
	}
	if second.Verdict.Decision != model.DecisionPass {
		t.Fatalf("拦截回跳不改变本次裁决，实际 %s", second.Verdict.Decision)
	}
	if second.Disposition.Action != "RELEASE" {
		t.Fatalf("处置按本次裁决映射，实际 %+v", second.Disposition)
	}

	// 状态未被拉低：再给一次低分也不应触发回跳。
	third := service.Dispatch(context.Background(), request("job:4", 3, "message"))
	if !third.StateRollbackProtected {
		t.Fatalf("状态应仍停在 INTERCEPT，低分裁决继续被拦截")
	}
}

func TestPayloadCarriesEventID(t *testing.T) {
	client := &stubClient{verdict: verdictFor("job:5", model.DecisionPass)}
	service := NewService(client)
	service.Dispatch(context.Background(), request("job:5", 1, "account"))

	if client.gotDomain != "account" {
		t.Fatalf("domain 未透传，实际 %q", client.gotDomain)
	}
	if client.gotPayload["eventId"] != "job:5" {
		t.Fatalf("payload 应补 eventId，实际 %v", client.gotPayload)
	}
}

func TestDispositionCoversEveryDecision(t *testing.T) {
	want := map[model.Decision]string{
		model.DecisionPass:        "RELEASE",
		model.DecisionWarn:        "NOTIFY",
		model.DecisionLimit:       "THROTTLE",
		model.DecisionExtraReview: "MANUAL_REVIEW",
		model.DecisionIntercept:   "BLOCK",
		model.DecisionBan:         "BAN_ACCOUNT",
	}
	if len(model.Dispositions) != len(want) {
		t.Fatalf("处置映射条数与决策数不一致：%d vs %d", len(model.Dispositions), len(want))
	}
	for decision, action := range want {
		got, ok := model.Dispositions[decision]
		if !ok {
			t.Fatalf("决策 %s 缺处置映射", decision)
		}
		if got.Action != action {
			t.Fatalf("决策 %s 处置应为 %s，实际 %s", decision, action, got.Action)
		}
	}
}
