package reasoning

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"tars/internal/contracts"
	"tars/internal/foundation/audit"
	foundationmetrics "tars/internal/foundation/metrics"
	"tars/internal/foundation/secrets"
)

type staticDesensitizationProvider struct {
	cfg DesensitizationConfig
}

func (p staticDesensitizationProvider) CurrentDesensitizationConfig() *DesensitizationConfig {
	cfg := normalizeDesensitizationConfig(p.cfg)
	return &cfg
}

func TestBuildDiagnosisFallsBackWithoutModelConfig(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-1",
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
	if !strings.Contains(result.Summary, "HighCPU") {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected no execution hint without local fallback, got %s", result.ExecutionHint)
	}
}

func TestBuildDiagnosisChatRequestSuggestsReadOnlyLoadCommand(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{LocalCommandFallbackEnable: true})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-chat-1",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"severity":     "info",
			"user_request": "看一下系统负载",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if !strings.Contains(result.Summary, "看一下系统负载") {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected read-only diagnosis to avoid direct execution hint, got %s", result.ExecutionHint)
	}
	if len(result.ToolPlan) == 0 || result.ToolPlan[0].Tool != "metrics.query_range" {
		t.Fatalf("expected read-only diagnosis to start with metrics evidence, got %+v", result.ToolPlan)
	}
	for _, step := range result.ToolPlan {
		if step.Tool == "execution.run_command" {
			t.Fatalf("expected read-only diagnosis to defer host execution, got %+v", result.ToolPlan)
		}
	}
}

func TestBuildDiagnosisChatRequestSuggestsReadOnlyExitIPCommand(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{LocalCommandFallbackEnable: true})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-chat-exit-ip",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"severity":     "info",
			"user_request": "看一下你的出口IP是多少",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if !strings.Contains(result.Summary, "看一下你的出口IP是多少") {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.ExecutionHint != "curl -fsS https://api.ipify.org && echo" {
		t.Fatalf("unexpected execution hint: %s", result.ExecutionHint)
	}
}

func TestBuildDiagnosisChatRequestSuggestsRestartCommandForService(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{LocalCommandFallbackEnable: true})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-chat-restart",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"service":      "sshd",
			"severity":     "info",
			"user_request": "重启 sshd",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.ExecutionHint != "systemctl restart sshd" {
		t.Fatalf("unexpected execution hint: %s", result.ExecutionHint)
	}
}

func TestBuildDiagnosisChatRequestSuggestsStatusCommandForExplicitChineseCommand(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{LocalCommandFallbackEnable: true})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-chat-status-cn",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"service":      "api",
			"severity":     "info",
			"summary":      "cpu too high",
			"user_request": "执行命令查看 api 状态",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.ExecutionHint != "systemctl status api --no-pager --lines=20 || true" {
		t.Fatalf("unexpected execution hint: %s", result.ExecutionHint)
	}
}

func TestBuildDiagnosisChatRequestUsesReadOnlyEvidenceForRootCauseQuestions(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{LocalCommandFallbackEnable: true})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-chat-root-cause",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"service":      "api",
			"severity":     "warning",
			"user_request": "最近 api 报错和最近一次发布有关系吗",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected root-cause diagnosis to avoid direct execution hint, got %s", result.ExecutionHint)
	}
	if len(result.ToolPlan) < 4 {
		t.Fatalf("expected root-cause diagnosis to gather read-only evidence, got %+v", result.ToolPlan)
	}
	if result.ToolPlan[0].Tool != "metrics.query_range" {
		t.Fatalf("expected metrics evidence first, got %+v", result.ToolPlan)
	}
	if result.ToolPlan[1].Tool != "logs.query" {
		t.Fatalf("expected logs evidence second, got %+v", result.ToolPlan)
	}
	if result.ToolPlan[2].Tool != "observability.query" {
		t.Fatalf("expected observability evidence third, got %+v", result.ToolPlan)
	}
	if result.ToolPlan[3].Tool != "delivery.query" {
		t.Fatalf("expected delivery correlation last, got %+v", result.ToolPlan)
	}
	for _, step := range result.ToolPlan {
		if step.Tool == "execution.run_command" {
			t.Fatalf("expected root-cause diagnosis to defer host execution, got %+v", result.ToolPlan)
		}
	}
}

func TestBuildDiagnosisUsesModelAPIWhenConfigured(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://model.example.test/v1/chat/completions" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer secret" {
				t.Fatalf("unexpected auth header: %s", req.Header.Get("Authorization"))
			}
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
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-1",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "model summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("unexpected execution hint: %s", result.ExecutionHint)
	}
}

func TestBuildDiagnosisAuditsRawAndSentModelRequest(t *testing.T) {
	t.Parallel()

	auditLogger := &captureAuditLogger{}
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"inspect [PATH_1] on [IP_1] with token [REDACTED]\",\"execution_hint\":\"cat [PATH_1]\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
		Audit:   auditLogger,
	})
	_, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-audit-1",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"user_request": "执行命令 curl -H 'Authorization: Bearer sk-abc123' /tmp/1.txt",
			"command":      "cat /tmp/1.txt",
			"severity":     "info",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}

	if len(auditLogger.entries) != 1 {
		t.Fatalf("expected one audit entry, got %d", len(auditLogger.entries))
	}
	entry := auditLogger.entries[0]
	if entry.ResourceType != "llm_request" || entry.Action != "chat_completions_send" || entry.ResourceID != "ses-audit-1" {
		t.Fatalf("unexpected audit entry: %+v", entry)
	}

	rawPrompt, _ := entry.Metadata["user_prompt_raw"].(string)
	sentPrompt, _ := entry.Metadata["user_prompt_sent"].(string)
	if !strings.Contains(rawPrompt, "sk-abc123") || !strings.Contains(rawPrompt, "/tmp/1.txt") {
		t.Fatalf("expected raw prompt to keep original values, got %s", rawPrompt)
	}
	if strings.Contains(sentPrompt, "sk-abc123") || strings.Contains(sentPrompt, "/tmp/1.txt") {
		t.Fatalf("expected sent prompt to be desensitized, got %s", sentPrompt)
	}
	if !strings.Contains(sentPrompt, "[REDACTED]") || !strings.Contains(sentPrompt, "[PATH_1]") || !strings.Contains(sentPrompt, "[IP_1]") {
		t.Fatalf("expected sent prompt to contain placeholders, got %s", sentPrompt)
	}

	rawRequest, _ := entry.Metadata["request_raw"].(map[string]interface{})
	sentRequest, _ := entry.Metadata["request_sent"].(map[string]interface{})
	if rawRequest["model"] != "gpt-4o-mini" || sentRequest["model"] != "gpt-4o-mini" {
		t.Fatalf("unexpected request metadata: raw=%+v sent=%+v", rawRequest, sentRequest)
	}
}

func TestBuildDiagnosisDesensitizesModelContextAndRehydratesSummary(t *testing.T) {
	t.Parallel()

	var requestBody string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			requestBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"investigate [HOST_1] via [IP_1] and keep token [REDACTED] hidden\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-1",
		Context: map[string]interface{}{
			"alert_name": "HighCPU",
			"host":       "openclaw",
			"note":       "node 192.168.3.106 token=abc123",
			"severity":   "critical",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}

	if strings.Contains(requestBody, "openclaw") || strings.Contains(requestBody, "192.168.3.106") || strings.Contains(requestBody, "abc123") {
		t.Fatalf("expected request body to be desensitized, got %s", requestBody)
	}
	if !strings.Contains(requestBody, "[HOST_1]") || !strings.Contains(requestBody, "[IP_1]") || !strings.Contains(requestBody, "[REDACTED]") {
		t.Fatalf("expected request body to contain placeholders, got %s", requestBody)
	}
	if result.Summary != "investigate openclaw via 192.168.3.106 and keep token [REDACTED] hidden" {
		t.Fatalf("unexpected rehydrated summary: %s", result.Summary)
	}
	if result.DesenseMap["[HOST_1]"] != "openclaw" {
		t.Fatalf("unexpected desense host map: %+v", result.DesenseMap)
	}
	if result.DesenseMap["[IP_1]"] != "192.168.3.106" {
		t.Fatalf("unexpected desense ip map: %+v", result.DesenseMap)
	}
}

func TestBuildDiagnosisDesensitizesPathsAsPathPlaceholders(t *testing.T) {
	t.Parallel()

	var requestBody string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			requestBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"inspect [PATH_1] on [IP_1]\",\"execution_hint\":\"cat [PATH_1]\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-path-placeholder",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"user_request": "执行命令 cat /tmp/1.txt",
			"command":      "cat /tmp/1.txt",
			"file_path":    "/tmp/1.txt",
			"severity":     "info",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}

	if strings.Contains(requestBody, "/tmp/1.txt") {
		t.Fatalf("expected path to be desensitized, got %s", requestBody)
	}
	if !strings.Contains(requestBody, "[PATH_1]") {
		t.Fatalf("expected request body to contain [PATH_1], got %s", requestBody)
	}
	if strings.Contains(requestBody, "[HOST_1]") {
		t.Fatalf("expected path not to be classified as HOST placeholder, got %s", requestBody)
	}
	if result.ExecutionHint != "cat /tmp/1.txt" {
		t.Fatalf("expected execution hint to be rehydrated, got %q", result.ExecutionHint)
	}
	if result.DesenseMap["[PATH_1]"] != "/tmp/1.txt" {
		t.Fatalf("unexpected desense path map: %+v", result.DesenseMap)
	}
}

func TestBuildDiagnosisRedactsPasswordsTokensAndAPIKeysBeforeModel(t *testing.T) {
	t.Parallel()

	var requestBody string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			requestBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"manual handling required with token [REDACTED]\",\"execution_hint\":\"\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-secret-redaction",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"user_request": "执行命令 curl -H 'Authorization: Bearer sk-abc123' 'https://example.test?api_key=abc' && export PASSWORD=hunter2 && echo token=xyz",
			"severity":     "info",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}

	for _, leaked := range []string{"sk-abc123", "hunter2", "api_key=abc", "token=xyz"} {
		if strings.Contains(requestBody, leaked) {
			t.Fatalf("expected secret %q to be redacted from model request body: %s", leaked, requestBody)
		}
	}
	for _, want := range []string{"Bearer [REDACTED]", "api_key=[REDACTED]", "token=[REDACTED]", "PASSWORD=[REDACTED]"} {
		if !strings.Contains(requestBody, want) {
			t.Fatalf("expected request body to contain %q, got %s", want, requestBody)
		}
	}
	if strings.Contains(result.Summary, "sk-abc123") || strings.Contains(result.Summary, "hunter2") {
		t.Fatalf("expected summary to keep secrets redacted, got %s", result.Summary)
	}
}

func TestBuildDiagnosisRehydratesExecutionHintBeforeApproval(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"delete [PATH_1] on [IP_1]\",\"execution_hint\":\"rm [PATH_1]\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-rehydrate-exec-1",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"user_request": "执行命令 rm /tmp/1.txt",
			"command":      "rm /tmp/1.txt",
			"severity":     "info",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.ExecutionHint != "rm /tmp/1.txt" {
		t.Fatalf("expected execution hint to be rehydrated, got %q", result.ExecutionHint)
	}
	if result.Summary != "delete /tmp/1.txt on 192.168.3.106" {
		t.Fatalf("unexpected rehydrated summary: %s", result.Summary)
	}
}

func TestBuildDiagnosisDropsExecutionHintContainingRedactedSecret(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"call upstream using token [REDACTED]\",\"execution_hint\":\"curl -H 'Authorization: Bearer [REDACTED]' https://example.test\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-secret-exec-drop",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"user_request": "执行命令 curl -H 'Authorization: Bearer sk-abc123' https://example.test",
			"severity":     "info",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected secret-bearing execution hint to be dropped, got %q", result.ExecutionHint)
	}
	if !strings.Contains(result.Summary, "[REDACTED]") {
		t.Fatalf("expected summary to keep secrets redacted, got %s", result.Summary)
	}
}

func TestBuildDiagnosisFallsBackWhenModelAPIErrors(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"upstream unavailable"}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-1",
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
	if !strings.Contains(result.Summary, "HighCPU") {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected no execution hint without local fallback, got %s", result.ExecutionHint)
	}
}

func TestBuildDiagnosisUsesLocalLLMAssistDetectionsWhenEnabled(t *testing.T) {
	t.Parallel()

	auditLogger := &captureAuditLogger{}
	var mainRequestBody string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Host {
			case "local-desense.example.test":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"choices":[{"message":{"content":"{\"secrets\":[],\"hosts\":[\"phoenix-cluster\"],\"ips\":[],\"paths\":[]}"}}]
					}`)),
				}, nil
			case "model.example.test":
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read main request body: %v", err)
				}
				mainRequestBody = string(body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"choices":[{"message":{"content":"{\"summary\":\"inspect [HOST_1]\",\"execution_hint\":\"hostname && uptime\"}"}}]
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected host: %s", req.URL.Host)
				return nil, nil
			}
		}),
	}

	cfg := DefaultDesensitizationConfig()
	cfg.LocalLLMAssist.Enabled = true
	cfg.LocalLLMAssist.BaseURL = "https://local-desense.example.test"
	cfg.LocalLLMAssist.Model = "qwen-local"

	svc := NewService(Options{
		BaseURL:                 "https://model.example.test",
		APIKey:                  "secret",
		Model:                   "gpt-4o-mini",
		Client:                  client,
		Audit:                   auditLogger,
		DesensitizationProvider: staticDesensitizationProvider{cfg: cfg},
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-local-desense-1",
		Context: map[string]interface{}{
			"user_request": "请分析 phoenix-cluster 的异常",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if strings.Contains(mainRequestBody, "phoenix-cluster") {
		t.Fatalf("expected local llm assist to mask phoenix-cluster, got %s", mainRequestBody)
	}
	if !strings.Contains(mainRequestBody, "[HOST_1]") {
		t.Fatalf("expected request to include host placeholder, got %s", mainRequestBody)
	}
	if result.Summary != "inspect phoenix-cluster" {
		t.Fatalf("expected summary to rehydrate local llm host placeholder, got %q", result.Summary)
	}

	actions := make([]string, 0, len(auditLogger.entries))
	for _, entry := range auditLogger.entries {
		actions = append(actions, entry.Action)
	}
	if !containsString(actions, "local_llm_desensitization_detect_send") || !containsString(actions, "local_llm_desensitization_detect_result") {
		t.Fatalf("expected local llm audit actions, got %+v", actions)
	}
}

func TestBuildDiagnosisFallsBackWhenLocalLLMAssistFails(t *testing.T) {
	t.Parallel()

	var mainRequestBody string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Host {
			case "local-desense.example.test":
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":"unavailable"}`)),
				}, nil
			case "model.example.test":
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read main request body: %v", err)
				}
				mainRequestBody = string(body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"choices":[{"message":{"content":"{\"summary\":\"model summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected host: %s", req.URL.Host)
				return nil, nil
			}
		}),
	}

	cfg := DefaultDesensitizationConfig()
	cfg.LocalLLMAssist.Enabled = true
	cfg.LocalLLMAssist.BaseURL = "https://local-desense.example.test"
	cfg.LocalLLMAssist.Model = "qwen-local"

	svc := NewService(Options{
		BaseURL:                 "https://model.example.test",
		APIKey:                  "secret",
		Model:                   "gpt-4o-mini",
		Client:                  client,
		DesensitizationProvider: staticDesensitizationProvider{cfg: cfg},
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-local-desense-fallback",
		Context: map[string]interface{}{
			"user_request": "执行命令 cat /tmp/1.txt token=abc123",
			"host":         "192.168.3.106",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "model summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if strings.Contains(mainRequestBody, "abc123") || strings.Contains(mainRequestBody, "/tmp/1.txt") || strings.Contains(mainRequestBody, "192.168.3.106") {
		t.Fatalf("expected rule-based desensitization to continue on local assist failure, got %s", mainRequestBody)
	}
	if !strings.Contains(mainRequestBody, "[REDACTED]") || !strings.Contains(mainRequestBody, "[PATH_1]") || !strings.Contains(mainRequestBody, "[IP_1]") {
		t.Fatalf("expected fallback request to remain desensitized, got %s", mainRequestBody)
	}
}

func TestBuildDiagnosisDoesNotEnableLocalLLMAssistFromProviderBindingWhenDisabled(t *testing.T) {
	t.Parallel()

	var modelCalls int
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Host {
			case "assist-model.example.test":
				t.Fatalf("did not expect local llm assist request when disabled")
				return nil, nil
			case "model.example.test":
				modelCalls++
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"choices":[{"message":{"content":"{\"summary\":\"model summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected host: %s", req.URL.Host)
				return nil, nil
			}
		}),
	}

	cfg := DefaultDesensitizationConfig()
	cfg.LocalLLMAssist.Enabled = false

	svc := NewService(Options{
		BaseURL:                 "https://model.example.test",
		Model:                   "gpt-4o-mini",
		Client:                  client,
		DesensitizationProvider: staticDesensitizationProvider{cfg: cfg},
		ProviderRegistry: staticProviderRegistry{
			assist: &ModelTarget{
				ProviderID: "gemini-backup",
				Protocol:   ModelProtocolGemini,
				BaseURL:    "https://assist-model.example.test",
				Model:      "gemini-flash-lite-latest",
			},
		},
	})

	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-local-desense-disabled",
		Context: map[string]interface{}{
			"user_request": "host=192.168.3.106 看系统负载",
			"host":         "192.168.3.106",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "model summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if modelCalls != 1 {
		t.Fatalf("expected only main model to be called once, got %d", modelCalls)
	}
}

type captureAuditLogger struct {
	entries []audit.Entry
}

func (c *captureAuditLogger) Log(_ context.Context, entry audit.Entry) {
	c.entries = append(c.entries, entry)
}

type staticProviderRegistry struct {
	primary    *ModelTarget
	assist     *ModelTarget
	byProvider map[string]*ModelTarget
}

func (s staticProviderRegistry) ResolvePrimaryModelTarget() *ModelTarget {
	return s.primary
}

func (s staticProviderRegistry) ResolveAssistModelTarget() *ModelTarget {
	return s.assist
}

func (s staticProviderRegistry) ResolvePrimaryModelTargetWithSecrets(_ *secrets.Store) *ModelTarget {
	return s.primary
}

func (s staticProviderRegistry) ResolveAssistModelTargetWithSecrets(_ *secrets.Store) *ModelTarget {
	return s.assist
}

func (s staticProviderRegistry) Snapshot() ProvidersSnapshot {
	config := ProvidersConfig{}
	appendEntry := func(providerID string, target *ModelTarget) {
		if target == nil || strings.TrimSpace(providerID) == "" {
			return
		}
		for _, existing := range config.Entries {
			if existing.ID == providerID {
				return
			}
		}
		config.Entries = append(config.Entries, ProviderEntry{
			ID:       providerID,
			Vendor:   target.Vendor,
			Protocol: target.Protocol,
			BaseURL:  target.BaseURL,
			APIKey:   target.APIKey,
			Enabled:  true,
		})
	}
	appendEntry(func() string {
		if s.primary == nil {
			return ""
		}
		return s.primary.ProviderID
	}(), s.primary)
	appendEntry(func() string {
		if s.assist == nil {
			return ""
		}
		return s.assist.ProviderID
	}(), s.assist)
	for providerID, target := range s.byProvider {
		appendEntry(providerID, target)
	}
	if s.primary != nil {
		config.Primary = ProviderBinding{ProviderID: s.primary.ProviderID, Model: s.primary.Model}
	}
	if s.assist != nil {
		config.Assist = ProviderBinding{ProviderID: s.assist.ProviderID, Model: s.assist.Model}
	}
	return ProvidersSnapshot{Config: config}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestBuildDiagnosisRecordsProviderMetrics(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
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
		Metrics: registry,
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	if _, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-1",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	}); err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}

	var output strings.Builder
	if err := registry.WritePrometheus(&output); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	if !strings.Contains(output.String(), `tars_external_provider_requests_total{operation="chat_completions",provider="model",result="success"} 1`) {
		t.Fatalf("expected provider metric, got:\n%s", output.String())
	}
}

func TestBuildDiagnosisParsesJSONWrappedInExtraText(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"answer: {\"summary\":\"wrapped summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-1",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "wrapped summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("unexpected execution hint: %s", result.ExecutionHint)
	}
}

func TestBuildDiagnosisFallsBackToSafeExecutionHintWhenModelSuggestsUnsafeCommand(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"model summary\",\"execution_hint\":\"ssh 192.168.3.106 'sudo systemctl status sshd'\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-1",
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
	if result.Summary != "model summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected unsafe command to be dropped, got %s", result.ExecutionHint)
	}
}

func TestBuildDiagnosisFallsBackToLocalCommandOnlyWhenEnabled(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"model summary\",\"execution_hint\":\"ssh 192.168.3.106 'sudo systemctl status sshd'\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL:                    "https://model.example.test",
		APIKey:                     "secret",
		Model:                      "gpt-4o-mini",
		Client:                     client,
		LocalCommandFallbackEnable: true,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-1",
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
	if result.ExecutionHint != "" {
		t.Fatalf("expected generic alert to avoid local fallback execution hint, got %s", result.ExecutionHint)
	}
}

func TestBuildDiagnosisSuppressesGenericExecutionHintWithoutExplicitCommandRequest(t *testing.T) {
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
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-generic-model-hint",
		Context: map[string]interface{}{
			"alert_name": "HighCPU",
			"host":       "host-1",
			"severity":   "critical",
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected generic alert model hint to be suppressed, got %q", result.ExecutionHint)
	}
}

func TestBuildDiagnosisUsesInjectedPromptSet(t *testing.T) {
	t.Parallel()

	var requestBody string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			requestBody = string(body)
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
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
		Prompts: NewPromptSet(
			"custom system prompt",
			"sid={{ .SessionID }} host={{ index .Context \"host\" }} json={{ .ContextJSON }}",
		),
	})
	if _, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-prompt-1",
		Context: map[string]interface{}{
			"host": "host-1",
		},
	}); err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}

	if !strings.Contains(requestBody, "custom system prompt") {
		t.Fatalf("expected custom system prompt in request body, got %s", requestBody)
	}
	if !strings.Contains(requestBody, "sid=ses-prompt-1 host=[HOST_1]") {
		t.Fatalf("expected rendered custom user prompt in request body, got %s", requestBody)
	}
}

func TestBuildDiagnosisFallsBackToAssistModelWhenPrimaryFails(t *testing.T) {
	t.Parallel()

	auditLogger := &captureAuditLogger{}
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Host {
			case "primary-model.example.test":
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":"temporary failure"}`)),
				}, nil
			case "assist-model.example.test":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"candidates":[{
							"content":{
								"parts":[{"text":"{\"summary\":\"assist summary\",\"execution_hint\":\"hostname && uptime\"}"}]
							}
						}]
					}`)),
				}, nil
			default:
				t.Fatalf("unexpected host: %s", req.URL.Host)
				return nil, nil
			}
		}),
	}

	svc := NewService(Options{
		Client: client,
		Audit:  auditLogger,
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "lmstudio-main",
				Protocol:   ModelProtocolLMStudio,
				BaseURL:    "https://primary-model.example.test",
				Model:      "qwen/qwen3-4b-2507",
			},
			assist: &ModelTarget{
				ProviderID: "gemini-backup",
				Protocol:   ModelProtocolGemini,
				BaseURL:    "https://assist-model.example.test",
				Model:      "gemini-flash-lite-latest",
			},
		},
	})

	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-assist-fallback-1",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "assist summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("unexpected execution hint: %s", result.ExecutionHint)
	}

	actions := make([]string, 0, len(auditLogger.entries))
	for _, entry := range auditLogger.entries {
		actions = append(actions, entry.Action)
	}
	if !containsString(actions, "chat_completions_failover") {
		t.Fatalf("expected failover audit action, got %+v", actions)
	}
}

func TestBuildDiagnosisUsesRoleModelBindingPrimaryTarget(t *testing.T) {
	t.Parallel()

	var calledHosts []string
	var calledModels []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calledHosts = append(calledHosts, req.URL.Host)
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}
			model, _ := payload["model"].(string)
			calledModels = append(calledModels, model)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"role-bound summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client: client,
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "openai-main",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://role-primary.example.test",
				Model:      "gpt-4o-mini",
			},
			assist: &ModelTarget{
				ProviderID: "platform-assist",
				Protocol:   ModelProtocolGemini,
				BaseURL:    "https://platform-assist.example.test",
				Model:      "platform-assist-model",
			},
			byProvider: map[string]*ModelTarget{
				"openai-main": {
					ProviderID: "openai-main",
					Protocol:   ModelProtocolOpenAICompatible,
					BaseURL:    "https://role-primary.example.test",
					Model:      "gpt-4o-mini",
				},
			},
		},
	})

	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-role-binding-primary",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
		RoleModelBinding: &contracts.RoleModelBinding{
			Primary: &contracts.RoleModelTargetBinding{
				ProviderID: "openai-main",
				Model:      "gpt-4o-mini",
			},
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "role-bound summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if len(calledHosts) != 1 || calledHosts[0] != "role-primary.example.test" {
		t.Fatalf("expected role-bound provider host, got %+v", calledHosts)
	}
	if len(calledModels) != 1 || calledModels[0] != "gpt-4o-mini" {
		t.Fatalf("expected role-bound provider model, got %+v", calledModels)
	}
}

func TestBuildDiagnosisUsesRoleModelBindingFallbackTarget(t *testing.T) {
	t.Parallel()

	var calledHosts []string
	var calledPaths []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calledHosts = append(calledHosts, req.URL.Host)
			calledPaths = append(calledPaths, req.URL.Path)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"candidates":[{"content":{"parts":[{"text":"{\"summary\":\"assist-role summary\",\"execution_hint\":\"hostname && uptime\"}"}]}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client: client,
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "platform-primary",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://platform-primary.example.test",
				Model:      "platform-main",
			},
			assist: &ModelTarget{
				ProviderID: "gemini-backup",
				Protocol:   ModelProtocolGemini,
				BaseURL:    "https://role-assist.example.test",
				Model:      "gemini-flash-lite-latest",
			},
			byProvider: map[string]*ModelTarget{
				"gemini-backup": {
					ProviderID: "gemini-backup",
					Protocol:   ModelProtocolGemini,
					BaseURL:    "https://role-assist.example.test",
					Model:      "gemini-flash-lite-latest",
				},
			},
		},
	})

	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-role-binding-assist",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
		RoleModelBinding: &contracts.RoleModelBinding{
			Primary: &contracts.RoleModelTargetBinding{
				ProviderID: "gemini-backup",
				Model:      "gemini-flash-lite-latest",
			},
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "assist-role summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if len(calledHosts) != 1 || calledHosts[0] != "role-assist.example.test" {
		t.Fatalf("expected assist role-bound provider host, got %+v", calledHosts)
	}
	if len(calledPaths) != 1 || !strings.Contains(calledPaths[0], "/models/gemini-flash-lite-latest:generateContent") {
		t.Fatalf("expected assist role-bound provider model in endpoint, got %+v", calledPaths)
	}
}

func TestBuildDiagnosisUsesRoleModelBindingFallbackBeforePlatformAssist(t *testing.T) {
	t.Parallel()

	var calledHosts []string
	var calledModels []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calledHosts = append(calledHosts, req.URL.Host)
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}
			model, _ := payload["model"].(string)
			calledModels = append(calledModels, model)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"platform fallback summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client: client,
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "platform-primary",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://platform-primary.example.test",
				Model:      "platform-main",
			},
			assist: &ModelTarget{
				ProviderID: "platform-assist",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://platform-assist.example.test",
				Model:      "platform-assist-model",
			},
			byProvider: map[string]*ModelTarget{
				"gemini-backup": {
					ProviderID: "gemini-backup",
					Protocol:   ModelProtocolOpenAICompatible,
					BaseURL:    "https://role-fallback.example.test",
					Model:      "gemini-flash-lite-latest",
				},
			},
		},
	})

	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-role-binding-fallback",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
		RoleModelBinding: &contracts.RoleModelBinding{
			Primary: &contracts.RoleModelTargetBinding{
				ProviderID: "openai-main",
				Model:      "missing-model-on-provider",
			},
			Fallback: &contracts.RoleModelTargetBinding{
				ProviderID: "gemini-backup",
				Model:      "gemini-flash-lite-latest",
			},
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "platform fallback summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if len(calledHosts) != 1 || calledHosts[0] != "role-fallback.example.test" {
		t.Fatalf("expected role fallback host, got %+v", calledHosts)
	}
	if len(calledModels) != 1 || calledModels[0] != "gemini-flash-lite-latest" {
		t.Fatalf("expected role fallback model, got %+v", calledModels)
	}
}

func TestBuildDiagnosisUsesPlatformPrimaryWhenRoleBindingInheritsDefault(t *testing.T) {
	t.Parallel()

	var calledHosts []string
	var calledModels []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calledHosts = append(calledHosts, req.URL.Host)
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}
			model, _ := payload["model"].(string)
			calledModels = append(calledModels, model)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"platform inherited summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client: client,
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "platform-primary",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://platform-primary.example.test",
				Model:      "platform-main",
			},
			assist: &ModelTarget{
				ProviderID: "platform-assist",
				Protocol:   ModelProtocolGemini,
				BaseURL:    "https://platform-assist.example.test",
				Model:      "platform-assist-model",
			},
		},
	})

	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-role-binding-inherit-primary",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
		RoleModelBinding: &contracts.RoleModelBinding{
			InheritPlatformDefault: true,
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "platform inherited summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if len(calledHosts) != 1 || calledHosts[0] != "platform-primary.example.test" {
		t.Fatalf("expected platform primary host, got %+v", calledHosts)
	}
	if len(calledModels) != 1 || calledModels[0] != "platform-main" {
		t.Fatalf("expected platform primary model, got %+v", calledModels)
	}
}

func TestBuildDiagnosisUsesRoleModelBindingExplicitPrimaryModel(t *testing.T) {
	t.Parallel()

	var calledHosts []string
	var calledModels []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calledHosts = append(calledHosts, req.URL.Host)
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}
			model, _ := payload["model"].(string)
			calledModels = append(calledModels, model)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"direct-bound summary\",\"execution_hint\":\"hostname\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client: client,
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "platform-primary",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://platform-primary.example.test",
				Model:      "platform-main",
			},
			assist: &ModelTarget{
				ProviderID: "platform-assist",
				Protocol:   ModelProtocolGemini,
				BaseURL:    "https://platform-assist.example.test",
				Model:      "platform-assist-model",
			},
			byProvider: map[string]*ModelTarget{
				"openai-main": {
					ProviderID: "openai-main",
					Protocol:   ModelProtocolOpenAICompatible,
					BaseURL:    "https://role-primary.example.test",
					Model:      "provider-default-that-should-not-be-used",
				},
			},
		},
	})

	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-role-binding-direct-model",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
		RoleModelBinding: &contracts.RoleModelBinding{
			Primary: &contracts.RoleModelTargetBinding{
				ProviderID: "openai-main",
				Model:      "gpt-4.1-mini",
			},
		},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "direct-bound summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if len(calledHosts) != 1 || calledHosts[0] != "role-primary.example.test" {
		t.Fatalf("expected direct-bound provider host, got %+v", calledHosts)
	}
	if len(calledModels) != 1 || calledModels[0] != "gpt-4.1-mini" {
		t.Fatalf("expected direct-bound provider model, got %+v", calledModels)
	}
}

func TestPlanDiagnosisUsesRoleModelBindingPrimaryTarget(t *testing.T) {
	t.Parallel()

	var calledHosts []string
	var calledModels []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calledHosts = append(calledHosts, req.URL.Host)
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}
			model, _ := payload["model"].(string)
			calledModels = append(calledModels, model)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"planner summary\",\"tool_plan\":[{\"tool\":\"metrics.query_range\",\"reason\":\"inspect host\",\"priority\":1,\"params\":{\"query\":\"up\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client: client,
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "openai-main",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://role-primary.example.test",
				Model:      "gpt-4o-mini",
			},
			assist: &ModelTarget{
				ProviderID: "platform-assist",
				Protocol:   ModelProtocolGemini,
				BaseURL:    "https://platform-assist.example.test",
				Model:      "platform-assist-model",
			},
			byProvider: map[string]*ModelTarget{
				"openai-main": {
					ProviderID: "openai-main",
					Protocol:   ModelProtocolOpenAICompatible,
					BaseURL:    "https://role-primary.example.test",
					Model:      "gpt-4o-mini",
				},
			},
		},
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-role-binding-plan-primary",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
		RoleModelBinding: &contracts.RoleModelBinding{
			Primary: &contracts.RoleModelTargetBinding{
				ProviderID: "openai-main",
				Model:      "gpt-4o-mini",
			},
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if plan.Summary != "planner summary" {
		t.Fatalf("unexpected summary: %s", plan.Summary)
	}
	if len(calledHosts) != 1 || calledHosts[0] != "role-primary.example.test" {
		t.Fatalf("expected planner to use role-bound provider host, got %+v", calledHosts)
	}
	if len(calledModels) != 1 || calledModels[0] != "gpt-4o-mini" {
		t.Fatalf("expected planner to use role-bound provider model, got %+v", calledModels)
	}
}

func TestFinalizeDiagnosisUsesRoleModelBindingPrimaryTarget(t *testing.T) {
	t.Parallel()

	var calledHosts []string
	var calledModels []string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calledHosts = append(calledHosts, req.URL.Host)
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("unmarshal request body: %v", err)
			}
			model, _ := payload["model"].(string)
			calledModels = append(calledModels, model)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"final summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client: client,
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "openai-main",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://role-primary.example.test",
				Model:      "gpt-4o-mini",
			},
			assist: &ModelTarget{
				ProviderID: "platform-assist",
				Protocol:   ModelProtocolGemini,
				BaseURL:    "https://platform-assist.example.test",
				Model:      "platform-assist-model",
			},
			byProvider: map[string]*ModelTarget{
				"openai-main": {
					ProviderID: "openai-main",
					Protocol:   ModelProtocolOpenAICompatible,
					BaseURL:    "https://role-primary.example.test",
					Model:      "gpt-4o-mini",
				},
			},
		},
	})

	result, err := svc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-role-binding-finalize-primary",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
		RoleModelBinding: &contracts.RoleModelBinding{
			Primary: &contracts.RoleModelTargetBinding{
				ProviderID: "openai-main",
				Model:      "gpt-4o-mini",
			},
		},
	})
	if err != nil {
		t.Fatalf("finalize diagnosis: %v", err)
	}
	if result.Summary != "final summary" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if len(calledHosts) != 1 || calledHosts[0] != "role-primary.example.test" {
		t.Fatalf("expected finalizer to use role-bound provider host, got %+v", calledHosts)
	}
	if len(calledModels) != 1 || calledModels[0] != "gpt-4o-mini" {
		t.Fatalf("expected finalizer to use role-bound provider model, got %+v", calledModels)
	}
}

func TestPlanDiagnosisRehydratesPlannerToolPlanPlaceholders(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"query [IP_1] load history\",\"tool_plan\":[{\"tool\":\"metrics.query_range\",\"connector_id\":\"prometheus-main\",\"reason\":\"inspect [IP_1] trend\",\"priority\":1,\"params\":{\"host\":\"[IP_1]\",\"mode\":\"range\",\"query\":\"node_load1{instance=\\\"[IP_1]\\\"}\",\"window\":\"1h\",\"step\":\"5m\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-1",
		Context: map[string]interface{}{
			"host":         "127.0.0.1:9100",
			"user_request": "过去一小时机器负载怎么样",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if plan.Summary != "query 127.0.0.1:9100 load history" {
		t.Fatalf("unexpected planner summary: %s", plan.Summary)
	}
	if len(plan.ToolPlan) != 1 {
		t.Fatalf("expected one tool step, got %d", len(plan.ToolPlan))
	}
	step := plan.ToolPlan[0]
	if got := step.Input["host"]; got != "127.0.0.1:9100" {
		t.Fatalf("expected rehydrated host, got %#v", got)
	}
	if got := step.Input["query"]; got != `node_load1{instance="127.0.0.1:9100"}` {
		t.Fatalf("expected rehydrated query, got %#v", got)
	}
}

func TestPlanDiagnosisAcceptsStringPriorityAndNormalizesMetricsQuery(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"query [IP_1] load history\",\"tool_plan\":[{\"tool\":\"metrics.query_range\",\"connector_id\":\"prometheus\",\"reason\":\"inspect [IP_1] trend\",\"priority\":\"1\",\"params\":{\"host\":\"[IP_1]\",\"mode\":\"range\",\"query\":\"node_load1\",\"window\":\"1h\",\"step\":\"1m\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		APIKey:  "secret",
		Model:   "gpt-4o-mini",
		Client:  client,
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-2",
		Context: map[string]interface{}{
			"host":         "127.0.0.1:9100",
			"service":      "node",
			"user_request": "过去一小时机器负载怎么样",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if len(plan.ToolPlan) != 1 {
		t.Fatalf("expected one tool step, got %d", len(plan.ToolPlan))
	}
	step := plan.ToolPlan[0]
	if step.Priority != 1 {
		t.Fatalf("expected parsed string priority 1, got %d", step.Priority)
	}
	if step.ConnectorID != "prometheus" {
		t.Fatalf("expected connector alias to be preserved for runtime resolution, got %q", step.ConnectorID)
	}
	if got := step.Input["query"]; got != `node_load1{instance="127.0.0.1:9100"}` {
		t.Fatalf("expected host-filtered query, got %#v", got)
	}
}

func TestPlanDiagnosisIncludesToolCapabilitiesInPlannerPrompt(t *testing.T) {
	t.Parallel()

	var systemPrompt string
	var userPrompt string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			payload := map[string]interface{}{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			messages, _ := payload["messages"].([]interface{})
			for _, item := range messages {
				message, _ := item.(map[string]interface{})
				role, _ := message["role"].(string)
				content, _ := message["content"].(string)
				switch role {
				case "system":
					systemPrompt = content
				case "user":
					userPrompt = content
				}
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"query load history\",\"tool_plan\":[{\"tool\":\"metrics.query_range\",\"connector_id\":\"prometheus-main\",\"reason\":\"inspect trend\",\"priority\":1,\"params\":{\"host\":\"192.168.3.106\",\"mode\":\"range\",\"query\":\"node_load1\",\"window\":\"1h\",\"step\":\"5m\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
		Client:  client,
	})
	_, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-capabilities",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"user_request": "过去一小时机器负载怎么样",
			"tool_capabilities_summary": strings.Join([]string{
				"- metrics.query_range via prometheus-main [query.range] (invocable): Run range PromQL query",
				"- connector.invoke_capability via skill-source-main [source.sync] (catalog-only): Sync skill metadata",
			}, "\n"),
			"tool_capabilities": []map[string]interface{}{
				{
					"tool":         "metrics.query_range",
					"connector_id": "prometheus-main",
					"invocable":    true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if !strings.Contains(systemPrompt, "Available session capabilities:") || !strings.Contains(systemPrompt, "metrics.query_range via prometheus-main") {
		t.Fatalf("expected planner system prompt to include capabilities, got %q", systemPrompt)
	}
	if !strings.Contains(userPrompt, "\"tool_capabilities\"") {
		t.Fatalf("expected user prompt to include tool_capabilities context, got %q", userPrompt)
	}
}

func TestPlanDiagnosisNormalizesObservabilityAndDeliveryTools(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"inspect logs and recent deployment\",\"tool_plan\":[{\"tool\":\"observability.query\",\"connector_id\":\"observability-stub\",\"reason\":\"check logs for api errors\",\"priority\":1,\"params\":{\"query\":\"error rate for api\",\"service\":\"api\"}},{\"tool\":\"delivery.query\",\"connector_id\":\"delivery-stub\",\"reason\":\"check recent deployment\",\"priority\":2,\"params\":{\"query\":\"recent deployments for api\",\"service\":\"api\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
		Client:  client,
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-observability-delivery",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"service":      "api",
			"user_request": "最近 api 报错和最近一次发布有关系吗",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if len(plan.ToolPlan) != 2 {
		t.Fatalf("expected 2 tool steps, got %d", len(plan.ToolPlan))
	}
	if plan.ToolPlan[0].Tool != "observability.query" {
		t.Fatalf("expected first tool to be observability.query, got %+v", plan.ToolPlan[0])
	}
	if got := plan.ToolPlan[0].Input["capability_id"]; got != "observability.query" {
		t.Fatalf("expected observability capability_id, got %#v", got)
	}
	if plan.ToolPlan[1].Tool != "delivery.query" {
		t.Fatalf("expected second tool to be delivery.query, got %+v", plan.ToolPlan[1])
	}
	if got := plan.ToolPlan[1].Input["capability_id"]; got != "delivery.query" {
		t.Fatalf("expected delivery capability_id, got %#v", got)
	}
}

func TestPlanDiagnosisNormalizesLogsAndTraceTools(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"inspect logs then traces\",\"tool_plan\":[{\"tool\":\"logs.query\",\"connector_id\":\"victorialogs-main\",\"reason\":\"collect api error logs\",\"priority\":1,\"params\":{\"query\":\"error AND service:api\",\"service\":\"api\"}},{\"tool\":\"observability.query\",\"connector_id\":\"tracing-main\",\"reason\":\"inspect traces after logs\",\"priority\":2,\"params\":{\"query\":\"service=api latency\",\"service\":\"api\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
		Client:  client,
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-logs-traces",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"service":      "api",
			"user_request": "先看 api 错误日志，再看 traces",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if len(plan.ToolPlan) != 2 {
		t.Fatalf("expected 2 tool steps, got %d", len(plan.ToolPlan))
	}
	if plan.ToolPlan[0].Tool != "logs.query" {
		t.Fatalf("expected first tool to be logs.query, got %+v", plan.ToolPlan[0])
	}
	if got := plan.ToolPlan[0].Input["capability_id"]; got != "logs.query" {
		t.Fatalf("expected logs capability_id, got %#v", got)
	}
	if plan.ToolPlan[1].Tool != "observability.query" {
		t.Fatalf("expected second tool to be observability.query, got %+v", plan.ToolPlan[1])
	}
	if got := plan.ToolPlan[1].Input["capability_id"]; got != "observability.query" {
		t.Fatalf("expected observability capability_id, got %#v", got)
	}
}

func TestPlanDiagnosisDefaultsMetricsConnectorFromCapabilities(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"query load history\",\"tool_plan\":[{\"tool\":\"metrics.query_range\",\"reason\":\"inspect load trend\",\"priority\":1,\"params\":{\"host\":\"192.168.3.106\",\"query\":\"node_load1\",\"window\":\"1h\",\"step\":\"5m\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
		Client:  client,
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-default-connector",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"user_request": "过去一小时机器负载怎么样",
			"tool_capabilities": []map[string]interface{}{
				{
					"tool":         "metrics.query_range",
					"connector_id": "prometheus-main",
					"invocable":    true,
				},
				{
					"tool":         "metrics.query_range",
					"connector_id": "victoriametrics-main",
					"invocable":    true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if len(plan.ToolPlan) != 1 {
		t.Fatalf("expected one tool step, got %d", len(plan.ToolPlan))
	}
	if plan.ToolPlan[0].ConnectorID != "prometheus-main" {
		t.Fatalf("expected first invocable connector to be selected, got %+v", plan.ToolPlan[0])
	}
	if got := plan.ToolPlan[0].Input["connector_id"]; got != "prometheus-main" {
		t.Fatalf("expected connector_id input to be populated, got %#v", got)
	}
}

func TestPlanDiagnosisDefaultsMetricsConnectorFromStructuredCapabilities(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"query load history\",\"tool_plan\":[{\"tool\":\"metrics.query_range\",\"reason\":\"inspect load trend\",\"priority\":1,\"params\":{\"host\":\"192.168.3.106\",\"query\":\"node_load1\",\"window\":\"1h\",\"step\":\"5m\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
		Client:  client,
	})

	type capability struct {
		Tool        string `json:"tool"`
		ConnectorID string `json:"connector_id"`
		Invocable   bool   `json:"invocable"`
	}

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-default-connector-struct",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"user_request": "过去一小时机器负载怎么样",
			"tool_capabilities": []capability{
				{Tool: "metrics.query_range", ConnectorID: "prometheus-main", Invocable: true},
				{Tool: "metrics.query_range", ConnectorID: "victoriametrics-main", Invocable: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if len(plan.ToolPlan) != 1 {
		t.Fatalf("expected one tool step, got %d", len(plan.ToolPlan))
	}
	if plan.ToolPlan[0].ConnectorID != "prometheus-main" {
		t.Fatalf("expected first invocable connector to be selected from structured capabilities, got %+v", plan.ToolPlan[0])
	}
	if got := plan.ToolPlan[0].Input["connector_id"]; got != "prometheus-main" {
		t.Fatalf("expected connector_id input to be populated from structured capabilities, got %#v", got)
	}
}

func TestPlanDiagnosisPlannerPromptDocumentsStepReferences(t *testing.T) {
	t.Parallel()

	var systemPrompt string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			payload := map[string]interface{}{}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			messages, _ := payload["messages"].([]interface{})
			for _, item := range messages {
				message, _ := item.(map[string]interface{})
				if role, _ := message["role"].(string); role == "system" {
					systemPrompt, _ = message["content"].(string)
				}
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"inspect and correlate\",\"tool_plan\":[{\"id\":\"observe_1\",\"tool\":\"observability.query\",\"reason\":\"collect errors\",\"priority\":1,\"params\":{\"query\":\"api errors\"}},{\"id\":\"knowledge_1\",\"tool\":\"knowledge.search\",\"reason\":\"use prior guidance\",\"priority\":2,\"params\":{\"query\":\"$steps.observe_1.output.result.summary\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
		Client:  client,
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-step-refs",
		Context: map[string]interface{}{
			"user_request": "结合错误和历史经验排查",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if !strings.Contains(systemPrompt, "$steps.<step_id>.output.<field>") {
		t.Fatalf("expected planner prompt to document step references, got %q", systemPrompt)
	}
	if len(plan.ToolPlan) != 2 {
		t.Fatalf("expected two tool steps, got %+v", plan.ToolPlan)
	}
	if got := plan.ToolPlan[1].Input["query"]; got != "$steps.observe_1.output.result.summary" {
		t.Fatalf("expected step reference to be preserved, got %#v", got)
	}
}

func TestSanitizeExecutionHintRejectsToolInvocationStrings(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})
	input := contracts.DiagnosisInput{
		SessionID: "ses-sanitize-tool-hint",
		Context: map[string]interface{}{
			"user_request": "过去一小时机器负载怎么样",
			"host":         "192.168.3.106",
		},
	}

	hint := `metrics.query_range --connector=victoriametrics-main --host=192.168.3.106 --query='node_load1'`
	if got := svc.sanitizeExecutionHint(input, hint); got != "" {
		t.Fatalf("expected tool invocation to be rejected, got %q", got)
	}
}

func TestSanitizeExecutionHintSuppressesWhenToolEvidenceAlreadyAnswers(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})
	input := contracts.DiagnosisInput{
		SessionID: "ses-sanitize-evidence",
		Context: map[string]interface{}{
			"user_request": "最近 api 报错和最近一次发布有关系吗",
			"observability_query_result": map[string]interface{}{
				"result_count": 4,
				"summary":      "error spike after release",
			},
			"delivery_query_result": map[string]interface{}{
				"result_count": 2,
				"branch":       "main",
			},
		},
	}

	if got := svc.sanitizeExecutionHint(input, "hostname && uptime"); got != "" {
		t.Fatalf("expected execution hint to be cleared when tool evidence exists, got %q", got)
	}
}

func TestFinalizeDiagnosisRewritesOverconfidentSummaryWhenCriticalEvidenceFails(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"最近API报错与最近一次发布无直接关联，因发布历史查询失败，无法确认发布变更内容或时间。\",\"execution_hint\":\"\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		BaseURL: "https://model.example.test",
		Model:   "gpt-4o-mini",
		Client:  client,
	})

	result, err := svc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-finalize-evidence-gate",
		Context: map[string]interface{}{
			"user_request": "最近 api 报错和最近一次发布有关系吗",
			"tool_results": []interface{}{
				map[string]interface{}{
					"tool":   "observability.query",
					"status": "completed",
					"output": map[string]interface{}{
						"result": map[string]interface{}{
							"summary": "observed api error spike",
						},
					},
				},
				map[string]interface{}{
					"tool":   "delivery.query",
					"status": "failed",
					"output": map[string]interface{}{
						"error": "delivery source unavailable",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("finalize diagnosis: %v", err)
	}
	if strings.Contains(result.Summary, "无直接关联") {
		t.Fatalf("expected overconfident summary to be rewritten, got %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "delivery.query 查询失败") {
		t.Fatalf("expected failure reason to be surfaced, got %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "无法确认") {
		t.Fatalf("expected cautious wording, got %q", result.Summary)
	}
}

func TestNormalizePlannerMetricsQueryRewritesLoadAlias(t *testing.T) {
	t.Parallel()

	got := normalizePlannerMetricsQuery(`1m:system_load_average{instance="192.168.3.106"}`, "192.168.3.106", "api")
	if got != `node_load1{instance="192.168.3.106"}` {
		t.Fatalf("expected load alias to normalize to node_load1, got %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
