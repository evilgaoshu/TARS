package connectors

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var ErrConnectorDisabled = errors.New("connector is disabled")
var ErrConnectorIncompatible = errors.New("connector is incompatible with current TARS runtime")
var ErrConnectorRuntimeUnsupported = errors.New("connector runtime is unsupported")
var ErrConnectorInvalidConfig = errors.New("connector config is invalid")

func ValidateRuntimeManifest(entry Manifest, expectedType string, requestedProtocol string, supportedProtocols map[string]struct{}) error {
	if strings.TrimSpace(expectedType) != "" && strings.TrimSpace(entry.Spec.Type) != strings.TrimSpace(expectedType) {
		return fmt.Errorf("%w: expected type %s", ErrConnectorRuntimeUnsupported, strings.TrimSpace(expectedType))
	}
	if !entry.Enabled() {
		return ErrConnectorDisabled
	}
	compatibility := CompatibilityReportForManifest(entry)
	if !compatibility.Compatible {
		return fmt.Errorf("%w: %s", ErrConnectorIncompatible, joinNonEmpty(compatibility.Reasons, "; "))
	}
	runtimeProtocol := strings.TrimSpace(requestedProtocol)
	if runtimeProtocol == "" {
		runtimeProtocol = strings.TrimSpace(entry.Spec.Protocol)
	}
	if len(supportedProtocols) > 0 {
		if _, ok := supportedProtocols[runtimeProtocol]; !ok {
			return fmt.Errorf("%w: protocol %s", ErrConnectorRuntimeUnsupported, runtimeProtocol)
		}
	}
	return nil
}

func ValidateRuntimeConfig(entry Manifest, runtimeProtocol string) error {
	missingFields := make([]string, 0, 2)
	seenMissing := make(map[string]struct{})
	addMissing := func(label string) {
		label = strings.TrimSpace(label)
		if label == "" {
			return
		}
		if _, ok := seenMissing[label]; ok {
			return
		}
		seenMissing[label] = struct{}{}
		missingFields = append(missingFields, label)
	}
	for _, field := range entry.Spec.ConnectionForm {
		if !field.Required || strings.TrimSpace(field.Key) == "" {
			continue
		}
		key := strings.TrimSpace(field.Key)
		if field.Secret {
			if strings.TrimSpace(entry.Config.SecretRefs[key]) == "" && strings.TrimSpace(entry.Config.Values[key]) == "" {
				addMissing(firstNonEmpty(strings.TrimSpace(field.Label), key))
			}
			continue
		}
		if strings.TrimSpace(entry.Config.Values[key]) == "" {
			addMissing(firstNonEmpty(strings.TrimSpace(field.Label), key))
		}
	}

	protocol := strings.TrimSpace(runtimeProtocol)
	if protocol == "" {
		protocol = strings.TrimSpace(entry.Spec.Protocol)
	}
	if !requiresStrictRuntimeConfigValidation(protocol) {
		return nil
	}

	switch protocol {
	case "ssh_native", "ssh_shell":
		if strings.TrimSpace(entry.Config.Values["host"]) == "" {
			addMissing("Host")
		}
		if strings.TrimSpace(entry.Config.Values["credential_id"]) == "" && strings.TrimSpace(entry.Config.SecretRefs["credential_id"]) == "" {
			addMissing("Credential ID")
		}
	case "victoriametrics_http", "victorialogs_http", "prometheus_http", "jumpserver_api":
		baseURL := strings.TrimSpace(entry.Config.Values["base_url"])
		if baseURL == "" {
			addMissing("Base URL")
		} else if err := validateHTTPBaseURL(baseURL); err != nil {
			return fmt.Errorf("%w: %v", ErrConnectorInvalidConfig, err)
		}
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("%w: missing required fields: %s", ErrConnectorInvalidConfig, joinNonEmpty(missingFields, ", "))
	}
	return nil
}

func requiresStrictRuntimeConfigValidation(protocol string) bool {
	switch strings.TrimSpace(protocol) {
	case "ssh_native", "victoriametrics_http", "victorialogs_http":
		return true
	default:
		return false
	}
}

func validateHTTPBaseURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("base_url is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("base_url must start with http:// or https://")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("base_url is invalid: host is required")
	}
	return nil
}

func ResolveRuntimeManifest(manager *Manager, connectorID string, expectedType string, requestedProtocol string, supportedProtocols map[string]struct{}) (Manifest, error) {
	if manager == nil {
		return Manifest{}, ErrConnectorNotFound
	}
	id := strings.TrimSpace(connectorID)
	if id == "" {
		return Manifest{}, ErrConnectorNotFound
	}
	entry, ok := manager.Get(id)
	if !ok {
		resolved, matched := resolveRuntimeManifestAlias(manager, id, expectedType, requestedProtocol, supportedProtocols)
		if !matched {
			return Manifest{}, ErrConnectorNotFound
		}
		entry = resolved
	}
	if err := ValidateRuntimeManifest(entry, expectedType, requestedProtocol, supportedProtocols); err != nil {
		return Manifest{}, err
	}
	return entry, nil
}

func SelectRuntimeManifest(manager *Manager, expectedType string, requestedProtocol string, supportedProtocols map[string]struct{}) (Manifest, bool) {
	return selectRuntimeManifest(manager, expectedType, requestedProtocol, supportedProtocols, false)
}

func SelectHealthyRuntimeManifest(manager *Manager, expectedType string, requestedProtocol string, supportedProtocols map[string]struct{}) (Manifest, bool) {
	return selectRuntimeManifest(manager, expectedType, requestedProtocol, supportedProtocols, true)
}

func selectRuntimeManifest(manager *Manager, expectedType string, requestedProtocol string, supportedProtocols map[string]struct{}, requireHealthy bool) (Manifest, bool) {
	if manager == nil {
		return Manifest{}, false
	}
	snapshot := manager.Snapshot()
	for _, entry := range prioritizedCapabilityEntries(snapshot) {
		if err := ValidateRuntimeManifest(entry, expectedType, requestedProtocol, supportedProtocols); err != nil {
			continue
		}
		if requireHealthy && !strings.EqualFold(strings.TrimSpace(snapshot.Lifecycle[strings.TrimSpace(entry.Metadata.ID)].Health.Status), "healthy") {
			continue
		}
		return entry, true
	}
	return Manifest{}, false
}

func resolveRuntimeManifestAlias(manager *Manager, connectorID string, expectedType string, requestedProtocol string, supportedProtocols map[string]struct{}) (Manifest, bool) {
	if manager == nil {
		return Manifest{}, false
	}
	alias := strings.ToLower(strings.TrimSpace(connectorID))
	if alias == "" {
		return Manifest{}, false
	}
	snapshot := manager.Snapshot()
	matches := make([]Manifest, 0, 2)
	for _, entry := range snapshot.Config.Entries {
		if !runtimeManifestAliasMatches(entry, alias) {
			continue
		}
		if strings.TrimSpace(expectedType) != "" && strings.TrimSpace(entry.Spec.Type) != strings.TrimSpace(expectedType) {
			continue
		}
		if strings.TrimSpace(requestedProtocol) != "" && strings.TrimSpace(entry.Spec.Protocol) != strings.TrimSpace(requestedProtocol) {
			continue
		}
		if len(supportedProtocols) > 0 {
			if _, ok := supportedProtocols[strings.TrimSpace(entry.Spec.Protocol)]; !ok {
				continue
			}
		}
		matches = append(matches, entry)
	}
	if len(matches) != 1 {
		return Manifest{}, false
	}
	return matches[0], true
}

func runtimeManifestAliasMatches(entry Manifest, alias string) bool {
	candidates := []string{
		entry.Metadata.ID,
		entry.Metadata.Name,
		entry.Metadata.DisplayName,
		entry.Metadata.Vendor,
		entry.Spec.Protocol,
		strings.TrimSuffix(entry.Spec.Protocol, "_http"),
	}
	for _, candidate := range candidates {
		if normalizeRuntimeAlias(candidate) == alias {
			return true
		}
	}
	return false
}

func normalizeRuntimeAlias(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "")
	return replacer.Replace(value)
}

func DefaultExecutionMode(protocol string) string {
	switch strings.TrimSpace(protocol) {
	case "jumpserver_api":
		return "jumpserver_job"
	case "ssh_native":
		return "ssh_native"
	case "":
		return "ssh"
	default:
		return strings.TrimSpace(protocol)
	}
}

func ValidateConfigCompatibility(cfg Config) error {
	for _, entry := range cfg.Entries {
		compatibility := CompatibilityReportForManifest(entry)
		if compatibility.Compatible {
			continue
		}
		return fmt.Errorf("connector %s is incompatible: %s", firstNonEmpty(entry.Metadata.ID, entry.Metadata.Name, entry.Metadata.DisplayName), joinNonEmpty(compatibility.Reasons, "; "))
	}
	return nil
}
