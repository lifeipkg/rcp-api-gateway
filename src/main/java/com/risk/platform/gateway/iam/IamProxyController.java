package com.risk.platform.gateway.iam;

import jakarta.servlet.http.HttpServletRequest;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.http.HttpEntity;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpMethod;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.util.StreamUtils;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.client.ResourceAccessException;
import org.springframework.web.client.RestTemplate;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

/**
 * /api/v1/iam/** 透传。
 *
 * 只做搬运：路径、查询串、请求体、Authorization 头原样转给 IAM，状态码与响应体原样
 * 返回。网关不改写 IAM 的错误语义（401/403/409 必须原样到前端），也不在这里判权限——
 * IAM 是权限的唯一判定点。
 */
@RestController
@RequestMapping("/api/v1/iam")
public class IamProxyController {

    private static final Logger log = LoggerFactory.getLogger(IamProxyController.class);

    private final IamAddressResolver resolver;
    private final RestTemplate restTemplate;

    public IamProxyController(IamAddressResolver resolver,
                              @Qualifier("iamProxyRestTemplate") RestTemplate restTemplate) {
        this.resolver = resolver;
        this.restTemplate = restTemplate;
    }

    @GetMapping("/**")
    public ResponseEntity<byte[]> get(HttpServletRequest request) throws IOException {
        return forward(request);
    }

    @PostMapping("/**")
    public ResponseEntity<byte[]> post(HttpServletRequest request) throws IOException {
        return forward(request);
    }

    @PutMapping("/**")
    public ResponseEntity<byte[]> put(HttpServletRequest request) throws IOException {
        return forward(request);
    }

    private ResponseEntity<byte[]> forward(HttpServletRequest request) throws IOException {
        byte[] body = StreamUtils.copyToByteArray(request.getInputStream());
        HttpHeaders headers = new HttpHeaders();
        String authorization = request.getHeader(HttpHeaders.AUTHORIZATION);
        if (authorization != null) {
            headers.set(HttpHeaders.AUTHORIZATION, authorization);
        }
        String contentType = request.getContentType();
        if (contentType != null) {
            headers.set(HttpHeaders.CONTENT_TYPE, contentType);
        }
        headers.set(HttpHeaders.ACCEPT, MediaType.APPLICATION_JSON_VALUE);

        String query = request.getQueryString();
        String target = resolver.resolve() + request.getRequestURI() + (query == null ? "" : "?" + query);

        ResponseEntity<byte[]> response;
        try {
            response = restTemplate.exchange(target,
                    HttpMethod.valueOf(request.getMethod()),
                    new HttpEntity<>(body.length == 0 ? null : body, headers),
                    byte[].class);
        } catch (ResourceAccessException e) {
            log.warn("IAM 不可达: {}", e.getMessage());
            return ResponseEntity.status(HttpStatus.SERVICE_UNAVAILABLE)
                    .contentType(MediaType.APPLICATION_JSON)
                    .body("{\"error\":\"AUTH_UNAVAILABLE\",\"message\":\"鉴权服务不可用\"}"
                            .getBytes(StandardCharsets.UTF_8));
        }

        MediaType responseType = response.getHeaders().getContentType();
        return ResponseEntity.status(response.getStatusCode())
                .contentType(responseType == null ? MediaType.APPLICATION_JSON : responseType)
                .body(response.getBody());
    }
}
