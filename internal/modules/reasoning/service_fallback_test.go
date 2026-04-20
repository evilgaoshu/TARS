package reasoning

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
)

type staticPromptProvider struct {
	promptSet *PromptSet
}

func (p staticPromptProvider) CurrentPromptSet() *PromptSet {
	return p.promptSet
}

func TestBuildDiagnosisUsesAssistWhenPrimaryIsNotConfigured(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host != "assist-model.example.test" {
				t.Fatalf("expected assist model host, got %s", req.URL.Host)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"assist-only summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client: client,
		ProviderRegistry: staticProviderRegistry{
			assist: &ModelTarget{
				ProviderID: "assist-openai",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://assist-model.example.test",
				Model:      "gpt-4o-mini",
			},
		},
	})

	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-assist-only",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "assist-only summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("unexpected execution hint: %s", result.ExecutionHint)
	}
}

func TestBuildDiagnosisFallsBackWhenPrimaryAndAssistBothFail(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Host {
			case "primary-model.example.test":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"not-json"}}]}`)),
				}, nil
			case "assist-model.example.test":
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"gateway down"}}`)),
				}, nil
			default:
				t.Fatalf("unexpected host: %s", req.URL.Host)
				return nil, nil
			}
		}),
	}

	svc := NewService(Options{
		Client:                     client,
		LocalCommandFallbackEnable: true,
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "primary-openai",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://primary-model.example.test",
				Model:      "gpt-4o-mini",
			},
			assist: &ModelTarget{
				ProviderID: "assist-openai",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://assist-model.example.test",
				Model:      "gpt-4o-mini-backup",
			},
		},
	})

	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-double-failure",
		Context: map[string]interface{}{
			"alert_name": "HighCPU",
			"host":       "host-1",
			"service":    "api",
			"severity":   "critical",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if !strings.Contains(result.Summary, "HighCPU on host-1") {
		t.Fatalf("expected heuristic summary after model failures, got %q", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected heuristic fallback to avoid generic execution hint, got %s", result.ExecutionHint)
	}
}

func TestBuildExecutionHintForUserRequestCoversRemainingBranches(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		userRequest string
		service     string
		want        string
	}{
		{name: "disk keyword", userRequest: "check disk usage", want: "df -h"},
		{name: "port keyword", userRequest: "which port is listening", want: "ss -lntp"},
		{name: "cpu keyword", userRequest: "show the hottest cpu process", want: "ps aux --sort=-%cpu | head"},
		{name: "stop service", userRequest: "停止 mysql", service: "mysql", want: "systemctl stop mysql"},
		{name: "service status", userRequest: "show service status", service: "nginx", want: "systemctl status nginx --no-pager --lines=20 || true"},
		{name: "explicit chinese status command", userRequest: "执行命令查看 api 状态", service: "api", want: "systemctl status api --no-pager --lines=20 || true"},
		{name: "unsupported request", userRequest: "just summarize the alert", service: "api", want: ""},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := buildExecutionHintForUserRequest(tc.userRequest, tc.service); got != tc.want {
				t.Fatalf("buildExecutionHintForUserRequest(%q, %q) = %q, want %q", tc.userRequest, tc.service, got, tc.want)
			}
		})
	}
}

func TestToolEvidenceMakesExecutionHintRedundant(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		ctx  map[string]interface{}
		want bool
	}{
		{
			name: "observability evidence",
			ctx: map[string]interface{}{
				"observability_query_result": map[string]interface{}{"summary": "root cause isolated"},
			},
			want: true,
		},
		{
			name: "knowledge hits count",
			ctx: map[string]interface{}{
				"knowledge_hits": 2,
			},
			want: true,
		},
		{
			name: "metrics series present",
			ctx: map[string]interface{}{
				"metrics_series": []interface{}{map[string]interface{}{"metric": "node_load1"}},
			},
			want: true,
		},
		{
			name: "completed tool result with output",
			ctx: map[string]interface{}{
				"tool_results": []interface{}{
					map[string]interface{}{
						"status": "completed",
						"output": map[string]interface{}{"results": []interface{}{"evidence"}},
					},
				},
			},
			want: true,
		},
		{
			name: "logs query result present",
			ctx: map[string]interface{}{
				"logs_query_result": map[string]interface{}{
					"result_count": 2,
					"summary":      "matched api errors",
				},
			},
			want: true,
		},
		{
			name: "incomplete tool result",
			ctx: map[string]interface{}{
				"tool_results": []interface{}{
					map[string]interface{}{
						"status": "planned",
						"output": map[string]interface{}{"summary": "not ready"},
					},
				},
			},
			want: false,
		},
		{
			name: "empty context",
			ctx:  map[string]interface{}{},
			want: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := toolEvidenceMakesExecutionHintRedundant(tc.ctx); got != tc.want {
				t.Fatalf("toolEvidenceMakesExecutionHintRedundant(%v) = %v, want %v", tc.ctx, got, tc.want)
			}
		})
	}
}

func TestHasSufficientReasoningEvidenceHandlesNestedShapes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{name: "nil", value: nil, want: false},
		{name: "empty interface slice", value: []interface{}{}, want: false},
		{name: "non-empty map slice", value: []map[string]interface{}{{"summary": "ok"}}, want: true},
		{name: "recognized count field", value: map[string]interface{}{"result_count": 1}, want: true},
		{name: "nested release field", value: map[string]interface{}{"nested": map[string]interface{}{"release": "2026.04.02"}}, want: true},
		{name: "empty nested map", value: map[string]interface{}{"nested": map[string]interface{}{}}, want: false},
		{name: "blank string", value: "   ", want: false},
		{name: "positive float", value: float64(2), want: true},
		{name: "false bool", value: false, want: false},
		{name: "fallback default true", value: struct{}{}, want: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := hasSufficientReasoningEvidence(tc.value); got != tc.want {
				t.Fatalf("hasSufficientReasoningEvidence(%#v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestInterfaceSliceFromContext(t *testing.T) {
	t.Parallel()

	ctx := map[string]interface{}{
		"series": []interface{}{"a", "b"},
		"other":  []string{"c"},
	}

	if got := interfaceSliceFromContext(ctx, "series"); len(got) != 2 {
		t.Fatalf("expected interface slice to be returned, got %+v", got)
	}
	if got := interfaceSliceFromContext(ctx, "other"); got != nil {
		t.Fatalf("expected non-interface slice to be ignored, got %+v", got)
	}
	if got := interfaceSliceFromContext(ctx, "missing"); got != nil {
		t.Fatalf("expected missing key to return nil, got %+v", got)
	}
}

func TestBuildDiagnosisUsesFallbackSummaryWhenModelSummaryIsBlank(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"   \",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-blank-summary",
		Context: map[string]interface{}{
			"host":         "host-1",
			"user_request": "请帮我检查负载",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if !strings.Contains(result.Summary, "已分析请求：请帮我检查负载") {
		t.Fatalf("expected fallback summary, got %q", result.Summary)
	}
}

func TestBuildDiagnosisSuppressesExecutionHintWhenToolEvidenceAlreadyExists(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"model summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-existing-evidence",
		Context: map[string]interface{}{
			"alert_name":     "HighCPU",
			"metrics_series": []interface{}{map[string]interface{}{"metric": "node_load1"}},
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected execution hint to be suppressed when tool evidence already exists, got %q", result.ExecutionHint)
	}
}

func TestBuildDiagnosisSuppressesExecutionHintWhenLogsEvidenceExists(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"model summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-logs-evidence",
		Context: map[string]interface{}{
			"user_request": "最近 api 报错",
			"logs_query_result": map[string]interface{}{
				"result_count": 3,
				"summary":      "matched api errors",
			},
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected generic execution hint to be suppressed, got %q", result.ExecutionHint)
	}
}

func TestBuildDiagnosisSuppressesExecutionHintWhenObservabilityEvidenceExists(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"model summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-observe-evidence",
		Context: map[string]interface{}{
			"user_request": "trace 一下 api latency 根因",
			"observability_query_result": map[string]interface{}{
				"result_count": 2,
				"summary":      "latency spike isolated",
			},
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected generic execution hint to be suppressed, got %q", result.ExecutionHint)
	}
}

func TestBuildDiagnosisKeepsExplicitEnglishStatusCommandEvenWithExistingEvidence(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"model summary\",\"execution_hint\":\"systemctl status api --no-pager --lines=20 || true\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-explicit-status",
		Context: map[string]interface{}{
			"service":        "api",
			"user_request":   "run status command for api service",
			"metrics_series": []interface{}{map[string]interface{}{"metric": "node_load1"}},
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.ExecutionHint != "systemctl status api --no-pager --lines=20 || true" {
		t.Fatalf("expected explicit status command to survive, got %q", result.ExecutionHint)
	}
}

func TestDetectSensitiveValuesUsesAssistTargetOverride(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
	auditLogger := &captureAuditLogger{}
	var calledHost string
	var calledModel string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calledHost = req.URL.Host
			if req.Header.Get("Authorization") != "Bearer assist-secret" {
				t.Fatalf("expected assist api key header, got %q", req.Header.Get("Authorization"))
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal detector payload: %v", err)
			}
			calledModel, _ = payload["model"].(string)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"secrets\":[\"abc123\"],\"hosts\":[\"phoenix-cluster\"],\"ips\":[\"10.0.0.8\"],\"paths\":[\"/srv/app/config.yaml\"]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		Metrics: registry,
		Audit:   auditLogger,
		ProviderRegistry: staticProviderRegistry{
			assist: &ModelTarget{
				ProviderID: "assist-openai",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://assist-detector.example.test",
				APIKey:     "assist-secret",
				Model:      "assist-model",
			},
		},
	})

	cfg := DefaultDesensitizationConfig()
	cfg.LocalLLMAssist.Enabled = true
	cfg.LocalLLMAssist.Provider = ModelProtocolGemini
	cfg.LocalLLMAssist.BaseURL = "https://ignored.example.test"
	cfg.LocalLLMAssist.Model = "ignored-model"
	cfg.LocalLLMAssist.Mode = ""

	detections := svc.detectSensitiveValues(context.Background(), "ses-detect", map[string]interface{}{
		"user_request": "connect to phoenix-cluster at 10.0.0.8 using /srv/app/config.yaml token=abc123",
	}, &cfg)
	if detections == nil {
		t.Fatalf("expected detections from local llm assist")
	}
	if calledHost != "assist-detector.example.test" || calledModel != "assist-model" {
		t.Fatalf("expected assist target override, host=%q model=%q", calledHost, calledModel)
	}
	if len(auditLogger.entries) != 2 {
		t.Fatalf("expected detector request/response audit entries, got %+v", auditLogger.entries)
	}

	var output strings.Builder
	if err := registry.WritePrometheus(&output); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	if !strings.Contains(output.String(), `tars_external_provider_requests_total{operation="detect_sensitive_values",provider="local_llm_desensitizer",result="success"} 1`) {
		t.Fatalf("expected detector success metric, got:\n%s", output.String())
	}
}

func TestLocalLLMDetectorValidationAndErrorMetrics(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
	detector := newLocalLLMDetector(nil, registry, nil, &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"gateway down"}`)),
			}, nil
		}),
	})

	if _, err := detector.DetectSensitiveValues(context.Background(), "ses-missing", map[string]interface{}{}, LocalLLMAssistConfig{}, ""); err == nil {
		t.Fatalf("expected missing base url/model validation error")
	}
	if _, err := detector.DetectSensitiveValues(context.Background(), "ses-mode", map[string]interface{}{}, LocalLLMAssistConfig{
		Provider: ModelProtocolOpenAICompatible,
		BaseURL:  "https://local-detector.example.test",
		Model:    "qwen",
		Mode:     "rewrite",
	}, ""); err == nil {
		t.Fatalf("expected unsupported mode validation error")
	}
	if _, err := detector.DetectSensitiveValues(context.Background(), "ses-error", map[string]interface{}{"host": "node-a"}, LocalLLMAssistConfig{
		Provider: ModelProtocolOpenAICompatible,
		BaseURL:  "https://local-detector.example.test",
		Model:    "qwen",
	}, ""); err == nil {
		t.Fatalf("expected model error from detector request")
	}

	var output strings.Builder
	if err := registry.WritePrometheus(&output); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	if !strings.Contains(output.String(), `tars_external_provider_requests_total{operation="detect_sensitive_values",provider="local_llm_desensitizer",result="error"} 1`) {
		t.Fatalf("expected detector error metric, got:\n%s", output.String())
	}
}

func TestPromptAndServiceHelperBranches(t *testing.T) {
	t.Parallel()

	prompts := &PromptSet{UserPromptTemplate: "{{"}
	if _, err := prompts.RenderUserPrompt("ses-invalid-template", map[string]interface{}{}); err == nil {
		t.Fatalf("expected invalid template to fail")
	}
	if _, err := DefaultPromptSet().RenderUserPrompt("ses-bad-json", map[string]interface{}{"bad": make(chan int)}); err == nil {
		t.Fatalf("expected non-json-marshallable context to fail")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.yaml")
	if err := writePromptFileAtomically(path, "reasoning:\n  system_prompt: test\n"); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}
	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("read prompt file: %v", err)
	}
	if err := writePromptFileAtomically(filepath.Join(dir, "missing", "prompts.yaml"), "x"); err == nil {
		t.Fatalf("expected write prompt helper to fail for missing directory")
	}

	if got := intFromContext(map[string]interface{}{"count": int64(3)}, "count"); got != 3 {
		t.Fatalf("expected int64 coercion, got %d", got)
	}
	if got := intFromContext(map[string]interface{}{"count": int(2)}, "count"); got != 2 {
		t.Fatalf("expected int coercion, got %d", got)
	}
	if got := intFromContext(map[string]interface{}{"count": float64(4)}, "count"); got != 4 {
		t.Fatalf("expected float64 coercion, got %d", got)
	}
	if got := intFromContext(map[string]interface{}{"count": "3"}, "count"); got != 0 {
		t.Fatalf("expected unsupported count type to return 0, got %d", got)
	}
	if got := interfaceStringFromAny(123); got != "" {
		t.Fatalf("expected unsupported type to return empty string, got %q", got)
	}

	if fallbackLogger(nil) == nil {
		t.Fatalf("expected fallback logger to always return a logger")
	}
	var nilService *Service
	if got := nilService.providerSnapshot(); !reflect.DeepEqual(got, ProvidersSnapshot{}) {
		t.Fatalf("expected nil service provider snapshot to be zero, got %+v", got)
	}
	provider := selectPromptProvider(nil, nil)
	if provider == nil || provider.CurrentPromptSet() == nil {
		t.Fatalf("expected prompt provider selection to fall back to defaults")
	}
}

func TestPromptProviderAndAssistFallbackHelpers(t *testing.T) {
	t.Parallel()

	detector := newLocalLLMDetector(nil, nil, nil, nil)
	if detector.client == nil || detector.logger == nil {
		t.Fatalf("expected detector helper to fill default client and logger")
	}

	customPrompts := NewPromptSet("custom system", "session={{ .SessionID }}")
	provider := selectPromptProvider(staticPromptProvider{promptSet: customPrompts}, nil)
	if got := provider.CurrentPromptSet(); got.SystemPrompt != "custom system" {
		t.Fatalf("expected explicit prompt provider to win, got %+v", got)
	}

	svc := &Service{promptProvider: staticPromptProvider{promptSet: customPrompts}}
	if got := svc.currentPromptSet(); got.SystemPrompt != "custom system" {
		t.Fatalf("expected service currentPromptSet to use provider, got %+v", got)
	}

	primary := modelRuntime{Protocol: ModelProtocolOpenAICompatible, BaseURL: "https://primary.example.test", Model: "gpt-4o-mini"}
	if canUseAssistFallback(primary, nil) {
		t.Fatalf("expected nil assist fallback to be disabled")
	}
	if canUseAssistFallback(primary, &modelRuntime{Protocol: ModelProtocolOpenAICompatible, BaseURL: "https://primary.example.test", Model: "gpt-4o-mini"}) {
		t.Fatalf("expected identical runtime not to be used as assist fallback")
	}
	if !canUseAssistFallback(primary, &modelRuntime{Protocol: ModelProtocolOpenAICompatible, BaseURL: "https://assist.example.test", Model: "gpt-4o-mini-backup"}) {
		t.Fatalf("expected distinct assist runtime to be eligible")
	}
}

func TestBuildDiagnosisRecordsStubErrorAndSuccessMetrics(t *testing.T) {
	t.Parallel()

	stubRegistry := foundationmetrics.New()
	stubSvc := NewService(Options{Metrics: stubRegistry})
	if _, err := stubSvc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-build-stub",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	}); err != nil {
		t.Fatalf("build diagnosis stub: %v", err)
	}
	var stubOutput strings.Builder
	if err := stubRegistry.WritePrometheus(&stubOutput); err != nil {
		t.Fatalf("write stub metrics: %v", err)
	}
	if !strings.Contains(stubOutput.String(), `tars_external_provider_requests_total{operation="chat_completions",provider="model",result="stub"} 1`) {
		t.Fatalf("expected stub metric, got:\n%s", stubOutput.String())
	}

	errorRegistry := foundationmetrics.New()
	errorSvc := NewService(Options{
		Metrics: errorRegistry,
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"gateway down"}}`)),
				}, nil
			}),
		},
		BaseURL: "https://build-error.example.test",
		Model:   "gpt-4o-mini",
	})
	if _, err := errorSvc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-build-error",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	}); err != nil {
		t.Fatalf("build diagnosis error: %v", err)
	}
	var errorOutput strings.Builder
	if err := errorRegistry.WritePrometheus(&errorOutput); err != nil {
		t.Fatalf("write error metrics: %v", err)
	}
	if !strings.Contains(errorOutput.String(), `tars_external_provider_requests_total{operation="chat_completions",provider="model",result="error"} 1`) {
		t.Fatalf("expected error metric, got:\n%s", errorOutput.String())
	}
}

func TestLocalAssistAuditAndCurrentPromptHelpers(t *testing.T) {
	t.Parallel()

	auditLogger := &captureAuditLogger{}
	detector := &localLLMDetector{audit: auditLogger}
	detector.auditResponse(context.Background(), "", LocalLLMAssistConfig{Model: "ignored"}, &SensitiveDetections{}, "")
	if len(auditLogger.entries) != 0 {
		t.Fatalf("expected blank session audit response to be ignored, got %+v", auditLogger.entries)
	}

	detector.auditResponse(context.Background(), "ses-audit-response", LocalLLMAssistConfig{Model: "qwen"}, &SensitiveDetections{
		Secrets: []string{"token"},
	}, "")
	if len(auditLogger.entries) != 1 {
		t.Fatalf("expected one audit response entry, got %+v", auditLogger.entries)
	}
	entry := auditLogger.entries[0]
	if entry.Action != "local_llm_desensitization_detect_result" {
		t.Fatalf("unexpected audit action: %+v", entry)
	}
	if provider, _ := entry.Metadata["provider"].(string); provider != ModelProtocolOpenAICompatible {
		t.Fatalf("expected provider fallback, got %+v", entry.Metadata)
	}
	if mode, _ := entry.Metadata["mode"].(string); mode != "detect_only" {
		t.Fatalf("expected mode fallback, got %+v", entry.Metadata)
	}
	if fallbackLocalAssistString("", "default") != "default" || fallbackLocalAssistString("value", "default") != "value" {
		t.Fatalf("unexpected fallbackLocalAssistString behavior")
	}

	if got := fallbackLogger(slog.Default()); got == nil {
		t.Fatalf("expected explicit logger to survive fallback")
	}
	var nilService *Service
	if got := nilService.currentPromptSet(); got == nil || got.SystemPrompt == "" {
		t.Fatalf("expected nil service currentPromptSet to return defaults, got %+v", got)
	}
	svc := &Service{}
	if got := svc.currentPromptSet(); got == nil || got.SystemPrompt == "" {
		t.Fatalf("expected empty service currentPromptSet to return defaults, got %+v", got)
	}
}

func TestBuildFromModelErrorBranches(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Prompts: NewPromptSet("system", "{{"),
	})
	if _, err := svc.buildFromModel(context.Background(),
		contracts.DiagnosisInput{SessionID: "ses-build-prompt-error", Context: map[string]interface{}{}},
		contracts.DiagnosisInput{SessionID: "ses-build-prompt-error", Context: map[string]interface{}{}},
		ModelProtocolOpenAICompatible,
		"https://model.example.test",
		"",
		"gpt-4o-mini",
	); err == nil {
		t.Fatalf("expected invalid prompt template to fail")
	}

	transportErrSvc := NewService(Options{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, io.ErrUnexpectedEOF
			}),
		},
	})
	if _, err := transportErrSvc.buildFromModel(context.Background(),
		contracts.DiagnosisInput{SessionID: "ses-build-transport-error", Context: map[string]interface{}{}},
		contracts.DiagnosisInput{SessionID: "ses-build-transport-error", Context: map[string]interface{}{}},
		ModelProtocolOpenAICompatible,
		"https://model.example.test",
		"",
		"gpt-4o-mini",
	); err == nil {
		t.Fatalf("expected transport error to fail")
	}

	invalidJSONSvc := NewService(Options{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"not-json"}}]}`)),
				}, nil
			}),
		},
	})
	if _, err := invalidJSONSvc.buildFromModel(context.Background(),
		contracts.DiagnosisInput{SessionID: "ses-build-decode-error", Context: map[string]interface{}{}},
		contracts.DiagnosisInput{SessionID: "ses-build-decode-error", Context: map[string]interface{}{}},
		ModelProtocolOpenAICompatible,
		"https://model.example.test",
		"",
		"gpt-4o-mini",
	); err == nil {
		t.Fatalf("expected invalid model json to fail")
	}

	if _, err := invalidJSONSvc.buildFromModel(context.Background(),
		contracts.DiagnosisInput{SessionID: "ses-build-url-error", Context: map[string]interface{}{}},
		contracts.DiagnosisInput{SessionID: "ses-build-url-error", Context: map[string]interface{}{}},
		ModelProtocolOpenAICompatible,
		"://bad-url",
		"",
		"gpt-4o-mini",
	); err == nil {
		t.Fatalf("expected invalid endpoint to fail request creation")
	}
}

func TestBuildExecutionHintHelper(t *testing.T) {
	t.Parallel()

	// no service → just hostname && uptime
	hint := buildExecutionHint("")
	if hint != "hostname && uptime" {
		t.Fatalf("expected plain hint, got %q", hint)
	}

	// with service → appends systemctl status
	hintWithSvc := buildExecutionHint("api")
	if !strings.Contains(hintWithSvc, "systemctl status api") {
		t.Fatalf("expected systemctl status api in hint, got %q", hintWithSvc)
	}
}

func TestBuildDirectExecutionHintForUserRequestBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		req     string
		svc     string
		want    string
		desc    string
	}{
		{"public ip", "", "curl -fsS https://api.ipify.org && echo", "public ip"},
		{"egress ip", "", "curl -fsS https://api.ipify.org && echo", "egress ip"},
		{"出口ip", "", "curl -fsS https://api.ipify.org && echo", "中文出口ip"},
		{"restart api", "api", "systemctl restart api", "restart"},
		{"重启 api", "api", "systemctl restart api", "中文重启"},
		{"stop api", "api", "systemctl stop api", "stop"},
		{"停止 api", "api", "systemctl stop api", "中文停止"},
		{"run status api", "api", "systemctl status api --no-pager --lines=20 || true", "run status"},
		{"check load", "", "", "no match → empty"},
	}
	for _, tc := range cases {
		got := buildDirectExecutionHintForUserRequest(tc.req, tc.svc)
		if got != tc.want {
			t.Fatalf("[%s] buildDirectExecutionHintForUserRequest(%q, %q) = %q, want %q", tc.desc, tc.req, tc.svc, got, tc.want)
		}
	}
}
