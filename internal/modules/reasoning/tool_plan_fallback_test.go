package reasoning

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
)

func TestFallbackDiagnosisPlanBranches(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})
	desenseMap := map[string]string{"[HOST_1]": "host-1"}

	loadPlan := svc.fallbackDiagnosisPlan(contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "看一下过去一小时负载趋势",
		},
	}, desenseMap)
	if !strings.Contains(loadPlan.Summary, "最近一小时") {
		t.Fatalf("expected load fallback summary, got %q", loadPlan.Summary)
	}
	if len(loadPlan.ToolPlan) != 1 || loadPlan.ToolPlan[0].Tool != "metrics.query_range" {
		t.Fatalf("expected metrics query fallback plan, got %+v", loadPlan.ToolPlan)
	}
	if got := loadPlan.ToolPlan[0].Input["query"]; got != `node_load1{instance="host-1"}` {
		t.Fatalf("expected default host metrics query, got %#v", got)
	}
	if loadPlan.DesenseMap["[HOST_1]"] != "host-1" {
		t.Fatalf("expected desense map to be preserved, got %+v", loadPlan.DesenseMap)
	}

	commandPlan := svc.fallbackDiagnosisPlan(contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "check disk usage",
		},
	}, nil)
	if len(commandPlan.ToolPlan) == 0 {
		t.Fatalf("expected read-only fallback plan, got %+v", commandPlan.ToolPlan)
	}
	if commandPlan.ToolPlan[0].Tool != "metrics.query_range" {
		t.Fatalf("expected disk fallback to start with metrics evidence, got %+v", commandPlan.ToolPlan)
	}
	for _, step := range commandPlan.ToolPlan {
		if step.Tool == "execution.run_command" {
			t.Fatalf("expected disk fallback to defer host execution, got %+v", commandPlan.ToolPlan)
		}
	}

	defaultPlan := svc.fallbackDiagnosisPlan(contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"user_request": "please summarize the alert",
		},
	}, nil)
	if len(defaultPlan.ToolPlan) != 0 {
		t.Fatalf("expected no heuristic tools for summary-only request, got %+v", defaultPlan.ToolPlan)
	}
}

func TestFallbackDiagnosisPlanUsesMonitoringFirstReadOnlyPlanForRootCauseRequests(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})

	plan := svc.fallbackDiagnosisPlan(contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "最近 api 报错和最近一次发布有关系吗",
		},
	}, nil)

	if len(plan.ToolPlan) < 4 {
		t.Fatalf("expected fallback plan to gather read-only evidence, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[0].Tool != "metrics.query_range" {
		t.Fatalf("expected metrics evidence first, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[1].Tool != "logs.query" {
		t.Fatalf("expected logs evidence second, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[2].Tool != "observability.query" {
		t.Fatalf("expected observability evidence third, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[3].Tool != "delivery.query" {
		t.Fatalf("expected delivery correlation last, got %+v", plan.ToolPlan)
	}
	for _, step := range plan.ToolPlan {
		if step.Tool == "execution.run_command" {
			t.Fatalf("expected root-cause fallback to defer host execution, got %+v", plan.ToolPlan)
		}
	}
}

func TestFallbackDiagnosisPlanUsesMetricsLogsAndObservabilityForRootCauseRequests(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})

	plan := svc.fallbackDiagnosisPlan(contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "最近 api 报错，先看指标、日志和 trace 判断根因",
		},
	}, nil)

	if len(plan.ToolPlan) < 3 {
		t.Fatalf("expected multi-step read-only fallback, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[0].Tool != "metrics.query_range" {
		t.Fatalf("expected metrics evidence first, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[1].Tool != "logs.query" {
		t.Fatalf("expected logs evidence second, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[2].Tool != "observability.query" {
		t.Fatalf("expected observability evidence third, got %+v", plan.ToolPlan)
	}
	for _, step := range plan.ToolPlan {
		if step.Tool == "execution.run_command" {
			t.Fatalf("expected root-cause fallback to remain read-only, got %+v", plan.ToolPlan)
		}
	}
}

func TestFallbackDiagnosisPlanAppendsDeliveryForReleaseCorrelationRequests(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})

	plan := svc.fallbackDiagnosisPlan(contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "最近 api 报错和最近一次发布有关系吗",
		},
	}, nil)

	if len(plan.ToolPlan) < 4 {
		t.Fatalf("expected release-correlation fallback, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[0].Tool != "metrics.query_range" {
		t.Fatalf("expected metrics evidence first, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[1].Tool != "logs.query" {
		t.Fatalf("expected logs evidence second, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[2].Tool != "observability.query" {
		t.Fatalf("expected observability evidence third, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[3].Tool != "delivery.query" {
		t.Fatalf("expected delivery evidence last, got %+v", plan.ToolPlan)
	}
}

func TestInvokeDiagnosisModelJSONWithFallbackUsesAssistWhenPrimaryMissing(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host != "assist-model.example.test" {
				t.Fatalf("expected assist host, got %s", req.URL.Host)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"assist planner\",\"tool_plan\":[{\"tool\":\"knowledge.search\",\"reason\":\"look up guidance\",\"priority\":1,\"params\":{\"query\":\"disk cleanup\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{Client: client})
	response := plannerModelResponse{}
	usedRuntime, err := svc.invokeDiagnosisModelJSONWithFallback(
		context.Background(),
		"planner",
		contracts.DiagnosisInput{SessionID: "ses-planner-assist", Context: map[string]interface{}{"user_request": "check disk"}},
		contracts.DiagnosisInput{SessionID: "ses-planner-assist", Context: map[string]interface{}{"user_request": "check disk"}},
		plannerSystemPrompt,
		modelRuntime{},
		&modelRuntime{
			Role:     "assist",
			Protocol: ModelProtocolOpenAICompatible,
			BaseURL:  "https://assist-model.example.test",
			Model:    "gpt-4o-mini",
		},
		&response,
	)
	if err != nil {
		t.Fatalf("invokeDiagnosisModelJSONWithFallback: %v", err)
	}
	if usedRuntime.Role != "assist" {
		t.Fatalf("expected assist runtime, got %+v", usedRuntime)
	}
	if response.Summary != "assist planner" || len(response.ToolPlan) != 1 {
		t.Fatalf("unexpected decoded planner response: %+v", response)
	}
}

func TestPlanDiagnosisUsesAssistWhenPrimaryPlannerPayloadFails(t *testing.T) {
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
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"choices":[{"message":{"content":"{\"summary\":\"assist planner summary\",\"tool_plan\":[{\"tool\":\"execution.run_command\",\"reason\":\"collect host evidence\",\"priority\":1,\"params\":{\"command\":\"df -h\"}}]}"}}]
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

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-assist",
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "check memory usage",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if plan.Summary != "assist planner summary" {
		t.Fatalf("unexpected plan summary: %s", plan.Summary)
	}
	if len(plan.ToolPlan) != 1 || plan.ToolPlan[0].Tool != "metrics.query_instant" {
		t.Fatalf("expected generic execution assist plan to fall back to read-only evidence, got %+v", plan.ToolPlan)
	}
	if got := plan.ToolPlan[0].Input["query"]; got != `node_load1{instance="host-1"}` {
		t.Fatalf("expected read-only fallback query, got %#v", got)
	}
}

func TestPlanDiagnosisFallsBackToHeuristicPlanOnPlannerFailure(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"not-json"}}]}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://primary-model.example.test",
		Model:   "gpt-4o-mini",
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-fallback",
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "看一下过去一小时负载趋势",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if len(plan.ToolPlan) != 1 || plan.ToolPlan[0].Tool != "metrics.query_range" {
		t.Fatalf("expected heuristic metrics fallback plan, got %+v", plan.ToolPlan)
	}
}

func TestFinalizeDiagnosisFallsBackWhenFinalizerFails(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"gateway down"}}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:                     client,
		BaseURL:                    "https://primary-model.example.test",
		Model:                      "gpt-4o-mini",
		LocalCommandFallbackEnable: true,
	})

	result, err := svc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-finalizer-fallback",
		Context: map[string]interface{}{
			"host":       "host-1",
			"service":    "api",
			"alert_name": "HighCPU",
			"severity":   "critical",
		},
	})
	if err != nil {
		t.Fatalf("finalize diagnosis: %v", err)
	}
	if !strings.Contains(result.Summary, "HighCPU on host-1") {
		t.Fatalf("expected fallback summary, got %q", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected generic finalizer fallback to avoid execution hint, got %s", result.ExecutionHint)
	}
}

func TestFinalizeDiagnosisUsesFallbackSummaryWhenModelSummaryIsBlank(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"   \",\"execution_hint\":\"metrics.query_range\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://primary-model.example.test",
		Model:   "gpt-4o-mini",
	})

	result, err := svc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-finalizer-blank-summary",
		Context: map[string]interface{}{
			"host":         "host-1",
			"user_request": "检查磁盘使用率",
		},
	})
	if err != nil {
		t.Fatalf("finalize diagnosis: %v", err)
	}
	if !strings.Contains(result.Summary, "已分析请求：检查磁盘使用率") {
		t.Fatalf("expected fallback summary when model summary is blank, got %q", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected tool-style execution hint to be suppressed, got %q", result.ExecutionHint)
	}
}

func TestFinalizeDiagnosisUsesEvidenceAwareFallbackSummaryWhenFinalizerFails(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"gateway down"}}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://primary-model.example.test",
		Model:   "gpt-4o-mini",
	})

	result, err := svc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-finalizer-evidence-fallback",
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"service":      "api",
			"user_request": "最近 api 报错和最近一次发布有关系吗",
			"tool_results": []interface{}{
				map[string]interface{}{
					"tool":   "metrics.query_range",
					"status": "completed",
					"output": map[string]interface{}{"result_count": 1, "summary": "load remained stable"},
				},
				map[string]interface{}{
					"tool":   "logs.query",
					"status": "completed",
					"output": map[string]interface{}{"result_count": 3, "summary": "matched api errors after release"},
				},
				map[string]interface{}{
					"tool":   "observability.query",
					"status": "completed",
					"output": map[string]interface{}{"result_count": 2, "summary": "trace latency spiked after release 2026.03.20-1"},
				},
				map[string]interface{}{
					"tool":   "delivery.query",
					"status": "completed",
					"output": map[string]interface{}{"result_count": 2, "summary": "release 2026.03.20-1 and rollback found"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("finalize diagnosis: %v", err)
	}
	if strings.Contains(result.Summary, "已分析请求：") {
		t.Fatalf("expected evidence-aware fallback summary, got %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "metrics.query_range") || !strings.Contains(result.Summary, "logs.query") || !strings.Contains(result.Summary, "delivery.query") {
		t.Fatalf("expected fallback summary to mention completed evidence tools, got %q", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected no execution hint when evidence already answers, got %q", result.ExecutionHint)
	}
}

func TestFallbackSummaryFromEvidencePrefersNestedCapabilitySummaries(t *testing.T) {
	t.Parallel()

	ctx := map[string]interface{}{
		"tool_results": []interface{}{
			map[string]interface{}{
				"tool":   "logs.query",
				"status": "completed",
				"output": map[string]interface{}{
					"artifact_count": 1,
					"result": map[string]interface{}{
						"result_count": 1,
						"summary":      "matched 1 shared host-file log marker",
					},
				},
			},
			map[string]interface{}{
				"tool":   "observability.query",
				"status": "completed",
				"output": map[string]interface{}{
					"artifact_count": 1,
					"result": map[string]interface{}{
						"result_count": 2,
						"summary":      "returned 2 alert(s), 2 firing",
					},
				},
			},
			map[string]interface{}{
				"tool":   "delivery.query",
				"status": "completed",
				"output": map[string]interface{}{
					"artifact_count": 1,
					"result": map[string]interface{}{
						"result_count": 5,
						"summary":      "delivery facts from github repo VictoriaMetrics/VictoriaMetrics",
					},
				},
			},
		},
	}

	got := fallbackSummaryFromEvidence(ctx, "fallback summary")
	if strings.Contains(got, "artifact_count=1") {
		t.Fatalf("expected nested capability summaries instead of attachment counts, got %q", got)
	}
	for _, fragment := range []string{
		"matched 1 shared host-file log marker",
		"returned 2 alert(s), 2 firing",
		"delivery facts from github repo VictoriaMetrics/VictoriaMetrics",
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("expected fallback summary to include %q, got %q", fragment, got)
		}
	}
}

func TestFinalizeDiagnosisUsesAutomationEvidenceFallbackWhenFinalizerFails(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"gateway down"}}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://primary-model.example.test",
		Model:   "gpt-4o-mini",
	})

	result, err := svc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-finalizer-automation-fallback",
		Context: map[string]interface{}{
			"automation_run":      true,
			"automation_job_id":   "golden-inspection-victoriametrics",
			"automation_job_type": "connector_capability",
			"run_status":          "completed",
			"run_summary":         "metrics query returned 1 series",
			"user_request":        "Summarize this automation run and only produce execution_hint if a human still needs to take an action.",
			"capability_output": map[string]interface{}{
				"summary":      "metrics query returned 1 series",
				"result_count": 1,
				"query_params": map[string]interface{}{"query": "up"},
			},
		},
	})
	if err != nil {
		t.Fatalf("finalize diagnosis: %v", err)
	}
	if strings.Contains(result.Summary, "已分析请求：") {
		t.Fatalf("expected automation evidence fallback summary, got %q", result.Summary)
	}
	if !strings.Contains(strings.ToLower(result.Summary), "automation") {
		t.Fatalf("expected automation fallback summary to mention automation evidence, got %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "metrics query returned 1 series") {
		t.Fatalf("expected automation fallback summary to surface capability summary, got %q", result.Summary)
	}
}

func TestNormalizePlannerToolPlanEnforcesOfficialDiskPlan(t *testing.T) {
	t.Parallel()

	input := contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "disk usage is climbing fast",
			"tool_capabilities": []interface{}{
				map[string]interface{}{
					"tool":         "metrics.query_range",
					"connector_id": "metrics-prom",
					"invocable":    true,
				},
			},
		},
	}

	steps := normalizePlannerToolPlan(input, []plannerToolPlanEntry{
		{
			ID:       "probe_existing",
			Tool:     "execution.run_command",
			Reason:   "run df after metrics",
			Priority: 7,
			Input:    map[string]interface{}{"command": "df -h"},
		},
	})

	if len(steps) != 3 {
		t.Fatalf("expected official disk plan without generic execution probe, got %+v", steps)
	}
	if steps[0].ID != "metrics_capacity" || steps[1].ID != "metrics_forecast" || steps[2].ID != "knowledge_disk" {
		t.Fatalf("expected official disk plan ordering, got %+v", steps)
	}
	if steps[0].ConnectorID != "metrics-prom" {
		t.Fatalf("expected metrics connector to be defaulted from capabilities, got %+v", steps[0])
	}
	for _, step := range steps {
		if step.Tool == "execution.run_command" {
			t.Fatalf("expected generic disk execution probe to be dropped, got %+v", steps)
		}
	}
}

func TestNormalizePlannerToolPlanKeepsUserPlanWhenSkillAlreadySelected(t *testing.T) {
	t.Parallel()

	input := contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "disk cleanup guidance",
			"skill_match":  map[string]interface{}{"skill_id": "official-disk-skill"},
			"tool_capabilities": []interface{}{
				map[string]interface{}{
					"tool":          "connector.invoke_capability",
					"connector_id":  "skill-connector",
					"capability_id": "skill.execute",
					"invocable":     true,
				},
			},
		},
	}

	steps := normalizePlannerToolPlan(input, []plannerToolPlanEntry{
		{Tool: " "},
		{
			Tool:     "connector.invoke_capability",
			Reason:   "use the selected skill",
			Priority: 1,
			Params: map[string]interface{}{
				"capability": "skill.execute",
			},
		},
		{
			Tool:              "execution.run_command",
			Reason:            "collect host evidence",
			Priority:          2,
			Params:            map[string]interface{}{"command": " df -h "},
			OnFailure:         "invalid",
			OnPendingApproval: "invalid",
			OnDenied:          "invalid",
		},
		{
			Tool:   "execution.run_command",
			Reason: "missing command should be skipped",
		},
	})

	if len(steps) != 1 {
		t.Fatalf("expected generic execution step to be removed, got %+v", steps)
	}
	if steps[0].Tool != "connector.invoke_capability" || steps[0].ConnectorID != "skill-connector" {
		t.Fatalf("expected connector step to survive without disk-plan override, got %+v", steps[0])
	}
	if got := steps[0].Input["capability_id"]; got != "skill.execute" {
		t.Fatalf("expected capability field to normalize into capability_id, got %#v", got)
	}
	if steps[0].OnFailure != "continue" || steps[0].OnPendingApproval != "stop" || steps[0].OnDenied != "continue" {
		t.Fatalf("expected policy fallbacks to apply, got %+v", steps[0])
	}
}

func TestFallbackDiagnosisPlanNormalizesLogsQueryForRootCauseRequests(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})

	plan := svc.fallbackDiagnosisPlan(contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"service":      "api",
			"user_request": "先看 api 错误日志，确认最近有没有异常 evidence",
		},
	}, nil)

	if len(plan.ToolPlan) < 2 {
		t.Fatalf("expected multi-step fallback plan, got %+v", plan.ToolPlan)
	}
	logsStep := plan.ToolPlan[1]
	if logsStep.Tool != "logs.query" {
		t.Fatalf("expected second fallback step to be logs.query, got %+v", logsStep)
	}
	query := interfaceString(logsStep.Input["query"])
	if strings.TrimSpace(query) == "" {
		t.Fatalf("expected logs query to be populated, got empty step %+v", logsStep)
	}
	if query == "先看 api 错误日志，确认最近有没有异常 evidence" {
		t.Fatalf("expected logs query to normalize natural language, got %q", query)
	}
	if strings.Contains(query, "，") {
		t.Fatalf("expected logs query to avoid raw chinese punctuation, got %q", query)
	}
	if !strings.Contains(query, "api") {
		t.Fatalf("expected logs query to preserve service hint, got %q", query)
	}
	if !strings.Contains(strings.ToLower(query), "error") {
		t.Fatalf("expected logs query to preserve error intent, got %q", query)
	}
}

func TestNormalizePlannerToolPlanNormalizesLogsQueryDefaults(t *testing.T) {
	t.Parallel()

	input := contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "192.168.3.106",
			"service":      "api",
			"user_request": "先看 api 错误日志，确认最近有没有异常 evidence",
		},
	}

	steps := normalizePlannerToolPlan(input, []plannerToolPlanEntry{{
		Tool: "logs.query",
	}})

	if len(steps) != 1 {
		t.Fatalf("expected one normalized step, got %+v", steps)
	}
	query := interfaceString(steps[0].Input["query"])
	if strings.TrimSpace(query) == "" {
		t.Fatalf("expected normalized logs query, got empty step %+v", steps[0])
	}
	if query == "先看 api 错误日志，确认最近有没有异常 evidence" {
		t.Fatalf("expected normalized logs query instead of raw summary, got %q", query)
	}
	if strings.Contains(query, "，") {
		t.Fatalf("expected normalized logs query to avoid raw chinese punctuation, got %q", query)
	}
	if !strings.Contains(query, "api") || !strings.Contains(strings.ToLower(query), "error") {
		t.Fatalf("expected normalized logs query to carry service/error hints, got %q", query)
	}
}

func TestNormalizePlannerLogsQueryUsesCrossFieldServiceSearch(t *testing.T) {
	t.Parallel()

	query := normalizePlannerLogsQuery("先看 api 错误日志，确认最近有没有异常 evidence", "api")

	if !strings.Contains(query, `"api"`) {
		t.Fatalf("expected normalized logs query to search service hint as a plain LogsQL term, got %q", query)
	}
	if strings.Contains(query, `service:"api"`) {
		t.Fatalf("expected normalized logs query to avoid exact service field dependency, got %q", query)
	}
}

func TestNormalizePlannerToolPlanUsesLongTimeRangeForSharedLogsMarker(t *testing.T) {
	t.Parallel()

	input := contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "192.168.3.100:9100",
			"service":      "api",
			"user_request": "先看 tars-observability-host-file-test 共享机日志 marker，确认 logs evidence path",
		},
	}

	steps := normalizePlannerToolPlan(input, []plannerToolPlanEntry{{
		Tool: "logs.query",
	}})

	if len(steps) != 1 {
		t.Fatalf("expected one normalized logs step, got %+v", steps)
	}
	if got := interfaceString(steps[0].Input["query"]); got != "tars-observability-host-file-test" {
		t.Fatalf("expected marker query to be preserved, got %q", got)
	}
	if got := interfaceString(steps[0].Input["time_range"]); got != "168h" {
		t.Fatalf("expected shared marker logs query to widen time_range, got %q", got)
	}
}

func TestNormalizeFinalSummaryForEvidenceAndFailureHelpers(t *testing.T) {
	t.Parallel()

	ctx := map[string]interface{}{
		"user_request": "is this related to the latest deploy?",
		"tool_results": []interface{}{
			map[string]interface{}{"tool": "delivery.query", "status": "failed"},
			map[string]interface{}{"tool": "observability.query", "status": "failed"},
			map[string]interface{}{"tool": "execution.run_command", "status": "failed"},
		},
	}

	failures := collectCriticalToolFailures(ctx)
	if len(failures) != 2 {
		t.Fatalf("expected only critical evidence failures, got %+v", failures)
	}
	if !isCriticalEvidenceTool("metrics.query_range") || isCriticalEvidenceTool("execution.run_command") {
		t.Fatalf("unexpected critical tool classification")
	}
	if !summaryContainsUnsupportedConclusion("This is not related to the deployment") {
		t.Fatalf("expected unsupported conclusion phrase to be detected")
	}

	got := normalizeFinalSummaryForEvidence(ctx, "This is not related to the deployment")
	if !strings.Contains(got, "delivery.query 查询失败") || !strings.Contains(got, "observability.query 查询失败") {
		t.Fatalf("expected cautious summary to mention failed evidence sources, got %q", got)
	}
	if !strings.Contains(got, "无法确认是否与最近一次发布或变更相关") {
		t.Fatalf("expected release-oriented caution in summary, got %q", got)
	}
	if normalized := normalizeFinalSummaryForEvidence(ctx, "Need more data"); normalized != "Need more data" {
		t.Fatalf("expected non-overconfident summary to pass through, got %q", normalized)
	}
}

func TestToolCapabilityHelpersAndNestedRehydration(t *testing.T) {
	t.Parallel()

	ctx := map[string]interface{}{
		"tool_capabilities": []map[string]interface{}{
			{
				"tool":          "connector.invoke_capability",
				"connector_id":  "skill-1",
				"capability_id": "skill.execute",
				"invocable":     "true",
			},
			{
				"tool":         "observability.query",
				"connector_id": "obs-1",
				"action":       "query",
				"invocable":    true,
			},
			{
				"tool":         "delivery.query",
				"connector_id": "delivery-1",
				"action":       "query",
				"invocable":    true,
			},
		},
	}

	if got := defaultConnectorIDForTool(ctx, "connector.invoke_capability", "skill.execute"); got != "skill-1" {
		t.Fatalf("expected capability connector, got %q", got)
	}
	if got := defaultConnectorIDForTool(ctx, "observability.query", ""); got != "obs-1" {
		t.Fatalf("expected observability connector, got %q", got)
	}
	if got := defaultConnectorIDForTool(ctx, "delivery.query", ""); got != "delivery-1" {
		t.Fatalf("expected delivery connector, got %q", got)
	}

	cfg := DefaultDesensitizationConfig()
	cfg.Rehydration.Path = false
	value := rehydrateInterfaceValueWithConfig([]interface{}{
		"[HOST_1]",
		map[string]interface{}{"path": "[PATH_1]", "ip": "[IP_1]"},
	}, map[string]string{
		"[HOST_1]": "node-a",
		"[PATH_1]": "/srv/app/config.yaml",
		"[IP_1]":   "10.0.0.8",
	}, &cfg)

	items, ok := value.([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("expected nested rehydrated array, got %#v", value)
	}
	if items[0] != "node-a" {
		t.Fatalf("expected host placeholder to rehydrate, got %#v", items[0])
	}
	nested, _ := items[1].(map[string]interface{})
	if nested["path"] != "[PATH_1]" || nested["ip"] != "10.0.0.8" {
		t.Fatalf("expected path to stay masked and ip to rehydrate, got %#v", nested)
	}
}

func TestParsePlannerPriorityAndQueryNormalizationHelpers(t *testing.T) {
	t.Parallel()

	if got := parsePlannerPriority(json.RawMessage(`"4"`)); got != 4 {
		t.Fatalf("expected quoted integer priority, got %d", got)
	}
	if got := parsePlannerPriority(json.RawMessage(`3.0`)); got != 3 {
		t.Fatalf("expected float priority to coerce to int, got %d", got)
	}
	if got := injectPromQLMatcher("node_load1", `instance="host-1"`); got != `node_load1{instance="host-1"}` {
		t.Fatalf("expected matcher injection, got %q", got)
	}
	if got := injectPromQLMatcher("node_load1{}", `instance="host-1"`); got != `node_load1{instance="host-1"}` {
		t.Fatalf("expected empty selector matcher injection, got %q", got)
	}
	if got := injectPromQLMatcher(`node_load1{job="api"}`, `instance="host-1"`); got != `node_load1{job="api",instance="host-1"}` {
		t.Fatalf("expected matcher append inside selector, got %q", got)
	}
	if got := injectPromQLMatcher("", `instance="host-1"`); got != "" {
		t.Fatalf("expected empty query to remain empty, got %q", got)
	}
	if got := normalizePlannerMetricsQuery("node_cpu_seconds_total", "host-1", ""); got != `node_cpu_seconds_total{instance="host-1"}` {
		t.Fatalf("expected host matcher injection, got %q", got)
	}
}

func TestPlanDiagnosisUsesFallbackSummaryAndPlanWhenModelReturnsEmptyPayload(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"   \",\"tool_plan\":[]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://planner.example.test",
		Model:   "gpt-4o-mini",
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-empty-planner-payload",
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "check disk usage",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if strings.TrimSpace(plan.Summary) == "" {
		t.Fatalf("expected fallback summary to replace blank planner summary")
	}
	if len(plan.ToolPlan) == 0 || plan.ToolPlan[0].Tool != "metrics.query_range" {
		t.Fatalf("expected fallback tool plan to start with read-only metrics evidence, got %+v", plan.ToolPlan)
	}
	for _, step := range plan.ToolPlan {
		if step.Tool == "execution.run_command" {
			t.Fatalf("expected fallback plan to defer host execution, got %+v", plan.ToolPlan)
		}
	}
}

func TestFinalizeDiagnosisUsesAssistWhenPrimaryFinalizerFails(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Host {
			case "primary-finalizer.example.test":
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"gateway down"}}`)),
				}, nil
			case "assist-finalizer.example.test":
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"choices":[{"message":{"content":"{\"summary\":\"assist finalizer summary\",\"execution_hint\":\"free -m\"}"}}]
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
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "primary-openai",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://primary-finalizer.example.test",
				Model:      "gpt-4o-mini",
			},
			assist: &ModelTarget{
				ProviderID: "assist-openai",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://assist-finalizer.example.test",
				Model:      "gpt-4o-mini-backup",
			},
		},
	})

	result, err := svc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-finalizer-assist",
		Context: map[string]interface{}{
			"user_request": "check memory",
		},
	})
	if err != nil {
		t.Fatalf("finalize diagnosis: %v", err)
	}
	if result.Summary != "assist finalizer summary" || result.ExecutionHint != "" {
		t.Fatalf("expected assist finalizer result, got %+v", result)
	}
}

func TestFinalizeDiagnosisUsesFallbackWhenModelIsNotConfigured(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{LocalCommandFallbackEnable: true})
	result, err := svc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-finalizer-stub",
		Context: map[string]interface{}{
			"alert_name": "HighMemory",
			"host":       "host-1",
			"service":    "api",
			"severity":   "critical",
		},
	})
	if err != nil {
		t.Fatalf("finalize diagnosis: %v", err)
	}
	if !strings.Contains(result.Summary, "HighMemory on host-1") {
		t.Fatalf("expected stub fallback summary, got %q", result.Summary)
	}
	if result.ExecutionHint != "" {
		t.Fatalf("expected stub fallback to avoid generic execution hint, got %q", result.ExecutionHint)
	}
}

func TestPlanDiagnosisDropsGenericExecutionPlanWithoutExplicitCommandRequest(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"planner summary\",\"tool_plan\":[{\"tool\":\"execution.run_command\",\"reason\":\"collect host evidence\",\"priority\":1,\"params\":{\"command\":\"hostname && uptime\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://planner.example.test",
		Model:   "gpt-4o-mini",
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-drop-generic-exec",
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "please diagnose this alert",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	for _, step := range plan.ToolPlan {
		if step.Tool == "execution.run_command" {
			t.Fatalf("expected generic diagnosis not to keep execution.run_command, got %+v", plan.ToolPlan)
		}
	}
}

func TestPlanDiagnosisKeepsExplicitChineseExecutionPlanAndRequiresApproval(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"planner summary\",\"tool_plan\":[{\"tool\":\"execution.run_command\",\"reason\":\"operator explicitly asked to restart\",\"priority\":1,\"params\":{\"command\":\"systemctl restart api\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://planner.example.test",
		Model:   "gpt-4o-mini",
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-explicit-cn-exec",
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "执行命令重启 api",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if len(plan.ToolPlan) != 1 || plan.ToolPlan[0].Tool != "execution.run_command" {
		t.Fatalf("expected explicit command request to preserve execution step, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[0].OnPendingApproval != "stop" {
		t.Fatalf("expected execution step to require approval boundary, got %+v", plan.ToolPlan[0])
	}
}

func TestPlanDiagnosisKeepsExplicitEnglishExecutionPlanAndRequiresApproval(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"planner summary\",\"tool_plan\":[{\"tool\":\"execution.run_command\",\"reason\":\"operator explicitly asked to check service status\",\"priority\":1,\"params\":{\"command\":\"systemctl status api --no-pager --lines=20 || true\"}}]}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Client:  client,
		BaseURL: "https://planner.example.test",
		Model:   "gpt-4o-mini",
	})

	plan, err := svc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-explicit-en-exec",
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "run status command for api service",
		},
	})
	if err != nil {
		t.Fatalf("plan diagnosis: %v", err)
	}
	if len(plan.ToolPlan) != 1 || plan.ToolPlan[0].Tool != "execution.run_command" {
		t.Fatalf("expected explicit command request to preserve execution step, got %+v", plan.ToolPlan)
	}
	if plan.ToolPlan[0].OnPendingApproval != "stop" {
		t.Fatalf("expected execution step to require approval boundary, got %+v", plan.ToolPlan[0])
	}
}

func TestFinalizeDiagnosisRecordsStubErrorAndSuccessMetrics(t *testing.T) {
	t.Parallel()

	stubRegistry := foundationmetrics.New()
	stubSvc := NewService(Options{Metrics: stubRegistry})
	if _, err := stubSvc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-finalizer-metrics-stub",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	}); err != nil {
		t.Fatalf("finalize diagnosis stub: %v", err)
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
		BaseURL: "https://finalizer-error.example.test",
		Model:   "gpt-4o-mini",
	})
	if _, err := errorSvc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-finalizer-metrics-error",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	}); err != nil {
		t.Fatalf("finalize diagnosis error: %v", err)
	}
	var errorOutput strings.Builder
	if err := errorRegistry.WritePrometheus(&errorOutput); err != nil {
		t.Fatalf("write error metrics: %v", err)
	}
	if !strings.Contains(errorOutput.String(), `tars_external_provider_requests_total{operation="chat_completions",provider="model",result="error"} 1`) {
		t.Fatalf("expected error metric, got:\n%s", errorOutput.String())
	}

	successRegistry := foundationmetrics.New()
	successSvc := NewService(Options{
		Metrics: successRegistry,
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"choices":[{"message":{"content":"{\"summary\":\"ok\",\"execution_hint\":\"\"}"}}]
					}`)),
				}, nil
			}),
		},
		BaseURL: "https://finalizer-success.example.test",
		Model:   "gpt-4o-mini",
	})
	if _, err := successSvc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-finalizer-metrics-success",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	}); err != nil {
		t.Fatalf("finalize diagnosis success: %v", err)
	}
	var successOutput strings.Builder
	if err := successRegistry.WritePrometheus(&successOutput); err != nil {
		t.Fatalf("write success metrics: %v", err)
	}
	if !strings.Contains(successOutput.String(), `tars_external_provider_requests_total{operation="chat_completions",provider="model",result="success"} 1`) {
		t.Fatalf("expected success metric, got:\n%s", successOutput.String())
	}
}

func TestPlanDiagnosisAndFinalizeDiagnosisCoverMetricSuccessBranches(t *testing.T) {
	t.Parallel()

	primaryRegistry := foundationmetrics.New()
	primaryPlanSvc := NewService(Options{
		Metrics: primaryRegistry,
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "planner-primary.example.test" {
					t.Fatalf("unexpected primary planner host: %s", req.URL.Host)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"choices":[{"message":{"content":"{\"summary\":\"primary plan\",\"tool_plan\":[{\"tool\":\"knowledge.search\",\"reason\":\"lookup\",\"priority\":1,\"params\":{\"query\":\"cpu\"}}]}"}}]
					}`)),
				}, nil
			}),
		},
		BaseURL: "https://planner-primary.example.test",
		Model:   "gpt-4o-mini",
	})
	primaryFinalSvc := NewService(Options{
		Metrics: primaryRegistry,
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "finalizer-primary.example.test" {
					t.Fatalf("unexpected primary finalizer host: %s", req.URL.Host)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"choices":[{"message":{"content":"{\"summary\":\"primary final\",\"execution_hint\":\"\"}"}}]
					}`)),
				}, nil
			}),
		},
		BaseURL: "https://finalizer-primary.example.test",
		Model:   "gpt-4o-mini",
	})
	primaryPlan, err := primaryPlanSvc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-metric-primary",
		Context:   map[string]interface{}{"user_request": "check cpu"},
	})
	if err != nil {
		t.Fatalf("plan diagnosis primary metrics case: %v", err)
	}
	if primaryPlan.Summary != "primary plan" {
		t.Fatalf("unexpected primary planner summary: %+v", primaryPlan)
	}
	primaryFinal, err := primaryFinalSvc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-final-metric-primary",
		Context:   map[string]interface{}{"user_request": "check cpu"},
	})
	if err != nil {
		t.Fatalf("finalize diagnosis primary metrics case: %v", err)
	}
	if primaryFinal.Summary != "primary final" {
		t.Fatalf("unexpected primary finalizer result: %+v", primaryFinal)
	}
	var primaryMetrics strings.Builder
	if err := primaryRegistry.WritePrometheus(&primaryMetrics); err != nil {
		t.Fatalf("write primary metrics: %v", err)
	}
	primaryOutput := primaryMetrics.String()
	if !strings.Contains(primaryOutput, `tars_external_provider_requests_total{operation="chat_completions",provider="model_primary",result="success"} 2`) {
		t.Fatalf("expected two primary-model success attempts, got:\n%s", primaryOutput)
	}
	if !strings.Contains(primaryOutput, `tars_external_provider_requests_total{operation="chat_completions",provider="model",result="success"} 1`) {
		t.Fatalf("expected finalizer success metric, got:\n%s", primaryOutput)
	}
	if strings.Contains(primaryOutput, `provider="model_assist",result="success"`) {
		t.Fatalf("did not expect assist success metrics in primary-only case, got:\n%s", primaryOutput)
	}

	assistRegistry := foundationmetrics.New()
	assistPlanSvc := NewService(Options{
		Metrics: assistRegistry,
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Host {
				case "planner-primary-fail.example.test":
					return &http.Response{
						StatusCode: http.StatusBadGateway,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"gateway down"}}`)),
					}, nil
				case "planner-assist.example.test":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"choices":[{"message":{"content":"{\"summary\":\"assist plan\",\"tool_plan\":[{\"tool\":\"knowledge.search\",\"reason\":\"lookup\",\"priority\":1,\"params\":{\"query\":\"memory\"}}]}"}}]
						}`)),
					}, nil
				default:
					t.Fatalf("unexpected host: %s", req.URL.Host)
					return nil, nil
				}
			}),
		},
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "primary-openai",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://planner-primary-fail.example.test",
				Model:      "gpt-4o-mini",
			},
			assist: &ModelTarget{
				ProviderID: "assist-openai",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://planner-assist.example.test",
				Model:      "gpt-4o-mini-backup",
			},
		},
	})
	assistFinalSvc := NewService(Options{
		Metrics: assistRegistry,
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Host {
				case "finalizer-primary-fail.example.test":
					return &http.Response{
						StatusCode: http.StatusBadGateway,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"gateway down"}}`)),
					}, nil
				case "finalizer-assist.example.test":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"choices":[{"message":{"content":"{\"summary\":\"assist final\",\"execution_hint\":\"free -m\"}"}}]
						}`)),
					}, nil
				default:
					t.Fatalf("unexpected host: %s", req.URL.Host)
					return nil, nil
				}
			}),
		},
		ProviderRegistry: staticProviderRegistry{
			primary: &ModelTarget{
				ProviderID: "primary-openai",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://finalizer-primary-fail.example.test",
				Model:      "gpt-4o-mini",
			},
			assist: &ModelTarget{
				ProviderID: "assist-openai",
				Protocol:   ModelProtocolOpenAICompatible,
				BaseURL:    "https://finalizer-assist.example.test",
				Model:      "gpt-4o-mini-backup",
			},
		},
	})
	assistPlan, err := assistPlanSvc.PlanDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-plan-metric-assist",
		Context:   map[string]interface{}{"user_request": "check memory"},
	})
	if err != nil {
		t.Fatalf("plan diagnosis assist metrics case: %v", err)
	}
	if assistPlan.Summary != "assist plan" {
		t.Fatalf("unexpected assist planner summary: %+v", assistPlan)
	}
	assistFinal, err := assistFinalSvc.FinalizeDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-final-metric-assist",
		Context:   map[string]interface{}{"user_request": "check memory"},
	})
	if err != nil {
		t.Fatalf("finalize diagnosis assist metrics case: %v", err)
	}
	if assistFinal.Summary != "assist final" || assistFinal.ExecutionHint != "" {
		t.Fatalf("unexpected assist finalizer result: %+v", assistFinal)
	}
	var assistMetrics strings.Builder
	if err := assistRegistry.WritePrometheus(&assistMetrics); err != nil {
		t.Fatalf("write assist metrics: %v", err)
	}
	assistOutput := assistMetrics.String()
	if !strings.Contains(assistOutput, `tars_external_provider_requests_total{operation="chat_completions",provider="model_primary",result="error"} 2`) {
		t.Fatalf("expected two primary-model errors before assist fallback, got:\n%s", assistOutput)
	}
	if !strings.Contains(assistOutput, `tars_external_provider_requests_total{operation="chat_completions",provider="model_assist",result="success"} 2`) {
		t.Fatalf("expected two assist-model successes, got:\n%s", assistOutput)
	}
	if !strings.Contains(assistOutput, `tars_external_provider_requests_total{operation="chat_completions",provider="model",result="success"} 1`) {
		t.Fatalf("expected finalizer success metric after assist fallback, got:\n%s", assistOutput)
	}
}

type stringerValue string

func (s stringerValue) String() string {
	return string(s)
}

func TestPlannerUtilityHelpers(t *testing.T) {
	t.Parallel()

	if got := firstNonEmpty("", " value ", "other"); got != "value" {
		t.Fatalf("expected first non-empty string, got %q", got)
	}
	if got := firstNonEmpty("", " ", ""); got != "" {
		t.Fatalf("expected empty result when all strings are blank, got %q", got)
	}
	if got := interfaceString(stringerValue("stringer")); got != "stringer" {
		t.Fatalf("expected stringer conversion, got %q", got)
	}
	if !interfaceBool("true") || interfaceBool("false") || !interfaceBool(true) || interfaceBool(1) {
		t.Fatalf("expected interfaceBool string parsing to work")
	}
	if got := normalizeToolPlanPolicy("stop", "continue"); got != "stop" {
		t.Fatalf("expected valid policy to be preserved, got %q", got)
	}

	steps := []contracts.ToolPlanStep{
		{Tool: "knowledge.search"},
		{Tool: "execution.run_command", Reason: "probe"},
	}
	if found := findExecutionStep(steps); found == nil || found.Reason != "probe" {
		t.Fatalf("expected execution step to be found, got %+v", found)
	}
	if found := findExecutionStep([]contracts.ToolPlanStep{{Tool: "metrics.query_range"}}); found != nil {
		t.Fatalf("expected nil when no execution step exists, got %+v", found)
	}
	if got := interfaceString(123); got != "" {
		t.Fatalf("expected unsupported interfaceString type to become empty, got %q", got)
	}
}

func TestIntFromEvidenceAllTypes(t *testing.T) {
	t.Parallel()

	if got := intFromEvidence(int(42)); got != 42 {
		t.Fatalf("expected 42 from int, got %d", got)
	}
	if got := intFromEvidence(int64(7)); got != 7 {
		t.Fatalf("expected 7 from int64, got %d", got)
	}
	if got := intFromEvidence(float64(3.9)); got != 3 {
		t.Fatalf("expected 3 from float64, got %d", got)
	}
	if got := intFromEvidence("hello"); got != 0 {
		t.Fatalf("expected 0 from string, got %d", got)
	}
	if got := intFromEvidence(nil); got != 0 {
		t.Fatalf("expected 0 from nil, got %d", got)
	}
}

func TestCompactEvidenceSummaryAllBranches(t *testing.T) {
	t.Parallel()

	// non-map input → ""
	if got := compactEvidenceSummary("plain string"); got != "" {
		t.Fatalf("expected empty for non-map, got %q", got)
	}

	// top-level "release" key
	if got := compactEvidenceSummary(map[string]interface{}{"release": "v1.2.3"}); got != "v1.2.3" {
		t.Fatalf("expected release value, got %q", got)
	}

	// top-level "branch" key (no release)
	if got := compactEvidenceSummary(map[string]interface{}{"branch": "main"}); got != "main" {
		t.Fatalf("expected branch value, got %q", got)
	}

	// nested result.release
	if got := compactEvidenceSummary(map[string]interface{}{
		"result": map[string]interface{}{"release": "v2.0"},
	}); got != "v2.0" {
		t.Fatalf("expected nested release, got %q", got)
	}

	// nested result.result_count
	if got := compactEvidenceSummary(map[string]interface{}{
		"result": map[string]interface{}{"result_count": 5},
	}); got != "result_count=5" {
		t.Fatalf("expected result_count from nested, got %q", got)
	}

	// nested result.artifact_count
	if got := compactEvidenceSummary(map[string]interface{}{
		"result": map[string]interface{}{"artifact_count": float64(3)},
	}); got != "artifact_count=3" {
		t.Fatalf("expected artifact_count from nested, got %q", got)
	}

	// points + series_count
	if got := compactEvidenceSummary(map[string]interface{}{
		"points": 100, "series_count": 2,
	}); got != "series_count=2, points=100" {
		t.Fatalf("expected series+points, got %q", got)
	}

	// just points
	if got := compactEvidenceSummary(map[string]interface{}{
		"points": int64(50),
	}); got != "points=50" {
		t.Fatalf("expected points only, got %q", got)
	}

	// just series_count (no points)
	if got := compactEvidenceSummary(map[string]interface{}{
		"series_count": 4,
	}); got != "series_count=4" {
		t.Fatalf("expected series_count only, got %q", got)
	}

	// top-level result_count (no nested result)
	if got := compactEvidenceSummary(map[string]interface{}{
		"result_count": float64(8),
	}); got != "result_count=8" {
		t.Fatalf("expected top-level result_count, got %q", got)
	}

	// top-level artifact_count (no nested result, no result_count)
	if got := compactEvidenceSummary(map[string]interface{}{
		"artifact_count": 6,
	}); got != "artifact_count=6" {
		t.Fatalf("expected top-level artifact_count, got %q", got)
	}

	// empty map → ""
	if got := compactEvidenceSummary(map[string]interface{}{}); got != "" {
		t.Fatalf("expected empty for empty map, got %q", got)
	}
}

func TestFallbackAutomationSummaryAllStatuses(t *testing.T) {
	t.Parallel()

	// automation_run=false → ""
	if got := fallbackAutomationSummaryFromEvidence(map[string]interface{}{
		"automation_run": false,
	}); got != "" {
		t.Fatalf("expected empty when not automation run, got %q", got)
	}

	// no result → ""
	if got := fallbackAutomationSummaryFromEvidence(map[string]interface{}{
		"automation_run": true,
	}); got != "" {
		t.Fatalf("expected empty when no result summary, got %q", got)
	}

	// status "blocked"
	got := fallbackAutomationSummaryFromEvidence(map[string]interface{}{
		"automation_run":  true,
		"capability_output": map[string]interface{}{"summary": "partial result"},
		"run_status":      "blocked",
	})
	if !strings.Contains(got, "blocked") {
		t.Fatalf("expected blocked status in summary, got %q", got)
	}

	// default (unknown) status
	got = fallbackAutomationSummaryFromEvidence(map[string]interface{}{
		"automation_run":  true,
		"capability_output": map[string]interface{}{"summary": "some result"},
		"run_status":      "unknown",
	})
	if !strings.Contains(got, "some result") {
		t.Fatalf("expected result in default status summary, got %q", got)
	}
}

func TestFallbackDiagnosisPlanWithDirectExecutionUserRequest(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})
	// "重启 api" + service "api" → buildDirectExecutionHintForUserRequest returns "systemctl restart api"
	plan := svc.fallbackDiagnosisPlan(contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "host-1",
			"service":      "api",
			"user_request": "重启 api",
		},
	}, nil)

	if len(plan.ToolPlan) != 1 {
		t.Fatalf("expected exactly 1 plan step, got %d: %+v", len(plan.ToolPlan), plan.ToolPlan)
	}
	if plan.ToolPlan[0].Tool != "execution.run_command" {
		t.Fatalf("expected execution.run_command, got %q", plan.ToolPlan[0].Tool)
	}
	cmd := interfaceString(plan.ToolPlan[0].Input["command"])
	if cmd != "systemctl restart api" {
		t.Fatalf("expected restart command, got %q", cmd)
	}
}

func TestFallbackDiagnosisPlanMetricsCurrentIntent(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})
	// "cpu memory usage" → classifyFallbackIntent returns "metrics_current" → metrics.query_instant
	plan := svc.fallbackDiagnosisPlan(contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":    "host-2",
			"service": "api",
			"summary": "cpu memory usage",
		},
	}, nil)

	if len(plan.ToolPlan) == 0 {
		t.Fatalf("expected at least 1 plan step")
	}
	if plan.ToolPlan[0].Tool != "metrics.query_instant" {
		t.Fatalf("expected metrics.query_instant, got %q", plan.ToolPlan[0].Tool)
	}
	if !strings.Contains(plan.Summary, "当前监控状态") {
		t.Fatalf("expected current-state summary, got %q", plan.Summary)
	}
}

func TestNormalizePlannerToolPlanMetricsQueryInstant(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})
	_ = svc // normalizePlannerToolPlan is a package-level function

	input := contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":    "host-3",
			"service": "web",
		},
	}
	steps := []plannerToolPlanEntry{{
		Tool: "metrics.query_instant",
		ID:   "m1",
	}}
	out := normalizePlannerToolPlan(input, steps)
	if len(out) != 1 {
		t.Fatalf("expected 1 step, got %d", len(out))
	}
	if out[0].Tool != "metrics.query_instant" {
		t.Fatalf("expected metrics.query_instant, got %q", out[0].Tool)
	}
	if out[0].Input["mode"] != "instant" {
		t.Fatalf("expected mode=instant, got %q", out[0].Input["mode"])
	}
	if out[0].Input["host"] != "host-3" {
		t.Fatalf("expected host to be filled, got %q", out[0].Input["host"])
	}
}

func TestNormalizePlannerToolPlanDiskPlanWithExplicitExecutionStep(t *testing.T) {
	t.Parallel()

	// user_request "disk 重启 api" + service "api" + command "systemctl restart api"
	// → shouldUseOfficialDiskPlan=true, executionHintExplicitlyAllowed=true
	// → enforceOfficialDiskPlan appends the execution step → 4 steps total
	input := contracts.DiagnosisInput{
		Context: map[string]interface{}{
			"host":         "host-4",
			"service":      "api",
			"user_request": "disk 重启 api",
		},
	}
	steps := []plannerToolPlanEntry{
		{
			Tool: "execution.run_command",
			Input: map[string]interface{}{
				"command": "systemctl restart api",
				"host":    "host-4",
				"service": "api",
			},
		},
	}
	out := normalizePlannerToolPlan(input, steps)
	// enforceOfficialDiskPlan: 3 standard disk steps + 1 execution step from input
	if len(out) != 4 {
		t.Fatalf("expected 4 steps (3 disk + 1 execution), got %d: %+v", len(out), out)
	}
	found := false
	for _, step := range out {
		if step.Tool == "execution.run_command" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected execution.run_command step in disk plan")
	}
}

func TestRehydrateToolPlanWithNilConfig(t *testing.T) {
	t.Parallel()

	steps := []contracts.ToolPlanStep{
		{
			Tool:        "metrics.query_range",
			ConnectorID: "[CONNECTOR_1]",
			Input:       map[string]interface{}{"host": "[HOST_1]"},
		},
	}
	mapping := map[string]string{"[HOST_1]": "real-host", "[CONNECTOR_1]": "prom-1"}

	// nil cfg should use DefaultDesensitizationConfig and not panic
	out := rehydrateToolPlanWithConfig(steps, mapping, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 step, got %d", len(out))
	}
	if out[0].ConnectorID != "prom-1" {
		t.Fatalf("expected connector_id rehydrated, got %q", out[0].ConnectorID)
	}
	if hostVal := interfaceString(out[0].Input["host"]); hostVal != "real-host" {
		t.Fatalf("expected host rehydrated, got %q", hostVal)
	}
}
