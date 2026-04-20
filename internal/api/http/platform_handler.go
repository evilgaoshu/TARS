package httpapi

import (
	"net/http"
	"sort"

	"tars/internal/api/dto"
	"tars/internal/modules/connectors"
	"tars/internal/modules/skills"
)

func platformDiscoveryHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		registeredIDs := make([]string, 0)
		registeredKindsSet := make(map[string]struct{})
		if deps.Connectors != nil {
			snapshot := deps.Connectors.Snapshot()
			for _, entry := range snapshot.Config.Entries {
				if entry.Metadata.ID != "" {
					registeredIDs = append(registeredIDs, entry.Metadata.ID)
				}
				if entry.Spec.Type != "" {
					registeredKindsSet[entry.Spec.Type] = struct{}{}
				}
			}
		}
		sort.Strings(registeredIDs)
		registeredKinds := make([]string, 0, len(registeredKindsSet))
		for kind := range registeredKindsSet {
			registeredKinds = append(registeredKinds, kind)
		}
		sort.Strings(registeredKinds)
		toolPlanCapabilities := toolPlanCapabilityDescriptors(deps.Connectors)
		if deps.Skills != nil {
			toolPlanCapabilities = append(toolPlanCapabilities, skillToolPlanCapabilityDescriptors(deps.Skills)...)
		}

		writeJSON(w, http.StatusOK, dto.PlatformDiscoveryResponse{
			ProductName:               "TARS",
			APIBasePath:               "/api/v1",
			APIVersion:                "v1",
			ManifestVersion:           "tars.connector/v1alpha1",
			SkillManifestVersion:      "tars.skill/v1alpha1",
			MarketplacePackageVersion: "tars.marketplace/v1alpha1",
			IntegrationModes: []string{
				"webhook",
				"ops_api",
				"connector_manifest",
				"mcp",
			},
			ConnectorKinds: []string{
				"metrics",
				"execution",
				"observability",
				"delivery",
				"mcp_tool",
				"skill_source",
			},
			RegisteredConnectorsCount: len(registeredIDs),
			RegisteredConnectorIDs:    registeredIDs,
			RegisteredConnectorKinds:  registeredKinds,
			SupportedProviderProtocols: []string{
				"openai_compatible",
				"anthropic",
				"gemini",
				"openrouter",
				"ollama",
				"lmstudio",
			},
			SupportedProviderVendors: []string{
				"openai",
				"claude",
				"gemini",
				"openrouter",
				"ollama",
				"lmstudio",
			},
			ImportExportFormats: []string{
				"yaml",
				"json",
				"tar.gz",
			},
			Docs: []string{
				"/api/v1/me",
				"/api/v1/auth/providers",
				"/api/v1/users",
				"/api/v1/roles",
				"/api/v1/providers",
				"/api/v1/channels",
				"/api/v1/people",
				"/api/v1/platform/discovery",
				"/api/v1/connectors",
				"/api/v1/connectors/{id}",
				"/api/v1/skills",
				"/api/v1/skills/{id}",
				"/specs/20-component-connectors.md",
				"/specs/20-component-skills.md",
			},
			ToolPlanCapabilities: toolPlanCapabilities,
		})
	}
}

func skillToolPlanCapabilityDescriptors(manager *skills.Manager) []dto.ToolPlanCapabilityDescriptor {
	items := make([]dto.ToolPlanCapabilityDescriptor, 0, 4)
	for _, item := range manager.PlannerDescriptors() {
		items = append(items, dto.ToolPlanCapabilityDescriptor{
			Tool:         item.Tool,
			CapabilityID: item.CapabilityID,
			Source:       item.Source,
			Description:  item.Description,
			Invocable:    item.Invocable,
			ReadOnly:     item.ReadOnly,
		})
	}
	return items
}

func toolPlanCapabilityDescriptors(manager *connectors.Manager) []dto.ToolPlanCapabilityDescriptor {
	items := make([]dto.ToolPlanCapabilityDescriptor, 0, 4)
	items = append(items, dto.ToolPlanCapabilityDescriptor{
		Tool:        "knowledge.search",
		ReadOnly:    true,
		Invocable:   true,
		Source:      "builtin",
		Description: "Search TARS knowledge memory, incident summaries, and imported references.",
		Scopes:      []string{"knowledge.read"},
	})
	for _, item := range connectors.ToolPlanCapabilities(manager) {
		items = append(items, dto.ToolPlanCapabilityDescriptor{
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
	return items
}
