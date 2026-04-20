package events

import (
	"context"
	"log/slog"
	"time"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
)

type ApprovalTimeoutWorker struct {
	logger   *slog.Logger
	metrics  *foundationmetrics.Registry
	workflow contracts.WorkflowService
	channel  contracts.ChannelService
	interval time.Duration
}

func NewApprovalTimeoutWorker(logger *slog.Logger, metrics *foundationmetrics.Registry, workflow contracts.WorkflowService, channel contracts.ChannelService) *ApprovalTimeoutWorker {
	return &ApprovalTimeoutWorker{
		logger:   logger,
		metrics:  metrics,
		workflow: workflow,
		channel:  channel,
		interval: 5 * time.Second,
	}
}

func (w *ApprovalTimeoutWorker) Start(ctx context.Context) {
	w.logger.Info("approval timeout worker started")
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		if err := w.RunOnce(ctx, time.Now().UTC()); err != nil {
			w.logger.Error("approval timeout cycle failed", "error", err)
		}

		select {
		case <-ctx.Done():
			w.logger.Info("approval timeout worker stopped")
			return
		case <-ticker.C:
		}
	}
}

func (w *ApprovalTimeoutWorker) RunOnce(ctx context.Context, now time.Time) error {
	notifications, err := w.workflow.SweepApprovalTimeouts(ctx, now)
	if err != nil {
		return err
	}
	if w.metrics != nil && len(notifications) > 0 {
		w.metrics.AddApprovalTimeouts(len(notifications))
	}

	for _, msg := range notifications {
		if _, err := w.channel.SendMessage(ctx, msg); err != nil {
			return err
		}
	}

	return nil
}
