package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tars/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New()
	if err != nil {
		slog.Error("failed to bootstrap app", "error", err)
		os.Exit(1)
	}

	go a.StartWorkers(ctx)

	server := &http.Server{
		Addr:              a.Config.Server.Listen,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_ = server.Shutdown(shutdownCtx)
	}()

	a.Logger.Info(
		"starting tars",
		"listen", a.Config.Server.Listen,
		"rollout_mode", a.Config.Features.RolloutMode,
		"diagnosis_enabled", a.Config.Features.DiagnosisEnabled,
		"approval_enabled", a.Config.Features.ApprovalEnabled,
		"execution_enabled", a.Config.Features.ExecutionEnabled,
		"knowledge_ingest_enabled", a.Config.Features.KnowledgeIngestEnabled,
	)

	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		a.Logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
