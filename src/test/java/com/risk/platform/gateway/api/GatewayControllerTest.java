package com.risk.platform.gateway.api;

import com.risk.platform.gateway.dispatch.PatrolClient;
import com.risk.platform.model.Decision;
import com.risk.platform.model.PatrolVerdict;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.boot.test.web.client.TestRestTemplate;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.when;

@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class GatewayControllerTest {

    @LocalServerPort
    int port;

    @Autowired
    TestRestTemplate rest;

    @MockBean
    PatrolClient patrolClient;

    private String url() {
        return "http://localhost:" + port + "/api/v1/patrol/dispatch";
    }

    @Test
    @DisplayName("统一入口：路由巡查线并返回裁决 + 处置")
    void dispatchEndToEnd() throws Exception {
        when(patrolClient.check(anyString(), any()))
                .thenReturn(new PatrolVerdict("msg:9", Decision.INTERCEPT, 80,
                        "KEYWORD_HIT_RECRUITER", Map.of("domain", "message")));

        PatrolEventRequest req = new PatrolEventRequest("msg:9", "message", 1L,
                Map.of("messageId", 9, "senderRole", "RECRUITER", "content", "加我微信"));
        ResponseEntity<Map> r = rest.postForEntity(url(), req, Map.class);

        assertEquals(HttpStatus.OK, r.getStatusCode());
        assertEquals(true, r.getBody().get("accepted"));
        Map<String, Object> verdict = (Map<String, Object>) r.getBody().get("verdict");
        assertEquals("INTERCEPT", verdict.get("decision"));
        Map<String, Object> disposition = (Map<String, Object>) r.getBody().get("disposition");
        assertEquals("BLOCK", disposition.get("action"));
    }

    @Test
    @DisplayName("未知巡查线返回 400")
    void unknownDomainRejected() {
        PatrolEventRequest req = new PatrolEventRequest("x:1", "unknown", 1L, Map.of());
        ResponseEntity<Map> r = rest.postForEntity(url(), req, Map.class);
        assertEquals(HttpStatus.BAD_REQUEST, r.getStatusCode());
    }
}
