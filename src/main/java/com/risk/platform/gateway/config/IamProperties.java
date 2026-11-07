package com.risk.platform.gateway.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

import java.util.ArrayList;
import java.util.List;

/** 网关的 IAM 接线配置：鉴权开关、公开路径、IAM 寻址。 */
@ConfigurationProperties(prefix = "iam")
public class IamProperties {

    private final Auth auth = new Auth();
    private final Routes routes = new Routes();

    public Auth getAuth() {
        return auth;
    }

    public Routes getRoutes() {
        return routes;
    }

    public static class Auth {

        /** 关闭后网关不做鉴权（仅供本地联调与既有单测；生产必须为 true）。 */
        private boolean enabled = true;

        /** 不需要令牌的路径（Ant 风格）。IAM 透传路径在过滤器里单独跳过。 */
        private final List<String> publicPaths = new ArrayList<>(
                List.of("/actuator/**", "/api/v1/patrol/domains"));

        public boolean isEnabled() {
            return enabled;
        }

        public void setEnabled(boolean enabled) {
            this.enabled = enabled;
        }

        public List<String> getPublicPaths() {
            return publicPaths;
        }
    }

    public static class Routes {

        /** 服务发现注册名（真源：abox/services.ttl 的 rcp:iam 个体）。 */
        private String service = "iam";

        /** 兜底地址：注册中心不可用或无实例时使用（本地开发无 compose 时即走这里）。 */
        private String endpoint = "http://localhost:8085";

        public String getService() {
            return service;
        }

        public void setService(String service) {
            this.service = service;
        }

        public String getEndpoint() {
            return endpoint;
        }

        public void setEndpoint(String endpoint) {
            this.endpoint = endpoint;
        }

    }
}
