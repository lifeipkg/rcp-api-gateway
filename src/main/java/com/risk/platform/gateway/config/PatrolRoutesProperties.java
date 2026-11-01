package com.risk.platform.gateway.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

import java.util.HashMap;
import java.util.Map;

/**
 * 巡查线路由配置：domain -> 巡查线服务基础地址。
 *
 * 本地默认指向各服务端口；K8s 环境经环境变量覆盖为集群内 Service DNS。
 */
@ConfigurationProperties(prefix = "patrol.routes")
public class PatrolRoutesProperties {

    private Map<String, String> endpoints = new HashMap<>();

    public PatrolRoutesProperties() {
        endpoints.put("content", "http://localhost:8081");
        endpoints.put("message", "http://localhost:8082");
        endpoints.put("account", "http://localhost:8083");
        endpoints.put("fund", "http://localhost:8084");
    }

    public Map<String, String> getEndpoints() {
        return endpoints;
    }

    public void setEndpoints(Map<String, String> endpoints) {
        this.endpoints = endpoints;
    }
}
