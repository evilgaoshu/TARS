package skills

import (
	"strings"
	"time"
)

const CurrentTARSMajorVersion = "1"

type Manifest struct {
	APIVersion    string              `json:"api_version" yaml:"api_version"`
	Kind          string              `json:"kind" yaml:"kind"`
	Disabled      bool                `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	Metadata      Metadata            `json:"metadata" yaml:"metadata"`
	Spec          Spec                `json:"spec" yaml:"spec"`
	Compatibility CompatibilityReport `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`
	Marketplace   MarketplaceMetadata `json:"marketplace,omitempty" yaml:"marketplace,omitempty"`
}

func (m Manifest) Enabled() bool {
	return !m.Disabled
}

type Metadata struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	DisplayName string   `json:"display_name" yaml:"display_name"`
	Version     string   `json:"version,omitempty" yaml:"version,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Vendor      string   `json:"vendor" yaml:"vendor"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Source      string   `json:"source,omitempty" yaml:"source,omitempty"`
	Content     string   `json:"content,omitempty" yaml:"content,omitempty"`
	OrgID       string   `json:"org_id,omitempty" yaml:"org_id,omitempty"`
	TenantID    string   `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	WorkspaceID string   `json:"workspace_id,omitempty" yaml:"workspace_id,omitempty"`
}

type Spec struct {
	Type       string     `json:"type,omitempty" yaml:"type,omitempty"`
	Triggers   Triggers   `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	Planner    Planner    `json:"planner,omitempty" yaml:"planner,omitempty"`
	Governance Governance `json:"governance,omitempty" yaml:"governance,omitempty"`
}

type Triggers struct {
	Alerts []string `json:"alerts,omitempty" yaml:"alerts,omitempty"`
}

type Planner struct {
	Summary        string        `json:"summary,omitempty" yaml:"summary,omitempty"`
	PreferredTools []string      `json:"preferred_tools,omitempty" yaml:"preferred_tools,omitempty"`
	Steps          []PlannerStep `json:"steps,omitempty" yaml:"steps,omitempty"`
}

type PlannerStep struct {
	ID       string                 `json:"id,omitempty" yaml:"id,omitempty"`
	Tool     string                 `json:"tool" yaml:"tool"`
	Required bool                   `json:"required,omitempty" yaml:"required,omitempty"`
	Reason   string                 `json:"reason,omitempty" yaml:"reason,omitempty"`
	Priority int                    `json:"priority,omitempty" yaml:"priority,omitempty"`
	Params   map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
	Approval *StepApproval          `json:"approval,omitempty" yaml:"approval,omitempty"`
}

type StepApproval struct {
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
	Reason  string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type Governance struct {
	ExecutionPolicy     string              `json:"execution_policy,omitempty" yaml:"execution_policy,omitempty"`
	ReadOnlyFirst       bool                `json:"read_only_first,omitempty" yaml:"read_only_first,omitempty"`
	ConnectorPreference ConnectorPreference `json:"connector_preference,omitempty" yaml:"connector_preference,omitempty"`
}

type ConnectorPreference struct {
	Metrics       []string `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	Execution     []string `json:"execution,omitempty" yaml:"execution,omitempty"`
	Observability []string `json:"observability,omitempty" yaml:"observability,omitempty"`
	Delivery      []string `json:"delivery,omitempty" yaml:"delivery,omitempty"`
}

type MarketplaceMetadata struct {
	Tags   []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Source string   `json:"source,omitempty" yaml:"source,omitempty"`
}

type LifecycleEvent struct {
	Type      string            `json:"type,omitempty" yaml:"type,omitempty"`
	Summary   string            `json:"summary,omitempty" yaml:"summary,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty" yaml:"created_at,omitempty"`
}

type RevisionSnapshot struct {
	Manifest  *Manifest `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	Reason    string    `json:"reason,omitempty" yaml:"reason,omitempty"`
	Action    string    `json:"action,omitempty" yaml:"action,omitempty"`
}

type LifecycleState struct {
	SkillID     string             `json:"skill_id,omitempty" yaml:"skill_id,omitempty"`
	DisplayName string             `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Source      string             `json:"source,omitempty" yaml:"source,omitempty"`
	Status      string             `json:"status,omitempty" yaml:"status,omitempty"`
	ReviewState string             `json:"review_state,omitempty" yaml:"review_state,omitempty"`
	RuntimeMode string             `json:"runtime_mode,omitempty" yaml:"runtime_mode,omitempty"`
	Enabled     bool               `json:"enabled" yaml:"enabled"`
	InstalledAt time.Time          `json:"installed_at,omitempty" yaml:"installed_at,omitempty"`
	UpdatedAt   time.Time          `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	PublishedAt time.Time          `json:"published_at,omitempty" yaml:"published_at,omitempty"`
	History     []LifecycleEvent   `json:"history,omitempty" yaml:"history,omitempty"`
	Revisions   []RevisionSnapshot `json:"revisions,omitempty" yaml:"revisions,omitempty"`
}

type CompatibilityReport struct {
	Compatible       bool      `json:"compatible" yaml:"compatible"`
	CurrentTARSMajor string    `json:"current_tars_major,omitempty" yaml:"current_tars_major,omitempty"`
	Reasons          []string  `json:"reasons,omitempty" yaml:"reasons,omitempty"`
	CheckedAt        time.Time `json:"checked_at,omitempty" yaml:"checked_at,omitempty"`
}

func CompatibilityReportForManifest(entry Manifest) CompatibilityReport {
	reasons := make([]string, 0, 1)
	compatible := true
	if entry.Metadata.ID == "" {
		compatible = false
		reasons = append(reasons, "skill metadata.id is required")
	}

	return CompatibilityReport{
		Compatible:       compatible,
		CurrentTARSMajor: CurrentTARSMajorVersion,
		Reasons:          cloneStrings(reasons),
		CheckedAt:        time.Now().UTC(),
	}
}

func cloneStrings(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	out := make([]string, 0, len(input))
	for _, item := range input {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
