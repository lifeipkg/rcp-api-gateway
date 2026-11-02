package com.risk.platform.gateway.dispatch;

import com.risk.platform.gateway.config.PatrolRoutesProperties;
import com.risk.platform.model.PatrolVerdict;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.cloud.client.ServiceInstance;
import org.springframework.cloud.client.discovery.DiscoveryClient;
import org.springframework.http.client.SimpleClientHttpRequestFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * HTTP 形态的巡查线客户端：POST {endpoint}/api/v1/patrol/check。
 *
 * 寻址：优先经服务发现按注册名解析（patrol.routes.services，Nacos），
 * 多实例轮转分摊；注册中心不可用或无实例时回落 patrol.routes.endpoints
 * （本地默认值 / 过渡期环境变量覆盖项），与平台"宁可降级、不可漏审"的降级语义衔接。
 *
 * 超时约束：同步巡查链路预算 200ms，connect 100ms / read 500ms（兜底留量）。
 */
@Component
public class HttpPatrolClient implements PatrolClient {

    private final PatrolRoutesProperties routes;
    private final ObjectProvider<DiscoveryClient> discovery;
    private final RestClient restClient;
    private final AtomicInteger cursor = new AtomicInteger();

    public HttpPatrolClient(PatrolRoutesProperties routes,
                            ObjectProvider<DiscoveryClient> discovery) {
        this.routes = routes;
        this.discovery = discovery;
        SimpleClientHttpRequestFactory factory = new SimpleClientHttpRequestFactory();
        factory.setConnectTimeout((int) Duration.ofMillis(100).toMillis());
        factory.setReadTimeout((int) Duration.ofMillis(500).toMillis());
        this.restClient = RestClient.builder().requestFactory(factory).build();
    }

    @Override
    @SuppressWarnings("unchecked")
    public PatrolVerdict check(String domain, Object payload) {
        String endpoint = resolve(domain);
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

    /**
     * 解析 domain 对应的上游基础地址：服务发现优先，配置地址兜底。
     * 返回 null 表示既无发现结果也无配置（未知巡查线）。
     */
    String resolve(String domain) {
        String service = routes.getServices().get(domain);
        if (service != null) {
            DiscoveryClient dc = discovery.getIfAvailable();
            if (dc != null) {
                try {
                    List<ServiceInstance> instances = dc.getInstances(service);
                    if (!instances.isEmpty()) {
                        ServiceInstance inst =
                                instances.get(Math.floorMod(cursor.getAndIncrement(), instances.size()));
                        return "http://" + inst.getHost() + ":" + inst.getPort();
                    }
                } catch (Exception e) {
                    // 注册中心不可达或查询失败：回落配置地址，由调用方按降级语义处理
                }
            }
        }
        return routes.getEndpoints().get(domain);
    }
}
