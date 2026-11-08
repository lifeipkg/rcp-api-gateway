package main

import (
	"log"
	"net/http"
	"time"

	"github.com/lifeipkg/rcp-api-gateway/internal/config"
	"github.com/lifeipkg/rcp-api-gateway/internal/iam"
	"github.com/lifeipkg/rcp-api-gateway/internal/web"
)

// 平台唯一对外入口：鉴权内省 + 路由透传。外部流量只经本服务（RL-002）。
func main() {
	cfg := config.Load()

	router := web.NewRouter(cfg, iam.NewClient(cfg.IAMAddress), iam.NewProxy(cfg.IAMAddress))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("api-gateway listening on :%s, iam=%s", cfg.Port, cfg.IAMAddress)
	log.Fatal(srv.ListenAndServe())
}
