package iam

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyPassesThroughStatusAndBody(t *testing.T) {
	var gotAuthorization, gotQuery, gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotQuery = r.URL.RawQuery
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"USER_EXISTS"}`))
	}))
	defer server.Close()

	proxy := NewProxy(server.URL)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/iam/users?dryRun=1",
		strings.NewReader(`{"username":"a"}`))
	request.Header.Set("Authorization", "Bearer token-1")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	proxy.ServeHTTP(recorder, request)

	// 反向断言：IAM 的 409 必须原样到前端；改写成 500 会让前端看不到真实原因。
	if recorder.Code != http.StatusConflict {
		t.Fatalf("状态码应原样透传 409，实际 %d", recorder.Code)
	}
	if recorder.Body.String() != `{"error":"USER_EXISTS"}` {
		t.Fatalf("响应体应原样透传，实际 %s", recorder.Body.String())
	}
	if gotAuthorization != "Bearer token-1" {
		t.Fatalf("Authorization 未转发，实际 %q", gotAuthorization)
	}
	if gotPath != "/api/v1/iam/users" || gotQuery != "dryRun=1" {
		t.Fatalf("路径/查询串未原样转发: %s?%s", gotPath, gotQuery)
	}
	if gotBody != `{"username":"a"}` {
		t.Fatalf("请求体未原样转发: %s", gotBody)
	}
}

func TestProxyUnavailableIs503(t *testing.T) {
	proxy := NewProxy("http://127.0.0.1:1")
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/iam/me", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("IAM 不可达应回 503，实际 %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "AUTH_UNAVAILABLE") {
		t.Fatalf("503 响应体应带 AUTH_UNAVAILABLE，实际 %s", recorder.Body.String())
	}
}
