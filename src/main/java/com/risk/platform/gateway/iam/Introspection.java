package com.risk.platform.gateway.iam;

import java.util.List;

/** 内省结果（网关侧视图）。只保留鉴权需要的字段，IAM 的其它返回一律忽略。 */
public record Introspection(boolean active, String userId, String username, String role, List<String> permissions) {
}
