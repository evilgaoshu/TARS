package httpapi

import (
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, deps Dependencies) {
	RegisterConnectorRegistryRoutes(mux, deps)
	RegisterExtensionRoutes(mux, deps)
	RegisterSkillRegistryRoutes(mux, deps)
	registerAutomationRoutes(mux, deps)
	registerAgentRoleRoutes(mux, deps)
	registerAccessRoutes(mux, deps)
	registerOrgRoutes(mux, deps)
	registerPlatformDiscoveryRoute(mux, deps)
	registerPublicCoreRoutes(mux, deps)
	registerOpsCoreRoutes(mux, deps)
	registerNotificationTemplateRoutes(mux, deps)
	registerInboxRoutes(mux, deps)
	registerTriggerRoutes(mux, deps)
	registerChatRoutes(mux, deps)
}

func RegisterConnectorRegistryRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/connectors", instrumentHandler(deps, "/api/v1/connectors", connectorsListHandler(deps)))
	mux.HandleFunc("/api/v1/connectors/", instrumentHandler(deps, "/api/v1/connectors/*", connectorRouterHandler(deps)))
}

func RegisterSkillRegistryRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/skills", instrumentHandler(deps, "/api/v1/skills", skillsListHandler(deps)))
	mux.HandleFunc("/api/v1/skills/", instrumentHandler(deps, "/api/v1/skills/*", skillRouterHandler(deps)))
	mux.HandleFunc("/api/v1/config/skills/import", instrumentHandler(deps, "/api/v1/config/skills/import", skillsImportHandler(deps)))
}

func RegisterExtensionRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/extensions", instrumentHandler(deps, "/api/v1/extensions", extensionsListHandler(deps)))
	mux.HandleFunc("/api/v1/extensions/review", instrumentHandler(deps, "/api/v1/extensions/review", extensionRouterHandler(deps)))
	mux.HandleFunc("/api/v1/extensions/validate", instrumentHandler(deps, "/api/v1/extensions/validate", extensionRouterHandler(deps)))
	mux.HandleFunc("/api/v1/extensions/import", instrumentHandler(deps, "/api/v1/extensions/import", extensionRouterHandler(deps)))
	mux.HandleFunc("/api/v1/extensions/", instrumentHandler(deps, "/api/v1/extensions/*", extensionRouterHandler(deps)))
}

func RegisterPublicRoutes(mux *http.ServeMux, deps Dependencies) {
	RegisterSkillRegistryRoutes(mux, deps)
	registerAutomationRoutes(mux, deps)
	registerAgentRoleRoutes(mux, deps)
	RegisterExtensionRoutes(mux, deps)
	registerAccessRoutes(mux, deps)
	registerOrgRoutes(mux, deps)
	registerPlatformDiscoveryRoute(mux, deps)
	registerPublicCoreRoutes(mux, deps)
	registerNotificationTemplateRoutes(mux, deps)
	registerInboxRoutes(mux, deps)
	registerTriggerRoutes(mux, deps)
	registerChatRoutes(mux, deps)
	mux.HandleFunc("/", instrumentHandler(deps, "/", webUIHandlerWithDir(deps.Config.Web.DistDir)))
}

func RegisterOpsRoutes(mux *http.ServeMux, deps Dependencies) {
	RegisterSkillRegistryRoutes(mux, deps)
	registerAutomationRoutes(mux, deps)
	registerAgentRoleRoutes(mux, deps)
	RegisterExtensionRoutes(mux, deps)
	registerAccessRoutes(mux, deps)
	registerOrgRoutes(mux, deps)
	registerPlatformDiscoveryRoute(mux, deps)
	registerOpsCoreRoutes(mux, deps)
	registerNotificationTemplateRoutes(mux, deps)
	registerInboxRoutes(mux, deps)
	registerTriggerRoutes(mux, deps)
	registerChatRoutes(mux, deps)
}

func registerPlatformDiscoveryRoute(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/platform/discovery", instrumentHandler(deps, "/api/v1/platform/discovery", platformDiscoveryHandler(deps)))
}

func registerAutomationRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/automations", instrumentHandler(deps, "/api/v1/automations", automationsListHandler(deps)))
	mux.HandleFunc("/api/v1/automations/", instrumentHandler(deps, "/api/v1/automations/*", automationRouterHandler(deps)))
}

func registerAgentRoleRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/agent-roles", instrumentHandler(deps, "/api/v1/agent-roles", agentRolesListHandler(deps)))
	mux.HandleFunc("/api/v1/agent-roles/", instrumentHandler(deps, "/api/v1/agent-roles/*", agentRoleRouterHandler(deps)))
}

func registerPublicCoreRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/healthz", instrumentHandler(deps, "/healthz", healthz()))
	mux.HandleFunc("/readyz", instrumentHandler(deps, "/readyz", readyz()))
	mux.HandleFunc("/metrics", instrumentHandler(deps, "/metrics", metricsHandler(deps)))
	mux.HandleFunc("/api/v1/bootstrap/status", instrumentHandler(deps, "/api/v1/bootstrap/status", bootstrapStatusHandler(deps)))
	mux.HandleFunc("/api/v1/webhooks/vmalert", instrumentHandler(deps, "/api/v1/webhooks/vmalert", webhookHandler(deps)))
	mux.HandleFunc("/api/v1/webhooks/vmalert/api/v2/alerts", instrumentHandler(deps, "/api/v1/webhooks/vmalert/api/v2/alerts", webhookHandler(deps)))
	mux.HandleFunc("/api/v1/channels/telegram/webhook", instrumentHandler(deps, "/api/v1/channels/telegram/webhook", telegramHandler(deps)))
}

func registerOpsCoreRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/summary", instrumentHandler(deps, "/api/v1/summary", opsSummaryHandler(deps)))
	mux.HandleFunc("/api/v1/setup/status", instrumentHandler(deps, "/api/v1/setup/status", setupStatusHandler(deps)))
	mux.HandleFunc("/api/v1/setup/wizard", instrumentHandler(deps, "/api/v1/setup/wizard", setupWizardHandler(deps)))
	mux.HandleFunc("/api/v1/setup/wizard/admin", instrumentHandler(deps, "/api/v1/setup/wizard/admin", setupWizardAdminHandler(deps)))
	mux.HandleFunc("/api/v1/setup/wizard/auth", instrumentHandler(deps, "/api/v1/setup/wizard/auth", setupWizardAuthHandler(deps)))
	mux.HandleFunc("/api/v1/setup/wizard/provider/check", instrumentHandler(deps, "/api/v1/setup/wizard/provider/check", setupWizardProviderCheckHandler(deps)))
	mux.HandleFunc("/api/v1/setup/wizard/provider", instrumentHandler(deps, "/api/v1/setup/wizard/provider", setupWizardProviderHandler(deps)))
	mux.HandleFunc("/api/v1/setup/wizard/channel", instrumentHandler(deps, "/api/v1/setup/wizard/channel", setupWizardChannelHandler(deps)))
	mux.HandleFunc("/api/v1/setup/wizard/complete", instrumentHandler(deps, "/api/v1/setup/wizard/complete", setupWizardCompleteHandler(deps)))
	mux.HandleFunc("/api/v1/smoke/alerts", instrumentHandler(deps, "/api/v1/smoke/alerts", smokeAlertHandler(deps)))
	mux.HandleFunc("/api/v1/config/authorization", instrumentHandler(deps, "/api/v1/config/authorization", authorizationConfigHandler(deps)))
	mux.HandleFunc("/api/v1/config/approval-routing", instrumentHandler(deps, "/api/v1/config/approval-routing", approvalRoutingConfigHandler(deps)))
	mux.HandleFunc("/api/v1/config/connectors", instrumentHandler(deps, "/api/v1/config/connectors", connectorsConfigHandler(deps)))
	mux.HandleFunc("/api/v1/config/connectors/import", instrumentHandler(deps, "/api/v1/config/connectors/import", connectorsImportHandler(deps)))
	mux.HandleFunc("/api/v1/config/secrets", instrumentHandler(deps, "/api/v1/config/secrets", secretsHandler(deps)))
	mux.HandleFunc("/api/v1/ssh-credentials", instrumentHandler(deps, "/api/v1/ssh-credentials", sshCredentialsListHandler(deps)))
	mux.HandleFunc("/api/v1/ssh-credentials/", instrumentHandler(deps, "/api/v1/ssh-credentials/*", sshCredentialRouterHandler(deps)))
	mux.HandleFunc("/api/v1/connectors/probe", instrumentHandler(deps, "/api/v1/connectors/probe", connectorProbeHandler(deps)))
	mux.HandleFunc("/api/v1/connectors/templates", instrumentHandler(deps, "/api/v1/connectors/templates", connectorTemplatesHandler(deps)))
	mux.HandleFunc("/api/v1/dashboard/health", instrumentHandler(deps, "/api/v1/dashboard/health", dashboardHealthHandler(deps)))
	mux.HandleFunc("/api/v1/logs", instrumentHandler(deps, "/api/v1/logs", logsHandler(deps)))
	mux.HandleFunc("/api/v1/observability", instrumentHandler(deps, "/api/v1/observability", observabilitySummaryHandler(deps)))
	mux.HandleFunc("/api/v1/config/reasoning-prompts", instrumentHandler(deps, "/api/v1/config/reasoning-prompts", reasoningPromptConfigHandler(deps)))
	mux.HandleFunc("/api/v1/config/desensitization", instrumentHandler(deps, "/api/v1/config/desensitization", desensitizationConfigHandler(deps)))
	mux.HandleFunc("/api/v1/config/providers", instrumentHandler(deps, "/api/v1/config/providers", providersConfigHandler(deps)))
	mux.HandleFunc("/api/v1/config/providers/models", instrumentHandler(deps, "/api/v1/config/providers/models", providersModelsHandler(deps)))
	mux.HandleFunc("/api/v1/config/providers/check", instrumentHandler(deps, "/api/v1/config/providers/check", providersCheckHandler(deps)))
	mux.HandleFunc("/api/v1/audit", instrumentHandler(deps, "/api/v1/audit", auditListHandler(deps)))
	mux.HandleFunc("/api/v1/audit/bulk/export", instrumentHandler(deps, "/api/v1/audit/bulk/export", auditBulkExportHandler(deps)))
	mux.HandleFunc("/api/v1/knowledge", instrumentHandler(deps, "/api/v1/knowledge", knowledgeListHandler(deps)))
	mux.HandleFunc("/api/v1/knowledge/bulk/export", instrumentHandler(deps, "/api/v1/knowledge/bulk/export", knowledgeBulkExportHandler(deps)))
	mux.HandleFunc("/api/v1/sessions", instrumentHandler(deps, "/api/v1/sessions", sessionsHandler(deps)))
	mux.HandleFunc("/api/v1/sessions/bulk/export", instrumentHandler(deps, "/api/v1/sessions/bulk/export", sessionsBulkExportHandler(deps)))
	mux.HandleFunc("/api/v1/sessions/", instrumentHandler(deps, "/api/v1/sessions/*", sessionRouterHandler(deps)))
	mux.HandleFunc("/api/v1/executions", instrumentHandler(deps, "/api/v1/executions", executionsHandler(deps)))
	mux.HandleFunc("/api/v1/executions/bulk/export", instrumentHandler(deps, "/api/v1/executions/bulk/export", executionsBulkExportHandler(deps)))
	mux.HandleFunc("/api/v1/executions/", instrumentHandler(deps, "/api/v1/executions/*", executionRouterHandler(deps)))
	mux.HandleFunc("/api/v1/outbox", instrumentHandler(deps, "/api/v1/outbox", outboxListHandler(deps)))
	mux.HandleFunc("/api/v1/outbox/bulk/replay", instrumentHandler(deps, "/api/v1/outbox/bulk/replay", outboxBulkReplayHandler(deps)))
	mux.HandleFunc("/api/v1/outbox/bulk/delete", instrumentHandler(deps, "/api/v1/outbox/bulk/delete", outboxBulkDeleteHandler(deps)))
	mux.HandleFunc("/api/v1/outbox/", instrumentHandler(deps, "/api/v1/outbox/*", outboxItemRouterHandler(deps)))
	mux.HandleFunc("/api/v1/reindex/documents", instrumentHandler(deps, "/api/v1/reindex/documents", reindexHandler(deps)))
	mux.HandleFunc("/", instrumentHandler(deps, "/", webUIHandlerWithDir(deps.Config.Web.DistDir)))
}
