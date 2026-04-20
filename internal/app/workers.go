package app

import (
	"context"
	"time"

	httpapi "tars/internal/api/http"
	"tars/internal/contracts"
	"tars/internal/events"
	channelmod "tars/internal/modules/channel"
	telegramchannel "tars/internal/modules/channel/telegram"
)

func (a *App) StartWorkers(ctx context.Context) {
	triggerWorker := events.NewTriggerWorker(
		a.Logger,
		a.services.Channel,
		a.services.Trigger,
		a.services.NotificationTemplates,
		a.services.Access,
	)

	dispatcher := events.NewDispatcher(
		a.Logger,
		a.Metrics,
		a.services.Workflow,
		a.services.Reasoning,
		a.services.Action,
		dispatcherKnowledgeService(a.services.Knowledge),
		a.services.Channel,
		a.services.Audit,
		a.services.Connectors,
		a.services.Skills,
		a.services.AgentRoles,
		triggerWorker,
	)
	approvalTimeoutWorker := events.NewApprovalTimeoutWorker(
		a.Logger,
		a.Metrics,
		a.services.Workflow,
		a.services.Channel,
	)
	garbageCollector := events.NewGarbageCollector(
		a.Logger,
		a.Metrics,
		a.services.Workflow,
		a.Config.Output.SpoolDir,
		a.Config.GC.Interval,
		a.Config.GC.ExecutionOutputRetain,
	)

	go dispatcher.Start(ctx)
	go approvalTimeoutWorker.Start(ctx)
	if a.Config.GC.Enabled {
		go garbageCollector.Start(ctx)
	}

	// Start Telegram polling — the channel may be a CompositeService wrapping a telegram.Service.
	var telegramSvc *telegramchannel.Service
	switch typed := a.services.Channel.(type) {
	case *telegramchannel.Service:
		telegramSvc = typed
	case *channelmod.CompositeService:
		telegramSvc = typed.TelegramService()
	}
	if telegramSvc != nil {
		go telegramSvc.StartPolling(ctx, func(ctx context.Context, rawUpdate []byte) error {
			return httpapi.ProcessTelegramUpdatePayload(ctx, a.HTTPDependencies(), rawUpdate)
		})
	}
	if a.services.Automations != nil {
		go a.services.Automations.StartScheduler(ctx)
	}
	if a.Observability != nil {
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = a.Observability.RunRetention(ctx)
				}
			}
		}()
	}

	<-ctx.Done()
	time.Sleep(10 * time.Millisecond)
}

func dispatcherKnowledgeService(service contracts.KnowledgeService) contracts.KnowledgeService {
	if service != nil {
		return service
	}
	return noopKnowledgeService{}
}

type noopKnowledgeService struct{}

func (noopKnowledgeService) Search(context.Context, contracts.KnowledgeQuery) ([]contracts.KnowledgeHit, error) {
	return nil, nil
}

func (noopKnowledgeService) IngestResolvedSession(context.Context, contracts.SessionClosedEvent) (contracts.KnowledgeIngestResult, error) {
	return contracts.KnowledgeIngestResult{}, nil
}

func (noopKnowledgeService) ReindexDocuments(context.Context, string) error {
	return nil
}
