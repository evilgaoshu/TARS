package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"tars/internal/foundation/config"
	foundationmetrics "tars/internal/foundation/metrics"
)

func TestStoreAppendUpdatesSummaryIncrementally(t *testing.T) {
	t.Parallel()

	store := newTestStore(t, nil)
	base := time.Now().UTC().Add(-30 * time.Minute)

	if err := store.AppendLog(SignalRecord{
		Timestamp: base,
		Level:     "error",
		Component: "telegram",
		Message:   "telegram getUpdates failed",
		TraceID:   "trace-log",
	}); err != nil {
		t.Fatalf("append log: %v", err)
	}
	if err := store.AppendEvent(SignalRecord{
		Timestamp:   base.Add(30 * time.Second),
		Component:   "http_api",
		Message:     "http request completed",
		SessionID:   "session-1",
		ExecutionID: "execution-1",
		TraceID:     "trace-event",
	}); err != nil {
		t.Fatalf("append event: %v", err)
	}

	summary := store.Summary()
	if summary.LogCount24h != 1 {
		t.Fatalf("expected log count 1, got %d", summary.LogCount24h)
	}
	if summary.ErrorCount24h != 1 {
		t.Fatalf("expected error count 1, got %d", summary.ErrorCount24h)
	}
	if summary.EventCount24h != 1 {
		t.Fatalf("expected event count 1, got %d", summary.EventCount24h)
	}
	if summary.TraceCount24h != 1 {
		t.Fatalf("expected trace count 1, got %d", summary.TraceCount24h)
	}
	if summary.LastLogAt != base {
		t.Fatalf("expected last log at %s, got %s", base, summary.LastLogAt)
	}
	if summary.LastEventAt != base.Add(30*time.Second) {
		t.Fatalf("expected last event at %s, got %s", base.Add(30*time.Second), summary.LastEventAt)
	}
	if summary.LogStorageBytes <= 0 {
		t.Fatalf("expected log storage bytes to increase, got %d", summary.LogStorageBytes)
	}
	if summary.TraceStorageBytes <= 0 {
		t.Fatalf("expected trace storage bytes to increase, got %d", summary.TraceStorageBytes)
	}
}

func TestStoreRunRetentionRebuildsSummaryForLowTraffic(t *testing.T) {
	t.Parallel()

	base := time.Now().UTC()
	store := newTestStore(t, func(cfg *config.ObservabilityConfig) {
		cfg.Logs.Retention = 2 * time.Hour
		cfg.Traces.Retention = 2 * time.Hour
	})

	if err := store.AppendLog(SignalRecord{
		Timestamp: base.Add(-90 * time.Minute),
		Level:     "info",
		Component: "runtime",
		Message:   "fresh enough before time passes",
	}); err != nil {
		t.Fatalf("append log: %v", err)
	}

	store.mu.Lock()
	store.summary = Summary{
		LogCount24h:   99,
		ErrorCount24h: 77,
		EventCount24h: 55,
		TraceCount24h: 44,
	}
	store.mu.Unlock()

	if err := store.RunRetention(context.Background()); err != nil {
		t.Fatalf("run retention: %v", err)
	}

	summary := store.Summary()
	if summary.LogCount24h != 1 {
		t.Fatalf("expected rebuilt log count 1, got %d", summary.LogCount24h)
	}
	if summary.ErrorCount24h != 0 {
		t.Fatalf("expected rebuilt error count 0, got %d", summary.ErrorCount24h)
	}
	if summary.EventCount24h != 0 || summary.TraceCount24h != 0 {
		t.Fatalf("expected rebuilt event/trace counts 0, got events=%d traces=%d", summary.EventCount24h, summary.TraceCount24h)
	}
	if summary.MetricsStorageBytes <= 0 {
		t.Fatalf("expected metrics snapshot bytes after retention, got %d", summary.MetricsStorageBytes)
	}
}

func TestStoreAppendTriggersPeriodicGovernanceCalibration(t *testing.T) {
	t.Parallel()

	base := time.Now().UTC()
	store := newSeededStore(t, func(cfg *config.ObservabilityConfig) {
		cfg.Logs.Retention = 2 * time.Hour
	}, func(dataDir string) {
		seedRecordsForTest(t, filepath.Join(dataDir, logSignalName, "runtime.jsonl"), []SignalRecord{
			newLogRecord("stale-log", base.Add(-48*time.Hour), "info", "scheduler", "stale log", "trace-stale"),
		})
	})

	store.mu.Lock()
	store.summary = Summary{LogCount24h: 99, ErrorCount24h: 77}
	store.lastGovernance = time.Now().Add(-governanceInterval - time.Second)
	store.mu.Unlock()

	if err := store.AppendLog(newLogRecord("fresh-log", base, "error", "telegram", "fresh log", "trace-fresh")); err != nil {
		t.Fatalf("append log: %v", err)
	}

	items, err := store.QueryLogs(LogQuery{Limit: 10})
	if err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if got := signalIDs(items); len(got) != 1 || got[0] != "fresh-log" {
		t.Fatalf("unexpected remaining log ids after append governance: %#v", got)
	}

	summary := store.Summary()
	if summary.LogCount24h != 1 {
		t.Fatalf("expected rebuilt log count 1, got %d", summary.LogCount24h)
	}
	if summary.ErrorCount24h != 1 {
		t.Fatalf("expected rebuilt error count 1, got %d", summary.ErrorCount24h)
	}
}

func TestStoreQueryLogsKeepsNewestTimestampOrderWithinFile(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	store := newSeededStore(t, nil, func(dataDir string) {
		seedRecordsForTest(t, filepath.Join(dataDir, logSignalName, "runtime.jsonl"), []SignalRecord{
			newLogRecord("ts-2h", base.Add(2*time.Hour), "error", "telegram", "2h", "trace-2h"),
			newLogRecord("ts-4h", base.Add(4*time.Hour), "error", "telegram", "4h", "trace-4h"),
			newLogRecord("ts-3h", base.Add(3*time.Hour), "error", "telegram", "3h", "trace-3h"),
		})
	})

	items, err := store.QueryLogs(LogQuery{
		Level:     "error",
		Component: "telegram",
		Limit:     2,
	})
	if err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if got := signalIDs(items); len(got) != 2 || got[0] != "ts-4h" || got[1] != "ts-3h" {
		t.Fatalf("unexpected query order for out-of-order timestamps: %#v", got)
	}
}

func TestStoreQueryLogsReturnsNewestMatchesAcrossRotatedFiles(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	store := newSeededStore(t, nil, func(dataDir string) {
		seedRecordsForTest(t, filepath.Join(dataDir, logSignalName, "runtime-2026-04-10.jsonl"), []SignalRecord{
			newLogRecord("rot-old-1", base.Add(-48*time.Hour), "error", "telegram", "telegram getUpdates failed old-1", "trace-old-1"),
			newLogRecord("rot-old-2", base.Add(-47*time.Hour), "error", "telegram", "telegram getUpdates failed old-2", "trace-old-2"),
		})
		seedRecordsForTest(t, filepath.Join(dataDir, logSignalName, "runtime-2026-04-11.jsonl"), []SignalRecord{
			newLogRecord("rot-mid-1", base.Add(-24*time.Hour), "error", "telegram", "telegram getUpdates failed mid-1", "trace-mid-1"),
			newLogRecord("rot-mid-2", base.Add(-23*time.Hour), "info", "scheduler", "scheduler idle cycle", "trace-mid-2"),
		})
		seedRecordsForTest(t, filepath.Join(dataDir, logSignalName, "runtime.jsonl"), []SignalRecord{
			newLogRecord("active-1", base.Add(-10*time.Minute), "error", "telegram", "telegram getUpdates failed new-1", "trace-new-1"),
			newLogRecord("active-2", base.Add(-5*time.Minute), "error", "telegram", "telegram getUpdates failed new-2", "trace-new-2"),
		})
	})

	items, err := store.QueryLogs(LogQuery{
		Query:     "getupdates failed",
		Level:     "error",
		Component: "telegram",
		Limit:     3,
	})
	if err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].ID != "active-2" || items[1].ID != "active-1" || items[2].ID != "rot-mid-1" {
		t.Fatalf("unexpected query order: %#v", signalIDs(items))
	}
}

func TestStoreRunRetentionAppliesRetentionAcrossRotatedFiles(t *testing.T) {
	t.Parallel()

	base := time.Now().UTC()
	store := newSeededStore(t, func(cfg *config.ObservabilityConfig) {
		cfg.Logs.Retention = 6 * time.Hour
	}, func(dataDir string) {
		seedRecordsForTest(t, filepath.Join(dataDir, logSignalName, "runtime-2026-04-10.jsonl"), []SignalRecord{
			newLogRecord("stale-1", base.Add(-48*time.Hour), "info", "scheduler", "stale-1", "trace-stale-1"),
		})
		seedRecordsForTest(t, filepath.Join(dataDir, logSignalName, "runtime-2026-04-11.jsonl"), []SignalRecord{
			newLogRecord("stale-2", base.Add(-12*time.Hour), "info", "scheduler", "stale-2", "trace-stale-2"),
			newLogRecord("fresh-rotated", base.Add(-2*time.Hour), "error", "telegram", "fresh-rotated", "trace-fresh-rotated"),
		})
		seedRecordsForTest(t, filepath.Join(dataDir, logSignalName, "runtime.jsonl"), []SignalRecord{
			newLogRecord("fresh-active", base.Add(-30*time.Minute), "error", "telegram", "fresh-active", "trace-fresh-active"),
		})
	})

	if err := store.RunRetention(context.Background()); err != nil {
		t.Fatalf("run retention: %v", err)
	}

	items, err := store.QueryLogs(LogQuery{Limit: 10})
	if err != nil {
		t.Fatalf("query logs: %v", err)
	}
	if got := signalIDs(items); len(got) != 2 || got[0] != "fresh-active" || got[1] != "fresh-rotated" {
		t.Fatalf("unexpected remaining log ids: %#v", got)
	}

	rotatedPath := filepath.Join(filepath.Dir(store.logPath), "runtime-2026-04-11.jsonl")
	rotatedRecords, err := readRecords(rotatedPath)
	if err != nil {
		t.Fatalf("read rotated file: %v", err)
	}
	if len(rotatedRecords) != 1 || rotatedRecords[0].ID != "fresh-rotated" {
		t.Fatalf("unexpected rotated records after retention: %#v", signalIDs(rotatedRecords))
	}
}

func TestStoreRunRetentionRotatesCurrentFileByDate(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
	store := newTestStore(t, nil)
	if err := store.AppendLog(newLogRecord("active-1", base, "error", "telegram", "first day", "trace-1")); err != nil {
		t.Fatalf("append first log: %v", err)
	}
	if err := store.AppendLog(newLogRecord("active-2", base.Add(25*time.Hour), "error", "telegram", "second day", "trace-2")); err != nil {
		t.Fatalf("append second log: %v", err)
	}

	rotatedPath := filepath.Join(filepath.Dir(store.logPath), "runtime-2026-04-12.jsonl")
	rotatedRecords, err := readRecords(rotatedPath)
	if err != nil {
		t.Fatalf("read rotated file: %v", err)
	}
	if len(rotatedRecords) != 1 || rotatedRecords[0].ID != "active-1" {
		t.Fatalf("unexpected rotated file content: %#v", signalIDs(rotatedRecords))
	}

	activeRecords, err := readRecords(store.logPath)
	if err != nil {
		t.Fatalf("read active file: %v", err)
	}
	if len(activeRecords) != 1 || activeRecords[0].ID != "active-2" {
		t.Fatalf("unexpected active file content: %#v", signalIDs(activeRecords))
	}
}

func TestStoreMetricsAreWrittenToRegistry(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
	store := newTestStore(t, func(cfg *config.ObservabilityConfig) {})
	attachStoreMetricsIfPossible(store, registry)

	if err := store.AppendLog(newLogRecord("metric-log", time.Now().UTC(), "error", "telegram", "telegram getUpdates failed", "trace-log")); err != nil {
		t.Fatalf("append log: %v", err)
	}
	if err := store.AppendEvent(SignalRecord{
		ID:          "metric-event",
		Kind:        SignalKindEvent,
		Timestamp:   time.Now().UTC(),
		Component:   "http_api",
		Message:     "http request completed",
		ExecutionID: "execution-1",
		TraceID:     "trace-event",
	}); err != nil {
		t.Fatalf("append event: %v", err)
	}
	if err := store.RunRetention(context.Background()); err != nil {
		t.Fatalf("run retention: %v", err)
	}

	output := metricsOutput(t, registry)
	assertMetricContains(t, output, `tars_observability_store_append_duration_seconds_count{signal="logs"} 1`)
	assertMetricContains(t, output, `tars_observability_store_append_duration_seconds_count{signal="traces"} 1`)
	assertMetricContains(t, output, `tars_observability_store_governance_duration_seconds_count{signal="logs"} 1`)
	assertMetricContains(t, output, `tars_observability_store_governance_duration_seconds_count{signal="traces"} 1`)
	assertMetricContains(t, output, `tars_observability_store_governance_duration_seconds_count{signal="metrics"} 1`)
	assertMetricContains(t, output, `tars_observability_store_records_total{signal="logs"} 1`)
	assertMetricContains(t, output, `tars_observability_store_records_total{signal="traces"} 1`)
	assertMetricContains(t, output, `tars_observability_store_file_bytes{signal="logs"}`)
	assertMetricContains(t, output, `tars_observability_store_file_bytes{signal="traces"}`)
	assertMetricContains(t, output, `tars_observability_store_file_bytes{signal="metrics"}`)
}

func TestStoreAppendDurationMetricIncludesGovernanceWork(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
	store := newTestStore(t, nil)
	attachStoreMetricsIfPossible(store, registry)

	store.mu.Lock()
	store.lastGovernance = time.Now().Add(-governanceInterval - time.Second)
	store.governanceTestHook = func() {
		time.Sleep(25 * time.Millisecond)
	}
	store.mu.Unlock()

	if err := store.AppendLog(newLogRecord("governance-latency", time.Now().UTC(), "error", "telegram", "governance latency sample", "trace-governance")); err != nil {
		t.Fatalf("append log: %v", err)
	}

	sum := metricValueForLine(t, metricsOutput(t, registry), `tars_observability_store_append_duration_seconds_sum{signal="logs"} `)
	if sum < 0.02 {
		t.Fatalf("expected append duration sum to include governance latency, got %f", sum)
	}
}

func newTestStore(t *testing.T, mutate func(*config.ObservabilityConfig)) *Store {
	t.Helper()
	return newSeededStore(t, mutate, nil)
}

func newSeededStore(t *testing.T, mutate func(*config.ObservabilityConfig), seed func(dataDir string)) *Store {
	t.Helper()
	dataDir := t.TempDir()
	if seed != nil {
		seed(dataDir)
	}
	cfg := config.ObservabilityConfig{
		DataDir: dataDir,
		Metrics: config.ObservabilitySignalConfig{Retention: 30 * 24 * time.Hour, MaxSizeBytes: 1 << 20},
		Logs:    config.ObservabilitySignalConfig{Retention: 30 * 24 * time.Hour, MaxSizeBytes: 1 << 20},
		Traces:  config.ObservabilitySignalConfig{Retention: 30 * 24 * time.Hour, MaxSizeBytes: 1 << 20},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

func seedRecordsForTest(t *testing.T, path string, records []SignalRecord) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			t.Fatalf("encode record: %v", err)
		}
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatalf("write seed file %s: %v", path, err)
	}
}

func newLogRecord(id string, ts time.Time, level string, component string, message string, traceID string) SignalRecord {
	return SignalRecord{
		ID:        id,
		Kind:      SignalKindLog,
		Timestamp: ts,
		Level:     level,
		Component: component,
		Message:   message,
		TraceID:   traceID,
	}
}

func signalIDs(records []SignalRecord) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}

func metricsOutput(t *testing.T, registry *foundationmetrics.Registry) string {
	t.Helper()
	var buf bytes.Buffer
	if err := registry.WritePrometheus(&buf); err != nil {
		t.Fatalf("write prometheus output: %v", err)
	}
	return buf.String()
}

func assertMetricContains(t *testing.T, output string, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("expected metrics output to contain %q, got:\n%s", want, output)
	}
}

func metricValueForLine(t *testing.T, output string, prefix string) float64 {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimPrefix(line, prefix), 64)
		if err != nil {
			t.Fatalf("parse metric line %q: %v", line, err)
		}
		return value
	}
	t.Fatalf("missing metric line with prefix %q in output:\n%s", prefix, output)
	return 0
}

func attachStoreMetricsIfPossible(store *Store, registry *foundationmetrics.Registry) {
	if store == nil || registry == nil {
		return
	}
	if setter, ok := any(store).(interface {
		SetMetrics(*foundationmetrics.Registry)
	}); ok {
		setter.SetMetrics(registry)
		return
	}
	value := reflect.ValueOf(store)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return
	}
	field := value.Elem().FieldByName("metrics")
	if !field.IsValid() || !field.CanSet() {
		return
	}
	field.Set(reflect.ValueOf(registry))
}
