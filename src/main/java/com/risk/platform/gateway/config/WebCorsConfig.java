package com.risk.platform.gateway.config;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.servlet.config.annotation.CorsRegistry;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

/**
 * 前端联调用的 CORS 配置。
 *
 * <p>运营台（rcp-web）在本地是独立端口，与网关不同源；生产走同源 Ingress 时不需要放开。
 * 允许来源按配置注入，默认只允许本地开发地址——**不使用 "*"**，避免把网关变成任意站点可调的开放接口。
 */
@Configuration
public class WebCorsConfig implements WebMvcConfigurer {

    private final String[] allowedOrigins;

    public WebCorsConfig(@Value("${patrol.web.allowed-origins:http://localhost:5173,http://127.0.0.1:5173}")
                         String allowedOrigins) {
        this.allowedOrigins = allowedOrigins.split("\\s*,\\s*");
    }

    @Override
    public void addCorsMappings(CorsRegistry registry) {
        registry.addMapping("/api/**")
                .allowedOrigins(allowedOrigins)
                .allowedMethods("GET", "POST", "OPTIONS")
                .allowedHeaders("Content-Type")
                .maxAge(3600);
    }
}
