package com.risk.platform.gateway.config;

import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Configuration;

/** 启用路由配置属性。 */
@Configuration
@EnableConfigurationProperties(PatrolRoutesProperties.class)
public class GatewayConfig {
}
