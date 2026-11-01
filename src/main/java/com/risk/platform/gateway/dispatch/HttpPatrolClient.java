package com.risk.platform.gateway.dispatch;

import com.risk.platform.gateway.config.PatrolRoutesProperties;
import com.risk.platform.model.PatrolVerdict;
import org.springframework.http.client.SimpleClientHttpRequestFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;

import java.time.Duration;
import java.util.Map;

/**
 * HTTP 形态的巡查线客户端：POST {endpoint}/api/v1/patrol/check。
 *
 * 超时约束：同步巡查链路预算 200ms，connect 100ms / read 500ms（兜底留量）。
 */
@Component
public class HttpPatrolClient implements PatrolClient {

    private final PatrolRoutesProperties routes;
    private final RestClient restClient;

    public HttpPatrolClient(PatrolRoutesProperties routes) {
        this.routes = routes;
        SimpleClientHttpRequestFactory factory = new SimpleClientHttpRequestFactory();
        factory.setConnectTimeout((int) Duration.ofMillis(100).toMillis());
        factory.setReadTimeout((int) Duration.ofMillis(500).toMillis());
        this.restClient = RestClient.builder().requestFactory(factory).build();
    }

    @Override
    @SuppressWarnings("unchecked")
    public PatrolVerdict check(String domain, Object payload) {
        String endpoint = routes.getEndpoints().get(domain);
        if (endpoint == null) {
            throw new IllegalArgumentException("未知巡查线: " + domain);
        }
        Map<String, Object> body = restClient.post()
                .uri(endpoint + "/api/v1/patrol/check")
                .header("Content-Type", "application/json")
                .body(payload)
                .retrieve()
                .body(Map.class);
        return new PatrolVerdict(
                String.valueOf(body.get("eventId")),
                com.risk.platform.model.Decision.valueOf(String.valueOf(body.get("decision"))),
                body.get("score") == null ? 0 : ((Number) body.get("score")).intValue(),
                String.valueOf(body.get("reason")),
                (Map<String, Object>) body.get("detail"));
    }
}
