package iam

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIntrospectReturnsPermissions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/iam/introspect" || r.Method != http.MethodPost {
			t.Fatalf("内省调用形态不对: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"token":"token-1"}` {
			t.Fatalf("内省请求体不对: %s", body)
		}
		_, _ = w.Write([]byte(`{"active":true,"userId":"u1","username":"operator","role":"operator","permissions":["patrol:dispatch"]}`))
	}))
	defer server.Close()

	introspection, err := NewClient(server.URL).Introspect(context.Background(), "token-1")
	if err != nil {
		t.Fatalf("有效令牌不应报错: %v", err)
	}
	if !introspection.Has("patrol:dispatch") {
		t.Fatalf("权限未解析，实际 %v", introspection.Permissions)
	}
	if introspection.Has("platform:admin") {
		t.Fatalf("不该凭空获得权限")
	}
}

func TestIntrospectInvalidToken(t *testing.T) {
	// 4xx 与 active=false 都必须归为"令牌无效"，调用方回 401 而不是 500。
	for name, respond := range map[string]func(http.ResponseWriter){
		"400": func(w http.ResponseWriter) { w.WriteHeader(http.StatusBadRequest) },
		"401": func(w http.ResponseWriter) { w.WriteHeader(http.StatusUnauthorized) },
		"inactive": func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"active":false}`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				respond(w)
			}))
			defer server.Close()

			_, err := NewClient(server.URL).Introspect(context.Background(), "bad")
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("应归类为令牌无效，实际 %v", err)
			}
		})
	}
}

func TestIntrospectUnavailableIsNotInvalidToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := NewClient(server.URL).Introspect(context.Background(), "token")
	if err == nil || errors.Is(err, ErrInvalidToken) {
		t.Fatalf("IAM 5xx 必须按不可用处理（503），不能退化成 401 或放行，实际 %v", err)
	}
}

func TestIntrospectConnectionRefusedIsUnavailable(t *testing.T) {
	_, err := NewClient("http://127.0.0.1:1").Introspect(context.Background(), "token")
	if err == nil || errors.Is(err, ErrInvalidToken) {
		t.Fatalf("连接失败必须按不可用处理，实际 %v", err)
	}
}
