// Package msgtpl manages message templates used for channel notifications.
//
// Storage: in-memory (default) or PostgreSQL (when *sql.DB is provided).
// Default templates are seeded on startup. PostgreSQL rows take precedence
// over in-memory defaults when both exist.
//
// Template ID convention: "{type}-{locale}"  e.g. "approval-zh-CN"
package msgtpl

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrTemplateNotFound = errors.New("notification template not found")
	ErrInvalidKind      = errors.New("invalid notification template kind")
	ErrInvalidLocale    = errors.New("invalid notification template locale")
)

// ValidKinds is the set of supported template kinds.
var ValidKinds = map[string]bool{
	"diagnosis":        true,
	"approval":         true,
	"execution_result": true,
}

// ValidLocales is the set of supported locales.
var ValidLocales = map[string]bool{
	"zh-CN": true,
	"en-US": true,
}

// TemplateContent holds the subject and body of a template.
type TemplateContent struct {
	Subject string
	Body    string
}

// NotificationTemplate is the domain model for a message template.
type NotificationTemplate struct {
	ID             string
	Kind           string
	Locale         string
	Name           string
	Status         string
	Enabled        bool
	VariableSchema map[string]string
	UsageRefs      []string
	Content        TemplateContent
	UpdatedAt      time.Time
}

type MsgTemplate = NotificationTemplate

// Render substitutes {{VarName}} placeholders in subject and body with the
// provided data map. Missing keys are left as-is (e.g. "{{Missing}}").
func (t NotificationTemplate) Render(data map[string]string) (subject, body string) {
	replace := func(s string) string {
		return strings.NewReplacer(buildReplacerPairs(data)...).Replace(s)
	}
	return replace(t.Content.Subject), replace(t.Content.Body)
}

func buildReplacerPairs(data map[string]string) []string {
	pairs := make([]string, 0, len(data)*2)
	for k, v := range data {
		pairs = append(pairs, "{{"+k+"}}", v)
	}
	return pairs
}

// Manager holds message templates (in-memory or PostgreSQL backed).
type Manager struct {
	mu        sync.RWMutex
	db        *sql.DB
	templates map[string]*NotificationTemplate // in-memory cache / fallback
	now       func() time.Time
}

// NewManager creates a new Manager backed by optional *sql.DB.
// Seeds default templates on startup; if DB is provided, persists defaults
// to Postgres using INSERT … ON CONFLICT DO NOTHING.
func NewManager(db *sql.DB) *Manager {
	m := &Manager{
		db:        db,
		templates: make(map[string]*NotificationTemplate),
		now:       func() time.Time { return time.Now().UTC() },
	}
	// Always seed in-memory defaults first (fallback if DB unavailable).
	for _, tpl := range defaultTemplates() {
		m.templates[tpl.ID] = tpl
	}
	if db != nil {
		// Seed defaults to DB (ignores conflicts — never overwrites user edits).
		ctx := context.Background()
		for _, tpl := range defaultTemplates() {
			_ = m.insertDefaultDB(ctx, *tpl) // best-effort
		}
	}
	return m
}

// List returns all templates sorted by id.
func (m *Manager) List() []MsgTemplate {
	if m.db != nil {
		if items, err := m.listDB(context.Background()); err == nil {
			return items
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]NotificationTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		out = append(out, *t)
	}
	sortTemplates(out)
	return out
}

// Get returns the template with the given id.
func (m *Manager) Get(id string) (NotificationTemplate, bool) {
	if m.db != nil {
		if t, err := m.getDB(context.Background(), id); err == nil {
			return t, true
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.templates[id]
	if !ok {
		return NotificationTemplate{}, false
	}
	return *t, true
}

// Upsert creates or updates a template, returning the saved value.
func (m *Manager) Upsert(tpl NotificationTemplate) (NotificationTemplate, error) {
	if err := validate(tpl); err != nil {
		return NotificationTemplate{}, err
	}
	if strings.TrimSpace(tpl.ID) == "" {
		tpl.ID = fmt.Sprintf("%s-%s", tpl.Kind, tpl.Locale)
	}
	tpl.Status = firstNonEmpty(strings.TrimSpace(tpl.Status), statusFromEnabled(tpl.Enabled))
	tpl.Enabled = enabledFromStatus(tpl.Status, tpl.Enabled)
	tpl.UsageRefs = dedupeStrings(tpl.UsageRefs)
	tpl.UpdatedAt = m.now()
	if m.db != nil {
		if err := m.upsertDB(context.Background(), tpl); err != nil {
			return NotificationTemplate{}, err
		}
		// Keep in-memory cache in sync.
		m.mu.Lock()
		clone := tpl
		m.templates[tpl.ID] = &clone
		m.mu.Unlock()
		return tpl, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := tpl
	m.templates[tpl.ID] = &clone
	return clone, nil
}

// SetEnabled enables or disables a template.
func (m *Manager) SetEnabled(id string, enabled bool) (NotificationTemplate, error) {
	tpl, ok := m.Get(id)
	if !ok {
		return NotificationTemplate{}, ErrTemplateNotFound
	}
	tpl.Enabled = enabled
	tpl.Status = statusFromEnabled(enabled)
	tpl.UpdatedAt = m.now()
	return m.Upsert(tpl)
}

// --- DB helpers ---

func (m *Manager) listDB(ctx context.Context) ([]NotificationTemplate, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, kind, locale, name, enabled, subject, body, updated_at FROM notification_templates ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NotificationTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (m *Manager) getDB(ctx context.Context, id string) (NotificationTemplate, error) {
	row := m.db.QueryRowContext(ctx,
		`SELECT id, kind, locale, name, enabled, subject, body, updated_at FROM notification_templates WHERE id = $1`, id)
	return scanTemplateRow(row)
}

func (m *Manager) upsertDB(ctx context.Context, tpl NotificationTemplate) error {
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO notification_templates (id, kind, locale, name, enabled, subject, body, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (id) DO UPDATE SET
		   kind=$2, locale=$3, name=$4, enabled=$5, subject=$6, body=$7, updated_at=$8`,
		tpl.ID, tpl.Kind, tpl.Locale, tpl.Name, tpl.Enabled,
		tpl.Content.Subject, tpl.Content.Body, tpl.UpdatedAt,
	)
	return err
}

func (m *Manager) insertDefaultDB(ctx context.Context, tpl NotificationTemplate) error {
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO notification_templates (id, kind, locale, name, enabled, subject, body, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (id) DO NOTHING`,
		tpl.ID, tpl.Kind, tpl.Locale, tpl.Name, tpl.Enabled,
		tpl.Content.Subject, tpl.Content.Body, tpl.UpdatedAt,
	)
	return err
}

// --- scan helpers ---

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTemplate(row rowScanner) (NotificationTemplate, error) {
	var t NotificationTemplate
	if err := row.Scan(&t.ID, &t.Kind, &t.Locale, &t.Name, &t.Enabled,
		&t.Content.Subject, &t.Content.Body, &t.UpdatedAt); err != nil {
		return NotificationTemplate{}, fmt.Errorf("scan notification_template: %w", err)
	}
	return t, nil
}

func scanTemplateRow(row *sql.Row) (NotificationTemplate, error) {
	var t NotificationTemplate
	if err := row.Scan(&t.ID, &t.Kind, &t.Locale, &t.Name, &t.Enabled,
		&t.Content.Subject, &t.Content.Body, &t.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return NotificationTemplate{}, ErrTemplateNotFound
		}
		return NotificationTemplate{}, fmt.Errorf("scan notification_template row: %w", err)
	}
	return t, nil
}

// --- validation ---

func validate(tpl NotificationTemplate) error {
	if !ValidKinds[tpl.Kind] {
		return fmt.Errorf("%w: %q", ErrInvalidKind, tpl.Kind)
	}
	if !ValidLocales[tpl.Locale] {
		return fmt.Errorf("%w: %q", ErrInvalidLocale, tpl.Locale)
	}
	if strings.TrimSpace(tpl.Name) == "" {
		return errors.New("template name is required")
	}
	return nil
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

func statusFromEnabled(enabled bool) string {
	if enabled {
		return "active"
	}
	return "disabled"
}

func enabledFromStatus(status string, fallback bool) bool {
	switch strings.TrimSpace(status) {
	case "active", "enabled":
		return true
	case "disabled", "inactive":
		return false
	default:
		return fallback
	}
}

func sortTemplates(items []NotificationTemplate) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}

// defaultTemplates returns the 6 built-in templates matching the frontend defaults.
func defaultTemplates() []*NotificationTemplate {
	now := time.Now().UTC()
	return []*NotificationTemplate{
		{
			ID:      "diagnosis-zh-CN",
			Kind:    "diagnosis",
			Locale:  "zh-CN",
			Name:    "诊断报告（中文）",
			Enabled: true,
			Content: TemplateContent{
				Subject: "{{AlertIcon}} 告警诊断：{{AlertName}}",
				Body: `**告警诊断报告**
目标上下文：{{TargetContext}}
摘要：{{Summary}}
建议：{{Recommendation}}
参考：{{Citations}}
会话ID：{{SessionID}}`,
			},
			UpdatedAt: now,
		},
		{
			ID:      "diagnosis-en-US",
			Kind:    "diagnosis",
			Locale:  "en-US",
			Name:    "Diagnosis Report (English)",
			Enabled: true,
			Content: TemplateContent{
				Subject: "{{AlertIcon}} Alert Diagnosis: {{AlertName}}",
				Body: `**Alert Diagnosis Report**
Target: {{TargetContext}}
Summary: {{Summary}}
Recommendation: {{Recommendation}}
Citations: {{Citations}}
Session: {{SessionID}}`,
			},
			UpdatedAt: now,
		},
		{
			ID:      "approval-zh-CN",
			Kind:    "approval",
			Locale:  "zh-CN",
			Name:    "执行审批（中文）",
			Enabled: true,
			Content: TemplateContent{
				Subject: "待审批：执行 {{Command}} 于 {{TargetHost}}",
				Body: `**执行审批请求**
执行ID：{{ExecutionID}}
目标主机：{{TargetHost}}
风险等级：{{RiskLevel}}
命令：{{Command}}
审批来源：{{ApprovalSource}}
超时：{{Timeout}}
会话ID：{{SessionID}}`,
			},
			UpdatedAt: now,
		},
		{
			ID:      "approval-en-US",
			Kind:    "approval",
			Locale:  "en-US",
			Name:    "Execution Approval (English)",
			Enabled: true,
			Content: TemplateContent{
				Subject: "Approval Required: Execute {{Command}} on {{TargetHost}}",
				Body: `**Execution Approval Request**
Execution ID: {{ExecutionID}}
Target Host: {{TargetHost}}
Risk Level: {{RiskLevel}}
Command: {{Command}}
Approval Source: {{ApprovalSource}}
Timeout: {{Timeout}}
Session: {{SessionID}}`,
			},
			UpdatedAt: now,
		},
		{
			ID:      "execution_result-zh-CN",
			Kind:    "execution_result",
			Locale:  "zh-CN",
			Name:    "执行结果（中文）",
			Enabled: true,
			Content: TemplateContent{
				Subject: "执行结果：{{ExecutionStatus}} - {{TargetHost}}",
				Body: `**执行结果**
目标主机：{{TargetHost}}
退出码：{{ExitCode}}
输出预览：{{OutputPreview}}{{TruncationFlag}}
{{ActionTip}}
会话ID：{{SessionID}}`,
			},
			UpdatedAt: now,
		},
		{
			ID:      "execution_result-en-US",
			Kind:    "execution_result",
			Locale:  "en-US",
			Name:    "Execution Result (English)",
			Enabled: true,
			Content: TemplateContent{
				Subject: "Execution Result: {{ExecutionStatus}} - {{TargetHost}}",
				Body: `**Execution Result**
Target Host: {{TargetHost}}
Exit Code: {{ExitCode}}
Output: {{OutputPreview}}{{TruncationFlag}}
{{ActionTip}}
Session: {{SessionID}}`,
			},
			UpdatedAt: now,
		},
	}
}
