package patrol

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientPostsToCheckEndpoint(t *testing.T) {
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"eventId":"job:1","decision":"WARN","score":20,"reason":"rule:keyword"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(map[string]string{"content": server.URL})
	verdict, err := client.Check(context.Background(), "content", map[string]any{"eventId": "job:1"})
	if err != nil {
		t.Fatalf("正常响应不应报错: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/patrol/check" {
		t.Fatalf("调用形态不对: %s %s", gotMethod, gotPath)
	}
	if verdict.Decision != "WARN" {
		t.Fatalf("裁决未解析，实际 %s", verdict.Decision)
	}
	if verdict.Detail == nil {
		t.Fatalf("detail 应归一为空对象而不是 nil")
	}
}

func TestHTTPClientRejectsUnknownDomain(t *testing.T) {
	client := NewHTTPClient(map[string]string{"content": "http://127.0.0.1:1"})
	if _, err := client.Check(context.Background(), "unknown", map[string]any{}); err == nil {
		t.Fatalf("未知巡查线应返回错误，由上层降级处理")
	}
}

func TestHTTPClientTreatsNon200AsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewHTTPClient(map[string]string{"content": server.URL})
	if _, err := client.Check(context.Background(), "content", map[string]any{}); err == nil {
		t.Fatalf("5xx 必须当作不可用（降级），不能当成放行")
	}
}

func TestHTTPClientRejectsUndeclaredDecision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"eventId":"job:1","decision":"TOTALLY_FINE","score":0,"reason":"x"}`))
	}))
	defer server.Close()

	client := NewHTTPClient(map[string]string{"content": server.URL})
	if _, err := client.Check(context.Background(), "content", map[string]any{}); err == nil {
		t.Fatalf("未声明决策必须报错并降级，不能透传成放行")
	}
}
