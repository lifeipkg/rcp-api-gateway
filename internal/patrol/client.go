package patrol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/lifeipkg/rcp-api-gateway/internal/model"
)

// Client 是巡查线调用抽象：便于测试替换，也便于将来换 gRPC/Kafka 形态。
type Client interface {
	// Check 调用巡查线检测。返回 error 时由调用方降级（转人工加审），不放行。
	Check(ctx context.Context, domain string, payload map[string]any) (model.Verdict, error)
}

// HTTPClient 是 HTTP 形态：POST {endpoint}/api/v1/patrol/check。
//
// 超时约束：同步巡查链路预算 200ms，connect 100ms / read 500ms（兜底留量）。
// 无 discovery，无服务端轮转：地址来自部署侧注入的服务名，负载由 k8s Service 分发。
type HTTPClient struct {
	targets map[string]string
	http    *http.Client
}

func NewHTTPClient(targets map[string]string) *HTTPClient {
	return &HTTPClient{
		targets: targets,
		http: &http.Client{
			Timeout: 500 * time.Millisecond,
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext,
				ResponseHeaderTimeout: 500 * time.Millisecond,
				MaxIdleConnsPerHost:   32,
			},
		},
	}
}

// Check 实现 Client。未知 domain 视为调用失败（由上层降级），不猜地址。
func (c *HTTPClient) Check(ctx context.Context, domain string, payload map[string]any) (model.Verdict, error) {
	target, ok := c.targets[domain]
	if !ok || target == "" {
		return model.Verdict{}, fmt.Errorf("未知巡查线: %s", domain)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return model.Verdict{}, fmt.Errorf("payload 序列化失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		target+"/api/v1/patrol/check", bytes.NewReader(body))
	if err != nil {
		return model.Verdict{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return model.Verdict{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return model.Verdict{}, fmt.Errorf("巡查线返回 %d", resp.StatusCode)
	}

	var verdict model.Verdict
	if err := json.NewDecoder(resp.Body).Decode(&verdict); err != nil {
		return model.Verdict{}, fmt.Errorf("巡查线响应不是合法裁决: %w", err)
	}
	if !verdict.Decision.Valid() {
		// 未声明的决策不猜、不放行：当成巡查线异常，走降级。
		return model.Verdict{}, fmt.Errorf("巡查线返回未声明决策: %s", verdict.Decision)
	}
	return verdict.Normalize(), nil
}
