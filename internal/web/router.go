package web

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifeipkg/rcp-api-gateway/internal/auth"
	"github.com/lifeipkg/rcp-api-gateway/internal/config"
	"github.com/lifeipkg/rcp-api-gateway/internal/model"
	"github.com/lifeipkg/rcp-api-gateway/internal/patrol"
)

// Handler 是平台统一接入入口的实现。
type Handler struct {
	service *patrol.Service
}

// NewRouter 注册全部对外路由。
//
// 业务路由一律写完整字面量路径并直接挂在 r 上：rcp-contracts/tools/contract_check.py
// 会机械抽取 r.GET/r.POST/... 与 OpenAPI 做两向差集，变量拼接或路由分组会让对表失效。
// 非业务路由（探活、IAM 透传）用 r.Handle/r.Any 注册，避免被误算成契约接口。
func NewRouter(cfg config.Config, service *patrol.Service, introspector auth.Introspector, proxy http.Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors(cfg.AllowedOrigins))
	r.Use(auth.Middleware(introspector, auth.Settings{
		AuthEnabled: cfg.AuthEnabled,
		PublicPaths: cfg.PublicPaths,
	}))

	h := &Handler{service: service}

	// 探活不属于公开契约路由。
	r.Handle(http.MethodGet, "/actuator/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	// /api/v1/iam/** 透传：网关不鉴权、不改写状态码（见 internal/auth 第 3 条硬规则）。
	r.Any("/api/v1/iam", gin.WrapH(proxy))
	r.Any("/api/v1/iam/*path", gin.WrapH(proxy))

	r.POST("/api/v1/patrol/dispatch", h.dispatch)
	r.GET("/api/v1/patrol/domains", h.domains)

	return r
}

func (h *Handler) dispatch(c *gin.Context) {
	var req model.EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求体不是合法 JSON")
		return
	}
	if strings.TrimSpace(req.EventID) == "" {
		badRequest(c, "eventId 不能为空")
		return
	}
	if strings.TrimSpace(req.Domain) == "" {
		badRequest(c, "domain 不能为空")
		return
	}
	if !isKnownDomain(req.Domain) {
		badRequest(c, "未知巡查线: "+req.Domain)
		return
	}
	// 契约里 version 是 minimum: 1 的必填字段；缺省与 0 都不受理。
	if req.Version == nil || *req.Version < 1 {
		badRequest(c, "version 必须是大于等于 1 的整数")
		return
	}
	if req.Payload == nil {
		req.Payload = map[string]any{}
	}
	c.JSON(http.StatusOK, h.service.Dispatch(c.Request.Context(), req))
}

func (h *Handler) domains(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"platform":    "risk-control-platform",
		"patrolLines": model.Domains,
	})
}

func isKnownDomain(domain string) bool {
	for _, candidate := range model.Domains {
		if candidate == domain {
			return true
		}
	}
	return false
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "BAD_REQUEST", "message": message})
}

// cors 只允许配置内的来源：默认本地运营台，**不使用 "*"**，
// 避免把网关变成任意站点可调的开放接口。
func cors(origins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		allowed[origin] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] && strings.HasPrefix(c.Request.URL.Path, "/api/") {
			header := c.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Set("Vary", "Origin")
			header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			header.Set("Access-Control-Max-Age", "3600")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
