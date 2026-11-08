package iam

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ErrInvalidToken 表示令牌无效/过期/已吊销：调用方必须回 401。
// 其余错误（连接失败、5xx）都按"鉴权不可用"处理，调用方必须回 503（fail closed）。
var ErrInvalidToken = errors.New("令牌无效")

// Introspection 是内省结果（网关侧视图）：只保留鉴权需要的字段。
type Introspection struct {
	Active      bool     `json:"active"`
	UserID      string   `json:"userId"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

// Has 判断内省结果是否包含某权限。
func (i Introspection) Has(permission string) bool {
	for _, granted := range i.Permissions {
		if granted == permission {
			return true
		}
	}
	return false
}

// Client 是 IAM 内省客户端：POST {iam}/api/v1/iam/introspect。
//
// 网关不解析令牌、不存角色表：权限展开只在 rcp-iam 一处。
// 超时预算 connect 300ms / 整体 1s——登录与内省不是热路径，但必须有上限，
// 否则 IAM 卡住会拖垮整个网关。
type Client struct {
	address string
	http    *http.Client
}

func NewClient(address string) *Client {
	return &Client{
		address: strings.TrimSuffix(address, "/"),
		http: &http.Client{
			Timeout: time.Second,
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 300 * time.Millisecond}).DialContext,
				ResponseHeaderTimeout: 800 * time.Millisecond,
				MaxIdleConnsPerHost:   64,
			},
		},
	}
}

// Introspect 判定令牌。令牌无效返回 ErrInvalidToken；IAM 不可达返回其它 error。
func (c *Client) Introspect(ctx context.Context, token string) (Introspection, error) {
	body, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		return Introspection{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.address+"/api/v1/iam/introspect", bytes.NewReader(body))
	if err != nil {
		return Introspection{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Introspection{}, fmt.Errorf("IAM 内省不可达: %w", err)
	}
	defer resp.Body.Close()

	// 4xx 不可能是"令牌有效"：按无效处理，让调用方回 401。
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return Introspection{}, ErrInvalidToken
	}
	if resp.StatusCode != http.StatusOK {
		return Introspection{}, fmt.Errorf("IAM 内省返回 %d", resp.StatusCode)
	}

	var result Introspection
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Introspection{}, fmt.Errorf("IAM 内省响应不是合法 JSON: %w", err)
	}
	if !result.Active {
		return Introspection{}, ErrInvalidToken
	}
	if result.Permissions == nil {
		result.Permissions = []string{}
	}
	return result, nil
}
