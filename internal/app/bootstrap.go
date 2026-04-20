package app

import (
	"log/slog"
	"strings"

	httpapi "tars/internal/api/http"
	"tars/internal/contracts"
	"tars/internal/foundation/audit"
	"tars/internal/foundation/config"
	foundationmetrics "tars/internal/foundation/metrics"
	"tars/internal/foundation/observability"
	"tars/internal/foundation/secrets"
	"tars/internal/modules/access"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/automations"
	"tars/internal/modules/connectors"
	"tars/internal/modules/extensions"
	"tars/internal/modules/inbox"
	"tars/internal/modules/msgtpl"
	"tars/internal/modules/org"
	"tars/internal/modules/reasoning"
	"tars/internal/modules/skills"
	"tars/internal/modules/sshcredentials"
	"tars/internal/modules/trigger"
	postgresrepo "tars/internal/repo/postgres"
)

type Services struct {
	AlertIngest           contracts.AlertIngestService
	Workflow              contracts.WorkflowService
	Reasoning             contracts.ReasoningService
	Action                contracts.ActionService
	Knowledge             contracts.KnowledgeService
	Channel               contracts.ChannelService
	Inbox                 *inbox.Manager
	Trigger               *trigger.Manager
	Audit                 audit.Logger
	Authz                 *authorization.Manager
	Approval              *approvalrouting.Manager
	Access                *access.Manager
	Prompts               *reasoning.PromptManager
	Desense               *reasoning.DesensitizationManager
	Providers             *reasoning.ProviderManager
	Connectors            *connectors.Manager
	Extensions            *extensions.Manager
	Skills                *skills.Manager
	Automations           *automations.Manager
	AgentRoles            *agentrole.Manager
	Secrets               *secrets.Store
	SSHCredentials        *sshcredentials.Manager
	Org                   *org.Manager
	NotificationTemplates *msgtpl.Manager
	RuntimeConfig         *postgresrepo.RuntimeConfigStore
}

type App struct {
	Config        config.Config
	Logger        *slog.Logger
	Metrics       *foundationmetrics.Registry
	Observability *observability.Store
	services      Services
}

func New() (*App, error) {
	return newWithConfig(config.LoadFromEnv())
}

func newWithConfig(cfg config.Config) (*App, error) {
	shared, err := buildSharedBootstrap(cfg)
	if err != nil {
		return nil, err
	}
	core, err := buildPilotCoreServices(shared)
	if err != nil {
		return nil, err
	}
	optional, err := buildOptionalServices(shared, core)
	if err != nil {
		return nil, err
	}

	return &App{
		Config:        cfg,
		Logger:        shared.logger,
		Metrics:       shared.metrics,
		Observability: shared.observability,
		services:      assembleServices(shared, core, optional),
	}, nil
}

func assembleServices(shared *bootstrapShared, core pilotCoreServices, optional optionalServices) Services {
	channelSvc := core.Channel
	if optional.Channel != nil {
		channelSvc = optional.Channel
	}
	return Services{
		AlertIngest:           core.AlertIngest,
		Workflow:              core.Workflow,
		Reasoning:             core.Reasoning,
		Action:                core.Action,
		Knowledge:             optional.Knowledge,
		Channel:               channelSvc,
		Inbox:                 optional.Inbox,
		Trigger:               optional.Trigger,
		Audit:                 shared.auditLogger,
		Authz:                 shared.authorizationManager,
		Approval:              shared.approvalManager,
		Access:                shared.accessManager,
		Prompts:               shared.promptManager,
		Desense:               shared.desensitizationManager,
		Providers:             shared.providerManager,
		Connectors:            shared.connectorManager,
		Extensions:            optional.Extensions,
		Skills:                optional.Skills,
		Automations:           optional.Automations,
		AgentRoles:            optional.AgentRoles,
		Secrets:               shared.secretStore,
		SSHCredentials:        shared.sshCredentialManager,
		Org:                   shared.orgManager,
		NotificationTemplates: optional.NotificationTemplates,
		RuntimeConfig:         optional.RuntimeConfig,
	}
}

func (a *App) HTTPDependencies() httpapi.Dependencies {
	return httpapi.Dependencies{
		Config:                a.Config,
		Logger:                a.Logger,
		Metrics:               a.Metrics,
		Observability:         a.Observability,
		AlertIngest:           a.services.AlertIngest,
		Workflow:              a.services.Workflow,
		Action:                a.services.Action,
		Knowledge:             a.services.Knowledge,
		Channel:               a.services.Channel,
		Inbox:                 a.services.Inbox,
		Trigger:               a.services.Trigger,
		Audit:                 a.services.Audit,
		Authorization:         a.services.Authz,
		Approval:              a.services.Approval,
		Access:                a.services.Access,
		Prompts:               a.services.Prompts,
		Desense:               a.services.Desense,
		Providers:             a.services.Providers,
		Connectors:            a.services.Connectors,
		Extensions:            a.services.Extensions,
		Skills:                a.services.Skills,
		Automations:           a.services.Automations,
		AgentRoles:            a.services.AgentRoles,
		Secrets:               a.services.Secrets,
		SSHCredentials:        a.services.SSHCredentials,
		Org:                   a.services.Org,
		NotificationTemplates: a.services.NotificationTemplates,
		RuntimeConfig:         a.services.RuntimeConfig,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
