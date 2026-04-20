package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"tars/internal/api/dto"
	"tars/internal/contracts"
	"tars/internal/events"
	"tars/internal/foundation/audit"
	"tars/internal/foundation/config"
	"tars/internal/foundation/logger"
	foundationmetrics "tars/internal/foundation/metrics"
	"tars/internal/foundation/secrets"
	"tars/internal/modules/access"
	"tars/internal/modules/action"
	actionprovider "tars/internal/modules/action/provider"
	actionssh "tars/internal/modules/action/ssh"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/alertintake"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/automations"
	"tars/internal/modules/connectors"
	"tars/internal/modules/knowledge"
	"tars/internal/modules/msgtpl"
	"tars/internal/modules/org"
	"tars/internal/modules/reasoning"
	"tars/internal/modules/skills"
	"tars/internal/modules/trigger"
	"tars/internal/modules/workflow"
	postgresrepo "tars/internal/repo/postgres"
)

func TestPlatformDiscoveryReturnsCapabilities(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/platform/discovery", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected discovery status: %d", resp.Code)
	}

	var payload dto.PlatformDiscoveryResponse
	decodeRecorderJSON(t, resp, &payload)

	if payload.ProductName != "TARS" {
		t.Fatalf("unexpected product name: %q", payload.ProductName)
	}
	if payload.ManifestVersion != "tars.connector/v1alpha1" {
		t.Fatalf("unexpected manifest version: %q", payload.ManifestVersion)
	}
	if payload.MarketplacePackageVersion != "tars.marketplace/v1alpha1" {
		t.Fatalf("unexpected marketplace package version: %q", payload.MarketplacePackageVersion)
	}
	if !containsString(payload.IntegrationModes, "connector_manifest") {
		t.Fatalf("expected connector_manifest integration mode, got %+v", payload.IntegrationModes)
	}
	if !containsString(payload.ConnectorKinds, "skill_source") {
		t.Fatalf("expected skill_source connector kind, got %+v", payload.ConnectorKinds)
	}
	if len(payload.ToolPlanCapabilities) == 0 {
		t.Fatalf("expected tool plan capabilities in discovery payload")
	}
	if !hasToolPlanCapability(payload.ToolPlanCapabilities, "knowledge.search", "", "") {
		t.Fatalf("expected builtin knowledge.search capability, got %+v", payload.ToolPlanCapabilities)
	}
	if !containsString(payload.Docs, "/specs/20-component-connectors.md") {
		t.Fatalf("expected connector platform spec doc link, got %+v", payload.Docs)
	}
	if !containsString(payload.Docs, "/specs/20-component-skills.md") {
		t.Fatalf("expected skill platform spec doc link, got %+v", payload.Docs)
	}
	if !hasToolPlanCapability(payload.ToolPlanCapabilities, "skill.select", "", "disk-space-incident") {
		t.Fatalf("expected skill.select capability in discovery payload, got %+v", payload.ToolPlanCapabilities)
	}
}

func TestVMAlertWebhookCreatesBlockedOutboxWhenDiagnosisDisabled(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, false, false, false)
	handler := system.handler
	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "HighCPU",
					"instance": "host-1",
					"severity": "critical"
				},
				"annotations": {
					"summary": "cpu too high"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)
	if !accepted.Accepted || accepted.EventCount != 1 || len(accepted.SessionIDs) != 1 {
		t.Fatalf("unexpected webhook response: %+v", accepted)
	}
	sessionID := accepted.SessionIDs[0]

	unauthorizedResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions", nil, nil)
	if unauthorizedResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized ops response, got %d", unauthorizedResp.Code)
	}

	sessionListResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionListResp.Code != http.StatusOK {
		t.Fatalf("unexpected sessions status: %d", sessionListResp.Code)
	}

	var sessions dto.SessionListResponse
	decodeRecorderJSON(t, sessionListResp, &sessions)
	if len(sessions.Items) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions.Items))
	}
	if sessions.Items[0].Status != "open" {
		t.Fatalf("expected session status open, got %s", sessions.Items[0].Status)
	}

	sessionResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+sessionID, nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session detail status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if sessionDetail.DiagnosisSummary != "Diagnosis feature disabled" {
		t.Fatalf("unexpected diagnosis summary: %q", sessionDetail.DiagnosisSummary)
	}
	if sessionDetail.GoldenSummary == nil || sessionDetail.GoldenSummary.Headline == "" {
		t.Fatalf("expected golden summary on session detail, got %+v", sessionDetail.GoldenSummary)
	}

	outboxResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/outbox?status=blocked", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if outboxResp.Code != http.StatusOK {
		t.Fatalf("unexpected outbox status: %d", outboxResp.Code)
	}

	var outbox dto.OutboxListResponse
	decodeRecorderJSON(t, outboxResp, &outbox)
	if len(outbox.Items) != 1 {
		t.Fatalf("expected 1 outbox item, got %d", len(outbox.Items))
	}
	if outbox.Items[0].AggregateID != sessionID || outbox.Items[0].BlockedReason != "diagnosis_disabled" {
		t.Fatalf("unexpected outbox item: %+v", outbox.Items[0])
	}

	replayResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/outbox/"+outbox.Items[0].ID+"/replay", []byte(`{"operator_reason":"retry after check"}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if replayResp.Code != http.StatusConflict {
		t.Fatalf("expected replay conflict, got %d", replayResp.Code)
	}

	var replayErr dto.ErrorEnvelope
	decodeRecorderJSON(t, replayResp, &replayErr)
	if replayErr.Error.Code != "blocked_by_feature_flag" {
		t.Fatalf("unexpected replay error: %+v", replayErr)
	}

	deleteResp := performJSONRequest(t, handler, http.MethodDelete, "/api/v1/outbox/"+outbox.Items[0].ID, []byte(`{"operator_reason":"cleanup historical residue"}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected delete success, got %d", deleteResp.Code)
	}

	outboxResp = performJSONRequest(t, handler, http.MethodGet, "/api/v1/outbox?status=blocked", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if outboxResp.Code != http.StatusOK {
		t.Fatalf("unexpected outbox status after delete: %d", outboxResp.Code)
	}
	decodeRecorderJSON(t, outboxResp, &outbox)
	if len(outbox.Items) != 0 {
		t.Fatalf("expected blocked outbox to be empty after delete, got %+v", outbox.Items)
	}
}

func TestVMAlertWebhookDuplicateReusesSession(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	handler := system.handler
	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "DiskFull",
					"instance": "host-2",
					"severity": "warning"
				},
				"annotations": {
					"summary": "disk usage high"
				}
			}
		]
	}`)

	firstResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if firstResp.Code != http.StatusOK {
		t.Fatalf("unexpected first webhook status: %d", firstResp.Code)
	}

	var firstAccepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, firstResp, &firstAccepted)

	secondResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if secondResp.Code != http.StatusOK {
		t.Fatalf("unexpected second webhook status: %d", secondResp.Code)
	}

	var secondAccepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, secondResp, &secondAccepted)
	if !secondAccepted.Duplicated {
		t.Fatalf("expected duplicated response, got %+v", secondAccepted)
	}
	if firstAccepted.SessionIDs[0] != secondAccepted.SessionIDs[0] {
		t.Fatalf("expected same session id, got %v and %v", firstAccepted.SessionIDs, secondAccepted.SessionIDs)
	}

	sessionListResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionListResp.Code != http.StatusOK {
		t.Fatalf("unexpected sessions status: %d", sessionListResp.Code)
	}

	var sessions dto.SessionListResponse
	decodeRecorderJSON(t, sessionListResp, &sessions)
	if len(sessions.Items) != 1 {
		t.Fatalf("expected deduplicated session list, got %d items", len(sessions.Items))
	}
	if sessions.Items[0].Status != "analyzing" {
		t.Fatalf("expected analyzing session status, got %s", sessions.Items[0].Status)
	}
}

func TestDeleteOutboxRejectsPendingEvent(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	handler := system.handler
	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "HighCPU",
					"instance": "host-1",
					"severity": "critical"
				},
				"annotations": {
					"summary": "cpu too high"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)

	items, err := system.workflow.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("list pending outbox: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 pending outbox item, got %d", len(items))
	}

	deleteResp := performJSONRequest(t, handler, http.MethodDelete, "/api/v1/outbox/"+items[0].ID, []byte(`{"operator_reason":"should fail"}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if deleteResp.Code != http.StatusConflict {
		t.Fatalf("expected delete conflict, got %d", deleteResp.Code)
	}

	var deleteErr dto.ErrorEnvelope
	decodeRecorderJSON(t, deleteResp, &deleteErr)
	if deleteErr.Error.Code != "invalid_state" {
		t.Fatalf("unexpected delete error: %+v", deleteErr)
	}
	if accepted.SessionIDs[0] == "" {
		t.Fatal("expected session id")
	}
}

func TestBulkDeleteOutboxSupportsPartialSuccess(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, false, false, false)
	handler := system.handler
	payloads := [][]byte{
		[]byte(`{"status":"firing","alerts":[{"labels":{"alertname":"BulkDeleteA","instance":"host-1","severity":"critical"},"annotations":{"summary":"bulk delete a"}}]}`),
		[]byte(`{"status":"firing","alerts":[{"labels":{"alertname":"BulkDeleteB","instance":"host-2","severity":"critical"},"annotations":{"summary":"bulk delete b"}}]}`),
	}

	for _, payload := range payloads {
		resp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
			"X-Tars-Signature": "test-signature",
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("unexpected webhook status: %d", resp.Code)
		}
	}

	items, err := system.workflow.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "blocked"})
	if err != nil {
		t.Fatalf("list blocked outbox: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 blocked outbox items, got %d", len(items))
	}

	resp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/outbox/bulk/delete", []byte(`{
		"ids": ["`+items[0].ID+`", "missing-id", "`+items[1].ID+`"],
		"operator_reason": "bulk cleanup"
	}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected bulk delete status: %d body=%s", resp.Code, resp.Body.String())
	}

	var result dto.BatchOperationResponse
	decodeRecorderJSON(t, resp, &result)
	if result.Operation != "delete" || result.Total != 3 || result.Succeeded != 2 || result.Failed != 1 {
		t.Fatalf("unexpected bulk delete response: %+v", result)
	}

	items, err = system.workflow.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "blocked"})
	if err != nil {
		t.Fatalf("list blocked outbox after bulk delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected blocked outbox to be empty, got %+v", items)
	}
}

func TestBulkReplayOutboxSupportsPartialSuccess(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	handler := system.handler
	payload := []byte(`{"status":"firing","alerts":[{"labels":{"alertname":"BulkReplay","instance":"host-1","severity":"critical"},"annotations":{"summary":"bulk replay"}}]}`)

	resp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", resp.Code)
	}

	items, err := system.workflow.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("list pending outbox: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 pending outbox item, got %d", len(items))
	}
	if err := system.workflow.MarkOutboxFailed(context.Background(), items[0].ID, "forced test failure"); err != nil {
		t.Fatalf("mark outbox failed: %v", err)
	}

	resp = performJSONRequest(t, handler, http.MethodPost, "/api/v1/outbox/bulk/replay", []byte(`{
		"ids": ["`+items[0].ID+`", "missing-id"],
		"operator_reason": "bulk replay"
	}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected bulk replay status: %d body=%s", resp.Code, resp.Body.String())
	}

	var result dto.BatchOperationResponse
	decodeRecorderJSON(t, resp, &result)
	if result.Operation != "replay" || result.Total != 2 || result.Succeeded != 1 || result.Failed != 1 {
		t.Fatalf("unexpected bulk replay response: %+v", result)
	}

	replayed, err := system.workflow.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("list pending outbox after bulk replay: %v", err)
	}
	if len(replayed) != 1 || replayed[0].ID != items[0].ID {
		t.Fatalf("expected replayed event to return to pending, got %+v", replayed)
	}
}

func TestBulkExportSessionsReturnsAttachmentWithPartialFailures(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	handler := system.handler

	payload := []byte(`{"status":"firing","alerts":[{"labels":{"alertname":"ExportSession","instance":"host-1","severity":"critical"},"annotations":{"summary":"bulk export session"}}]}`)
	webhookResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d body=%s", webhookResp.Code, webhookResp.Body.String())
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)
	if len(accepted.SessionIDs) != 1 {
		t.Fatalf("expected one session id, got %+v", accepted.SessionIDs)
	}

	resp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/sessions/bulk/export", []byte(`{
		"ids": ["`+accepted.SessionIDs[0]+`", "not-a-uuid"],
		"operator_reason": "bulk export"
	}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected bulk export status: %d body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Disposition"); !strings.Contains(got, "tars-sessions-export-") {
		t.Fatalf("expected attachment filename, got %q", got)
	}

	var export dto.SessionExportResponse
	decodeRecorderJSON(t, resp, &export)
	if export.ExportedCount != 1 || export.FailedCount != 1 || len(export.Items) != 1 || len(export.Failures) != 1 {
		t.Fatalf("unexpected session export payload: %+v", export)
	}
	if export.Failures[0].Code != "validation_failed" {
		t.Fatalf("expected validation_failed for invalid id, got %+v", export.Failures[0])
	}
}

func TestBulkExportExecutionsReturnsAttachmentWithPartialFailures(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	handler := system.handler

	payload := []byte(`{"status":"firing","alerts":[{"labels":{"alertname":"ExportExecution","instance":"host-1","service":"api","severity":"critical"},"annotations":{"summary":"bulk export execution","user_request":"执行命令查看 api 状态"}}]}`)
	webhookResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d body=%s", webhookResp.Code, webhookResp.Body.String())
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status: %d body=%s", sessionResp.Code, sessionResp.Body.String())
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if len(sessionDetail.Executions) != 1 {
		t.Fatalf("expected one execution draft, got %d", len(sessionDetail.Executions))
	}

	resp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/executions/bulk/export", []byte(`{
		"ids": ["`+sessionDetail.Executions[0].ExecutionID+`", "bad-id"],
		"operator_reason": "bulk export"
	}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected bulk export status: %d body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Disposition"); !strings.Contains(got, "tars-executions-export-") {
		t.Fatalf("expected attachment filename, got %q", got)
	}

	var export dto.ExecutionExportResponse
	decodeRecorderJSON(t, resp, &export)
	if export.ExportedCount != 1 || export.FailedCount != 1 || len(export.Items) != 1 || len(export.Failures) != 1 {
		t.Fatalf("unexpected execution export payload: %+v", export)
	}
	if export.Failures[0].Code != "validation_failed" {
		t.Fatalf("expected validation_failed for invalid id, got %+v", export.Failures[0])
	}
}

func TestVMAlertWebhookRejectsInvalidSecret(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.VMAlert.WebhookSecret = "expected-signature"
	system := newTestSystemWithConfig(t, true, false, false, cfg)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert", []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "HighCPU",
					"instance": "host-1",
					"severity": "critical"
				},
				"annotations": {
					"summary": "cpu too high"
				}
			}
		]
	}`), map[string]string{
		"X-Tars-Signature": "wrong-signature",
	})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.Code)
	}
}

func TestVMAlertAlertmanagerPathAcceptsRequestsWhenSecretDisabled(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert/api/v2/alerts", []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "TarsSmokeNodeUp",
					"instance": "host-vm",
					"service": "sshd",
					"severity": "critical"
				},
				"annotations": {
					"summary": "vm alertmanager path smoke"
				}
			}
		]
	}`), nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected accepted alertmanager path, got %d", resp.Code)
	}
}

func TestTelegramApprovalExecutesExecution(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	handler := system.handler

	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "HighCPU",
					"instance": "host-3",
					"service": "api",
					"severity": "critical"
				},
				"annotations": {
					"summary": "cpu too high",
					"user_request": "执行命令查看 api 状态"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if sessionDetail.Status != "pending_approval" {
		t.Fatalf("expected pending_approval, got %s", sessionDetail.Status)
	}
	if len(sessionDetail.Executions) != 1 {
		t.Fatalf("expected 1 execution draft, got %d", len(sessionDetail.Executions))
	}

	executionID := sessionDetail.Executions[0].ExecutionID
	callbackPayload := []byte(fmt.Sprintf(`{
		"update_id": 1001,
		"callback_query": {
			"id": "cbq-1",
			"data": "approve:%s",
			"from": {
				"id": 42,
				"username": "alice"
			},
			"message": {
				"chat": {
					"id": "-1001001"
				}
			}
		}
	}`, executionID))

	callbackResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/channels/telegram/webhook", callbackPayload, nil)
	if callbackResp.Code != http.StatusOK {
		t.Fatalf("unexpected callback status: %d", callbackResp.Code)
	}
	if len(system.channel.callbackAcks) != 1 {
		t.Fatalf("expected one callback ack, got %d", len(system.channel.callbackAcks))
	}
	if system.channel.callbackAcks[0].ID != "cbq-1" || system.channel.callbackAcks[0].Text != "已批准，开始执行" {
		t.Fatalf("unexpected callback ack: %+v", system.channel.callbackAcks[0])
	}
	messageCount := len(system.channel.messages)

	duplicateResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/channels/telegram/webhook", callbackPayload, nil)
	if duplicateResp.Code != http.StatusOK {
		t.Fatalf("unexpected duplicate callback status: %d", duplicateResp.Code)
	}
	if len(system.channel.callbackAcks) != 2 {
		t.Fatalf("expected duplicate callback ack, got %d", len(system.channel.callbackAcks))
	}
	if system.channel.callbackAcks[1].Text != "已批准，开始执行" {
		t.Fatalf("unexpected duplicate callback ack: %+v", system.channel.callbackAcks[1])
	}
	if len(system.channel.messages) != messageCount {
		t.Fatalf("expected duplicate callback to be deduplicated, got %d messages vs %d", len(system.channel.messages), messageCount)
	}

	sessionResp = performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status after callback: %d", sessionResp.Code)
	}
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if sessionDetail.Status != "resolved" {
		t.Fatalf("expected resolved session, got %s", sessionDetail.Status)
	}
	if sessionDetail.Executions[0].Status != "completed" {
		t.Fatalf("expected completed execution, got %s", sessionDetail.Executions[0].Status)
	}
	if len(system.channel.messages) < 4 {
		t.Fatalf("expected diagnosis, approval, start, and result messages, got %d", len(system.channel.messages))
	}
	resultMessage := system.channel.messages[len(system.channel.messages)-1]
	if !strings.Contains(resultMessage.Body, "输出:") {
		t.Fatalf("expected execution result preview in telegram message, got %+v", resultMessage)
	}
	if !strings.Contains(resultMessage.Body, "10:00 up 1 day") {
		t.Fatalf("expected execution output content in telegram result message, got %+v", resultMessage)
	}
}

func TestTelegramConversationMessageCreatesPendingApproval(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.SSH.AllowedHosts = []string{"host-3"}
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	messagePayload := []byte(`{
		"update_id": 2001,
		"message": {
			"message_id": 1,
			"text": "看系统负载",
			"from": {
				"id": 42,
				"username": "alice",
				"is_bot": false
			},
			"chat": {
				"id": "445308292",
				"type": "private"
			}
		}
	}`)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram/webhook", messagePayload, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected telegram message status: %d", resp.Code)
	}
	if len(system.channel.messages) != 1 {
		t.Fatalf("expected immediate ack message, got %d", len(system.channel.messages))
	}
	if !strings.Contains(system.channel.messages[0].Body, "[TARS 对话]") {
		t.Fatalf("expected conversation ack body, got %+v", system.channel.messages[0])
	}
	if system.channel.messages[0].Target != "445308292" {
		t.Fatalf("expected ack sent back to same chat, got %+v", system.channel.messages[0])
	}

	sessionListResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionListResp.Code != http.StatusOK {
		t.Fatalf("unexpected sessions status: %d", sessionListResp.Code)
	}

	var sessions dto.SessionListResponse
	decodeRecorderJSON(t, sessionListResp, &sessions)
	if len(sessions.Items) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions.Items))
	}

	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions/"+sessions.Items[0].SessionID, nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session detail status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if sessionDetail.Status != "resolved" {
		t.Fatalf("expected resolved session, got %s", sessionDetail.Status)
	}
	if len(sessionDetail.Executions) != 0 {
		t.Fatalf("expected no execution drafts for read-only load request, got %+v", sessionDetail.Executions)
	}
	if len(system.channel.messages) != 2 {
		t.Fatalf("expected ack + diagnosis messages, got %d", len(system.channel.messages))
	}
	if system.channel.messages[1].Target != "445308292" {
		t.Fatalf("expected diagnosis routed back to same chat, got %+v", system.channel.messages[1])
	}
	if !strings.Contains(system.channel.messages[1].Body, "请求: 看系统负载") {
		t.Fatalf("expected diagnosis to mention user request, got %+v", system.channel.messages[1])
	}
}

func TestTelegramConversationMessageCreatesPendingApprovalForExitIPQuery(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.SSH.AllowedHosts = []string{"192.168.3.106"}
	system := newTestSystemWithExecutor(t, true, true, true, cfg, &fakeExecutor{
		runFunc: func(_ context.Context, _ string, command string) (actionssh.Result, error) {
			if strings.HasPrefix(command, "curl -fsS https://api.ipify.org") {
				return actionssh.Result{ExitCode: 0, Output: "203.0.113.10\n"}, nil
			}
			return actionssh.Result{ExitCode: 0, Output: "hostname\n 10:00 up 1 day"}, nil
		},
	})

	messagePayload := []byte(`{
		"update_id": 2004,
		"message": {
			"message_id": 4,
			"text": "看一下你的出口IP是多少",
			"from": {
				"id": 52,
				"username": "bigluandou",
				"is_bot": false
			},
			"chat": {
				"id": "445308292",
				"type": "private"
			}
		}
	}`)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram/webhook", messagePayload, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected telegram message status: %d", resp.Code)
	}
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionListResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionListResp.Code != http.StatusOK {
		t.Fatalf("unexpected sessions status: %d", sessionListResp.Code)
	}

	var sessions dto.SessionListResponse
	decodeRecorderJSON(t, sessionListResp, &sessions)
	if len(sessions.Items) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions.Items))
	}

	sessionResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions/"+sessions.Items[0].SessionID, nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session detail status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if sessionDetail.Status != "pending_approval" {
		t.Fatalf("expected pending_approval session, got %s", sessionDetail.Status)
	}
	if len(sessionDetail.Executions) != 1 {
		t.Fatalf("expected one execution draft, got %d", len(sessionDetail.Executions))
	}
	if sessionDetail.Executions[0].Command != "curl -fsS https://api.ipify.org && echo" {
		t.Fatalf("unexpected execution hint: %+v", sessionDetail.Executions[0])
	}
	if !strings.Contains(system.channel.messages[2].Body, "命令: curl -fsS https://api.ipify.org && echo") {
		t.Fatalf("expected approval message to include exit ip command, got %+v", system.channel.messages[2])
	}
}

func TestTelegramConversationMessageWithoutExecutionHintResolvesAfterDiagnosis(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.SSH.AllowedHosts = []string{"192.168.3.106"}
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	messagePayload := []byte(`{
		"update_id": 2005,
		"message": {
			"message_id": 5,
			"text": "帮我总结一下当前状态",
			"from": {
				"id": 53,
				"username": "alice",
				"is_bot": false
			},
			"chat": {
				"id": "445308292",
				"type": "private"
			}
		}
	}`)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram/webhook", messagePayload, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected telegram message status: %d", resp.Code)
	}
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionListResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionListResp.Code != http.StatusOK {
		t.Fatalf("unexpected sessions status: %d", sessionListResp.Code)
	}

	var sessions dto.SessionListResponse
	decodeRecorderJSON(t, sessionListResp, &sessions)
	if len(sessions.Items) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions.Items))
	}

	sessionResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions/"+sessions.Items[0].SessionID, nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session detail status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if sessionDetail.Status != "resolved" {
		t.Fatalf("expected resolved session, got %s", sessionDetail.Status)
	}
	if len(sessionDetail.Executions) != 0 {
		t.Fatalf("expected no executions, got %d", len(sessionDetail.Executions))
	}
	if len(system.channel.messages) != 2 {
		t.Fatalf("expected ack + diagnosis messages, got %d", len(system.channel.messages))
	}
	if !strings.Contains(system.channel.messages[1].Body, "帮我总结一下当前状态") {
		t.Fatalf("expected diagnosis to mention user request, got %+v", system.channel.messages[1])
	}
}

func TestTelegramConversationMessageRequiresHostWhenMultipleHostsAllowed(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.SSH.AllowedHosts = []string{"host-1", "host-2"}
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	messagePayload := []byte(`{
		"update_id": 2002,
		"message": {
			"message_id": 2,
			"text": "看系统负载",
			"from": {
				"id": 42,
				"username": "alice",
				"is_bot": false
			},
			"chat": {
				"id": "445308292",
				"type": "private"
			}
		}
	}`)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram/webhook", messagePayload, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected telegram message status: %d", resp.Code)
	}
	if len(system.channel.messages) != 1 {
		t.Fatalf("expected one guidance message, got %d", len(system.channel.messages))
	}
	if !strings.Contains(system.channel.messages[0].Body, "host=192.168.3.106 看系统负载") {
		t.Fatalf("expected host guidance message, got %+v", system.channel.messages[0])
	}

	sessionListResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionListResp.Code != http.StatusOK {
		t.Fatalf("unexpected sessions status: %d", sessionListResp.Code)
	}

	var sessions dto.SessionListResponse
	decodeRecorderJSON(t, sessionListResp, &sessions)
	if len(sessions.Items) != 0 {
		t.Fatalf("expected no session to be created, got %d", len(sessions.Items))
	}
}

func TestTelegramConversationWritesAuditEntries(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.SSH.AllowedHosts = []string{"host-3"}
	auditLogger := &captureAuditLogger{}
	system := newTestSystemWithExecutorAndAudit(t, true, true, true, cfg, &fakeExecutor{
		runFunc: func(_ context.Context, _ string, command string) (actionssh.Result, error) {
			if strings.HasPrefix(command, "systemctl is-active") {
				return actionssh.Result{ExitCode: 0, Output: "active\n"}, nil
			}
			return actionssh.Result{ExitCode: 0, Output: "hostname\n 10:00 up 1 day"}, nil
		},
	}, auditLogger)

	messagePayload := []byte(`{
		"update_id": 2003,
		"message": {
			"message_id": 3,
			"text": "看系统负载",
			"from": {
				"id": 42,
				"username": "alice",
				"is_bot": false
			},
			"chat": {
				"id": "445308292",
				"type": "private"
			}
		}
	}`)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram/webhook", messagePayload, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected telegram message status: %d", resp.Code)
	}

	if len(auditLogger.entries) < 2 {
		t.Fatalf("expected audit entries for chat request and ack, got %d", len(auditLogger.entries))
	}
	if auditLogger.entries[0].ResourceType != "telegram_chat" || auditLogger.entries[0].Action != "receive" {
		t.Fatalf("unexpected first audit entry: %+v", auditLogger.entries[0])
	}
	if auditLogger.entries[1].ResourceType != "telegram_message" || auditLogger.entries[1].Action != "dispatch" {
		t.Fatalf("unexpected second audit entry: %+v", auditLogger.entries[1])
	}
}

func TestTelegramApprovalEnqueuesResultNotificationRetryWhenSendFails(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	system.channel.failOnCalls = map[int]error{
		4: fmt.Errorf("telegram send failed"),
	}
	handler := system.handler

	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "HighCPU",
					"instance": "host-3",
					"service": "api",
					"severity": "critical"
				},
				"annotations": {
					"summary": "cpu too high",
					"user_request": "执行命令查看 api 状态"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	executionID := sessionDetail.Executions[0].ExecutionID
	callbackPayload := []byte(fmt.Sprintf(`{
		"update_id": 1003,
		"callback_query": {
			"id": "cbq-retry",
			"data": "approve:%s",
			"from": {
				"id": 42,
				"username": "alice"
			},
			"message": {
				"chat": {
					"id": "-1001001"
				}
			}
		}
	}`, executionID))

	callbackResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/channels/telegram/webhook", callbackPayload, nil)
	if callbackResp.Code != http.StatusOK {
		t.Fatalf("unexpected callback status: %d", callbackResp.Code)
	}
	if len(system.channel.callbackAcks) != 1 || system.channel.callbackAcks[0].Text != "执行已处理，结果通知将重试发送" {
		t.Fatalf("unexpected callback ack: %+v", system.channel.callbackAcks)
	}

	sessionResp = performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status after callback: %d", sessionResp.Code)
	}
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if sessionDetail.Status != "resolved" {
		t.Fatalf("expected resolved session, got %s", sessionDetail.Status)
	}

	outboxResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/outbox?status=pending", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if outboxResp.Code != http.StatusOK {
		t.Fatalf("unexpected outbox status: %d", outboxResp.Code)
	}

	var outbox dto.OutboxListResponse
	decodeRecorderJSON(t, outboxResp, &outbox)
	if len(outbox.Items) != 1 || outbox.Items[0].Topic != "telegram.send" {
		t.Fatalf("expected pending telegram retry outbox, got %+v", outbox.Items)
	}

	delete(system.channel.failOnCalls, 4)
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run retry dispatcher: %v", err)
	}
	if len(system.channel.messages) != 4 {
		t.Fatalf("expected diagnosis, approval, approval-start, retried result messages, got %d", len(system.channel.messages))
	}
	if !strings.Contains(system.channel.messages[3].Body, "状态: completed") {
		t.Fatalf("expected retried result message, got %+v", system.channel.messages[3])
	}
}

func TestTelegramApprovalExecutionFailureMarksSessionFailed(t *testing.T) {
	t.Parallel()

	system := newTestSystemWithExecutor(t, true, true, true, defaultTestConfig(), &fakeExecutor{
		err: fmt.Errorf("ssh transport failed"),
	})
	handler := system.handler

	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "HighCPU",
					"instance": "host-3",
					"service": "api",
					"severity": "critical"
				},
				"annotations": {
					"summary": "cpu too high",
					"user_request": "执行命令查看 api 状态"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if len(sessionDetail.Executions) != 1 {
		t.Fatalf("expected 1 execution draft, got %d", len(sessionDetail.Executions))
	}

	executionID := sessionDetail.Executions[0].ExecutionID
	callbackPayload := []byte(fmt.Sprintf(`{
		"update_id": 1002,
		"callback_query": {
			"id": "cbq-fail",
			"data": "approve:%s",
			"from": {
				"id": 42,
				"username": "alice"
			},
			"message": {
				"chat": {
					"id": "-1001001"
				}
			}
		}
	}`, executionID))

	callbackResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/channels/telegram/webhook", callbackPayload, nil)
	if callbackResp.Code != http.StatusInternalServerError {
		t.Fatalf("expected callback failure status, got %d", callbackResp.Code)
	}

	if len(system.channel.callbackAcks) != 1 {
		t.Fatalf("expected callback ack, got %d", len(system.channel.callbackAcks))
	}
	if system.channel.callbackAcks[0].Text != "执行失败，请查看结果消息" {
		t.Fatalf("unexpected callback ack: %+v", system.channel.callbackAcks[0])
	}

	executionResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/executions/"+executionID, nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if executionResp.Code != http.StatusOK {
		t.Fatalf("unexpected execution status: %d", executionResp.Code)
	}

	var executionDetail dto.ExecutionDetail
	decodeRecorderJSON(t, executionResp, &executionDetail)
	if executionDetail.Status != "failed" {
		t.Fatalf("expected failed execution, got %s", executionDetail.Status)
	}
	if executionDetail.ExitCode != 1 {
		t.Fatalf("expected synthetic exit code 1, got %d", executionDetail.ExitCode)
	}
	if executionDetail.GoldenSummary == nil || executionDetail.GoldenSummary.Result == "" {
		t.Fatalf("expected execution golden summary, got %+v", executionDetail.GoldenSummary)
	}

	sessionResp = performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status after callback: %d", sessionResp.Code)
	}

	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if sessionDetail.Status != "failed" {
		t.Fatalf("expected failed session, got %s", sessionDetail.Status)
	}
	if len(system.channel.messages) < 2 {
		t.Fatalf("expected diagnosis and approval messages, got %d", len(system.channel.messages))
	}
}

func TestTelegramApprovalVerificationFailureReturnsSessionToAnalyzing(t *testing.T) {
	t.Parallel()

	system := newTestSystemWithExecutor(t, true, true, true, defaultTestConfig(), &fakeExecutor{
		runFunc: func(_ context.Context, _ string, command string) (actionssh.Result, error) {
			if strings.HasPrefix(command, "systemctl is-active") {
				return actionssh.Result{
					ExitCode: 3,
					Output:   "inactive\n",
				}, actionssh.ErrRemoteCommandFailed
			}
			return actionssh.Result{
				ExitCode: 0,
				Output:   "hostname\n 10:00 up 1 day",
			}, nil
		},
	})
	handler := system.handler

	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "HighCPU",
					"instance": "host-3",
					"service": "api",
					"severity": "critical"
				},
				"annotations": {
					"summary": "cpu too high",
					"user_request": "执行命令查看 api 状态"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	executionID := sessionDetail.Executions[0].ExecutionID
	callbackPayload := []byte(fmt.Sprintf(`{
		"update_id": 1003,
		"callback_query": {
			"id": "cbq-verify-fail",
			"data": "approve:%s",
			"from": {
				"id": 42,
				"username": "alice"
			},
			"message": {
				"chat": {
					"id": "-1001001"
				}
			}
		}
	}`, executionID))

	callbackResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/channels/telegram/webhook", callbackPayload, nil)
	if callbackResp.Code != http.StatusOK {
		t.Fatalf("unexpected callback status: %d", callbackResp.Code)
	}

	sessionResp = performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status after callback: %d", sessionResp.Code)
	}
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if sessionDetail.Status != "analyzing" {
		t.Fatalf("expected analyzing session, got %s", sessionDetail.Status)
	}
	if sessionDetail.Verification == nil || sessionDetail.Verification.Status != "failed" {
		t.Fatalf("expected failed verification, got %+v", sessionDetail.Verification)
	}
}

func TestExecutionOutputEndpointReturnsStoredChunks(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	handler := system.handler

	webhookResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "ExecutionOutput",
					"instance": "host-1",
					"service": "sshd",
					"severity": "critical"
				},
				"annotations": {
					"description": "output endpoint drill",
					"user_request": "执行命令查看 sshd 状态"
				}
			}
		]
	}`), map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session detail status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	executionID := sessionDetail.Executions[0].ExecutionID

	callbackPayload := []byte(fmt.Sprintf(`{
		"update_id": 2201,
		"callback_query": {
			"id": "cbq-output",
			"data": "approve:%s",
			"from": {
				"id": 42,
				"username": "alice"
			},
			"message": {
				"chat": {
					"id": "-1001001"
				}
			}
		}
	}`, executionID))

	callbackResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/channels/telegram/webhook", callbackPayload, nil)
	if callbackResp.Code != http.StatusOK {
		t.Fatalf("unexpected callback status: %d", callbackResp.Code)
	}

	outputResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/executions/"+executionID+"/output", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if outputResp.Code != http.StatusOK {
		t.Fatalf("unexpected output status: %d", outputResp.Code)
	}

	var output dto.ExecutionOutputResponse
	decodeRecorderJSON(t, outputResp, &output)
	if output.ExecutionID != executionID {
		t.Fatalf("unexpected execution id: %+v", output)
	}
	if len(output.Chunks) == 0 {
		t.Fatalf("expected output chunks, got %+v", output)
	}
	if output.Chunks[0].Content == "" {
		t.Fatalf("expected first chunk content, got %+v", output.Chunks[0])
	}
}

func TestReindexDocumentsRequiresOpsAccessAndReason(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	unauthorized := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/reindex/documents", []byte(`{"operator_reason":"rebuild"}`), nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", unauthorized.Code)
	}

	invalid := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/reindex/documents", []byte(`{}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", invalid.Code)
	}

	accepted := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/reindex/documents", []byte(`{"operator_reason":"rebuild"}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if accepted.Code != http.StatusOK {
		t.Fatalf("expected accepted, got %d", accepted.Code)
	}
}

func TestOpsReadsWriteAuditEntries(t *testing.T) {
	t.Parallel()

	auditLogger := &captureAuditLogger{}
	system := newTestSystemWithExecutorAndAudit(t, true, false, false, defaultTestConfig(), &fakeExecutor{
		result: actionssh.Result{
			ExitCode: 0,
			Output:   "hostname\n 10:00 up 1 day",
		},
	}, auditLogger)

	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "AuditRead",
					"instance": "host-audit",
					"severity": "critical"
				},
				"annotations": {
					"summary": "audit read smoke"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)

	listResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions?status=open", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if listResp.Code != http.StatusOK {
		t.Fatalf("unexpected session list status: %d", listResp.Code)
	}

	detailResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if detailResp.Code != http.StatusOK {
		t.Fatalf("unexpected session detail status: %d", detailResp.Code)
	}

	if len(auditLogger.entries) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(auditLogger.entries))
	}
	if auditLogger.entries[0].Action != "list" || auditLogger.entries[0].ResourceType != "session" {
		t.Fatalf("unexpected list audit entry: %+v", auditLogger.entries[0])
	}
	if auditLogger.entries[0].Metadata["status_filter"] != "open" {
		t.Fatalf("unexpected list audit metadata: %+v", auditLogger.entries[0].Metadata)
	}
	if auditLogger.entries[1].Action != "get" || auditLogger.entries[1].ResourceID != accepted.SessionIDs[0] {
		t.Fatalf("unexpected get audit entry: %+v", auditLogger.entries[1])
	}
}

func TestSessionTraceReturnsAuditAndKnowledge(t *testing.T) {
	t.Parallel()

	auditLogger := &captureAuditLogger{}
	knowledgeSvc := &fakeKnowledgeService{
		trace: contracts.SessionKnowledgeTrace{
			Available:      true,
			DocumentID:     "doc-trace-1",
			Title:          "DiskTrace on host-trace",
			Summary:        "Disk pressure linked to temp files.",
			ContentPreview: "Session: ses-123\nConversation:\n- operator(yue): 查看磁盘使用情况",
			Conversation: []string{
				"operator(yue): 查看磁盘使用情况",
				"tars(diagnosis): Recommend checking disk usage and cleanup candidates.",
			},
			UpdatedAt: time.Date(2026, 3, 12, 8, 15, 0, 0, time.UTC),
		},
	}
	system := newTestSystemWithExecutorAuditAndKnowledge(t, true, false, false, defaultTestConfig(), &fakeExecutor{
		result: actionssh.Result{
			ExitCode: 0,
			Output:   "hostname\n 10:00 up 1 day",
		},
	}, auditLogger, knowledgeSvc)

	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "DiskTrace",
					"instance": "host-trace",
					"severity": "warning"
				},
				"annotations": {
					"summary": "trace smoke"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)

	detailResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if detailResp.Code != http.StatusOK {
		t.Fatalf("unexpected session detail status: %d", detailResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, detailResp, &sessionDetail)
	if sessionDetail.GoldenSummary == nil || sessionDetail.GoldenSummary.NotificationHeadline == "" {
		t.Fatalf("expected notification headline in golden summary, got %+v", sessionDetail.GoldenSummary)
	}
	if len(sessionDetail.Executions) > 0 && len(sessionDetail.Notifications) == 0 {
		t.Fatalf("expected notification digests when notifications exist, got %+v", sessionDetail.Notifications)
	}

	traceResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0]+"/trace", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if traceResp.Code != http.StatusOK {
		t.Fatalf("unexpected trace status: %d", traceResp.Code)
	}

	var trace dto.SessionTraceResponse
	decodeRecorderJSON(t, traceResp, &trace)
	if trace.SessionID != accepted.SessionIDs[0] {
		t.Fatalf("unexpected trace session id: %+v", trace)
	}
	if len(trace.AuditEntries) == 0 {
		t.Fatalf("expected audit entries, got %+v", trace)
	}
	if trace.Knowledge == nil || trace.Knowledge.DocumentID != "doc-trace-1" {
		t.Fatalf("expected knowledge trace, got %+v", trace.Knowledge)
	}
	if len(trace.Knowledge.Conversation) != 2 {
		t.Fatalf("unexpected knowledge conversation: %+v", trace.Knowledge)
	}
}

func TestAuditListReturnsPaginatedRecords(t *testing.T) {
	t.Parallel()

	auditLogger := &captureAuditLogger{
		records: []audit.Record{
			{ResourceType: "session", ResourceID: "ses-1", Action: "get", Actor: "ops_api", CreatedAt: time.Date(2026, 3, 13, 3, 0, 0, 0, time.UTC)},
			{ResourceType: "execution", ResourceID: "exe-1", Action: "get", Actor: "ops_api", CreatedAt: time.Date(2026, 3, 13, 2, 0, 0, 0, time.UTC)},
			{ResourceType: "telegram_chat", ResourceID: "tg-1", Action: "receive", Actor: "alice", CreatedAt: time.Date(2026, 3, 13, 1, 0, 0, 0, time.UTC)},
		},
	}
	system := newTestSystemWithExecutorAndAudit(t, false, false, false, defaultTestConfig(), &fakeExecutor{}, auditLogger)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/audit?resource_type=session&limit=1&page=1&sort_by=created_at&sort_order=desc", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected audit list status: %d body=%s", resp.Code, resp.Body.String())
	}

	var result dto.AuditListResponse
	decodeRecorderJSON(t, resp, &result)
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ResourceType != "session" {
		t.Fatalf("unexpected audit list response: %+v", result)
	}
}

func TestKnowledgeListReturnsPaginatedRecords(t *testing.T) {
	t.Parallel()

	knowledgeSvc := &fakeKnowledgeService{
		listItems: []contracts.KnowledgeRecordDetail{
			{
				DocumentID: "doc-1",
				SessionID:  "ses-knowledge-1",
				Title:      "CPU spike on host-a",
				Summary:    "High load traced to backup job",
				UpdatedAt:  time.Date(2026, 3, 13, 5, 0, 0, 0, time.UTC),
			},
			{
				DocumentID: "doc-2",
				SessionID:  "ses-knowledge-2",
				Title:      "Memory leak on host-b",
				Summary:    "Leak suspected in worker process",
				UpdatedAt:  time.Date(2026, 3, 12, 5, 0, 0, 0, time.UTC),
			},
		},
	}
	system := newTestSystemWithExecutorAuditAndKnowledge(t, false, false, false, defaultTestConfig(), &fakeExecutor{}, audit.NewNoop(), knowledgeSvc)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/knowledge?q=CPU&limit=1&page=1&sort_by=updated_at&sort_order=desc", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected knowledge list status: %d body=%s", resp.Code, resp.Body.String())
	}

	var result dto.KnowledgeListResponse
	decodeRecorderJSON(t, resp, &result)
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].DocumentID != "doc-1" {
		t.Fatalf("unexpected knowledge list response: %+v", result)
	}
}

func TestOpsSummaryReturnsAggregatesAndAudit(t *testing.T) {
	t.Parallel()

	auditLogger := &captureAuditLogger{}
	system := newTestSystemWithExecutorAndAudit(t, false, true, true, defaultTestConfig(), &fakeExecutor{}, auditLogger)

	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "SummaryRead",
					"instance": "host-summary",
					"service": "sshd",
					"severity": "warning"
				},
				"annotations": {
					"summary": "summary read smoke"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	summaryResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/summary", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if summaryResp.Code != http.StatusOK {
		t.Fatalf("unexpected summary status: %d", summaryResp.Code)
	}

	var summary dto.OpsSummaryResponse
	decodeRecorderJSON(t, summaryResp, &summary)
	if summary.ActiveSessions != 1 || summary.PendingApprovals != 0 {
		t.Fatalf("unexpected session counts: %+v", summary)
	}
	if summary.BlockedOutbox != 1 || summary.VisibleOutbox != 1 {
		t.Fatalf("unexpected outbox counts: %+v", summary)
	}
	if len(auditLogger.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditLogger.entries))
	}
	if auditLogger.entries[0].ResourceType != "ops_dashboard" || auditLogger.entries[0].Action != "get" {
		t.Fatalf("unexpected audit entry: %+v", auditLogger.entries[0])
	}
}

func TestOpsSummaryRequiresAuthorization(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/summary", nil, nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.Code)
	}
}

func TestSetupStatusAllowsFirstRunWithoutAuthorization(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/setup/status", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected first-run setup status access, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestSetupWizardAllowsFirstRunWithoutAuthorization(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/setup/wizard", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected first-run wizard access, got %d body=%s", resp.Code, resp.Body.String())
	}

	var payload dto.SetupWizardResponse
	decodeRecorderJSON(t, resp, &payload)
	if payload.Initialization.Mode != "wizard" || payload.Initialization.Initialized {
		t.Fatalf("unexpected initialization payload: %+v", payload.Initialization)
	}
	if payload.Initialization.NextStep != "admin" {
		t.Fatalf("expected admin next step, got %+v", payload.Initialization)
	}
}

func TestBootstrapStatusAllowsAnonymousAccessBeforeInitialization(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/bootstrap/status", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected anonymous bootstrap status before initialization, got %d body=%s", resp.Code, resp.Body.String())
	}

	var payload dto.BootstrapStatusResponse
	decodeRecorderJSON(t, resp, &payload)
	if payload.Initialized {
		t.Fatalf("expected uninitialized bootstrap payload, got %+v", payload)
	}
	if payload.Mode != "wizard" {
		t.Fatalf("expected wizard mode before initialization, got %+v", payload)
	}
	if payload.NextStep != "admin" {
		t.Fatalf("expected admin next step, got %+v", payload)
	}
}

func TestSetupWizardAdminStepAutoConfiguresLocalPasswordAuth(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/admin", []byte(`{
		"username":"setup-admin",
		"display_name":"Setup Admin",
		"password":"Password-123!"
	}`), nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected admin step status: %d body=%s", resp.Code, resp.Body.String())
	}

	var wizard dto.SetupWizardResponse
	decodeRecorderJSON(t, resp, &wizard)
	if !wizard.Initialization.AuthConfigured {
		t.Fatalf("expected auth to be auto-configured after admin step, got %+v", wizard.Initialization)
	}
	if wizard.Initialization.AuthProviderID != "local_password" {
		t.Fatalf("expected local_password auth provider, got %+v", wizard.Initialization)
	}
	if wizard.Initialization.NextStep != "provider" {
		t.Fatalf("expected provider to become next step, got %+v", wizard.Initialization)
	}
	if wizard.Initialization.LoginHint.Provider != "local_password" || wizard.Initialization.LoginHint.Username != "setup-admin" {
		t.Fatalf("expected login hint to target local_password admin login, got %+v", wizard.Initialization.LoginHint)
	}
}

func TestSetupWizardRejectsWeakAdminPassword(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/admin", []byte(`{
		"username":"setup-admin",
		"display_name":"Setup Admin",
		"password":"weak"
	}`), nil)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected weak password validation error, got %d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "password must") {
		t.Fatalf("expected password complexity guidance, got body=%s", resp.Body.String())
	}
}

func TestSetupWizardPersistsMinimalInitializationFlow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	secretsPath := dir + "/secrets.yaml"
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer providerServer.Close()
	if err := os.WriteFile(secretsPath, []byte(`secrets:
  entries:
    - ref: secret://providers/setup-openai/api-key
      value: test-key
`), 0o600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}
	cfg := defaultTestConfig()
	cfg.Reasoning.SecretsConfigPath = secretsPath
	cfg.Connectors.SecretsPath = secretsPath
	cfg.Telegram.BotToken = "bot-token"
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	adminResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/admin", []byte(`{"username":"setup-admin","display_name":"Setup Admin","email":"setup@example.com","password":"Password-123!"}`), nil)
	if adminResp.Code != http.StatusOK {
		t.Fatalf("unexpected admin step status: %d body=%s", adminResp.Code, adminResp.Body.String())
	}

	authResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/auth", []byte(`{"type":"local_password","name":"Local Password"}`), nil)
	if authResp.Code != http.StatusOK {
		t.Fatalf("unexpected auth step status: %d body=%s", authResp.Code, authResp.Body.String())
	}

	providerResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/provider", []byte(`{"provider_id":"setup-openai","vendor":"openai","protocol":"openai_compatible","base_url":"`+providerServer.URL+`","api_key_ref":"secret://providers/setup-openai/api-key","model":"gpt-4o-mini"}`), nil)
	if providerResp.Code != http.StatusOK {
		t.Fatalf("unexpected provider step status: %d body=%s", providerResp.Code, providerResp.Body.String())
	}

	channelResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/channel", []byte(`{"channel_id":"ops-room","name":"Ops Room","type":"telegram","target":"-10012345"}`), nil)
	if channelResp.Code != http.StatusOK {
		t.Fatalf("unexpected channel step status: %d body=%s", channelResp.Code, channelResp.Body.String())
	}

	completeResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/complete", nil, nil)
	if completeResp.Code != http.StatusOK {
		t.Fatalf("unexpected complete step status: %d body=%s", completeResp.Code, completeResp.Body.String())
	}

	statusResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/setup/status", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if statusResp.Code != http.StatusOK {
		t.Fatalf("unexpected setup status after wizard: %d body=%s", statusResp.Code, statusResp.Body.String())
	}
	var status dto.SetupStatusResponse
	decodeRecorderJSON(t, statusResp, &status)
	if !status.Initialization.Initialized || status.Initialization.Mode != "runtime" {
		t.Fatalf("expected runtime mode after completion, got %+v", status.Initialization)
	}
	if status.Initialization.AdminUserID != "setup-admin" || status.Initialization.AuthProviderID != "local_password" {
		t.Fatalf("unexpected persisted setup ids: %+v", status.Initialization)
	}
	if !strings.Contains(status.Initialization.LoginHint.LoginURL, "next=%2Fruntime-checks") {
		t.Fatalf("expected setup login hint to target runtime checks, got %+v", status.Initialization.LoginHint)
	}

	meResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/auth/login", []byte(`{"provider_id":"local_password","username":"setup-admin","password":"Password-123!"}`), nil)
	if meResp.Code != http.StatusOK {
		t.Fatalf("expected wizard-created admin login to work, got %d body=%s", meResp.Code, meResp.Body.String())
	}
	var login dto.AuthLoginResponse
	decodeRecorderJSON(t, meResp, &login)
	if strings.TrimSpace(login.SessionToken) == "" {
		t.Fatalf("expected session token from setup admin login, got %+v", login)
	}
}

func TestBootstrapStatusAllowsAnonymousAccessAfterInitialization(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	secretsPath := dir + "/secrets.yaml"
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer providerServer.Close()
	if err := os.WriteFile(secretsPath, []byte(`secrets:
  entries:
    - ref: secret://providers/setup-openai/api-key
      value: test-key
`), 0o600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}
	cfg := defaultTestConfig()
	cfg.Reasoning.SecretsConfigPath = secretsPath
	cfg.Connectors.SecretsPath = secretsPath
	cfg.Telegram.BotToken = "bot-token"
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/admin", []byte(`{"username":"setup-admin","display_name":"Setup Admin","email":"setup@example.com","password":"Password-123!"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/auth", []byte(`{"type":"local_password","name":"Local Password"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/provider", []byte(`{"provider_id":"setup-openai","vendor":"openai","protocol":"openai_compatible","base_url":"`+providerServer.URL+`","api_key_ref":"secret://providers/setup-openai/api-key","model":"gpt-4o-mini"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/channel", []byte(`{"channel_id":"ops-room","name":"Ops Room","type":"telegram","target":"-10012345"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/complete", nil, nil)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/bootstrap/status", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected anonymous bootstrap status after initialization, got %d body=%s", resp.Code, resp.Body.String())
	}

	var payload dto.BootstrapStatusResponse
	decodeRecorderJSON(t, resp, &payload)
	if !payload.Initialized || payload.Mode != "runtime" {
		t.Fatalf("expected runtime bootstrap payload after initialization, got %+v", payload)
	}
}

func TestSetupStatusRequiresAuthorizationAfterInitialization(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	secretsPath := dir + "/secrets.yaml"
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer providerServer.Close()
	if err := os.WriteFile(secretsPath, []byte(`secrets:
  entries:
    - ref: secret://providers/setup-openai/api-key
      value: test-key
`), 0o600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Reasoning.SecretsConfigPath = secretsPath
	cfg.Connectors.SecretsPath = secretsPath
	cfg.Telegram.BotToken = "bot-token"
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/admin", []byte(`{"username":"setup-admin","display_name":"Setup Admin","email":"setup@example.com","password":"Password-123!"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/auth", []byte(`{"type":"local_password","name":"Local Password"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/provider", []byte(`{"provider_id":"setup-openai","vendor":"openai","protocol":"openai_compatible","base_url":"`+providerServer.URL+`","api_key_ref":"secret://providers/setup-openai/api-key","model":"gpt-4o-mini"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/channel", []byte(`{"channel_id":"ops-room","name":"Ops Room","type":"telegram","target":"-10012345"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/complete", nil, nil)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/setup/status", nil, nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected anonymous setup status after initialization to require auth, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestSetupWizardRequiresSecretRefAndProviderConnectivity(t *testing.T) {
	t.Parallel()

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer providerServer.Close()

	dir := t.TempDir()
	secretsPath := dir + "/secrets.yaml"
	if err := os.WriteFile(secretsPath, []byte(`secrets:
  entries:
    - ref: secret://providers/primary-openai/api-key
      value: test-key
`), 0o600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Reasoning.SecretsConfigPath = secretsPath
	cfg.Connectors.SecretsPath = secretsPath
	cfg.Telegram.BotToken = "bot-token"
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/admin", []byte(`{"username":"admin","password":"Password-123!"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/auth", []byte(`{"type":"local_password"}`), nil)

	missingSecretResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/provider", []byte(`{
  "provider_id":"primary-openai",
  "base_url":"`+providerServer.URL+`",
  "api_key_ref":"secret://providers/missing/api-key",
  "model":"gpt-4o-mini"
}`), nil)
	if missingSecretResp.Code != http.StatusBadRequest {
		t.Fatalf("expected missing secret validation error, got %d body=%s", missingSecretResp.Code, missingSecretResp.Body.String())
	}

	providerResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/provider", []byte(`{
  "provider_id":"primary-openai",
  "vendor":"openai",
  "protocol":"openai_compatible",
  "base_url":"`+providerServer.URL+`",
  "api_key_ref":"secret://providers/primary-openai/api-key",
  "model":"gpt-4o-mini"
}`), nil)
	if providerResp.Code != http.StatusOK {
		t.Fatalf("expected provider save success, got %d body=%s", providerResp.Code, providerResp.Body.String())
	}

	var wizard dto.SetupWizardResponse
	decodeRecorderJSON(t, providerResp, &wizard)
	if !wizard.Initialization.ProviderChecked || !wizard.Initialization.ProviderCheckOK {
		t.Fatalf("expected provider check state, got %+v", wizard.Initialization)
	}
	if strings.TrimSpace(wizard.Initialization.ProviderCheckNote) == "" {
		t.Fatalf("expected provider check note, got %+v", wizard.Initialization)
	}
	if wizard.Initialization.LoginHint.Provider != "local_password" {
		t.Fatalf("expected login hint provider, got %+v", wizard.Initialization.LoginHint)
	}
	if !strings.Contains(wizard.Initialization.LoginHint.LoginURL, "/login?") {
		t.Fatalf("expected login hint URL, got %+v", wizard.Initialization.LoginHint)
	}

	state, err := system.runtimeConfig.LoadSetupState(context.Background())
	if err != nil {
		t.Fatalf("load setup state: %v", err)
	}
	if !state.ProviderChecked || !state.ProviderCheckOK {
		t.Fatalf("expected persisted provider check state, got %+v", state)
	}

	invalidChannelResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/channel", []byte(`{
  "channel_id":"default-ops-room",
  "type":"telegram",
  "target":"not-a-chat-id"
}`), nil)
	if invalidChannelResp.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid telegram target error, got %d body=%s", invalidChannelResp.Code, invalidChannelResp.Body.String())
	}
}

func TestSetupWizardAcceptsRawAPIKeyAndPersistsGeneratedSecretRef(t *testing.T) {
	t.Parallel()

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer providerServer.Close()

	dir := t.TempDir()
	secretsPath := dir + "/secrets.yaml"

	cfg := defaultTestConfig()
	cfg.Reasoning.SecretsConfigPath = secretsPath
	cfg.Connectors.SecretsPath = secretsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/admin", []byte(`{"username":"admin","password":"Password-123!"}`), nil)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/provider", []byte(`{
	  "provider_id":"primary-openai",
	  "vendor":"openai",
	  "protocol":"openai_compatible",
	  "base_url":"`+providerServer.URL+`",
	  "api_key":"sk-test-setup-key",
	  "model":"gpt-4o-mini"
	}`), nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected raw api key setup provider success, got %d body=%s", resp.Code, resp.Body.String())
	}

	var wizard dto.SetupWizardResponse
	decodeRecorderJSON(t, resp, &wizard)
	if got := wizard.Provider.Provider.APIKeyRef; got != "secret://providers/primary-openai/api-key" {
		t.Fatalf("expected generated secret ref, got %q", got)
	}

	content, err := os.ReadFile(secretsPath)
	if err != nil {
		t.Fatalf("read secrets path: %v", err)
	}
	if !strings.Contains(string(content), "ref: secret://providers/primary-openai/api-key") || !strings.Contains(string(content), "value: sk-test-setup-key") {
		t.Fatalf("expected generated provider secret in secret store, got %s", string(content))
	}
	if wizard.Provider.Provider.APIKey != "" {
		t.Fatalf("expected provider response to omit raw api key, got %+v", wizard.Provider.Provider)
	}
}

func TestSetupWizardProviderCheckAllowsFirstRunWithoutAuthorization(t *testing.T) {
	t.Parallel()

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer providerServer.Close()

	dir := t.TempDir()
	secretsPath := dir + "/secrets.yaml"
	if err := os.WriteFile(secretsPath, []byte(`secrets:
  entries:
    - ref: secret://providers/primary-openai/api-key
      value: test-key
`), 0o600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Reasoning.SecretsConfigPath = secretsPath
	cfg.Connectors.SecretsPath = secretsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/provider/check", []byte(`{
	  "provider_id":"primary-openai",
	  "vendor":"openai",
	  "protocol":"openai_compatible",
	  "base_url":"`+providerServer.URL+`",
	  "api_key_ref":"secret://providers/primary-openai/api-key",
	  "model":"gpt-4o-mini"
	}`), nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected anonymous setup provider check success, got %d body=%s", resp.Code, resp.Body.String())
	}

	var checked dto.ProviderCheckResponse
	decodeRecorderJSON(t, resp, &checked)
	if !checked.Available {
		t.Fatalf("expected available provider check, got %+v", checked)
	}
}

func TestSetupWizardProviderCheckAllowsConnectivityOnlyWithoutModel(t *testing.T) {
	t.Parallel()

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4.1"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer providerServer.Close()

	dir := t.TempDir()
	secretsPath := dir + "/secrets.yaml"
	if err := os.WriteFile(secretsPath, []byte(`secrets:
  entries:
    - ref: secret://providers/primary-openai/api-key
      value: test-key
`), 0o600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Reasoning.SecretsConfigPath = secretsPath
	cfg.Connectors.SecretsPath = secretsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/provider/check", []byte(`{
	  "provider_id":"primary-openai",
	  "vendor":"openai",
	  "protocol":"openai_compatible",
	  "base_url":"`+providerServer.URL+`",
	  "api_key_ref":"secret://providers/primary-openai/api-key"
	}`), nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected connectivity-only setup provider check success, got %d body=%s", resp.Code, resp.Body.String())
	}

	var checked dto.ProviderCheckResponse
	decodeRecorderJSON(t, resp, &checked)
	if !checked.Available {
		t.Fatalf("expected available provider connectivity check, got %+v", checked)
	}
	if !strings.Contains(checked.Detail, "list_models ok") {
		t.Fatalf("expected connectivity-only detail to report list-models success, got %+v", checked)
	}
}

func TestSetupWizardTelegramChannelRequiresBotToken(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/admin", []byte(`{"username":"admin","password":"Password-123!"}`), nil)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/channel", []byte(`{
	  "channel_id":"telegram-main",
	  "type":"telegram",
	  "target":"-10012345"
	}`), nil)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected telegram bot token validation error, got %d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "telegram bot token") {
		t.Fatalf("expected telegram token guidance, got body=%s", resp.Body.String())
	}
}

func TestSetupWizardChannelAcceptsKindAndUsagesCompatibility(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/admin", []byte(`{"username":"admin","password":"Password-123!"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/auth", []byte(`{"type":"local_password"}`), nil)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/channel", []byte(`{
	  "channel_id":"inbox-primary",
	  "name":"Primary Inbox",
	  "kind":"in_app_inbox",
	  "target":"default",
	  "usages":["approval","notifications"]
	}`), nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected compatibility channel save success, got %d body=%s", resp.Code, resp.Body.String())
	}

	var wizard dto.SetupWizardResponse
	decodeRecorderJSON(t, resp, &wizard)
	if wizard.Channel.Channel.Kind != "in_app_inbox" {
		t.Fatalf("expected channel kind to be preserved, got %+v", wizard.Channel.Channel)
	}
	if wizard.Channel.Channel.Type != "in_app_inbox" {
		t.Fatalf("expected legacy type to mirror kind, got %+v", wizard.Channel.Channel)
	}
	if len(wizard.Channel.Channel.Usages) != 2 || wizard.Channel.Channel.Usages[0] != "approval" {
		t.Fatalf("expected usages to be preserved, got %+v", wizard.Channel.Channel)
	}
	if len(wizard.Channel.Channel.Capabilities) != 2 || wizard.Channel.Channel.Capabilities[0] != "approval" {
		t.Fatalf("expected legacy capabilities to mirror usages, got %+v", wizard.Channel.Channel)
	}
	if wizard.Initialization.DefaultChannelID != "inbox-primary" {
		t.Fatalf("expected default channel id to persist, got %+v", wizard.Initialization)
	}

	channel, found := system.access.GetChannel("inbox-primary")
	if !found {
		t.Fatalf("expected saved channel to exist")
	}
	if channel.Kind != "in_app_inbox" || channel.Type != "in_app_inbox" {
		t.Fatalf("expected saved channel kind/type compatibility, got %+v", channel)
	}
	if len(channel.Usages) != 2 || len(channel.Capabilities) != 2 {
		t.Fatalf("expected saved channel usages/capabilities compatibility, got %+v", channel)
	}
}

func TestSetupWizardChannelAcceptsLegacyCapabilitiesCompatibility(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.Telegram.BotToken = "bot-token"
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/admin", []byte(`{"username":"admin","password":"Password-123!"}`), nil)
	performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/auth", []byte(`{"type":"local_password"}`), nil)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/setup/wizard/channel", []byte(`{
	  "channel_id":"telegram-legacy",
	  "name":"Legacy Telegram",
	  "type":"telegram",
	  "target":"-10012345",
	  "capabilities":["notifications"]
	}`), nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected legacy capabilities save success, got %d body=%s", resp.Code, resp.Body.String())
	}

	var wizard dto.SetupWizardResponse
	decodeRecorderJSON(t, resp, &wizard)
	if len(wizard.Channel.Channel.Usages) != 1 || wizard.Channel.Channel.Usages[0] != "notifications" {
		t.Fatalf("expected usages to mirror legacy capabilities, got %+v", wizard.Channel.Channel)
	}
	if len(wizard.Channel.Channel.Capabilities) != 1 || wizard.Channel.Channel.Capabilities[0] != "notifications" {
		t.Fatalf("expected capabilities to be preserved, got %+v", wizard.Channel.Channel)
	}

	channel, found := system.access.GetChannel("telegram-legacy")
	if !found {
		t.Fatalf("expected saved channel to exist")
	}
	if len(channel.Usages) != 1 || channel.Usages[0] != "notifications" {
		t.Fatalf("expected saved channel usages from legacy capabilities, got %+v", channel)
	}
	if len(channel.Capabilities) != 1 || channel.Capabilities[0] != "notifications" {
		t.Fatalf("expected saved channel capabilities to persist, got %+v", channel)
	}
}

func TestTriggerHTTPAcceptsGovernanceCompatibility(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	headers := map[string]string{"Authorization": "Bearer ops-token"}

	createResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/triggers", []byte(`{
	  "display_name":"Governed Inbox Rule",
	  "description":"advanced review path",
	  "event_type":"on_execution_completed",
	  "channel_id":"inbox-primary",
	  "automation_job_id":"daily-health",
	  "governance":"advanced_review",
	  "filter_expr":"severity == 'critical'",
	  "target_audience":"ops.primary",
	  "template_id":"execution_result-zh-CN",
	  "cooldown_sec":30,
	  "operator_reason":"create governance rule"
	}`), headers)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected trigger create success, got %d body=%s", createResp.Code, createResp.Body.String())
	}

	var created dto.TriggerDTO
	decodeRecorderJSON(t, createResp, &created)
	if created.Governance != "advanced_review" {
		t.Fatalf("expected governance in create response, got %+v", created)
	}
	if created.AutomationJobID != "daily-health" {
		t.Fatalf("expected automation_job_id in create response, got %+v", created)
	}

	detailResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/triggers/"+created.ID, nil, headers)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("expected trigger detail success, got %d body=%s", detailResp.Code, detailResp.Body.String())
	}

	var detail dto.TriggerDTO
	decodeRecorderJSON(t, detailResp, &detail)
	if detail.Governance != "advanced_review" {
		t.Fatalf("expected governance in detail response, got %+v", detail)
	}
	if detail.AutomationJobID != "daily-health" {
		t.Fatalf("expected automation_job_id in detail response, got %+v", detail)
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/triggers/"+created.ID, []byte(`{
	  "display_name":"Governed Inbox Rule",
	  "description":"escalated",
	  "event_type":"on_execution_completed",
	  "channel_id":"inbox-primary",
	  "automation_job_id":"daily-health",
	  "governance":"org_guardrail",
	  "filter_expr":"severity in ['warning','critical']",
	  "target_audience":"ops.leads",
	  "template_id":"execution_result-zh-CN",
	  "cooldown_sec":60,
	  "operator_reason":"update governance rule"
	}`), headers)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected trigger update success, got %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	decodeRecorderJSON(t, updateResp, &detail)
	if detail.Governance != "org_guardrail" {
		t.Fatalf("expected updated governance in response, got %+v", detail)
	}
}

func TestTriggerHTTPRejectsUnsupportedRegistryChannelKind(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	headers := map[string]string{"Authorization": "Bearer ops-token"}

	if _, err := system.access.UpsertChannel(access.Channel{
		ID:      "slack-primary",
		Name:    "Primary Slack",
		Kind:    "slack",
		Type:    "slack",
		Target:  "#ops",
		Enabled: true,
	}); err != nil {
		t.Fatalf("upsert slack channel: %v", err)
	}

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/triggers", []byte(`{
	  "display_name":"Slack Rule",
	  "description":"unsupported direct delivery",
	  "event_type":"on_execution_completed",
	  "channel_id":"slack-primary",
	  "governance":"advanced_review",
	  "operator_reason":"create unsupported rule"
	}`), headers)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected trigger create validation failure, got %d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "unsupported trigger delivery channel kind") {
		t.Fatalf("expected unsupported channel kind error, got %s", resp.Body.String())
	}
}

func TestOpsRootServesWebConsoleSPA(t *testing.T) {
	t.Parallel()

	distDir := t.TempDir()
	if err := os.WriteFile(distDir+"/index.html", []byte("<!doctype html><html><body><div id=\"root\">tars</div></body></html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(distDir+"/assets", 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(distDir+"/assets/app.js", []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Web = config.WebConfig{DistDir: distDir}
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	loginResp := performJSONRequest(t, system.handler, http.MethodGet, "/login", nil, nil)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("expected login page, got %d", loginResp.Code)
	}
	if !strings.Contains(loginResp.Body.String(), "<div id=\"root\">") {
		t.Fatalf("expected spa shell, got %s", loginResp.Body.String())
	}

	assetResp := performJSONRequest(t, system.handler, http.MethodGet, "/assets/app.js", nil, nil)
	if assetResp.Code != http.StatusOK {
		t.Fatalf("expected asset response, got %d", assetResp.Code)
	}
	if !strings.Contains(assetResp.Body.String(), "console.log('ok')") {
		t.Fatalf("unexpected asset body: %s", assetResp.Body.String())
	}
}

func TestSetupStatusReturnsRuntimeStateAndLatestSmoke(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	dir := t.TempDir()
	keyPath := t.TempDir() + "/id_rsa"
	if err := os.WriteFile(keyPath, []byte("test-key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	approvalPath := dir + "/approvals.yaml"
	if err := os.WriteFile(approvalPath, []byte(`approval:
  prohibit_self_approval: true
  routing:
    service_owner:
      sshd:
        - "445308292"
  execution:
    command_allowlist:
      sshd:
        - "systemctl restart sshd"
`), 0o600); err != nil {
		t.Fatalf("write approvals: %v", err)
	}
	promptPath := dir + "/prompts.yaml"
	if err := os.WriteFile(promptPath, []byte(`reasoning:
  system_prompt: |
    You are TARS.
  user_prompt_template: |
    session_id={{ .SessionID }}
    context={{ .ContextJSON }}
`), 0o600); err != nil {
		t.Fatalf("write prompts: %v", err)
	}
	desensePath := dir + "/desensitization.yaml"
	if err := os.WriteFile(desensePath, []byte(`desensitization:
  enabled: true
  secrets:
    key_names: [password, token]
    query_key_names: [token]
    redact_bearer: true
    redact_basic_auth_url: true
    redact_sk_tokens: true
  placeholders:
    host_key_fragments: [host]
    path_key_fragments: [path, file]
    replace_inline_ip: true
    replace_inline_host: true
    replace_inline_path: true
  rehydration:
    host: true
    ip: true
    path: true
  local_llm_assist:
    enabled: false
    provider: openai_compatible
    mode: detect_only
`), 0o600); err != nil {
		t.Fatalf("write desensitization config: %v", err)
	}
	providersPath := dir + "/providers.yaml"
	if err := os.WriteFile(providersPath, []byte(`providers:
  primary:
    provider_id: "registry-main"
    model: "gpt-4.1-mini"
  assist:
    provider_id: "registry-assist"
    model: "qwen/qwen3-4b-2507"
  entries:
    - id: "registry-main"
      vendor: "openrouter"
      protocol: "openrouter"
      base_url: "https://openrouter.ai/api/v1"
      enabled: true
    - id: "registry-assist"
      vendor: "lmstudio"
      protocol: "lmstudio"
      base_url: "http://127.0.0.1:1234"
      enabled: true
`), 0o600); err != nil {
		t.Fatalf("write providers config: %v", err)
	}
	cfg.Telegram = config.TelegramConfig{
		BotToken:       "bot-token",
		BaseURL:        "https://api.telegram.org",
		PollingEnabled: true,
	}
	cfg.Model = config.ModelConfig{
		BaseURL: "https://model.example.com/v1",
		Model:   "kimi-k2.5",
	}
	cfg.VM = config.VictoriaMetricsConfig{
		BaseURL: "http://vm.example.com:8428",
	}
	cfg.SSH = config.SSHConfig{
		User:                   "root",
		PrivateKeyPath:         keyPath,
		AllowedHosts:           []string{"host-smoke", "host-2"},
		DisableHostKeyChecking: true,
	}
	cfg.Approval.ConfigPath = approvalPath
	cfg.Reasoning.PromptsConfigPath = promptPath
	cfg.Reasoning.DesensitizationConfigPath = desensePath
	cfg.Reasoning.ProvidersConfigPath = providersPath
	cfg.Features = config.FeatureFlags{
		RolloutMode:            "knowledge_on",
		DiagnosisEnabled:       true,
		ApprovalEnabled:        true,
		ExecutionEnabled:       true,
		KnowledgeIngestEnabled: true,
	}

	system := newTestSystemWithConfig(t, true, true, true, cfg)
	system.metrics.RecordComponentResult("telegram", "success", "message delivered")
	system.metrics.RecordComponentResult("model_primary", "success", "primary model ok")
	system.metrics.RecordComponentResult("model_assist", "error", "assist model timeout")
	system.metrics.RecordComponentResult("victoriametrics", "error", "dial tcp timeout")
	system.metrics.RecordComponentResult("ssh", "completed", "command finished")

	smokeResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/smoke/alerts", []byte(`{
		"alertname":"TarsSmokeManual",
		"service":"sshd",
		"host":"host-smoke",
		"severity":"critical",
		"summary":"manual smoke from setup page"
	}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if smokeResp.Code != http.StatusOK {
		t.Fatalf("unexpected smoke status: %d", smokeResp.Code)
	}

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/setup/status", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected setup status code: %d", resp.Code)
	}

	var status dto.SetupStatusResponse
	decodeRecorderJSON(t, resp, &status)
	if status.RolloutMode != "knowledge_on" {
		t.Fatalf("unexpected rollout mode: %+v", status)
	}
	if !status.Telegram.Configured || !status.Telegram.Polling || status.Telegram.Mode != "polling" {
		t.Fatalf("unexpected telegram status: %+v", status.Telegram)
	}
	if status.Telegram.LastResult != "success" {
		t.Fatalf("unexpected telegram runtime status: %+v", status.Telegram)
	}
	if !status.Model.Configured {
		t.Fatalf("unexpected model status: %+v", status.Model)
	}
	if status.Model.ProviderID != "registry-main" || status.Model.Protocol != "openrouter" || status.Model.ModelName != "gpt-4.1-mini" {
		t.Fatalf("expected primary model to reflect provider registry, got %+v", status.Model)
	}
	if status.Model.LastResult != "success" || status.Model.LastDetail != "primary model ok" {
		t.Fatalf("expected primary model runtime status, got %+v", status.Model)
	}
	if !status.AssistModel.Configured || status.AssistModel.ProviderID != "registry-assist" || status.AssistModel.Protocol != "lmstudio" {
		t.Fatalf("unexpected assist model status: %+v", status.AssistModel)
	}
	if status.AssistModel.LastResult != "error" || status.AssistModel.LastDetail != "assist model timeout" {
		t.Fatalf("expected assist model runtime status, got %+v", status.AssistModel)
	}
	if status.LegacyFallbacks != nil {
		t.Fatalf("expected legacy fallbacks to be omitted from connector-first setup status, got %+v", status.LegacyFallbacks)
	}
	if len(status.SmokeDefaults.Hosts) != 2 || status.SmokeDefaults.Hosts[0] != "host-smoke" {
		t.Fatalf("unexpected smoke defaults: %+v", status.SmokeDefaults)
	}
	if status.Authorization.Configured {
		t.Fatalf("expected authorization to be unconfigured by default, got %+v", status.Authorization)
	}
	if !status.Approval.Configured || !status.Approval.Loaded || status.Approval.Path != approvalPath {
		t.Fatalf("unexpected approval status: %+v", status.Approval)
	}
	if !status.Reasoning.Configured || !status.Reasoning.Loaded || status.Reasoning.Path != promptPath {
		t.Fatalf("unexpected reasoning status: %+v", status.Reasoning)
	}
	if !status.Desensitization.Configured || !status.Desensitization.Loaded || status.Desensitization.Path != desensePath || !status.Desensitization.Enabled {
		t.Fatalf("unexpected desensitization status: %+v", status.Desensitization)
	}
	if !status.Providers.Configured || !status.Providers.Loaded || status.Providers.Path != providersPath {
		t.Fatalf("unexpected providers status: %+v", status.Providers)
	}
	if status.LatestSmoke == nil || status.LatestSmoke.AlertName != "TarsSmokeManual" || status.LatestSmoke.Host != "host-smoke" {
		t.Fatalf("unexpected latest smoke: %+v", status.LatestSmoke)
	}
	if status.LatestSmoke.ApprovalRequested {
		t.Fatalf("expected latest smoke to still be analyzing before dispatcher runs: %+v", status.LatestSmoke)
	}
}

func TestSetupStatusSmokeDefaultsPreferAllowedHostsOverLatestSmokeHost(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.SSH.AllowedHosts = []string{"192.168.3.106", "host-2"}
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/smoke/alerts", []byte(`{
		"alertname":"TarsMetricsSmoke",
		"service":"node-exporter",
		"host":"127.0.0.1:9100",
		"severity":"warning",
		"summary":"metrics-only smoke"
	}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected smoke status: %d body=%s", resp.Code, resp.Body.String())
	}

	setupResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/setup/status", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if setupResp.Code != http.StatusOK {
		t.Fatalf("unexpected setup status: %d body=%s", setupResp.Code, setupResp.Body.String())
	}

	var status dto.SetupStatusResponse
	decodeRecorderJSON(t, setupResp, &status)
	if len(status.SmokeDefaults.Hosts) < 2 {
		t.Fatalf("expected smoke defaults to include allowed host and latest smoke host, got %+v", status.SmokeDefaults)
	}
	if status.SmokeDefaults.Hosts[0] != "192.168.3.106" {
		t.Fatalf("expected allowed host to be preferred over latest smoke host, got %+v", status.SmokeDefaults)
	}
	if status.SmokeDefaults.Hosts[1] != "host-2" {
		t.Fatalf("expected remaining allowed hosts to preserve order, got %+v", status.SmokeDefaults)
	}
}

func TestAuthorizationConfigCanBeReadAndUpdatedViaOpsAPI(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	authPath := dir + "/authorization.yaml"
	initialPolicy := `authorization:
  defaults:
    whitelist_action: direct_execute
    blacklist_action: suggest_only
    unmatched_action: require_approval
  ssh_command:
    normalize_whitespace: true
    blacklist:
      - "systemctl restart *"
`
	if err := os.WriteFile(authPath, []byte(initialPolicy), 0o600); err != nil {
		t.Fatalf("write authorization config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Authorization.ConfigPath = authPath
	cfg.SSH.AllowedHosts = []string{"host-1"}

	system := newTestSystemWithConfig(t, true, true, true, cfg)

	getResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/config/authorization", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d", getResp.Code)
	}

	var current dto.AuthorizationConfigResponse
	decodeRecorderJSON(t, getResp, &current)
	if !current.Configured || !current.Loaded || current.Path != authPath {
		t.Fatalf("unexpected current authorization config: %+v", current)
	}
	if current.Config.BlacklistAction != "suggest_only" {
		t.Fatalf("unexpected authorization config payload: %+v", current.Config)
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/config/authorization", []byte(`{
  "content": "authorization:\n  defaults:\n    whitelist_action: direct_execute\n    blacklist_action: require_approval\n    unmatched_action: require_approval\n  ssh_command:\n    normalize_whitespace: true\n    blacklist:\n      - \"systemctl restart *\"\n",
  "operator_reason": "switch restart commands to approval"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	callbackResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram/webhook", []byte(`{
		"update_id": 400001,
		"message": {
			"message_id": 400001,
			"date": 1710000000,
			"text": "重启 nginx",
			"chat": {"id": 445308292, "type": "private"},
			"from": {"id": 445308292, "is_bot": false, "username": "tester", "first_name": "tester"}
		}
	}`), nil)
	if callbackResp.Code != http.StatusOK {
		t.Fatalf("unexpected telegram message status: %d", callbackResp.Code)
	}
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionListResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionListResp.Code != http.StatusOK {
		t.Fatalf("unexpected sessions status: %d", sessionListResp.Code)
	}

	var sessions dto.SessionListResponse
	decodeRecorderJSON(t, sessionListResp, &sessions)
	if len(sessions.Items) == 0 {
		t.Fatalf("expected at least one session")
	}
	if sessions.Items[0].Status != "pending_approval" {
		t.Fatalf("expected updated authorization policy to require approval, got %+v", sessions.Items[0])
	}
	if len(sessions.Items[0].Executions) != 1 {
		t.Fatalf("expected execution draft after approval policy update, got %+v", sessions.Items[0].Executions)
	}
	if sessions.Items[0].Executions[0].Command != "systemctl restart nginx" {
		t.Fatalf("unexpected execution command: %+v", sessions.Items[0].Executions[0])
	}
}

func TestApprovalRoutingConfigCanBeReadAndUpdatedViaOpsAPI(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	approvalPath := dir + "/approvals.yaml"
	initialConfig := `approval:
  prohibit_self_approval: true
  routing:
    service_owner:
      sshd:
        - "445308292"
    oncall_group:
      default:
        - "ops-room"
  execution:
    command_allowlist:
      sshd:
        - "systemctl restart sshd"
`
	if err := os.WriteFile(approvalPath, []byte(initialConfig), 0o600); err != nil {
		t.Fatalf("write approval config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Approval.ConfigPath = approvalPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	getResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/config/approval-routing", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d", getResp.Code)
	}

	var current dto.ApprovalRoutingConfigResponse
	decodeRecorderJSON(t, getResp, &current)
	if !current.Configured || !current.Loaded || current.Path != approvalPath {
		t.Fatalf("unexpected approval routing snapshot: %+v", current)
	}
	if len(current.Config.ServiceOwners) != 1 || current.Config.ServiceOwners[0].Key != "sshd" {
		t.Fatalf("unexpected approval routing config payload: %+v", current.Config)
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/config/approval-routing", []byte(`{
  "config": {
    "prohibit_self_approval": true,
    "service_owners": [{"key":"sshd","targets":["445308292","445308293"]}],
    "oncall_groups": [{"key":"default","targets":["ops-room"]}],
    "command_allowlist": [{"key":"sshd","targets":["systemctl restart sshd","journalctl -u sshd"]}]
  },
  "operator_reason": "expand sshd approvers"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	var updated dto.ApprovalRoutingConfigResponse
	decodeRecorderJSON(t, updateResp, &updated)
	if len(updated.Config.ServiceOwners) != 1 || len(updated.Config.ServiceOwners[0].Targets) != 2 {
		t.Fatalf("unexpected updated approval routing config: %+v", updated.Config)
	}
}

func TestReasoningPromptConfigCanBeReadAndUpdatedViaOpsAPI(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	promptPath := dir + "/prompts.yaml"
	initialConfig := `reasoning:
  system_prompt: |
    You are TARS.
  user_prompt_template: |
    session_id={{ .SessionID }}
    context={{ .ContextJSON }}
`
	if err := os.WriteFile(promptPath, []byte(initialConfig), 0o600); err != nil {
		t.Fatalf("write prompt config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Reasoning.PromptsConfigPath = promptPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	getResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/config/reasoning-prompts", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d", getResp.Code)
	}

	var current dto.ReasoningPromptConfigResponse
	decodeRecorderJSON(t, getResp, &current)
	if !current.Configured || !current.Loaded || current.Path != promptPath {
		t.Fatalf("unexpected prompt snapshot: %+v", current)
	}
	if !strings.Contains(current.Config.SystemPrompt, "You are TARS") {
		t.Fatalf("unexpected prompt payload: %+v", current.Config)
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/config/reasoning-prompts", []byte(`{
  "config": {
    "system_prompt": "You are TARS, optimize for safe command proposals.",
    "user_prompt_template": "session_id={{ .SessionID }}\ncontext={{ .ContextJSON }}"
  },
  "operator_reason": "tighten system prompt"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	var updated dto.ReasoningPromptConfigResponse
	decodeRecorderJSON(t, updateResp, &updated)
	if !strings.Contains(updated.Config.SystemPrompt, "safe command proposals") {
		t.Fatalf("unexpected updated prompt payload: %+v", updated.Config)
	}
}

func TestDesensitizationConfigCanBeReadAndUpdatedViaOpsAPI(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	desensePath := dir + "/desensitization.yaml"
	initialConfig := `desensitization:
  enabled: true
  secrets:
    key_names:
      - password
      - token
    query_key_names:
      - token
    additional_patterns:
      - "corp-[A-Z0-9]{6}"
    redact_bearer: true
    redact_basic_auth_url: true
    redact_sk_tokens: true
  placeholders:
    host_key_fragments:
      - host
    path_key_fragments:
      - path
      - file
    replace_inline_ip: true
    replace_inline_host: true
    replace_inline_path: true
  rehydration:
    host: true
    ip: true
    path: false
  local_llm_assist:
    enabled: false
    provider: openai_compatible
    mode: detect_only
`
	if err := os.WriteFile(desensePath, []byte(initialConfig), 0o600); err != nil {
		t.Fatalf("write desensitization config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Reasoning.DesensitizationConfigPath = desensePath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	getResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/config/desensitization", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d", getResp.Code)
	}

	var current dto.DesensitizationConfigResponse
	decodeRecorderJSON(t, getResp, &current)
	if !current.Configured || !current.Loaded || current.Path != desensePath {
		t.Fatalf("unexpected desensitization snapshot: %+v", current)
	}
	if current.Config.Rehydration.Path {
		t.Fatalf("expected path rehydration to be disabled, got %+v", current.Config)
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/config/desensitization", []byte(`{
  "config": {
    "enabled": true,
    "secrets": {
      "key_names": ["password","cookie"],
      "query_key_names": ["token","api_key"],
      "additional_patterns": ["corp-[A-Z0-9]{6}"],
      "redact_bearer": true,
      "redact_basic_auth_url": true,
      "redact_sk_tokens": true
    },
    "placeholders": {
      "host_key_fragments": ["host","peer"],
      "path_key_fragments": ["path","file"],
      "replace_inline_ip": true,
      "replace_inline_host": true,
      "replace_inline_path": true
    },
    "rehydration": {
      "host": true,
      "ip": true,
      "path": false
    },
    "local_llm_assist": {
      "enabled": true,
      "provider": "openai_compatible",
      "base_url": "http://127.0.0.1:11434/v1",
      "model": "qwen2.5",
      "mode": "detect_only"
    }
  },
  "operator_reason": "prepare local llm assisted detection and keep path rehydration off"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	var updated dto.DesensitizationConfigResponse
	decodeRecorderJSON(t, updateResp, &updated)
	if !updated.Config.LocalLLMAssist.Enabled || updated.Config.LocalLLMAssist.Model != "qwen2.5" {
		t.Fatalf("unexpected updated desensitization config: %+v", updated.Config)
	}
}

func TestProvidersConfigCanBeReadUpdatedAndProbedViaOpsAPI(t *testing.T) {
	t.Parallel()

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4.1-mini"},{"id":"qwen/qwen3-4b-2507"}]}`))
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary\":\"ok\",\"execution_hint\":\"\"}"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer providerServer.Close()

	dir := t.TempDir()
	providersPath := dir + "/providers.yaml"
	initialConfig := `providers:
  primary:
    provider_id: "openai-main"
    model: "gpt-4.1-mini"
  assist:
    provider_id: "openai-main"
    model: "qwen/qwen3-4b-2507"
  entries:
    - id: "openai-main"
      vendor: "openai"
      protocol: "openai_compatible"
      base_url: "` + providerServer.URL + `"
      api_key: "test-secret"
      enabled: true
`
	if err := os.WriteFile(providersPath, []byte(initialConfig), 0o600); err != nil {
		t.Fatalf("write providers config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Reasoning.ProvidersConfigPath = providersPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	getResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/config/providers", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d", getResp.Code)
	}

	var current dto.ProvidersConfigResponse
	decodeRecorderJSON(t, getResp, &current)
	if !current.Configured || !current.Loaded || current.Path != providersPath {
		t.Fatalf("unexpected providers snapshot: %+v", current)
	}
	if current.Config.Primary.ProviderID != "openai-main" || len(current.Config.Entries) != 1 {
		t.Fatalf("unexpected providers payload: %+v", current.Config)
	}
	if !current.Config.Entries[0].APIKeySet {
		t.Fatalf("expected API key marker to be preserved: %+v", current.Config.Entries[0])
	}
	if strings.Contains(current.Content, "test-secret") {
		t.Fatalf("expected providers content to redact secrets, got %q", current.Content)
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/config/providers", []byte(`{
  "config": {
    "primary": {"provider_id":"openai-main","model":"gpt-4.1-mini"},
    "assist": {"provider_id":"openai-main","model":"qwen/qwen3-4b-2507"},
    "entries": [{
      "id":"openai-main",
      "vendor":"openai",
      "protocol":"openai_compatible",
      "base_url":"`+providerServer.URL+`",
      "enabled":true
    }]
  },
  "operator_reason": "centralize provider registry"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	var updated dto.ProvidersConfigResponse
	decodeRecorderJSON(t, updateResp, &updated)
	if updated.Config.Primary.Model != "gpt-4.1-mini" || !updated.Config.Entries[0].APIKeySet {
		t.Fatalf("unexpected updated providers payload: %+v", updated.Config)
	}

	modelsResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/config/providers/models", []byte(`{
  "provider_id": "openai-main"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if modelsResp.Code != http.StatusOK {
		t.Fatalf("unexpected models status: %d body=%s", modelsResp.Code, modelsResp.Body.String())
	}

	var models dto.ProviderListModelsResponse
	decodeRecorderJSON(t, modelsResp, &models)
	if models.ProviderID != "openai-main" || len(models.Models) != 2 {
		t.Fatalf("unexpected models response: %+v", models)
	}

	checkResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/config/providers/check", []byte(`{
  "provider_id": "openai-main",
  "model": "gpt-4.1-mini"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if checkResp.Code != http.StatusOK {
		t.Fatalf("unexpected check status: %d body=%s", checkResp.Code, checkResp.Body.String())
	}

	var checked dto.ProviderCheckResponse
	decodeRecorderJSON(t, checkResp, &checked)
	if !checked.Available {
		t.Fatalf("expected provider to be available, got %+v", checked)
	}
}

func TestProviderBindingsCanBeReadAndUpdatedViaRegistryAPI(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	providersPath := dir + "/providers.yaml"
	initialConfig := `providers:
  primary:
    provider_id: "openai-main"
    model: "gpt-4.1-mini"
  assist:
    provider_id: "openai-main"
    model: "qwen/qwen3-4b-2507"
  entries:
    - id: "openai-main"
      vendor: "openai"
      protocol: "openai_compatible"
      base_url: "https://api.openai.example/v1"
      api_key_ref: "provider/openai-main/api_key"
      enabled: true
`
	if err := os.WriteFile(providersPath, []byte(initialConfig), 0o600); err != nil {
		t.Fatalf("write providers config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Reasoning.ProvidersConfigPath = providersPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	getResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/providers/bindings", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d body=%s", getResp.Code, getResp.Body.String())
	}

	var current struct {
		Configured bool   `json:"configured"`
		Loaded     bool   `json:"loaded"`
		Path       string `json:"path,omitempty"`
		Bindings   struct {
			Primary dto.ProviderBinding `json:"primary"`
			Assist  dto.ProviderBinding `json:"assist"`
		} `json:"bindings"`
	}
	decodeRecorderJSON(t, getResp, &current)
	if !current.Configured || !current.Loaded || current.Path != providersPath {
		t.Fatalf("unexpected bindings response: %+v", current)
	}
	if current.Bindings.Primary.ProviderID != "openai-main" || current.Bindings.Assist.Model != "qwen/qwen3-4b-2507" {
		t.Fatalf("unexpected bindings payload: %+v", current.Bindings)
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/providers/bindings", []byte(`{
  "bindings": {
    "primary": {"provider_id":"openai-main","model":"gpt-4.1-mini"},
    "assist": {"provider_id":"openai-main","model":"qwen/qwen3-32b"}
  },
  "operator_reason": "move provider bindings to providers page"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	var updated struct {
		Bindings struct {
			Primary dto.ProviderBinding `json:"primary"`
			Assist  dto.ProviderBinding `json:"assist"`
		} `json:"bindings"`
	}
	decodeRecorderJSON(t, updateResp, &updated)
	if updated.Bindings.Assist.Model != "qwen/qwen3-32b" {
		t.Fatalf("unexpected updated bindings payload: %+v", updated.Bindings)
	}

	configResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/config/providers", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if configResp.Code != http.StatusOK {
		t.Fatalf("unexpected config status: %d body=%s", configResp.Code, configResp.Body.String())
	}

	var snapshot dto.ProvidersConfigResponse
	decodeRecorderJSON(t, configResp, &snapshot)
	if snapshot.Config.Assist.Model != "qwen/qwen3-32b" {
		t.Fatalf("expected bindings update to persist in providers config, got %+v", snapshot.Config)
	}
}

func TestProvidersCheckFallsBackWhenListModelsUnsupported(t *testing.T) {
	t.Parallel()

	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			http.NotFound(w, r)
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary\":\"ok\",\"execution_hint\":\"\"}"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer providerServer.Close()

	dir := t.TempDir()
	providersPath := dir + "/providers.yaml"
	if err := os.WriteFile(providersPath, []byte(`providers:
  primary:
    provider_id: "lmstudio-local"
    model: "qwen/qwen3-4b-2507"
  entries:
    - id: "lmstudio-local"
      vendor: "lmstudio"
      protocol: "lmstudio"
      base_url: "`+providerServer.URL+`"
      enabled: true
`), 0o600); err != nil {
		t.Fatalf("write providers config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Reasoning.ProvidersConfigPath = providersPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	checkResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/config/providers/check", []byte(`{
  "provider_id": "lmstudio-local",
  "model": "qwen/qwen3-4b-2507"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if checkResp.Code != http.StatusOK {
		t.Fatalf("unexpected check status: %d body=%s", checkResp.Code, checkResp.Body.String())
	}

	var checked dto.ProviderCheckResponse
	decodeRecorderJSON(t, checkResp, &checked)
	if !checked.Available {
		t.Fatalf("expected provider to be available, got %+v", checked)
	}
	if !strings.Contains(checked.Detail, "minimal inference succeeded") {
		t.Fatalf("expected fallback detail, got %+v", checked)
	}
}

func TestConnectorsConfigCanBeReadUpdatedImportedAndShownInSetup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	connectorsPath := dir + "/connectors.yaml"
	initialConfig := `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: prometheus-main
        name: prometheus
        display_name: Prometheus Main
        vendor: prometheus
        version: 1.0.0
      spec:
        type: metrics
        protocol: prometheus_http
        connection_form:
          - key: base_url
            label: Base URL
            type: string
            required: true
          - key: bearer_token
            label: Bearer Token
            type: secret
            required: false
            secret: true
        import_export:
          exportable: true
          importable: true
          formats: ["yaml","json"]
      config:
        values:
          base_url: https://prom.example.test
          bearer_token: secret-token
      compatibility:
        tars_major_versions: ["1"]
        upstream_major_versions: ["2"]
        modes: ["managed"]
      marketplace:
        category: observability
        source: official
`
	if err := os.WriteFile(connectorsPath, []byte(initialConfig), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	getResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/config/connectors", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected get status: %d body=%s", getResp.Code, getResp.Body.String())
	}

	var current dto.ConnectorsConfigResponse
	decodeRecorderJSON(t, getResp, &current)
	if !current.Configured || !current.Loaded || current.Path != connectorsPath {
		t.Fatalf("unexpected connectors snapshot: %+v", current)
	}
	if len(current.Config.Entries) != 1 || current.Config.Entries[0].Metadata.ID != "prometheus-main" {
		t.Fatalf("unexpected connectors payload: %+v", current.Config)
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/config/connectors", []byte(`{
  "config": {
    "entries": [{
      "api_version": "tars.connector/v1alpha1",
      "kind": "connector",
      "metadata": {
        "id": "prometheus-main",
        "name": "prometheus",
        "display_name": "Prometheus Main",
        "vendor": "prometheus",
        "version": "1.1.0"
      },
      "spec": {
        "type": "metrics",
        "protocol": "prometheus_http",
        "import_export": {
          "exportable": true,
          "importable": true,
          "formats": ["yaml", "json", "tar.gz"]
        }
      },
      "compatibility": {
        "tars_major_versions": ["1"],
        "upstream_major_versions": ["2"],
        "modes": ["managed", "imported"]
      },
      "marketplace": {
        "category": "observability",
        "tags": ["metrics", "prometheus"],
        "source": "official"
      }
    }]
  },
  "operator_reason": "promote connectors config to v1.1.0"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	var updated dto.ConnectorsConfigResponse
	decodeRecorderJSON(t, updateResp, &updated)
	if updated.Config.Entries[0].Metadata.Version != "1.1.0" {
		t.Fatalf("unexpected updated connector version: %+v", updated.Config.Entries[0])
	}

	importResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/config/connectors/import", []byte(`{
  "manifest": {
    "api_version": "tars.connector/v1alpha1",
    "kind": "connector",
    "metadata": {
      "id": "jumpserver-main",
      "name": "jumpserver",
      "display_name": "JumpServer Main",
      "vendor": "jumpserver",
      "version": "1.0.0"
    },
    "spec": {
      "type": "execution",
      "protocol": "jumpserver_api",
      "capabilities": [{
        "id": "command.execute",
        "action": "execute",
        "read_only": false,
        "scopes": ["execution.approved"]
      }],
      "import_export": {
        "exportable": true,
        "importable": true,
        "formats": ["yaml", "json", "tar.gz"]
      }
    },
    "compatibility": {
      "tars_major_versions": ["1"],
      "upstream_major_versions": ["3"],
      "modes": ["managed"]
    },
    "marketplace": {
      "category": "execution",
      "tags": ["jumpserver"],
      "source": "official"
    }
  },
  "operator_reason": "import official jumpserver manifest"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if importResp.Code != http.StatusOK {
		t.Fatalf("unexpected import status: %d body=%s", importResp.Code, importResp.Body.String())
	}

	var imported dto.ConnectorsConfigResponse
	decodeRecorderJSON(t, importResp, &imported)
	if len(imported.Config.Entries) != 2 {
		t.Fatalf("expected two connector entries after import, got %+v", imported.Config)
	}

	setupResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/setup/status", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if setupResp.Code != http.StatusOK {
		t.Fatalf("unexpected setup status: %d body=%s", setupResp.Code, setupResp.Body.String())
	}

	var setup dto.SetupStatusResponse
	decodeRecorderJSON(t, setupResp, &setup)
	if !setup.Connectors.Configured || setup.Connectors.TotalEntries != 2 {
		t.Fatalf("unexpected connectors setup status: %+v", setup.Connectors)
	}
	if !containsString(setup.Connectors.Kinds, "metrics") || !containsString(setup.Connectors.Kinds, "execution") {
		t.Fatalf("unexpected connector kinds: %+v", setup.Connectors.Kinds)
	}
}

func TestConnectorsCanBeCreatedAndUpdatedViaRegistryAPI(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	connectorsPath := dir + "/connectors.yaml"
	if err := os.WriteFile(connectorsPath, []byte("connectors:\n  entries: []\n"), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	createResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors", []byte(`{
  "manifest": {
    "api_version": "tars.connector/v1alpha1",
    "kind": "connector",
    "enabled": true,
    "metadata": {
      "id": "prometheus-page",
      "name": "prometheus",
      "display_name": "Prometheus Page",
      "vendor": "prometheus",
      "version": "1.0.0"
    },
    "spec": {
      "type": "metrics",
      "protocol": "prometheus_http",
      "capabilities": [],
      "connection_form": [
        {"key":"base_url","label":"Base URL","type":"string","required":true},
        {"key":"bearer_token","label":"Bearer Token","type":"secret","required":false,"secret":true}
      ],
      "import_export": {"exportable":true,"importable":true,"formats":["yaml","json"]}
    },
    "config": {
      "values": {"base_url":"https://prom.example.test"},
      "secret_refs": {"bearer_token":"secret://connector/prometheus-page/bearer_token"}
    },
    "compatibility": {
      "tars_major_versions": ["1"],
      "upstream_major_versions": ["2"],
      "modes": ["managed"]
    },
    "marketplace": {"category":"observability","tags":["metrics"],"source":"custom"}
  },
  "operator_reason": "create connector from connectors page"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createResp.Code, createResp.Body.String())
	}

	var created dto.ConnectorManifest
	decodeRecorderJSON(t, createResp, &created)
	if created.Metadata.ID != "prometheus-page" {
		t.Fatalf("unexpected connector created: %+v", created)
	}
	if len(created.Config.Values) == 0 || created.Config.Values["base_url"] != "https://prom.example.test" {
		t.Fatalf("expected authenticated create response to include editable config values, got %+v", created.Config)
	}

	publicDetailResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors/prometheus-page", nil, nil)
	if publicDetailResp.Code != http.StatusOK {
		t.Fatalf("unexpected public detail status: %d body=%s", publicDetailResp.Code, publicDetailResp.Body.String())
	}
	var publicDetail dto.ConnectorManifest
	decodeRecorderJSON(t, publicDetailResp, &publicDetail)
	if len(publicDetail.Config.Values) > 0 {
		t.Fatalf("expected public detail to hide runtime values, got %+v", publicDetail.Config)
	}

	authedDetailResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors/prometheus-page", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if authedDetailResp.Code != http.StatusOK {
		t.Fatalf("unexpected authed detail status: %d body=%s", authedDetailResp.Code, authedDetailResp.Body.String())
	}
	var authedDetail dto.ConnectorManifest
	decodeRecorderJSON(t, authedDetailResp, &authedDetail)
	if len(authedDetail.Config.Values) == 0 || authedDetail.Config.Values["base_url"] != "https://prom.example.test" {
		t.Fatalf("expected authenticated detail to include editable config values, got %+v", authedDetail.Config)
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/connectors/prometheus-page", []byte(`{
  "manifest": {
    "api_version": "tars.connector/v1alpha1",
    "kind": "connector",
    "enabled": true,
    "metadata": {
      "id": "prometheus-page",
      "name": "prometheus",
      "display_name": "Prometheus Production",
      "vendor": "prometheus",
      "version": "1.0.1"
    },
    "spec": {
      "type": "metrics",
      "protocol": "prometheus_http",
      "capabilities": [],
      "connection_form": [
        {"key":"base_url","label":"Base URL","type":"string","required":true},
        {"key":"bearer_token","label":"Bearer Token","type":"secret","required":false,"secret":true}
      ],
      "import_export": {"exportable":true,"importable":true,"formats":["yaml","json"]}
    },
    "config": {
      "values": {"base_url":"https://prom.prod.example.test"},
      "secret_refs": {"bearer_token":"secret://connector/prometheus-page/bearer_token"}
    },
    "compatibility": {
      "tars_major_versions": ["1"],
      "upstream_major_versions": ["2"],
      "modes": ["managed"]
    },
    "marketplace": {"category":"observability","tags":["metrics","prod"],"source":"custom"}
  },
  "operator_reason": "update connector from connectors page"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	var updated dto.ConnectorManifest
	decodeRecorderJSON(t, updateResp, &updated)
	if updated.Metadata.DisplayName != "Prometheus Production" || updated.Metadata.Version != "1.0.1" {
		t.Fatalf("unexpected updated connector metadata: %+v", updated.Metadata)
	}
	if len(updated.Config.Values) == 0 || updated.Config.Values["base_url"] != "https://prom.prod.example.test" {
		t.Fatalf("expected updated connector config values, got %+v", updated.Config)
	}
}

func TestConnectorProbeValidatesDraftWithoutPersisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	connectorsPath := dir + "/connectors.yaml"
	if err := os.WriteFile(connectorsPath, []byte("connectors:\n  entries: []\n"), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	probeResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/probe", []byte(`{
  "manifest": {
    "api_version": "tars.connector/v1alpha1",
    "kind": "connector",
    "enabled": true,
    "metadata": {
      "id": "jumpserver-draft",
      "name": "jumpserver",
      "display_name": "JumpServer Draft",
      "vendor": "jumpserver",
      "version": "1.0.0"
    },
    "spec": {
      "type": "execution",
      "protocol": "jumpserver_api",
      "capabilities": [],
      "connection_form": [
        {"key":"base_url","label":"Base URL","type":"string","required":true},
        {"key":"access_key","label":"Access Key","type":"secret","required":true,"secret":true},
        {"key":"secret_key","label":"Secret Key","type":"secret","required":true,"secret":true}
      ],
      "import_export": {"exportable":true,"importable":true,"formats":["yaml","json"]}
    },
    "config": {
      "values": {
        "base_url":"https://jumpserver.example.test",
        "access_key":"ak-test",
        "secret_key":"sk-test"
      }
    },
    "compatibility": {
      "tars_major_versions": ["1"],
      "upstream_major_versions": ["3"],
      "modes": ["managed"]
    },
    "marketplace": {"category":"execution","tags":["jumpserver"],"source":"custom"}
  }
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if probeResp.Code != http.StatusOK {
		t.Fatalf("unexpected probe status: %d body=%s", probeResp.Code, probeResp.Body.String())
	}

	var probed dto.ConnectorLifecycle
	decodeRecorderJSON(t, probeResp, &probed)
	if probed.ConnectorID != "jumpserver-draft" {
		t.Fatalf("unexpected probe connector id: %+v", probed)
	}
	if probed.Health.Status != "healthy" || !strings.Contains(probed.Health.Summary, "jumpserver API probe succeeded") {
		t.Fatalf("expected probe response to expose runtime health, got %+v", probed.Health)
	}

	listResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors", nil, nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("unexpected connectors list status: %d body=%s", listResp.Code, listResp.Body.String())
	}
	var listed dto.ConnectorListResponse
	decodeRecorderJSON(t, listResp, &listed)
	if listed.Total != 0 || len(listed.Items) != 0 {
		t.Fatalf("expected draft probe to avoid persistence, got %+v", listed)
	}
}

func TestConnectorsConfigWorksWithoutFilePathUsingRuntimeStore(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = ""
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/config/connectors", []byte(`{
  "config": {
    "entries": [{
      "api_version":"tars.connector/v1alpha1",
      "kind":"connector",
      "enabled":true,
      "metadata":{"id":"jumpserver-main","name":"jumpserver","display_name":"JumpServer Main","vendor":"jumpserver","version":"1.0.0"},
      "spec":{
        "type":"execution",
        "protocol":"jumpserver_api",
        "import_export":{"exportable":true,"importable":true,"formats":["yaml"]}
      },
      "config":{"secret_refs":{"api_token":"secret://connectors/jumpserver-main/api-token"}},
      "compatibility":{"tars_major_versions":["1"],"upstream_major_versions":["3"],"modes":["managed"]},
      "marketplace":{"category":"execution","source":"official"}
    }]
  },
  "operator_reason":"bootstrap connectors in runtime db"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected connectors update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	getResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/config/connectors", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected connectors get status: %d body=%s", getResp.Code, getResp.Body.String())
	}

	var current dto.ConnectorsConfigResponse
	decodeRecorderJSON(t, getResp, &current)
	if current.Configured {
		t.Fatalf("expected runtime-backed connectors config to report unconfigured path, got %+v", current)
	}
	if len(current.Config.Entries) != 1 || current.Config.Entries[0].Metadata.ID != "jumpserver-main" {
		t.Fatalf("unexpected runtime connectors payload: %+v", current.Config)
	}

	importResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/config/connectors", []byte(`{
	  "config": {
	    "entries": [
	      {
	        "api_version":"tars.connector/v1alpha1",
	        "kind":"connector",
	        "enabled":true,
	        "metadata":{"id":"jumpserver-main","name":"jumpserver","display_name":"JumpServer Main","vendor":"jumpserver","version":"1.0.0"},
	        "spec":{
	          "type":"execution",
	          "protocol":"jumpserver_api",
	          "import_export":{"exportable":true,"importable":true,"formats":["yaml"]}
	        },
	        "config":{"secret_refs":{"api_token":"secret://connectors/jumpserver-main/api-token"}},
	        "compatibility":{"tars_major_versions":["1"],"upstream_major_versions":["3"],"modes":["managed"]},
	        "marketplace":{"category":"execution","source":"official"}
	      },
	      {
	        "api_version":"tars.connector/v1alpha1",
	        "kind":"connector",
	        "enabled":true,
	        "metadata":{"id":"prometheus-main","name":"prometheus","display_name":"Prometheus Main","vendor":"prometheus","version":"1.0.0"},
	        "spec":{
	          "type":"metrics",
	          "protocol":"prometheus_http",
	          "import_export":{"exportable":true,"importable":true,"formats":["yaml"]}
	        },
	        "compatibility":{"tars_major_versions":["1"],"upstream_major_versions":["2"],"modes":["managed"]},
	        "marketplace":{"category":"observability","source":"official"}
	      }
	    ]
	  },
	  "operator_reason":"import runtime metrics connector"
	}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if importResp.Code != http.StatusOK {
		t.Fatalf("unexpected connectors import status: %d body=%s", importResp.Code, importResp.Body.String())
	}
	decodeRecorderJSON(t, importResp, &current)
	if len(current.Config.Entries) != 2 {
		t.Fatalf("expected 2 runtime-backed connectors after update, got %+v", current.Config)
	}
}

func TestConnectorsRegistryCanListDetailAndPopulateDiscovery(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	connectorsPath := dir + "/connectors.yaml"
	configContent := `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: prometheus-main
        name: prometheus
        display_name: Prometheus Main
        vendor: prometheus
        version: 1.0.0
        description: Main Prometheus metrics connector
      spec:
        type: metrics
        protocol: prometheus_http
        capabilities:
          - id: metrics.query
            action: query
            read_only: true
            scopes: ["metrics.read"]
        import_export:
          exportable: true
          importable: true
          formats: ["yaml","json"]
      compatibility:
        tars_major_versions: ["1"]
        upstream_major_versions: ["2"]
        modes: ["managed"]
      marketplace:
        category: observability
        tags: ["metrics","prometheus"]
        source: official
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: jumpserver-main
        name: jumpserver
        display_name: JumpServer Main
        vendor: jumpserver
        version: 1.0.0
        description: Approved execution broker
      spec:
        type: execution
        protocol: jumpserver_api
        capabilities:
          - id: command.execute
            action: execute
            read_only: false
            scopes: ["execution.approved"]
        import_export:
          exportable: true
          importable: true
          formats: ["yaml","json","tar.gz"]
      compatibility:
        tars_major_versions: ["1"]
        upstream_major_versions: ["3"]
        modes: ["managed"]
      marketplace:
        category: execution
        tags: ["execution","jumpserver"]
        source: official
`
	if err := os.WriteFile(connectorsPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	listResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors?type=metrics&limit=1&page=1&sort_by=display_name&sort_order=asc", nil, nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("unexpected connectors list status: %d body=%s", listResp.Code, listResp.Body.String())
	}

	var list dto.ConnectorListResponse
	decodeRecorderJSON(t, listResp, &list)
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].Metadata.ID != "prometheus-main" {
		t.Fatalf("unexpected connector list payload: %+v", list)
	}

	searchResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors?q=jumpserver", nil, nil)
	if searchResp.Code != http.StatusOK {
		t.Fatalf("unexpected connectors search status: %d body=%s", searchResp.Code, searchResp.Body.String())
	}
	decodeRecorderJSON(t, searchResp, &list)
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].Metadata.ID != "jumpserver-main" {
		t.Fatalf("unexpected connector search payload: %+v", list)
	}

	detailResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors/jumpserver-main", nil, nil)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("unexpected connector detail status: %d body=%s", detailResp.Code, detailResp.Body.String())
	}

	var detail dto.ConnectorManifest
	decodeRecorderJSON(t, detailResp, &detail)
	if detail.Metadata.ID != "jumpserver-main" || detail.Spec.Protocol != "jumpserver_api" {
		t.Fatalf("unexpected connector detail payload: %+v", detail)
	}

	discoveryResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/platform/discovery", nil, nil)
	if discoveryResp.Code != http.StatusOK {
		t.Fatalf("unexpected discovery status: %d body=%s", discoveryResp.Code, discoveryResp.Body.String())
	}

	var discovery dto.PlatformDiscoveryResponse
	decodeRecorderJSON(t, discoveryResp, &discovery)
	if discovery.RegisteredConnectorsCount != 2 {
		t.Fatalf("unexpected registered connectors count: %+v", discovery)
	}
	if !containsString(discovery.RegisteredConnectorIDs, "prometheus-main") || !containsString(discovery.RegisteredConnectorIDs, "jumpserver-main") {
		t.Fatalf("unexpected connector ids in discovery: %+v", discovery.RegisteredConnectorIDs)
	}
	if !containsString(discovery.Docs, "/api/v1/connectors") || !containsString(discovery.Docs, "/api/v1/connectors/{id}") {
		t.Fatalf("unexpected discovery docs: %+v", discovery.Docs)
	}
	if !containsString(discovery.RegisteredConnectorKinds, "metrics") || !containsString(discovery.RegisteredConnectorKinds, "execution") {
		t.Fatalf("unexpected discovery connector kinds: %+v", discovery.RegisteredConnectorKinds)
	}
	if !hasToolPlanCapability(discovery.ToolPlanCapabilities, "metrics.query_range", "prometheus-main", "") {
		t.Fatalf("expected prometheus metrics.query_range capability in discovery, got %+v", discovery.ToolPlanCapabilities)
	}
	if !hasToolPlanCapability(discovery.ToolPlanCapabilities, "execution.run_command", "jumpserver-main", "command.execute") {
		t.Fatalf("expected jumpserver execution.run_command capability in discovery, got %+v", discovery.ToolPlanCapabilities)
	}
}

func TestConnectorControlPlaneCanExportToggleAndQueryMetrics(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	connectorsPath := dir + "/connectors.yaml"
	metricsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer secret-token" {
			t.Fatalf("unexpected authorization header: %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"result":[{"metric":{"instance":"192.168.3.106"},"value":[1710000000,"1"]}]}}`))
	}))
	defer metricsServer.Close()

	configContent := fmt.Sprintf(`connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: prometheus-main
        name: prometheus
        display_name: Prometheus Main
        vendor: prometheus
        version: 1.0.0
      spec:
        type: metrics
        protocol: prometheus_http
        connection_form:
          - key: base_url
            label: Base URL
            type: string
            required: true
          - key: bearer_token
            label: Bearer Token
            type: secret
            required: false
            secret: true
        import_export:
          exportable: true
          importable: true
          formats: ["yaml","json"]
      config:
        values:
          base_url: %s
          bearer_token: secret-token
      compatibility:
        tars_major_versions: ["1"]
        upstream_major_versions: ["2"]
        modes: ["managed"]
      marketplace:
        category: observability
        source: official
`, metricsServer.URL)
	if err := os.WriteFile(connectorsPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	exportResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors/prometheus-main/export?format=yaml", nil, nil)
	if exportResp.Code != http.StatusOK {
		t.Fatalf("unexpected connector export status: %d body=%s", exportResp.Code, exportResp.Body.String())
	}
	if got := exportResp.Header().Get("Content-Disposition"); !strings.Contains(got, "prometheus-main-1.0.0.yaml") {
		t.Fatalf("unexpected connector export filename: %s", got)
	}
	if !strings.Contains(exportResp.Body.String(), "prometheus-main") {
		t.Fatalf("expected exported manifest to include connector id, got %s", exportResp.Body.String())
	}
	if strings.Contains(exportResp.Body.String(), "secret-token") {
		t.Fatalf("expected exported manifest to redact connector secrets, got %s", exportResp.Body.String())
	}

	detailResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors/prometheus-main", nil, nil)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("unexpected connector detail status: %d body=%s", detailResp.Code, detailResp.Body.String())
	}

	var publicDetail dto.ConnectorManifest
	decodeRecorderJSON(t, detailResp, &publicDetail)
	if publicDetail.Config.Values["base_url"] != "" || publicDetail.Config.Values["bearer_token"] != "" {
		t.Fatalf("expected public connector detail to omit runtime config values, got %+v", publicDetail.Config.Values)
	}

	disableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/prometheus-main/disable", []byte(`{
  "operator_reason": "temporarily isolate metrics connector"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if disableResp.Code != http.StatusOK {
		t.Fatalf("unexpected connector disable status: %d body=%s", disableResp.Code, disableResp.Body.String())
	}

	var disabled dto.ConnectorManifest
	decodeRecorderJSON(t, disableResp, &disabled)
	if disabled.Enabled == nil || *disabled.Enabled {
		t.Fatalf("expected connector to be disabled, got %+v", disabled)
	}

	queryDisabledResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/prometheus-main/metrics/query", []byte(`{
  "host": "192.168.3.106"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if queryDisabledResp.Code != http.StatusConflict {
		t.Fatalf("expected disabled connector conflict, got %d body=%s", queryDisabledResp.Code, queryDisabledResp.Body.String())
	}

	enableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/prometheus-main/enable", []byte(`{
  "operator_reason": "restore metrics connector"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if enableResp.Code != http.StatusOK {
		t.Fatalf("unexpected connector enable status: %d body=%s", enableResp.Code, enableResp.Body.String())
	}

	var enabled dto.ConnectorManifest
	decodeRecorderJSON(t, enableResp, &enabled)
	if enabled.Enabled == nil || !*enabled.Enabled {
		t.Fatalf("expected connector to be enabled, got %+v", enabled)
	}

	queryResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/prometheus-main/metrics/query", []byte(`{
  "host": "192.168.3.106"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if queryResp.Code != http.StatusOK {
		t.Fatalf("unexpected connector metrics query status: %d body=%s", queryResp.Code, queryResp.Body.String())
	}

	var metrics dto.ConnectorMetricsQueryResponse
	decodeRecorderJSON(t, queryResp, &metrics)
	if metrics.ConnectorID != "prometheus-main" || metrics.Protocol != "prometheus_http" || len(metrics.Series) == 0 {
		t.Fatalf("unexpected connector metrics response: %+v", metrics)
	}

	setupResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/setup/status", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if setupResp.Code != http.StatusOK {
		t.Fatalf("unexpected setup status: %d body=%s", setupResp.Code, setupResp.Body.String())
	}

	var setup dto.SetupStatusResponse
	decodeRecorderJSON(t, setupResp, &setup)
	if setup.Connectors.EnabledEntries != 1 || setup.Connectors.TotalEntries != 1 {
		t.Fatalf("unexpected connectors setup counts: %+v", setup.Connectors)
	}
	if setup.Connectors.MetricsRuntime == nil || setup.Connectors.MetricsRuntime.Primary == nil {
		t.Fatalf("expected metrics runtime primary connector, got %+v", setup.Connectors.MetricsRuntime)
	}
	if setup.Connectors.MetricsRuntime.Component != "prometheus-main" {
		t.Fatalf("expected metrics runtime component to follow selected connector, got %+v", setup.Connectors.MetricsRuntime)
	}
	if setup.Connectors.MetricsRuntime.Fallback == nil || setup.Connectors.MetricsRuntime.Fallback.FallbackUsed {
		t.Fatalf("expected metrics fallback to remain standby only, got %+v", setup.Connectors.MetricsRuntime.Fallback)
	}
	if setup.Connectors.MetricsRuntime.Fallback.FallbackReason != "" {
		t.Fatalf("expected no fallback reason when primary connector is selected, got %+v", setup.Connectors.MetricsRuntime.Fallback)
	}
}

func TestConnectorExecutionAndHealthRuntime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	connectorsPath := dir + "/connectors.yaml"
	configContent := `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: jumpserver-main
        name: jumpserver
        display_name: JumpServer Main
        vendor: jumpserver
        version: 1.0.0
      spec:
        type: execution
        protocol: jumpserver_api
        capabilities:
          - id: command.execute
            action: execute
            read_only: false
            scopes: ["execution.approved"]
        import_export:
          exportable: true
          importable: true
          formats: ["yaml","json","tar.gz"]
      config:
        values:
          base_url: https://jumpserver.example.test
          access_key: ak-test
          secret_key: sk-test
      compatibility:
        tars_major_versions: ["1"]
        upstream_major_versions: ["3"]
        modes: ["managed"]
      marketplace:
        category: execution
        tags: ["jumpserver","execution"]
        source: official
`
	if err := os.WriteFile(connectorsPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	detailResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors/jumpserver-main", nil, nil)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("unexpected connector detail status: %d body=%s", detailResp.Code, detailResp.Body.String())
	}
	var detail dto.ConnectorManifest
	decodeRecorderJSON(t, detailResp, &detail)
	if detail.Lifecycle == nil || detail.Lifecycle.CurrentVersion != "1.0.0" {
		t.Fatalf("expected lifecycle metadata in connector detail, got %+v", detail.Lifecycle)
	}

	healthResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/jumpserver-main/health", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if healthResp.Code != http.StatusOK {
		t.Fatalf("unexpected connector health status: %d body=%s", healthResp.Code, healthResp.Body.String())
	}
	var health dto.ConnectorLifecycle
	decodeRecorderJSON(t, healthResp, &health)
	if health.Health.Status != "healthy" || len(health.HealthHistory) == 0 {
		t.Fatalf("unexpected connector health payload: %+v", health)
	}
	if !strings.Contains(health.Health.Summary, "jumpserver API probe succeeded") {
		t.Fatalf("expected runtime health summary, got %+v", health.Health)
	}
	if health.Compatibility.CheckedAt.IsZero() {
		t.Fatalf("expected refreshed compatibility timestamp, got %+v", health.Compatibility)
	}

	execResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/jumpserver-main/execution/execute", []byte(`{
  "target_host": "192.168.3.106",
  "command": "systemctl restart sshd",
  "service": "sshd",
  "operator_reason": "validate jumpserver runtime",
  "execution_mode": "jumpserver_job"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if execResp.Code != http.StatusOK {
		t.Fatalf("unexpected connector execution status: %d body=%s", execResp.Code, execResp.Body.String())
	}
	var execution dto.ConnectorExecutionResponse
	decodeRecorderJSON(t, execResp, &execution)
	if execution.ConnectorID != "jumpserver-main" || execution.Protocol != "jumpserver_api" || execution.ExecutionMode != "jumpserver_job" {
		t.Fatalf("unexpected connector execution payload: %+v", execution)
	}
	if execution.OutputRef == "" || execution.OutputBytes == 0 {
		t.Fatalf("expected persisted execution output, got %+v", execution)
	}
	if !strings.Contains(execution.OutputPreview, "asset=192.168.3.106") && !strings.Contains(execution.OutputPreview, "module=shell") {
		t.Fatalf("unexpected execution preview: %+v", execution)
	}
}

func TestConnectorUpgradeRollbackAndBulkExports(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	connectorsPath := dir + "/connectors.yaml"
	if err := os.WriteFile(connectorsPath, []byte(`connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: jumpserver-main
        name: jumpserver
        display_name: JumpServer Main
        vendor: jumpserver
        version: 1.0.0
      spec:
        type: execution
        protocol: jumpserver_api
        import_export:
          exportable: true
          importable: true
          formats: ["yaml","json","tar.gz"]
      config:
        values:
          base_url: https://jumpserver.example.test
          access_key: ak-test
          secret_key: sk-test
      compatibility:
        tars_major_versions: ["1"]
`), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	auditLogger := &captureAuditLogger{}
	knowledgeSvc := &fakeKnowledgeService{listItems: []contracts.KnowledgeRecordDetail{{
		DocumentID: "doc-1",
		SessionID:  "ses-1",
		Title:      "Recovered SSHD Session",
		Summary:    "sshd was restarted and verified",
		UpdatedAt:  time.Now().UTC(),
	}}}
	system := newTestSystemWithExecutorAuditAndKnowledge(t, true, true, true, cfg, &fakeExecutor{}, auditLogger, knowledgeSvc)

	upgradeResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/jumpserver-main/upgrade", []byte(`{
  "manifest": {
    "api_version": "tars.connector/v1alpha1",
    "kind": "connector",
    "metadata": {
      "id": "jumpserver-main",
      "name": "jumpserver",
      "display_name": "JumpServer Main",
      "vendor": "jumpserver",
      "version": "1.1.0"
    },
    "spec": {
      "type": "execution",
      "protocol": "jumpserver_api",
      "import_export": {"exportable": true, "importable": true, "formats": ["yaml", "json"]}
    },
    "config": {
      "values": {
        "base_url": "https://jumpserver.example.test",
        "access_key": "ak-test",
        "secret_key": "sk-test",
        "org_id": "ops-org"
      }
    },
    "compatibility": {"tars_major_versions": ["1"]},
    "marketplace": {"category": "execution", "tags": ["jumpserver"], "source": "official"}
  },
  "available_version": "1.1.0",
  "operator_reason": "upgrade jumpserver connector"
}`), map[string]string{"Authorization": "Bearer ops-token"})
	if upgradeResp.Code != http.StatusOK {
		t.Fatalf("unexpected upgrade status: %d body=%s", upgradeResp.Code, upgradeResp.Body.String())
	}
	var upgraded dto.ConnectorManifest
	decodeRecorderJSON(t, upgradeResp, &upgraded)
	if upgraded.Lifecycle == nil || upgraded.Lifecycle.CurrentVersion != "1.1.0" || upgraded.Lifecycle.AvailableVersion != "1.1.0" {
		t.Fatalf("unexpected upgraded lifecycle: %+v", upgraded.Lifecycle)
	}
	if upgraded.Lifecycle.Health.Status != "unknown" || upgraded.Lifecycle.Health.Summary != "runtime health check required after connector change" {
		t.Fatalf("expected pending runtime probe after upgrade, got %+v", upgraded.Lifecycle.Health)
	}

	rollbackResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/jumpserver-main/rollback", []byte(`{
  "target_version": "1.0.0",
  "operator_reason": "rollback after validation"
}`), map[string]string{"Authorization": "Bearer ops-token"})
	if rollbackResp.Code != http.StatusOK {
		t.Fatalf("unexpected rollback status: %d body=%s", rollbackResp.Code, rollbackResp.Body.String())
	}
	var rolledBack dto.ConnectorManifest
	decodeRecorderJSON(t, rollbackResp, &rolledBack)
	if rolledBack.Lifecycle == nil || rolledBack.Lifecycle.CurrentVersion != "1.0.0" || len(rolledBack.Lifecycle.Revisions) == 0 {
		t.Fatalf("unexpected rollback lifecycle: %+v", rolledBack.Lifecycle)
	}
	if rolledBack.Lifecycle.Health.Status != "unknown" || rolledBack.Lifecycle.Health.Summary != "runtime health check required after connector change" {
		t.Fatalf("expected pending runtime probe after rollback, got %+v", rolledBack.Lifecycle.Health)
	}
	if len(rolledBack.Lifecycle.Revisions) != 3 {
		t.Fatalf("expected install, upgrade, rollback revisions, got %+v", rolledBack.Lifecycle.Revisions)
	}
	if rolledBack.Lifecycle.Revisions[0].Version != "1.0.0" || rolledBack.Lifecycle.Revisions[0].Reason != "install" {
		t.Fatalf("expected install revision to be preserved, got %+v", rolledBack.Lifecycle.Revisions[0])
	}
	if rolledBack.Lifecycle.Revisions[2].Version != "1.0.0" || rolledBack.Lifecycle.Revisions[2].Reason != "rollback after validation" {
		t.Fatalf("expected rollback revision to be preserved, got %+v", rolledBack.Lifecycle.Revisions[2])
	}

	performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions", nil, map[string]string{"Authorization": "Bearer ops-token"})
	performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/knowledge", nil, map[string]string{"Authorization": "Bearer ops-token"})

	auditResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/audit/bulk/export", []byte(`{"ids":["1","2"],"operator_reason":"export audit evidence"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if auditResp.Code != http.StatusOK {
		t.Fatalf("unexpected audit export status: %d body=%s", auditResp.Code, auditResp.Body.String())
	}
	var auditExport dto.AuditExportResponse
	decodeRecorderJSON(t, auditResp, &auditExport)
	if auditExport.ExportedCount == 0 || len(auditExport.Items) == 0 {
		t.Fatalf("unexpected audit export payload: %+v", auditExport)
	}

	knowledgeResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/knowledge/bulk/export", []byte(`{"ids":["doc-1"],"operator_reason":"export knowledge evidence"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if knowledgeResp.Code != http.StatusOK {
		t.Fatalf("unexpected knowledge export status: %d body=%s", knowledgeResp.Code, knowledgeResp.Body.String())
	}
	var knowledgeExport dto.KnowledgeExportResponse
	decodeRecorderJSON(t, knowledgeResp, &knowledgeExport)
	if knowledgeExport.ExportedCount != 1 || len(knowledgeExport.Items) != 1 {
		t.Fatalf("unexpected knowledge export payload: %+v", knowledgeExport)
	}
}

func TestConnectorInvokeCapabilityReturnsPendingApproval(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	connectorsPath := dir + "/connectors.yaml"
	if err := os.WriteFile(connectorsPath, []byte(`connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: skill-source-main
        name: skill-source
        display_name: Skill Source Main
        vendor: tars
        version: 1.0.0
      spec:
        type: skill_source
        protocol: http_index
        capabilities:
          - id: source.sync
            action: import
            read_only: false
      compatibility:
        tars_major_versions: ["1"]
`), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/skill-source-main/capabilities/invoke", []byte(`{
	  "capability_id": "source.sync",
	  "params": {"source": "default"}
	}`), map[string]string{"Authorization": "Bearer ops-token"})
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected 202 pending approval, got %d body=%s", resp.Code, resp.Body.String())
	}
	var payload dto.ConnectorInvokeCapabilityResponse
	decodeRecorderJSON(t, resp, &payload)
	if payload.Status != "pending_approval" {
		t.Fatalf("expected pending_approval payload, got %+v", payload)
	}
	if payload.Metadata["rule_id"] != "default" {
		t.Fatalf("expected default rule metadata, got %+v", payload.Metadata)
	}
}

func TestConnectorInvokeCapabilityReturnsForbiddenForHardDeniedSkillSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	connectorsPath := dir + "/connectors.yaml"
	authPath := dir + "/authorization.yaml"
	if err := os.WriteFile(connectorsPath, []byte(`connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: skill-source-main
        name: skill-source
        display_name: Skill Source Main
        vendor: tars
        version: 1.0.0
      spec:
        type: skill_source
        protocol: http_index
        capabilities:
          - id: source.sync
            action: import
            read_only: true
      compatibility:
        tars_major_versions: ["1"]
`), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}
	if err := os.WriteFile(authPath, []byte(`authorization:
  hard_deny:
    mcp_skill:
      - "source.sync"
`), 0o600); err != nil {
		t.Fatalf("write authorization config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	cfg.Authorization.ConfigPath = authPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/skill-source-main/capabilities/invoke", []byte(`{
	  "capability_id": "source.sync",
	  "params": {"source": "default"}
	}`), map[string]string{"Authorization": "Bearer ops-token"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 hard deny, got %d body=%s", resp.Code, resp.Body.String())
	}
	var payload dto.ConnectorInvokeCapabilityResponse
	decodeRecorderJSON(t, resp, &payload)
	if payload.Status != "denied" {
		t.Fatalf("expected denied payload, got %+v", payload)
	}
	if payload.Metadata["matched_by"] != "hard_deny" {
		t.Fatalf("expected hard deny metadata, got %+v", payload.Metadata)
	}
}

func TestSkillRegistryCanListPromoteDisableAndExport(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	listResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/skills", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if listResp.Code != http.StatusOK {
		t.Fatalf("unexpected skills list status: %d body=%s", listResp.Code, listResp.Body.String())
	}
	var listPayload dto.SkillListResponse
	decodeRecorderJSON(t, listResp, &listPayload)
	if len(listPayload.Items) == 0 {
		t.Fatalf("expected bundled skills in list payload")
	}
	if listPayload.Items[0].Metadata.ID == "" {
		t.Fatalf("expected skill id in list payload: %+v", listPayload.Items[0])
	}

	detailResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/skills/disk-space-incident", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if detailResp.Code != http.StatusOK {
		t.Fatalf("unexpected skill detail status: %d body=%s", detailResp.Code, detailResp.Body.String())
	}
	var detail dto.SkillManifest
	decodeRecorderJSON(t, detailResp, &detail)
	if detail.Metadata.ID != "disk-space-incident" {
		t.Fatalf("unexpected skill detail payload: %+v", detail)
	}

	promoteResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/skills/disk-space-incident/promote", []byte(`{"operator_reason":"publish official skill","review_state":"approved","runtime_mode":"planner_visible"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if promoteResp.Code != http.StatusOK {
		t.Fatalf("unexpected skill promote status: %d body=%s", promoteResp.Code, promoteResp.Body.String())
	}
	decodeRecorderJSON(t, promoteResp, &detail)
	if detail.Lifecycle == nil || detail.Lifecycle.Status != "active" || detail.Lifecycle.ReviewState != "approved" {
		t.Fatalf("expected promoted lifecycle, got %+v", detail.Lifecycle)
	}

	disableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/skills/disk-space-incident/disable", []byte(`{"operator_reason":"maintenance window"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if disableResp.Code != http.StatusOK {
		t.Fatalf("unexpected skill disable status: %d body=%s", disableResp.Code, disableResp.Body.String())
	}
	decodeRecorderJSON(t, disableResp, &detail)
	if detail.Enabled || detail.Lifecycle == nil || detail.Lifecycle.Status != "disabled" {
		t.Fatalf("expected disabled skill lifecycle, got %+v", detail)
	}

	enableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/skills/disk-space-incident/enable", []byte(`{"operator_reason":"re-enable after maintenance"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if enableResp.Code != http.StatusOK {
		t.Fatalf("unexpected skill enable status: %d body=%s", enableResp.Code, enableResp.Body.String())
	}
	decodeRecorderJSON(t, enableResp, &detail)
	if !detail.Enabled {
		t.Fatalf("expected enabled skill, got %+v", detail)
	}

	exportResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/skills/disk-space-incident/export?format=json", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if exportResp.Code != http.StatusOK {
		t.Fatalf("unexpected skill export status: %d body=%s", exportResp.Code, exportResp.Body.String())
	}
	if contentType := exportResp.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("expected json content type, got %q", contentType)
	}
	var exported dto.SkillManifest
	decodeRecorderJSON(t, exportResp, &exported)
	if exported.Metadata.ID != "disk-space-incident" {
		t.Fatalf("unexpected exported skill payload: %+v", exported)
	}
}

func TestAutomationRegistryCanListCreateRunAndDisable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	automationsPath := dir + "/automations.yaml"
	connectorsPath := dir + "/connectors.yaml"
	if err := os.WriteFile(connectorsPath, []byte(`connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: skill-source-main
        name: skill-source
        display_name: Skill Source Main
        vendor: tars
        version: 1.0.0
      spec:
        type: skill_source
        protocol: http_index
        capabilities:
          - id: source.sync
            action: import
            read_only: true
      compatibility:
        tars_major_versions: ["1"]
`), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}
	if err := os.WriteFile(automationsPath, []byte(`automations:
  jobs:
    - id: disk-scan
      display_name: Disk Scan
      type: skill
      target_ref: disk-space-incident
      schedule: "@every 30m"
      enabled: true
      owner: ops
      skill:
        skill_id: disk-space-incident
        context:
          host: host-1
          service: api
`), 0o600); err != nil {
		t.Fatalf("write automations config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	cfg.Automations.ConfigPath = automationsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	listResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/automations", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if listResp.Code != http.StatusOK {
		t.Fatalf("unexpected automations list status: %d body=%s", listResp.Code, listResp.Body.String())
	}
	var listPayload dto.AutomationListResponse
	decodeRecorderJSON(t, listResp, &listPayload)
	if len(listPayload.Items) != 1 || listPayload.Items[0].ID != "disk-scan" {
		t.Fatalf("unexpected automation list payload: %+v", listPayload)
	}

	createResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/automations", []byte(`{
  "job": {
    "id": "source-sync",
    "display_name": "Source Sync",
    "type": "connector_capability",
    "target_ref": "skill-source-main/source.sync",
    "schedule": "@every 15m",
    "enabled": true,
    "connector_capability": {
      "connector_id": "skill-source-main",
      "capability_id": "source.sync",
      "params": {"source": "default"}
    }
  }
}`), map[string]string{"Authorization": "Bearer ops-token"})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("unexpected automation create status: %d body=%s", createResp.Code, createResp.Body.String())
	}
	var created dto.AutomationJob
	decodeRecorderJSON(t, createResp, &created)
	if created.ID != "source-sync" || created.Type != "connector_capability" {
		t.Fatalf("unexpected created automation payload: %+v", created)
	}

	runResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/automations/source-sync/run", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if runResp.Code != http.StatusOK {
		t.Fatalf("unexpected automation run status: %d body=%s", runResp.Code, runResp.Body.String())
	}
	var runPayload dto.AutomationJob
	decodeRecorderJSON(t, runResp, &runPayload)
	if runPayload.LastRun == nil || runPayload.LastRun.Status != "completed" {
		t.Fatalf("expected completed run payload, got %+v", runPayload)
	}

	disableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/automations/source-sync/disable", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if disableResp.Code != http.StatusOK {
		t.Fatalf("unexpected automation disable status: %d body=%s", disableResp.Code, disableResp.Body.String())
	}
	decodeRecorderJSON(t, disableResp, &created)
	if created.Enabled {
		t.Fatalf("expected disabled automation, got %+v", created)
	}
}

func TestAutomationRunBlocksHighRiskConnectorCapability(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	automationsPath := dir + "/automations.yaml"
	connectorsPath := dir + "/connectors.yaml"
	if err := os.WriteFile(connectorsPath, []byte(`connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: skill-source-main
        name: skill-source
        display_name: Skill Source Main
        vendor: tars
        version: 1.0.0
      spec:
        type: skill_source
        protocol: http_index
        capabilities:
          - id: source.sync
            action: import
            read_only: false
      compatibility:
        tars_major_versions: ["1"]
`), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}
	if err := os.WriteFile(automationsPath, []byte(`automations:
  jobs:
    - id: sync-job
      display_name: Sync Job
      type: connector_capability
      target_ref: skill-source-main/source.sync
      schedule: "@every 30m"
      enabled: true
      connector_capability:
        connector_id: skill-source-main
        capability_id: source.sync
        params:
          source: default
`), 0o600); err != nil {
		t.Fatalf("write automations config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	cfg.Automations.ConfigPath = automationsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	runResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/automations/sync-job/run", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if runResp.Code != http.StatusOK {
		t.Fatalf("unexpected automation run status: %d body=%s", runResp.Code, runResp.Body.String())
	}
	var payload dto.AutomationJob
	decodeRecorderJSON(t, runResp, &payload)
	if payload.LastRun == nil || payload.LastRun.Status != "blocked" {
		t.Fatalf("expected blocked automation run, got %+v", payload)
	}
}

func TestAutomationRunPublishesInboxStyleNotification(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	automationsPath := dir + "/automations.yaml"
	connectorsPath := dir + "/connectors.yaml"
	if err := os.WriteFile(connectorsPath, []byte(`connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: skill-source-main
        name: skill-source
        display_name: Skill Source Main
        vendor: tars
        version: 1.0.0
      spec:
        type: skill_source
        protocol: stub
        capabilities:
          - id: source.query
            action: query
            read_only: true
            invocable: true
      compatibility:
        tars_major_versions: ["1"]
`), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}
	if err := os.WriteFile(automationsPath, []byte(`automations:
  jobs:
    - id: golden-inspection-stub
      display_name: Golden Inspection
      type: connector_capability
      target_ref: skill-source-main/source.query
      schedule: "@every 15m"
      enabled: true
      connector_capability:
        connector_id: skill-source-main
        capability_id: source.query
        params:
          source: default
`), 0o600); err != nil {
		t.Fatalf("write automations config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	cfg.Automations.ConfigPath = automationsPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	runResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/automations/golden-inspection-stub/run", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if runResp.Code != http.StatusOK {
		t.Fatalf("unexpected automation run status: %d body=%s", runResp.Code, runResp.Body.String())
	}
	var payload dto.AutomationJob
	decodeRecorderJSON(t, runResp, &payload)
	if payload.LastRun == nil || payload.LastRun.Status != "completed" {
		t.Fatalf("expected completed automation run, got %+v", payload)
	}
	if len(system.channel.messages) == 0 {
		t.Fatalf("expected trigger notification message, got none")
	}
	msg := system.channel.messages[len(system.channel.messages)-1]
	if msg.Channel != "in_app_inbox" {
		t.Fatalf("expected inbox channel, got %+v", msg)
	}
	if msg.RefType != "automation_run" || msg.RefID != payload.LastRun.RunID {
		t.Fatalf("unexpected automation notification ref: %+v", msg)
	}
	if !strings.Contains(msg.Body, payload.LastRun.Summary) {
		t.Fatalf("expected notification body to include run summary, got %+v", msg)
	}
}

func TestWebChatFingerprintIncludesHostAndService(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	headers := map[string]string{"Authorization": "Bearer ops-token"}
	bodyA := []byte(`{"message":"检查系统负载和 CPU 使用情况","host":"host-1","service":"api","severity":"info"}`)
	bodyB := []byte(`{"message":"检查系统负载和 CPU 使用情况","host":"host-2","service":"worker","severity":"info"}`)

	respA := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/chat/messages", bodyA, headers)
	if respA.Code != http.StatusOK {
		t.Fatalf("unexpected first chat status: %d body=%s", respA.Code, respA.Body.String())
	}
	respB := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/chat/messages", bodyB, headers)
	if respB.Code != http.StatusOK {
		t.Fatalf("unexpected second chat status: %d body=%s", respB.Code, respB.Body.String())
	}

	var first dto.ChatMessageResponse
	var second dto.ChatMessageResponse
	decodeRecorderJSON(t, respA, &first)
	decodeRecorderJSON(t, respB, &second)
	if first.Duplicated || second.Duplicated {
		t.Fatalf("expected distinct sessions, got first=%+v second=%+v", first, second)
	}
	if first.SessionID == second.SessionID {
		t.Fatalf("expected different sessions for different host/service, got %s", first.SessionID)
	}
}

func TestWebExecutionApproveEndpointCompletesSharedApprovalFlow(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	handler := system.handler
	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "HighCPU",
					"instance": "host-1",
					"service": "api",
					"severity": "critical"
				},
				"annotations": {
					"summary": "cpu too high",
					"user_request": "执行命令查看 api 状态"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionResp := performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if len(sessionDetail.Executions) != 1 {
		t.Fatalf("expected one execution, got %+v", sessionDetail.Executions)
	}

	executionID := sessionDetail.Executions[0].ExecutionID
	approveResp := performJSONRequest(t, handler, http.MethodPost, "/api/v1/executions/"+executionID+"/approve", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if approveResp.Code != http.StatusOK {
		t.Fatalf("unexpected approve status: %d body=%s", approveResp.Code, approveResp.Body.String())
	}

	var executionDetail dto.ExecutionDetail
	decodeRecorderJSON(t, approveResp, &executionDetail)
	if executionDetail.Status != "completed" {
		t.Fatalf("expected completed execution, got %+v", executionDetail)
	}

	sessionResp = performJSONRequest(t, handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status after approve: %d", sessionResp.Code)
	}
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	if sessionDetail.Status != "resolved" {
		t.Fatalf("expected resolved session, got %+v", sessionDetail)
	}

	foundResult := false
	for _, message := range system.channel.messages {
		if message.Channel == "in_app_inbox" && message.RefID == executionID && strings.Contains(message.Body, "[TARS] 执行结果") {
			foundResult = true
			break
		}
	}
	if !foundResult {
		t.Fatalf("expected inbox execution result message, got %+v", system.channel.messages)
	}
}

func TestSkillImportEndpointRegistersCustomSkill(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := dir + "/skills.yaml"
	cfg := defaultTestConfig()
	cfg.Skills.ConfigPath = configPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	body := []byte(`{
	  "operator_reason": "import custom skill",
	  "manifest": {
	    "api_version": "tars.skill/v1alpha1",
	    "kind": "skill_package",
	    "enabled": true,
	    "metadata": {
	      "id": "api-error-after-deploy",
	      "name": "api-error-after-deploy",
	      "display_name": "API Error After Deploy",
	      "version": "0.1.0",
	      "vendor": "team",
	      "source": "imported"
	    },
	    "spec": {
	      "type": "incident_skill",
	      "triggers": {
	        "alerts": ["ApiErrorRateHigh"],
	        "intents": ["api error after deploy"]
	      },
	      "planner": {
	        "summary": "Check logs and recent deploys first.",
	        "preferred_tools": ["observability.query","delivery.query"],
	        "steps": [
	          {"id":"observe_1","tool":"observability.query","required":true,"reason":"Inspect the failing traces.","params":{"query":"error rate api"}},
	          {"id":"delivery_1","tool":"delivery.query","required":false,"reason":"Check the latest deploy.","params":{"service":"api"}}
	        ]
	      }
	    }
	  }
	}`)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/config/skills/import", body, map[string]string{"Authorization": "Bearer ops-token"})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected skill import status: %d body=%s", resp.Code, resp.Body.String())
	}
	var payload dto.SkillImportResponse
	decodeRecorderJSON(t, resp, &payload)
	if payload.Manifest.Metadata.ID != "api-error-after-deploy" {
		t.Fatalf("unexpected imported skill payload: %+v", payload)
	}

	listResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/skills?source=imported", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if listResp.Code != http.StatusOK {
		t.Fatalf("unexpected imported skills list status: %d body=%s", listResp.Code, listResp.Body.String())
	}
	var listPayload dto.SkillListResponse
	decodeRecorderJSON(t, listResp, &listPayload)
	if len(listPayload.Items) != 1 || listPayload.Items[0].Metadata.ID != "api-error-after-deploy" {
		t.Fatalf("unexpected imported skills list payload: %+v", listPayload)
	}
}

func TestSkillRegistryRequiresOperatorReasonForWrites(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := dir + "/skills.yaml"
	cfg := defaultTestConfig()
	cfg.Skills.ConfigPath = configPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	createBody := []byte(`{
	  "manifest": {
	    "api_version": "tars.skill/v1alpha1",
	    "kind": "skill_package",
	    "metadata": {
	      "id": "missing-reason-create",
	      "name": "missing-reason-create",
	      "display_name": "Missing Reason Create",
	      "version": "1.0.0"
	    },
	    "spec": {
	      "type": "incident_skill",
	      "planner": {"steps": []}
	    }
	  }
	}`)
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/api/v1/skills",
			body:   createBody,
		},
		{
			name:   "import",
			method: http.MethodPost,
			path:   "/api/v1/config/skills/import",
			body:   createBody,
		},
		{
			name:   "promote",
			method: http.MethodPost,
			path:   "/api/v1/skills/disk-space-incident/promote",
			body:   []byte(`{"review_state":"approved","runtime_mode":"planner_visible"}`),
		},
		{
			name:   "disable",
			method: http.MethodPost,
			path:   "/api/v1/skills/disk-space-incident/disable",
			body:   []byte(`{}`),
		},
		{
			name:   "enable",
			method: http.MethodPost,
			path:   "/api/v1/skills/disk-space-incident/enable",
			body:   []byte(`{}`),
		},
		{
			name:   "rollback",
			method: http.MethodPost,
			path:   "/api/v1/skills/disk-space-incident/rollback",
			body:   []byte(`{}`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := performJSONRequest(t, system.handler, tc.method, tc.path, tc.body, map[string]string{"Authorization": "Bearer ops-token"})
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("expected validation failure for %s, got %d body=%s", tc.name, resp.Code, resp.Body.String())
			}
			var payload dto.ErrorEnvelope
			decodeRecorderJSON(t, resp, &payload)
			if payload.Error.Code != "validation_failed" || !strings.Contains(payload.Error.Message, "operator_reason is required") {
				t.Fatalf("unexpected validation payload for %s: %+v", tc.name, payload)
			}
		})
	}
}

func TestSkillRegistryRollbackWithoutTargetVersionUsesPreviousRevisionOverHTTP(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := dir + "/skills.yaml"
	cfg := defaultTestConfig()
	cfg.Skills.ConfigPath = configPath
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	createResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/skills", []byte(`{
	  "operator_reason": "create skill",
	  "manifest": {
	    "api_version": "tars.skill/v1alpha1",
	    "kind": "skill_package",
	    "metadata": {
	      "id": "rollback-skill",
	      "name": "rollback-skill",
	      "display_name": "Rollback Skill",
	      "version": "1.0.0",
	      "source": "custom"
	    },
	    "spec": {
	      "type": "incident_skill",
	      "planner": {
	        "steps": [
	          {"id":"step_1","tool":"knowledge.search","params":{"query":"one"}}
	        ]
	      }
	    }
	  }
	}`), map[string]string{"Authorization": "Bearer ops-token"})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("unexpected create status: %d body=%s", createResp.Code, createResp.Body.String())
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/skills/rollback-skill", []byte(`{
	  "operator_reason": "update skill",
	  "manifest": {
	    "api_version": "tars.skill/v1alpha1",
	    "kind": "skill_package",
	    "metadata": {
	      "id": "rollback-skill",
	      "name": "rollback-skill",
	      "display_name": "Rollback Skill",
	      "version": "1.1.0",
	      "source": "custom"
	    },
	    "spec": {
	      "type": "incident_skill",
	      "planner": {
	        "steps": [
	          {"id":"step_2","tool":"knowledge.search","params":{"query":"two"}}
	        ]
	      }
	    }
	  }
	}`), map[string]string{"Authorization": "Bearer ops-token"})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	rollbackResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/skills/rollback-skill/rollback", []byte(`{"operator_reason":"rollback latest previous revision"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if rollbackResp.Code != http.StatusOK {
		t.Fatalf("unexpected rollback status: %d body=%s", rollbackResp.Code, rollbackResp.Body.String())
	}
	var payload dto.SkillManifest
	decodeRecorderJSON(t, rollbackResp, &payload)
	if payload.Metadata.Version != "1.0.0" {
		t.Fatalf("expected rollback to previous version 1.0.0, got %+v", payload.Metadata)
	}
	if payload.Lifecycle == nil || len(payload.Lifecycle.Revisions) < 3 {
		t.Fatalf("expected revisions preserved after rollback, got %+v", payload.Lifecycle)
	}
}

func TestPlatformHardeningSecretsTemplatesAndDashboard(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	connectorsPath := dir + "/connectors.yaml"
	providersPath := dir + "/providers.yaml"
	secretsPath := dir + "/secrets.yaml"

	if err := os.WriteFile(connectorsPath, []byte(`connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: prometheus-main
        name: prometheus
        display_name: Prometheus Main
        vendor: prometheus
        version: 1.0.0
      spec:
        type: metrics
        protocol: prometheus_http
        connection_form:
          - key: base_url
            label: Base URL
            type: string
            required: true
          - key: bearer_token
            label: Bearer Token
            type: secret
            required: false
            secret: true
        import_export:
          exportable: true
          importable: true
          formats: ["yaml","json"]
      config:
        values:
          base_url: https://prom.example.test
        secret_refs:
          bearer_token: connector.prometheus-main.bearer_token
      compatibility:
        tars_major_versions: ["1"]
        upstream_major_versions: ["2"]
        modes: ["managed"]
      marketplace:
        category: observability
        source: official
`), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}
	if err := os.WriteFile(providersPath, []byte(`providers:
  primary:
    provider_id: openai-main
    model: gpt-4.1-mini
  entries:
    - id: openai-main
      vendor: openai
      protocol: openai_compatible
      base_url: https://api.openai.example/v1
      api_key_ref: provider.openai-main.api_key
      enabled: true
`), 0o600); err != nil {
		t.Fatalf("write providers config: %v", err)
	}
	if err := os.WriteFile(secretsPath, []byte(`secrets:
  entries:
    - ref: connector.prometheus-main.bearer_token
      value: prom-secret
      source: secret_store
    - ref: provider.openai-main.api_key
      value: provider-secret
      source: secret_store
`), 0o600); err != nil {
		t.Fatalf("write secrets config: %v", err)
	}

	cfg := defaultTestConfig()
	cfg.Connectors.ConfigPath = connectorsPath
	cfg.Connectors.SecretsPath = secretsPath
	cfg.Reasoning.ProvidersConfigPath = providersPath
	auditLogger := &captureAuditLogger{}
	system := newTestSystemWithExecutorAndAudit(t, true, true, true, cfg, &fakeExecutor{}, auditLogger)

	secretsResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/config/secrets", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if secretsResp.Code != http.StatusOK {
		t.Fatalf("unexpected secrets status: %d body=%s", secretsResp.Code, secretsResp.Body.String())
	}
	var inventory dto.SecretsInventoryResponse
	decodeRecorderJSON(t, secretsResp, &inventory)
	if len(inventory.Items) != 2 {
		t.Fatalf("expected 2 secret descriptors, got %+v", inventory.Items)
	}

	updateResp := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/config/secrets", []byte(`{
  "upserts": [{"ref":"connector.prometheus-main.bearer_token","value":"rotated-token"}],
  "operator_reason": "rotate prometheus token"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("unexpected secrets update status: %d body=%s", updateResp.Code, updateResp.Body.String())
	}

	templatesResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors/templates", nil, nil)
	if templatesResp.Code != http.StatusOK {
		t.Fatalf("unexpected templates status: %d body=%s", templatesResp.Code, templatesResp.Body.String())
	}
	var templates dto.ConnectorTemplateListResponse
	decodeRecorderJSON(t, templatesResp, &templates)
	if len(templates.Items) == 0 {
		t.Fatalf("expected connector templates, got %+v", templates)
	}

	applyResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/connectors/prometheus-main/templates/apply", []byte(`{
  "template_id": "prometheus-pilot",
  "operator_reason": "bootstrap template"
}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if applyResp.Code != http.StatusOK {
		t.Fatalf("unexpected template apply status: %d body=%s", applyResp.Code, applyResp.Body.String())
	}

	healthResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/dashboard/health", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if healthResp.Code != http.StatusOK {
		t.Fatalf("unexpected dashboard health status: %d body=%s", healthResp.Code, healthResp.Body.String())
	}
	var health dto.DashboardHealthResponse
	decodeRecorderJSON(t, healthResp, &health)
	if health.Summary.ConfiguredSecrets != 2 || health.Resources.Goroutines <= 0 {
		t.Fatalf("unexpected dashboard health payload: %+v", health)
	}
	if health.Resources.TracingProvider == "" {
		t.Fatalf("expected tracing provider metadata, got %+v", health.Resources)
	}

	providerModelsResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/config/providers/models", []byte(`{"provider_id":"openai-main"}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if providerModelsResp.Code != http.StatusBadGateway {
		t.Fatalf("expected provider call to attempt remote request, got %d body=%s", providerModelsResp.Code, providerModelsResp.Body.String())
	}

	if !hasAuditEntry(auditLogger.entries, "secret_ref", "connector.prometheus-main.bearer_token", "upsert") {
		t.Fatalf("expected secret_ref audit entry, got %+v", auditLogger.entries)
	}
	if !hasAuditEntry(auditLogger.entries, "connector_template", "prometheus-main", "apply") {
		t.Fatalf("expected connector_template audit entry, got %+v", auditLogger.entries)
	}
}

func TestSmokeAlertRequiresAuthorizationAndValidation(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	unauthorized := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/smoke/alerts", []byte(`{"alertname":"x"}`), nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", unauthorized.Code)
	}

	invalid := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/smoke/alerts", []byte(`{"alertname":"x"}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected validation failure, got %d", invalid.Code)
	}
}

func TestSmokeAlertCreatesSessionAndMarksSmoke(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, false)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/smoke/alerts", []byte(`{
		"alertname":"TarsSmokeManual",
		"service":"sshd",
		"host":"host-smoke",
		"severity":"critical",
		"summary":"manual smoke from setup page"
	}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected smoke status: %d", resp.Code)
	}

	var accepted dto.SmokeAlertResponse
	decodeRecorderJSON(t, resp, &accepted)
	if !accepted.Accepted || accepted.SessionID == "" || accepted.Status != "analyzing" {
		t.Fatalf("unexpected smoke response: %+v", accepted)
	}
	if accepted.TGTarget == "" {
		t.Fatalf("expected telegram target in smoke response: %+v", accepted)
	}

	sessionResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionID, nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session detail status: %d", sessionResp.Code)
	}

	var session dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &session)
	if !session.IsSmoke {
		t.Fatalf("expected smoke session, got %+v", session)
	}
	labels := session.Alert["labels"].(map[string]interface{})
	if labels["tars_smoke"] != "true" {
		t.Fatalf("expected smoke label, got %+v", labels)
	}
	if labels["chat_id"] != accepted.TGTarget || labels["telegram_target"] != accepted.TGTarget {
		t.Fatalf("expected smoke session to persist telegram target, got %+v", labels)
	}
}

func TestSmokeAlertReturnsOpenWhenDiagnosisDisabled(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, false, false, false)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/smoke/alerts", []byte(`{
		"alertname":"TarsSmokeDisabled",
		"service":"sshd",
		"host":"host-smoke",
		"severity":"critical",
		"summary":"manual smoke when diagnosis is disabled"
	}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected smoke status: %d", resp.Code)
	}

	var accepted dto.SmokeAlertResponse
	decodeRecorderJSON(t, resp, &accepted)
	if accepted.Status != "open" {
		t.Fatalf("expected open status, got %+v", accepted)
	}

	outboxResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/outbox?status=blocked", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if outboxResp.Code != http.StatusOK {
		t.Fatalf("unexpected outbox status: %d", outboxResp.Code)
	}
	var outbox dto.OutboxListResponse
	decodeRecorderJSON(t, outboxResp, &outbox)
	if len(outbox.Items) != 1 || outbox.Items[0].BlockedReason != "diagnosis_disabled" {
		t.Fatalf("unexpected outbox items: %+v", outbox.Items)
	}
}

func TestSessionsListSupportsPaginationSearchAndSort(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)

	payloads := [][]byte{
		[]byte(`{"status":"firing","alerts":[{"labels":{"alertname":"CPUAlpha","instance":"host-alpha","severity":"critical"},"annotations":{"summary":"alpha load"}}]}`),
		[]byte(`{"status":"firing","alerts":[{"labels":{"alertname":"CPUBeta","instance":"host-beta","severity":"warning"},"annotations":{"summary":"beta load"}}]}`),
		[]byte(`{"status":"firing","alerts":[{"labels":{"alertname":"CPUGamma","instance":"host-gamma","severity":"critical"},"annotations":{"summary":"gamma load"}}]}`),
	}

	for _, payload := range payloads {
		resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
			"X-Tars-Signature": "test-signature",
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("unexpected webhook status: %d", resp.Code)
		}
	}

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions?page=1&limit=2&sort_by=session_id&sort_order=asc", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected sessions status: %d", resp.Code)
	}

	var sessions dto.SessionListResponse
	decodeRecorderJSON(t, resp, &sessions)
	if sessions.Total != 3 || sessions.Page != 1 || sessions.Limit != 2 || !sessions.HasNext {
		t.Fatalf("unexpected pagination metadata: %+v", sessions)
	}
	if len(sessions.Items) != 2 {
		t.Fatalf("expected 2 paginated items, got %d", len(sessions.Items))
	}
	if sessions.Items[0].GoldenSummary == nil || sessions.Items[0].GoldenSummary.Headline == "" {
		t.Fatalf("expected session list golden summary, got %+v", sessions.Items[0].GoldenSummary)
	}
	if sessions.Items[0].SessionID > sessions.Items[1].SessionID {
		t.Fatalf("expected ascending session_id order, got %s > %s", sessions.Items[0].SessionID, sessions.Items[1].SessionID)
	}

	searchResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions?q=host-beta", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if searchResp.Code != http.StatusOK {
		t.Fatalf("unexpected search status: %d", searchResp.Code)
	}
	decodeRecorderJSON(t, searchResp, &sessions)
	if sessions.Total != 1 || len(sessions.Items) != 1 {
		t.Fatalf("expected single search hit, got %+v", sessions)
	}
	labels := sessions.Items[0].Alert["labels"].(map[string]interface{})
	if labels["instance"] != "host-beta" {
		t.Fatalf("unexpected search result labels: %+v", labels)
	}
}

func TestSessionsListTriageSortsBeforePagination(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)

	_, err := system.workflow.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "critical-old",
		Labels: map[string]string{
			"alertname": "CriticalOld",
			"instance":  "host-critical",
		},
		Annotations: map[string]string{"summary": "critical but older"},
		ReceivedAt:  time.Date(2026, time.April, 18, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create critical session: %v", err)
	}
	_, err = system.workflow.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "warning",
		Fingerprint: "warning-new",
		Labels: map[string]string{
			"alertname": "WarningNew",
			"instance":  "host-warning",
		},
		Annotations: map[string]string{"summary": "warning but newer"},
		ReceivedAt:  time.Date(2026, time.April, 18, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create warning session: %v", err)
	}

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions?page=1&limit=1&sort_by=triage&sort_order=desc", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected sessions status: %d", resp.Code)
	}
	var sessions dto.SessionListResponse
	decodeRecorderJSON(t, resp, &sessions)
	if sessions.Total != 2 || len(sessions.Items) != 1 || !sessions.HasNext {
		t.Fatalf("unexpected pagination metadata: %+v", sessions)
	}
	labels := sessions.Items[0].Alert["labels"].(map[string]interface{})
	if labels["alertname"] != "CriticalOld" {
		t.Fatalf("expected triage pagination to return older critical session first, got %+v", sessions.Items[0])
	}
}

func TestExecutionsListSupportsPaginationSearchAndSort(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.SSH.AllowedHosts = []string{"host-3"}
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	messagePayloads := [][]byte{
		[]byte(`{"update_id":3001,"message":{"message_id":1,"text":"看系统负载","from":{"id":42,"username":"alice","is_bot":false},"chat":{"id":"445308292","type":"private"}}}`),
		[]byte(`{"update_id":3002,"message":{"message_id":2,"text":"看系统负载","from":{"id":42,"username":"alice","is_bot":false},"chat":{"id":"445308292","type":"private"}}}`),
	}

	for _, payload := range messagePayloads {
		resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram/webhook", payload, nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("unexpected telegram webhook status: %d", resp.Code)
		}
		if err := system.dispatcher.RunOnce(context.Background()); err != nil {
			t.Fatalf("run dispatcher: %v", err)
		}
	}

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/executions?q=uptime&page=1&limit=1&sort_by=execution_id&sort_order=asc", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected executions status: %d", resp.Code)
	}

	var executions dto.ExecutionListResponse
	decodeRecorderJSON(t, resp, &executions)
	if executions.Total != 0 || executions.Page != 1 || executions.Limit != 1 || executions.HasNext {
		t.Fatalf("unexpected execution pagination metadata: %+v", executions)
	}
	if len(executions.Items) != 0 {
		t.Fatalf("expected no execution items for read-only load sessions, got %d", len(executions.Items))
	}
}

func TestExecutionsListTriageSortsBeforePagination(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	alerts := []contracts.AlertEvent{
		{
			Source:      "vmalert",
			Severity:    "critical",
			Fingerprint: "execution-triage-critical-old",
			Labels: map[string]string{
				"alertname": "CriticalExecutionOld",
				"instance":  "host-critical",
				"service":   "api",
				"severity":  "critical",
			},
			Annotations: map[string]string{
				"summary":      "critical but older",
				"user_request": "执行命令查看 api 状态",
			},
			ReceivedAt: time.Date(2026, time.April, 18, 8, 0, 0, 0, time.UTC),
		},
		{
			Source:      "vmalert",
			Severity:    "warning",
			Fingerprint: "execution-triage-warning-new",
			Labels: map[string]string{
				"alertname": "WarningExecutionNew",
				"instance":  "host-warning",
				"service":   "api",
				"severity":  "warning",
			},
			Annotations: map[string]string{
				"summary":      "warning but newer",
				"user_request": "执行命令查看 api 状态",
			},
			ReceivedAt: time.Date(2026, time.April, 18, 12, 0, 0, 0, time.UTC),
		},
	}

	for _, alert := range alerts {
		if _, err := system.workflow.HandleAlertEvent(context.Background(), alert); err != nil {
			t.Fatalf("handle alert %s: %v", alert.Fingerprint, err)
		}
		if err := system.dispatcher.RunOnce(context.Background()); err != nil {
			t.Fatalf("run dispatcher for %s: %v", alert.Fingerprint, err)
		}
	}

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/executions?page=1&limit=1&sort_by=triage&sort_order=desc", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected executions status: %d", resp.Code)
	}

	var executions dto.ExecutionListResponse
	decodeRecorderJSON(t, resp, &executions)
	if executions.Total != 2 || len(executions.Items) != 1 || !executions.HasNext {
		t.Fatalf("unexpected execution triage pagination metadata: %+v", executions)
	}
	if executions.Items[0].RiskLevel != "critical" {
		t.Fatalf("expected triage pagination to return the critical pending execution before newer warning rows, got %+v", executions.Items[0])
	}
}

func TestOutboxListSupportsPaginationSearchAndSort(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, false, false, false)

	var sessionIDs []string
	payloads := [][]byte{
		[]byte(`{"status":"firing","alerts":[{"labels":{"alertname":"OutboxAlpha","instance":"host-alpha","severity":"critical"},"annotations":{"summary":"alpha blocked"}}]}`),
		[]byte(`{"status":"firing","alerts":[{"labels":{"alertname":"OutboxBeta","instance":"host-beta","severity":"critical"},"annotations":{"summary":"beta blocked"}}]}`),
	}

	for _, payload := range payloads {
		resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
			"X-Tars-Signature": "test-signature",
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("unexpected webhook status: %d", resp.Code)
		}
		var accepted dto.VMAlertWebhookResponse
		decodeRecorderJSON(t, resp, &accepted)
		sessionIDs = append(sessionIDs, accepted.SessionIDs[0])
	}

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/outbox?status=blocked&page=1&limit=1&sort_by=aggregate_id&sort_order=asc", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected outbox status: %d", resp.Code)
	}

	var outbox dto.OutboxListResponse
	decodeRecorderJSON(t, resp, &outbox)
	if outbox.Total != 2 || outbox.Page != 1 || outbox.Limit != 1 || !outbox.HasNext {
		t.Fatalf("unexpected outbox pagination metadata: %+v", outbox)
	}
	if len(outbox.Items) != 1 {
		t.Fatalf("expected 1 outbox item, got %d", len(outbox.Items))
	}

	searchResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/outbox?status=blocked&q="+sessionIDs[1], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if searchResp.Code != http.StatusOK {
		t.Fatalf("unexpected outbox search status: %d", searchResp.Code)
	}
	decodeRecorderJSON(t, searchResp, &outbox)
	if outbox.Total != 1 || len(outbox.Items) != 1 {
		t.Fatalf("expected one outbox search hit, got %+v", outbox)
	}
	if outbox.Items[0].AggregateID != sessionIDs[1] {
		t.Fatalf("unexpected outbox aggregate: %+v", outbox.Items[0])
	}
}

func TestExecutionOutputReturnsNotFoundForMissingExecution(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/executions/does-not-exist/output", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected not found, got %d", resp.Code)
	}

	var errBody dto.ErrorEnvelope
	decodeRecorderJSON(t, resp, &errBody)
	if errBody.Error.Code != "not_found" {
		t.Fatalf("unexpected error body: %+v", errBody)
	}
}

func TestExecutionOutputReadWritesAuditEntry(t *testing.T) {
	t.Parallel()

	auditLogger := &captureAuditLogger{}
	system := newTestSystemWithExecutorAndAudit(t, true, true, true, defaultTestConfig(), &fakeExecutor{
		result: actionssh.Result{
			ExitCode: 0,
			Output:   "host-audit-output\n",
		},
	}, auditLogger)

	payload := []byte(`{
		"status": "firing",
		"alerts": [
			{
				"labels": {
					"alertname": "AuditExecutionOutput",
					"instance": "host-1",
					"service": "sshd",
					"severity": "critical"
				},
				"annotations": {
					"summary": "audit execution output",
					"user_request": "执行命令查看 sshd 状态"
				}
			}
		]
	}`)

	webhookResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, map[string]string{
		"X-Tars-Signature": "test-signature",
	})
	if webhookResp.Code != http.StatusOK {
		t.Fatalf("unexpected webhook status: %d", webhookResp.Code)
	}

	var accepted dto.VMAlertWebhookResponse
	decodeRecorderJSON(t, webhookResp, &accepted)
	if err := system.dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions/"+accepted.SessionIDs[0], nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if sessionResp.Code != http.StatusOK {
		t.Fatalf("unexpected session status: %d", sessionResp.Code)
	}

	var sessionDetail dto.SessionDetail
	decodeRecorderJSON(t, sessionResp, &sessionDetail)
	executionID := sessionDetail.Executions[0].ExecutionID

	callbackPayload := []byte(fmt.Sprintf(`{
		"update_id": 3301,
		"callback_query": {
			"id": "cbq-audit-output",
			"data": "approve:%s",
			"from": {
				"id": 42,
				"username": "alice"
			},
			"message": {
				"chat": {
					"id": "-1001001"
				}
			}
		}
	}`, executionID))

	callbackResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram/webhook", callbackPayload, nil)
	if callbackResp.Code != http.StatusOK {
		t.Fatalf("unexpected callback status: %d", callbackResp.Code)
	}

	outputResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/executions/"+executionID+"/output", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if outputResp.Code != http.StatusOK {
		t.Fatalf("unexpected output status: %d", outputResp.Code)
	}

	found := false
	for _, entry := range auditLogger.entries {
		if entry.ResourceType == "execution" && entry.ResourceID == executionID && entry.Action == "get_output" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected execution output audit entry, got %+v", auditLogger.entries)
	}
}

func TestTelegramWebhookRejectsInvalidSecret(t *testing.T) {
	t.Parallel()

	cfg := defaultTestConfig()
	cfg.Telegram.WebhookSecret = "expected-secret"
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	resp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram/webhook", []byte(`{
		"update_id": 2001,
		"callback_query": {
			"id": "cbq-secret",
			"data": "approve:exe-000001",
			"from": {
				"id": 42,
				"username": "alice"
			},
			"message": {
				"chat": {
					"id": "-1001001"
				}
			}
		}
	}`), nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.Code)
	}
}

type testSystem struct {
	handler       http.Handler
	workflow      *workflow.Service
	dispatcher    *events.Dispatcher
	channel       *captureChannel
	metrics       *foundationmetrics.Registry
	skills        *skills.Manager
	access        *access.Manager
	runtimeConfig *postgresrepo.RuntimeConfigStore
}

func newTestSystem(t *testing.T, diagnosisEnabled bool, approvalEnabled bool, executionEnabled bool) testSystem {
	return newTestSystemWithConfig(t, diagnosisEnabled, approvalEnabled, executionEnabled, defaultTestConfig())
}

func newTestSystemWithConfig(t *testing.T, diagnosisEnabled bool, approvalEnabled bool, executionEnabled bool, cfg config.Config) testSystem {
	return newTestSystemWithExecutor(t, diagnosisEnabled, approvalEnabled, executionEnabled, cfg, &fakeExecutor{
		runFunc: func(_ context.Context, _ string, command string) (actionssh.Result, error) {
			if strings.HasPrefix(command, "systemctl is-active") {
				return actionssh.Result{
					ExitCode: 0,
					Output:   "active\n",
				}, nil
			}
			return actionssh.Result{
				ExitCode: 0,
				Output:   "hostname\n 10:00 up 1 day",
			}, nil
		},
	})
}

func newTestSystemWithExecutor(t *testing.T, diagnosisEnabled bool, approvalEnabled bool, executionEnabled bool, cfg config.Config, executor action.Executor) testSystem {
	return newTestSystemWithExecutorAndAudit(t, diagnosisEnabled, approvalEnabled, executionEnabled, cfg, executor, audit.NewNoop())
}

func newTestSystemWithExecutorAndAudit(t *testing.T, diagnosisEnabled bool, approvalEnabled bool, executionEnabled bool, cfg config.Config, executor action.Executor, auditLogger audit.Logger) testSystem {
	return newTestSystemWithExecutorAuditAndKnowledge(t, diagnosisEnabled, approvalEnabled, executionEnabled, cfg, executor, auditLogger, knowledge.NewService())
}

func newTestSystemWithExecutorAuditAndKnowledge(t *testing.T, diagnosisEnabled bool, approvalEnabled bool, executionEnabled bool, cfg config.Config, executor action.Executor, auditLogger audit.Logger, knowledgeSvc contracts.KnowledgeService) testSystem {
	t.Helper()

	channelSvc := &captureChannel{}
	metricsRegistry := foundationmetrics.New()
	triggerManager := trigger.NewManager(nil)
	msgTemplateManager := msgtpl.NewManager(nil)
	authManager, err := authorization.NewManager(cfg.Authorization.ConfigPath)
	if err != nil {
		t.Fatalf("new authorization manager: %v", err)
	}
	approvalManager, err := approvalrouting.NewManager(cfg.Approval.ConfigPath)
	if err != nil {
		t.Fatalf("new approval routing manager: %v", err)
	}
	accessManager, err := access.NewManager(cfg.Access.ConfigPath)
	if err != nil {
		t.Fatalf("new access manager: %v", err)
	}
	promptManager, err := reasoning.NewPromptManager(cfg.Reasoning.PromptsConfigPath)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}
	desenseManager, err := reasoning.NewDesensitizationManager(cfg.Reasoning.DesensitizationConfigPath)
	if err != nil {
		t.Fatalf("new desensitization manager: %v", err)
	}
	providerManager, err := reasoning.NewProviderManager(cfg.Reasoning.ProvidersConfigPath)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	orgManager, err := org.NewManager(cfg.Org.ConfigPath)
	if err != nil {
		t.Fatalf("new org manager: %v", err)
	}
	secretPath := strings.TrimSpace(cfg.Connectors.SecretsPath)
	if secretPath == "" {
		secretPath = strings.TrimSpace(cfg.Reasoning.SecretsConfigPath)
	}
	secretStore, err := secrets.NewStore(secretPath)
	if err != nil {
		t.Fatalf("new secret store: %v", err)
	}
	connectorManager, err := connectors.NewManager(cfg.Connectors.ConfigPath)
	if err != nil {
		t.Fatalf("new connectors manager: %v", err)
	}
	skillManager, err := skills.NewManager(cfg.Skills.ConfigPath, cfg.Skills.MarketplacePath)
	if err != nil {
		t.Fatalf("new skills manager: %v", err)
	}
	agentRoleManager, err := agentrole.NewManager(cfg.AgentRoles.ConfigPath, agentrole.Options{})
	if err != nil {
		t.Fatalf("new agent role manager: %v", err)
	}
	workflowSvc := workflow.NewService(workflow.Options{
		DiagnosisEnabled:        diagnosisEnabled,
		ApprovalEnabled:         approvalEnabled,
		ExecutionEnabled:        executionEnabled,
		ApprovalRouter:          approvalManager,
		AuthorizationPolicy:     authManager,
		DesensitizationProvider: desenseManager,
		Connectors:              connectorManager,
	})
	actionSvc := action.NewService(action.Options{
		Executor:            executor,
		AllowedHosts:        []string{"host-1", "host-2", "host-3", "host-smoke", "127.0.0.1", "localhost"},
		OutputSpoolDir:      t.TempDir(),
		ApprovalRouter:      approvalManager,
		AuthorizationPolicy: authManager,
		Connectors:          connectorManager,
		Secrets:             secretStore,
		QueryRuntimes: map[string]action.QueryRuntime{
			"prometheus_http":      actionprovider.NewMetricsConnectorRuntime(actionprovider.VictoriaMetricsConfig{Metrics: metricsRegistry}),
			"victoriametrics_http": actionprovider.NewMetricsConnectorRuntime(actionprovider.VictoriaMetricsConfig{Metrics: metricsRegistry}),
		},
		ExecutionRuntimes: map[string]action.ExecutionRuntime{
			"jumpserver_api": actionprovider.NewJumpServerRuntime(&http.Client{Transport: fakeJumpServerTransport()}),
		},
		CapabilityRuntimes: map[string]action.CapabilityRuntime{
			"observability":        actionprovider.NewObservabilityHTTPRuntime(&http.Client{Timeout: 2 * time.Second}),
			"delivery":             actionprovider.NewDeliveryRuntime(&http.Client{Timeout: 2 * time.Second}),
			"mcp":                  actionprovider.NewMCPStubRuntime(),
			"skill":                actionprovider.NewSkillStubRuntime(),
			"victoriametrics_http": actionprovider.NewMetricsCapabilityRuntime(actionprovider.VictoriaMetricsConfig{Metrics: metricsRegistry}),
			"prometheus_http":      actionprovider.NewMetricsCapabilityRuntime(actionprovider.VictoriaMetricsConfig{Metrics: metricsRegistry}),
		},
	})
	triggerWorker := events.NewTriggerWorker(
		logger.New("INFO"),
		channelSvc,
		triggerManager,
		msgTemplateManager,
		accessManager,
	)
	automationManager, err := automations.NewManager(cfg.Automations.ConfigPath, automations.Options{
		Logger:    logger.New("INFO"),
		Audit:     auditLogger,
		Action:    actionSvc,
		Knowledge: knowledgeSvc,
		Reasoning: reasoning.NewService(reasoning.Options{
			LocalCommandFallbackEnable: true,
			Audit:                      auditLogger,
			DesensitizationProvider:    desenseManager,
			ProviderRegistry:           providerManager,
			SecretStore:                secretStore,
		}),
		Connectors: connectorManager,
		Skills:     skillManager,
		AgentRoles: agentRoleManager,
		RunNotifier: func(ctx context.Context, job automations.Job, run automations.Run) {
			eventType := trigger.EventOnExecutionCompleted
			subject := "自动化巡检完成"
			statusLabel := "完成"
			if run.Status == "failed" {
				eventType = trigger.EventOnExecutionFailed
				subject = "自动化巡检失败"
				statusLabel = "失败"
			}
			triggerWorker.FireEvent(ctx, trigger.FireEvent{
				EventType: eventType,
				TenantID:  "default",
				RefType:   "automation_run",
				RefID:     run.RunID,
				Subject:   subject,
				Body:      firstNonEmpty(strings.TrimSpace(run.Summary), strings.TrimSpace(run.Error), job.ID),
				Source:    "automation_scheduler",
				TemplateData: map[string]string{
					"ExecutionID":     run.RunID,
					"TargetHost":      firstNonEmpty(job.TargetRef, "automation"),
					"ExitCode":        map[bool]string{true: "1", false: "0"}[run.Status == "failed"],
					"ExecutionStatus": statusLabel,
					"OutputPreview":   firstNonEmpty(strings.TrimSpace(run.Summary), strings.TrimSpace(run.Error), job.ID),
					"TruncationFlag":  "",
					"ActionTip":       "",
					"SessionID":       job.ID,
				},
			})
		},
	})
	if err != nil {
		t.Fatalf("new automations manager: %v", err)
	}

	deps := Dependencies{
		Config:                cfg,
		Logger:                logger.New("INFO"),
		Metrics:               metricsRegistry,
		AlertIngest:           alertintake.NewService(),
		Workflow:              workflowSvc,
		Action:                actionSvc,
		Knowledge:             knowledgeSvc,
		Channel:               channelSvc,
		Trigger:               triggerManager,
		Audit:                 auditLogger,
		Authorization:         authManager,
		Approval:              approvalManager,
		Access:                accessManager,
		Prompts:               promptManager,
		Desense:               desenseManager,
		Providers:             providerManager,
		Connectors:            connectorManager,
		Skills:                skillManager,
		Automations:           automationManager,
		Secrets:               secretStore,
		Org:                   orgManager,
		NotificationTemplates: msgTemplateManager,
		RuntimeConfig:         postgresrepo.NewRuntimeConfigStore(nil),
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)
	return testSystem{
		handler:  mux,
		workflow: workflowSvc,
		dispatcher: events.NewDispatcher(
			deps.Logger,
			deps.Metrics,
			workflowSvc,
			reasoning.NewService(reasoning.Options{
				LocalCommandFallbackEnable: true,
				Audit:                      auditLogger,
				DesensitizationProvider:    desenseManager,
				ProviderRegistry:           providerManager,
				SecretStore:                secretStore,
			}),
			actionSvc,
			knowledgeSvc,
			channelSvc,
			auditLogger,
			connectorManager,
			skillManager,
		),
		channel:       channelSvc,
		metrics:       metricsRegistry,
		skills:        skillManager,
		access:        accessManager,
		runtimeConfig: deps.RuntimeConfig,
	}
}

func TestSetupStatusIncludesObservabilityAndDeliveryRuntime(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/setup/status", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var status dto.SetupStatusResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode setup status: %v", err)
	}
	if status.Connectors.ObservabilityRuntime == nil {
		t.Fatalf("expected observability runtime in setup status")
	}
	if status.Connectors.DeliveryRuntime == nil {
		t.Fatalf("expected delivery runtime in setup status")
	}
}

func TestSetupStatusAllowsAnonymousAccessBeforeInitialization(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/setup/status", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected anonymous setup status before initialization to be allowed, got %d: %s", resp.Code, resp.Body.String())
	}

	var status dto.SetupStatusResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode setup status: %v", err)
	}
	if status.Initialization.Initialized {
		t.Fatalf("expected clean system to remain uninitialized, got %+v", status.Initialization)
	}
	if status.Initialization.Mode != "wizard" {
		t.Fatalf("expected wizard mode before initialization, got %+v", status.Initialization)
	}
}

func TestAccessSessionLoginAndMe(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	_, err := system.access.UpsertAuthProvider(access.AuthProvider{
		ID:           "local_token",
		Type:         "local_token",
		Name:         "Local Token",
		Enabled:      true,
		ClientSecret: "local-secret",
		DefaultRoles: []string{"ops_admin"},
	})
	if err != nil {
		t.Fatalf("upsert auth provider: %v", err)
	}

	loginResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/auth/login", []byte(`{"provider_id":"local_token","token":"local-secret"}`), nil)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("unexpected login status: %d body=%s", loginResp.Code, loginResp.Body.String())
	}

	var login dto.AuthLoginResponse
	decodeRecorderJSON(t, loginResp, &login)
	if strings.TrimSpace(login.SessionToken) == "" {
		t.Fatalf("expected session token, got %+v", login)
	}

	meResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/me", nil, map[string]string{
		"Authorization": "Bearer " + login.SessionToken,
	})
	if meResp.Code != http.StatusOK {
		t.Fatalf("unexpected me status: %d body=%s", meResp.Code, meResp.Body.String())
	}

	var me dto.MeResponse
	decodeRecorderJSON(t, meResp, &me)
	if me.User.UserID == "" || me.AuthSource != "local_token" {
		t.Fatalf("unexpected me payload: %+v", me)
	}
	if len(me.Permissions) == 0 {
		t.Fatalf("expected permissions in me payload: %+v", me)
	}
}

func TestOrgContextReturnsDefaultHierarchy(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/org/context", nil, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected org context status: %d body=%s", resp.Code, resp.Body.String())
	}

	var payload dto.OrgContextResponse
	decodeRecorderJSON(t, resp, &payload)
	if payload.DefaultOrg.ID != org.DefaultOrgID {
		t.Fatalf("unexpected default org: %+v", payload.DefaultOrg)
	}
	if payload.DefaultTenant.ID != org.DefaultTenantID {
		t.Fatalf("unexpected default tenant: %+v", payload.DefaultTenant)
	}
	if payload.DefaultWorkspace.ID != org.DefaultWorkspaceID {
		t.Fatalf("unexpected default workspace: %+v", payload.DefaultWorkspace)
	}
}

func TestOrgCRUDLifecycle(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	headers := map[string]string{"Authorization": "Bearer ops-token"}

	createOrg := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/organizations", []byte(`{"id":"acme","name":"Acme Org","slug":"acme","description":"Enterprise org"}`), headers)
	if createOrg.Code != http.StatusCreated {
		t.Fatalf("unexpected create org status: %d body=%s", createOrg.Code, createOrg.Body.String())
	}
	var createdOrg dto.Organization
	decodeRecorderJSON(t, createOrg, &createdOrg)
	if createdOrg.ID != "acme" {
		t.Fatalf("unexpected org payload: %+v", createdOrg)
	}

	createTenant := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/tenants", []byte(`{"id":"acme-prod","org_id":"acme","name":"Acme Prod","slug":"acme-prod","default_locale":"en-US","default_timezone":"UTC"}`), headers)
	if createTenant.Code != http.StatusCreated {
		t.Fatalf("unexpected create tenant status: %d body=%s", createTenant.Code, createTenant.Body.String())
	}
	var createdTenant dto.Tenant
	decodeRecorderJSON(t, createTenant, &createdTenant)
	if createdTenant.OrgID != "acme" {
		t.Fatalf("unexpected tenant payload: %+v", createdTenant)
	}

	createWorkspace := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/workspaces", []byte(`{"id":"acme-ops","org_id":"acme","tenant_id":"acme-prod","name":"Acme Ops","slug":"acme-ops","description":"Ops workspace"}`), headers)
	if createWorkspace.Code != http.StatusCreated {
		t.Fatalf("unexpected create workspace status: %d body=%s", createWorkspace.Code, createWorkspace.Body.String())
	}
	var createdWorkspace dto.Workspace
	decodeRecorderJSON(t, createWorkspace, &createdWorkspace)
	if createdWorkspace.TenantID != "acme-prod" {
		t.Fatalf("unexpected workspace payload: %+v", createdWorkspace)
	}

	listTenants := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/tenants?org_id=acme", nil, nil)
	if listTenants.Code != http.StatusOK {
		t.Fatalf("unexpected list tenants status: %d body=%s", listTenants.Code, listTenants.Body.String())
	}
	var tenantList dto.TenantListResponse
	decodeRecorderJSON(t, listTenants, &tenantList)
	if len(tenantList.Items) == 0 {
		t.Fatalf("expected tenant items, got %+v", tenantList)
	}

	disableWorkspace := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/workspaces/acme-ops/disable", nil, headers)
	if disableWorkspace.Code != http.StatusOK {
		t.Fatalf("unexpected disable workspace status: %d body=%s", disableWorkspace.Code, disableWorkspace.Body.String())
	}
	decodeRecorderJSON(t, disableWorkspace, &createdWorkspace)
	if createdWorkspace.Status != "disabled" {
		t.Fatalf("expected disabled workspace, got %+v", createdWorkspace)
	}

	updateTenant := performJSONRequest(t, system.handler, http.MethodPut, "/api/v1/tenants/acme-prod", []byte(`{"id":"ignored","org_id":"acme","name":"Acme Production","slug":"acme-production","description":"Primary tenant"}`), headers)
	if updateTenant.Code != http.StatusOK {
		t.Fatalf("unexpected update tenant status: %d body=%s", updateTenant.Code, updateTenant.Body.String())
	}
	decodeRecorderJSON(t, updateTenant, &createdTenant)
	if createdTenant.Name != "Acme Production" {
		t.Fatalf("unexpected updated tenant payload: %+v", createdTenant)
	}
}

func TestAccessPasswordChallengeAndMFAFlow(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	hash, err := bcrypt.GenerateFromPassword([]byte("password-123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	_, err = system.access.UpsertAuthProvider(access.AuthProvider{
		ID:                  "local_password",
		Type:                "local_password",
		Name:                "Local Password",
		Enabled:             true,
		RequireChallenge:    true,
		ChallengeChannel:    "builtin",
		ChallengeTTLSeconds: 300,
		ChallengeCodeLength: 6,
		RequireMFA:          true,
		DefaultRoles:        []string{"viewer"},
	})
	if err != nil {
		t.Fatalf("upsert auth provider: %v", err)
	}
	_, err = system.access.UpsertUser(access.User{
		UserID:               "alice",
		Username:             "alice",
		DisplayName:          "Alice",
		Email:                "alice@example.com",
		Status:               "active",
		Source:               "local_password",
		PasswordHash:         string(hash),
		PasswordLoginEnabled: true,
		ChallengeRequired:    true,
		MFAEnabled:           true,
		MFAMethod:            "totp",
		TOTPSecret:           "JBSWY3DPEHPK3PXP",
		Roles:                []string{"viewer"},
	})
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	loginResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/auth/login", []byte(`{"provider_id":"local_password","username":"alice","password":"password-123"}`), nil)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("unexpected password login status: %d body=%s", loginResp.Code, loginResp.Body.String())
	}
	var login dto.AuthLoginResponse
	decodeRecorderJSON(t, loginResp, &login)
	if login.PendingToken == "" || login.NextStep != "challenge" || login.ChallengeCode == "" {
		t.Fatalf("expected challenge step, got %+v", login)
	}

	verifyResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/auth/verify", []byte(fmt.Sprintf(`{"pending_token":%q,"challenge_id":%q,"code":%q}`, login.PendingToken, login.ChallengeID, login.ChallengeCode)), nil)
	if verifyResp.Code != http.StatusOK {
		t.Fatalf("unexpected challenge verify status: %d body=%s", verifyResp.Code, verifyResp.Body.String())
	}
	var challenge dto.AuthLoginResponse
	decodeRecorderJSON(t, verifyResp, &challenge)
	if challenge.NextStep != "mfa" || challenge.PendingToken == "" {
		t.Fatalf("expected mfa step, got %+v", challenge)
	}
	totpCode, err := totp.GenerateCode("JBSWY3DPEHPK3PXP", time.Now())
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}

	mfaResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/auth/mfa/verify", []byte(fmt.Sprintf(`{"pending_token":%q,"code":%q}`, challenge.PendingToken, totpCode)), nil)
	if mfaResp.Code != http.StatusOK {
		t.Fatalf("unexpected mfa verify status: %d body=%s", mfaResp.Code, mfaResp.Body.String())
	}
	var final dto.AuthLoginResponse
	decodeRecorderJSON(t, mfaResp, &final)
	if strings.TrimSpace(final.SessionToken) == "" {
		t.Fatalf("expected final session token, got %+v", final)
	}

	meResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/me", nil, map[string]string{"Authorization": "Bearer " + final.SessionToken})
	if meResp.Code != http.StatusOK {
		t.Fatalf("unexpected me status: %d body=%s", meResp.Code, meResp.Body.String())
	}
}

func TestGroupsAndRolesEndpoints(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	groupCreateResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/groups", []byte(`{"group":{"group_id":"sre","display_name":"SRE","roles":["viewer"],"members":["ops-admin"]}}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if groupCreateResp.Code != http.StatusCreated {
		t.Fatalf("unexpected group create status: %d body=%s", groupCreateResp.Code, groupCreateResp.Body.String())
	}

	rolesCreateResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/roles", []byte(`{"role":{"id":"custom_operator","display_name":"Custom Operator","permissions":["platform.read","users.read"]}}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if rolesCreateResp.Code != http.StatusCreated {
		t.Fatalf("unexpected role create status: %d body=%s", rolesCreateResp.Code, rolesCreateResp.Body.String())
	}

	bindResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/roles/custom_operator/bindings", []byte(`{"group_ids":["sre"],"operator_reason":"bootstrap access"}`), map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if bindResp.Code != http.StatusOK {
		t.Fatalf("unexpected role binding status: %d body=%s", bindResp.Code, bindResp.Body.String())
	}

	groupResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/groups/sre", nil, map[string]string{
		"Authorization": "Bearer ops-token",
	})
	if groupResp.Code != http.StatusOK {
		t.Fatalf("unexpected group detail status: %d body=%s", groupResp.Code, groupResp.Body.String())
	}

	var group dto.Group
	decodeRecorderJSON(t, groupResp, &group)
	if !containsString(group.Roles, "custom_operator") {
		t.Fatalf("expected bound role on group, got %+v", group)
	}
}

func TestAccessEnableDisableAndConfigEndpoints(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	userCreateResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/users", []byte(`{"user":{"user_id":"alice-user","username":"alice","display_name":"Alice User","email":"alice@example.com","status":"active","source":"local","roles":["viewer"]}}`), map[string]string{"Authorization": "Bearer ops-token"})
	if userCreateResp.Code != http.StatusCreated {
		t.Fatalf("unexpected user create status: %d body=%s", userCreateResp.Code, userCreateResp.Body.String())
	}
	_, err := system.access.UpsertAuthProvider(access.AuthProvider{ID: "google-workspace", Type: "oidc", Name: "Google Workspace", Enabled: true})
	if err != nil {
		t.Fatalf("upsert auth provider: %v", err)
	}
	_, err = system.access.UpsertPerson(access.Person{ID: "alice", DisplayName: "Alice", Status: "active"})
	if err != nil {
		t.Fatalf("upsert person: %v", err)
	}
	_, err = system.access.UpsertChannel(access.Channel{ID: "telegram-main", Name: "Telegram Main", Type: "telegram", Enabled: true})
	if err != nil {
		t.Fatalf("upsert channel: %v", err)
	}

	userDisableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/users/alice-user/disable", []byte(`{"operator_reason":"suspend access"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if userDisableResp.Code != http.StatusOK {
		t.Fatalf("unexpected user disable status: %d body=%s", userDisableResp.Code, userDisableResp.Body.String())
	}
	var disabledUser dto.User
	decodeRecorderJSON(t, userDisableResp, &disabledUser)
	if disabledUser.Status != "disabled" {
		t.Fatalf("expected disabled user status, got %+v", disabledUser)
	}

	userEnableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/users/alice-user/enable", []byte(`{"operator_reason":"restore access"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if userEnableResp.Code != http.StatusOK {
		t.Fatalf("unexpected user enable status: %d body=%s", userEnableResp.Code, userEnableResp.Body.String())
	}
	var enabledUser dto.User
	decodeRecorderJSON(t, userEnableResp, &enabledUser)
	if enabledUser.Status != "active" {
		t.Fatalf("expected active user status, got %+v", enabledUser)
	}

	authDisableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/auth/providers/google-workspace/disable", []byte(`{"operator_reason":"rotate provider"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if authDisableResp.Code != http.StatusOK {
		t.Fatalf("unexpected auth provider disable status: %d body=%s", authDisableResp.Code, authDisableResp.Body.String())
	}
	var authProvider dto.AuthProvider
	decodeRecorderJSON(t, authDisableResp, &authProvider)
	if authProvider.Enabled {
		t.Fatalf("expected auth provider to be disabled, got %+v", authProvider)
	}

	authEnableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/auth/providers/google-workspace/enable", []byte(`{"operator_reason":"finish rotation"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if authEnableResp.Code != http.StatusOK {
		t.Fatalf("unexpected auth provider enable status: %d body=%s", authEnableResp.Code, authEnableResp.Body.String())
	}
	decodeRecorderJSON(t, authEnableResp, &authProvider)
	if !authProvider.Enabled {
		t.Fatalf("expected auth provider to be enabled, got %+v", authProvider)
	}

	personDisableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/people/alice/disable", []byte(`{"operator_reason":"offboard"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if personDisableResp.Code != http.StatusOK {
		t.Fatalf("unexpected person disable status: %d body=%s", personDisableResp.Code, personDisableResp.Body.String())
	}
	var person dto.Person
	decodeRecorderJSON(t, personDisableResp, &person)
	if person.Status != "disabled" {
		t.Fatalf("expected disabled person status, got %+v", person)
	}

	personEnableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/people/alice/enable", []byte(`{"operator_reason":"restore routing"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if personEnableResp.Code != http.StatusOK {
		t.Fatalf("unexpected person enable status: %d body=%s", personEnableResp.Code, personEnableResp.Body.String())
	}
	decodeRecorderJSON(t, personEnableResp, &person)
	if person.Status != "active" {
		t.Fatalf("expected active person status, got %+v", person)
	}

	channelDisableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram-main/disable", []byte(`{"operator_reason":"maintenance"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if channelDisableResp.Code != http.StatusOK {
		t.Fatalf("unexpected channel disable status: %d body=%s", channelDisableResp.Code, channelDisableResp.Body.String())
	}
	var channel dto.Channel
	decodeRecorderJSON(t, channelDisableResp, &channel)
	if channel.Enabled {
		t.Fatalf("expected channel to be disabled, got %+v", channel)
	}

	channelEnableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/channels/telegram-main/enable", []byte(`{"operator_reason":"maintenance complete"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if channelEnableResp.Code != http.StatusOK {
		t.Fatalf("unexpected channel enable status: %d body=%s", channelEnableResp.Code, channelEnableResp.Body.String())
	}
	decodeRecorderJSON(t, channelEnableResp, &channel)
	if !channel.Enabled {
		t.Fatalf("expected channel to be enabled, got %+v", channel)
	}

	providerCreateResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/providers", []byte(`{"provider":{"id":"lab-openai","vendor":"openai","protocol":"openai_compatible","base_url":"https://example.com","enabled":true},"operator_reason":"add provider"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if providerCreateResp.Code != http.StatusCreated {
		t.Fatalf("unexpected provider create status: %d body=%s", providerCreateResp.Code, providerCreateResp.Body.String())
	}

	providerDisableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/providers/lab-openai/disable", []byte(`{"operator_reason":"pause provider"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if providerDisableResp.Code != http.StatusOK {
		t.Fatalf("unexpected provider disable status: %d body=%s", providerDisableResp.Code, providerDisableResp.Body.String())
	}
	var provider dto.ProviderRegistryEntry
	decodeRecorderJSON(t, providerDisableResp, &provider)
	if provider.Enabled {
		t.Fatalf("expected provider to be disabled, got %+v", provider)
	}

	providerEnableResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/providers/lab-openai/enable", []byte(`{"operator_reason":"resume provider"}`), map[string]string{"Authorization": "Bearer ops-token"})
	if providerEnableResp.Code != http.StatusOK {
		t.Fatalf("unexpected provider enable status: %d body=%s", providerEnableResp.Code, providerEnableResp.Body.String())
	}
	decodeRecorderJSON(t, providerEnableResp, &provider)
	if !provider.Enabled {
		t.Fatalf("expected provider to be enabled, got %+v", provider)
	}

	usersResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/users?q=alice", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if usersResp.Code != http.StatusOK {
		t.Fatalf("unexpected users list status: %d body=%s", usersResp.Code, usersResp.Body.String())
	}
	var users dto.UserListResponse
	decodeRecorderJSON(t, usersResp, &users)
	if len(users.Items) == 0 || users.Items[0].UserID != "alice-user" {
		t.Fatalf("unexpected users list payload: %+v", users)
	}

	configResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/config/auth", nil, map[string]string{"Authorization": "Bearer ops-token"})
	if configResp.Code != http.StatusOK {
		t.Fatalf("unexpected auth config status: %d body=%s", configResp.Code, configResp.Body.String())
	}

	var cfg dto.AccessConfigResponse
	decodeRecorderJSON(t, configResp, &cfg)
	if len(cfg.Config.AuthProviders) == 0 || len(cfg.Config.Channels) == 0 || len(cfg.Config.People) == 0 {
		t.Fatalf("unexpected auth config payload: %+v", cfg.Config)
	}
}

func defaultTestConfig() config.Config {
	return config.Config{
		OpsAPI: config.OpsAPIConfig{
			Enabled: true,
			Token:   "ops-token",
		},
		Skills: config.SkillConfig{
			MarketplacePath: "../../../configs/marketplace/skills",
		},
		Automations: config.AutomationConfig{
			ConfigPath: "../../../configs/automations.yaml",
		},
	}
}

type captureChannel struct {
	messages     []contracts.ChannelMessage
	callbackAcks []callbackAck
	sendCalls    int
	failOnCalls  map[int]error
}

func (c *captureChannel) SendMessage(_ context.Context, msg contracts.ChannelMessage) (contracts.SendResult, error) {
	c.sendCalls++
	if err, ok := c.failOnCalls[c.sendCalls]; ok {
		return contracts.SendResult{}, err
	}
	c.messages = append(c.messages, msg)
	return contracts.SendResult{MessageID: "msg-1"}, nil
}

func (c *captureChannel) AnswerCallbackQuery(_ context.Context, callbackID string, text string) error {
	c.callbackAcks = append(c.callbackAcks, callbackAck{ID: callbackID, Text: text})
	return nil
}

type callbackAck struct {
	ID   string
	Text string
}

type captureAuditLogger struct {
	entries []audit.Entry
	records []audit.Record
}

func (c *captureAuditLogger) Log(_ context.Context, entry audit.Entry) {
	c.entries = append(c.entries, entry)
	c.records = append(c.records, audit.Record{
		ID:           fmt.Sprintf("%d", len(c.records)+1),
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		Action:       entry.Action,
		Actor:        entry.Actor,
		Metadata:     entry.Metadata,
		CreatedAt:    time.Now().UTC(),
	})
}

func (c *captureAuditLogger) ListBySession(_ context.Context, sessionID string, limit int) ([]audit.Record, error) {
	items := make([]audit.Record, 0, len(c.records))
	for _, record := range c.records {
		if record.ResourceID == sessionID {
			items = append(items, record)
			continue
		}
		if value, ok := record.Metadata["session_id"].(string); ok && value == sessionID {
			items = append(items, record)
		}
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (c *captureAuditLogger) List(_ context.Context, filter audit.ListFilter) ([]audit.Record, error) {
	items := make([]audit.Record, 0, len(c.records))
	for _, record := range c.records {
		if filter.ResourceType != "" && record.ResourceType != filter.ResourceType {
			continue
		}
		if filter.Action != "" && record.Action != filter.Action {
			continue
		}
		if filter.Actor != "" && record.Actor != filter.Actor {
			continue
		}
		if filter.Query != "" {
			query := strings.ToLower(filter.Query)
			if !strings.Contains(strings.ToLower(record.ResourceType), query) &&
				!strings.Contains(strings.ToLower(record.ResourceID), query) &&
				!strings.Contains(strings.ToLower(record.Action), query) &&
				!strings.Contains(strings.ToLower(record.Actor), query) {
				continue
			}
		}
		items = append(items, record)
	}
	return items, nil
}

func (c *captureAuditLogger) ListByIDs(_ context.Context, ids []string) ([]audit.Record, error) {
	items := make([]audit.Record, 0, len(ids))
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[strings.TrimSpace(id)] = struct{}{}
	}
	for _, record := range c.records {
		if _, ok := set[record.ID]; ok {
			items = append(items, record)
		}
	}
	return items, nil
}

func hasAuditEntry(entries []audit.Entry, resourceType string, resourceID string, action string) bool {
	for _, entry := range entries {
		if entry.ResourceType == resourceType && entry.ResourceID == resourceID && entry.Action == action {
			return true
		}
	}
	return false
}

type fakeExecutor struct {
	result  actionssh.Result
	err     error
	runFunc func(context.Context, string, string) (actionssh.Result, error)
}

func (f *fakeExecutor) Run(ctx context.Context, host string, command string) (actionssh.Result, error) {
	if f.runFunc != nil {
		return f.runFunc(ctx, host, command)
	}
	return f.result, f.err
}

type fakeKnowledgeService struct {
	trace     contracts.SessionKnowledgeTrace
	listItems []contracts.KnowledgeRecordDetail
}

func (f *fakeKnowledgeService) Search(context.Context, contracts.KnowledgeQuery) ([]contracts.KnowledgeHit, error) {
	return nil, nil
}

func (f *fakeKnowledgeService) IngestResolvedSession(context.Context, contracts.SessionClosedEvent) (contracts.KnowledgeIngestResult, error) {
	return contracts.KnowledgeIngestResult{}, nil
}

func (f *fakeKnowledgeService) ReindexDocuments(context.Context, string) error {
	return nil
}

func (f *fakeKnowledgeService) GetSessionKnowledge(context.Context, string) (contracts.SessionKnowledgeTrace, error) {
	return f.trace, nil
}

func (f *fakeKnowledgeService) ListKnowledgeRecords(_ context.Context, filter contracts.ListKnowledgeFilter) ([]contracts.KnowledgeRecordDetail, error) {
	if strings.TrimSpace(filter.Query) == "" {
		return append([]contracts.KnowledgeRecordDetail(nil), f.listItems...), nil
	}
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	items := make([]contracts.KnowledgeRecordDetail, 0, len(f.listItems))
	for _, item := range f.listItems {
		if strings.Contains(strings.ToLower(item.DocumentID), query) ||
			strings.Contains(strings.ToLower(item.SessionID), query) ||
			strings.Contains(strings.ToLower(item.Title), query) ||
			strings.Contains(strings.ToLower(item.Summary), query) {
			items = append(items, item)
		}
	}
	return items, nil
}

func (f *fakeKnowledgeService) ListKnowledgeRecordsByIDs(_ context.Context, ids []string) ([]contracts.KnowledgeRecordDetail, error) {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[strings.TrimSpace(id)] = struct{}{}
	}
	items := make([]contracts.KnowledgeRecordDetail, 0, len(ids))
	for _, item := range f.listItems {
		if _, ok := set[item.DocumentID]; ok {
			items = append(items, item)
		}
	}
	return items, nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func fakeJumpServerTransport() http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var status = http.StatusOK
		body := `{}`
		switch {
		case strings.HasPrefix(req.URL.Path, "/api/v1/assets/hosts/"):
			body = `[{"id":"asset-1","address":"192.168.3.106","name":"host-106"}]`
		case req.URL.Path == "/api/v1/ops/jobs/":
			body = `{"id":"job-1","task_id":"task-1"}`
		case req.URL.Path == "/api/v1/ops/job-execution/task-detail/task-1/":
			body = `{"status":{"value":"success","label":"Success"},"is_finished":true,"is_success":true,"summary":{"result":"success","asset":"192.168.3.106"}}`
		case req.URL.Path == "/api/v1/ops/job-executions/task-1/":
			body = `{"id":"task-1","result":{"stdout":"active\n"},"summary":{"result":"success","module":"shell"}}`
		case req.URL.Path == "/api/v1/ops/ansible/job-execution/task-1/log/":
			body = "active\n"
		default:
			status = http.StatusNotFound
			body = `{"detail":"not found"}`
		}
		resp := &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})
}

func performJSONRequest(t *testing.T, handler http.Handler, method string, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func decodeRecorderJSON(t *testing.T, recorder *httptest.ResponseRecorder, target interface{}) {
	t.Helper()

	if err := json.NewDecoder(bytes.NewReader(recorder.Body.Bytes())).Decode(target); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func hasToolPlanCapability(items []dto.ToolPlanCapabilityDescriptor, tool string, connectorID string, capabilityID string) bool {
	for _, item := range items {
		if item.Tool != tool {
			continue
		}
		if connectorID != "" && item.ConnectorID != connectorID {
			continue
		}
		if capabilityID != "" && item.CapabilityID != capabilityID {
			continue
		}
		return true
	}
	return false
}
