package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lifeipkg/rcp-api-gateway/internal/config"
	"github.com/lifeipkg/rcp-api-gateway/internal/iam"
	"github.com/lifeipkg/rcp-api-gateway/internal/model"
	"github.com/lifeipkg/rcp-api-gateway/internal/patrol"
)

type stubPatrol struct {
	verdict model.Verdict
	err     error
}

func (s *stubPatrol) Check(_ context.Context, _ string, _ map[string]any) (model.Verdict, error) {
	return s.verdict, s.err
}

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
		PublicPaths:    []string{"/actuator/**", "/api/v1/patrol/domains"},
		AllowedOrigins: []string{"http://localhost:5173"},
		IAMAddress:     "http://127.0.0.1:1",
		PatrolTargets:  map[string]string{},
	}
}

func newRouter(client patrol.Client, introspector *stubIntrospector) http.Handler {
	return NewRouter(testConfig(), patrol.NewService(client), introspector,
		iam.NewProxy("http://127.0.0.1:1"))
}

func do(r http.Handler, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	return recorder
}

func dispatchRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/patrol/dispatch", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer token-1")
	return request
}

func TestDomainsIsPublic(t *testing.T) {
	r := newRouter(&stubPatrol{}, &stubIntrospector{})
	recorder := do(r, httptest.NewRequest(http.MethodGet, "/api/v1/patrol/domains", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("domains 应公开返回 200，实际 %d", recorder.Code)
	}
	var body struct {
		Platform    string   `json:"platform"`
		PatrolLines []string `json:"patrolLines"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if body.Platform != "risk-control-platform" || len(body.PatrolLines) != 4 {
		t.Fatalf("平台自描述内容不对: %+v", body)
	}
}

func TestDispatchReturnsVerdictAndDisposition(t *testing.T) {
	client := &stubPatrol{verdict: model.Verdict{
		EventID: "job:1", Decision: model.DecisionWarn, Score: 20, Reason: "rule:keyword",
	}}
	introspector := &stubIntrospector{result: iam.Introspection{Active: true, Permissions: []string{"patrol:dispatch"}}}
	r := newRouter(client, introspector)

	recorder := do(r, dispatchRequest(`{"eventId":"job:1","domain":"content","version":1,"payload":{"jobId":1}}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("应 200，实际 %d %s", recorder.Code, recorder.Body.String())
	}
	var body model.EventResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if !body.Accepted || body.Verdict == nil || body.Verdict.Decision != model.DecisionWarn {
		t.Fatalf("裁决未按预期返回: %s", recorder.Body.String())
	}
	if body.Disposition == nil || body.Disposition.Action != "NOTIFY" {
		t.Fatalf("处置未按预期返回: %s", recorder.Body.String())
	}
	// 契约要求这两个字段必须出现（即使为 null/false）。
	if !strings.Contains(recorder.Body.String(), `"rejectReason"`) ||
		!strings.Contains(recorder.Body.String(), `"stateRollbackProtected"`) {
		t.Fatalf("响应缺契约必填字段: %s", recorder.Body.String())
	}
}

func TestDispatchValidatesRequest(t *testing.T) {
	client := &stubPatrol{verdict: model.Verdict{EventID: "job:1", Decision: model.DecisionPass}}
	introspector := &stubIntrospector{result: iam.Introspection{Active: true, Permissions: []string{"patrol:dispatch"}}}
	r := newRouter(client, introspector)

	cases := map[string]string{
		"未知域":     `{"eventId":"job:1","domain":"order","version":1}`,
		"缺版本":     `{"eventId":"job:1","domain":"content"}`,
		"版本小于 1":  `{"eventId":"job:1","domain":"content","version":0}`,
		"缺事件ID":    `{"domain":"content","version":1}`,
		"空事件ID":    `{"eventId":"  ","domain":"content","version":1}`,
		"非 JSON":   `not-json`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if recorder := do(r, dispatchRequest(payload)); recorder.Code != http.StatusBadRequest {
				t.Fatalf("应 400，实际 %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestDegradedDispatchIsNotReleased(t *testing.T) {
	client := &stubPatrol{err: context.DeadlineExceeded}
	introspector := &stubIntrospector{result: iam.Introspection{Active: true, Permissions: []string{"patrol:dispatch"}}}
	r := newRouter(client, introspector)

	recorder := do(r, dispatchRequest(`{"eventId":"job:9","domain":"fund","version":1}`))
	if !strings.Contains(recorder.Body.String(), "MANUAL_REVIEW") {
		t.Fatalf("巡查线不可用必须转人工加审，实际 %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"RELEASE"`) {
		t.Fatalf("巡查线不可用绝不放行，实际 %s", recorder.Body.String())
	}
}

func TestCorsAllowsConfiguredOriginOnly(t *testing.T) {
	r := newRouter(&stubPatrol{}, &stubIntrospector{})

	allowed := httptest.NewRequest(http.MethodOptions, "/api/v1/patrol/dispatch", nil)
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

	blocked := httptest.NewRequest(http.MethodOptions, "/api/v1/patrol/dispatch", nil)
	blocked.Header.Set("Origin", "https://evil.example.com")
	if got := do(r, blocked).Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("未登记来源不得放行，实际 %q", got)
	}
}

func TestHealthEndpoint(t *testing.T) {
	r := newRouter(&stubPatrol{}, &stubIntrospector{})
	if recorder := do(r, httptest.NewRequest(http.MethodGet, "/actuator/health", nil)); recorder.Code != http.StatusOK {
		t.Fatalf("健康检查应 200，实际 %d", recorder.Code)
	}
}
