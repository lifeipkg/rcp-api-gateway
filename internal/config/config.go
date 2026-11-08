package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/lifeipkg/rcp-api-gateway/internal/model"
)

// Config 只从环境变量取值：部署清单里只出现服务名与端口，不出现上游 URL（RL-007）。
//
// 与 Java 版的差别（破坏性）：去掉 Nacos 客户端。Go 侧不做注册中心接线，
// 按部署侧 Service 名经 DNS 解析（rcp:fallback = k8s-dns），多实例负载由
// k8s Service / compose DNS 承担，网关不再自己轮转。
type Config struct {
	Port           string
	AuthEnabled    bool
	PublicPaths    []string
	AllowedOrigins []string
	IAMAddress     string
	// PatrolTargets 是 domain -> 巡查线基础地址。
	PatrolTargets map[string]string
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envList(key, fallback string) []string {
	items := []string{}
	for _, part := range strings.Split(env(key, fallback), ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func envBool(key string, fallback bool) bool {
	raw, err := strconv.ParseBool(env(key, strconv.FormatBool(fallback)))
	if err != nil {
		return fallback
	}
	return raw
}

// address 由「服务名 + 端口」拼基础地址，签名与 rcp-iam 的 DATAWAY_HOST/PORT 一致。
func address(prefix, defaultHost, defaultPort string) string {
	return "http://" + env(prefix+"_HOST", defaultHost) + ":" + env(prefix+"_PORT", defaultPort)
}

func Load() Config {
	targets := make(map[string]string, len(model.Domains))
	for _, domain := range model.Domains {
		// PATROL_<DOMAIN>_HOST / _PORT，例：PATROL_CONTENT_HOST=content-patrol
		targets[domain] = address("PATROL_"+strings.ToUpper(domain), "localhost", defaultPatrolPort(domain))
	}
	return Config{
		Port:        env("SERVER_PORT", "8080"),
		AuthEnabled: envBool("IAM_AUTH_ENABLED", true),
		PublicPaths: envList("IAM_PUBLIC_PATHS", "/actuator/**,/api/v1/patrol/domains"),
		AllowedOrigins: envList("PATROL_WEB_ALLOWED_ORIGINS",
			"http://localhost:5173,http://127.0.0.1:5173"),
		IAMAddress:    address("IAM", "localhost", "8085"),
		PatrolTargets: targets,
	}
}

// defaultPatrolPort 保持与各巡查线登记端口一致（services.ttl）。
func defaultPatrolPort(domain string) string {
	switch domain {
	case "content":
		return "8081"
	case "message":
		return "8082"
	case "account":
		return "8083"
	case "fund":
		return "8084"
	default:
		return "0"
	}
}
