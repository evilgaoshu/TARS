package app

import (
	"context"
	"fmt"
	"strings"

	"tars/internal/contracts"
	"tars/internal/events"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/automations"
	"tars/internal/modules/channel"
	"tars/internal/modules/extensions"
	"tars/internal/modules/inbox"
	"tars/internal/modules/knowledge"
	"tars/internal/modules/msgtpl"
	"tars/internal/modules/skills"
	"tars/internal/modules/trigger"
	postgresrepo "tars/internal/repo/postgres"
)

type optionalServices struct {
	Channel               contracts.ChannelService
	Knowledge             contracts.KnowledgeService
	Inbox                 *inbox.Manager
	Trigger               *trigger.Manager
	NotificationTemplates *msgtpl.Manager
	Skills                *skills.Manager
	Extensions            *extensions.Manager
	Automations           *automations.Manager
	AgentRoles            *agentrole.Manager
	RuntimeConfig         *postgresrepo.RuntimeConfigStore
}

func buildOptionalServices(shared *bootstrapShared, core pilotCoreServices) (optionalServices, error) {
	inboxManager := inbox.NewManager(shared.db)
	triggerManager := trigger.NewManager(shared.db)
	notificationTemplateManager := msgtpl.NewManager(shared.db)
	compositeChannel := channel.NewCompositeService(core.Telegram, inboxManager)
	knowledgeSvc := knowledge.NewServiceWithOptions(knowledge.Options{
		DB:       shared.db,
		Vector:   shared.vectorStore,
		Workflow: core.Workflow,
		Metrics:  shared.metrics,
	})
	triggerWorker := events.NewTriggerWorker(
		shared.logger,
		compositeChannel,
		triggerManager,
		notificationTemplateManager,
		shared.accessManager,
	)

	automationManager, err := automations.NewManager(shared.cfg.Automations.ConfigPath, automations.Options{
		Logger:     shared.logger,
		Audit:      shared.auditLogger,
		Action:     core.Action,
		Knowledge:  knowledgeSvc,
		Reasoning:  core.Reasoning,
		Connectors: shared.connectorManager,
		Skills:     shared.skillManager,
		AgentRoles: shared.agentRoleManager,
		RunNotifier: func(ctx context.Context, job automations.Job, run automations.Run) {
			eventType := trigger.EventOnExecutionCompleted
			subject := "自动化巡检完成"
			statusLabel := "完成"
			if run.Status == "failed" {
				eventType = trigger.EventOnExecutionFailed
				subject = "自动化巡检失败"
				statusLabel = "失败"
			}
			host := automationTargetHost(job, run)
			outputPreview := firstNonEmpty(strings.TrimSpace(run.Summary), strings.TrimSpace(run.Error), fmt.Sprintf("job %s %s", job.ID, run.Status))
			triggerWorker.FireEvent(ctx, trigger.FireEvent{
				EventType: eventType,
				TenantID:  "default",
				RefType:   "automation_run",
				RefID:     run.RunID,
				Subject:   subject,
				Body:      outputPreview,
				Source:    "automation_scheduler",
				TemplateData: map[string]string{
					"ExecutionID":     run.RunID,
					"TargetHost":      host,
					"ExitCode":        automationExitCode(run.Status),
					"ExecutionStatus": statusLabel,
					"OutputPreview":   outputPreview,
					"TruncationFlag":  "",
					"ActionTip":       automationActionTip(job, run),
					"SessionID":       job.ID,
				},
			})
		},
	})
	if err != nil {
		return optionalServices{}, err
	}

	return optionalServices{
		Channel:               compositeChannel,
		Knowledge:             knowledgeSvc,
		Inbox:                 inboxManager,
		Trigger:               triggerManager,
		NotificationTemplates: notificationTemplateManager,
		Skills:                shared.skillManager,
		Extensions:            shared.extensionManager,
		Automations:           automationManager,
		AgentRoles:            shared.agentRoleManager,
		RuntimeConfig:         shared.runtimeConfigStore,
	}, nil
}

func automationTargetHost(job automations.Job, run automations.Run) string {
	if host, ok := run.Metadata["host"].(string); ok && strings.TrimSpace(host) != "" {
		return strings.TrimSpace(host)
	}
	if target := job.Skill; target != nil {
		if host, ok := target.Context["host"].(string); ok && strings.TrimSpace(host) != "" {
			return strings.TrimSpace(host)
		}
	}
	if target := job.ConnectorCapability; target != nil {
		if host, ok := target.Params["host"].(string); ok && strings.TrimSpace(host) != "" {
			return strings.TrimSpace(host)
		}
	}
	if strings.TrimSpace(job.TargetRef) != "" {
		return strings.TrimSpace(job.TargetRef)
	}
	return "automation"
}

func automationExitCode(status string) string {
	if status == "failed" {
		return "1"
	}
	return "0"
}

func automationActionTip(job automations.Job, run automations.Run) string {
	parts := []string{}
	if strings.TrimSpace(job.DisplayName) != "" {
		parts = append(parts, fmt.Sprintf("任务：%s", strings.TrimSpace(job.DisplayName)))
	}
	if strings.TrimSpace(run.Summary) != "" && strings.TrimSpace(run.Error) != strings.TrimSpace(run.Summary) {
		parts = append(parts, fmt.Sprintf("摘要：%s", strings.TrimSpace(run.Summary)))
	}
	if strings.TrimSpace(run.Error) != "" {
		parts = append(parts, fmt.Sprintf("错误：%s", strings.TrimSpace(run.Error)))
	}
	return strings.Join(parts, "\n")
}
