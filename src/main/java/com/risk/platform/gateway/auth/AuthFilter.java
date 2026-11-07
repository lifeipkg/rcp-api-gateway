package com.risk.platform.gateway.auth;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.risk.platform.gateway.config.IamProperties;
import com.risk.platform.gateway.iam.IamClient;
import com.risk.platform.gateway.iam.Introspection;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.util.AntPathMatcher;
import org.springframework.web.filter.OncePerRequestFilter;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Map;

/**
 * 统一鉴权：令牌 → IAM 内省 → 权限判定。
 *
 * 三条硬规则：
 *   1. fail closed——令牌缺失/无效、IAM 不可用，统统拒绝。鉴权链路上没有"降级放行"。
 *   2. 网关不解析令牌、不存角色表：权限以 IAM 返回的 permissions 为准。
 *   3. /api/v1/iam/** 不在此处鉴权：登录、内省、用户管理由 IAM 自己校验，
 *      网关只透传，避免两处各判一遍、结论还可能不一致。
 */
@Component
public class AuthFilter extends OncePerRequestFilter {

    private static final Logger log = LoggerFactory.getLogger(AuthFilter.class);

    /** 需要 patrol:dispatch 的唯一路由（其余受保护路由只要求"已登录"）。 */
    private static final String PATROL_DISPATCH_PATH = "/api/v1/patrol/dispatch";
    private static final String PATROL_DISPATCH_PERMISSION = "patrol:dispatch";

    private final IamClient iamClient;
    private final IamProperties properties;
    private final ObjectMapper objectMapper;
    private final AntPathMatcher matcher = new AntPathMatcher();

    public AuthFilter(IamClient iamClient, IamProperties properties, ObjectMapper objectMapper) {
        this.iamClient = iamClient;
        this.properties = properties;
        this.objectMapper = objectMapper;
    }

    @Override
    protected boolean shouldNotFilter(HttpServletRequest request) {
        if (!properties.getAuth().isEnabled()) {
            return true;
        }
        // 浏览器 CORS 预检不带 Authorization，放行交给 WebCorsConfig 处理
        if ("OPTIONS".equalsIgnoreCase(request.getMethod())) {
            return true;
        }
        String path = request.getRequestURI();
        // 同时覆盖 /api/v1/iam/** 与 /api/v1/iam/**：两个 IAM 都在服务内自行校验
        // 令牌，网关只透传，避免两处各判一遍、结论还可能不一致。
        if (path.startsWith("/api/v1/iam")) {
            return true;
        }
        return properties.getAuth().getPublicPaths().stream().anyMatch(pattern -> matcher.match(pattern, path));
    }

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
            throws ServletException, IOException {
        String token = BearerToken.extract(request.getHeader(HttpHeaders.AUTHORIZATION));
        if (token == null) {
            reject(response, HttpStatus.UNAUTHORIZED, "MISSING_TOKEN", "缺少 Bearer 令牌");
            return;
        }

        Introspection introspection;
        try {
            introspection = iamClient.introspect(token).orElse(null);
        } catch (RuntimeException e) {
            log.warn("IAM 内省不可用，按拒绝处理: {}", e.getMessage());
            reject(response, HttpStatus.SERVICE_UNAVAILABLE, "AUTH_UNAVAILABLE", "鉴权服务不可用");
            return;
        }
        if (introspection == null) {
            reject(response, HttpStatus.UNAUTHORIZED, "INVALID_TOKEN", "令牌无效或已吊销");
            return;
        }

        String required = requiredPermission(request);
        if (required != null && !introspection.permissions().contains(required)) {
            reject(response, HttpStatus.FORBIDDEN, "FORBIDDEN", "缺少权限: " + required);
            return;
        }

        request.setAttribute("iam.userId", introspection.userId());
        request.setAttribute("iam.username", introspection.username());
        request.setAttribute("iam.role", introspection.role());
        chain.doFilter(request, response);
    }

    /** 路由 → 所需权限的映射。新增受控路由只改这里，不在控制器里散着判。 */
    private String requiredPermission(HttpServletRequest request) {
        if ("POST".equalsIgnoreCase(request.getMethod())
                && PATROL_DISPATCH_PATH.equals(request.getRequestURI())) {
            return PATROL_DISPATCH_PERMISSION;
        }
        return null;
    }

    private void reject(HttpServletResponse response, HttpStatus status, String code, String message)
            throws IOException {
        response.setStatus(status.value());
        response.setContentType(MediaType.APPLICATION_JSON_VALUE);
        response.setCharacterEncoding(StandardCharsets.UTF_8.name());
        objectMapper.writeValue(response.getWriter(), Map.of("error", code, "message", message));
    }
}
