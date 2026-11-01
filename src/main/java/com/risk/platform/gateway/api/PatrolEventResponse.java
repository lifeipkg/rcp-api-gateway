package com.risk.platform.gateway.api;

import java.util.Map;

/**
 * 统一巡查响应：裁决 + 处置动作 + 基建处理信息。
 *
 * @param accepted          是否受理（幂等拒绝时为 false）
 * @param rejectReason      拒绝原因（旧版本重放等）
 * @param verdict           巡查裁决（降级时 reason=PATROL_SERVICE_DEGRADED）
 * @param disposition       分级处置动作
 * @param stateRollbackProtected 状态机是否拦截了一次状态回跳
 */
public record PatrolEventResponse(
        boolean accepted,
        String rejectReason,
        Map<String, Object> verdict,
        Map<String, Object> disposition,
        boolean stateRollbackProtected) {
}
