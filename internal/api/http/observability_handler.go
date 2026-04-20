package httpapi

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"tars/internal/api/dto"
	"tars/internal/foundation/observability"
)

func logsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.Observability == nil {
			writeJSON(w, http.StatusOK, dto.LogListResponse{Items: []dto.LogRecord{}, ListPage: dto.ListPage{Page: 1, Limit: 20}})
			return
		}
		query := parseListQuery(r)
		from, _ := parseRFC3339(r.URL.Query().Get("from"))
		to, _ := parseRFC3339(r.URL.Query().Get("to"))
		items, err := deps.Observability.QueryLogs(observability.LogQuery{
			Query:       query.Query,
			Level:       r.URL.Query().Get("level"),
			Component:   r.URL.Query().Get("component"),
			SessionID:   r.URL.Query().Get("session_id"),
			ExecutionID: r.URL.Query().Get("execution_id"),
			TraceID:     r.URL.Query().Get("trace_id"),
			From:        from,
			To:          to,
			Limit:       1000,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		pageItems, meta := paginateItems(items, query)
		response := dto.LogListResponse{Items: make([]dto.LogRecord, 0, len(pageItems)), ListPage: meta}
		for _, item := range pageItems {
			response.Items = append(response.Items, logRecordDTO(item))
		}
		auditOpsRead(r.Context(), deps, "runtime_log", "*", "list", map[string]any{
			"query":        query.Query,
			"level":        r.URL.Query().Get("level"),
			"component":    r.URL.Query().Get("component"),
			"session_id":   r.URL.Query().Get("session_id"),
			"execution_id": r.URL.Query().Get("execution_id"),
			"trace_id":     r.URL.Query().Get("trace_id"),
			"page":         query.Page,
			"limit":        query.Limit,
			"total":        meta.Total,
		})
		writeJSON(w, http.StatusOK, response)
	}
}

func observabilitySummaryHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		response, err := buildObservabilityResponse(deps)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		auditOpsRead(r.Context(), deps, "observability", "summary", "get", map[string]any{
			"recent_logs":   len(response.RecentLogs),
			"recent_events": len(response.RecentEvents),
			"active_traces": response.Summary.ActiveTraces,
		})
		writeJSON(w, http.StatusOK, response)
	}
}

func logRecordDTO(item observability.SignalRecord) dto.LogRecord {
	return dto.LogRecord{
		ID:          item.ID,
		Timestamp:   item.Timestamp,
		Level:       item.Level,
		Component:   item.Component,
		Message:     item.Message,
		Route:       item.Route,
		Actor:       item.Actor,
		SessionID:   item.SessionID,
		ExecutionID: item.ExecutionID,
		TraceID:     item.TraceID,
		Metadata:    item.Metadata,
	}
}

func eventRecordDTO(item observability.SignalRecord) dto.TraceEventRecord {
	return dto.TraceEventRecord{
		ID:          item.ID,
		Timestamp:   item.Timestamp,
		Kind:        string(item.Kind),
		Component:   item.Component,
		Message:     item.Message,
		SessionID:   item.SessionID,
		ExecutionID: item.ExecutionID,
		TraceID:     item.TraceID,
		Actor:       item.Actor,
		Metadata:    item.Metadata,
	}
}

func buildObservabilityResponse(deps Dependencies) (dto.ObservabilityResponse, error) {
	response := dto.ObservabilityResponse{
		MetricsEndpoint: "/metrics",
		Health:          buildDashboardHealthResponse(deps),
	}
	if deps.Observability == nil {
		return response, nil
	}
	summary := deps.Observability.Summary()
	response.Retention = configStatusDTO(deps.Observability.ConfigStatus())
	response.Summary = dto.ObservabilitySummary{
		LogEntries24h:   int(summary.LogCount24h),
		ErrorEntries24h: int(summary.ErrorCount24h),
		EventEntries24h: int(summary.EventCount24h),
		ActiveTraces:    int(summary.TraceCount24h),
		LastLogAt:       summary.LastLogAt,
		LastEventAt:     summary.LastEventAt,
	}
	logs, err := deps.Observability.QueryLogs(observability.LogQuery{Limit: 8})
	if err != nil {
		return response, err
	}
	for _, item := range logs {
		response.RecentLogs = append(response.RecentLogs, logRecordDTO(item))
	}
	events, err := deps.Observability.QueryEvents(observability.EventQuery{Limit: 200})
	if err != nil {
		return response, err
	}
	for _, item := range events {
		if len(response.RecentEvents) >= 12 {
			break
		}
		response.RecentEvents = append(response.RecentEvents, eventRecordDTO(item))
	}
	response.TraceSamples = buildTraceSamples(events)
	return response, nil
}

func buildTraceSamples(items []observability.SignalRecord) []dto.TraceSample {
	groups := map[string][]observability.SignalRecord{}
	for _, item := range items {
		traceID := strings.TrimSpace(item.TraceID)
		if traceID == "" {
			traceID = firstNonEmptyString(item.SessionID, item.ExecutionID)
		}
		if traceID == "" {
			continue
		}
		groups[traceID] = append(groups[traceID], item)
	}
	out := make([]dto.TraceSample, 0, len(groups))
	for traceID, records := range groups {
		sort.SliceStable(records, func(i, j int) bool {
			return records[i].Timestamp.After(records[j].Timestamp)
		})
		latest := records[0]
		components := make([]string, 0, len(records))
		seen := map[string]struct{}{}
		for _, record := range records {
			component := strings.TrimSpace(record.Component)
			if component == "" {
				continue
			}
			if _, ok := seen[component]; ok {
				continue
			}
			seen[component] = struct{}{}
			components = append(components, component)
		}
		out = append(out, dto.TraceSample{
			TraceID:     traceID,
			SessionID:   latest.SessionID,
			ExecutionID: latest.ExecutionID,
			Component:   latest.Component,
			EventCount:  len(records),
			LastEventAt: latest.Timestamp,
			LastMessage: latest.Message,
			Components:  components,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LastEventAt.After(out[j].LastEventAt)
	})
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func configStatusDTO(status observability.ConfigStatus) dto.ObservabilityRetentionStatus {
	return dto.ObservabilityRetentionStatus{
		DataDir: status.DataDir,
		Metrics: dto.ObservabilitySignalConfig{
			RetentionHours: status.Metrics.RetentionHours,
			MaxSizeBytes:   status.Metrics.MaxSizeBytes,
			CurrentBytes:   status.Metrics.CurrentBytes,
			FilePath:       status.Metrics.FilePath,
		},
		Logs: dto.ObservabilitySignalConfig{
			RetentionHours: status.Logs.RetentionHours,
			MaxSizeBytes:   status.Logs.MaxSizeBytes,
			CurrentBytes:   status.Logs.CurrentBytes,
			FilePath:       status.Logs.FilePath,
		},
		Traces: dto.ObservabilitySignalConfig{
			RetentionHours: status.Traces.RetentionHours,
			MaxSizeBytes:   status.Traces.MaxSizeBytes,
			CurrentBytes:   status.Traces.CurrentBytes,
			FilePath:       status.Traces.FilePath,
		},
		OTLP: dto.OTLPStatus{
			Endpoint:       status.OTLP.Endpoint,
			Protocol:       status.OTLP.Protocol,
			Insecure:       status.OTLP.Insecure,
			MetricsEnabled: status.OTLP.MetricsEnabled,
			LogsEnabled:    status.OTLP.LogsEnabled,
			TracesEnabled:  status.OTLP.TracesEnabled,
		},
		Exporters: status.Exporters,
	}
}

func parseRFC3339(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
