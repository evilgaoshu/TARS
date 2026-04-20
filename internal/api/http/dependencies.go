package httpapi

import (
	"log/slog"

	"tars/internal/contracts"
	"tars/internal/foundation/audit"
	"tars/internal/foundation/config"
	"tars/internal/foundation/metrics"
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

type Dependencies struct {
	Config        config.Config
	Logger        *slog.Logger
	Metrics       *metrics.Registry
	Observability *observability.Store
	AlertIngest   contracts.AlertIngestService
	Workflow      contracts.WorkflowService
	Action        contracts.ActionService
	Knowledge     contracts.KnowledgeService
	Channel       contracts.ChannelService
	Inbox         *inbox.Manager
	Trigger       *trigger.Manager
	Audit         audit.Logger
	Authorization *authorization.Manager
	Approval      *approvalrouting.Manager
	Access        *access.Manager
	Prompts       *reasoning.PromptManager
	Desense       *reasoning.DesensitizationManager
	Providers     *reasoning.ProviderManager
	Connectors    *connectors.Manager
	Extensions    *extensions.Manager
	Skills        *skills.Manager
	Automations   *automations.Manager
	AgentRoles    *agentrole.Manager
	Secrets               *secrets.Store
	SSHCredentials        *sshcredentials.Manager
	Org                   *org.Manager
	NotificationTemplates *msgtpl.Manager
	RuntimeConfig         *postgresrepo.RuntimeConfigStore
}
