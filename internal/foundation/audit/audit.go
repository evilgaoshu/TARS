package audit

import (
	"context"
	"time"
)

type Entry struct {
	ResourceType string
	ResourceID   string
	Action       string
	Actor        string
	TraceID      string
	Metadata     map[string]any
}

type Logger interface {
	Log(ctx context.Context, entry Entry)
}

type Record struct {
	ID           string
	ResourceType string
	ResourceID   string
	Action       string
	Actor        string
	TraceID      string
	Metadata     map[string]any
	CreatedAt    time.Time
}

type ListFilter struct {
	Query        string
	ResourceType string
	Action       string
	Actor        string
	SortBy       string
	SortOrder    string
}

type SessionReader interface {
	ListBySession(ctx context.Context, sessionID string, limit int) ([]Record, error)
}

type ListReader interface {
	List(ctx context.Context, filter ListFilter) ([]Record, error)
}

type BulkReader interface {
	ListByIDs(ctx context.Context, ids []string) ([]Record, error)
}

type noopLogger struct{}

func NewNoop() Logger {
	return noopLogger{}
}

func (noopLogger) Log(_ context.Context, _ Entry) {}

func (noopLogger) ListBySession(_ context.Context, _ string, _ int) ([]Record, error) {
	return nil, nil
}

func (noopLogger) List(_ context.Context, _ ListFilter) ([]Record, error) {
	return nil, nil
}

func (noopLogger) ListByIDs(_ context.Context, _ []string) ([]Record, error) {
	return nil, nil
}
