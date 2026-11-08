package config

import (
	"os"
	"strconv"
	"strings"
)

// Config 只从环境变量取值：部署清单里只出现服务名与端口，不出现上游 URL（RL-007）。
//
// 不接注册中心：按部署侧 Service 名经 DNS 解析（rcp:fallback = k8s-dns），
// 多实例负载由 k8s Service / compose DNS 承担，网关不自己轮转。
type Config struct {
	Port           string
	AuthEnabled    bool
	PublicPaths    []string
	AllowedOrigins []string
	IAMAddress     string
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
	return Config{
		Port:        env("SERVER_PORT", "8080"),
		AuthEnabled: envBool("IAM_AUTH_ENABLED", true),
		PublicPaths: envList("IAM_PUBLIC_PATHS", "/actuator/**"),
		AllowedOrigins: envList("GATEWAY_WEB_ALLOWED_ORIGINS",
			"http://localhost:5173,http://127.0.0.1:5173"),
		IAMAddress: address("IAM", "localhost", "8085"),
	}
}
