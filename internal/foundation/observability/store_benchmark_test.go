package observability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tars/internal/foundation/config"
)

func BenchmarkAppend(b *testing.B) {
	seedTime := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	store := benchmarkStoreWithSeed(b, func(dataDir string) {
		seedSignalRecords(b, filepath.Join(dataDir, logSignalName, "runtime.jsonl"), benchmarkSignalRecords(seedTime.Add(-45*time.Minute), 50000, SignalKindLog, "scheduler", "info", "baseline runtime log"))
	})
	store.lastGovernance = time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		record := SignalRecord{
			Timestamp: seedTime.Add(time.Duration(i) * time.Millisecond),
			Level:     "error",
			Component: "telegram",
			Message:   fmt.Sprintf("telegram getUpdates failed iteration=%d", i),
			TraceID:   fmt.Sprintf("trace-%d", i),
		}
		if err := store.AppendLog(record); err != nil {
			b.Fatalf("append log: %v", err)
		}
	}
}

func BenchmarkQuery(b *testing.B) {
	seedTime := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	store := benchmarkStoreWithSeed(b, func(dataDir string) {
		seedSignalRecords(b, filepath.Join(dataDir, logSignalName, "runtime-2026-04-10.jsonl"), benchmarkSignalRecords(seedTime.Add(-48*time.Hour), 50000, SignalKindLog, "scheduler", "info", "older runtime log"))
		seedSignalRecords(b, filepath.Join(dataDir, logSignalName, "runtime-2026-04-11.jsonl"), benchmarkMixedLogRecords(seedTime.Add(-24*time.Hour), 50000, 8))
		seedSignalRecords(b, filepath.Join(dataDir, logSignalName, "runtime.jsonl"), benchmarkMixedLogRecords(seedTime.Add(-30*time.Minute), 50000, 8))
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items, err := store.QueryLogs(LogQuery{
			Query:     "getupdates failed",
			Level:     "error",
			Component: "telegram",
			Limit:     50,
		})
		if err != nil {
			b.Fatalf("query logs: %v", err)
		}
		if len(items) == 0 {
			b.Fatal("expected matching benchmark records")
		}
	}
}

func BenchmarkRebuildSummary(b *testing.B) {
	seedTime := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	store := benchmarkStoreWithSeed(b, func(dataDir string) {
		seedSignalRecords(b, filepath.Join(dataDir, logSignalName, "runtime-2026-04-11.jsonl"), benchmarkMixedLogRecords(seedTime.Add(-24*time.Hour), 60000, 12))
		seedSignalRecords(b, filepath.Join(dataDir, logSignalName, "runtime.jsonl"), benchmarkMixedLogRecords(seedTime.Add(-30*time.Minute), 60000, 12))
		seedSignalRecords(b, filepath.Join(dataDir, traceSignalName, "events-2026-04-11.jsonl"), benchmarkEventRecords(seedTime.Add(-24*time.Hour), 30000))
		seedSignalRecords(b, filepath.Join(dataDir, traceSignalName, "events.jsonl"), benchmarkEventRecords(seedTime.Add(-30*time.Minute), 30000))
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.rebuildSummary(); err != nil {
			b.Fatalf("rebuild summary: %v", err)
		}
	}
}

func benchmarkStoreWithSeed(b *testing.B, seed func(dataDir string)) *Store {
	b.Helper()
	dataDir := b.TempDir()
	if seed != nil {
		seed(dataDir)
	}
	store, err := NewStore(benchmarkObservabilityConfig(dataDir))
	if err != nil {
		b.Fatalf("create benchmark store: %v", err)
	}
	return store
}

func benchmarkObservabilityConfig(dataDir string) config.ObservabilityConfig {
	return config.ObservabilityConfig{
		DataDir: dataDir,
		Metrics: config.ObservabilitySignalConfig{Retention: 30 * 24 * time.Hour, MaxSizeBytes: 1 << 30},
		Logs:    config.ObservabilitySignalConfig{Retention: 30 * 24 * time.Hour, MaxSizeBytes: 1 << 30},
		Traces:  config.ObservabilitySignalConfig{Retention: 30 * 24 * time.Hour, MaxSizeBytes: 1 << 30},
	}
}

func seedSignalRecords(b *testing.B, path string, records []SignalRecord) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatalf("mkdir %s: %v", path, err)
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			b.Fatalf("encode record: %v", err)
		}
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		b.Fatalf("write %s: %v", path, err)
	}
}

func benchmarkSignalRecords(start time.Time, count int, kind SignalKind, component string, level string, message string) []SignalRecord {
	records := make([]SignalRecord, 0, count)
	for i := 0; i < count; i++ {
		records = append(records, SignalRecord{
			ID:        fmt.Sprintf("seed-%s-%d", kind, i),
			Kind:      kind,
			Timestamp: start.Add(time.Duration(i) * time.Second),
			Level:     level,
			Component: component,
			Message:   message,
			TraceID:   fmt.Sprintf("trace-%d", i),
		})
	}
	return records
}

func benchmarkMixedLogRecords(start time.Time, count int, matchEvery int) []SignalRecord {
	records := make([]SignalRecord, 0, count)
	for i := 0; i < count; i++ {
		level := "info"
		component := "scheduler"
		message := "dispatcher idle cycle"
		if i%matchEvery == 0 {
			level = "error"
			component = "telegram"
			message = fmt.Sprintf("telegram getUpdates failed request=%d", i)
		}
		records = append(records, SignalRecord{
			ID:        fmt.Sprintf("mixed-log-%d", i),
			Kind:      SignalKindLog,
			Timestamp: start.Add(time.Duration(i) * time.Second),
			Level:     level,
			Component: component,
			Message:   message,
			TraceID:   fmt.Sprintf("trace-%d", i),
		})
	}
	return records
}

func benchmarkEventRecords(start time.Time, count int) []SignalRecord {
	records := make([]SignalRecord, 0, count)
	for i := 0; i < count; i++ {
		records = append(records, SignalRecord{
			ID:          fmt.Sprintf("event-%d", i),
			Kind:        SignalKindEvent,
			Timestamp:   start.Add(time.Duration(i) * time.Second),
			Component:   "http_api",
			Message:     "http request completed",
			Route:       "/api/v1/observability",
			ExecutionID: fmt.Sprintf("execution-%d", i),
			TraceID:     fmt.Sprintf("trace-%d", i),
		})
	}
	return records
}
