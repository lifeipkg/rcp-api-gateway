package com.risk.platform.gateway.dispatch;

import com.risk.platform.gateway.config.PatrolRoutesProperties;
import com.risk.platform.model.Decision;
import com.risk.platform.model.PatrolVerdict;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.cloud.client.ServiceInstance;
import org.springframework.cloud.client.discovery.DiscoveryClient;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class HttpPatrolClientTest {

    /** 发现中心的可编程桩。 */
    static class FakeDiscoveryClient implements DiscoveryClient {
        private final Map<String, List<ServiceInstance>> byService;
        private final RuntimeException failOnQuery;

        FakeDiscoveryClient(Map<String, List<ServiceInstance>> byService, RuntimeException failOnQuery) {
            this.byService = byService;
            this.failOnQuery = failOnQuery;
        }

        @Override
        public String description() {
            return "fake";
        }

        @Override
        public List<ServiceInstance> getInstances(String serviceId) {
            if (failOnQuery != null) {
                throw failOnQuery;
            }
            return byService.getOrDefault(serviceId, List.of());
        }

        @Override
        public List<String> getServices() {
            return List.copyOf(byService.keySet());
        }
    }

    /** ObjectProvider 桩：Spring 6 的 ObjectProvider 有多个抽象方法，需全量实现。 */
    static class StubProvider implements ObjectProvider<DiscoveryClient> {
        private final DiscoveryClient dc;
        private final boolean present;

        StubProvider(DiscoveryClient dc, boolean present) {
            this.dc = dc;
            this.present = present;
        }

        @Override
        public DiscoveryClient getObject() {
            if (!present) {
                throw new IllegalStateException("no DiscoveryClient bean");
            }
            return dc;
        }

        @Override
        public DiscoveryClient getObject(Object... args) {
            return getObject();
        }

        @Override
        public DiscoveryClient getIfAvailable() {
            return present ? dc : null;
        }

        @Override
        public DiscoveryClient getIfUnique() {
            return present ? dc : null;
        }
    }

    static ObjectProvider<DiscoveryClient> provider(DiscoveryClient dc) {
        return new StubProvider(dc, true);
    }

    static ObjectProvider<DiscoveryClient> absent() {
        return new StubProvider(null, false);
    }

    HttpServer server;
    AtomicInteger hits = new AtomicInteger();

    @BeforeEach
    void startStub() throws IOException {
        server = HttpServer.create(new InetSocketAddress("localhost", 0), 0);
        server.createContext("/api/v1/patrol/check", exchange -> {
            hits.incrementAndGet();
            byte[] body = ("{\"eventId\":\"e1\",\"decision\":\"PASS\",\"score\":0,"
                    + "\"reason\":\"OK\",\"detail\":{}}").getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().add("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, body.length);
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(body);
            }
        });
        server.start();
    }

    @AfterEach
    void stopStub() {
        server.stop(0);
    }

    private int stubPort() {
        return server.getAddress().getPort();
    }

    @Test
    @DisplayName("寻址：注册中心有实例 -> 按发现结果路由，不经配置地址")
    void discoveryWins() {
        ServiceInstance inst = new org.springframework.cloud.client.DefaultServiceInstance(
                "i1", "content-patrol", "localhost", stubPort(), false);
        FakeDiscoveryClient dc = new FakeDiscoveryClient(Map.of("content-patrol", List.of(inst)), null);
        HttpPatrolClient client = new HttpPatrolClient(new PatrolRoutesProperties(), provider(dc));

        PatrolVerdict v = client.check("content", Map.of("eventId", "e1"));
        assertEquals(Decision.PASS, v.decision());
        assertEquals(1, hits.get());
    }

    @Test
    @DisplayName("寻址：多实例 -> 轮转分摊，不固定打一个实例")
    void roundRobinAcrossInstances() {
        ServiceInstance i1 = new org.springframework.cloud.client.DefaultServiceInstance(
                "i1", "content-patrol", "localhost", stubPort(), false);
        ServiceInstance i2 = new org.springframework.cloud.client.DefaultServiceInstance(
                "i2", "content-patrol", "127.0.0.1", stubPort(), false);
        FakeDiscoveryClient dc = new FakeDiscoveryClient(Map.of("content-patrol", List.of(i1, i2)), null);
        HttpPatrolClient client = new HttpPatrolClient(new PatrolRoutesProperties(), provider(dc));

        java.util.Set<String> seen = new java.util.HashSet<>();
        for (int i = 0; i < 4; i++) {
            seen.add(client.resolve("content"));
        }
        // 两个实例都被轮转到（localhost 与 127.0.0.1 两个地址）
        assertEquals(2, seen.size());
    }

    @Test
    @DisplayName("寻址：无 DiscoveryClient -> 回落配置地址（本地默认/环境变量覆盖项）")
    void fallsBackWithoutDiscovery() {
        HttpPatrolClient client = new HttpPatrolClient(new PatrolRoutesProperties(), absent());
        assertEquals("http://localhost:8081", client.resolve("content"));

        // check 走兜底地址打到桩服务（把 endpoints 指向桩）
        PatrolRoutesProperties routes = new PatrolRoutesProperties();
        routes.getEndpoints().put("content", "http://localhost:" + stubPort());
        HttpPatrolClient wired = new HttpPatrolClient(routes, absent());
        assertEquals(Decision.PASS, wired.check("content", Map.of()).decision());
    }

    @Test
    @DisplayName("降级：注册中心查询异常 -> 回落配置地址，不抛错")
    void fallsBackWhenRegistryDown() {
        FakeDiscoveryClient dc = new FakeDiscoveryClient(Map.of(), new IllegalStateException("nacos down"));
        PatrolRoutesProperties routes = new PatrolRoutesProperties();
        routes.getEndpoints().put("content", "http://localhost:" + stubPort());
        HttpPatrolClient client = new HttpPatrolClient(routes, provider(dc));

        assertEquals(Decision.PASS, client.check("content", Map.of()).decision());
    }

    @Test
    @DisplayName("未知巡查线：既无注册名也无兜底地址 -> 拒绝")
    void unknownDomainRejected() {
        HttpPatrolClient client = new HttpPatrolClient(new PatrolRoutesProperties(), absent());
        assertThrows(IllegalArgumentException.class, () -> client.check("unknown", Map.of()));
        assertTrue(hits.get() == 0);
    }
}
