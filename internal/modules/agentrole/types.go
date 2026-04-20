package agentrole

import "time"

// AgentRole defines an AI agent's identity, capability scope, and risk boundary.
// It is the 10th first-class platform component in TARS.
type AgentRole struct {
	RoleID      string `yaml:"role_id" json:"role_id"`
	DisplayName string `yaml:"display_name" json:"display_name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Status      string `yaml:"status" json:"status"` // active | disabled
	IsBuiltin   bool   `yaml:"is_builtin" json:"is_builtin"`

	Profile           Profile           `yaml:"profile" json:"profile"`
	CapabilityBinding CapabilityBinding `yaml:"capability_binding" json:"capability_binding"`
	PolicyBinding     PolicyBinding     `yaml:"policy_binding" json:"policy_binding"`
	ModelBinding      ModelBinding      `yaml:"model_binding,omitempty" json:"model_binding,omitempty"`

	OrgID    string `yaml:"org_id,omitempty" json:"org_id,omitempty"`
	TenantID string `yaml:"tenant_id,omitempty" json:"tenant_id,omitempty"`

	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`
}

// Profile defines the agent's persona: system prompt and semantic tags.
type Profile struct {
	SystemPrompt string   `yaml:"system_prompt" json:"system_prompt"`
	PersonaTags  []string `yaml:"persona_tags,omitempty" json:"persona_tags,omitempty"`
}

// CapabilityBinding defines what connector capabilities and skills this role may use.
type CapabilityBinding struct {
	AllowedConnectorCapabilities []string `yaml:"allowed_connector_capabilities,omitempty" json:"allowed_connector_capabilities,omitempty"`
	DeniedConnectorCapabilities  []string `yaml:"denied_connector_capabilities,omitempty" json:"denied_connector_capabilities,omitempty"`
	AllowedSkills                []string `yaml:"allowed_skills,omitempty" json:"allowed_skills,omitempty"`
	AllowedSkillTags             []string `yaml:"allowed_skill_tags,omitempty" json:"allowed_skill_tags,omitempty"`
	Mode                         string   `yaml:"mode" json:"mode"` // whitelist | blacklist | unrestricted
}

// PolicyBinding defines the risk and action boundaries for this role.
type PolicyBinding struct {
	MaxRiskLevel       string   `yaml:"max_risk_level" json:"max_risk_level"` // info | warning | critical
	MaxAction          string   `yaml:"max_action" json:"max_action"`         // direct_execute | require_approval | suggest_only
	RequireApprovalFor []string `yaml:"require_approval_for,omitempty" json:"require_approval_for,omitempty"`
	HardDeny           []string `yaml:"hard_deny,omitempty" json:"hard_deny,omitempty"`
}

type ModelTargetBinding struct {
	ProviderID string `yaml:"provider_id,omitempty" json:"provider_id,omitempty"`
	Model      string `yaml:"model,omitempty" json:"model,omitempty"`
}

type ModelBinding struct {
	Primary                *ModelTargetBinding `yaml:"primary,omitempty" json:"primary,omitempty"`
	Fallback               *ModelTargetBinding `yaml:"fallback,omitempty" json:"fallback,omitempty"`
	InheritPlatformDefault bool                `yaml:"inherit_platform_default,omitempty" json:"inherit_platform_default,omitempty"`
}

// Config is the top-level configuration containing all agent roles.
type Config struct {
	AgentRoles []AgentRole `yaml:"agent_roles,omitempty"`
}

// Snapshot captures the current state of the agent role registry.
type Snapshot struct {
	Path      string
	Content   string
	Config    Config
	UpdatedAt time.Time
	Loaded    bool
}

type fileConfig struct {
	AgentRoles Config `yaml:"agent_roles"`
}
