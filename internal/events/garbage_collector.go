package events

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
)

type idempotencyGCStore interface {
	DeleteExpiredIdempotencyKeys(ctx context.Context, now time.Time) (int, error)
}

type executionOutputChunkGCStore interface {
	DeleteExpiredExecutionOutputChunks(ctx context.Context, now time.Time) (int, error)
}

type GarbageCollector struct {
	logger                *slog.Logger
	metrics               *foundationmetrics.Registry
	workflow              contracts.WorkflowService
	outputSpoolDir        string
	interval              time.Duration
	executionOutputRetain time.Duration
}

func NewGarbageCollector(logger *slog.Logger, metrics *foundationmetrics.Registry, workflow contracts.WorkflowService, outputSpoolDir string, interval time.Duration, executionOutputRetain time.Duration) *GarbageCollector {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = time.Hour
	}
	return &GarbageCollector{
		logger:                logger,
		metrics:               metrics,
		workflow:              workflow,
		outputSpoolDir:        outputSpoolDir,
		interval:              interval,
		executionOutputRetain: executionOutputRetain,
	}
}

func (g *GarbageCollector) Start(ctx context.Context) {
	g.logger.Info("garbage collector started")
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	for {
		if err := g.RunOnce(ctx, time.Now().UTC()); err != nil {
			g.logger.Error("garbage collector cycle failed", "error", err)
		}

		select {
		case <-ctx.Done():
			g.logger.Info("garbage collector stopped")
			return
		case <-ticker.C:
		}
	}
}

func (g *GarbageCollector) RunOnce(ctx context.Context, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	if store, ok := g.workflow.(idempotencyGCStore); ok {
		deleted, err := store.DeleteExpiredIdempotencyKeys(ctx, now)
		if err != nil {
			if g.metrics != nil {
				g.metrics.IncGCRun("error")
			}
			return err
		}
		if deleted > 0 {
			g.logger.Info("garbage collector deleted expired idempotency keys", "count", deleted)
			if g.metrics != nil {
				g.metrics.AddGCDeleted("idempotency_keys", deleted)
			}
		}
	}
	if store, ok := g.workflow.(executionOutputChunkGCStore); ok {
		deleted, err := store.DeleteExpiredExecutionOutputChunks(ctx, now)
		if err != nil {
			if g.metrics != nil {
				g.metrics.IncGCRun("error")
			}
			return err
		}
		if deleted > 0 {
			g.logger.Info("garbage collector deleted expired execution output chunks", "count", deleted)
			if g.metrics != nil {
				g.metrics.AddGCDeleted("execution_output_chunks", deleted)
			}
		}
	}

	if deleted, err := g.cleanupExecutionOutput(now); err != nil {
		if g.metrics != nil {
			g.metrics.IncGCRun("error")
		}
		return err
	} else if deleted > 0 {
		g.logger.Info("garbage collector deleted expired execution outputs", "count", deleted)
		if g.metrics != nil {
			g.metrics.AddGCDeleted("execution_output", deleted)
		}
	}
	if g.metrics != nil {
		g.metrics.IncGCRun("success")
	}

	return nil
}

func (g *GarbageCollector) cleanupExecutionOutput(now time.Time) (int, error) {
	if g.executionOutputRetain <= 0 {
		return 0, nil
	}
	if g.outputSpoolDir == "" {
		return 0, nil
	}

	entries, err := os.ReadDir(g.outputSpoolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := now.Add(-g.executionOutputRetain)
	deleted := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(g.outputSpoolDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return deleted, err
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}
