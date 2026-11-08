package main

import (
	"log"
	"net/http"
	"time"

	"github.com/lifeipkg/rcp-api-gateway/internal/config"
	"github.com/lifeipkg/rcp-api-gateway/internal/iam"
	"github.com/lifeipkg/rcp-api-gateway/internal/patrol"
	"github.com/lifeipkg/rcp-api-gateway/internal/web"
)

// 平台唯一对外入口：事件路由 → 消费幂等(event_id+version) → 巡查线检测
// → 状态机单调前进 → 分级处置。外部流量只经本服务（RL-002）。
func main() {
	cfg := config.Load()

	service := patrol.NewService(patrol.NewHTTPClient(cfg.PatrolTargets))
	router := web.NewRouter(cfg, service, iam.NewClient(cfg.IAMAddress), iam.NewProxy(cfg.IAMAddress))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("api-gateway listening on :%s, iam=%s, patrol=%v", cfg.Port, cfg.IAMAddress, cfg.PatrolTargets)
	log.Fatal(srv.ListenAndServe())
}
