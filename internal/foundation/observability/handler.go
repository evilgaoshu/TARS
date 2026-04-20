package observability

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"tars/internal/foundation/metrics"
)

type Handler struct {
	level   slog.Leveler
	writer  slog.Handler
	store   *Store
	metrics *metrics.Registry
	attrs   []slog.Attr
}

func NewHandler(level slog.Leveler, writer slog.Handler, store *Store, registry *metrics.Registry, attrs ...slog.Attr) slog.Handler {
	return &Handler{
		level:   level,
		writer:  writer,
		store:   store,
		metrics: registry,
		attrs:   append([]slog.Attr(nil), attrs...),
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.writer == nil {
		return false
	}
	return h.writer.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	if h.writer == nil {
		return nil
	}
	if h.store != nil {
		item := SignalRecord{
			Timestamp: record.Time.UTC(),
			Level:     strings.ToLower(record.Level.String()),
			Message:   record.Message,
			Metadata:  map[string]any{},
		}
		for _, attr := range h.attrs {
			assignSignalField(&item, attr.Key, attr.Value.Any())
		}
		record.Attrs(func(attr slog.Attr) bool {
			assignSignalField(&item, attr.Key, attr.Value.Any())
			return true
		})
		if item.Component == "" {
			item.Component = "runtime"
		}
		_ = h.store.AppendLog(item)
		if h.metrics != nil && strings.EqualFold(item.Level, "error") {
			h.metrics.IncCounter("tars_observability_logs_total", "Total observability logs captured by level and component.", metrics.Labels{
				"level":     normalizeLabel(item.Level, "info"),
				"component": normalizeLabel(item.Component, "runtime"),
			})
		} else if h.metrics != nil {
			h.metrics.IncCounter("tars_observability_logs_total", "Total observability logs captured by level and component.", metrics.Labels{
				"level":     normalizeLabel(item.Level, "info"),
				"component": normalizeLabel(item.Component, "runtime"),
			})
		}
	}
	return h.writer.Handle(ctx, record)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nextAttrs := append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &Handler{
		level:   h.level,
		writer:  h.writer.WithAttrs(attrs),
		store:   h.store,
		metrics: h.metrics,
		attrs:   nextAttrs,
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		level:   h.level,
		writer:  h.writer.WithGroup(name),
		store:   h.store,
		metrics: h.metrics,
		attrs:   append([]slog.Attr(nil), h.attrs...),
	}
}

func assignSignalField(record *SignalRecord, key string, value any) {
	key = strings.TrimSpace(key)
	if key == "" || record == nil {
		return
	}
	switch key {
	case "component":
		record.Component = stringValue(value)
	case "session_id":
		record.SessionID = stringValue(value)
	case "execution_id":
		record.ExecutionID = stringValue(value)
	case "trace_id":
		record.TraceID = stringValue(value)
	case "actor":
		record.Actor = stringValue(value)
	case "route":
		record.Route = stringValue(value)
	default:
		if record.Metadata == nil {
			record.Metadata = map[string]any{}
		}
		record.Metadata[key] = normalizeValue(value)
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	default:
		bytes, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return strings.Trim(string(bytes), `"`)
	}
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case slog.Value:
		return normalizeValue(typed.Any())
	default:
		return typed
	}
}

func normalizeLabel(value string, fallback string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return fallback
	}
	return value
}
