package events

import (
	"fmt"
	"sort"
	"strings"

	"tars/internal/modules/connectors"
)

type plannerToolCapability struct {
	Tool            string   `json:"tool,omitempty"`
	ConnectorID     string   `json:"connector_id,omitempty"`
	ConnectorType   string   `json:"connector_type,omitempty"`
	ConnectorVendor string   `json:"connector_vendor,omitempty"`
	Protocol        string   `json:"protocol,omitempty"`
	CapabilityID    string   `json:"capability_id,omitempty"`
	Action          string   `json:"action,omitempty"`
	Scopes          []string `json:"scopes,omitempty"`
	ReadOnly        bool     `json:"read_only"`
	Invocable       bool     `json:"invocable"`
	Source          string   `json:"source,omitempty"`
	Description     string   `json:"description,omitempty"`
}

func plannerToolCapabilities(manager *connectors.Manager) []plannerToolCapability {
	connectorCapabilities := connectors.ToolPlanCapabilities(manager)
	out := make([]plannerToolCapability, 0, len(connectorCapabilities)+1)
	out = append(out, plannerToolCapability{
		Tool:        "knowledge.search",
		ReadOnly:    true,
		Invocable:   true,
		Source:      "builtin",
		Description: "Search TARS knowledge memory, incident summaries, and imported references.",
		Scopes:      []string{"knowledge.read"},
	})
	for _, item := range connectorCapabilities {
		out = append(out, plannerToolCapability{
			Tool:            item.Tool,
			ConnectorID:     item.ConnectorID,
			ConnectorType:   item.ConnectorType,
			ConnectorVendor: item.ConnectorVendor,
			Protocol:        item.Protocol,
			CapabilityID:    item.CapabilityID,
			Action:          item.Action,
			Scopes:          append([]string(nil), item.Scopes...),
			ReadOnly:        item.ReadOnly,
			Invocable:       item.Invocable,
			Source:          item.Source,
			Description:     item.Description,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Invocable != out[j].Invocable {
			return out[i].Invocable
		}
		if out[i].Tool != out[j].Tool {
			return out[i].Tool < out[j].Tool
		}
		if out[i].ConnectorType != out[j].ConnectorType {
			return out[i].ConnectorType < out[j].ConnectorType
		}
		if out[i].ConnectorID != out[j].ConnectorID {
			return out[i].ConnectorID < out[j].ConnectorID
		}
		return out[i].CapabilityID < out[j].CapabilityID
	})
	return out
}

func toolCapabilitySummary(items []plannerToolCapability) string {
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		label := item.Tool
		if strings.TrimSpace(item.ConnectorID) != "" {
			label += fmt.Sprintf(" via %s", item.ConnectorID)
		}
		if strings.TrimSpace(item.CapabilityID) != "" {
			label += fmt.Sprintf(" [%s]", item.CapabilityID)
		}
		if item.Invocable {
			label += " (invocable)"
		} else {
			label += " (catalog-only)"
		}
		if strings.TrimSpace(item.Description) != "" {
			label += ": " + item.Description
		}
		lines = append(lines, "- "+label)
	}
	return strings.Join(lines, "\n")
}
