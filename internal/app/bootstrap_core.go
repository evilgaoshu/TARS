package app

import (
	"net/http"
	"time"

	"tars/internal/contracts"
	"tars/internal/modules/action"
	actionprovider "tars/internal/modules/action/provider"
	actionssh "tars/internal/modules/action/ssh"
	"tars/internal/modules/alertintake"
	"tars/internal/modules/channel/telegram"
	"tars/internal/modules/reasoning"
	"tars/internal/modules/workflow"
	postgresrepo "tars/internal/repo/postgres"
)

type pilotCoreServices struct {
	AlertIngest contracts.AlertIngestService
	Workflow    contracts.WorkflowService
	Reasoning   contracts.ReasoningService
	Action      contracts.ActionService
	Channel     contracts.ChannelService
	Telegram    *telegram.Service
}

func buildPilotCoreServices(shared *bootstrapShared) (pilotCoreServices, error) {
	workflowSvc := contracts.WorkflowService(workflow.NewService(workflow.Options{
		DiagnosisEnabled:        shared.cfg.Features.DiagnosisEnabled,
		ApprovalEnabled:         shared.cfg.Features.ApprovalEnabled,
		ExecutionEnabled:        shared.cfg.Features.ExecutionEnabled,
		KnowledgeIngestEnabled:  shared.cfg.Features.KnowledgeIngestEnabled,
		ApprovalTimeout:         shared.cfg.Approval.Timeout,
		ApprovalRouter:          shared.approvalManager,
		AuthorizationPolicy:     shared.authorizationManager,
		AgentRoleManager:        shared.agentRoleManager,
		DesensitizationProvider: shared.desensitizationManager,
		Connectors:              shared.connectorManager,
	}))
	if shared.db != nil {
		workflowSvc = postgresrepo.NewStore(shared.db, postgresrepo.Options{
			Logger:                  shared.logger,
			DiagnosisEnabled:        shared.cfg.Features.DiagnosisEnabled,
			ApprovalEnabled:         shared.cfg.Features.ApprovalEnabled,
			ExecutionEnabled:        shared.cfg.Features.ExecutionEnabled,
			KnowledgeIngestEnabled:  shared.cfg.Features.KnowledgeIngestEnabled,
			ApprovalTimeout:         shared.cfg.Approval.Timeout,
			ApprovalRouter:          shared.approvalManager,
			AuthorizationPolicy:     shared.authorizationManager,
			AgentRoleManager:        shared.agentRoleManager,
			DesensitizationProvider: shared.desensitizationManager,
			Connectors:              shared.connectorManager,
			OutputChunkBytes:        shared.cfg.Output.ChunkBytes,
			OutputRetention:         shared.cfg.Output.Retention,
		})
	}

	telegramSvc := telegram.NewService(shared.logger, telegram.Config{
		BotToken:       shared.cfg.Telegram.BotToken,
		BaseURL:        shared.cfg.Telegram.BaseURL,
		PollingEnabled: shared.cfg.Telegram.PollingEnabled,
		PollTimeout:    shared.cfg.Telegram.PollTimeout,
		PollInterval:   shared.cfg.Telegram.PollInterval,
		Metrics:        shared.metrics,
	})
	sshExecutor := actionssh.NewExecutor(actionssh.Config{
		User:                   shared.cfg.SSH.User,
		PrivateKeyPath:         shared.cfg.SSH.PrivateKeyPath,
		ConnectTimeout:         shared.cfg.SSH.ConnectTimeout,
		CommandTimeout:         shared.cfg.SSH.CommandTimeout,
		DisableHostKeyChecking: shared.cfg.SSH.DisableHostKeyChecking,
	})

	return pilotCoreServices{
		AlertIngest: alertintake.NewService(),
		Workflow:    workflowSvc,
		Reasoning: reasoning.NewService(reasoning.Options{
			Logger:                     shared.logger,
			Metrics:                    shared.metrics,
			Audit:                      shared.auditLogger,
			Protocol:                   shared.cfg.Model.Protocol,
			BaseURL:                    shared.cfg.Model.BaseURL,
			APIKey:                     shared.cfg.Model.APIKey,
			Model:                      shared.cfg.Model.Model,
			Client:                     &http.Client{Timeout: shared.cfg.Model.Timeout},
			PromptProvider:             shared.promptManager,
			DesensitizationProvider:    shared.desensitizationManager,
			ProviderRegistry:           shared.providerManager,
			SecretStore:                shared.secretStore,
			LocalCommandFallbackEnable: shared.cfg.Reasoning.LocalCommandFallbackEnable,
		}),
		Action: action.NewService(action.Options{
			Logger:   shared.logger,
			Metrics:  shared.metrics,
			Executor: sshExecutor,
			MetricsProvider: actionprovider.NewVictoriaMetricsProvider(actionprovider.VictoriaMetricsConfig{
				BaseURL: shared.cfg.VM.BaseURL,
				Client:  &http.Client{Timeout: shared.cfg.VM.Timeout},
				Metrics: shared.metrics,
			}),
			AllowedHosts:            shared.cfg.SSH.AllowedHosts,
			AllowedCommandPrefixes:  shared.cfg.SSH.AllowedCommandPrefixes,
			ApprovalRouter:          shared.approvalManager,
			BlockedCommandFragments: shared.cfg.SSH.BlockedCommandFragments,
			AuthorizationPolicy:     shared.authorizationManager,
			Connectors:              shared.connectorManager,
			Secrets:                 shared.secretStore,
			SSHCredentials:          shared.sshCredentialManager,
			QueryRuntimes: map[string]action.QueryRuntime{
				"prometheus_http":      actionprovider.NewMetricsConnectorRuntime(actionprovider.VictoriaMetricsConfig{Metrics: shared.metrics}),
				"victoriametrics_http": actionprovider.NewMetricsConnectorRuntime(actionprovider.VictoriaMetricsConfig{Metrics: shared.metrics}),
			},
			ExecutionRuntimes: map[string]action.ExecutionRuntime{
				"jumpserver_api": actionprovider.NewJumpServerRuntime(&http.Client{Timeout: 15 * time.Second}),
				"ssh_native":     action.NewSSHNativeRuntime(sshExecutor, shared.sshCredentialManager),
			},
			CapabilityRuntimes: map[string]action.CapabilityRuntime{
				"observability":        actionprovider.NewObservabilityHTTPRuntime(&http.Client{Timeout: 15 * time.Second}),
				"delivery":             actionprovider.NewDeliveryRuntime(&http.Client{Timeout: 15 * time.Second}),
				"mcp":                  actionprovider.NewMCPStubRuntime(),
				"skill":                actionprovider.NewSkillStubRuntime(),
				"victoriametrics_http": actionprovider.NewMetricsCapabilityRuntime(actionprovider.VictoriaMetricsConfig{Metrics: shared.metrics}),
				"victorialogs_http":    actionprovider.NewVictoriaLogsRuntime(&http.Client{Timeout: 15 * time.Second}),
				"prometheus_http":      actionprovider.NewMetricsCapabilityRuntime(actionprovider.VictoriaMetricsConfig{Metrics: shared.metrics}),
			},
			OutputSpoolDir:          shared.cfg.Output.SpoolDir,
			MaxPersistedOutputBytes: shared.cfg.Output.MaxPersistedBytes,
		}),
		Channel:  telegramSvc,
		Telegram: telegramSvc,
	}, nil
}
