package observability

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"tars/internal/foundation/config"
	foundationmetrics "tars/internal/foundation/metrics"
)

const (
	logSignalName     = "logs"
	traceSignalName   = "traces"
	metricsSignalName = "metrics"
)

const reverseScanChunkSize int64 = 64 * 1024

type SignalKind string

const (
	SignalKindLog            SignalKind = "log"
	SignalKindEvent          SignalKind = "event"
	SignalKindMetricSnapshot SignalKind = "metric_snapshot"
)

var storeDurationBuckets = []float64{0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

type SignalRecord struct {
	ID          string         `json:"id"`
	Kind        SignalKind     `json:"kind"`
	Timestamp   time.Time      `json:"timestamp"`
	Level       string         `json:"level,omitempty"`
	Component   string         `json:"component,omitempty"`
	Message     string         `json:"message"`
	Route       string         `json:"route,omitempty"`
	Actor       string         `json:"actor,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	ExecutionID string         `json:"execution_id,omitempty"`
	TraceID     string         `json:"trace_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type LogQuery struct {
	Query       string
	Level       string
	Component   string
	SessionID   string
	ExecutionID string
	TraceID     string
	From        time.Time
	To          time.Time
	Limit       int
}

type EventQuery struct {
	Query       string
	Component   string
	SessionID   string
	ExecutionID string
	TraceID     string
	From        time.Time
	To          time.Time
	Limit       int
}

type Summary struct {
	LogCount24h         int64
	ErrorCount24h       int64
	EventCount24h       int64
	TraceCount24h       int64
	LastLogAt           time.Time
	LastEventAt         time.Time
	LogStorageBytes     int64
	TraceStorageBytes   int64
	MetricsStorageBytes int64
}

type SignalRetentionStatus struct {
	RetentionHours float64 `json:"retention_hours"`
	MaxSizeBytes   int64   `json:"max_size_bytes"`
	CurrentBytes   int64   `json:"current_bytes"`
	FilePath       string  `json:"file_path,omitempty"`
}

type OTLPStatus struct {
	Endpoint       string `json:"endpoint,omitempty"`
	Protocol       string `json:"protocol,omitempty"`
	Insecure       bool   `json:"insecure"`
	MetricsEnabled bool   `json:"metrics_enabled"`
	LogsEnabled    bool   `json:"logs_enabled"`
	TracesEnabled  bool   `json:"traces_enabled"`
}

type ConfigStatus struct {
	DataDir   string                `json:"data_dir"`
	Metrics   SignalRetentionStatus `json:"metrics"`
	Logs      SignalRetentionStatus `json:"logs"`
	Traces    SignalRetentionStatus `json:"traces"`
	OTLP      OTLPStatus            `json:"otlp"`
	Exporters []string              `json:"exporters,omitempty"`
}

// governanceInterval controls how often append() runs the expensive
// full-file governance (retention + size trim) and summary rebuild.
// Between governance runs, summary counters are updated incrementally
// in O(1) per write instead of O(N) full-file scans.
const governanceInterval = 5 * time.Minute

type Store struct {
	mu             sync.RWMutex
	config         config.ObservabilityConfig
	logPath        string
	tracePath      string
	metricsPath    string
	metrics        *foundationmetrics.Registry
	summary        Summary
	nextID         uint64
	lastGovernance time.Time
	signalCounts   map[string]int64
	activeDays     map[string]string
	governanceTestHook func()
}

func NewStore(cfg config.ObservabilityConfig) (*Store, error) {
	dataDir := strings.TrimSpace(cfg.DataDir)
	if dataDir == "" {
		dataDir = "./data/observability"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create observability data dir: %w", err)
	}
	logsDir := filepath.Join(dataDir, logSignalName)
	tracesDir := filepath.Join(dataDir, traceSignalName)
	metricsDir := filepath.Join(dataDir, metricsSignalName)
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create observability logs dir: %w", err)
	}
	if err := os.MkdirAll(tracesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create observability traces dir: %w", err)
	}
	if err := os.MkdirAll(metricsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create observability metrics dir: %w", err)
	}
	store := &Store{
		config:         cfg,
		logPath:        filepath.Join(logsDir, "runtime.jsonl"),
		tracePath:      filepath.Join(tracesDir, "events.jsonl"),
		metricsPath:    filepath.Join(metricsDir, "snapshots.jsonl"),
		lastGovernance: time.Now(),
		signalCounts:   map[string]int64{},
		activeDays:     map[string]string{},
	}
	if err := store.rebuildSummary(); err != nil {
		return nil, err
	}
	return store, nil

}

func (s *Store) SetMetrics(registry *foundationmetrics.Registry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = registry
	s.publishStoreMetricsLocked()
}

func (s *Store) LogPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logPath
}

func (s *Store) AppendLog(record SignalRecord) error {
	record.Kind = SignalKindLog
	return s.append(logSignalName, s.logPath, s.config.Logs, record)
}

func (s *Store) AppendEvent(record SignalRecord) error {
	record.Kind = SignalKindEvent
	return s.append(traceSignalName, s.tracePath, s.config.Traces, record)
}

func (s *Store) append(signal string, path string, cfg config.ObservabilitySignalConfig, record SignalRecord) error {
	started := time.Now()
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}
	if record.ID == "" {
		record.ID = s.nextRecordID(signal, record.Timestamp)
	}
	if record.Metadata == nil {
		record.Metadata = map[string]any{}
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	writeCommitted := false
	defer func() {
		if writeCommitted {
			s.observeStoreHistogramLocked("tars_observability_store_append_duration_seconds", "Append duration for observability store writes.", signal, time.Since(started))
		}
	}()
	if err := s.rotateCurrentFileLocked(signal, path, record.Timestamp); err != nil {
		return err
	}
	if err := appendBytes(path, payload); err != nil {
		return err
	}
	writeCommitted = true
	s.signalCounts[signal]++
	// O(1) incremental summary update instead of full-file rebuild.
	s.updateSummaryLocked(record, int64(len(payload)))
	s.publishStoreMetricsLocked()

	// Run expensive governance (retention trim + full rebuild) only periodically.
	if time.Since(s.lastGovernance) >= governanceInterval {
		if s.governanceTestHook != nil {
			s.governanceTestHook()
		}
		if err := s.applyGovernanceLocked(path, cfg); err != nil {
			return err
		}
		s.lastGovernance = time.Now()
		return s.rebuildSummaryLocked()
	}
	return nil
}

func (s *Store) QueryLogs(query LogQuery) ([]SignalRecord, error) {
	return s.querySignalFiles(logSignalName, query.Limit, func(record SignalRecord) bool {
		if record.Kind != SignalKindLog {
			return false
		}
		if !matchesTimeRange(record.Timestamp, query.From, query.To) {
			return false
		}
		if !matchesFolded(record.Level, query.Level) {
			return false
		}
		if !matchesFolded(record.Component, query.Component) {
			return false
		}
		if !matchesExact(record.SessionID, query.SessionID) {
			return false
		}
		if !matchesExact(record.ExecutionID, query.ExecutionID) {
			return false
		}
		if !matchesExact(record.TraceID, query.TraceID) {
			return false
		}
		return matchesText(record, query.Query)
	})
}

func (s *Store) QueryEvents(query EventQuery) ([]SignalRecord, error) {
	return s.querySignalFiles(traceSignalName, query.Limit, func(record SignalRecord) bool {
		if record.Kind != SignalKindEvent {
			return false
		}
		if !matchesTimeRange(record.Timestamp, query.From, query.To) {
			return false
		}
		if !matchesFolded(record.Component, query.Component) {
			return false
		}
		if !matchesExact(record.SessionID, query.SessionID) {
			return false
		}
		if !matchesExact(record.ExecutionID, query.ExecutionID) {
			return false
		}
		if !matchesExact(record.TraceID, query.TraceID) {
			return false
		}
		return matchesText(record, query.Query)
	})
}

func (s *Store) Summary() Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summary
}

func (s *Store) ConfigStatus() ConfigStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ConfigStatus{
		DataDir: strings.TrimSpace(s.config.DataDir),
		Metrics: SignalRetentionStatus{
			RetentionHours: s.config.Metrics.Retention.Hours(),
			MaxSizeBytes:   s.config.Metrics.MaxSizeBytes,
			CurrentBytes:   s.summary.MetricsStorageBytes,
			FilePath:       s.metricsPath,
		},
		Logs: SignalRetentionStatus{
			RetentionHours: s.config.Logs.Retention.Hours(),
			MaxSizeBytes:   s.config.Logs.MaxSizeBytes,
			CurrentBytes:   s.summary.LogStorageBytes,
			FilePath:       s.logPath,
		},
		Traces: SignalRetentionStatus{
			RetentionHours: s.config.Traces.Retention.Hours(),
			MaxSizeBytes:   s.config.Traces.MaxSizeBytes,
			CurrentBytes:   s.summary.TraceStorageBytes,
			FilePath:       s.tracePath,
		},
		OTLP: OTLPStatus{
			Endpoint:       s.config.OTLP.Endpoint,
			Protocol:       s.config.OTLP.Protocol,
			Insecure:       s.config.OTLP.Insecure,
			MetricsEnabled: s.config.OTLP.MetricsEnable,
			LogsEnabled:    s.config.OTLP.LogsEnable,
			TracesEnabled:  s.config.OTLP.TracesEnable,
		},
		Exporters: enabledExporters(s.config.OTLP),
	}
}

func (s *Store) RunRetention(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.applyGovernanceLocked(s.logPath, s.config.Logs); err != nil {
		return err
	}
	if err := s.applyGovernanceLocked(s.tracePath, s.config.Traces); err != nil {
		return err
	}
	// Write a metrics snapshot entry so CurrentBytes reflects real on-disk data.
	_ = s.appendMetricsSnapshotLocked()
	if err := s.applyGovernanceLocked(s.metricsPath, s.config.Metrics); err != nil {
		return err
	}
	s.lastGovernance = time.Now()
	return s.rebuildSummaryLocked()
}

// appendMetricsSnapshotLocked writes one JSONL line to the metrics snapshot file.
// Must be called with s.mu held.
func (s *Store) appendMetricsSnapshotLocked() error {
	type metricsSnapshot struct {
		Timestamp     time.Time `json:"timestamp"`
		LogCount24h   int64     `json:"log_count_24h"`
		ErrorCount24h int64     `json:"error_count_24h"`
		EventCount24h int64     `json:"event_count_24h"`
		TraceCount24h int64     `json:"trace_count_24h"`
	}
	snap := metricsSnapshot{
		Timestamp:     time.Now().UTC(),
		LogCount24h:   s.summary.LogCount24h,
		ErrorCount24h: s.summary.ErrorCount24h,
		EventCount24h: s.summary.EventCount24h,
		TraceCount24h: s.summary.TraceCount24h,
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := s.rotateCurrentFileLocked(metricsSignalName, s.metricsPath, snap.Timestamp); err != nil {
		return err
	}
	if err := appendBytes(s.metricsPath, payload); err != nil {
		return err
	}
	s.signalCounts[metricsSignalName]++
	return nil
}

func (s *Store) rebuildSummary() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rebuildSummaryLocked()
}

func (s *Store) rebuildSummaryLocked() error {
	s.summary = Summary{}
	logs, err := readSignalRecords(s.signalFilesLocked(logSignalName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	traces, err := readSignalRecords(s.signalFilesLocked(traceSignalName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s.summary.LogStorageBytes = totalFileBytes(s.signalFilesLocked(logSignalName))
	s.summary.TraceStorageBytes = totalFileBytes(s.signalFilesLocked(traceSignalName))
	s.summary.MetricsStorageBytes = totalFileBytes(s.signalFilesLocked(metricsSignalName))
	s.signalCounts[logSignalName] = int64(len(logs))
	s.signalCounts[traceSignalName] = int64(len(traces))
	s.signalCounts[metricsSignalName] = countSignalRecords(s.signalFilesLocked(metricsSignalName))
	for _, item := range logs {
		s.updateSummaryLocked(item, 0)
	}
	for _, item := range traces {
		s.updateSummaryLocked(item, 0)
	}
	s.publishStoreMetricsLocked()
	return nil
}

func (s *Store) updateSummaryLocked(record SignalRecord, deltaBytes int64) {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	if deltaBytes != 0 {
		if record.Kind == SignalKindLog {
			s.summary.LogStorageBytes += deltaBytes
		} else if record.Kind == SignalKindEvent {
			s.summary.TraceStorageBytes += deltaBytes
		}
	}
	if record.Timestamp.After(cutoff) {
		switch record.Kind {
		case SignalKindLog:
			s.summary.LogCount24h++
			if strings.EqualFold(record.Level, "error") {
				s.summary.ErrorCount24h++
			}
		case SignalKindEvent:
			s.summary.EventCount24h++
			if strings.TrimSpace(record.TraceID) != "" {
				s.summary.TraceCount24h++
			}
		}
	}
	if record.Kind == SignalKindLog && record.Timestamp.After(s.summary.LastLogAt) {
		s.summary.LastLogAt = record.Timestamp
	}
	if record.Kind == SignalKindEvent && record.Timestamp.After(s.summary.LastEventAt) {
		s.summary.LastEventAt = record.Timestamp
	}
}

func (s *Store) querySignalFiles(signal string, limit int, match func(SignalRecord) bool) ([]SignalRecord, error) {
	s.mu.RLock()
	files := append([]string(nil), s.signalFilesLocked(signal)...)
	s.mu.RUnlock()
	if len(files) == 0 {
		return nil, nil
	}
	return querySignalFilesNewestFirst(files, limit, match)
}

func (s *Store) applyGovernanceLocked(path string, cfg config.ObservabilitySignalConfig) error {
	signal := signalNameForPath(path)
	started := time.Now()
	files := s.signalFilesLocked(signal)
	cutoff := time.Time{}
	if cfg.Retention > 0 {
		cutoff = time.Now().UTC().Add(-cfg.Retention)
	}
	changedAny := false
	retainedPerFile := make(map[string][]SignalRecord, len(files))
	retainedAll := make([]SignalRecord, 0, 256)

	for _, candidate := range files {
		records, err := readRecords(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		trimmed := records
		if !cutoff.IsZero() {
			filtered := trimmed[:0]
			for _, item := range trimmed {
				if item.Timestamp.IsZero() || !item.Timestamp.Before(cutoff) {
					filtered = append(filtered, item)
				}
			}
			if len(filtered) != len(records) {
				changedAny = true
			}
			trimmed = filtered
		}
		retained := append([]SignalRecord(nil), trimmed...)
		retainedPerFile[candidate] = retained
		retainedAll = append(retainedAll, retained...)
	}

	if cfg.MaxSizeBytes > 0 {
		trimmed, removed, err := trimBySize(retainedAll, cfg.MaxSizeBytes)
		if err != nil {
			return err
		}
		if removed {
			changedAny = true
		}
		retainedAll = trimmed
		allowed := make(map[string]int, len(retainedAll))
		for _, item := range retainedAll {
			allowed[item.ID]++
		}
		for file, records := range retainedPerFile {
			filtered := records[:0]
			for _, item := range records {
				if allowed[item.ID] <= 0 {
					continue
				}
				allowed[item.ID]--
				filtered = append(filtered, item)
			}
			if len(filtered) != len(records) {
				changedAny = true
			}
			retainedPerFile[file] = append([]SignalRecord(nil), filtered...)
		}
	}

	for _, candidate := range files {
		records := retainedPerFile[candidate]
		if len(records) == 0 {
			if err := removeIfExists(candidate); err != nil {
				return err
			}
			continue
		}
		if !changedAny {
			continue
		}
		if err := rewriteRecords(candidate, records); err != nil {
			return err
		}
	}
	if signal != "" {
		s.observeStoreHistogramLocked("tars_observability_store_governance_duration_seconds", "Governance duration for observability store maintenance.", signal, time.Since(started))
	}
	return nil
}

func (s *Store) nextRecordID(signal string, ts time.Time) string {
	s.nextID++
	return fmt.Sprintf("%s-%d-%d", signal, ts.UnixNano(), s.nextID)
}

func enabledExporters(cfg config.ObservabilityOTLPConfig) []string {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil
	}
	items := make([]string, 0, 3)
	if cfg.MetricsEnable {
		items = append(items, "otlp-metrics")
	}
	if cfg.LogsEnable {
		items = append(items, "otlp-logs")
	}
	if cfg.TracesEnable {
		items = append(items, "otlp-traces")
	}
	return items
}

func (s *Store) publishStoreMetricsLocked() {
	if s.metrics == nil {
		return
	}
	for _, signal := range []string{logSignalName, traceSignalName, metricsSignalName} {
		s.metrics.SetGauge("tars_observability_store_file_bytes", "On-disk bytes for observability store files by signal.", foundationmetrics.Labels{"signal": signal}, float64(s.storeBytesForSignalLocked(signal)))
		s.metrics.SetGauge("tars_observability_store_records_total", "Total retained observability records by signal.", foundationmetrics.Labels{"signal": signal}, float64(s.signalCounts[signal]))
	}
}

func (s *Store) observeStoreHistogramLocked(name string, help string, signal string, duration time.Duration) {
	if s.metrics == nil || strings.TrimSpace(signal) == "" {
		return
	}
	s.metrics.ObserveHistogram(name, help, foundationmetrics.Labels{"signal": signal}, storeDurationBuckets, duration.Seconds())
}

func (s *Store) rotateCurrentFileLocked(signal string, activePath string, ts time.Time) error {
	if ts.IsZero() {
		return nil
	}
	day := ts.UTC().Format("2006-01-02")
	if day == "" {
		return nil
	}
	previousDay := s.activeDays[signal]
	if previousDay == "" {
		if hasAnyRecords(activePath) {
			if latest, ok := latestTimestampFromFile(activePath); ok {
				previousDay = latest.UTC().Format("2006-01-02")
			}
		}
		if previousDay == "" {
			s.activeDays[signal] = day
			return nil
		}
	}
	if previousDay == day {
		s.activeDays[signal] = day
		return nil
	}
	if previousDay > day {
		return nil
	}
	rotatedPath := rotatedPathForDay(activePath, previousDay)
	if hasAnyRecords(activePath) {
		if err := appendFileContents(rotatedPath, activePath); err != nil {
			return err
		}
	}
	if err := os.Remove(activePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ensureParentDir(activePath); err != nil {
		return err
	}
	s.activeDays[signal] = day
	return nil
}

func (s *Store) signalFilesLocked(signal string) []string {
	active := s.activePathForSignal(signal)
	if active == "" {
		return nil
	}
	files := signalFilesForActivePath(active)
	if len(files) == 0 {
		return []string{active}
	}
	return files
}

func (s *Store) activePathForSignal(signal string) string {
	switch signal {
	case logSignalName:
		return s.logPath
	case traceSignalName:
		return s.tracePath
	case metricsSignalName:
		return s.metricsPath
	default:
		return ""
	}
}

func (s *Store) storeBytesForSignalLocked(signal string) int64 {
	switch signal {
	case logSignalName:
		return s.summary.LogStorageBytes
	case traceSignalName:
		return s.summary.TraceStorageBytes
	case metricsSignalName:
		return s.summary.MetricsStorageBytes
	default:
		return 0
	}
}

func signalNameForPath(path string) string {
	base := filepath.Base(path)
	if strings.HasPrefix(base, "runtime") {
		return logSignalName
	}
	if strings.HasPrefix(base, "events") {
		return traceSignalName
	}
	if strings.HasPrefix(base, "snapshots") {
		return metricsSignalName
	}
	return ""
}

func signalFilesForActivePath(activePath string) []string {
	dir := filepath.Dir(activePath)
	base := filepath.Base(activePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{activePath}
	}
	prefix, suffix := rotationPattern(base)
	files := make([]string, 0, len(entries)+1)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == base || rotatedNameMatches(name, prefix, suffix) {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.SliceStable(files, func(i, j int) bool {
		return files[i] < files[j]
	})
	if len(files) == 0 {
		files = append(files, activePath)
	}
	return files
}

func rotationPattern(base string) (string, string) {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return stem + "-", ext
}

func rotatedNameMatches(name string, prefix string, suffix string) bool {
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return false
	}
	datePart := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
	if len(datePart) != len("2006-01-02") {
		return false
	}
	_, err := time.Parse("2006-01-02", datePart)
	return err == nil
}

func rotatedPathForDay(activePath string, day string) string {
	dir := filepath.Dir(activePath)
	base := filepath.Base(activePath)
	prefix, suffix := rotationPattern(base)
	return filepath.Join(dir, prefix+day+suffix)
}

func ensureParentDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func appendFileContents(dst string, src string) error {
	if err := ensureParentDir(dst); err != nil {
		return err
	}
	srcFile, err := os.Open(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

func hasAnyRecords(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

func latestTimestampFromFile(path string) (time.Time, bool) {
	records, err := readRecords(path)
	if err != nil || len(records) == 0 {
		return time.Time{}, false
	}
	return records[len(records)-1].Timestamp, true
}

func totalFileBytes(paths []string) int64 {
	var total int64
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		total += info.Size()
	}
	return total
}

func countSignalRecords(paths []string) int64 {
	var total int64
	for _, path := range paths {
		records, err := readRecords(path)
		if err != nil {
			continue
		}
		total += int64(len(records))
	}
	return total
}

func readSignalRecords(paths []string) ([]SignalRecord, error) {
	all := make([]SignalRecord, 0, 256)
	for _, path := range paths {
		records, err := readRecords(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		all = append(all, records...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Timestamp.Before(all[j].Timestamp)
	})
	return all, nil
}

func queryFileNewestFirst(path string, limit int, match func(SignalRecord) bool) ([]SignalRecord, error) {
	if limit > 0 {
		items, err := queryFileNewestFirstStream(path, limit, match)
		if err == nil {
			return items, nil
		}
		if !errors.Is(err, errReverseScanFallback) {
			return nil, err
		}
	}
	records, err := readRecords(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	filtered := make([]SignalRecord, 0, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		if match(records[i]) {
			filtered = append(filtered, records[i])
			if limit > 0 && len(filtered) >= limit {
				break
			}
		}
	}
	return filtered, nil
}

func querySignalFilesNewestFirst(paths []string, limit int, match func(SignalRecord) bool) ([]SignalRecord, error) {
	filtered := make([]SignalRecord, 0, max(limit, 16))
	for i := len(paths) - 1; i >= 0; i-- {
		items, err := queryFileNewestFirst(paths[i], limitRemaining(limit, len(filtered)), match)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		filtered = append(filtered, items...)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Timestamp.Equal(filtered[j].Timestamp) {
			return filtered[i].ID > filtered[j].ID
		}
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func limitRemaining(limit int, current int) int {
	if limit <= 0 {
		return 0
	}
	remaining := limit - current
	if remaining < 0 {
		return 0
	}
	return remaining
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

var errReverseScanFallback = errors.New("reverse scan fallback")

func queryFileNewestFirstStream(path string, limit int, match func(SignalRecord) bool) ([]SignalRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return nil, nil
	}
	var (
		offset   = info.Size()
		carry    []byte
		matches  []SignalRecord
		seenOld  time.Time
		hitOlder bool
	)
	for offset > 0 && len(matches) < limit {
		chunkSize := reverseScanChunkSize
		if offset < chunkSize {
			chunkSize = offset
		}
		offset -= chunkSize
		buf := make([]byte, chunkSize)
		if _, err := file.ReadAt(buf, offset); err != nil {
			return nil, err
		}
		buf = append(buf, carry...)
		lines := bytes.Split(buf, []byte{'\n'})
		carry = append(carry[:0], lines[0]...)
		for i := len(lines) - 1; i >= 1 && len(matches) < limit; i-- {
			line := bytes.TrimSpace(lines[i])
			if len(line) == 0 {
				continue
			}
			var item SignalRecord
			if err := json.Unmarshal(line, &item); err != nil {
				return nil, errReverseScanFallback
			}
			if !seenOld.IsZero() && item.Timestamp.After(seenOld) {
				return nil, errReverseScanFallback
			}
			seenOld = item.Timestamp
			if match(item) {
				matches = append(matches, item)
				hitOlder = true
			}
		}
		if hitOlder && len(matches) >= limit {
			break
		}
	}
	if len(carry) > 0 && len(matches) < limit {
		line := bytes.TrimSpace(carry)
		if len(line) > 0 {
			var item SignalRecord
			if err := json.Unmarshal(line, &item); err != nil {
				return nil, errReverseScanFallback
			}
			if !seenOld.IsZero() && item.Timestamp.After(seenOld) {
				return nil, errReverseScanFallback
			}
			if match(item) {
				matches = append(matches, item)
			}
		}
	}
	return matches, nil
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func appendBytes(path string, payload []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(payload)
	return err
}

func readRecords(path string) ([]SignalRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	items := make([]SignalRecord, 0, 128)
	for {
		line, err := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var item SignalRecord
			if decodeErr := json.Unmarshal(bytes.TrimSpace(line), &item); decodeErr == nil {
				items = append(items, item)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Timestamp.Before(items[j].Timestamp)
	})
	return items, nil
}

func rewriteRecords(path string, records []SignalRecord) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for _, item := range records {
		if err := encoder.Encode(item); err != nil {
			return err
		}
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}

func trimBySize(records []SignalRecord, maxBytes int64) ([]SignalRecord, bool, error) {
	if maxBytes <= 0 || len(records) == 0 {
		return records, false, nil
	}
	total := int64(0)
	sizes := make([]int64, len(records))
	for i, item := range records {
		payload, err := json.Marshal(item)
		if err != nil {
			return nil, false, err
		}
		sizes[i] = int64(len(payload) + 1)
		total += sizes[i]
	}
	if total <= maxBytes {
		return records, false, nil
	}
	start := 0
	for start < len(records) && total > maxBytes {
		total -= sizes[start]
		start++
	}
	if start >= len(records) {
		return nil, true, nil
	}
	return append([]SignalRecord(nil), records[start:]...), true, nil
}

func matchesTimeRange(ts, from, to time.Time) bool {
	if !from.IsZero() && ts.Before(from) {
		return false
	}
	if !to.IsZero() && ts.After(to) {
		return false
	}
	return true
}

func matchesFolded(value string, filter string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(value), filter)
}

func matchesExact(value string, filter string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	return strings.TrimSpace(value) == filter
}

func matchesText(record SignalRecord, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	fields := []string{record.Message, record.Component, record.Level, record.Route, record.Actor, record.SessionID, record.ExecutionID, record.TraceID}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	if len(record.Metadata) == 0 {
		return false
	}
	payload, err := json.Marshal(record.Metadata)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(payload)), query)
}
