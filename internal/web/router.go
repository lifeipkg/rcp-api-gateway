package web

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifeipkg/rcp-api-gateway/internal/auth"
	"github.com/lifeipkg/rcp-api-gateway/internal/config"
)

// NewRouter 注册全部对外路由。
//
// 业务路由一律写完整字面量路径并直接挂在 r 上：rcp-contracts/tools/contract_check.py
// 会机械抽取 r.GET/r.POST/... 与 OpenAPI 做两向差集，变量拼接或路由分组会让对表失效。
// 非业务路由（探活、IAM 透传）用 r.Handle/r.Any 注册，避免被误算成契约接口。
func NewRouter(cfg config.Config, introspector auth.Introspector, proxy http.Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors(cfg.AllowedOrigins))
	r.Use(auth.Middleware(introspector, auth.Settings{
		AuthEnabled: cfg.AuthEnabled,
		PublicPaths: cfg.PublicPaths,
	}))

	// 探活不属于公开契约路由。
	r.Handle(http.MethodGet, "/actuator/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP"})
	})

	// /api/v1/iam/** 透传：网关不鉴权、不改写状态码（见 internal/auth 第 3 条硬规则）。
	// 逐个方法注册而不写 r.Any：契约里就是 GET/POST/PUT 三条 /api/v1/iam/**，
	// 对表脚本按 r.<METHOD>("字面量") 抽取，r.Any 抽不到会让契约侧的透传口"看起来缺实现"。
	// Gin 的通配是 *path，对表脚本会把 *path 归一成 ** 后与契约比较。
	iamProxy := gin.WrapH(proxy)
	r.GET("/api/v1/iam/*path", iamProxy)
	r.POST("/api/v1/iam/*path", iamProxy)
	r.PUT("/api/v1/iam/*path", iamProxy)

	return r
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
