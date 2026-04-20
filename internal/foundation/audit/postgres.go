package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

type postgresLogger struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewPostgres(db *sql.DB, logger *slog.Logger) Logger {
	if db == nil {
		return NewNoop()
	}
	return postgresLogger{db: db, logger: logger}
}

func (p postgresLogger) Log(ctx context.Context, entry Entry) {
	if p.db == nil {
		return
	}

	payload, err := json.Marshal(entry.Metadata)
	if err != nil {
		p.logFailure("marshal audit payload failed", err, entry)
		return
	}
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}

	if _, err := p.db.ExecContext(ctx, `
		INSERT INTO audit_logs (
			tenant_id, trace_id, actor_id, resource_type, resource_id, action, payload, created_at
		) VALUES ('default', $1, $2, $3, $4, $5, $6, $7)
	`, nullableString(entry.TraceID), nullableString(entry.Actor), entry.ResourceType, entry.ResourceID, entry.Action, payload, time.Now().UTC()); err != nil {
		p.logFailure("write audit log failed", err, entry)
	}
}

func (p postgresLogger) ListBySession(ctx context.Context, sessionID string, limit int) ([]Record, error) {
	if p.db == nil || strings.TrimSpace(sessionID) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := p.db.QueryContext(ctx, `
		SELECT id::text, resource_type, resource_id, action, COALESCE(actor_id, ''), COALESCE(trace_id, ''), payload, created_at
		FROM audit_logs
		WHERE resource_id = $1
		   OR payload->>'session_id' = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Record, 0, limit)
	for rows.Next() {
		var item Record
		var payload []byte
		if err := rows.Scan(&item.ID, &item.ResourceType, &item.ResourceID, &item.Action, &item.Actor, &item.TraceID, &payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &item.Metadata); err != nil {
				item.Metadata = map[string]any{"raw_payload": string(payload)}
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (p postgresLogger) List(ctx context.Context, filter ListFilter) ([]Record, error) {
	if p.db == nil {
		return nil, nil
	}

	sortBy := strings.TrimSpace(filter.SortBy)
	switch sortBy {
	case "resource_type", "action", "actor", "created_at":
	default:
		sortBy = "created_at"
	}
	sortOrder := strings.ToLower(strings.TrimSpace(filter.SortOrder))
	if sortOrder != "asc" {
		sortOrder = "desc"
	}

	args := []any{"default"}
	conditions := []string{"tenant_id = $1"}
	if value := strings.TrimSpace(filter.ResourceType); value != "" {
		args = append(args, value)
		conditions = append(conditions, "resource_type = $"+strconv.Itoa(len(args)))
	}
	if value := strings.TrimSpace(filter.Action); value != "" {
		args = append(args, value)
		conditions = append(conditions, "action = $"+strconv.Itoa(len(args)))
	}
	if value := strings.TrimSpace(filter.Actor); value != "" {
		args = append(args, value)
		conditions = append(conditions, "COALESCE(actor_id, '') = $"+strconv.Itoa(len(args)))
	}
	if value := strings.TrimSpace(filter.Query); value != "" {
		args = append(args, "%"+value+"%")
		conditions = append(conditions, "(resource_type ILIKE $"+strconv.Itoa(len(args))+" OR resource_id ILIKE $"+strconv.Itoa(len(args))+" OR action ILIKE $"+strconv.Itoa(len(args))+" OR COALESCE(actor_id, '') ILIKE $"+strconv.Itoa(len(args))+" OR payload::text ILIKE $"+strconv.Itoa(len(args))+")")
	}

	query := `
		SELECT id::text, resource_type, resource_id, action, COALESCE(actor_id, ''), COALESCE(trace_id, ''), payload, created_at
		FROM audit_logs
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY ` + sortBy + ` ` + sortOrder + `
	`
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Record, 0, 64)
	for rows.Next() {
		var item Record
		var payload []byte
		if err := rows.Scan(&item.ID, &item.ResourceType, &item.ResourceID, &item.Action, &item.Actor, &item.TraceID, &payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &item.Metadata); err != nil {
				item.Metadata = map[string]any{"raw_payload": string(payload)}
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (p postgresLogger) ListByIDs(ctx context.Context, ids []string) ([]Record, error) {
	if p.db == nil || len(ids) == 0 {
		return nil, nil
	}
	trimmedIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			trimmedIDs = append(trimmedIDs, id)
		}
	}
	if len(trimmedIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, 0, len(trimmedIDs))
	args := make([]any, 0, len(trimmedIDs)+1)
	args = append(args, "default")
	for i, id := range trimmedIDs {
		args = append(args, id)
		placeholders = append(placeholders, "$"+strconv.Itoa(i+2))
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT id::text, resource_type, resource_id, action, COALESCE(actor_id, ''), COALESCE(trace_id, ''), payload, created_at
		FROM audit_logs
		WHERE tenant_id = $1 AND id::text IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY created_at DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Record, 0, len(trimmedIDs))
	for rows.Next() {
		var item Record
		var payload []byte
		if err := rows.Scan(&item.ID, &item.ResourceType, &item.ResourceID, &item.Action, &item.Actor, &item.TraceID, &payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &item.Metadata); err != nil {
				item.Metadata = map[string]any{"raw_payload": string(payload)}
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (p postgresLogger) logFailure(message string, err error, entry Entry) {
	if p.logger == nil {
		return
	}
	p.logger.Error(message,
		"error", err,
		"resource_type", entry.ResourceType,
		"resource_id", entry.ResourceID,
		"action", entry.Action,
	)
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
