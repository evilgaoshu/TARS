package events

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tars/internal/contracts"
)

func TestGarbageCollectorDeletesExpiredOutputFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	expired := filepath.Join(dir, "expired.log")
	fresh := filepath.Join(dir, "fresh.log")
	if err := os.WriteFile(expired, []byte("old"), 0o600); err != nil {
		t.Fatalf("write expired file: %v", err)
	}
	if err := os.WriteFile(fresh, []byte("new"), 0o600); err != nil {
		t.Fatalf("write fresh file: %v", err)
	}

	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(expired, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes expired file: %v", err)
	}

	gc := NewGarbageCollector(slog.Default(), nil, nil, dir, time.Hour, 7*24*time.Hour)
	if err := gc.RunOnce(context.Background(), time.Now()); err != nil {
		t.Fatalf("run garbage collector: %v", err)
	}

	if _, err := os.Stat(expired); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected expired file to be deleted, got %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("expected fresh file to remain, got %v", err)
	}
}

func TestGarbageCollectorDeletesExpiredIdempotencyKeys(t *testing.T) {
	t.Parallel()

	store := &captureGCStore{deleted: 3}
	gc := NewGarbageCollector(slog.Default(), nil, store, "", time.Hour, 7*24*time.Hour)
	if err := gc.RunOnce(context.Background(), time.Now()); err != nil {
		t.Fatalf("run garbage collector: %v", err)
	}
	if store.idempotencyCalls != 1 {
		t.Fatalf("expected idempotency gc to run once, got %d", store.idempotencyCalls)
	}
	if store.chunkCalls != 1 {
		t.Fatalf("expected execution output chunk gc to run once, got %d", store.chunkCalls)
	}
}

type captureGCStore struct {
	contracts.WorkflowService
	idempotencyCalls int
	chunkCalls       int
	deleted          int
}

func (c *captureGCStore) DeleteExpiredIdempotencyKeys(_ context.Context, _ time.Time) (int, error) {
	c.idempotencyCalls++
	return c.deleted, nil
}

func (c *captureGCStore) DeleteExpiredExecutionOutputChunks(_ context.Context, _ time.Time) (int, error) {
	c.chunkCalls++
	return c.deleted, nil
}
