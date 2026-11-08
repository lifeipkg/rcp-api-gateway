package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lifeipkg/rcp-api-gateway/internal/iam"
)

type stubIntrospector struct {
	result iam.Introspection
	err    error
	called bool
}

func (s *stubIntrospector) Introspect(_ context.Context, _ string) (iam.Introspection, error) {
	s.called = true
	return s.result, s.err
}

func newTestRouter(introspector Introspector, settings Settings) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(introspector, settings))
	r.POST("/api/v1/patrol/dispatch", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/api/v1/patrol/domains", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.Any("/api/v1/iam/*path", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"proxy": true}) })
	r.Handle(http.MethodGet, "/actuator/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func serve(r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	return recorder
}

func enabled(introspector Introspector) *gin.Engine {
	return newTestRouter(introspector, Settings{
		AuthEnabled: true,
		PublicPaths: []string{"/actuator/**", "/api/v1/patrol/domains"},
	})
}

func TestMissingTokenIs401(t *testing.T) {
	stub := &stubIntrospector{}
	recorder := serve(enabled(stub), http.MethodPost, "/api/v1/patrol/dispatch", "")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("缺令牌应 401，实际 %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "MISSING_TOKEN") {
		t.Fatalf("缺少 MISSING_TOKEN 标识，实际 %s", recorder.Body.String())
	}
	if stub.called {
		t.Fatalf("无令牌时不该调用内省")
	}
}

func TestInvalidTokenIs401(t *testing.T) {
	stub := &stubIntrospector{err: iam.ErrInvalidToken}
	recorder := serve(enabled(stub), http.MethodPost, "/api/v1/patrol/dispatch", "bad")

	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "INVALID_TOKEN") {
		t.Fatalf("无效令牌应 401 INVALID_TOKEN，实际 %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestIAMUnavailableIs503NeverPass(t *testing.T) {
	stub := &stubIntrospector{err: errors.New("dial tcp: connection refused")}
	recorder := serve(enabled(stub), http.MethodPost, "/api/v1/patrol/dispatch", "token")

	// 反向断言：IAM 不可达绝不能放行，也不能退化成 401 让前端误判成登录态问题。
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("IAM 不可用应 503 fail closed，实际 %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "AUTH_UNAVAILABLE") {
		t.Fatalf("缺少 AUTH_UNAVAILABLE 标识，实际 %s", recorder.Body.String())
	}
}

func TestMissingPermissionIs403(t *testing.T) {
	stub := &stubIntrospector{result: iam.Introspection{Active: true, Permissions: []string{"viewer"}}}
	recorder := serve(enabled(stub), http.MethodPost, "/api/v1/patrol/dispatch", "token")

	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "patrol:dispatch") {
		t.Fatalf("缺权限应 403 并指明权限，实际 %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestGrantedPermissionPasses(t *testing.T) {
	stub := &stubIntrospector{result: iam.Introspection{Active: true, Permissions: []string{"patrol:dispatch"}}}
	if recorder := serve(enabled(stub), http.MethodPost, "/api/v1/patrol/dispatch", "token"); recorder.Code != http.StatusOK {
		t.Fatalf("有权限应放行，实际 %d", recorder.Code)
	}
}

func TestPublicPathsSkipAuth(t *testing.T) {
	stub := &stubIntrospector{}
	for _, path := range []string{"/api/v1/patrol/domains", "/actuator/health"} {
		if recorder := serve(enabled(stub), http.MethodGet, path, ""); recorder.Code != http.StatusOK {
			t.Fatalf("%s 应公开，实际 %d", path, recorder.Code)
		}
	}
	if stub.called {
		t.Fatalf("公开路径不该调用内省")
	}
}

func TestIamPathSkipsAuth(t *testing.T) {
	stub := &stubIntrospector{err: errors.New("不该被调用")}
	if recorder := serve(enabled(stub), http.MethodPost, "/api/v1/iam/login", ""); recorder.Code != http.StatusOK {
		t.Fatalf("/api/v1/iam/** 应由 IAM 自校验，网关不拦，实际 %d", recorder.Code)
	}
	if stub.called {
		t.Fatalf("/api/v1/iam/** 不该调用内省")
	}
}

func TestPreflightSkipsAuth(t *testing.T) {
	stub := &stubIntrospector{}
	if recorder := serve(enabled(stub), http.MethodOptions, "/api/v1/patrol/dispatch", ""); recorder.Code == http.StatusUnauthorized {
		t.Fatalf("CORS 预检不带令牌，不该 401")
	}
	if stub.called {
		t.Fatalf("预检不该调用内省")
	}
}

func TestAuthDisabledPassesEverything(t *testing.T) {
	stub := &stubIntrospector{}
	r := newTestRouter(stub, Settings{AuthEnabled: false})
	if recorder := serve(r, http.MethodPost, "/api/v1/patrol/dispatch", ""); recorder.Code != http.StatusOK {
		t.Fatalf("关闭鉴权后应放行，实际 %d", recorder.Code)
	}
	if stub.called {
		t.Fatalf("关闭鉴权后不该调用内省")
	}
}
