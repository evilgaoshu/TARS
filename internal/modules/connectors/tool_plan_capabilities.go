package connectors

import (
	"sort"
	"strings"
)

type ToolPlanCapability struct {
	Tool            string   `json:"tool,omitempty" yaml:"tool,omitempty"`
	ConnectorID     string   `json:"connector_id,omitempty" yaml:"connector_id,omitempty"`
	ConnectorType   string   `json:"connector_type,omitempty" yaml:"connector_type,omitempty"`
	ConnectorVendor string   `json:"connector_vendor,omitempty" yaml:"connector_vendor,omitempty"`
	Protocol        string   `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	CapabilityID    string   `json:"capability_id,omitempty" yaml:"capability_id,omitempty"`
	Action          string   `json:"action,omitempty" yaml:"action,omitempty"`
	Scopes          []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	ReadOnly        bool     `json:"read_only" yaml:"read_only"`
	Invocable       bool     `json:"invocable" yaml:"invocable"`
	Source          string   `json:"source,omitempty" yaml:"source,omitempty"`
	Description     string   `json:"description,omitempty" yaml:"description,omitempty"`
}

func ToolPlanCapabilities(manager *Manager) []ToolPlanCapability {
	if manager == nil {
		return nil
	}
	snapshot := manager.Snapshot()
	if len(snapshot.Config.Entries) == 0 {
		return nil
	}

	preference := make(map[string]int, len(snapshot.Config.Entries))
	ranked := make([]Manifest, 0, len(snapshot.Config.Entries))
	for _, entry := range prioritizedCapabilityEntries(snapshot) {
		preference[strings.TrimSpace(entry.Metadata.ID)] = len(preference)
		ranked = append(ranked, entry)
	}

	out := make([]ToolPlanCapability, 0, len(ranked)*2)
	for _, entry := range ranked {
		for _, capability := range entry.Spec.Capabilities {
			for _, item := range mapCapabilityToTools(entry, capability) {
				out = append(out, ToolPlanCapability{
					Tool:            item.tool,
					ConnectorID:     strings.TrimSpace(entry.Metadata.ID),
					ConnectorType:   strings.TrimSpace(entry.Spec.Type),
					ConnectorVendor: strings.TrimSpace(entry.Metadata.Vendor),
					Protocol:        strings.TrimSpace(entry.Spec.Protocol),
					CapabilityID:    strings.TrimSpace(capability.ID),
					Action:          strings.TrimSpace(capability.Action),
					Scopes:          cloneStrings(capability.Scopes),
					ReadOnly:        capability.ReadOnly,
					Invocable:       item.invocable,
					Source:          "connector",
					Description:     strings.TrimSpace(capability.Description),
				})
			}
		}
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
		rankI := preference[strings.TrimSpace(out[i].ConnectorID)]
		rankJ := preference[strings.TrimSpace(out[j].ConnectorID)]
		if rankI != rankJ {
			return rankI < rankJ
		}
		if out[i].ConnectorID != out[j].ConnectorID {
			return out[i].ConnectorID < out[j].ConnectorID
		}
		return out[i].CapabilityID < out[j].CapabilityID
	})
	return out
}

func prioritizedCapabilityEntries(snapshot Snapshot) []Manifest {
	type rankedManifest struct {
		manifest        Manifest
		healthRank      int
		protocolRank    int
		connectorIDRank string
	}

	items := make([]rankedManifest, 0, len(snapshot.Config.Entries))
	for _, entry := range snapshot.Config.Entries {
		if !entry.Enabled() {
			continue
		}
		compatibility := CompatibilityReportForManifest(entry)
		if !compatibility.Compatible {
			continue
		}
		healthRank := capabilityHealthRank(snapshot.Lifecycle[strings.TrimSpace(entry.Metadata.ID)].Health.Status)
		items = append(items, rankedManifest{
			manifest:        entry,
			healthRank:      healthRank,
			protocolRank:    capabilityProtocolRank(entry),
			connectorIDRank: strings.TrimSpace(entry.Metadata.ID),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].healthRank != items[j].healthRank {
			return items[i].healthRank < items[j].healthRank
		}
		if items[i].protocolRank != items[j].protocolRank {
			return items[i].protocolRank < items[j].protocolRank
		}
		return items[i].connectorIDRank < items[j].connectorIDRank
	})
	out := make([]Manifest, 0, len(items))
	for _, item := range items {
		out = append(out, item.manifest)
	}
	return out
}

func capabilityHealthRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "healthy":
		return 0
	case "unknown", "":
		return 1
	case "degraded":
		return 2
	case "disabled":
		return 3
	default:
		return 4
	}
}

func capabilityProtocolRank(entry Manifest) int {
	switch strings.ToLower(strings.TrimSpace(entry.Spec.Protocol)) {
	// First-class protocols: metrics, logs, execution, observability, delivery
	case "victoriametrics_http", "victorialogs_http", "prometheus_http", "jumpserver_api", "ssh_native":
		return 0
	case "observability_http", "log_file", "delivery_git", "delivery_github":
		return 0
	case "stub":
		return 2
	default:
		return 1
	}
}

type mappedToolCapability struct {
	tool      string
	invocable bool
}

func mapCapabilityToTools(entry Manifest, capability Capability) []mappedToolCapability {
	connectorType := NormalizeCapabilityConnectorType(entry.Spec.Type)
	capabilityID := strings.ToLower(strings.TrimSpace(capability.ID))
	action := strings.ToLower(strings.TrimSpace(capability.Action))
	switch connectorType {
	case "logs":
		// VictoriaLogs first-class: maps to logs.query
		switch capabilityID {
		case "logs.query", "victorialogs.query", "log.query", "query":
			return []mappedToolCapability{{tool: "logs.query", invocable: true}}
		}
		if action == "query" {
			return []mappedToolCapability{{tool: "logs.query", invocable: true}}
		}
		return []mappedToolCapability{{tool: "logs.query", invocable: true}}
	case "metrics":
		switch capabilityID {
		case "query.range":
			return []mappedToolCapability{{tool: "metrics.query_range", invocable: true}}
		case "query.instant":
			return []mappedToolCapability{{tool: "metrics.query_instant", invocable: true}}
		case "metrics.query":
			return []mappedToolCapability{
				{tool: "metrics.query_instant", invocable: true},
				{tool: "metrics.query_range", invocable: true},
			}
		}
		if action == "query" {
			return []mappedToolCapability{
				{tool: "metrics.query_instant", invocable: true},
				{tool: "metrics.query_range", invocable: true},
			}
		}
	case "execution":
		if capabilityID == "command.execute" {
			return []mappedToolCapability{{tool: "execution.run_command", invocable: true}}
		}
	case "observability":
		switch capabilityID {
		case "query", "observability.query", "log.query", "trace.query":
			return []mappedToolCapability{{tool: "observability.query", invocable: true}}
		}
		if action == "query" {
			return []mappedToolCapability{{tool: "observability.query", invocable: true}}
		}
		return []mappedToolCapability{{tool: "observability.query", invocable: true}}
	case "delivery":
		switch capabilityID {
		case "query", "delivery.query", "status.query":
			return []mappedToolCapability{{tool: "delivery.query", invocable: true}}
		}
		if action == "query" {
			return []mappedToolCapability{{tool: "delivery.query", invocable: true}}
		}
		return []mappedToolCapability{{tool: "delivery.query", invocable: true}}
	case "mcp":
		return []mappedToolCapability{{tool: "connector.invoke_capability", invocable: true}}
	case "skill":
		return []mappedToolCapability{{tool: "connector.invoke_capability", invocable: true}}
	}
	return []mappedToolCapability{{tool: "connector.invoke_capability", invocable: false}}
}

// NormalizeCapabilityConnectorType folds connector type aliases into the
// canonical runtime categories used by tool-plan capability mapping and
// authorization.
func NormalizeCapabilityConnectorType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mcp", "mcp_tool":
		return "mcp"
	case "skill", "skill_source":
		return "skill"
	// VictoriaLogs canonical type is "logs"
	case "logs", "victorialogs":
		return "logs"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}
