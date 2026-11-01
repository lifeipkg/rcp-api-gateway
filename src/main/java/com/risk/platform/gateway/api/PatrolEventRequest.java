package com.risk.platform.gateway.api;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;

import java.util.Map;

/**
 * 统一巡查事件（平台唯一接入契约）。
 *
 * @param eventId 事件 ID（幂等键，如 job:123 / msg:456 / acc:13800... / fund:789）
 * @param domain  巡查线：content / message / account / fund
 * @param version 事件版本（重放/更正场景单调递增）
 * @param payload 巡查线业务字段（转发给对应巡查线服务）
 */
public record PatrolEventRequest(
        @NotBlank String eventId,
        @NotBlank String domain,
        @NotNull Long version,
        Map<String, Object> payload) {
}
