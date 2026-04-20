package audit

import (
	"context"
	"maps"
	"sort"
	"time"
)

type compositeLogger struct {
	loggers []Logger
}

func NewComposite(loggers ...Logger) Logger {
	filtered := make([]Logger, 0, len(loggers))
	for _, logger := range loggers {
		if logger == nil {
			continue
		}
		filtered = append(filtered, logger)
	}
	if len(filtered) == 0 {
		return NewNoop()
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return compositeLogger{loggers: filtered}
}

func (c compositeLogger) Log(ctx context.Context, entry Entry) {
	for _, logger := range c.loggers {
		logger.Log(ctx, entry)
	}
}

func (c compositeLogger) ListBySession(ctx context.Context, sessionID string, limit int) ([]Record, error) {
	items := make([]Record, 0)
	for _, logger := range c.loggers {
		reader, ok := logger.(SessionReader)
		if !ok {
			continue
		}
		records, err := reader.ListBySession(ctx, sessionID, limit)
		if err != nil {
			return nil, err
		}
		items = append(items, records...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	items = dedupeRecords(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (c compositeLogger) List(ctx context.Context, filter ListFilter) ([]Record, error) {
	items := make([]Record, 0)
	for _, logger := range c.loggers {
		reader, ok := logger.(ListReader)
		if !ok {
			continue
		}
		records, err := reader.List(ctx, filter)
		if err != nil {
			return nil, err
		}
		items = append(items, records...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return dedupeRecords(items), nil
}

func (c compositeLogger) ListByIDs(ctx context.Context, ids []string) ([]Record, error) {
	items := make([]Record, 0)
	for _, logger := range c.loggers {
		reader, ok := logger.(BulkReader)
		if !ok {
			continue
		}
		records, err := reader.ListByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		items = append(items, records...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return dedupeRecords(items), nil
}

func dedupeRecords(items []Record) []Record {
	if len(items) <= 1 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]Record, 0, len(items))
	for _, item := range items {
		key := item.ID
		if key == "" {
			key = item.ResourceType + "|" + item.ResourceID + "|" + item.Action + "|" + item.Actor + "|" + item.TraceID + "|" + item.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if item.Metadata != nil {
			item.Metadata = maps.Clone(item.Metadata)
		}
		out = append(out, item)
	}
	return out
}
