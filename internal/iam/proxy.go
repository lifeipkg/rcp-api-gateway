package iam

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Proxy 是 /api/v1/iam/** 的透传口。
//
// 只做搬运：路径、查询串、请求体、Authorization 头原样转给 IAM，状态码与响应体原样
// 返回。网关不改写 IAM 的错误语义（401/403/409 必须原样到前端），也不在这里判权限。
// 用 http.Handler 而不是 gin HandlerFunc：透传不参与路由分组，测试也可以直接打。
type Proxy struct {
	address string
	http    *http.Client
}

func NewProxy(address string) *Proxy {
	return &Proxy{
		address: strings.TrimSuffix(address, "/"),
		http: &http.Client{
			// IAM 的登录/改密不是热路径，给足 2s；但必须有上限。
			Timeout: 2 * time.Second,
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 300 * time.Millisecond}).DialContext,
				ResponseHeaderTimeout: 2 * time.Second,
				MaxIdleConnsPerHost:   64,
			},
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请求体读取失败")
		return
	}

	target := p.address + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "鉴权服务不可用")
		return
	}
	if authorization := r.Header.Get("Authorization"); authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "鉴权服务不可用")
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "鉴权服务不可用")
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(responseBody)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + code + `","message":"` + message + `"}`))
}
