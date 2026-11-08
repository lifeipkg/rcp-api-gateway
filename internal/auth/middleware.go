package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifeipkg/rcp-api-gateway/internal/iam"
)

// Introspector 是网关对 rcp-iam 的唯一依赖面；测试可替换。
type Introspector interface {
	Introspect(ctx context.Context, token string) (iam.Introspection, error)
}

// Settings 是鉴权行为配置。
type Settings struct {
	AuthEnabled bool
	PublicPaths []string
}

// 受控路由 → 所需权限。新增受控路由只改这里，不在 handler 里散着判。
const (
	patrolDispatchPath       = "/api/v1/patrol/dispatch"
	patrolDispatchPermission = "patrol:dispatch"
)

// Middleware 是统一鉴权：令牌 → IAM 内省 → 权限判定。
//
// 三条硬规则：
//  1. fail closed——令牌缺失/无效、IAM 不可用，统统拒绝。鉴权链路上没有"降级放行"。
//  2. 网关不解析令牌、不存角色表：权限以 IAM 返回的 permissions 为准。
//  3. /api/v1/iam/** 不在此处鉴权：登录、内省、用户管理由 IAM 自己校验，网关只透传。
func Middleware(introspector Introspector, settings Settings) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !settings.AuthEnabled {
			c.Next()
			return
		}
		// 浏览器 CORS 预检不带 Authorization，放行交给 CORS 中间件
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/v1/iam") {
			c.Next()
			return
		}
		if isPublic(settings.PublicPaths, path) {
			c.Next()
			return
		}

		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			reject(c, http.StatusUnauthorized, "MISSING_TOKEN", "缺少 Bearer 令牌")
			return
		}

		introspection, err := introspector.Introspect(c.Request.Context(), token)
		if errors.Is(err, iam.ErrInvalidToken) {
			reject(c, http.StatusUnauthorized, "INVALID_TOKEN", "令牌无效或已吊销")
			return
		}
		if err != nil {
			log.Printf("IAM 内省不可用，按拒绝处理: %v", err)
			reject(c, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "鉴权服务不可用")
			return
		}

		required := requiredPermission(c.Request.Method, path)
		if required != "" && !introspection.Has(required) {
			reject(c, http.StatusForbidden, "FORBIDDEN", "缺少权限: "+required)
			return
		}

		c.Set("iam.userId", introspection.UserID)
		c.Set("iam.username", introspection.Username)
		c.Set("iam.role", introspection.Role)
		c.Next()
	}
}

// bearerToken 只认 Bearer 方案，缺失或格式不对一律视为无令牌。
func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	const scheme = "Bearer "
	if len(header) <= len(scheme) || !strings.EqualFold(header[:len(scheme)], scheme) {
		return ""
	}
	return strings.TrimSpace(header[len(scheme):])
}

func requiredPermission(method, path string) string {
	if method == http.MethodPost && path == patrolDispatchPath {
		return patrolDispatchPermission
	}
	return ""
}

// isPublic 判定公开路径：支持精确匹配与 "/**" 尾部通配（与 Java 版 AntPathMatcher 对齐）。
func isPublic(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				return true
			}
			continue
		}
		if pattern == path {
			return true
		}
	}
	return false
}

func reject(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code, "message": message})
}
