package workflow

import (
	"context"
	"strings"
	"testing"
	"time"

	"tars/internal/contracts"
)

func TestPublishEventAndCompleteOutbox(t *testing.T) {
	t.Parallel()

	service := NewService(Options{DiagnosisEnabled: true})
	sessionID := mustCreateWorkflowSession(t, service, "publish-complete", "host-publish", "api")
	createdAt := time.Date(2026, time.April, 2, 9, 0, 0, 0, time.UTC)
	payload := []byte(`{"body":"hello"}`)

	envelope, err := service.PublishEvent(context.Background(), contracts.EventPublishRequest{
		Topic:       " telegram.send ",
		AggregateID: " " + sessionID + " ",
		Payload:     payload,
		Headers:     map[string]string{"x-request-id": "req-1"},
		Metadata:    contracts.EventMetadata{CorrelationID: "corr-1"},
		CreatedAt:   createdAt,
	})
	if err != nil {
		t.Fatalf("publish event: %v", err)
	}

	payload[0] = 'X'

	if envelope.Topic != "telegram.send" {
		t.Fatalf("expected trimmed topic, got %+v", envelope)
	}
	if envelope.AggregateID != sessionID {
		t.Fatalf("expected trimmed aggregate id, got %+v", envelope)
	}
	if envelope.Status != "pending" || envelope.Attempt != 1 {
		t.Fatalf("unexpected initial outbox envelope: %+v", envelope)
	}
	if envelope.AvailableAt != createdAt || envelope.CreatedAt != createdAt {
		t.Fatalf("expected created/available timestamps to match, got %+v", envelope)
	}
	if got := string(envelope.Payload); got != `{"body":"hello"}` {
		t.Fatalf("expected payload copy, got %q", got)
	}

	if err := service.CompleteOutbox(context.Background(), envelope.EventID); err != nil {
		t.Fatalf("complete outbox: %v", err)
	}

	item := service.outbox[envelope.EventID]
	if item.Status != "done" || item.LastError != "" || item.BlockedReason != "" {
		t.Fatalf("expected completed outbox item, got %+v", item)
	}
	if !item.AvailableAt.IsZero() {
		t.Fatalf("expected available_at to be cleared, got %+v", item)
	}
}

func TestResolveEventLifecycle(t *testing.T) {
	t.Parallel()

	service := NewService(Options{DiagnosisEnabled: true})
	sessionID := mustCreateWorkflowSession(t, service, "resolve-event", "host-resolve", "api")

	envelope, err := service.PublishEvent(context.Background(), contracts.EventPublishRequest{
		Topic:       "telegram.send",
		AggregateID: sessionID,
		Payload:     []byte(`{"body":"retry me"}`),
	})
	if err != nil {
		t.Fatalf("publish event: %v", err)
	}

	retryStartedAt := time.Now().UTC()
	if err := service.ResolveEvent(context.Background(), envelope.EventID, contracts.DeliveryResult{
		Decision:  contracts.DeliveryDecisionRetry,
		LastError: " temporary network failure ",
		Delay:     2 * time.Second,
		Reason:    "transient",
	}); err != nil {
		t.Fatalf("resolve retry: %v", err)
	}
	retryFinishedAt := time.Now().UTC()

	retried := service.outbox[envelope.EventID]
	if retried.Status != "pending" || retried.RetryCount != 1 || retried.LastError != "temporary network failure" {
		t.Fatalf("expected retry state, got %+v", retried)
	}
	earliestAvailableAt := retryStartedAt.Add(2 * time.Second)
	latestAvailableAt := retryFinishedAt.Add(2 * time.Second)
	if retried.AvailableAt.Before(earliestAvailableAt) || retried.AvailableAt.After(latestAvailableAt) {
		t.Fatalf("expected retry delay to be applied within [%s, %s], got %+v", earliestAvailableAt, latestAvailableAt, retried)
	}

	session, err := service.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("get session after retry: %v", err)
	}
	if !timelineContains(session.Timeline, "outbox_retry_scheduled", "temporary network failure") ||
		!timelineContains(session.Timeline, "outbox_retry_scheduled", "transient") {
		t.Fatalf("expected retry timeline entry, got %+v", session.Timeline)
	}

	if err := service.ResolveEvent(context.Background(), envelope.EventID, contracts.DeliveryResult{
		Decision:  contracts.DeliveryDecisionDeadLetter,
		LastError: "permanent failure",
		Reason:    "max attempts reached",
	}); err != nil {
		t.Fatalf("resolve dead-letter: %v", err)
	}

	failed := service.outbox[envelope.EventID]
	if failed.Status != "failed" || failed.RetryCount != 2 || failed.LastError != "permanent failure" {
		t.Fatalf("expected failed state, got %+v", failed)
	}
	if !failed.AvailableAt.IsZero() {
		t.Fatalf("expected failed event to clear available_at, got %+v", failed)
	}

	session, err = service.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("get session after dead-letter: %v", err)
	}
	if !timelineContains(session.Timeline, "outbox_failed", "permanent failure") ||
		!timelineContains(session.Timeline, "outbox_failed", "max attempts reached") {
		t.Fatalf("expected outbox_failed timeline entry, got %+v", session.Timeline)
	}

	if err := service.ResolveEvent(context.Background(), envelope.EventID, contracts.DeliveryResult{
		Decision: contracts.DeliveryDecision("unsupported"),
	}); err == nil || !strings.Contains(err.Error(), "unsupported delivery decision") {
		t.Fatalf("expected unsupported delivery decision error, got %v", err)
	}
}

func TestMarkOutboxFailedAndReplayOutbox(t *testing.T) {
	t.Parallel()

	t.Run("delivery policy retries then dead-letters", func(t *testing.T) {
		t.Parallel()

		service := NewService(Options{DiagnosisEnabled: true})
		sessionID := mustCreateWorkflowSession(t, service, "mark-failed", "host-mark", "api")
		envelope, err := service.PublishEvent(context.Background(), contracts.EventPublishRequest{
			Topic:       "telegram.send",
			AggregateID: sessionID,
			Payload:     []byte(`{"body":"notify"}`),
		})
		if err != nil {
			t.Fatalf("publish event: %v", err)
		}

		for attempt := 1; attempt <= 3; attempt++ {
			if err := service.MarkOutboxFailed(context.Background(), envelope.EventID, " delivery failed "); err != nil {
				t.Fatalf("mark outbox failed attempt %d: %v", attempt, err)
			}
		}

		item := service.outbox[envelope.EventID]
		if item.Status != "failed" || item.RetryCount != 3 || item.LastError != "delivery failed" {
			t.Fatalf("expected failed outbox item after policy exhaustion, got %+v", item)
		}

		if err := service.ReplayOutbox(context.Background(), envelope.EventID, "manual replay"); err != nil {
			t.Fatalf("replay outbox: %v", err)
		}

		item = service.outbox[envelope.EventID]
		if item.Status != "pending" || item.RetryCount != 4 || item.LastError != "" || item.BlockedReason != "" {
			t.Fatalf("expected replayed outbox item to be reset, got %+v", item)
		}

		session, err := service.GetSession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("get session: %v", err)
		}
		if !timelineContains(session.Timeline, "outbox_replayed", "manual replay") {
			t.Fatalf("expected outbox_replayed timeline entry, got %+v", session.Timeline)
		}
	})

	t.Run("blocked feature-flag event cannot be replayed", func(t *testing.T) {
		t.Parallel()

		service := NewService(Options{DiagnosisEnabled: false})
		_, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
			Source:      "vmalert",
			Severity:    "warning",
			Fingerprint: "blocked-replay",
			Labels: map[string]string{
				"alertname": "DiskFull",
				"instance":  "host-blocked",
				"service":   "api",
				"severity":  "warning",
			},
			Annotations: map[string]string{"summary": "disk pressure"},
		})
		if err != nil {
			t.Fatalf("handle alert: %v", err)
		}

		items, err := service.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "blocked"})
		if err != nil {
			t.Fatalf("list blocked outbox: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected blocked outbox event, got %+v", items)
		}

		err = service.ReplayOutbox(context.Background(), items[0].ID, "force requeue")
		if err != contracts.ErrBlockedByFeatureFlag {
			t.Fatalf("expected ErrBlockedByFeatureFlag, got %v", err)
		}
	})
}

func TestListOutboxFiltersSortsAndDelete(t *testing.T) {
	t.Parallel()

	service := NewService(Options{DiagnosisEnabled: false})
	blockedSessionID := mustCreateWorkflowSession(t, service, "list-blocked", "host-blocked", "api")
	failedSessionID := "ses-manual-failed"
	service.sessions[failedSessionID] = &sessionRecord{
		detail: contracts.SessionDetail{
			SessionID: failedSessionID,
			Status:    "open",
			Alert: map[string]interface{}{
				"host":     "host-failed",
				"severity": "warning",
				"labels": map[string]interface{}{
					"alertname": "WorkflowCoverage",
					"instance":  "host-failed",
					"service":   "worker",
				},
				"annotations": map[string]interface{}{
					"summary": "coverage session for list-failed",
				},
			},
			Timeline: []contracts.TimelineEvent{{
				Event:     "alert_received",
				Message:   "manually seeded failed session",
				CreatedAt: time.Date(2026, time.April, 2, 9, 15, 0, 0, time.UTC),
			}},
		},
		host: "host-failed",
	}
	service.sessionOrder = append(service.sessionOrder, failedSessionID)

	failedEvent, err := service.PublishEvent(context.Background(), contracts.EventPublishRequest{
		Topic:       "telegram.send",
		AggregateID: failedSessionID,
		Payload:     []byte(`{"body":"page operator"}`),
		CreatedAt:   time.Date(2026, time.April, 2, 9, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("publish failed event: %v", err)
	}
	if err := service.ResolveEvent(context.Background(), failedEvent.EventID, contracts.DeliveryResult{
		Decision:  contracts.DeliveryDecisionDeadLetter,
		LastError: "smtp refused",
	}); err != nil {
		t.Fatalf("dead-letter failed event: %v", err)
	}

	if _, err := service.PublishEvent(context.Background(), contracts.EventPublishRequest{
		Topic:       "telegram.send",
		AggregateID: failedSessionID,
		Payload:     []byte(`{"body":"still pending"}`),
	}); err != nil {
		t.Fatalf("publish pending event: %v", err)
	}

	all, err := service.ListOutbox(context.Background(), contracts.ListOutboxFilter{
		SortBy:    "topic",
		SortOrder: "asc",
	})
	if err != nil {
		t.Fatalf("list all outbox: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected blocked and failed events only, got %+v", all)
	}
	if all[0].Topic != "session.analyze_requested" || all[1].Topic != "telegram.send" {
		t.Fatalf("expected topic sort order, got %+v", all)
	}

	blocked, err := service.ListOutbox(context.Background(), contracts.ListOutboxFilter{
		Status: "blocked",
		Query:  "diagnosis_disabled",
	})
	if err != nil {
		t.Fatalf("list blocked outbox: %v", err)
	}
	if len(blocked) != 1 || blocked[0].AggregateID != blockedSessionID {
		t.Fatalf("expected blocked event for diagnosis_disabled, got %+v", blocked)
	}

	failed, err := service.ListOutbox(context.Background(), contracts.ListOutboxFilter{
		Status: "failed",
		Query:  "smtp refused",
	})
	if err != nil {
		t.Fatalf("list failed outbox: %v", err)
	}
	if len(failed) != 1 || failed[0].ID != failedEvent.EventID {
		t.Fatalf("expected failed telegram event, got %+v", failed)
	}

	if err := service.DeleteOutbox(context.Background(), blocked[0].ID, "cleanup blocked residue"); err != nil {
		t.Fatalf("delete blocked outbox: %v", err)
	}
	if _, ok := service.outbox[blocked[0].ID]; ok {
		t.Fatalf("expected blocked outbox event to be deleted")
	}

	session, err := service.GetSession(context.Background(), blockedSessionID)
	if err != nil {
		t.Fatalf("get blocked session: %v", err)
	}
	if !timelineContains(session.Timeline, "outbox_deleted", "cleanup blocked residue") {
		t.Fatalf("expected outbox_deleted timeline entry, got %+v", session.Timeline)
	}
}

func TestSortOutboxEventsHelper(t *testing.T) {
	t.Parallel()

	items := []contracts.OutboxEvent{
		{ID: "evt-2", Topic: "telegram.send", Status: "failed", CreatedAt: time.Date(2026, time.April, 2, 10, 0, 0, 0, time.UTC)},
		{ID: "evt-1", Topic: "session.analyze_requested", Status: "blocked", CreatedAt: time.Date(2026, time.April, 2, 9, 0, 0, 0, time.UTC)},
		{ID: "evt-3", Topic: "session.closed", Status: "done", CreatedAt: time.Date(2026, time.April, 2, 11, 0, 0, 0, time.UTC)},
	}

	sortOutboxEvents(items, "status", "asc")
	if items[0].Status != "blocked" || items[2].Status != "failed" {
		t.Fatalf("expected status ascending sort, got %+v", items)
	}
	sortOutboxEvents(items, "status", "desc")
	if items[0].Status != "failed" {
		t.Fatalf("expected status descending sort, got %+v", items)
	}

	sortOutboxEvents(items, "topic", "asc")
	if items[0].Topic != "session.analyze_requested" || items[2].Topic != "telegram.send" {
		t.Fatalf("expected topic ascending sort, got %+v", items)
	}

	sortOutboxEvents(items, "created_at", "desc")
	if items[0].ID != "evt-3" || items[2].ID != "evt-1" {
		t.Fatalf("expected created_at descending sort, got %+v", items)
	}
}

func mustCreateWorkflowSession(t *testing.T, service *Service, fingerprint string, host string, serviceName string) string {
	t.Helper()

	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "warning",
		Fingerprint: fingerprint,
		Labels: map[string]string{
			"alertname": "WorkflowCoverage",
			"instance":  host,
			"host":      host,
			"service":   serviceName,
			"severity":  "warning",
		},
		Annotations: map[string]string{
			"summary": "coverage session for " + fingerprint,
		},
		ReceivedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}
	return result.SessionID
}
