package com.risk.platform.gateway.config;

import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.HttpStatusCode;
import org.springframework.http.client.SimpleClientHttpRequestFactory;
import org.springframework.web.client.DefaultResponseErrorHandler;
import org.springframework.web.client.RestTemplate;

/** 启用路由与 IAM 配置属性；IAM 透传用的 RestTemplate 也在此声明，便于测试注入。 */
@Configuration
@EnableConfigurationProperties({PatrolRoutesProperties.class, IamProperties.class})
public class GatewayConfig {

    /**
     * IAM 透传专用 RestTemplate。
     *
     * 关键点是错误处理器不抛异常：IAM 的 401/403/409 是正常业务结果，必须原样
     * 返回给调用方；默认实现会把 4xx 变成异常，前端就永远看不到"口令错误"。
     */
    @Bean
    public RestTemplate iamProxyRestTemplate() {
        SimpleClientHttpRequestFactory factory = new SimpleClientHttpRequestFactory();
        factory.setConnectTimeout(300);
        factory.setReadTimeout(2000);
        RestTemplate restTemplate = new RestTemplate(factory);
        restTemplate.setErrorHandler(new DefaultResponseErrorHandler() {
            @Override
            public boolean hasError(HttpStatusCode statusCode) {
                return false;
            }
        });
        return restTemplate;
    }
}
