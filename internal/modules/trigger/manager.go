// Package trigger implements platform trigger policies.
//
// A Trigger defines: when a named event fires → deliver a message to a channel
// (optionally using a message template), subject to cooldown/dedupe.
//
// Storage backends: in-memory (default) or PostgreSQL.
package trigger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a trigger record does not exist.
var ErrNotFound = errors.New("trigger not found")

// Built-in event type constants for reference.
const (
	EventOnSkillCompleted     = "on_skill_completed"
	EventOnSkillFailed        = "on_skill_failed"
	EventOnExecutionCompleted = "on_execution_completed"
	EventOnExecutionFailed    = "on_execution_failed"
	EventOnApprovalRequested  = "on_approval_requested"
	EventOnSessionClosed      = "on_session_closed"
)

// Trigger is the domain model.
type Trigger struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	DisplayName     string    `json:"display_name"`
	Description     string    `json:"description"`
	Enabled         bool      `json:"enabled"`
	EventType       string    `json:"event_type"`
	ChannelID       string    `json:"channel_id"`
	AutomationJobID string    `json:"automation_job_id,omitempty"`
	Governance      string    `json:"governance,omitempty"`
	FilterExpr      string    `json:"filter_expr,omitempty"`
	TargetAudience  string    `json:"target_audience,omitempty"`
	TemplateID      string    `json:"template_id"`
	CooldownSec     int       `json:"cooldown_sec"`
	LastFiredAt     time.Time `json:"last_fired_at,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ListFilter controls trigger list queries.
type ListFilter struct {
	TenantID  string
	EventType string
	Enabled   *bool
	Limit     int
	Offset    int
}

// FireEvent is the payload passed when an event fires.
type FireEvent struct {
	EventType    string
	TenantID     string
	RefType      string
	RefID        string
	Subject      string
	Body         string
	Source       string
	TemplateData map[string]string // optional data for template rendering
}

// MatchRequest describes runtime event data used to decide whether an enabled
// trigger should fire.
type MatchRequest struct {
	TenantID       string
	EventType      string
	TargetAudience string
	EventData      map[string]string
}

// Manager manages trigger records.
type Manager struct {
	mu       sync.RWMutex
	db       *sql.DB
	triggers map[string]*Trigger
	now      func() time.Time
}

// NewManager creates a Manager. Pass non-nil *sql.DB for Postgres.
func NewManager(db *sql.DB) *Manager {
	m := &Manager{
		db:       db,
		triggers: make(map[string]*Trigger),
		now:      func() time.Time { return time.Now().UTC() },
	}
	m.seedDefaults()
	return m
}

// seedDefaults adds built-in trigger policies and persists defaults when DB is enabled.
func (m *Manager) seedDefaults() {
	defaults := defaultTriggers(m.now())
	for i := range defaults {
		clone := defaults[i]
		m.triggers[clone.ID] = &clone
	}
	if m.db != nil {
		ctx := context.Background()
		for _, item := range defaults {
			_ = m.insertDefaultDB(ctx, item)
		}
	}
}

func defaultTriggers(now time.Time) []Trigger {
	return []Trigger{
		{
			ID:          "trg_skill_completed",
			TenantID:    "default",
			DisplayName: "Skill Completed → Inbox",
			Description: "将每次 skill 执行完成事件投递到站内信",
			Enabled:     true,
			EventType:   EventOnSkillCompleted,
			ChannelID:   "in_app_inbox",
			TemplateID:  "execution_result-zh-CN",
			CooldownSec: 0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "trg_skill_failed",
			TenantID:    "default",
			DisplayName: "Skill Failed → Inbox",
			Description: "将 skill 执行失败事件投递到站内信",
			Enabled:     true,
			EventType:   EventOnSkillFailed,
			ChannelID:   "in_app_inbox",
			TemplateID:  "execution_result-zh-CN",
			CooldownSec: 0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "trg_approval_requested",
			TenantID:    "default",
			DisplayName: "Approval Requested → Inbox",
			Description: "将审批请求事件投递到站内信",
			Enabled:     true,
			EventType:   EventOnApprovalRequested,
			ChannelID:   "in_app_inbox",
			TemplateID:  "approval-zh-CN",
			CooldownSec: 0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "trg_execution_completed",
			TenantID:    "default",
			DisplayName: "Execution Completed → Inbox",
			Description: "将巡检任务执行完成事件投递到站内信",
			Enabled:     true,
			EventType:   EventOnExecutionCompleted,
			ChannelID:   "in_app_inbox",
			TemplateID:  "execution_result-zh-CN",
			CooldownSec: 0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "trg_execution_failed",
			TenantID:    "default",
			DisplayName: "Execution Failed → Inbox",
			Description: "将命令执行失败事件投递到站内信",
			Enabled:     true,
			EventType:   EventOnExecutionFailed,
			ChannelID:   "in_app_inbox",
			TemplateID:  "execution_result-zh-CN",
			CooldownSec: 0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "trg_session_closed",
			TenantID:    "default",
			DisplayName: "Session Closed → Inbox",
			Description: "将会话关闭事件投递到站内信",
			Enabled:     true,
			EventType:   EventOnSessionClosed,
			ChannelID:   "in_app_inbox",
			TemplateID:  "",
			CooldownSec: 0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

// List returns triggers matching the filter.
func (m *Manager) List(ctx context.Context, filter ListFilter) ([]Trigger, error) {
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if m.db != nil {
		return m.listDB(ctx, filter)
	}
	return m.listMem(filter)
}

func (m *Manager) listMem(filter ListFilter) ([]Trigger, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Trigger, 0, len(m.triggers))
	for _, t := range m.triggers {
		if filter.TenantID != "" && t.TenantID != filter.TenantID {
			continue
		}
		if filter.EventType != "" && t.EventType != filter.EventType {
			continue
		}
		if filter.Enabled != nil && t.Enabled != *filter.Enabled {
			continue
		}
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if filter.Offset >= len(out) {
		return []Trigger{}, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(out) {
		end = len(out)
	}
	return out[filter.Offset:end], nil
}

func (m *Manager) listDB(ctx context.Context, filter ListFilter) ([]Trigger, error) {
	args := []interface{}{}
	where := []string{}
	idx := 1

	if filter.TenantID != "" {
		where = append(where, fmt.Sprintf("tenant_id = $%d", idx))
		args = append(args, filter.TenantID)
		idx++
	}
	if filter.EventType != "" {
		where = append(where, fmt.Sprintf("event_type = $%d", idx))
		args = append(args, filter.EventType)
		idx++
	}
	if filter.Enabled != nil {
		where = append(where, fmt.Sprintf("enabled = $%d", idx))
		args = append(args, *filter.Enabled)
		idx++
	}

	query := `SELECT id, tenant_id, display_name, description, enabled, event_type, channel_id, automation_job_id, governance, filter_expr, target_audience, template_id, cooldown_sec, last_fired_at, created_at, updated_at FROM triggers`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY id LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("trigger list: %w", err)
	}
	defer rows.Close()

	var out []Trigger
	for rows.Next() {
		t, err := scanTriggerRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns the trigger with the given ID.
func (m *Manager) Get(ctx context.Context, id string) (Trigger, error) {
	if m.db != nil {
		return m.getDB(ctx, id)
	}
	return m.getMem(id)
}

func (m *Manager) getMem(id string) (Trigger, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.triggers[id]
	if !ok {
		return Trigger{}, ErrNotFound
	}
	return *t, nil
}

func (m *Manager) getDB(ctx context.Context, id string) (Trigger, error) {
	query := `SELECT id, tenant_id, display_name, description, enabled, event_type, channel_id, automation_job_id, governance, filter_expr, target_audience, template_id, cooldown_sec, last_fired_at, created_at, updated_at FROM triggers WHERE id = $1`
	row := m.db.QueryRowContext(ctx, query, id)
	return scanTrigger(row)
}

// Upsert creates or updates a trigger.
func (m *Manager) Upsert(ctx context.Context, t Trigger) (Trigger, error) {
	if strings.TrimSpace(t.ID) == "" {
		t.ID = "trg_" + uuid.New().String()
	}
	if strings.TrimSpace(t.TenantID) == "" {
		t.TenantID = "default"
	}
	t.ChannelID = strings.TrimSpace(t.ChannelID)
	if t.ChannelID == "" {
		return Trigger{}, fmt.Errorf("channel_id is required")
	}
	t.AutomationJobID = strings.TrimSpace(t.AutomationJobID)
	t.Governance = strings.TrimSpace(t.Governance)
	t.FilterExpr = strings.TrimSpace(t.FilterExpr)
	t.TargetAudience = strings.TrimSpace(t.TargetAudience)
	now := m.now()
	t.UpdatedAt = now
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if m.db != nil {
		return m.upsertDB(ctx, t)
	}
	return m.upsertMem(t)
}

func (m *Manager) upsertMem(t Trigger) (Trigger, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := t
	m.triggers[t.ID] = &clone
	return clone, nil
}

func (m *Manager) upsertDB(ctx context.Context, t Trigger) (Trigger, error) {
	var lastFiredAt *time.Time
	if !t.LastFiredAt.IsZero() {
		lastFiredAt = &t.LastFiredAt
	}
	query := `INSERT INTO triggers (id, tenant_id, display_name, description, enabled, event_type, channel_id, automation_job_id, governance, filter_expr, target_audience, template_id, cooldown_sec, last_fired_at, created_at, updated_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			  ON CONFLICT (id) DO UPDATE SET
			    tenant_id = EXCLUDED.tenant_id,
			    display_name = EXCLUDED.display_name,
			    description = EXCLUDED.description,
			    enabled = EXCLUDED.enabled,
			    event_type = EXCLUDED.event_type,
			    channel_id = EXCLUDED.channel_id,
			    automation_job_id = EXCLUDED.automation_job_id,
			    governance = EXCLUDED.governance,
			    filter_expr = EXCLUDED.filter_expr,
			    target_audience = EXCLUDED.target_audience,
			    template_id = EXCLUDED.template_id,
			    cooldown_sec = EXCLUDED.cooldown_sec,
			    updated_at = EXCLUDED.updated_at`
	_, err := m.db.ExecContext(ctx, query,
		t.ID, t.TenantID, t.DisplayName, t.Description, t.Enabled,
		t.EventType, t.ChannelID, t.AutomationJobID, t.Governance, t.FilterExpr,
		t.TargetAudience, t.TemplateID, t.CooldownSec, lastFiredAt,
		t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return Trigger{}, fmt.Errorf("trigger upsert: %w", err)
	}
	return t, nil
}

func (m *Manager) insertDefaultDB(ctx context.Context, t Trigger) error {
	var lastFiredAt *time.Time
	if !t.LastFiredAt.IsZero() {
		lastFiredAt = &t.LastFiredAt
	}
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO triggers
			(id, tenant_id, display_name, description, enabled, event_type, channel_id, automation_job_id, governance, filter_expr, target_audience, template_id, cooldown_sec, last_fired_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		ON CONFLICT (id) DO NOTHING`,
		t.ID, t.TenantID, t.DisplayName, t.Description, t.Enabled,
		t.EventType, t.ChannelID, t.AutomationJobID, t.Governance, t.FilterExpr, t.TargetAudience, t.TemplateID, t.CooldownSec,
		lastFiredAt, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("trigger insert default: %w", err)
	}
	return nil
}

// SetEnabled enables or disables a trigger.
func (m *Manager) SetEnabled(ctx context.Context, id string, enabled bool) (Trigger, error) {
	t, err := m.Get(ctx, id)
	if err != nil {
		return Trigger{}, err
	}
	t.Enabled = enabled
	t.UpdatedAt = m.now()
	return m.Upsert(ctx, t)
}

// MatchEnabled returns enabled triggers that match the runtime request.
//
// Compatibility forms:
//   - MatchEnabled(ctx, tenantID, eventType)
//   - MatchEnabled(ctx, MatchRequest{...})
func (m *Manager) MatchEnabled(ctx context.Context, args ...interface{}) ([]Trigger, error) {
	req, err := parseMatchRequest(args...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.TenantID) == "" {
		req.TenantID = "default"
	}
	enabled := true
	matched, err := m.List(ctx, ListFilter{
		TenantID:  req.TenantID,
		EventType: req.EventType,
		Enabled:   &enabled,
		Limit:     100,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Trigger, 0, len(matched))
	for _, item := range matched {
		ok, matchErr := matchRuntimeRequest(item, req)
		if matchErr != nil {
			slog.Default().Warn("trigger match skipped due to invalid runtime filter",
				"trigger_id", item.ID,
				"filter_expr", item.FilterExpr,
				"error", matchErr,
			)
			continue
		}
		if ok {
			out = append(out, item)
		}
	}
	return out, nil
}

// RecordFired updates last_fired_at for cooldown tracking.
func (m *Manager) RecordFired(ctx context.Context, id string) error {
	now := m.now()
	if m.db != nil {
		_, err := m.db.ExecContext(ctx,
			`UPDATE triggers SET last_fired_at = $1 WHERE id = $2`, now, id)
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.triggers[id]; ok {
		t.LastFiredAt = now
	}
	return nil
}

// IsOnCooldown returns true if the trigger's cooldown period hasn't elapsed.
func IsOnCooldown(t Trigger, now time.Time) bool {
	if t.CooldownSec <= 0 || t.LastFiredAt.IsZero() {
		return false
	}
	return now.Before(t.LastFiredAt.Add(time.Duration(t.CooldownSec) * time.Second))
}

// --- scan helpers ---

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanTrigger(row rowScanner) (Trigger, error) {
	var t Trigger
	var lastFiredAt sql.NullTime
	if err := row.Scan(
		&t.ID, &t.TenantID, &t.DisplayName, &t.Description,
		&t.Enabled, &t.EventType, &t.ChannelID, &t.AutomationJobID, &t.Governance, &t.FilterExpr, &t.TargetAudience, &t.TemplateID,
		&t.CooldownSec, &lastFiredAt, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Trigger{}, ErrNotFound
		}
		return Trigger{}, fmt.Errorf("trigger scan: %w", err)
	}
	if lastFiredAt.Valid {
		t.LastFiredAt = lastFiredAt.Time
	}
	return t, nil
}

func scanTriggerRow(rows *sql.Rows) (Trigger, error) {
	var t Trigger
	var lastFiredAt sql.NullTime
	if err := rows.Scan(
		&t.ID, &t.TenantID, &t.DisplayName, &t.Description, &t.Enabled,
		&t.EventType, &t.ChannelID, &t.AutomationJobID, &t.Governance, &t.FilterExpr,
		&t.TargetAudience, &t.TemplateID, &t.CooldownSec, &lastFiredAt,
		&t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return Trigger{}, fmt.Errorf("trigger scan row: %w", err)
	}
	if lastFiredAt.Valid {
		t.LastFiredAt = lastFiredAt.Time
	}
	return t, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func isDirectChannelKind(value string) bool {
	return IsDirectDeliveryChannelKind(value)
}

func parseMatchRequest(args ...interface{}) (MatchRequest, error) {
	switch len(args) {
	case 1:
		req, ok := args[0].(MatchRequest)
		if !ok {
			return MatchRequest{}, fmt.Errorf("trigger match: expected MatchRequest, got %T", args[0])
		}
		return req, nil
	case 2:
		tenantID, ok := args[0].(string)
		if !ok {
			return MatchRequest{}, fmt.Errorf("trigger match: expected tenant id string, got %T", args[0])
		}
		eventType, ok := args[1].(string)
		if !ok {
			return MatchRequest{}, fmt.Errorf("trigger match: expected event type string, got %T", args[1])
		}
		return MatchRequest{TenantID: tenantID, EventType: eventType}, nil
	default:
		return MatchRequest{}, fmt.Errorf("trigger match: unsupported argument count %d", len(args))
	}
}

func matchRuntimeRequest(t Trigger, req MatchRequest) (bool, error) {
	if audience := strings.TrimSpace(t.TargetAudience); audience != "" {
		incomingAudience := strings.TrimSpace(req.TargetAudience)
		if incomingAudience == "" {
			incomingAudience = strings.TrimSpace(req.EventData["target_audience"])
		}
		if incomingAudience == "" || incomingAudience != audience {
			return false, nil
		}
	}
	if expr := strings.TrimSpace(t.FilterExpr); expr != "" {
		matched, err := evalFilterExpr(expr, req.EventData)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

var (
	filterEqualsPattern = regexp.MustCompile(`^([A-Za-z0-9_.-]+)\s*(==|!=)\s*'([^']*)'$`)
	filterInPattern     = regexp.MustCompile(`^([A-Za-z0-9_.-]+)\s+in\s+\[(.*)\]$`)
)

func evalFilterExpr(expr string, data map[string]string) (bool, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return true, nil
	}
	if matches := filterEqualsPattern.FindStringSubmatch(trimmed); len(matches) == 4 {
		key := matches[1]
		op := matches[2]
		want := matches[3]
		got := strings.TrimSpace(data[key])
		if op == "==" {
			return got == want, nil
		}
		return got != want, nil
	}
	if matches := filterInPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
		key := matches[1]
		values, err := parseQuotedList(matches[2])
		if err != nil {
			return false, err
		}
		got := strings.TrimSpace(data[key])
		for _, value := range values {
			if got == value {
				return true, nil
			}
		}
		return false, nil
	}
	return false, fmt.Errorf("unsupported filter expression %s", strconv.Quote(expr))
}

func parseQuotedList(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if len(item) < 2 || !strings.HasPrefix(item, "'") || !strings.HasSuffix(item, "'") {
			return nil, fmt.Errorf("invalid list item %s", strconv.Quote(item))
		}
		out = append(out, item[1:len(item)-1])
	}
	if len(out) == 0 {
		return nil, errors.New("empty filter list")
	}
	return out, nil
}
