package observability

import (
	"context"
	"strings"
	"time"

	"tars/internal/foundation/audit"
)

type AuditLogger struct {
	store *Store
}

func NewAuditLogger(store *Store) audit.Logger {
	if store == nil {
		return audit.NewNoop()
	}
	return AuditLogger{store: store}
}

func (l AuditLogger) Log(_ context.Context, entry audit.Entry) {
	if l.store == nil {
		return
	}
	_ = l.store.AppendEvent(SignalRecord{
		Timestamp:   time.Now().UTC(),
		Component:   firstSignalValue(entry.ResourceType, "audit"),
		Message:     firstSignalValue(entry.Action, "audit event"),
		Actor:       entry.Actor,
		SessionID:   signalString(entry.Metadata, "session_id"),
		ExecutionID: signalString(entry.Metadata, "execution_id"),
		TraceID:     firstSignalValue(strings.TrimSpace(entry.TraceID), signalString(entry.Metadata, "trace_id"), signalString(entry.Metadata, "session_id"), signalString(entry.Metadata, "execution_id")),
		Metadata: map[string]any{
			"resource_type": entry.ResourceType,
			"resource_id":   entry.ResourceID,
			"action":        entry.Action,
			"metadata":      entry.Metadata,
		},
	})
}

func firstSignalValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func signalString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
