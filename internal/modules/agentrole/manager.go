package agentrole

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"tars/internal/foundation/audit"

	"gopkg.in/yaml.v3"
)

var (
	ErrConfigPathNotSet    = errors.New("agent role config path is not set")
	ErrRoleNotFound        = errors.New("agent role not found")
	ErrRoleIsBuiltin       = errors.New("cannot delete a built-in agent role")
	ErrRoleIDRequired      = errors.New("role_id is required")
	ErrRoleIDConflict      = errors.New("agent role already exists")
	ErrInvalidModelBinding = errors.New("invalid model_binding")
)

// Manager provides thread-safe CRUD for the agent role registry.
type Manager struct {
	mu        sync.RWMutex
	path      string
	content   string
	config    *Config
	updatedAt time.Time
	logger    *slog.Logger
	audit     audit.Logger
	persist   func(Config) error
}

type Options struct {
	Logger *slog.Logger
	Audit  audit.Logger
}

func NewManager(path string, opts Options) (*Manager, error) {
	m := &Manager{
		path:   strings.TrimSpace(path),
		logger: fallbackLogger(opts.Logger),
		audit:  opts.Audit,
	}
	if err := m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
}

// Reload reads the YAML config from disk and ensures built-in roles exist.
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.path == "" {
		m.config = builtinConfig()
		m.updatedAt = time.Now()
		return nil
	}
	raw, err := os.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			m.config = builtinConfig()
			m.content = ""
			m.updatedAt = time.Now()
			return m.persistLocked()
		}
		return fmt.Errorf("read agent_roles config: %w", err)
	}
	legacyProviderPreference, err := containsLegacyProviderPreferenceKey(raw)
	if err != nil {
		return fmt.Errorf("parse agent_roles config: %w", err)
	}
	if legacyProviderPreference {
		return fmt.Errorf("parse agent_roles config: provider_preference has been removed; migrate to model_binding")
	}
	var fc fileConfig
	if err := yaml.Unmarshal(raw, &fc); err != nil {
		return fmt.Errorf("parse agent_roles config: %w", err)
	}
	cfg := &fc.AgentRoles
	ensureBuiltinRoles(cfg)
	for i := range cfg.AgentRoles {
		cfg.AgentRoles[i].ModelBinding = normalizeModelBinding(cfg.AgentRoles[i].ModelBinding)
		if err := validateModelBinding(cfg.AgentRoles[i].ModelBinding); err != nil {
			return fmt.Errorf("parse agent_roles config: role %q: %w", cfg.AgentRoles[i].RoleID, err)
		}
	}
	m.config = cfg
	m.content = string(raw)
	m.updatedAt = time.Now()
	return nil
}

// Snapshot returns a read-only copy of the current state.
func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config == nil {
		return Snapshot{}
	}
	roles := make([]AgentRole, len(m.config.AgentRoles))
	copy(roles, m.config.AgentRoles)
	return Snapshot{
		Path:      m.path,
		Content:   m.content,
		Config:    Config{AgentRoles: roles},
		UpdatedAt: m.updatedAt,
		Loaded:    true,
	}
}

func (m *Manager) SetPersistence(persist func(Config) error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.persist = persist
	m.mu.Unlock()
}

func (m *Manager) LoadRuntimeConfig(cfg Config) error {
	if m == nil {
		return ErrConfigPathNotSet
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.applyConfigLocked(cfg)
}

// Get returns a single agent role by ID.
func (m *Manager) Get(roleID string) (AgentRole, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config == nil {
		return AgentRole{}, ErrRoleNotFound
	}
	for _, r := range m.config.AgentRoles {
		if r.RoleID == roleID {
			return r, nil
		}
	}
	return AgentRole{}, ErrRoleNotFound
}

// List returns all agent roles.
func (m *Manager) List() []AgentRole {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config == nil {
		return nil
	}
	out := make([]AgentRole, len(m.config.AgentRoles))
	copy(out, m.config.AgentRoles)
	return out
}

// Create adds a new agent role. Returns error if the role_id already exists.
func (m *Manager) Create(role AgentRole) (AgentRole, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.config == nil {
		m.config = &Config{}
	}
	role.RoleID = strings.TrimSpace(role.RoleID)
	if role.RoleID == "" {
		return AgentRole{}, ErrRoleIDRequired
	}
	for _, r := range m.config.AgentRoles {
		if r.RoleID == role.RoleID {
			return AgentRole{}, ErrRoleIDConflict
		}
	}
	now := time.Now()
	if role.Status == "" {
		role.Status = "active"
	}
	if role.CapabilityBinding.Mode == "" {
		role.CapabilityBinding.Mode = "unrestricted"
	}
	if role.PolicyBinding.MaxRiskLevel == "" {
		role.PolicyBinding.MaxRiskLevel = "warning"
	}
	if role.PolicyBinding.MaxAction == "" {
		role.PolicyBinding.MaxAction = "require_approval"
	}
	role.ModelBinding = normalizeModelBinding(role.ModelBinding)
	if err := validateModelBinding(role.ModelBinding); err != nil {
		return AgentRole{}, err
	}
	role.CreatedAt = now
	role.UpdatedAt = now

	m.config.AgentRoles = append(m.config.AgentRoles, role)
	if err := m.persistLocked(); err != nil {
		return AgentRole{}, err
	}
	m.auditEvent("agent_role.created", role.RoleID, nil)
	return role, nil
}

// Update modifies an existing agent role. Built-in roles can be updated but not deleted.
func (m *Manager) Update(role AgentRole) (AgentRole, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.config == nil {
		return AgentRole{}, ErrRoleNotFound
	}
	role.RoleID = strings.TrimSpace(role.RoleID)
	idx := -1
	for i, r := range m.config.AgentRoles {
		if r.RoleID == role.RoleID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return AgentRole{}, ErrRoleNotFound
	}
	existing := m.config.AgentRoles[idx]
	role.IsBuiltin = existing.IsBuiltin
	role.CreatedAt = existing.CreatedAt
	role.UpdatedAt = time.Now()
	if role.Status == "" {
		role.Status = existing.Status
	}
	if role.CapabilityBinding.Mode == "" {
		role.CapabilityBinding.Mode = existing.CapabilityBinding.Mode
	}
	if role.PolicyBinding.MaxRiskLevel == "" {
		role.PolicyBinding.MaxRiskLevel = existing.PolicyBinding.MaxRiskLevel
	}
	if role.PolicyBinding.MaxAction == "" {
		role.PolicyBinding.MaxAction = existing.PolicyBinding.MaxAction
	}
	role.ModelBinding = normalizeModelBinding(role.ModelBinding)
	if err := validateModelBinding(role.ModelBinding); err != nil {
		return AgentRole{}, err
	}
	m.config.AgentRoles[idx] = role
	if err := m.persistLocked(); err != nil {
		return AgentRole{}, err
	}
	m.auditEvent("agent_role.updated", role.RoleID, nil)
	return role, nil
}

// Delete removes a non-builtin agent role.
func (m *Manager) Delete(roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.config == nil {
		return ErrRoleNotFound
	}
	idx := -1
	for i, r := range m.config.AgentRoles {
		if r.RoleID == roleID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrRoleNotFound
	}
	if m.config.AgentRoles[idx].IsBuiltin {
		return ErrRoleIsBuiltin
	}
	m.config.AgentRoles = append(m.config.AgentRoles[:idx], m.config.AgentRoles[idx+1:]...)
	if err := m.persistLocked(); err != nil {
		return err
	}
	m.auditEvent("agent_role.deleted", roleID, nil)
	return nil
}

// SetEnabled enables or disables an agent role.
func (m *Manager) SetEnabled(roleID string, enabled bool) (AgentRole, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.config == nil {
		return AgentRole{}, ErrRoleNotFound
	}
	idx := -1
	for i, r := range m.config.AgentRoles {
		if r.RoleID == roleID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return AgentRole{}, ErrRoleNotFound
	}
	status := "disabled"
	if enabled {
		status = "active"
	}
	m.config.AgentRoles[idx].Status = status
	m.config.AgentRoles[idx].UpdatedAt = time.Now()
	if err := m.persistLocked(); err != nil {
		return AgentRole{}, err
	}
	eventType := "agent_role.disabled"
	if enabled {
		eventType = "agent_role.enabled"
	}
	m.auditEvent(eventType, roleID, nil)
	return m.config.AgentRoles[idx], nil
}

// ResolveForSession returns the agent role to use for a session context.
// Falls back to "diagnosis" if roleID is empty or not found.
func (m *Manager) ResolveForSession(roleID string) AgentRole {
	if roleID != "" {
		if r, err := m.Get(roleID); err == nil && r.Status == "active" {
			return r
		}
	}
	if r, err := m.Get("diagnosis"); err == nil {
		return r
	}
	return builtinDiagnosis()
}

// ResolveForAutomation returns the agent role to use for an automation context.
// Falls back to "automation_operator" if roleID is empty or not found.
func (m *Manager) ResolveForAutomation(roleID string) AgentRole {
	if roleID != "" {
		if r, err := m.Get(roleID); err == nil && r.Status == "active" {
			return r
		}
	}
	if r, err := m.Get("automation_operator"); err == nil {
		return r
	}
	return builtinAutomationOperator()
}

// SortedRoles returns agent roles sorted by display name.
func SortedRoles(roles []AgentRole) []AgentRole {
	out := make([]AgentRole, len(roles))
	copy(out, roles)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName)
	})
	return out
}

// ---------- internal ----------

func (m *Manager) persistLocked() error {
	if m.config == nil {
		m.config = builtinConfig()
	}
	fc := fileConfig{AgentRoles: *m.config}
	raw, err := yaml.Marshal(&fc)
	if err != nil {
		return fmt.Errorf("marshal agent_roles config: %w", err)
	}
	if m.path != "" {
		if err := os.WriteFile(m.path, raw, 0o644); err != nil {
			return fmt.Errorf("write agent_roles config: %w", err)
		}
	}
	m.content = string(raw)
	m.updatedAt = time.Now()
	if m.persist != nil {
		return m.persist(Config{AgentRoles: append([]AgentRole(nil), m.config.AgentRoles...)})
	}
	return nil
}

func (m *Manager) applyConfigLocked(cfg Config) error {
	ensureBuiltinRoles(&cfg)
	for i := range cfg.AgentRoles {
		cfg.AgentRoles[i].ModelBinding = normalizeModelBinding(cfg.AgentRoles[i].ModelBinding)
		if err := validateModelBinding(cfg.AgentRoles[i].ModelBinding); err != nil {
			return fmt.Errorf("agent_roles runtime config: role %q: %w", cfg.AgentRoles[i].RoleID, err)
		}
	}
	fc := fileConfig{AgentRoles: cfg}
	raw, err := yaml.Marshal(&fc)
	if err != nil {
		return fmt.Errorf("marshal agent_roles runtime config: %w", err)
	}
	m.config = &cfg
	m.content = string(raw)
	m.updatedAt = time.Now()
	return nil
}

func (m *Manager) auditEvent(eventType, resourceID string, meta map[string]any) {
	if m.audit == nil {
		return
	}
	m.audit.Log(context.Background(), audit.Entry{
		ResourceType: "agent_role",
		ResourceID:   resourceID,
		Action:       eventType,
		Actor:        "platform",
	})
}

func fallbackLogger(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.Default()
}

// ---------- built-in roles ----------

func builtinConfig() *Config {
	return &Config{
		AgentRoles: []AgentRole{
			builtinDiagnosis(),
			builtinAutomationOperator(),
			builtinReviewer(),
			builtinKnowledgeCurator(),
		},
	}
}

func ensureBuiltinRoles(cfg *Config) {
	builtins := []AgentRole{
		builtinDiagnosis(),
		builtinAutomationOperator(),
		builtinReviewer(),
		builtinKnowledgeCurator(),
	}
	existing := map[string]bool{}
	for i := range cfg.AgentRoles {
		existing[cfg.AgentRoles[i].RoleID] = true
		// Ensure the builtin flag is always set correctly
		for _, b := range builtins {
			if cfg.AgentRoles[i].RoleID == b.RoleID {
				cfg.AgentRoles[i].IsBuiltin = true
			}
		}
		cfg.AgentRoles[i].ModelBinding = normalizeModelBinding(cfg.AgentRoles[i].ModelBinding)
	}
	for _, b := range builtins {
		if !existing[b.RoleID] {
			b.ModelBinding = normalizeModelBinding(b.ModelBinding)
			cfg.AgentRoles = append(cfg.AgentRoles, b)
		}
	}
}

func builtinDiagnosis() AgentRole {
	return AgentRole{
		RoleID:      "diagnosis",
		DisplayName: "Diagnosis Expert",
		Description: "Responsible for alert diagnosis, root cause analysis, and metrics querying.",
		Status:      "active",
		IsBuiltin:   true,
		Profile: Profile{
			SystemPrompt: "You are a diagnosis expert in the TARS AIOps platform. Your responsibilities are:\n1. Analyze alerts and metrics data\n2. Identify root causes\n3. Provide evidence-based repair recommendations\nBe precise, cautious, and evidence-driven. Do not guess or execute high-risk operations.",
			PersonaTags:  []string{"diagnosis", "observability", "root-cause-analysis"},
		},
		CapabilityBinding: CapabilityBinding{
			AllowedConnectorCapabilities: []string{
				"metrics.query_instant",
				"metrics.query_range",
				"metrics.capacity_forecast",
				"observability.query",
				"delivery.query",
			},
			Mode: "whitelist",
		},
		PolicyBinding: PolicyBinding{
			MaxRiskLevel:       "warning",
			MaxAction:          "require_approval",
			RequireApprovalFor: []string{"execution.run_command"},
		},
	}
}

func builtinAutomationOperator() AgentRole {
	return AgentRole{
		RoleID:      "automation_operator",
		DisplayName: "Automation Operator",
		Description: "Executes automation jobs, runs skills, and performs scheduled tasks.",
		Status:      "active",
		IsBuiltin:   true,
		Profile: Profile{
			SystemPrompt: "You are an automation operator in the TARS AIOps platform. Execute tasks precisely according to the defined playbook. Report results accurately.",
			PersonaTags:  []string{"automation", "execution", "operator"},
		},
		CapabilityBinding: CapabilityBinding{
			Mode: "unrestricted",
		},
		PolicyBinding: PolicyBinding{
			MaxRiskLevel: "critical",
			MaxAction:    "direct_execute",
		},
	}
}

func builtinReviewer() AgentRole {
	return AgentRole{
		RoleID:      "reviewer",
		DisplayName: "Reviewer",
		Description: "Reviews execution results, performs verification and quality checks.",
		Status:      "active",
		IsBuiltin:   true,
		Profile: Profile{
			SystemPrompt: "You are a reviewer in the TARS AIOps platform. Your role is to verify results, check quality, and provide review feedback. You should not execute any operations directly.",
			PersonaTags:  []string{"review", "verification", "quality"},
		},
		CapabilityBinding: CapabilityBinding{
			AllowedConnectorCapabilities: []string{
				"metrics.query_instant",
				"metrics.query_range",
				"observability.query",
			},
			Mode: "whitelist",
		},
		PolicyBinding: PolicyBinding{
			MaxRiskLevel: "info",
			MaxAction:    "suggest_only",
		},
	}
}

func builtinKnowledgeCurator() AgentRole {
	return AgentRole{
		RoleID:      "knowledge_curator",
		DisplayName: "Knowledge Curator",
		Description: "Maintains the knowledge base, rebuilds indexes, and curates documentation.",
		Status:      "active",
		IsBuiltin:   true,
		Profile: Profile{
			SystemPrompt: "You are a knowledge curator in the TARS AIOps platform. Maintain and organize the knowledge base. Ensure documentation is accurate and up to date.",
			PersonaTags:  []string{"knowledge", "documentation", "curation"},
		},
		CapabilityBinding: CapabilityBinding{
			Mode: "unrestricted",
		},
		PolicyBinding: PolicyBinding{
			MaxRiskLevel: "info",
			MaxAction:    "direct_execute",
		},
	}
}

// actionRank returns a numeric rank for authorization actions (lower = more restrictive).
func actionRank(a string) int {
	switch strings.ToLower(strings.TrimSpace(a)) {
	case "deny":
		return 0
	case "suggest_only":
		return 1
	case "require_approval":
		return 2
	case "direct_execute":
		return 3
	default:
		return 2 // default to require_approval level
	}
}

// riskRank returns a numeric rank for risk levels (lower = more restrictive).
func riskRank(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "info":
		return 1
	case "warning":
		return 2
	case "critical":
		return 3
	default:
		return 1
	}
}

func normalizeModelBinding(binding ModelBinding) ModelBinding {
	binding.Primary = normalizeModelTargetBinding(binding.Primary)
	binding.Fallback = normalizeModelTargetBinding(binding.Fallback)
	if binding.Primary == nil && binding.Fallback == nil {
		binding.InheritPlatformDefault = true
	}
	return binding
}

func normalizeModelTargetBinding(binding *ModelTargetBinding) *ModelTargetBinding {
	if binding == nil {
		return nil
	}
	target := &ModelTargetBinding{
		ProviderID: strings.TrimSpace(binding.ProviderID),
		Model:      strings.TrimSpace(binding.Model),
	}
	if target.ProviderID == "" && target.Model == "" {
		return nil
	}
	return target
}

func containsLegacyProviderPreferenceKey(raw []byte) (bool, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return false, err
	}
	return yamlNodeContainsKey(&root, "provider_preference"), nil
}

func yamlNodeContainsKey(node *yaml.Node, key string) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if strings.EqualFold(strings.TrimSpace(node.Content[i].Value), key) {
				return true
			}
			if yamlNodeContainsKey(node.Content[i+1], key) {
				return true
			}
		}
		return false
	}
	for _, child := range node.Content {
		if yamlNodeContainsKey(child, key) {
			return true
		}
	}
	return false
}

func validateModelBinding(binding ModelBinding) error {
	if err := validateModelTargetBinding("primary", binding.Primary); err != nil {
		return err
	}
	if err := validateModelTargetBinding("fallback", binding.Fallback); err != nil {
		return err
	}
	if binding.Primary == nil && binding.Fallback != nil && !binding.InheritPlatformDefault {
		return fmt.Errorf("%w: fallback requires either primary or inherit_platform_default=true", ErrInvalidModelBinding)
	}
	return nil
}

func validateModelTargetBinding(name string, binding *ModelTargetBinding) error {
	if binding == nil {
		return nil
	}
	providerID := strings.TrimSpace(binding.ProviderID)
	model := strings.TrimSpace(binding.Model)
	switch {
	case providerID == "" && model == "":
		return nil
	case providerID == "":
		return fmt.Errorf("%w: %s.provider_id is required when %s.model is set", ErrInvalidModelBinding, name, name)
	case model == "":
		return fmt.Errorf("%w: %s.model is required when %s.provider_id is set", ErrInvalidModelBinding, name, name)
	default:
		return nil
	}
}

// EnforcePolicy applies the AgentRole's PolicyBinding and CapabilityBinding
// on top of an existing authorization action, returning the more restrictive result.
// The command parameter is used for hard-deny matching against PolicyBinding.HardDeny globs.
func EnforcePolicy(role AgentRole, currentAction string, command string) string {
	// 1. Hard-deny check: if command matches any hard-deny glob, deny outright.
	for _, pattern := range role.PolicyBinding.HardDeny {
		if matchGlob(pattern, command) {
			return "deny"
		}
	}

	// 2. Require-approval check: if command matches any require_approval_for glob.
	for _, pattern := range role.PolicyBinding.RequireApprovalFor {
		if matchGlob(pattern, command) {
			if actionRank(currentAction) > actionRank("require_approval") {
				currentAction = "require_approval"
			}
		}
	}

	// 3. MaxAction constraint: the role's max_action caps the allowed action.
	if role.PolicyBinding.MaxAction != "" && actionRank(currentAction) > actionRank(role.PolicyBinding.MaxAction) {
		currentAction = role.PolicyBinding.MaxAction
	}

	return currentAction
}

// EnforceCapabilityBinding checks whether a connector capability is allowed
// by the role's CapabilityBinding. Returns "deny" if blocked, or the currentAction unchanged.
func EnforceCapabilityBinding(role AgentRole, currentAction string, capabilityID string) string {
	switch strings.ToLower(role.CapabilityBinding.Mode) {
	case "whitelist":
		for _, allowed := range role.CapabilityBinding.AllowedConnectorCapabilities {
			if strings.EqualFold(allowed, capabilityID) {
				return currentAction
			}
		}
		return "deny"
	case "blacklist":
		for _, denied := range role.CapabilityBinding.DeniedConnectorCapabilities {
			if strings.EqualFold(denied, capabilityID) {
				return "deny"
			}
		}
		return currentAction
	default: // unrestricted
		return currentAction
	}
}

// matchGlob performs simple glob matching (supports * as wildcard).
func matchGlob(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	value = strings.TrimSpace(value)
	if pattern == "" || value == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return strings.EqualFold(pattern, value)
	}
	// Simple prefix/suffix glob matching.
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		return strings.Contains(strings.ToLower(value), strings.ToLower(strings.Trim(pattern, "*")))
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(strings.ToLower(value), strings.ToLower(strings.TrimSuffix(pattern, "*")))
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(strings.ToLower(value), strings.ToLower(strings.TrimPrefix(pattern, "*")))
	}
	return strings.EqualFold(pattern, value)
}
