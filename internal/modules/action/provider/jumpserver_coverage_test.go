package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"tars/internal/contracts"
)

func TestJumpServerExecuteHappyPath(t *testing.T) {
	t.Parallel()

	var submittedBody string
	var pollCalls int
	runtime := NewJumpServerRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/assets/hosts/":
				if got := req.URL.RawQuery; got != "address=192.168.3.106" {
					t.Fatalf("unexpected asset lookup query %q", got)
				}
				return jsonResponse(http.StatusOK, `{"count":1,"results":[{"id":"asset-1","address":"192.168.3.106","name":"node-1"}]}`), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/v1/ops/jobs/":
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				submittedBody = string(body)
				return jsonResponse(http.StatusCreated, `{"id":"job-1","task_id":"task-1"}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/job-execution/task-detail/task-1/":
				pollCalls++
				return jsonResponse(http.StatusOK, `{"status":{"value":"success"},"is_finished":true,"is_success":true,"summary":{"phase":"done"}}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/job-executions/task-1/":
				return jsonResponse(http.StatusOK, `{"id":"task-1","result":{"stdout":"line1\nline2"},"summary":{"z":"last","a":"first"}}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/ansible/job-execution/task-1/log/":
				return jsonResponse(http.StatusOK, `{"content":"log line one\nlog line two"}`), nil
			default:
				return jsonResponse(http.StatusNotFound, `{"detail":"not found"}`), nil
			}
		}),
	})

	manifest := jumpServerManifest()
	manifest.Config.Values["timeout_seconds"] = "5"
	manifest.Config.Values["poll_interval_seconds"] = "1"
	manifest.Config.Values["poll_timeout_seconds"] = "3"

	result, err := runtime.Execute(context.Background(), manifest, contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "192.168.3.106",
		Command:     "whoami",
	})
	if err != nil {
		t.Fatalf("execute jumpserver job: %v", err)
	}
	if result.Status != "completed" || result.ExitCode != 0 {
		t.Fatalf("unexpected execution result: %+v", result)
	}
	if result.ExecutionMode != "jumpserver_job" {
		t.Fatalf("expected default execution mode, got %+v", result)
	}
	if pollCalls != 1 {
		t.Fatalf("expected one poll call, got %d", pollCalls)
	}
	if !strings.Contains(submittedBody, `"args":"whoami"`) || !strings.Contains(submittedBody, `"assets":["asset-1"]`) {
		t.Fatalf("unexpected job payload: %s", submittedBody)
	}
	if !strings.Contains(result.Output, "[logs]\nlog line one\nlog line two") {
		t.Fatalf("expected logs section, got:\n%s", result.Output)
	}
	if !strings.Contains(result.Output, "[result]\n{\n  \"stdout\": \"line1\\nline2\"\n}") {
		t.Fatalf("expected result section, got:\n%s", result.Output)
	}
	if !strings.Contains(result.Output, "[summary]\na=first; z=last") {
		t.Fatalf("expected summary section, got:\n%s", result.Output)
	}
	if result.OutputPreview != "a=first; z=last" {
		t.Fatalf("unexpected output preview: %q", result.OutputPreview)
	}
}

func TestJumpServerExecuteContinuesWhenDetailAndLogFetchFail(t *testing.T) {
	t.Parallel()

	runtime := NewJumpServerRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/assets/hosts/":
				return jsonResponse(http.StatusOK, `{"results":[{"id":"asset-1","address":"192.168.3.106","name":"node-1"}]}`), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/v1/ops/jobs/":
				return jsonResponse(http.StatusCreated, `{"id":"job-1","task_id":"task-1"}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/job-execution/task-detail/task-1/":
				return jsonResponse(http.StatusOK, `{"status":{"value":"success"},"is_finished":true,"is_success":true,"summary":{}}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/job-executions/task-1/":
				return textResponse(http.StatusInternalServerError, "detail unavailable"), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/ansible/job-execution/task-1/log/":
				return textResponse(http.StatusBadGateway, "log unavailable"), nil
			default:
				return jsonResponse(http.StatusNotFound, `{"detail":"not found"}`), nil
			}
		}),
	})

	result, err := runtime.Execute(context.Background(), jumpServerManifest(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-2",
		SessionID:   "ses-2",
		TargetHost:  "192.168.3.106",
		Command:     "whoami",
	})
	if err != nil {
		t.Fatalf("execute jumpserver job: %v", err)
	}
	if result.Status != "completed" || result.ExitCode != 0 {
		t.Fatalf("unexpected execution result: %+v", result)
	}
	if !strings.Contains(result.Output, "[warn] fetch execution detail failed: status=500 body=detail unavailable") {
		t.Fatalf("expected execution detail warning, got:\n%s", result.Output)
	}
	if !strings.Contains(result.Output, "[warn] fetch execution log failed: status=502 body=log unavailable") {
		t.Fatalf("expected log warning, got:\n%s", result.Output)
	}
	for _, token := range []string{"host=192.168.3.106", "asset_id=asset-1", "task_id=task-1", "job_id=job-1"} {
		if !strings.Contains(result.OutputPreview, token) {
			t.Fatalf("expected output preview to contain %q, got %q", token, result.OutputPreview)
		}
	}
}

func TestJumpServerExecuteReturnsTimeoutStatus(t *testing.T) {
	t.Parallel()

	runtime := NewJumpServerRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/assets/hosts/":
				return jsonResponse(http.StatusOK, `{"results":[{"id":"asset-2","address":"192.168.3.107","name":"node-timeout"}]}`), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/v1/ops/jobs/":
				return jsonResponse(http.StatusCreated, `{"id":"job-timeout","task_id":"task-timeout"}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/job-execution/task-detail/task-timeout/":
				return jsonResponse(http.StatusOK, `{"status":{"value":"timeout"},"is_finished":true,"is_success":false,"summary":{"reason":"command timeout"}}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/job-executions/task-timeout/":
				return jsonResponse(http.StatusOK, `{"id":"task-timeout","result":"command exceeded timeout","summary":{}}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/ansible/job-execution/task-timeout/log/":
				return textResponse(http.StatusOK, "command exceeded timeout\n"), nil
			default:
				return jsonResponse(http.StatusNotFound, `{"detail":"not found"}`), nil
			}
		}),
	})

	result, err := runtime.Execute(context.Background(), jumpServerManifest(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-timeout",
		SessionID:   "ses-timeout",
		TargetHost:  "192.168.3.107",
		Command:     "sleep 999",
	})
	if err != nil {
		t.Fatalf("execute jumpserver timeout job: %v", err)
	}
	if result.Status != "timeout" || result.ExitCode != 124 {
		t.Fatalf("unexpected execution result: %+v", result)
	}
	if result.OutputPreview != "reason=command timeout" {
		t.Fatalf("unexpected output preview: %q", result.OutputPreview)
	}
	if !strings.Contains(result.Output, "[result]\ncommand exceeded timeout") {
		t.Fatalf("expected timeout result section, got:\n%s", result.Output)
	}
}

func TestJumpServerExecuteFailsWhenSubmissionReturnsNoTaskID(t *testing.T) {
	t.Parallel()

	runtime := NewJumpServerRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/assets/hosts/":
				return jsonResponse(http.StatusOK, `{"results":[{"id":"asset-3","address":"192.168.3.108","name":"node-no-task"}]}`), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/v1/ops/jobs/":
				return jsonResponse(http.StatusCreated, `{}`), nil
			default:
				return jsonResponse(http.StatusNotFound, `{"detail":"not found"}`), nil
			}
		}),
	})

	_, err := runtime.Execute(context.Background(), jumpServerManifest(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-no-task",
		SessionID:   "ses-no-task",
		TargetHost:  "192.168.3.108",
		Command:     "uptime",
	})
	if err == nil || err.Error() != "jumpserver job submission did not return task id" {
		t.Fatalf("expected missing task id error, got %v", err)
	}
}

func TestJumpServerCheckHealth(t *testing.T) {
	t.Parallel()

	t.Run("healthy", func(t *testing.T) {
		t.Parallel()

		runtime := NewJumpServerRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/api/v1/assets/hosts/" || req.URL.RawQuery != "limit=1" {
					t.Fatalf("unexpected health probe request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
				}
				return jsonResponse(http.StatusOK, `{"count":0,"results":[{"id":"asset-1","address":"127.0.0.1","name":"local"}]}`), nil
			}),
		})

		status, message, err := runtime.CheckHealth(context.Background(), jumpServerManifest())
		if err != nil {
			t.Fatalf("check health: %v", err)
		}
		if status != "healthy" || message != "jumpserver API probe succeeded" {
			t.Fatalf("unexpected health result: %s %s", status, message)
		}
	})

	t.Run("config error", func(t *testing.T) {
		t.Parallel()

		manifest := jumpServerManifest()
		delete(manifest.Config.Values, "base_url")

		status, message, err := NewJumpServerRuntime(nil).CheckHealth(context.Background(), manifest)
		if err == nil {
			t.Fatalf("expected configuration error")
		}
		if status != "unhealthy" || message != "jumpserver connector is not fully configured: missing required fields: base_url" {
			t.Fatalf("unexpected health result: %s %s", status, message)
		}
	})

	t.Run("probe error", func(t *testing.T) {
		t.Parallel()

		runtime := NewJumpServerRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return textResponse(http.StatusServiceUnavailable, "maintenance"), nil
			}),
		})

		status, message, err := runtime.CheckHealth(context.Background(), jumpServerManifest())
		if err == nil {
			t.Fatalf("expected probe error")
		}
		if status != "unhealthy" || !strings.HasPrefix(message, "jumpserver API error: ") {
			t.Fatalf("unexpected health result: %s %s", status, message)
		}
	})
}

func TestJumpServerLookupAssetBranches(t *testing.T) {
	t.Parallel()

	t.Run("empty target host", func(t *testing.T) {
		t.Parallel()

		_, err := NewJumpServerRuntime(nil).lookupAsset(context.Background(), jumpServerConfig{}, "")
		if err == nil || err.Error() != "target host is required" {
			t.Fatalf("expected target host error, got %v", err)
		}
	})

	t.Run("fallbacks from exact lookup to search and returns single asset", func(t *testing.T) {
		t.Parallel()

		var requests []string
		runtime := NewJumpServerRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.URL.RequestURI())
				switch req.URL.RequestURI() {
				case "/api/v1/assets/hosts/?address=host-1":
					return jsonResponse(http.StatusOK, `{"count":0,"results":[]}`), nil
				case "/api/v1/assets/hosts/?search=host-1":
					return jsonResponse(http.StatusOK, `{"results":[{"id":"asset-1","address":"10.0.0.1","name":"node-1"}]}`), nil
				default:
					return jsonResponse(http.StatusNotFound, `{"detail":"not found"}`), nil
				}
			}),
		})

		asset, err := runtime.lookupAsset(context.Background(), jumpServerConfig{
			BaseURL:    "https://jumpserver.example.test",
			AccessKey:  "ak",
			SecretKey:  "sk",
			HostField:  "address",
			HostSearch: "exact",
		}, "host-1")
		if err != nil {
			t.Fatalf("lookup asset: %v", err)
		}
		if asset.ID != "asset-1" {
			t.Fatalf("unexpected asset: %+v", asset)
		}
		if len(requests) != 2 {
			t.Fatalf("expected fallback search request, got %v", requests)
		}
	})

	t.Run("search mode uses search query directly", func(t *testing.T) {
		t.Parallel()

		var requested string
		runtime := NewJumpServerRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requested = req.URL.RequestURI()
				return jsonResponse(http.StatusOK, `{"results":[{"id":"asset-2","address":"host-2","name":"node-2"}]}`), nil
			}),
		})

		asset, err := runtime.lookupAsset(context.Background(), jumpServerConfig{
			BaseURL:    "https://jumpserver.example.test",
			AccessKey:  "ak",
			SecretKey:  "sk",
			HostField:  "address",
			HostSearch: "search",
		}, "host-2")
		if err != nil {
			t.Fatalf("lookup asset: %v", err)
		}
		if requested != "/api/v1/assets/hosts/?search=host-2" {
			t.Fatalf("unexpected request uri: %s", requested)
		}
		if asset.ID != "asset-2" {
			t.Fatalf("unexpected asset: %+v", asset)
		}
	})

	t.Run("returns not found when multiple results do not match", func(t *testing.T) {
		t.Parallel()

		runtime := NewJumpServerRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{"results":[{"id":"asset-1","address":"10.0.0.1","name":"node-1"},{"id":"asset-2","address":"10.0.0.2","name":"node-2"}]}`), nil
			}),
		})

		_, err := runtime.lookupAsset(context.Background(), jumpServerConfig{
			BaseURL:    "https://jumpserver.example.test",
			AccessKey:  "ak",
			SecretKey:  "sk",
			HostField:  "address",
			HostSearch: "exact",
		}, "host-3")
		if err == nil || !strings.Contains(err.Error(), "jumpserver asset not found for host host-3") {
			t.Fatalf("expected not found error, got %v", err)
		}
	})

	t.Run("propagates lookup failure from the api", func(t *testing.T) {
		t.Parallel()

		runtime := NewJumpServerRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return textResponse(http.StatusInternalServerError, "boom"), nil
			}),
		})

		_, err := runtime.lookupAsset(context.Background(), jumpServerConfig{
			BaseURL:    "https://jumpserver.example.test",
			AccessKey:  "ak",
			SecretKey:  "sk",
			HostField:  "address",
			HostSearch: "exact",
		}, "host-4")
		if err == nil || !strings.Contains(err.Error(), "jumpserver asset lookup failed:") {
			t.Fatalf("expected lookup failure, got %v", err)
		}
	})
}

func TestJumpServerTaskDone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{name: "success", status: "success", want: true},
		{name: "failed", status: "failed", want: true},
		{name: "timeout", status: "timeout", want: true},
		{name: "canceled", status: "canceled", want: true},
		{name: "cancelled", status: "cancelled", want: true},
		{name: "error", status: "error", want: true},
		{name: "running", status: "running", want: false},
		{name: "empty", status: "", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := jumpServerTaskDone(tc.status); got != tc.want {
				t.Fatalf("jumpServerTaskDone(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

func TestJumpServerPollTaskTimesOut(t *testing.T) {
	runtime := NewJumpServerRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet || req.URL.Path != "/api/v1/ops/job-execution/task-detail/task-poll-timeout/" {
				t.Fatalf("unexpected poll request: %s %s", req.Method, req.URL.Path)
			}
			return jsonResponse(http.StatusOK, `{"status":{"value":"running"},"is_finished":false,"is_success":false,"summary":{}}`), nil
		}),
	})

	_, err := runtime.pollTask(context.Background(), jumpServerConfig{
		BaseURL:      "https://jumpserver.example.test",
		AccessKey:    "ak",
		SecretKey:    "sk",
		OrgID:        jumpServerDefaultOrgID,
		PollInterval: 10 * time.Millisecond,
		PollTimeout:  75 * time.Millisecond,
	}, "task-poll-timeout")
	if err == nil || !strings.Contains(err.Error(), "jumpserver task polling timed out:") {
		t.Fatalf("expected poll timeout error, got %v", err)
	}
}

func TestMapJumpServerExecutionStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		detail *jumpServerTaskDetailResponse
		status string
		exit   int
	}{
		{name: "nil detail", detail: nil, status: "failed", exit: 1},
		{name: "success status", detail: taskDetail("success", false, false), status: "completed", exit: 0},
		{name: "timeout status", detail: taskDetail("timeout", false, false), status: "timeout", exit: 124},
		{name: "canceled status", detail: taskDetail("canceled", false, false), status: "failed", exit: 130},
		{name: "cancelled status", detail: taskDetail("cancelled", false, false), status: "failed", exit: 130},
		{name: "implicit success", detail: taskDetail("running", false, true), status: "completed", exit: 0},
		{name: "implicit failure", detail: taskDetail("running", false, false), status: "failed", exit: 1},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, exitCode := mapJumpServerExecutionStatus(tc.detail)
			if status != tc.status || exitCode != tc.exit {
				t.Fatalf("mapJumpServerExecutionStatus(...) = %q, %d; want %q, %d", status, exitCode, tc.status, tc.exit)
			}
		})
	}
}

func TestStringifyJumpServerPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value interface{}
		want  string
		check func(string) bool
	}{
		{name: "nil", value: nil, want: ""},
		{name: "string", value: "  hello \n world  ", want: "hello\nworld"},
		{name: "stringer", value: jumpserverStringer("  active \n now "), want: "active\nnow"},
		{name: "json", value: jumpserverPayload{Name: "alice", Count: 3}, want: "{\n  \"name\": \"alice\",\n  \"count\": 3\n}"},
		{name: "marshal error", value: map[string]interface{}{"fn": func() {}}, check: func(got string) bool { return strings.Contains(got, "fn") }},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := stringifyJumpServerPayload(tc.value)
			if tc.want != "" && got != tc.want {
				t.Fatalf("stringifyJumpServerPayload(...) = %q, want %q", got, tc.want)
			}
			if tc.check != nil && !tc.check(got) {
				t.Fatalf("stringifyJumpServerPayload(...) = %q, failed check", got)
			}
		})
	}
}

func TestParsePositiveInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		fallback int
		want     int
	}{
		{name: "blank", raw: " ", fallback: 9, want: 9},
		{name: "invalid", raw: "abc", fallback: 9, want: 9},
		{name: "zero", raw: "0", fallback: 9, want: 9},
		{name: "negative", raw: "-3", fallback: 9, want: 9},
		{name: "positive", raw: "12", fallback: 9, want: 12},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := parsePositiveInt(tc.raw, tc.fallback); got != tc.want {
				t.Fatalf("parsePositiveInt(%q, %d) = %d, want %d", tc.raw, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestJumpServerHostListResponseUnmarshalJSON(t *testing.T) {
	t.Parallel()

	t.Run("null", func(t *testing.T) {
		t.Parallel()

		var resp jumpServerHostListResponse
		if err := resp.UnmarshalJSON([]byte("null")); err != nil {
			t.Fatalf("unmarshal null: %v", err)
		}
		if resp.Count != 0 || len(resp.Results) != 0 {
			t.Fatalf("expected zero value, got %+v", resp)
		}
	})

	t.Run("array", func(t *testing.T) {
		t.Parallel()

		var resp jumpServerHostListResponse
		if err := resp.UnmarshalJSON([]byte(`[{"id":"asset-1","address":"10.0.0.1","name":"node-1"}]`)); err != nil {
			t.Fatalf("unmarshal array: %v", err)
		}
		if resp.Count != 1 || len(resp.Results) != 1 {
			t.Fatalf("expected one result, got %+v", resp)
		}
	})

	t.Run("object with inferred count", func(t *testing.T) {
		t.Parallel()

		var resp jumpServerHostListResponse
		if err := resp.UnmarshalJSON([]byte(`{"count":0,"results":[{"id":"asset-2","address":"10.0.0.2","name":"node-2"}]}`)); err != nil {
			t.Fatalf("unmarshal object: %v", err)
		}
		if resp.Count != 1 || len(resp.Results) != 1 {
			t.Fatalf("expected inferred count, got %+v", resp)
		}
	})
}

func taskDetail(status string, finished bool, success bool) *jumpServerTaskDetailResponse {
	detail := &jumpServerTaskDetailResponse{}
	detail.Status.Value = status
	detail.IsFinished = finished
	detail.IsSuccess = success
	return detail
}

type jumpserverStringer string

func (s jumpserverStringer) String() string {
	return strings.TrimSpace(string(s))
}

type jumpserverPayload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
