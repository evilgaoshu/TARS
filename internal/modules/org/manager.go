// Package org manages Organization, Tenant, and Workspace lifecycle.
// It follows the same YAML-backed in-memory pattern as access.Manager so that
// the binary stays compatible with the existing single-tenant deployment without
// requiring a DB or extra configuration.
package org

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Default IDs used when booting in single-tenant compat mode.
const (
	DefaultOrgID       = "default"
	DefaultTenantID    = "default"
	DefaultWorkspaceID = "default"
)

var (
	ErrOrgNotFound       = errors.New("organization not found")
	ErrTenantNotFound    = errors.New("tenant not found")
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrIDRequired        = errors.New("id is required")
	ErrConfigPathNotSet  = errors.New("org config path is not set")
)

// ---------------------------------------------------------------------------
// Core model types
// ---------------------------------------------------------------------------

// Organization is the top-level enterprise boundary.
type Organization struct {
	ID          string    `yaml:"id"          json:"id"`
	Name        string    `yaml:"name"        json:"name"`
	Slug        string    `yaml:"slug"        json:"slug"`
	Status      string    `yaml:"status"      json:"status"`
	Description string    `yaml:"description" json:"description,omitempty"`
	Domain      string    `yaml:"domain"      json:"domain,omitempty"`
	CreatedAt   time.Time `yaml:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `yaml:"updated_at"  json:"updated_at"`
}

// Tenant is a logical partition inside an Organization.
// In single-tenant mode, there is exactly one tenant per org.
type Tenant struct {
	ID              string    `yaml:"id"              json:"id"`
	OrgID           string    `yaml:"org_id"          json:"org_id"`
	Name            string    `yaml:"name"            json:"name"`
	Slug            string    `yaml:"slug"            json:"slug"`
	Status          string    `yaml:"status"          json:"status"`
	Description     string    `yaml:"description"     json:"description,omitempty"`
	DefaultLocale   string    `yaml:"default_locale"  json:"default_locale,omitempty"`
	DefaultTimezone string    `yaml:"default_timezone" json:"default_timezone,omitempty"`
	CreatedAt       time.Time `yaml:"created_at"      json:"created_at"`
	UpdatedAt       time.Time `yaml:"updated_at"      json:"updated_at"`
}

// Workspace is an operational context inside a Tenant.
// It carries the org/tenant affiliation hooks that other platform objects will
// reference in future iterations.
type Workspace struct {
	ID          string    `yaml:"id"          json:"id"`
	TenantID    string    `yaml:"tenant_id"   json:"tenant_id"`
	OrgID       string    `yaml:"org_id"      json:"org_id"`
	Name        string    `yaml:"name"        json:"name"`
	Slug        string    `yaml:"slug"        json:"slug"`
	Status      string    `yaml:"status"      json:"status"`
	Description string    `yaml:"description" json:"description,omitempty"`
	CreatedAt   time.Time `yaml:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `yaml:"updated_at"  json:"updated_at"`
}

// ---------------------------------------------------------------------------
// File config
// ---------------------------------------------------------------------------

type fileConfig struct {
	Org struct {
		Organizations  []Organization `yaml:"organizations,omitempty"`
		Tenants        []Tenant       `yaml:"tenants,omitempty"`
		Workspaces     []Workspace    `yaml:"workspaces,omitempty"`
		OrgPolicies    []OrgPolicy    `yaml:"org_policies,omitempty"`
		TenantPolicies []TenantPolicy `yaml:"tenant_policies,omitempty"`
	} `yaml:"org"`
}

type Config struct {
	Organizations  []Organization
	Tenants        []Tenant
	Workspaces     []Workspace
	OrgPolicies    []OrgPolicy
	TenantPolicies []TenantPolicy
}

// ---------------------------------------------------------------------------
// Manager
// ---------------------------------------------------------------------------

// Manager holds the in-memory org hierarchy and exposes CRUD operations.
// It boots default objects when running in single-tenant compat mode (no config
// path, or empty file), ensuring the existing platform environment still works.
type Manager struct {
	mu             sync.RWMutex
	path           string
	organizations  []Organization
	tenants        []Tenant
	workspaces     []Workspace
	orgPolicies    map[string]OrgPolicy
	tenantPolicies map[string]TenantPolicy
	updatedAt      time.Time
	now            func() time.Time
	persist        func(Config) error
}

// NewManager creates a Manager. If path is empty, it boots the default
// single-tenant hierarchy so callers never need to nil-check.
func NewManager(path string) (*Manager, error) {
	m := &Manager{
		path:           strings.TrimSpace(path),
		orgPolicies:    make(map[string]OrgPolicy),
		tenantPolicies: make(map[string]TenantPolicy),
		now:            func() time.Time { return time.Now().UTC() },
	}
	if m.path == "" {
		m.bootDefaults()
		return m, nil
	}
	if err := m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
}

// bootDefaults seeds a default org / tenant / workspace so the platform can
// run in single-tenant mode without any config file.
func (m *Manager) bootDefaults() {
	now := m.now()
	m.organizations = []Organization{{
		ID: DefaultOrgID, Name: "Default Organization", Slug: DefaultOrgID,
		Status: "active", Description: "Automatically created default organization.",
		CreatedAt: now, UpdatedAt: now,
	}}
	m.tenants = []Tenant{{
		ID: DefaultTenantID, OrgID: DefaultOrgID,
		Name: "Default Tenant", Slug: DefaultTenantID, Status: "active",
		Description: "Automatically created default tenant.",
		CreatedAt:   now, UpdatedAt: now,
	}}
	m.workspaces = []Workspace{{
		ID: DefaultWorkspaceID, TenantID: DefaultTenantID, OrgID: DefaultOrgID,
		Name: "Default Workspace", Slug: DefaultWorkspaceID, Status: "active",
		Description: "Automatically created default workspace.",
		CreatedAt:   now, UpdatedAt: now,
	}}
	m.updatedAt = now
}

// ensureDefaults guarantees that at least one of each type exists.
func (m *Manager) ensureDefaults() {
	now := m.now()
	if len(m.organizations) == 0 {
		m.organizations = []Organization{{
			ID: DefaultOrgID, Name: "Default Organization", Slug: DefaultOrgID,
			Status: "active", CreatedAt: now, UpdatedAt: now,
		}}
	}
	if len(m.tenants) == 0 {
		orgID := m.organizations[0].ID
		m.tenants = []Tenant{{
			ID: DefaultTenantID, OrgID: orgID,
			Name: "Default Tenant", Slug: DefaultTenantID, Status: "active",
			CreatedAt: now, UpdatedAt: now,
		}}
	}
	if len(m.workspaces) == 0 {
		orgID := m.organizations[0].ID
		tenantID := m.tenants[0].ID
		m.workspaces = []Workspace{{
			ID: DefaultWorkspaceID, TenantID: tenantID, OrgID: orgID,
			Name: "Default Workspace", Slug: DefaultWorkspaceID, Status: "active",
			CreatedAt: now, UpdatedAt: now,
		}}
	}
}

// Reload reads the config file and replaces the in-memory state.
func (m *Manager) Reload() error {
	if m.path == "" {
		return ErrConfigPathNotSet
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.bootDefaults()
			return nil
		}
		return fmt.Errorf("org config read: %w", err)
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("org config parse: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.organizations = fc.Org.Organizations
	m.tenants = fc.Org.Tenants
	m.workspaces = fc.Org.Workspaces
	// rebuild policy maps from slices
	m.orgPolicies = make(map[string]OrgPolicy, len(fc.Org.OrgPolicies))
	for _, p := range fc.Org.OrgPolicies {
		m.orgPolicies[p.OrgID] = p
	}
	m.tenantPolicies = make(map[string]TenantPolicy, len(fc.Org.TenantPolicies))
	for _, p := range fc.Org.TenantPolicies {
		m.tenantPolicies[p.TenantID] = p
	}
	m.ensureDefaults()
	m.updatedAt = m.now()
	return nil
}

// save persists the in-memory state to disk. Must be called inside a write lock.
func (m *Manager) save() error {
	if m.path == "" {
		m.updatedAt = m.now()
		if m.persist != nil {
			return m.persist(m.configLocked())
		}
		return nil
	}
	fc := fileConfig{}
	fc.Org.Organizations = m.organizations
	fc.Org.Tenants = m.tenants
	fc.Org.Workspaces = m.workspaces
	// persist policies as slices
	for _, p := range m.orgPolicies {
		fc.Org.OrgPolicies = append(fc.Org.OrgPolicies, p)
	}
	for _, p := range m.tenantPolicies {
		fc.Org.TenantPolicies = append(fc.Org.TenantPolicies, p)
	}
	data, err := yaml.Marshal(fc)
	if err != nil {
		return fmt.Errorf("org config marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return fmt.Errorf("org config mkdir: %w", err)
	}
	if err := os.WriteFile(m.path, data, 0o644); err != nil {
		return fmt.Errorf("org config write: %w", err)
	}
	m.updatedAt = m.now()
	if m.persist != nil {
		return m.persist(m.configLocked())
	}
	return nil
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
	m.organizations = append([]Organization(nil), cfg.Organizations...)
	m.tenants = append([]Tenant(nil), cfg.Tenants...)
	m.workspaces = append([]Workspace(nil), cfg.Workspaces...)
	m.orgPolicies = make(map[string]OrgPolicy, len(cfg.OrgPolicies))
	for _, p := range cfg.OrgPolicies {
		m.orgPolicies[p.OrgID] = p
	}
	m.tenantPolicies = make(map[string]TenantPolicy, len(cfg.TenantPolicies))
	for _, p := range cfg.TenantPolicies {
		m.tenantPolicies[p.TenantID] = p
	}
	m.ensureDefaults()
	m.updatedAt = m.now()
	return nil
}

func (m *Manager) RuntimeConfig() Config {
	if m == nil {
		return Config{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configLocked()
}

func (m *Manager) configLocked() Config {
	cfg := Config{
		Organizations:  append([]Organization(nil), m.organizations...),
		Tenants:        append([]Tenant(nil), m.tenants...),
		Workspaces:     append([]Workspace(nil), m.workspaces...),
		OrgPolicies:    make([]OrgPolicy, 0, len(m.orgPolicies)),
		TenantPolicies: make([]TenantPolicy, 0, len(m.tenantPolicies)),
	}
	for _, p := range m.orgPolicies {
		cfg.OrgPolicies = append(cfg.OrgPolicies, p)
	}
	for _, p := range m.tenantPolicies {
		cfg.TenantPolicies = append(cfg.TenantPolicies, p)
	}
	return cfg
}

// UpdatedAt returns when the config was last changed.
func (m *Manager) UpdatedAt() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.updatedAt
}

// ---------------------------------------------------------------------------
// Organizations
// ---------------------------------------------------------------------------

func (m *Manager) ListOrganizations() []Organization {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Organization, len(m.organizations))
	copy(out, m.organizations)
	return out
}

func (m *Manager) GetOrganization(id string) (Organization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, org := range m.organizations {
		if org.ID == id {
			return org, nil
		}
	}
	return Organization{}, ErrOrgNotFound
}

func (m *Manager) CreateOrganization(org Organization) (Organization, error) {
	if strings.TrimSpace(org.ID) == "" {
		return Organization{}, ErrIDRequired
	}
	now := m.now()
	org.CreatedAt = now
	org.UpdatedAt = now
	if org.Status == "" {
		org.Status = "active"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.organizations {
		if existing.ID == org.ID {
			return Organization{}, fmt.Errorf("organization %q already exists", org.ID)
		}
	}
	m.organizations = append(m.organizations, org)
	return org, m.save()
}

func (m *Manager) UpdateOrganization(id string, org Organization) (Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, existing := range m.organizations {
		if existing.ID == id {
			org.ID = id
			org.CreatedAt = existing.CreatedAt
			org.UpdatedAt = m.now()
			m.organizations[i] = org
			return org, m.save()
		}
	}
	return Organization{}, ErrOrgNotFound
}

func (m *Manager) SetOrganizationStatus(id, status string) (Organization, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, existing := range m.organizations {
		if existing.ID == id {
			m.organizations[i].Status = status
			m.organizations[i].UpdatedAt = m.now()
			return m.organizations[i], m.save()
		}
	}
	return Organization{}, ErrOrgNotFound
}

// ---------------------------------------------------------------------------
// Tenants
// ---------------------------------------------------------------------------

func (m *Manager) ListTenants(orgID string) []Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Tenant
	for _, t := range m.tenants {
		if orgID == "" || t.OrgID == orgID {
			out = append(out, t)
		}
	}
	return out
}

func (m *Manager) GetTenant(id string) (Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tenants {
		if t.ID == id {
			return t, nil
		}
	}
	return Tenant{}, ErrTenantNotFound
}

func (m *Manager) CreateTenant(tenant Tenant) (Tenant, error) {
	if strings.TrimSpace(tenant.ID) == "" {
		return Tenant{}, ErrIDRequired
	}
	now := m.now()
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	if tenant.Status == "" {
		tenant.Status = "active"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.tenants {
		if existing.ID == tenant.ID {
			return Tenant{}, fmt.Errorf("tenant %q already exists", tenant.ID)
		}
	}
	m.tenants = append(m.tenants, tenant)
	return tenant, m.save()
}

func (m *Manager) UpdateTenant(id string, tenant Tenant) (Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, existing := range m.tenants {
		if existing.ID == id {
			tenant.ID = id
			tenant.CreatedAt = existing.CreatedAt
			tenant.UpdatedAt = m.now()
			m.tenants[i] = tenant
			return tenant, m.save()
		}
	}
	return Tenant{}, ErrTenantNotFound
}

func (m *Manager) SetTenantStatus(id, status string) (Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, existing := range m.tenants {
		if existing.ID == id {
			m.tenants[i].Status = status
			m.tenants[i].UpdatedAt = m.now()
			return m.tenants[i], m.save()
		}
	}
	return Tenant{}, ErrTenantNotFound
}

// ---------------------------------------------------------------------------
// Workspaces
// ---------------------------------------------------------------------------

func (m *Manager) ListWorkspaces(tenantID string) []Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Workspace
	for _, ws := range m.workspaces {
		if tenantID == "" || ws.TenantID == tenantID {
			out = append(out, ws)
		}
	}
	return out
}

func (m *Manager) GetWorkspace(id string) (Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ws := range m.workspaces {
		if ws.ID == id {
			return ws, nil
		}
	}
	return Workspace{}, ErrWorkspaceNotFound
}

func (m *Manager) CreateWorkspace(ws Workspace) (Workspace, error) {
	if strings.TrimSpace(ws.ID) == "" {
		return Workspace{}, ErrIDRequired
	}
	now := m.now()
	ws.CreatedAt = now
	ws.UpdatedAt = now
	if ws.Status == "" {
		ws.Status = "active"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.workspaces {
		if existing.ID == ws.ID {
			return Workspace{}, fmt.Errorf("workspace %q already exists", ws.ID)
		}
	}
	m.workspaces = append(m.workspaces, ws)
	return ws, m.save()
}

func (m *Manager) UpdateWorkspace(id string, ws Workspace) (Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, existing := range m.workspaces {
		if existing.ID == id {
			ws.ID = id
			ws.CreatedAt = existing.CreatedAt
			ws.UpdatedAt = m.now()
			m.workspaces[i] = ws
			return ws, m.save()
		}
	}
	return Workspace{}, ErrWorkspaceNotFound
}

func (m *Manager) SetWorkspaceStatus(id, status string) (Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, existing := range m.workspaces {
		if existing.ID == id {
			m.workspaces[i].Status = status
			m.workspaces[i].UpdatedAt = m.now()
			return m.workspaces[i], m.save()
		}
	}
	return Workspace{}, ErrWorkspaceNotFound
}

// ---------------------------------------------------------------------------
// Convenience helpers for single-tenant compat
// ---------------------------------------------------------------------------

// DefaultOrg returns the first active organization or the seeded default.
func (m *Manager) DefaultOrg() Organization {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, o := range m.organizations {
		if o.Status == "active" {
			return o
		}
	}
	if len(m.organizations) > 0 {
		return m.organizations[0]
	}
	return Organization{ID: DefaultOrgID, Name: "Default", Slug: DefaultOrgID, Status: "active"}
}

// DefaultTenant returns the first active tenant or the seeded default.
func (m *Manager) DefaultTenant() Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.tenants {
		if t.Status == "active" {
			return t
		}
	}
	if len(m.tenants) > 0 {
		return m.tenants[0]
	}
	return Tenant{ID: DefaultTenantID, OrgID: DefaultOrgID, Name: "Default", Slug: DefaultTenantID, Status: "active"}
}

// DefaultWorkspace returns the first active workspace or the seeded default.
func (m *Manager) DefaultWorkspace() Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ws := range m.workspaces {
		if ws.Status == "active" {
			return ws
		}
	}
	if len(m.workspaces) > 0 {
		return m.workspaces[0]
	}
	return Workspace{ID: DefaultWorkspaceID, TenantID: DefaultTenantID, OrgID: DefaultOrgID, Name: "Default", Slug: DefaultWorkspaceID, Status: "active"}
}
