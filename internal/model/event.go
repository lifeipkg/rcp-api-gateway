package model

// EventRequest 是统一巡查事件（平台唯一接入契约，见
// rcp-contracts/contracts/schemas/patrol-event.yaml）。
//
// Version 用指针接收：缺字段与 version=0 是两回事，前者是请求体不合法，
// 后者违反契约的 minimum: 1。
type EventRequest struct {
	EventID string         `json:"eventId"`
	Domain  string         `json:"domain"`
	Version *int64         `json:"version"`
	Payload map[string]any `json:"payload"`
}

// Verdict 是巡查线返回的统一裁决。
type Verdict struct {
	EventID  string         `json:"eventId"`
	Decision Decision       `json:"decision"`
	Score    int            `json:"score"`
	Reason   string         `json:"reason"`
	Detail   map[string]any `json:"detail"`
}

// Normalize 把 nil detail 归一成空对象：契约里 detail 是 object，不能是 null。
func (v Verdict) Normalize() Verdict {
	if v.Detail == nil {
		v.Detail = map[string]any{}
	}
	return v
}

// EventResponse 是统一巡查响应：裁决 + 处置动作 + 基建处理信息。
//
// 字段全部显式输出（含 null）：幂等拒绝时 verdict/disposition 就是 null，
// 调用方据此区分"拒绝受理"与"受理但裁决为空"。
type EventResponse struct {
	Accepted               bool         `json:"accepted"`
	RejectReason           *string      `json:"rejectReason"`
	Verdict                *Verdict     `json:"verdict"`
	Disposition            *Disposition `json:"disposition"`
	StateRollbackProtected bool         `json:"stateRollbackProtected"`
}

// RejectStaleVersion 是幂等拒绝码：同键旧版本重放。
const RejectStaleVersion = "STALE_VERSION_REPLAY"

// ReasonPatrolDegraded 是降级裁决的原因标识：巡查线不可用时转人工加审。
const ReasonPatrolDegraded = "PATROL_SERVICE_DEGRADED"

// DegradedScore 降级裁决的风险分：与 EXTRA_REVIEW 基准分一致。
const DegradedScore = 60
