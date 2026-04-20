package audit

import (
	"context"
	"log/slog"
)

type slogLogger struct {
	logger *slog.Logger
}

func NewSlog(logger *slog.Logger) Logger {
	if logger == nil {
		return NewNoop()
	}
	return slogLogger{logger: logger.With("component", "audit")}
}

func (l slogLogger) Log(_ context.Context, entry Entry) {
	attrs := []any{
		"resource_type", entry.ResourceType,
		"resource_id", entry.ResourceID,
		"action", entry.Action,
	}
	if entry.Actor != "" {
		attrs = append(attrs, "actor", entry.Actor)
	}
	if entry.TraceID != "" {
		attrs = append(attrs, "trace_id", entry.TraceID)
	}
	if len(entry.Metadata) > 0 {
		attrs = append(attrs, "metadata", entry.Metadata)
	}
	l.logger.Info("audit event", attrs...)
}

func (l slogLogger) ListBySession(_ context.Context, _ string, _ int) ([]Record, error) {
	return nil, nil
}

func (l slogLogger) List(_ context.Context, _ ListFilter) ([]Record, error) {
	return nil, nil
}

func (l slogLogger) ListByIDs(_ context.Context, _ []string) ([]Record, error) {
	return nil, nil
}
