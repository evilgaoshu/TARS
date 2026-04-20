package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"tars/internal/modules/access"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/msgtpl"
	"tars/internal/modules/trigger"
)

func TestCompatibilityChannelDTOIncludesKindAndUsages(t *testing.T) {
	t.Parallel()

	payload := marshalToMap(t, channelToDTO(access.Channel{
		ID:           "ch-1",
		Type:         "telegram",
		Name:         "Ops Room",
		Target:       "-10012345",
		Enabled:      true,
		Capabilities: []access.ChannelCapability{"approval", "notifications"},
	}))

	if payload["type"] != "telegram" {
		t.Fatalf("expected legacy type field, got %#v", payload["type"])
	}
	if payload["kind"] != "telegram" {
		t.Fatalf("expected kind=telegram, got %#v", payload["kind"])
	}
	usages, ok := payload["usages"].([]any)
	if !ok || len(usages) == 0 {
		t.Fatalf("expected non-empty usages, got %#v", payload["usages"])
	}
	if usages[0] != "approval" {
		t.Fatalf("expected usages to preserve capabilities order, got %#v", usages)
	}
}

func TestCompatibilityTriggerDTOIncludesChannelID(t *testing.T) {
	t.Parallel()

	payload := marshalToMap(t, triggerToDTO(trigger.Trigger{
		ID:          "trg-1",
		TenantID:    "default",
		DisplayName: "Approval Requested",
		Enabled:     true,
		EventType:   trigger.EventOnApprovalRequested,
		ChannelID:   "legacy-channel",
		Governance:  "advanced_review",
		CreatedAt:   time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC),
	}))

	if payload["channel_id"] != "legacy-channel" {
		t.Fatalf("expected channel_id to mirror legacy channel, got %#v", payload["channel_id"])
	}
	if payload["governance"] != "advanced_review" {
		t.Fatalf("expected governance to be preserved, got %#v", payload["governance"])
	}
}

func TestCompatibilityTriggerDTOIncludesAutomationJobID(t *testing.T) {
	t.Parallel()

	payload := marshalToMap(t, triggerToDTO(trigger.Trigger{
		ID:              "trg-1",
		TenantID:        "default",
		DisplayName:     "Approval Requested",
		Enabled:         true,
		EventType:       trigger.EventOnApprovalRequested,
		ChannelID:       "inbox-primary",
		AutomationJobID: "daily-health",
		CreatedAt:       time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC),
	}))

	if payload["automation_job_id"] != "daily-health" {
		t.Fatalf("expected automation_job_id to be preserved, got %#v", payload["automation_job_id"])
	}
}

func TestTriggerManagerUpsertPreservesGovernanceCompatibilityFields(t *testing.T) {
	t.Parallel()

	manager := trigger.NewManager(nil)
	saved, err := manager.Upsert(t.Context(), trigger.Trigger{
		ID:             "trg-governance-memory",
		TenantID:       "default",
		DisplayName:    "Governed Rule",
		Enabled:        true,
		EventType:      trigger.EventOnExecutionCompleted,
		ChannelID:      "inbox-primary",
		Governance:     "org_guardrail",
		FilterExpr:     "severity == 'critical'",
		TargetAudience: "ops.leads",
		TemplateID:     "execution_result-zh-CN",
		CooldownSec:    30,
	})
	if err != nil {
		t.Fatalf("upsert trigger: %v", err)
	}
	if saved.Governance != "org_guardrail" || saved.FilterExpr == "" || saved.TargetAudience == "" {
		t.Fatalf("expected governance fields to persist in manager, got %+v", saved)
	}
	if saved.ChannelID != "inbox-primary" {
		t.Fatalf("expected channel_id compatibility field to persist, got %+v", saved)
	}
}

func TestCompatibilityMsgTemplateDTOIncludesStatus(t *testing.T) {
	t.Parallel()

	payload := marshalToMap(t, notificationTemplateToDTO(msgtpl.MsgTemplate{
		ID:      "approval-zh-CN",
		Kind:    "approval",
		Locale:  "zh-CN",
		Name:    "Approval",
		Enabled: true,
		Content: msgtpl.TemplateContent{
			Subject: "subject",
			Body:    "body",
		},
	}))

	if payload["enabled"] != true {
		t.Fatalf("expected legacy enabled=true, got %#v", payload["enabled"])
	}
	if payload["status"] != "active" {
		t.Fatalf("expected status=active, got %#v", payload["status"])
	}
}

func TestAgentRoleDTORendersStructuredModelBindingOnly(t *testing.T) {
	t.Parallel()

	payload := marshalToMap(t, agentRoleDTO(agentrole.AgentRole{
		RoleID:      "diagnosis",
		DisplayName: "Diagnosis",
		Status:      "active",
		Profile: agentrole.Profile{
			SystemPrompt: "diagnose",
		},
		CapabilityBinding: agentrole.CapabilityBinding{Mode: "unrestricted"},
		PolicyBinding: agentrole.PolicyBinding{
			MaxRiskLevel: "info",
			MaxAction:    "suggest_only",
		},
		ModelBinding: agentrole.ModelBinding{
			Primary: &agentrole.ModelTargetBinding{
				ProviderID: "openai-main",
				Model:      "gpt-4.1-mini",
			},
			Fallback: &agentrole.ModelTargetBinding{
				ProviderID: "openai-backup",
				Model:      "gpt-4o-mini",
			},
		},
	}))

	if _, ok := payload["provider_preference"]; ok {
		t.Fatalf("expected provider_preference to be removed, got %#v", payload["provider_preference"])
	}
	binding, ok := payload["model_binding"].(map[string]any)
	if !ok {
		t.Fatalf("expected model_binding object, got %#v", payload["model_binding"])
	}
	primary, ok := binding["primary"].(map[string]any)
	if !ok {
		t.Fatalf("expected primary binding, got %#v", binding["primary"])
	}
	if primary["provider_id"] != "openai-main" || primary["model"] != "gpt-4.1-mini" {
		t.Fatalf("unexpected primary binding: %#v", primary)
	}
	fallback, ok := binding["fallback"].(map[string]any)
	if !ok {
		t.Fatalf("expected fallback binding, got %#v", binding["fallback"])
	}
	if fallback["provider_id"] != "openai-backup" || fallback["model"] != "gpt-4o-mini" {
		t.Fatalf("unexpected fallback binding: %#v", fallback)
	}
}

func marshalToMap(t *testing.T, v any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return payload
}
