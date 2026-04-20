package connectors

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
	Config        RuntimeConfig       `json:"config,omitempty" yaml:"config,omitempty"`
	Compatibility Compatibility       `json:"compatibility" yaml:"compatibility"`
	Marketplace   MarketplaceMetadata `json:"marketplace,omitempty" yaml:"marketplace,omitempty"`
}

func (m Manifest) Enabled() bool {
	return !m.Disabled
}

type Metadata struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	DisplayName string `json:"display_name" yaml:"display_name"`
	Vendor      string `json:"vendor" yaml:"vendor"`
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	OrgID       string `json:"org_id,omitempty" yaml:"org_id,omitempty"`
	TenantID    string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty" yaml:"workspace_id,omitempty"`
}

type Spec struct {
	Type           string       `json:"type" yaml:"type"`
	Protocol       string       `json:"protocol" yaml:"protocol"`
	Capabilities   []Capability `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	ConnectionForm []Field      `json:"connection_form,omitempty" yaml:"connection_form,omitempty"`
	ImportExport   ImportExport `json:"import_export" yaml:"import_export"`
}

type Capability struct {
	ID          string   `json:"id" yaml:"id"`
	Action      string   `json:"action" yaml:"action"`
	ReadOnly    bool     `json:"read_only" yaml:"read_only"`
	Invocable   bool     `json:"invocable,omitempty" yaml:"invocable,omitempty"`
	Scopes      []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
}

type Field struct {
	Key         string   `json:"key" yaml:"key"`
	Label       string   `json:"label" yaml:"label"`
	Type        string   `json:"type" yaml:"type"`
	Required    bool     `json:"required" yaml:"required"`
	Secret      bool     `json:"secret,omitempty" yaml:"secret,omitempty"`
	Default     string   `json:"default,omitempty" yaml:"default,omitempty"`
	Options     []string `json:"options,omitempty" yaml:"options,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
}

type ImportExport struct {
	Exportable bool     `json:"exportable" yaml:"exportable"`
	Importable bool     `json:"importable" yaml:"importable"`
	Formats    []string `json:"formats,omitempty" yaml:"formats,omitempty"`
}

type RuntimeConfig struct {
	Values     map[string]string `json:"values,omitempty" yaml:"values,omitempty"`
	SecretRefs map[string]string `json:"secret_refs,omitempty" yaml:"secret_refs,omitempty"`
}

type Compatibility struct {
	TARSMajorVersions     []string `json:"tars_major_versions,omitempty" yaml:"tars_major_versions,omitempty"`
	UpstreamMajorVersions []string `json:"upstream_major_versions,omitempty" yaml:"upstream_major_versions,omitempty"`
	Modes                 []string `json:"modes,omitempty" yaml:"modes,omitempty"`
}

type MarketplaceMetadata struct {
	Category string   `json:"category,omitempty" yaml:"category,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Source   string   `json:"source,omitempty" yaml:"source,omitempty"`
}

type RuntimeMetadata struct {
	Type     string `json:"type,omitempty" yaml:"type,omitempty"`
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Vendor   string `json:"vendor,omitempty" yaml:"vendor,omitempty"`
	Mode     string `json:"mode,omitempty" yaml:"mode,omitempty"`
}

type CompatibilityReport struct {
	Compatible       bool      `json:"compatible" yaml:"compatible"`
	CurrentTARSMajor string    `json:"current_tars_major,omitempty" yaml:"current_tars_major,omitempty"`
	Reasons          []string  `json:"reasons,omitempty" yaml:"reasons,omitempty"`
	CheckedAt        time.Time `json:"checked_at,omitempty" yaml:"checked_at,omitempty"`
}

type HealthStatus struct {
	Status           string    `json:"status,omitempty" yaml:"status,omitempty"`
	CredentialStatus string    `json:"credential_status,omitempty" yaml:"credential_status,omitempty"`
	Summary          string    `json:"summary,omitempty" yaml:"summary,omitempty"`
	CheckedAt        time.Time `json:"checked_at,omitempty" yaml:"checked_at,omitempty"`
}

type LifecycleEvent struct {
	Type        string            `json:"type,omitempty" yaml:"type,omitempty"`
	Summary     string            `json:"summary,omitempty" yaml:"summary,omitempty"`
	Version     string            `json:"version,omitempty" yaml:"version,omitempty"`
	FromVersion string            `json:"from_version,omitempty" yaml:"from_version,omitempty"`
	ToVersion   string            `json:"to_version,omitempty" yaml:"to_version,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty" yaml:"created_at,omitempty"`
}

type RevisionSnapshot struct {
	Version   string    `json:"version,omitempty" yaml:"version,omitempty"`
	Manifest  *Manifest `json:"manifest,omitempty" yaml:"manifest,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	Reason    string    `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type LifecycleState struct {
	ConnectorID      string               `json:"connector_id,omitempty" yaml:"connector_id,omitempty"`
	DisplayName      string               `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	CurrentVersion   string               `json:"current_version,omitempty" yaml:"current_version,omitempty"`
	AvailableVersion string               `json:"available_version,omitempty" yaml:"available_version,omitempty"`
	Enabled          bool                 `json:"enabled" yaml:"enabled"`
	InstalledAt      time.Time            `json:"installed_at,omitempty" yaml:"installed_at,omitempty"`
	UpdatedAt        time.Time            `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	Runtime          RuntimeMetadata      `json:"runtime" yaml:"runtime"`
	Compatibility    CompatibilityReport  `json:"compatibility" yaml:"compatibility"`
	Health           HealthStatus         `json:"health" yaml:"health"`
	History          []LifecycleEvent     `json:"history,omitempty" yaml:"history,omitempty"`
	HealthHistory    []HealthStatus       `json:"health_history,omitempty" yaml:"health_history,omitempty"`
	Revisions        []RevisionSnapshot   `json:"revisions,omitempty" yaml:"revisions,omitempty"`
	SecretRefs       map[string]string    `json:"secret_refs,omitempty" yaml:"secret_refs,omitempty"`
	Templates        []TemplateAssignment `json:"templates,omitempty" yaml:"templates,omitempty"`
	LastFingerprint  string               `json:"-" yaml:"last_fingerprint,omitempty"`
}

type TemplateAssignment struct {
	ID          string            `json:"id,omitempty" yaml:"id,omitempty"`
	Name        string            `json:"name,omitempty" yaml:"name,omitempty"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Values      map[string]string `json:"values,omitempty" yaml:"values,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty" yaml:"created_at,omitempty"`
}

func CompatibilityReportForManifest(entry Manifest) CompatibilityReport {
	reasons := make([]string, 0, 2)
	compatible := true
	if len(entry.Compatibility.TARSMajorVersions) > 0 {
		compatible = false
		for _, version := range entry.Compatibility.TARSMajorVersions {
			if version == CurrentTARSMajorVersion {
				compatible = true
				break
			}
		}
		if !compatible {
			reasons = append(reasons, "connector is not compatible with current TARS major version")
		}
	}
	if entry.Metadata.ID == "" {
		compatible = false
		reasons = append(reasons, "connector metadata.id is required")
	}
	return CompatibilityReport{
		Compatible:       compatible,
		CurrentTARSMajor: CurrentTARSMajorVersion,
		Reasons:          cloneStrings(reasons),
	}
}

func RuntimeMetadataForManifest(entry Manifest) RuntimeMetadata {
	mode := "managed"
	if len(entry.Compatibility.Modes) > 0 {
		mode = entry.Compatibility.Modes[0]
	}
	return RuntimeMetadata{
		Type:     entry.Spec.Type,
		Protocol: entry.Spec.Protocol,
		Vendor:   entry.Metadata.Vendor,
		Mode:     mode,
	}
}

func HealthStatusForManifest(entry Manifest, compatibility CompatibilityReport, checkedAt time.Time) HealthStatus {
	status := "healthy"
	credentialStatus := "configured"
	summary := "connector is enabled and compatible"

	// Check for missing required fields and credentials
	missingRequired := make([]string, 0)
	missingSecrets := make([]string, 0)
	for _, field := range entry.Spec.ConnectionForm {
		if !field.Required {
			continue
		}
		if field.Secret {
			if ref, ok := entry.Config.SecretRefs[field.Key]; !ok || ref == "" {
				missingSecrets = append(missingSecrets, field.Label)
			}
		} else {
			if val, ok := entry.Config.Values[field.Key]; !ok || val == "" {
				missingRequired = append(missingRequired, field.Label)
			}
		}
	}

	switch {
	case !entry.Enabled():
		status = "disabled"
		summary = "connector is disabled"
	case !compatibility.Compatible:
		status = "unhealthy"
		summary = firstNonEmpty(joinNonEmpty(compatibility.Reasons, "; "), "connector compatibility check failed")
	case len(missingRequired) > 0:
		status = "unhealthy"
		summary = "missing required fields: " + joinNonEmpty(missingRequired, ", ")
	case len(missingSecrets) > 0:
		status = "unhealthy"
		credentialStatus = "missing_credentials"
		summary = "missing credentials: " + joinNonEmpty(missingSecrets, ", ")
	}

	return HealthStatus{
		Status:           status,
		CredentialStatus: credentialStatus,
		Summary:          summary,
		CheckedAt:        checkedAt,
	}
}

// NormalizeHealthStatus normalizes a health status string to ensure it is one of the
// valid statuses: healthy, unhealthy, disabled, unknown.
// It maps "error" or empty strings to "unhealthy".
func NormalizeHealthStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "healthy":
		return "healthy"
	case "unhealthy":
		return "unhealthy"
	case "disabled":
		return "disabled"
	case "unknown":
		return "unknown"
	default:
		return "unhealthy"
	}
}

func joinNonEmpty(values []string, sep string) string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value = firstNonEmpty(value); value != "" {
			filtered = append(filtered, value)
		}
	}
	return firstNonEmpty(stringsJoin(filtered, sep))
}

func stringsJoin(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	result := values[0]
	for i := 1; i < len(values); i++ {
		result += sep + values[i]
	}
	return result
}
