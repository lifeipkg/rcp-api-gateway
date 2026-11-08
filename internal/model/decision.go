package model

// Decision 是平台统一风控决策。语义真源在
// rcp-ontology/ontology/tbox/decisions.ttl；本包只做网关侧的序与处置映射，
// 不在这里增加新决策（新增决策要先改 TTL 与契约）。
type Decision string

const (
	DecisionPass        Decision = "PASS"
	DecisionWarn        Decision = "WARN"
	DecisionLimit       Decision = "LIMIT"
	DecisionExtraReview Decision = "EXTRA_REVIEW"
	DecisionIntercept   Decision = "INTERCEPT"
	DecisionBan         Decision = "BAN"
)

// decisionRank 是状态机单调前进的序：只允许变大，回跳一律拦截。
var decisionRank = map[Decision]int{
	DecisionPass:        0,
	DecisionWarn:        1,
	DecisionLimit:       2,
	DecisionExtraReview: 3,
	DecisionIntercept:   4,
	DecisionBan:         5,
}

// Domains 是平台的四条巡查线；dispatch 的 domain 只接受这四个值。
var Domains = []string{"content", "message", "account", "fund"}

// Rank 返回决策在状态机中的序；未知决策返回 -1（上游给出未声明的决策即视为异常）。
func (d Decision) Rank() int {
	if rank, ok := decisionRank[d]; ok {
		return rank
	}
	return -1
}

// Valid 判断是否为已声明决策。上游返回未声明值时网关不猜，按巡查线异常降级。
func (d Decision) Valid() bool {
	return d.Rank() >= 0
}

// Disposition 是分级处置动作：同一决策对所有巡查线落地同一语义。
type Disposition struct {
	Action string `json:"action"`
	Note   string `json:"note,omitempty"`
}

// Dispositions 是决策 → 处置动作的唯一映射表。
var Dispositions = map[Decision]Disposition{
	DecisionPass:        {Action: "RELEASE"},
	DecisionWarn:        {Action: "NOTIFY", Note: "仅提示不拦截"},
	DecisionLimit:       {Action: "THROTTLE", Note: "当日触达/申请次数受限"},
	DecisionExtraReview: {Action: "MANUAL_REVIEW", Note: "进入人工复核队列"},
	DecisionIntercept:   {Action: "BLOCK", Note: "拦截并进入复核队列"},
	DecisionBan:         {Action: "BAN_ACCOUNT", Note: "账号封禁，可申诉"},
}
