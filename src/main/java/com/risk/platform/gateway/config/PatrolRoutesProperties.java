package com.risk.platform.gateway.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

import java.util.HashMap;
import java.util.Map;

/**
 * 巡查线路由配置：domain -> 巡查线服务。
 *
 * 寻址优先级（见 rcp-contracts/docs/DISCOVERY.md 第 4 步）：
 *   1. services    domain -> Nacos 注册名，经服务发现解析实例地址（主路径）；
 *   2. endpoints   domain -> 基础地址，仅在注册中心不可用或无实例时兜底
 *                  （本地默认 localhost:<port>；过渡期部署侧可经
 *                  PATROL_ROUTES_ENDPOINTS_* 覆盖，discovery 开启后部署侧不再注入）。
 */
@ConfigurationProperties(prefix = "patrol.routes")
public class PatrolRoutesProperties {

    private Map<String, String> endpoints = new HashMap<>();

    /** domain -> 服务发现注册名。真源：rcp-contracts/registry/services.yaml 的 services[].name。 */
    private Map<String, String> services = new HashMap<>();

    public PatrolRoutesProperties() {
        endpoints.put("content", "http://localhost:8081");
        endpoints.put("message", "http://localhost:8082");
        endpoints.put("account", "http://localhost:8083");
        endpoints.put("fund", "http://localhost:8084");
        services.put("content", "content-patrol");
        services.put("message", "message-patrol");
        services.put("account", "account-patrol");
        services.put("fund", "fund-patrol");
    }

    public Map<String, String> getEndpoints() {
        return endpoints;
    }

    public void setEndpoints(Map<String, String> endpoints) {
        this.endpoints = endpoints;
    }

    public Map<String, String> getServices() {
        return services;
    }

    public void setServices(Map<String, String> services) {
        this.services = services;
    }
}
