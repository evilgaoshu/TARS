package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"tars/internal/contracts"
	"tars/internal/modules/connectors"
	"tars/internal/modules/workflow"
)

func TestResolveToolPlanInputResolvesContextAndStepReferences(t *testing.T) {
	t.Parallel()

	session := contracts.SessionDetail{
		Alert: map[string]interface{}{
			"host": "host-1",
			"labels": map[string]interface{}{
				"service": "api",
			},
		},
	}
	context := map[string]interface{}{
		"planner_summary": "check api health",
	}
	executed := []contracts.ToolPlanStep{{
		ID: "step_1",
		ResolvedInput: map[string]interface{}{
			"query": "cpu",
		},
		Output: map[string]interface{}{
			"result": map[string]interface{}{
				"next_query": "error rate",
			},
		},
	}}

	resolved := resolveToolPlanInput(map[string]interface{}{
		"summary": "$context.planner_summary",
		"service": "$alert.labels.service",
		"query":   "$steps.step_1.output.result.next_query",
		"origin":  "$steps.step_1.input.query",
	}, session, context, executed)

	if resolved["summary"] != "check api health" {
		t.Fatalf("expected context reference, got %+v", resolved)
	}
	if resolved["service"] != "api" {
		t.Fatalf("expected alert reference, got %+v", resolved)
	}
	if resolved["query"] != "error rate" {
		t.Fatalf("expected step output reference, got %+v", resolved)
	}
	if resolved["origin"] != "cpu" {
		t.Fatalf("expected step input reference, got %+v", resolved)
	}
}

func TestExecuteToolPlanDefaultsMetricsConnectorByType(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "connectors.yaml")
	if err := os.WriteFile(path, []byte("connectors:\n  entries: []\n"), 0o644); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}
	manager, err := connectors.NewManager(path)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	cfg := connectors.Config{
		Entries: []connectors.Manifest{{
			APIVersion: "tars/v1",
			Kind:       "Connector",
			Metadata: connectors.Metadata{
				ID:      "prometheus-main",
				Name:    "prometheus-main",
				Version: "1.0.0",
				Vendor:  "prometheus",
			},
			Spec: connectors.Spec{
				Type:     "metrics",
				Protocol: "prometheus_http",
				Capabilities: []connectors.Capability{{
					ID:        "metrics.query",
					Action:    "query",
					ReadOnly:  true,
					Invocable: true,
				}},
			},
			Compatibility: connectors.Compatibility{
				TARSMajorVersions: []string{connectors.CurrentTARSMajorVersion},
			},
		}},
	}
	if err := manager.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	action := &capturingActionService{
		metricsResult: contracts.MetricsResult{
			Series: []map[string]interface{}{
				{"metric": "node_load1", "values": []interface{}{1.0, 2.0}},
			},
			Runtime: &contracts.RuntimeMetadata{
				Runtime:     "connector",
				ConnectorID: "prometheus-main",
				Protocol:    "prometheus_http",
			},
		},
	}
	dispatcher := &Dispatcher{
		action:     action,
		connectors: manager,
	}

	session := contracts.SessionDetail{
		SessionID: "ses-tool-plan-default-metrics-connector",
		Alert: map[string]interface{}{
			"host": "192.168.3.106",
		},
	}

	result, err := dispatcher.executeToolPlan(context.Background(), session, []contracts.ToolPlanStep{{
		ID:     "metrics_1",
		Tool:   "metrics.query_range",
		Status: "planned",
		Input: map[string]interface{}{
			"host":   "192.168.3.106",
			"query":  "node_load1",
			"mode":   "range",
			"window": "1h",
			"step":   "5m",
		},
	}})
	if err != nil {
		t.Fatalf("execute tool plan: %v", err)
	}
	if action.lastMetricsQuery.ConnectorID != "prometheus-main" {
		t.Fatalf("expected metrics query connector_id to default from connector type, got %+v", action.lastMetricsQuery)
	}
	if len(result.ExecutedPlan) != 1 {
		t.Fatalf("expected one executed step, got %+v", result.ExecutedPlan)
	}
	if result.ExecutedPlan[0].ResolvedInput["connector_id"] != "prometheus-main" {
		t.Fatalf("expected resolved_input connector_id to be backfilled, got %+v", result.ExecutedPlan[0].ResolvedInput)
	}
}

func TestExecuteToolPlanBuildsDiskAnalysisAttachment(t *testing.T) {
	t.Parallel()

	action := &capturingActionService{
		metricsResult: contracts.MetricsResult{
			Series: []map[string]interface{}{{
				"mountpoint": "/data",
				"device":     "/dev/sda1",
				"values": []interface{}{
					[]interface{}{float64(1710000000), "82"},
					[]interface{}{float64(1710003600), "88"},
				},
			}},
		},
	}
	dispatcher := &Dispatcher{action: action}
	session := contracts.SessionDetail{
		SessionID: "ses-disk-analysis",
		Alert: map[string]interface{}{
			"annotations": map[string]string{"summary": "disk usage high"},
			"labels":      map[string]string{"alertname": "DiskFull", "service": "api"},
		},
	}

	result, err := dispatcher.executeToolPlan(context.Background(), session, []contracts.ToolPlanStep{{
		ID:     "metrics_capacity",
		Tool:   "metrics.query_range",
		Status: "planned",
		Input: map[string]interface{}{
			"query":  "node_filesystem_usage_percent",
			"mode":   "range",
			"window": "1h",
			"step":   "5m",
		},
	}})
	if err != nil {
		t.Fatalf("execute tool plan: %v", err)
	}
	if len(result.Attachments) == 0 {
		t.Fatalf("expected attachments, got none")
	}
	found := false
	for _, item := range result.Attachments {
		if item.Name != "disk-space-analysis-metrics_capacity.json" {
			continue
		}
		found = true
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(item.Content), &payload); err != nil {
			t.Fatalf("decode disk attachment: %v", err)
		}
		if payload["mountpoint"] != "/data" {
			t.Fatalf("unexpected disk analysis payload: %+v", payload)
		}
	}
	if !found {
		t.Fatalf("expected disk analysis attachment, got %+v", result.Attachments)
	}
	if analysis, ok := result.Context["disk_space_analysis"].(map[string]interface{}); !ok || analysis["analysis_type"] != "usage_trend" {
		t.Fatalf("expected disk space analysis context, got %+v", result.Context["disk_space_analysis"])
	}
}

func TestExecuteToolPlanDeliversCapabilityApprovalMessages(t *testing.T) {
	t.Parallel()

	workflowSvc := workflow.NewService(workflow.Options{ApprovalEnabled: true})
	_, err := workflowSvc.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "web_chat",
		Severity:    "warning",
		Fingerprint: "tool-plan-capability-approval",
		Labels: map[string]string{
			"host":           "host-1",
			"service":        "api",
			"severity":       "warning",
			"chat_id":        "ops-admin",
			"tars_generated": "web_chat",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	channel := &captureToolPlanChannel{}
	dispatcher := &Dispatcher{
		action:   &toolPlanApprovalActionService{},
		workflow: workflowSvc,
		channel:  channel,
	}
	sessions, err := workflowSvc.ListSessions(context.Background(), contracts.ListSessionsFilter{})
	if err != nil || len(sessions) != 1 {
		t.Fatalf("list sessions: %v %+v", err, sessions)
	}

	result, err := dispatcher.executeToolPlan(context.Background(), sessions[0], []contracts.ToolPlanStep{{
		ID:     "cap_1",
		Tool:   "connector.invoke_capability",
		Status: "planned",
		Input: map[string]interface{}{
			"connector_id":  "delivery-main",
			"capability_id": "deployment.promote",
			"service":       "api",
		},
		OnPendingApproval: "stop",
	}})
	if err != nil {
		t.Fatalf("execute tool plan: %v", err)
	}
	if len(channel.messages) == 0 {
		t.Fatalf("expected delivered approval message, got none")
	}
	if result.ExecutedPlan[0].Output["notification_count"] == nil {
		t.Fatalf("expected notification_count in step output, got %+v", result.ExecutedPlan[0].Output)
	}
}

func TestExecuteToolPlanQueuesCapabilityApprovalMessagesWhenChannelMissing(t *testing.T) {
	t.Parallel()

	workflowSvc := workflow.NewService(workflow.Options{ApprovalEnabled: true})
	_, err := workflowSvc.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "web_chat",
		Severity:    "warning",
		Fingerprint: "tool-plan-capability-approval-no-channel",
		Labels: map[string]string{
			"host":           "host-1",
			"service":        "api",
			"severity":       "warning",
			"chat_id":        "ops-admin",
			"tars_generated": "web_chat",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	dispatcher := &Dispatcher{
		action:   &toolPlanApprovalActionService{},
		workflow: workflowSvc,
	}
	sessions, err := workflowSvc.ListSessions(context.Background(), contracts.ListSessionsFilter{})
	if err != nil || len(sessions) != 1 {
		t.Fatalf("list sessions: %v %+v", err, sessions)
	}

	result, err := dispatcher.executeToolPlan(context.Background(), sessions[0], []contracts.ToolPlanStep{{
		ID:     "cap_1",
		Tool:   "connector.invoke_capability",
		Status: "planned",
		Input: map[string]interface{}{
			"connector_id":  "delivery-main",
			"capability_id": "deployment.promote",
			"service":       "api",
		},
		OnPendingApproval: "stop",
	}})
	if err != nil {
		t.Fatalf("execute tool plan: %v", err)
	}
	if result.ExecutedPlan[0].Status != "pending_approval" {
		t.Fatalf("expected pending_approval step, got %+v", result.ExecutedPlan)
	}
	if result.ExecutedPlan[0].Output["notification_count"] == nil {
		t.Fatalf("expected notification_count in step output, got %+v", result.ExecutedPlan[0].Output)
	}

	pendingOutbox, err := workflowSvc.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("list pending outbox: %v", err)
	}
	if len(pendingOutbox) != 1 || pendingOutbox[0].Topic != "telegram.send" {
		t.Fatalf("expected queued approval notification in outbox, got %+v", pendingOutbox)
	}
}

func TestExecuteToolPlanRunsLogsQueryAsCapability(t *testing.T) {
	t.Parallel()

	action := &capturingActionService{
		capabilityResult: contracts.CapabilityResult{
			Status: "completed",
			Output: map[string]interface{}{
				"result_count": 1,
				"summary":      "matched 1 log line",
				"logs":         []interface{}{map[string]interface{}{"line": "error api timeout"}},
			},
			Artifacts: []contracts.MessageAttachment{{
				Type:    "file",
				Name:    "logs-evidence.json",
				Content: `{"summary":"matched 1 log line"}`,
			}},
			Runtime: &contracts.RuntimeMetadata{
				Runtime:     "connector_capability",
				ConnectorID: "victorialogs-main",
				Protocol:    "victorialogs_http",
			},
		},
	}
	dispatcher := &Dispatcher{action: action}
	session := contracts.SessionDetail{SessionID: "ses-logs-query"}

	result, err := dispatcher.executeToolPlan(context.Background(), session, []contracts.ToolPlanStep{{
		ID:          "logs_1",
		Tool:        "logs.query",
		ConnectorID: "victorialogs-main",
		Status:      "planned",
		Input: map[string]interface{}{
			"connector_id":  "victorialogs-main",
			"capability_id": "logs.query",
			"query":         "error AND service:api",
			"service":       "api",
		},
	}})
	if err != nil {
		t.Fatalf("execute tool plan: %v", err)
	}
	if action.lastCapabilityRequest.CapabilityID != "logs.query" {
		t.Fatalf("expected logs.query capability invoke, got %+v", action.lastCapabilityRequest)
	}
	if len(result.ExecutedPlan) != 1 || result.ExecutedPlan[0].Status != "completed" {
		t.Fatalf("expected completed logs step, got %+v", result.ExecutedPlan)
	}
	if _, ok := result.Context["logs_query_result"].(map[string]interface{}); !ok {
		t.Fatalf("expected logs_query_result context, got %+v", result.Context)
	}
	if len(result.Attachments) != 1 || result.Attachments[0].Name != "logs-evidence.json" {
		t.Fatalf("expected logs attachment, got %+v", result.Attachments)
	}
}

func TestExecuteToolPlanBackfillsResolvedConnectorIDWithoutRuntimeMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tool         string
		connectorID  string
		capabilityID string
		input        map[string]interface{}
	}{
		{
			name:         "connector capability",
			tool:         "connector.invoke_capability",
			connectorID:  "delivery-main",
			capabilityID: "deployment.promote",
			input: map[string]interface{}{
				"connector_id":  "delivery-main",
				"capability_id": "deployment.promote",
			},
		},
		{
			name:         "logs query",
			tool:         "logs.query",
			connectorID:  "victorialogs-main",
			capabilityID: "logs.query",
			input: map[string]interface{}{
				"connector_id":  "victorialogs-main",
				"capability_id": "logs.query",
				"query":         "error",
			},
		},
		{
			name:         "delivery query",
			tool:         "delivery.query",
			connectorID:  "delivery-main",
			capabilityID: "query",
			input: map[string]interface{}{
				"connector_id":  "delivery-main",
				"capability_id": "query",
				"service":       "api",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			action := &capturingActionService{
				capabilityResult: contracts.CapabilityResult{
					Status: "completed",
					Output: map[string]interface{}{"summary": "ok"},
				},
			}
			dispatcher := &Dispatcher{action: action}

			result, err := dispatcher.executeToolPlan(context.Background(), contracts.SessionDetail{SessionID: fmt.Sprintf("ses-%s", tc.name)}, []contracts.ToolPlanStep{{
				ID:     "step_1",
				Tool:   tc.tool,
				Status: "planned",
				Input:  tc.input,
			}})
			if err != nil {
				t.Fatalf("execute tool plan: %v", err)
			}
			if action.lastCapabilityRequest.ConnectorID != tc.connectorID {
				t.Fatalf("expected capability request connector_id %q, got %+v", tc.connectorID, action.lastCapabilityRequest)
			}
			if result.ExecutedPlan[0].ConnectorID != tc.connectorID {
				t.Fatalf("expected executed step connector_id %q, got %+v", tc.connectorID, result.ExecutedPlan[0])
			}
			if got := lookupStepReference("$steps.step_1.connector_id", result.ExecutedPlan); got != tc.connectorID {
				t.Fatalf("expected step connector reference %q, got %#v", tc.connectorID, got)
			}
		})
	}
}

type capturingActionService struct {
	lastMetricsQuery      contracts.MetricsQuery
	metricsResult         contracts.MetricsResult
	metricsErr            error
	lastCapabilityRequest contracts.CapabilityRequest
	capabilityResult      contracts.CapabilityResult
	capabilityErr         error
}

type toolPlanApprovalActionService struct{}

func (t *toolPlanApprovalActionService) QueryMetrics(context.Context, contracts.MetricsQuery) (contracts.MetricsResult, error) {
	return contracts.MetricsResult{}, nil
}

func (t *toolPlanApprovalActionService) ExecuteApproved(context.Context, contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
	return contracts.ExecutionResult{}, nil
}

func (t *toolPlanApprovalActionService) InvokeApprovedCapability(context.Context, contracts.ApprovedCapabilityRequest) (contracts.CapabilityResult, error) {
	return contracts.CapabilityResult{}, nil
}

func (t *toolPlanApprovalActionService) VerifyExecution(context.Context, contracts.VerificationRequest) (contracts.VerificationResult, error) {
	return contracts.VerificationResult{}, nil
}

func (t *toolPlanApprovalActionService) CheckConnectorHealth(context.Context, string) (connectors.LifecycleState, error) {
	return connectors.LifecycleState{}, nil
}

func (t *toolPlanApprovalActionService) InvokeCapability(_ context.Context, req contracts.CapabilityRequest) (contracts.CapabilityResult, error) {
	return contracts.CapabilityResult{
		Status: "pending_approval",
		Runtime: &contracts.RuntimeMetadata{
			Runtime:     "capability",
			ConnectorID: req.ConnectorID,
		},
	}, nil
}

type captureToolPlanChannel struct {
	messages []contracts.ChannelMessage
}

func (c *captureToolPlanChannel) SendMessage(_ context.Context, msg contracts.ChannelMessage) (contracts.SendResult, error) {
	c.messages = append(c.messages, msg)
	return contracts.SendResult{MessageID: fmt.Sprintf("msg-%d", len(c.messages))}, nil
}

func (c *capturingActionService) QueryMetrics(_ context.Context, query contracts.MetricsQuery) (contracts.MetricsResult, error) {
	c.lastMetricsQuery = query
	return c.metricsResult, c.metricsErr
}

func (c *capturingActionService) ExecuteApproved(context.Context, contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
	return contracts.ExecutionResult{}, nil
}

func (c *capturingActionService) InvokeApprovedCapability(context.Context, contracts.ApprovedCapabilityRequest) (contracts.CapabilityResult, error) {
	return contracts.CapabilityResult{}, nil
}

func (c *capturingActionService) VerifyExecution(context.Context, contracts.VerificationRequest) (contracts.VerificationResult, error) {
	return contracts.VerificationResult{}, nil
}

func (c *capturingActionService) CheckConnectorHealth(context.Context, string) (connectors.LifecycleState, error) {
	return connectors.LifecycleState{}, nil
}

func (c *capturingActionService) InvokeCapability(_ context.Context, req contracts.CapabilityRequest) (contracts.CapabilityResult, error) {
	c.lastCapabilityRequest = req
	return c.capabilityResult, c.capabilityErr
}
