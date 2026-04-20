package events

import (
	"context"
	"testing"

	"tars/internal/foundation/logger"
	"tars/internal/modules/access"
	"tars/internal/modules/msgtpl"
	"tars/internal/modules/trigger"
)

func TestTriggerWorkerResolvesChannelReferenceToRuntimeKind(t *testing.T) {
	t.Parallel()

	channelSvc := &captureChannel{}
	triggerManager := trigger.NewManager(nil)
	accessManager, err := access.NewManager("")
	if err != nil {
		t.Fatalf("new access manager: %v", err)
	}
	if _, err := accessManager.UpsertChannel(access.Channel{
		ID:      "inbox-primary",
		Kind:    "in_app_inbox",
		Name:    "Primary Inbox",
		Target:  "ops-room",
		Enabled: true,
	}); err != nil {
		t.Fatalf("upsert access channel: %v", err)
	}
	if _, err := triggerManager.Upsert(context.Background(), trigger.Trigger{
		ID:          "trg_channel_ref_runtime",
		TenantID:    "default",
		DisplayName: "Channel ref runtime",
		Enabled:     true,
		EventType:   "on_custom_runtime_resolution",
		ChannelID:   "inbox-primary",
	}); err != nil {
		t.Fatalf("upsert trigger: %v", err)
	}

	worker := NewTriggerWorker(
		logger.New("ERROR"),
		channelSvc,
		triggerManager,
		msgtpl.NewManager(nil),
		accessManager,
	)

	worker.FireEvent(context.Background(), trigger.FireEvent{
		TenantID:  "default",
		EventType: "on_custom_runtime_resolution",
		Subject:   "Skill completed",
		Body:      "done",
	})

	if len(channelSvc.messages) != 1 {
		t.Fatalf("expected exactly one channel message, got %d", len(channelSvc.messages))
	}
	if channelSvc.messages[0].Channel != "in_app_inbox" {
		t.Fatalf("expected resolved runtime channel kind, got %+v", channelSvc.messages[0])
	}
	if channelSvc.messages[0].Target != "ops-room" {
		t.Fatalf("expected resolved channel target, got %+v", channelSvc.messages[0])
	}
}

func TestTriggerWorkerFiltersByAudienceAndExpr(t *testing.T) {
	t.Parallel()

	channelSvc := &captureChannel{}
	triggerManager := trigger.NewManager(nil)
	const governedEventType = "on_governed_runtime_match"
	if _, err := triggerManager.Upsert(context.Background(), trigger.Trigger{
		ID:             "trg_gated",
		TenantID:       "default",
		DisplayName:    "Governed trigger",
		Enabled:        true,
		EventType:      governedEventType,
		ChannelID:      "telegram",
		TargetAudience: "ops.leads",
		FilterExpr:     "risk_level == 'critical'",
	}); err != nil {
		t.Fatalf("upsert trigger: %v", err)
	}

	worker := NewTriggerWorker(
		logger.New("ERROR"),
		channelSvc,
		triggerManager,
		msgtpl.NewManager(nil),
		nil,
	)

	worker.FireEvent(context.Background(), trigger.FireEvent{
		TenantID:  "default",
		EventType: governedEventType,
		Subject:   "Approval requested",
		Body:      "body",
		TemplateData: map[string]string{
			"risk_level":      "warning",
			"target_audience": "ops.leads",
		},
	})
	worker.FireEvent(context.Background(), trigger.FireEvent{
		TenantID:  "default",
		EventType: governedEventType,
		Subject:   "Approval requested",
		Body:      "body",
		TemplateData: map[string]string{
			"risk_level":      "critical",
			"target_audience": "ops.engineering",
		},
	})

	if len(channelSvc.messages) != 0 {
		t.Fatalf("expected gated trigger to skip non-matching events, got %+v", channelSvc.messages)
	}

	worker.FireEvent(context.Background(), trigger.FireEvent{
		TenantID:  "default",
		EventType: governedEventType,
		Subject:   "Approval requested",
		Body:      "body",
		TemplateData: map[string]string{
			"risk_level":      "critical",
			"target_audience": "ops.leads",
		},
	})

	if len(channelSvc.messages) != 1 {
		t.Fatalf("expected exactly one matching governed trigger delivery, got %+v", channelSvc.messages)
	}
	if channelSvc.messages[0].Channel != "telegram" {
		t.Fatalf("expected governed trigger to deliver on telegram, got %+v", channelSvc.messages[0])
	}
}

func TestTriggerWorkerSkipsUnsupportedRegistryChannelKind(t *testing.T) {
	t.Parallel()

	channelSvc := &captureChannel{}
	triggerManager := trigger.NewManager(nil)
	accessManager, err := access.NewManager("")
	if err != nil {
		t.Fatalf("new access manager: %v", err)
	}
	if _, err := accessManager.UpsertChannel(access.Channel{
		ID:      "slack-primary",
		Kind:    "slack",
		Name:    "Primary Slack",
		Target:  "#ops-room",
		Enabled: true,
	}); err != nil {
		t.Fatalf("upsert access channel: %v", err)
	}
	if _, err := triggerManager.Upsert(context.Background(), trigger.Trigger{
		ID:          "trg_unsupported_kind",
		TenantID:    "default",
		DisplayName: "Unsupported kind",
		Enabled:     true,
		EventType:   "on_custom_unsupported_kind",
		ChannelID:   "slack-primary",
	}); err != nil {
		t.Fatalf("upsert trigger: %v", err)
	}

	worker := NewTriggerWorker(
		logger.New("ERROR"),
		channelSvc,
		triggerManager,
		msgtpl.NewManager(nil),
		accessManager,
	)

	worker.FireEvent(context.Background(), trigger.FireEvent{
		TenantID:  "default",
		EventType: "on_custom_unsupported_kind",
		Subject:   "Unsupported transport",
		Body:      "should not fall back to telegram",
	})

	if len(channelSvc.messages) != 0 {
		t.Fatalf("expected unsupported registry kind to be skipped, got %+v", channelSvc.messages)
	}
}
