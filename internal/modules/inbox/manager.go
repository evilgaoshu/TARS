// Package inbox implements the in-app inbox (站内信) channel.
//
// It supports two storage backends:
//   - In-memory (default, for non-Postgres deployments)
//   - PostgreSQL (when a *sql.DB is provided)
//
// The Manager satisfies the contracts.InboxService interface so it can be
// injected wherever inbox delivery is needed.
package inbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a message does not exist.
var ErrNotFound = errors.New("inbox message not found")

// Message is the domain model for an in-app inbox message.
type Message struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Channel   string    `json:"channel"`
	RefType   string    `json:"ref_type"`
	RefID     string    `json:"ref_id"`
	Source    string    `json:"source"`
	Actions   []Action  `json:"actions,omitempty"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
	ReadAt    time.Time `json:"read_at,omitempty"`
}

type Action struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ListFilter controls inbox list queries.
type ListFilter struct {
	TenantID   string
	UnreadOnly bool
	Limit      int
	Offset     int
}

// Manager stores and retrieves inbox messages.
type Manager struct {
	mu   sync.RWMutex
	db   *sql.DB
	msgs map[string]*Message // in-memory fallback
	now  func() time.Time
}

// NewManager creates a Manager. Pass a non-nil *sql.DB to use Postgres.
func NewManager(db *sql.DB) *Manager {
	return &Manager{
		db:   db,
		msgs: make(map[string]*Message),
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// Create stores a new inbox message and returns the saved copy.
func (m *Manager) Create(ctx context.Context, msg Message) (Message, error) {
	if strings.TrimSpace(msg.ID) == "" {
		msg.ID = "inbox_" + uuid.New().String()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = m.now()
	}
	if strings.TrimSpace(msg.TenantID) == "" {
		msg.TenantID = "default"
	}
	if strings.TrimSpace(msg.Channel) == "" {
		msg.Channel = "in_app_inbox"
	}

	if m.db != nil {
		return m.createDB(ctx, msg)
	}
	return m.createMem(msg)
}

func (m *Manager) createMem(msg Message) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := msg
	m.msgs[msg.ID] = &clone
	return clone, nil
}

func (m *Manager) createDB(ctx context.Context, msg Message) (Message, error) {
	var readAt *time.Time
	actionsJSON, err := json.Marshal(msg.Actions)
	if err != nil {
		return Message{}, fmt.Errorf("inbox actions marshal: %w", err)
	}
	if !msg.ReadAt.IsZero() {
		readAt = &msg.ReadAt
	}
	_, err = m.db.ExecContext(ctx,
		`INSERT INTO inbox_messages
			(id, tenant_id, subject, body, channel, ref_type, ref_id, source, actions, is_read, created_at, read_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO NOTHING`,
		msg.ID, msg.TenantID, msg.Subject, msg.Body,
		msg.Channel, msg.RefType, msg.RefID, msg.Source,
		nullableJSON(actionsJSON), msg.IsRead, msg.CreatedAt, readAt,
	)
	if err != nil {
		return Message{}, fmt.Errorf("inbox create: %w", err)
	}
	return msg, nil
}

// Get returns the message with the given ID.
func (m *Manager) Get(ctx context.Context, id string) (Message, error) {
	if m.db != nil {
		return m.getDB(ctx, id)
	}
	return m.getMem(id)
}

func (m *Manager) getMem(id string) (Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msg, ok := m.msgs[id]
	if !ok {
		return Message{}, ErrNotFound
	}
	return *msg, nil
}

func (m *Manager) getDB(ctx context.Context, id string) (Message, error) {
	row := m.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, subject, body, channel, ref_type, ref_id, source, actions, is_read, created_at, read_at
		 FROM inbox_messages WHERE id = $1`, id)
	return scanMessage(row)
}

// List returns messages matching the filter.
func (m *Manager) List(ctx context.Context, filter ListFilter) ([]Message, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if m.db != nil {
		return m.listDB(ctx, filter)
	}
	return m.listMem(filter)
}

func (m *Manager) listMem(filter ListFilter) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Message, 0, len(m.msgs))
	for _, msg := range m.msgs {
		if filter.TenantID != "" && msg.TenantID != filter.TenantID {
			continue
		}
		if filter.UnreadOnly && msg.IsRead {
			continue
		}
		out = append(out, *msg)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if filter.Offset >= len(out) {
		return []Message{}, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(out) {
		end = len(out)
	}
	return out[filter.Offset:end], nil
}

func (m *Manager) listDB(ctx context.Context, filter ListFilter) ([]Message, error) {
	args := []interface{}{}
	where := []string{}
	idx := 1

	if filter.TenantID != "" {
		where = append(where, fmt.Sprintf("tenant_id = $%d", idx))
		args = append(args, filter.TenantID)
		idx++
	}
	if filter.UnreadOnly {
		where = append(where, "is_read = FALSE")
	}

	query := "SELECT id, tenant_id, subject, body, channel, ref_type, ref_id, source, actions, is_read, created_at, read_at FROM inbox_messages"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("inbox list: %w", err)
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		msg, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, rows.Err()
}

// CountUnread returns the number of unread messages for a tenant.
func (m *Manager) CountUnread(ctx context.Context, tenantID string) (int, error) {
	if m.db != nil {
		var count int
		err := m.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM inbox_messages WHERE tenant_id = $1 AND is_read = FALSE`, tenantID).Scan(&count)
		if err != nil {
			return 0, fmt.Errorf("inbox count_unread: %w", err)
		}
		return count, nil
	}
	msgs, err := m.listMem(ListFilter{TenantID: tenantID, UnreadOnly: true, Limit: 10000})
	if err != nil {
		return 0, err
	}
	return len(msgs), nil
}

// MarkRead marks a message as read.
func (m *Manager) MarkRead(ctx context.Context, id string) (Message, error) {
	if m.db != nil {
		return m.markReadDB(ctx, id)
	}
	return m.markReadMem(id)
}

func (m *Manager) markReadMem(id string) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg, ok := m.msgs[id]
	if !ok {
		return Message{}, ErrNotFound
	}
	msg.IsRead = true
	msg.ReadAt = m.now()
	return *msg, nil
}

func (m *Manager) markReadDB(ctx context.Context, id string) (Message, error) {
	now := m.now()
	_, err := m.db.ExecContext(ctx,
		`UPDATE inbox_messages SET is_read = TRUE, read_at = $1 WHERE id = $2`, now, id)
	if err != nil {
		return Message{}, fmt.Errorf("inbox mark_read: %w", err)
	}
	return m.getDB(ctx, id)
}

// MarkAllRead marks all unread messages for a tenant as read.
func (m *Manager) MarkAllRead(ctx context.Context, tenantID string) (int64, error) {
	if m.db != nil {
		res, err := m.db.ExecContext(ctx,
			`UPDATE inbox_messages SET is_read = TRUE, read_at = $1 WHERE tenant_id = $2 AND is_read = FALSE`,
			m.now(), tenantID)
		if err != nil {
			return 0, fmt.Errorf("inbox mark_all_read: %w", err)
		}
		return res.RowsAffected()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	now := m.now()
	for _, msg := range m.msgs {
		if msg.TenantID == tenantID && !msg.IsRead {
			msg.IsRead = true
			msg.ReadAt = now
			count++
		}
	}
	return count, nil
}

// --- scan helpers ---

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanMessage(row rowScanner) (Message, error) {
	var msg Message
	var readAt sql.NullTime
	var actionsRaw []byte
	if err := row.Scan(
		&msg.ID, &msg.TenantID, &msg.Subject, &msg.Body,
		&msg.Channel, &msg.RefType, &msg.RefID, &msg.Source, &actionsRaw,
		&msg.IsRead, &msg.CreatedAt, &readAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Message{}, ErrNotFound
		}
		return Message{}, fmt.Errorf("inbox scan: %w", err)
	}
	if readAt.Valid {
		msg.ReadAt = readAt.Time
	}
	if len(actionsRaw) > 0 {
		_ = json.Unmarshal(actionsRaw, &msg.Actions)
	}
	return msg, nil
}

func scanMessageRow(rows *sql.Rows) (Message, error) {
	var msg Message
	var readAt sql.NullTime
	var actionsRaw []byte
	if err := rows.Scan(
		&msg.ID, &msg.TenantID, &msg.Subject, &msg.Body,
		&msg.Channel, &msg.RefType, &msg.RefID, &msg.Source, &actionsRaw,
		&msg.IsRead, &msg.CreatedAt, &readAt,
	); err != nil {
		return Message{}, fmt.Errorf("inbox scan row: %w", err)
	}
	if readAt.Valid {
		msg.ReadAt = readAt.Time
	}
	if len(actionsRaw) > 0 {
		_ = json.Unmarshal(actionsRaw, &msg.Actions)
	}
	return msg, nil
}

func nullableJSON(value []byte) interface{} {
	if len(value) == 0 {
		return nil
	}
	return value
}
