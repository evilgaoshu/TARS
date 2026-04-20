// Package org — policy.go
// ORG-N5: Organization-level policy with per-tenant override.
//
// OrgPolicy defines platform-wide governance defaults at the organization
// level. A Tenant can override individual settings; any field left at zero
// value inherits the parent Org policy.
//
// This is the "base layer" of a classic three-tier policy inheritance:
//
//	Platform defaults → Organization policy → Tenant override
//
// Full enforcement (e.g. rejecting logins from non-allowed methods or
// enforcing hard session timeouts) is the responsibility of the handlers that
// consume the resolved policy. This module only provides the storage, merge,
// and query API.
package org

import (
	"time"
)

// ---------------------------------------------------------------------------
// Policy models
// ---------------------------------------------------------------------------

// OrgPolicy holds governance defaults for an Organization.
// All fields are optional; zero value means "not set / inherit platform default".
type OrgPolicy struct {
	// OrgID that owns this policy. Empty means "platform-wide default".
	OrgID string `yaml:"org_id" json:"org_id,omitempty"`

	// Authentication constraints
	AllowedAuthMethods []string `yaml:"allowed_auth_methods" json:"allowed_auth_methods,omitempty"`
	RequireMFA         bool     `yaml:"require_mfa"          json:"require_mfa"`

	// Session governance
	MaxSessionDuration time.Duration `yaml:"max_session_duration"  json:"max_session_duration_s,omitempty"`
	IdleSessionTimeout time.Duration `yaml:"idle_session_timeout"  json:"idle_session_timeout_s,omitempty"`

	// Approval requirements
	RequireApprovalForExecution bool `yaml:"require_approval_for_execution" json:"require_approval_for_execution"`
	ProhibitSelfApproval        bool `yaml:"prohibit_self_approval"         json:"prohibit_self_approval"`

	// Default roles assigned to JIT-provisioned users within this org
	DefaultJITRoles []string `yaml:"default_jit_roles" json:"default_jit_roles,omitempty"`

	// Which skills / capabilities are allowed / blocked at org level
	SkillAllowlist []string `yaml:"skill_allowlist" json:"skill_allowlist,omitempty"`
	SkillBlocklist []string `yaml:"skill_blocklist" json:"skill_blocklist,omitempty"`

	// Audit and data retention
	AuditRetentionDays     int `yaml:"audit_retention_days"   json:"audit_retention_days,omitempty"`
	KnowledgeRetentionDays int `yaml:"knowledge_retention_days" json:"knowledge_retention_days,omitempty"`

	// Metadata
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at,omitempty"`
}

// TenantPolicy holds per-tenant overrides. Any field that is non-zero
// overrides the parent OrgPolicy for that tenant.
type TenantPolicy struct {
	TenantID string `yaml:"tenant_id" json:"tenant_id"`
	OrgID    string `yaml:"org_id"    json:"org_id,omitempty"`

	// Override auth methods for this tenant (nil = inherit org)
	AllowedAuthMethods *[]string `yaml:"allowed_auth_methods" json:"allowed_auth_methods,omitempty"`
	RequireMFA         *bool     `yaml:"require_mfa"          json:"require_mfa,omitempty"`

	// Session overrides
	MaxSessionDuration *time.Duration `yaml:"max_session_duration" json:"max_session_duration_s,omitempty"`
	IdleSessionTimeout *time.Duration `yaml:"idle_session_timeout" json:"idle_session_timeout_s,omitempty"`

	// Approval overrides
	RequireApprovalForExecution *bool `yaml:"require_approval_for_execution" json:"require_approval_for_execution,omitempty"`
	ProhibitSelfApproval        *bool `yaml:"prohibit_self_approval"         json:"prohibit_self_approval,omitempty"`

	// JIT roles override
	DefaultJITRoles *[]string `yaml:"default_jit_roles" json:"default_jit_roles,omitempty"`

	// Skill allow/block overrides (override entire list if set)
	SkillAllowlist *[]string `yaml:"skill_allowlist" json:"skill_allowlist,omitempty"`
	SkillBlocklist *[]string `yaml:"skill_blocklist" json:"skill_blocklist,omitempty"`

	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at,omitempty"`
}

// ResolvedPolicy is the effective policy after merging OrgPolicy with a
// TenantPolicy override. This is what handlers consume.
type ResolvedPolicy struct {
	OrgID    string `json:"org_id"`
	TenantID string `json:"tenant_id"`

	AllowedAuthMethods          []string
	RequireMFA                  bool
	MaxSessionDuration          time.Duration
	IdleSessionTimeout          time.Duration
	RequireApprovalForExecution bool
	ProhibitSelfApproval        bool
	DefaultJITRoles             []string
	SkillAllowlist              []string
	SkillBlocklist              []string
	AuditRetentionDays          int
	KnowledgeRetentionDays      int
}

// ---------------------------------------------------------------------------
// Manager extensions for ORG-N5
// ---------------------------------------------------------------------------

// GetOrgPolicy returns the OrgPolicy for the given org ID.
// Returns a zero-value policy if none has been set.
func (m *Manager) GetOrgPolicy(orgID string) OrgPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.orgPolicies[orgID]; ok {
		return p
	}
	return OrgPolicy{OrgID: orgID}
}

// SetOrgPolicy stores (or fully replaces) the policy for an org.
func (m *Manager) SetOrgPolicy(policy OrgPolicy) error {
	now := m.now()
	policy.UpdatedAt = now
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.orgPolicies == nil {
		m.orgPolicies = make(map[string]OrgPolicy)
	}
	m.orgPolicies[policy.OrgID] = policy
	return m.save()
}

// GetTenantPolicy returns the TenantPolicy for a given tenant.
func (m *Manager) GetTenantPolicy(tenantID string) TenantPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.tenantPolicies[tenantID]; ok {
		return p
	}
	return TenantPolicy{TenantID: tenantID}
}

// SetTenantPolicy stores (or fully replaces) the policy for a tenant.
func (m *Manager) SetTenantPolicy(policy TenantPolicy) error {
	now := m.now()
	policy.UpdatedAt = now
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tenantPolicies == nil {
		m.tenantPolicies = make(map[string]TenantPolicy)
	}
	m.tenantPolicies[policy.TenantID] = policy
	return m.save()
}

// ResolvePolicy merges the OrgPolicy with any TenantPolicy override and
// returns the effective ResolvedPolicy. TenantPolicy non-nil pointer fields
// always win over OrgPolicy values.
func (m *Manager) ResolvePolicy(orgID, tenantID string) ResolvedPolicy {
	org := m.GetOrgPolicy(orgID)
	tenant := m.GetTenantPolicy(tenantID)

	resolved := ResolvedPolicy{
		OrgID:                       orgID,
		TenantID:                    tenantID,
		AllowedAuthMethods:          org.AllowedAuthMethods,
		RequireMFA:                  org.RequireMFA,
		MaxSessionDuration:          org.MaxSessionDuration,
		IdleSessionTimeout:          org.IdleSessionTimeout,
		RequireApprovalForExecution: org.RequireApprovalForExecution,
		ProhibitSelfApproval:        org.ProhibitSelfApproval,
		DefaultJITRoles:             org.DefaultJITRoles,
		SkillAllowlist:              org.SkillAllowlist,
		SkillBlocklist:              org.SkillBlocklist,
		AuditRetentionDays:          org.AuditRetentionDays,
		KnowledgeRetentionDays:      org.KnowledgeRetentionDays,
	}

	// Apply tenant overrides for non-nil pointer fields
	if tenant.AllowedAuthMethods != nil {
		resolved.AllowedAuthMethods = *tenant.AllowedAuthMethods
	}
	if tenant.RequireMFA != nil {
		resolved.RequireMFA = *tenant.RequireMFA
	}
	if tenant.MaxSessionDuration != nil {
		resolved.MaxSessionDuration = *tenant.MaxSessionDuration
	}
	if tenant.IdleSessionTimeout != nil {
		resolved.IdleSessionTimeout = *tenant.IdleSessionTimeout
	}
	if tenant.RequireApprovalForExecution != nil {
		resolved.RequireApprovalForExecution = *tenant.RequireApprovalForExecution
	}
	if tenant.ProhibitSelfApproval != nil {
		resolved.ProhibitSelfApproval = *tenant.ProhibitSelfApproval
	}
	if tenant.DefaultJITRoles != nil {
		resolved.DefaultJITRoles = *tenant.DefaultJITRoles
	}
	if tenant.SkillAllowlist != nil {
		resolved.SkillAllowlist = *tenant.SkillAllowlist
	}
	if tenant.SkillBlocklist != nil {
		resolved.SkillBlocklist = *tenant.SkillBlocklist
	}

	return resolved
}
