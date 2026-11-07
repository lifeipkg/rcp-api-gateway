package com.risk.platform.gateway.iam;

import org.springframework.http.MediaType;
import org.springframework.http.client.SimpleClientHttpRequestFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.client.HttpClientErrorException;
import org.springframework.web.client.RestClient;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * IAM 内省客户端：POST {iam}/api/v1/iam/introspect。
 *
 * 网关不解析令牌，只信 IAM 的结论：令牌格式与权限展开都在 IAM 一处。
 * 超时预算 300ms connect / 1000ms read——比巡查链路宽松（登录不是热路径），
 * 但必须有上限，否则 IAM 卡住会拖垮整个网关线程池。
 */
@Component
public class IamClient {

    private final IamAddressResolver resolver;
    private final RestClient restClient;

    public IamClient(IamAddressResolver resolver) {
        this.resolver = resolver;
        SimpleClientHttpRequestFactory factory = new SimpleClientHttpRequestFactory();
        factory.setConnectTimeout((int) Duration.ofMillis(300).toMillis());
        factory.setReadTimeout((int) Duration.ofMillis(1000).toMillis());
        this.restClient = RestClient.builder().requestFactory(factory).build();
    }

    /**
     * @return 令牌有效时的内省结果；令牌无效/过期/已吊销返回 empty
     * @throws RuntimeException 内省端点不可达或返回 5xx（调用方须 fail closed）
     */
    @SuppressWarnings("unchecked")
    public Optional<Introspection> introspect(String token) {
        String endpoint = resolver.resolve();
        if (endpoint == null || endpoint.isBlank()) {
            throw new IllegalStateException("未配置 IAM 地址");
        }
        Map<String, Object> body;
        try {
            body = restClient.post()
                    .uri(endpoint + "/api/v1/iam/introspect")
                    .contentType(MediaType.APPLICATION_JSON)
                    .body(Map.of("token", token))
                    .retrieve()
                    .body(Map.class);
        } catch (HttpClientErrorException e) {
            // 400（请求体不合法）不可能是"令牌有效"，按无效处理
            return Optional.empty();
        }
        if (body == null || !Boolean.TRUE.equals(body.get("active"))) {
            return Optional.empty();
        }
        List<String> permissions = body.get("permissions") instanceof List<?> list
                ? list.stream().map(String::valueOf).toList()
                : List.of();
        return Optional.of(new Introspection(true,
                stringValue(body.get("userId")),
                stringValue(body.get("username")),
                stringValue(body.get("role")),
                permissions));
    }

    private static String stringValue(Object value) {
        return value == null ? null : String.valueOf(value);
    }
}
