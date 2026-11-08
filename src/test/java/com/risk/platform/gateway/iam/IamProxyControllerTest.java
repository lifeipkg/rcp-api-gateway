package com.risk.platform.gateway.iam;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.client.TestRestTemplate;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.http.HttpEntity;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpMethod;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.test.web.client.MockRestServiceServer;
import org.springframework.web.client.RestTemplate;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.requestTo;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withStatus;

/**
 * IAM 透传：路径/请求体/状态码必须原样搬运。
 * 重点断言"401 原样透回"——网关若把 IAM 的错误改写成 500，前端就看不到真实原因。
 */
@SpringBootTest(
        webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT,
        properties = "spring.cloud.nacos.discovery.enabled=false")
class IamProxyControllerTest {

    @LocalServerPort
    int port;

    @Autowired
    TestRestTemplate rest;

    @Autowired
    @Qualifier("iamProxyRestTemplate")
    RestTemplate iamRestTemplate;

    private MockRestServiceServer iam;

    @BeforeEach
    void bindIamStub() {
        iam = MockRestServiceServer.bindTo(iamRestTemplate).build();
    }

    @Test
    @DisplayName("登录透传：无令牌也放行（由 IAM 自己校验），响应原样返回")
    void loginIsForwarded() {
        iam.expect(requestTo("http://localhost:8085/api/v1/iam/login"))
                .andRespond(withStatus(HttpStatus.OK)
                        .contentType(MediaType.APPLICATION_JSON)
                        .body("{\"accessToken\":\"rcp_abc\",\"tokenType\":\"Bearer\"}"));

        ResponseEntity<Map> response = rest.exchange(url("/api/v1/iam/login"), HttpMethod.POST,
                new HttpEntity<>(Map.of("username", "admin", "password", "Admin@12345"), jsonHeaders()),
                Map.class);

        assertEquals(HttpStatus.OK, response.getStatusCode());
        assertEquals("rcp_abc", response.getBody().get("accessToken"));
        iam.verify();
    }

    @Test
    @DisplayName("IAM 返回 401 时原样透回，不改写成 500（反向用例）")
    void iamErrorIsPassedThrough() {
        iam.expect(requestTo("http://localhost:8085/api/v1/iam/login"))
                .andRespond(withStatus(HttpStatus.UNAUTHORIZED)
                        .contentType(MediaType.APPLICATION_JSON)
                        .body("{\"error\":\"INVALID_CREDENTIALS\",\"message\":\"用户名或口令错误\"}"));

        ResponseEntity<Map> response = rest.exchange(url("/api/v1/iam/login"), HttpMethod.POST,
                new HttpEntity<>(Map.of("username", "admin", "password", "WrongPass1"), jsonHeaders()),
                Map.class);

        assertEquals(HttpStatus.UNAUTHORIZED, response.getStatusCode());
        assertEquals("INVALID_CREDENTIALS", response.getBody().get("error"));
    }

    @Test
    @DisplayName("带 Authorization 头透传到 IAM（用户管理需要令牌）")
    void authorizationHeaderIsForwarded() {
        iam.expect(requestTo("http://localhost:8085/api/v1/iam/users"))
                .andExpect(request -> assertTrue(
                        "Bearer rcp_abc".equals(request.getHeaders().getFirst(HttpHeaders.AUTHORIZATION))))
                .andRespond(withStatus(HttpStatus.OK)
                        .contentType(MediaType.APPLICATION_JSON)
                        .body("[]"));

        HttpHeaders headers = jsonHeaders();
        headers.setBearerAuth("rcp_abc");
        ResponseEntity<String> response = rest.exchange(url("/api/v1/iam/users"), HttpMethod.GET,
                new HttpEntity<>(headers), String.class);

        assertEquals(HttpStatus.OK, response.getStatusCode());
        iam.verify();
    }

    private HttpHeaders jsonHeaders() {
        HttpHeaders headers = new HttpHeaders();
        headers.setContentType(MediaType.APPLICATION_JSON);
        return headers;
    }

    private String url(String path) {
        return "http://localhost:" + port + path;
    }
}
