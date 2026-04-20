package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJumpServerPostJSONRoundTripsRequestAndResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", req.Method)
		}
		if req.URL.Path != "/api/v1/ops/jobs/" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		if got := req.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("unexpected accept header: %q", got)
		}
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content type: %q", got)
		}
		if got := req.Header.Get("X-JMS-ORG"); got != "org-1" {
			t.Fatalf("unexpected org header: %q", got)
		}
		if got := req.Header.Get("Authorization"); !strings.Contains(got, `keyId="ak-test"`) || !strings.Contains(got, `signature="`) {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["args"] != "whoami" {
			t.Fatalf("unexpected args: %+v", payload["args"])
		}
		assets, ok := payload["assets"].([]interface{})
		if !ok || len(assets) != 1 || assets[0] != "asset-1" {
			t.Fatalf("unexpected assets: %+v", payload["assets"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"job-1","task_id":"task-1"}`))
	}))
	defer srv.Close()

	runtime := NewJumpServerRuntime(srv.Client())
	cfg := jumpServerConfig{
		BaseURL:   srv.URL,
		AccessKey: "ak-test",
		SecretKey: "sk-test",
		OrgID:     "org-1",
	}
	var response jumpServerExecutionResponse
	err := runtime.postJSON(context.Background(), cfg, "/api/v1/ops/jobs/", map[string]interface{}{
		"args":    "whoami",
		"assets":  []string{"asset-1"},
		"timeout": 5,
	}, &response)
	if err != nil {
		t.Fatalf("post json: %v", err)
	}
	if response.ID != "job-1" || response.TaskID != "task-1" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestJumpServerPostJSONHandlesErrorBranches(t *testing.T) {
	t.Parallel()

	t.Run("marshal error", func(t *testing.T) {
		t.Parallel()

		runtime := NewJumpServerRuntime(nil)
		err := runtime.postJSON(context.Background(), jumpServerConfig{BaseURL: "https://jumpserver.example.test", AccessKey: "ak", SecretKey: "sk"}, "/api/v1/ops/jobs/", map[string]interface{}{
			"bad": make(chan int),
		}, nil)
		if err == nil || !strings.Contains(err.Error(), "unsupported type") {
			t.Fatalf("expected marshal error, got %v", err)
		}
	})

	t.Run("status error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("job submission failed"))
		}))
		defer srv.Close()

		runtime := NewJumpServerRuntime(srv.Client())
		err := runtime.postJSON(context.Background(), jumpServerConfig{BaseURL: srv.URL, AccessKey: "ak", SecretKey: "sk"}, "/api/v1/ops/jobs/", map[string]interface{}{
			"args": "uptime",
		}, nil)
		if err == nil || !strings.Contains(err.Error(), "status=500 body=job submission failed") {
			t.Fatalf("expected status error, got %v", err)
		}
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		runtime := NewJumpServerRuntime(&http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("transport boom")
			}),
		})
		err := runtime.postJSON(context.Background(), jumpServerConfig{BaseURL: "https://jumpserver.example.test", AccessKey: "ak", SecretKey: "sk"}, "/api/v1/ops/jobs/", map[string]interface{}{
			"args": "uptime",
		}, nil)
		if err == nil || !strings.Contains(err.Error(), "transport boom") {
			t.Fatalf("expected transport error, got %v", err)
		}
	})

	t.Run("nil target", func(t *testing.T) {
		t.Parallel()

		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			called = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ignored":true}`))
		}))
		defer srv.Close()

		runtime := NewJumpServerRuntime(srv.Client())
		err := runtime.postJSON(context.Background(), jumpServerConfig{BaseURL: srv.URL, AccessKey: "ak", SecretKey: "sk"}, "/api/v1/ops/jobs/", map[string]interface{}{
			"args": "uptime",
		}, nil)
		if err != nil {
			t.Fatalf("expected nil target success, got %v", err)
		}
		if !called {
			t.Fatalf("expected request to reach server")
		}
	})
}

func TestJumpServerGetJSONHandlesSuccessAndErrorBranches(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodGet || req.URL.Path != "/api/v1/assets/hosts/" || req.URL.RawQuery != "limit=1" {
				t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"value":"ok"}`))
		}))
		defer srv.Close()

		runtime := NewJumpServerRuntime(srv.Client())
		var response struct {
			Value string `json:"value"`
		}
		err := runtime.getJSON(context.Background(), jumpServerConfig{BaseURL: srv.URL, AccessKey: "ak", SecretKey: "sk"}, "/api/v1/assets/hosts/?limit=1", &response)
		if err != nil {
			t.Fatalf("get json: %v", err)
		}
		if response.Value != "ok" {
			t.Fatalf("unexpected response: %+v", response)
		}
	})

	t.Run("status error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))
		defer srv.Close()

		runtime := NewJumpServerRuntime(srv.Client())
		var response struct {
			Value string `json:"value"`
		}
		err := runtime.getJSON(context.Background(), jumpServerConfig{BaseURL: srv.URL, AccessKey: "ak", SecretKey: "sk"}, "/api/v1/assets/hosts/?limit=1", &response)
		if err == nil || !strings.Contains(err.Error(), "status=404 body=not found") {
			t.Fatalf("expected status error, got %v", err)
		}
	})

	t.Run("decode error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte("{"))
		}))
		defer srv.Close()

		runtime := NewJumpServerRuntime(srv.Client())
		var response struct {
			Value string `json:"value"`
		}
		err := runtime.getJSON(context.Background(), jumpServerConfig{BaseURL: srv.URL, AccessKey: "ak", SecretKey: "sk"}, "/api/v1/assets/hosts/?limit=1", &response)
		if err == nil || !strings.Contains(err.Error(), "unexpected EOF") {
			t.Fatalf("expected decode error, got %v", err)
		}
	})

	t.Run("request error", func(t *testing.T) {
		t.Parallel()

		runtime := NewJumpServerRuntime(nil)
		var response struct {
			Value string `json:"value"`
		}
		err := runtime.getJSON(context.Background(), jumpServerConfig{BaseURL: "http://[::1", AccessKey: "ak", SecretKey: "sk"}, "/api/v1/assets/hosts/?limit=1", &response)
		if err == nil {
			t.Fatalf("expected request construction error")
		}
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		runtime := NewJumpServerRuntime(&http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("transport boom")
			}),
		})
		var response struct {
			Value string `json:"value"`
		}
		err := runtime.getJSON(context.Background(), jumpServerConfig{BaseURL: "https://jumpserver.example.test", AccessKey: "ak", SecretKey: "sk"}, "/api/v1/assets/hosts/?limit=1", &response)
		if err == nil || !strings.Contains(err.Error(), "transport boom") {
			t.Fatalf("expected transport error, got %v", err)
		}
	})
}

func TestJumpServerFetchExecutionLogsHandlesResponses(t *testing.T) {
	t.Parallel()

	t.Run("structured payload", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodGet || req.URL.Path != "/api/v1/ops/ansible/job-execution/task-1/log/" {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			_, _ = w.Write([]byte(`{"data":"first line\nsecond line","text":"ignored"}`))
		}))
		defer srv.Close()

		runtime := NewJumpServerRuntime(srv.Client())
		logs, err := runtime.fetchExecutionLogs(context.Background(), jumpServerConfig{BaseURL: srv.URL, AccessKey: "ak", SecretKey: "sk"}, "task-1")
		if err != nil {
			t.Fatalf("fetch logs: %v", err)
		}
		if logs != "first line\nsecond line" {
			t.Fatalf("unexpected logs: %q", logs)
		}
	})

	t.Run("plain text fallback", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte("  plain log line  \n"))
		}))
		defer srv.Close()

		runtime := NewJumpServerRuntime(srv.Client())
		logs, err := runtime.fetchExecutionLogs(context.Background(), jumpServerConfig{BaseURL: srv.URL, AccessKey: "ak", SecretKey: "sk"}, "task-2")
		if err != nil {
			t.Fatalf("fetch logs: %v", err)
		}
		if logs != "plain log line" {
			t.Fatalf("unexpected logs: %q", logs)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		runtime := NewJumpServerRuntime(srv.Client())
		logs, err := runtime.fetchExecutionLogs(context.Background(), jumpServerConfig{BaseURL: srv.URL, AccessKey: "ak", SecretKey: "sk"}, "task-3")
		if err != nil {
			t.Fatalf("fetch logs: %v", err)
		}
		if logs != "" {
			t.Fatalf("expected empty logs, got %q", logs)
		}
	})

	t.Run("status error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("unavailable"))
		}))
		defer srv.Close()

		runtime := NewJumpServerRuntime(srv.Client())
		logs, err := runtime.fetchExecutionLogs(context.Background(), jumpServerConfig{BaseURL: srv.URL, AccessKey: "ak", SecretKey: "sk"}, "task-4")
		if err == nil || !strings.Contains(err.Error(), "status=503 body=unavailable") {
			t.Fatalf("expected status error, got logs=%q err=%v", logs, err)
		}
	})

	t.Run("request error", func(t *testing.T) {
		t.Parallel()

		runtime := NewJumpServerRuntime(nil)
		logs, err := runtime.fetchExecutionLogs(context.Background(), jumpServerConfig{BaseURL: "http://[::1", AccessKey: "ak", SecretKey: "sk"}, "task-5")
		if err == nil {
			t.Fatalf("expected request construction error, got logs=%q", logs)
		}
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		runtime := NewJumpServerRuntime(&http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("transport boom")
			}),
		})
		logs, err := runtime.fetchExecutionLogs(context.Background(), jumpServerConfig{BaseURL: "https://jumpserver.example.test", AccessKey: "ak", SecretKey: "sk"}, "task-6")
		if err == nil || !strings.Contains(err.Error(), "transport boom") {
			t.Fatalf("expected transport error, got logs=%q err=%v", logs, err)
		}
	})
}

func TestJumpServerFirstLineTrimsAndTruncates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  string
		maxLen int
		want   string
	}{
		{
			name:   "trims to first line",
			value:  "  hello world\nsecond line",
			maxLen: 240,
			want:   "hello world",
		},
		{
			name:   "truncates long line",
			value:  "abcdef",
			maxLen: 3,
			want:   "abc",
		},
		{
			name:   "empty after trim",
			value:  "  \n  ",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := firstLine(tc.value, tc.maxLen); got != tc.want {
				t.Fatalf("unexpected first line: got %q want %q", got, tc.want)
			}
		})
	}
}
