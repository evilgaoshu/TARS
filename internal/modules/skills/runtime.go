package skills

import (
	"strconv"
	"strings"

	"tars/internal/contracts"
)

func (m *Manager) Match(input contracts.DiagnosisInput) *contracts.SkillMatch {
	if m == nil {
		return nil
	}
	snapshot := m.Snapshot()
	needleParts := []string{
		strings.ToLower(strings.TrimSpace(stringFromContext(input.Context, "alert_name"))),
		strings.ToLower(strings.TrimSpace(stringFromContext(input.Context, "user_request"))),
		strings.ToLower(strings.TrimSpace(stringFromContext(input.Context, "summary"))),
	}
	for _, entry := range snapshot.Config.Entries {
		state, ok := snapshot.Lifecycle[entry.Metadata.ID]
		if !ok || !state.Enabled || strings.EqualFold(state.Status, "disabled") || strings.EqualFold(state.RuntimeMode, "disabled") {
			continue
		}
		if trigger, matchedBy, ok := matchesManifest(entry, needleParts); ok {
			return &contracts.SkillMatch{
				SkillID:     entry.Metadata.ID,
				DisplayName: firstNonEmpty(entry.Metadata.DisplayName, entry.Metadata.Name, entry.Metadata.ID),
				Summary:     entry.Metadata.Description,
				MatchedBy:   matchedBy,
				Trigger:     trigger,
				ReviewState: state.ReviewState,
				RuntimeMode: state.RuntimeMode,
				Source:      firstNonEmpty(state.Source, entry.Metadata.Source, entry.Marketplace.Source),
				Manifest: map[string]interface{}{
					"skill_id": entry.Metadata.ID,
					"content":  entry.Metadata.Content,
				},
			}
		}
	}
	return nil
}

func (m *Manager) Expand(match contracts.SkillMatch, input contracts.DiagnosisInput, capabilities []contracts.ToolCapabilityDescriptor) []contracts.ToolPlanStep {
	if m == nil {
		return nil
	}
	snapshot := m.Snapshot()
	entry, found := findEntryByID(snapshot.Config.Entries, match.SkillID)
	if !found || len(entry.Spec.Planner.Steps) == 0 {
		return nil
	}
	steps := make([]contracts.ToolPlanStep, 0, len(entry.Spec.Planner.Steps))
	for idx, ps := range entry.Spec.Planner.Steps {
		stepID := ps.ID
		if stepID == "" {
			stepID = fallbackStepID(ps.Tool, idx)
		}
		params := cloneParams(ps.Params)
		injectSkillRuntimeDefaults(params, input)
		connectorID := preferredConnectorForTool(entry, ps.Tool, capabilities)
		onPendingApproval := ""
		if ps.Approval != nil && strings.EqualFold(ps.Approval.Default, "require_approval") {
			onPendingApproval = "stop"
		}
		steps = append(steps, contracts.ToolPlanStep{
			ID:                stepID,
			Tool:              ps.Tool,
			ConnectorID:       connectorID,
			Reason:            ps.Reason,
			Priority:          ps.Priority,
			Input:             params,
			OnPendingApproval: onPendingApproval,
		})
	}
	return steps
}

func (m *Manager) PlannerDescriptors() []contracts.ToolCapabilityDescriptor {
	if m == nil {
		return nil
	}
	snapshot := m.Snapshot()
	items := make([]contracts.ToolCapabilityDescriptor, 0, len(snapshot.Config.Entries))
	for _, entry := range snapshot.Config.Entries {
		state, ok := snapshot.Lifecycle[entry.Metadata.ID]
		if !ok || !state.Enabled || strings.EqualFold(state.RuntimeMode, "disabled") {
			continue
		}
		items = append(items, contracts.ToolCapabilityDescriptor{
			Tool:         "skill.select",
			CapabilityID: entry.Metadata.ID,
			Invocable:    true,
			ReadOnly:     true,
			Source:       "skill",
			Description:  entry.Metadata.Description,
		})
	}
	return items
}

func matchesManifest(entry Manifest, parts []string) (string, string, bool) {
	// Check trigger alerts first — parts[0] is the alert_name.
	if len(entry.Spec.Triggers.Alerts) > 0 && len(parts) > 0 {
		alertName := parts[0]
		for _, trigger := range entry.Spec.Triggers.Alerts {
			if strings.EqualFold(strings.TrimSpace(trigger), alertName) {
				return trigger, "trigger_alert", true
			}
		}
	}
	// Fall back to tag matching.
	for _, tag := range entry.Metadata.Tags {
		needle := strings.ToLower(strings.TrimSpace(tag))
		for _, part := range parts {
			if needle != "" && strings.Contains(part, needle) {
				return tag, "tag", true
			}
		}
	}
	return "", "", false
}

func injectSkillRuntimeDefaults(params map[string]interface{}, input contracts.DiagnosisInput) {
	if params == nil {
		return
	}
	setIfMissing(params, "host", stringFromContext(input.Context, "host"))
	setIfMissing(params, "service", stringFromContext(input.Context, "service"))
	if tool := strings.TrimSpace(interfaceString(params["tool"])); tool != "" {
		params["tool"] = tool
	}
}

func preferredConnectorForTool(entry Manifest, tool string, capabilities []contracts.ToolCapabilityDescriptor) string {
	ordered := preferredConnectors(entry, tool)
	for _, preferred := range ordered {
		for _, capability := range capabilities {
			if capability.Tool == tool && capability.Invocable && capability.ConnectorID == preferred {
				return preferred
			}
		}
	}
	for _, capability := range capabilities {
		if capability.Tool == tool && capability.Invocable {
			return capability.ConnectorID
		}
	}
	return ""
}

func preferredConnectors(entry Manifest, tool string) []string {
	switch tool {
	case "metrics.query_range", "metrics.query_instant":
		return append([]string(nil), entry.Spec.Governance.ConnectorPreference.Metrics...)
	case "execution.run_command":
		return append([]string(nil), entry.Spec.Governance.ConnectorPreference.Execution...)
	case "observability.query":
		return append([]string(nil), entry.Spec.Governance.ConnectorPreference.Observability...)
	case "delivery.query":
		return append([]string(nil), entry.Spec.Governance.ConnectorPreference.Delivery...)
	default:
		return nil
	}
}

func setIfMissing(target map[string]interface{}, key string, value string) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
		return
	}
	if current, ok := target[key]; ok && strings.TrimSpace(interfaceString(current)) != "" {
		return
	}
	target[key] = value
}

func fallbackStepID(tool string, idx int) string {
	base := strings.ReplaceAll(strings.TrimSpace(tool), ".", "_")
	base = strings.ReplaceAll(base, "-", "_")
	if base == "" {
		base = "skill_step"
	}
	return base + "_" + strconv.Itoa(idx)
}

func stringFromContext(ctx map[string]interface{}, key string) string {
	if len(ctx) == 0 {
		return ""
	}
	return interfaceString(ctx[key])
}

func interfaceString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func findEntryByID(entries []Manifest, id string) (Manifest, bool) {
	for _, entry := range entries {
		if entry.Metadata.ID == id {
			return entry, true
		}
	}
	return Manifest{}, false
}

func cloneParams(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return make(map[string]interface{})
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
