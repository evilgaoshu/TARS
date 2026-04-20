package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tars/internal/contracts"
	"tars/internal/modules/connectors"
)

// VictoriaLogsRuntime is the first-class runtime for connectors with protocol
// "victorialogs_http". It supports:
//
//   - CheckHealth: GET <base_url>/health
//   - Invoke("logs.query" | "victorialogs.query" | …): POST <base_url>/select/logsql/query
//
// Parameters for logs.query:
//
//	query      string  – LogsQL expression (required)
//	limit      int     – max # of log entries to return (default 20, max 200)
//	time_range string  – duration string, e.g. "1h", "24h" (default "1h")
//	start      string  – RFC3339 start time (overrides time_range)
//	end        string  – RFC3339 end time (defaults to now)
//
// No credentials are stored in plain values. If bearer_token is needed,
// specify via secret_ref; this runtime reads manifest.Config.Values["bearer_token"]
// (populated at runtime from the resolved secret_ref).
type VictoriaLogsRuntime struct {
	client *http.Client
}

// NewVictoriaLogsRuntime creates a VictoriaLogsRuntime with the given HTTP client.
// If client is nil, a 15-second timeout default is used.
func NewVictoriaLogsRuntime(client *http.Client) *VictoriaLogsRuntime {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &VictoriaLogsRuntime{client: client}
}

// CheckHealth probes <base_url>/health. VictoriaLogs returns "OK" (text/plain)
// on a healthy instance.
func (r *VictoriaLogsRuntime) CheckHealth(ctx context.Context, manifest connectors.Manifest) (string, string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(manifest.Config.Values["base_url"]), "/")
	if baseURL == "" {
		return "unhealthy", "victorialogs base_url is not configured", fmt.Errorf("victorialogs base_url is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return "unhealthy", formatHTTPProbeRequestError("victorialogs", err), err
	}
	if token := strings.TrimSpace(manifest.Config.Values["bearer_token"]); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "unhealthy", formatHTTPProbeTransportError("victorialogs", err), err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "unhealthy", "victorialogs health probe read failed", err
	}

	if resp.StatusCode >= 400 {
		return "unhealthy", fmt.Sprintf("victorialogs health probe returned status=%d", resp.StatusCode), fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	trimmedBody := strings.TrimSpace(string(body))
	if trimmedBody != "OK" {
		return "unhealthy", fmt.Sprintf("victorialogs health probe returned unexpected body: %q", trimmedBody), fmt.Errorf("expected OK, got %q", trimmedBody)
	}

	return "healthy", fmt.Sprintf("victorialogs health probe succeeded (%s)", trimmedBody), nil
}


// Invoke handles capability IDs: "logs.query", "victorialogs.query", "query",
// "log.query", "observability.query", or any capability with action "query".
func (r *VictoriaLogsRuntime) Invoke(ctx context.Context, manifest connectors.Manifest, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	capID := strings.ToLower(strings.TrimSpace(capabilityID))
	switch capID {
	case "", "logs.query", "victorialogs.query", "query", "log.query", "observability.query":
		// supported
	default:
		// also accept query-action capabilities
		action := strings.ToLower(strings.TrimSpace(capabilityFirstNonEmpty(interfaceString(params["action"]))))
		if action != "query" {
			return contracts.CapabilityResult{
				Status:  "failed",
				Error:   fmt.Sprintf("unsupported victorialogs capability %q", capabilityID),
				Runtime: capabilityRuntimeMetadataWithFallback(manifest, "victorialogs_http", false, false, "", ""),
			}, fmt.Errorf("unsupported victorialogs capability %q", capabilityID)
		}
	}

	baseURL := strings.TrimRight(strings.TrimSpace(manifest.Config.Values["base_url"]), "/")
	if baseURL == "" {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   "victorialogs connector base_url is not configured",
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "victorialogs_http", false, false, "", ""),
		}, fmt.Errorf("victorialogs connector base_url is not configured")
	}

	reqURL, err := buildVictoriaLogsQueryURL(baseURL, params)
	if err != nil {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "victorialogs_http", false, false, "", ""),
		}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "victorialogs_http", false, false, "", ""),
		}, err
	}
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(manifest.Config.Values["bearer_token"]); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "victorialogs_http", false, false, "", ""),
		}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MB
	if err != nil {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "victorialogs_http", false, false, "", ""),
		}, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("victorialogs query failed with status %d", resp.StatusCode)
		}
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   message,
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "victorialogs_http", false, false, "", ""),
			Metadata: map[string]interface{}{
				"http_status": resp.StatusCode,
			},
		}, fmt.Errorf("victorialogs query failed: %s", message)
	}

	logs, summary := parseVictoriaLogsPayload(body)
	effectiveCapID := capabilityFirstNonEmpty(capabilityID, "logs.query")

	artifacts := buildCapabilityArtifacts("victorialogs", logs, summary, map[string]interface{}{
		"connector_id":  strings.TrimSpace(manifest.Metadata.ID),
		"capability_id": effectiveCapID,
		"query_params":  cloneCapabilityParams(params),
		"request_url":   req.URL.String(),
	})

	return contracts.CapabilityResult{
		Status: "completed",
		Output: map[string]interface{}{
			"source":          "victorialogs_http",
			"capability_id":   effectiveCapID,
			"connector_id":    strings.TrimSpace(manifest.Metadata.ID),
			"query_params":    cloneCapabilityParams(params),
			"result_count":    len(logs),
			"summary":         summary,
			"logs":            logs,
			"queried_at":      time.Now().UTC().Format(time.RFC3339),
			"runtime_state":   "real",
			"capability_kind": "logs",
			"request_url":     req.URL.String(),
		},
		Artifacts: artifacts,
		Metadata: map[string]interface{}{
			"http_status": resp.StatusCode,
			"request_url": req.URL.String(),
			"log_count":   len(logs),
		},
		Runtime: capabilityRuntimeMetadataWithFallback(manifest, "victorialogs_http", true, false, "", "stub"),
	}, nil
}

// buildVictoriaLogsQueryURL constructs the VictoriaLogs query URL.
// VictoriaLogs uses GET /select/logsql/query with query params:
//
//	query     – LogsQL expression
//	limit     – max entries
//	start/end – RFC3339 or relative time
func buildVictoriaLogsQueryURL(baseURL string, params map[string]interface{}) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse victorialogs base_url: %w", err)
	}
	// Append the standard VictoriaLogs query endpoint
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/select/logsql/query"

	q := parsed.Query()

	// LogsQL query expression
	query := strings.TrimSpace(interfaceString(params["query"]))
	if query == "" {
		// Default: return everything (last entries)
		query = "*"
	}
	q.Set("query", query)

	// limit
	limit := clampInt(intFromAny(params["limit"], 20), 1, 200)
	q.Set("limit", strconv.Itoa(limit))

	// Time range: use start/end if provided, otherwise time_range duration
	if start := strings.TrimSpace(interfaceString(params["start"])); start != "" {
		q.Set("start", start)
	} else {
		timeRange := strings.TrimSpace(interfaceString(params["time_range"]))
		if timeRange == "" {
			timeRange = "1h"
		}
		// VictoriaLogs accepts relative start like "now-1h"
		q.Set("start", "now-"+timeRange)
	}
	if end := strings.TrimSpace(interfaceString(params["end"])); end != "" {
		q.Set("end", end)
	}

	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

// victoriaLogsEntry is a single log line returned by VictoriaLogs.
type victoriaLogsEntry struct {
	Msg    string            `json:"_msg"`
	Stream map[string]string `json:"_stream_fields"`
	Time   string            `json:"_time"`
	// All extra fields are preserved via JSON unpacking
}

// parseVictoriaLogsPayload parses the NDJSON response from VictoriaLogs.
// VictoriaLogs responds with newline-delimited JSON objects:
//
//	{"_msg":"log line","_time":"...", "_stream_fields":{...}, ...}
func parseVictoriaLogsPayload(body []byte) ([]interface{}, string) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, "no log entries returned"
	}

	entries := make([]interface{}, 0, 20)
	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			// Fallback: treat as raw text
			entries = append(entries, map[string]interface{}{"_msg": line})
			continue
		}
		entries = append(entries, obj)
	}

	if len(entries) == 0 {
		return nil, "no log entries matched"
	}
	summary := fmt.Sprintf("victorialogs returned %d log entry/entries", len(entries))
	return entries, summary
}
