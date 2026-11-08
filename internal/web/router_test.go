package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lifeipkg/rcp-api-gateway/internal/config"
	"github.com/lifeipkg/rcp-api-gateway/internal/iam"
)

type stubIntrospector struct {
	result iam.Introspection
	err    error
}

func (s *stubIntrospector) Introspect(_ context.Context, _ string) (iam.Introspection, error) {
	return s.result, s.err
}

func testConfig() config.Config {
	return config.Config{
		Port:           "8080",
		AuthEnabled:    true,
		PublicPaths:    []string{"/actuator/**"},
		AllowedOrigins: []string{"http://localhost:5173"},
		IAMAddress:     "http://127.0.0.1:1",
	}
}

func newRouter(introspector *stubIntrospector) http.Handler {
	return NewRouter(testConfig(), introspector, iam.NewProxy("http://127.0.0.1:1"))
}

func do(r http.Handler, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	return recorder
}

func TestCorsAllowsConfiguredOriginOnly(t *testing.T) {
	r := newRouter(&stubIntrospector{})

	allowed := httptest.NewRequest(http.MethodOptions, "/api/v1/iam/login", nil)
	allowed.Header.Set("Origin", "http://localhost:5173")
	allowed.Header.Set("Access-Control-Request-Method", "POST")
	recorder := do(r, allowed)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("预检应 204，实际 %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("允许来源未回写，实际 %q", got)
	}
	if !strings.Contains(recorder.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("预检必须放行 Authorization 头")
	}

	blocked := httptest.NewRequest(http.MethodOptions, "/api/v1/iam/login", nil)
	blocked.Header.Set("Origin", "https://evil.example.com")
	if got := do(r, blocked).Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("未登记来源不得放行，实际 %q", got)
	}
}

func TestHealthEndpoint(t *testing.T) {
	r := newRouter(&stubIntrospector{})
	if recorder := do(r, httptest.NewRequest(http.MethodGet, "/actuator/health", nil)); recorder.Code != http.StatusOK {
		t.Fatalf("健康检查应 200，实际 %d", recorder.Code)
	}
}

// IAM 透传的三种方法都必须挂上：契约里就是 GET/POST/PUT /api/v1/iam/**。
// IAM 地址在测试里指向不可达端口，命中即 503 AUTH_UNAVAILABLE——404 说明路由没注册，
// 401 说明鉴权中间件把透传路径也拦了（两者都违反契约）。
func TestIamPassthroughRoutesAreMounted(t *testing.T) {
	r := newRouter(&stubIntrospector{})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
		request := httptest.NewRequest(method, "/api/v1/iam/users/u1", strings.NewReader("{}"))
		request.Header.Set("Content-Type", "application/json")
		recorder := do(r, request)
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s /api/v1/iam/users/u1 应命中透传（IAM 不可达 → 503），实际 %d", method, recorder.Code)
		}
	}
}
