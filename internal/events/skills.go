package events

import (
	"context"
	"strings"

	"tars/internal/contracts"
	"tars/internal/foundation/audit"
	"tars/internal/modules/skills"
)

func plannerSkillCapabilities(manager *skills.Manager) []plannerToolCapability {
	if manager == nil {
		return nil
	}
	items := manager.PlannerDescriptors()
	out := make([]plannerToolCapability, 0, len(items))
	for _, item := range items {
		out = append(out, plannerToolCapability{Tool: item.Tool, CapabilityID: item.CapabilityID, Invocable: item.Invocable, ReadOnly: item.ReadOnly, Source: item.Source, Description: item.Description})
	}
	return out
}

func activeSkillSummaries(manager *skills.Manager) []map[string]interface{} {
	if manager == nil {
		return nil
	}
	snapshot := manager.Snapshot()
	items := make([]map[string]interface{}, 0, len(snapshot.Config.Entries))
	for _, entry := range snapshot.Config.Entries {
		state, ok := snapshot.Lifecycle[entry.Metadata.ID]
		if !ok || !state.Enabled || strings.EqualFold(state.RuntimeMode, "disabled") {
			continue
		}
		items = append(items, map[string]interface{}{
			"skill_id":     entry.Metadata.ID,
			"display_name": entry.Metadata.DisplayName,
			"summary":      entry.Metadata.Description,
			"content":      entry.Metadata.Content,
			"tags":         entry.Metadata.Tags,
		})
	}
	return items
}

func toToolCapabilityDescriptors(items []plannerToolCapability) []contracts.ToolCapabilityDescriptor {
	if len(items) == 0 {
		return nil
	}
	out := make([]contracts.ToolCapabilityDescriptor, 0, len(items))
	for _, item := range items {
		out = append(out, contracts.ToolCapabilityDescriptor{
			Tool:            item.Tool,
			ConnectorID:     item.ConnectorID,
			ConnectorType:   item.ConnectorType,
			ConnectorVendor: item.ConnectorVendor,
			Protocol:        item.Protocol,
			CapabilityID:    item.CapabilityID,
			Action:          item.Action,
			ReadOnly:        item.ReadOnly,
			Invocable:       item.Invocable,
			Source:          item.Source,
			Description:     item.Description,
		})
	}
	return out
}

func (d *Dispatcher) auditSkillSelected(ctx context.Context, sessionID string, match contracts.SkillMatch) {
	if d == nil || d.audit == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	d.audit.Log(ctx, audit.Entry{
		ResourceType: "skill",
		ResourceID:   match.SkillID,
		Action:       "skill_selected_by_planner",
		Actor:        "tars_dispatcher",
		Metadata: map[string]any{
			"session_id":   sessionID,
			"skill_id":     match.SkillID,
			"matched_by":   match.MatchedBy,
			"trigger":      match.Trigger,
			"runtime_mode": match.RuntimeMode,
		},
	})
}

func (d *Dispatcher) auditSkillExpanded(ctx context.Context, sessionID string, match contracts.SkillMatch, steps []contracts.ToolPlanStep) {
	if d == nil || d.audit == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	items := make([]map[string]any, 0, len(steps))
	for _, step := range steps {
		items = append(items, map[string]any{
			"id":           step.ID,
			"tool":         step.Tool,
			"connector_id": step.ConnectorID,
			"reason":       step.Reason,
		})
	}
	d.audit.Log(ctx, audit.Entry{
		ResourceType: "skill",
		ResourceID:   match.SkillID,
		Action:       "skill_expanded_to_tool_plan",
		Actor:        "tars_dispatcher",
		Metadata: map[string]any{
			"session_id": sessionID,
			"skill_id":   match.SkillID,
			"step_count": len(steps),
			"steps":      items,
		},
	})
}

func matchSummaryFromContext(ctx map[string]interface{}) string {
	if len(ctx) == 0 {
		return ""
	}
	if summary, ok := ctx["summary"].(string); ok {
		return strings.TrimSpace(summary)
	}
	if skillMatch, ok := ctx["skill_match"].(map[string]interface{}); ok {
		if summary, ok := skillMatch["summary"].(string); ok {
			return strings.TrimSpace(summary)
		}
	}
	return ""
}
