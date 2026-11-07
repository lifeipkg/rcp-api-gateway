package com.risk.platform.gateway.auth;

import com.risk.platform.gateway.dispatch.PatrolClient;
import com.risk.platform.gateway.iam.IamClient;
import com.risk.platform.gateway.iam.Introspection;
import com.risk.platform.model.Decision;
import com.risk.platform.model.PatrolVerdict;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.boot.test.web.client.TestRestTemplate;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.http.HttpEntity;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpMethod;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;

import java.util.List;
import java.util.Map;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.when;

/**
 * 网关鉴权链路的正反用例：缺令牌、坏令牌、越权、鉴权服务不可用都必须被拒。
 * IAM 与巡查线都是桩，断言只针对"网关自己的判定"，不测上游实现。
 */
@SpringBootTest(
        webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT,
        properties = "spring.cloud.nacos.discovery.enabled=false")
class AuthFilterTest {

    private static final String DISPATCH = "/api/v1/patrol/dispatch";
    private static final String TOKEN = "rcp_test-token";

    @LocalServerPort
    int port;

    @Autowired
    TestRestTemplate rest;

    @MockBean
    PatrolClient patrolClient;

    @MockBean
    IamClient iamClient;

    @Test
    @DisplayName("缺令牌：401 MISSING_TOKEN（反向用例）")
    void missingTokenRejected() {
        ResponseEntity<Map> response = dispatch(null);

        assertEquals(HttpStatus.UNAUTHORIZED, response.getStatusCode());
        assertEquals("MISSING_TOKEN", response.getBody().get("error"));
    }

    @Test
    @DisplayName("令牌无效：401 INVALID_TOKEN（反向用例）")
    void invalidTokenRejected() {
        when(iamClient.introspect(TOKEN)).thenReturn(Optional.empty());

        ResponseEntity<Map> response = dispatch(TOKEN);

        assertEquals(HttpStatus.UNAUTHORIZED, response.getStatusCode());
        assertEquals("INVALID_TOKEN", response.getBody().get("error"));
    }

    @Test
    @DisplayName("已登录但无 patrol:dispatch：403（反向用例：VIEWER 不能提交）")
    void viewerForbidden() {
        when(iamClient.introspect(TOKEN)).thenReturn(Optional.of(
                new Introspection(true, "usr_v", "viewer", "VIEWER", List.of())));

        ResponseEntity<Map> response = dispatch(TOKEN);

        assertEquals(HttpStatus.FORBIDDEN, response.getStatusCode());
        assertEquals("FORBIDDEN", response.getBody().get("error"));
    }

    @Test
    @DisplayName("鉴权服务不可用：503，不放行（反向用例：fail closed）")
    void iamUnavailableFailsClosed() {
        when(iamClient.introspect(TOKEN)).thenThrow(new IllegalStateException("iam down"));

        ResponseEntity<Map> response = dispatch(TOKEN);

        assertEquals(HttpStatus.SERVICE_UNAVAILABLE, response.getStatusCode());
        assertEquals("AUTH_UNAVAILABLE", response.getBody().get("error"));
    }

    @Test
    @DisplayName("OPERATOR 带权限提交：放行到巡查线")
    void operatorAllowed() {
        when(iamClient.introspect(TOKEN)).thenReturn(Optional.of(
                new Introspection(true, "usr_o", "operator", "OPERATOR", List.of("patrol:dispatch"))));
        when(patrolClient.check(anyString(), any())).thenReturn(
                new PatrolVerdict("e1", Decision.PASS, 0, "OK", Map.of("domain", "content")));

        ResponseEntity<Map> response = dispatch(TOKEN);

        assertEquals(HttpStatus.OK, response.getStatusCode());
        assertEquals(true, response.getBody().get("accepted"));
    }

    @Test
    @DisplayName("平台自描述是公开路径：无令牌可访问")
    void publicPathNeedsNoToken() {
        ResponseEntity<Map> response = rest.getForEntity(
                "http://localhost:" + port + "/api/v1/patrol/domains", Map.class);

        assertEquals(HttpStatus.OK, response.getStatusCode());
        assertEquals("risk-control-platform", response.getBody().get("platform"));
    }

    private ResponseEntity<Map> dispatch(String token) {
        HttpHeaders headers = new HttpHeaders();
        headers.setContentType(MediaType.APPLICATION_JSON);
        if (token != null) {
            headers.setBearerAuth(token);
        }
        Map<String, Object> body = Map.of(
                "eventId", "e1",
                "domain", "content",
                "version", 1,
                "payload", Map.of("jobId", 1));
        return rest.exchange("http://localhost:" + port + DISPATCH, HttpMethod.POST,
                new HttpEntity<>(body, headers), Map.class);
    }
}
