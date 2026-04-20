package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
	"tars/internal/modules/connectors"
)

type VictoriaMetricsConfig struct {
	BaseURL     string
	BearerToken string
	Provider    string
	Client      *http.Client
	Metrics     *foundationmetrics.Registry
}

type VictoriaMetricsProvider struct {
	baseURL     string
	bearerToken string
	provider    string
	client      *http.Client
	metrics     *foundationmetrics.Registry
}

type MetricsConnectorRuntime struct {
	client  *http.Client
	metrics *foundationmetrics.Registry
}

func NewVictoriaMetricsProvider(cfg VictoriaMetricsConfig) *VictoriaMetricsProvider {
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	return &VictoriaMetricsProvider{
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		bearerToken: strings.TrimSpace(cfg.BearerToken),
		provider:    firstNonEmpty(strings.TrimSpace(cfg.Provider), "victoriametrics"),
		client:      client,
		metrics:     cfg.Metrics,
	}
}

func NewMetricsConnectorRuntime(cfg VictoriaMetricsConfig) *MetricsConnectorRuntime {
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &MetricsConnectorRuntime{client: client, metrics: cfg.Metrics}
}

func (p *VictoriaMetricsProvider) Query(ctx context.Context, query contracts.MetricsQuery) (contracts.MetricsResult, error) {
	if p.baseURL == "" {
		if p.metrics != nil {
			p.metrics.IncExternalProvider(p.provider, "query", "stub")
			p.metrics.RecordComponentResult(p.provider, "stub", "base url not configured")
		}
		return contracts.MetricsResult{
			Series: []map[string]interface{}{
				{
					"service": query.Service,
					"host":    query.Host,
					"value":   1,
					"source":  "stub",
				},
			},
			Runtime: &contracts.RuntimeMetadata{
				Runtime:         "stub",
				Selection:       "fallback",
				Protocol:        "victoriametrics_http",
				FallbackEnabled: true,
				FallbackUsed:    true,
				FallbackReason:  "provider_not_configured",
				FallbackTarget:  "stub",
			},
		}, nil
	}

	if isRangeMetricsQuery(query) {
		return p.queryRange(ctx, query)
	}

	values := url.Values{}
	values.Set("query", buildMetricsQuery(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/v1/query?"+values.Encode(), nil)
	if err != nil {
		if p.metrics != nil {
			p.metrics.IncExternalProvider(p.provider, "query", "error")
			p.metrics.RecordComponentResult(p.provider, "error", err.Error())
		}
		return contracts.MetricsResult{}, err
	}
	if p.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.bearerToken)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		if p.metrics != nil {
			p.metrics.IncExternalProvider(p.provider, "query", "error")
			p.metrics.RecordComponentResult(p.provider, "error", err.Error())
		}
		return contracts.MetricsResult{}, err
	}
	defer resp.Body.Close()

	var payload instantQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		if p.metrics != nil {
			p.metrics.IncExternalProvider(p.provider, "query", "error")
			p.metrics.RecordComponentResult(p.provider, "error", err.Error())
		}
		return contracts.MetricsResult{}, err
	}
	if resp.StatusCode >= 400 || payload.Status != "success" {
		if p.metrics != nil {
			p.metrics.IncExternalProvider(p.provider, "query", "error")
			p.metrics.RecordComponentResult(p.provider, "error", fmt.Sprintf("status=%d", resp.StatusCode))
		}
		return contracts.MetricsResult{}, fmt.Errorf("victoriametrics query failed: status=%d", resp.StatusCode)
	}

	series := make([]map[string]interface{}, 0, len(payload.Data.Result))
	for _, item := range payload.Data.Result {
		row := map[string]interface{}{}
		for key, value := range item.Metric {
			row[key] = value
		}
		if len(item.Value) == 2 {
			row["timestamp"] = item.Value[0]
			row["value"] = item.Value[1]
		}
		series = append(series, row)
	}

	if p.metrics != nil {
		p.metrics.IncExternalProvider(p.provider, "query", "success")
		p.metrics.RecordComponentResult(p.provider, "success", "query succeeded")
	}
	return contracts.MetricsResult{Series: series, Runtime: &contracts.RuntimeMetadata{
		Runtime:         "legacy_provider",
		Selection:       "fallback",
		Protocol:        p.provider,
		FallbackEnabled: true,
		FallbackTarget:  "stub",
	}}, nil
}

func (r *MetricsConnectorRuntime) Query(ctx context.Context, manifest connectors.Manifest, query contracts.MetricsQuery) (contracts.MetricsResult, error) {
	baseURL := strings.TrimSpace(manifest.Config.Values["base_url"])
	if baseURL == "" {
		return contracts.MetricsResult{}, fmt.Errorf("connector base_url is not configured")
	}
	bearerToken := strings.TrimSpace(manifest.Config.Values["bearer_token"])
	providerName := "prometheus"
	switch strings.TrimSpace(manifest.Spec.Protocol) {
	case "victoriametrics_http":
		providerName = "victoriametrics"
	case "prometheus_http":
		providerName = "prometheus"
	default:
		return contracts.MetricsResult{}, fmt.Errorf("connector does not support metrics runtime")
	}
	provider := NewVictoriaMetricsProvider(VictoriaMetricsConfig{
		BaseURL:     baseURL,
		BearerToken: bearerToken,
		Provider:    providerName,
		Client:      r.client,
		Metrics:     r.metrics,
	})
	return provider.Query(ctx, query)
}

func (p *VictoriaMetricsProvider) queryRange(ctx context.Context, query contracts.MetricsQuery) (contracts.MetricsResult, error) {
	values := url.Values{}
	values.Set("query", buildMetricsQuery(query))
	start, end, step := normalizeRangeQuery(query)
	values.Set("start", start.Format(time.RFC3339))
	values.Set("end", end.Format(time.RFC3339))
	values.Set("step", step)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/v1/query_range?"+values.Encode(), nil)
	if err != nil {
		if p.metrics != nil {
			p.metrics.IncExternalProvider(p.provider, "query_range", "error")
			p.metrics.RecordComponentResult(p.provider, "error", err.Error())
		}
		return contracts.MetricsResult{}, err
	}
	if p.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.bearerToken)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		if p.metrics != nil {
			p.metrics.IncExternalProvider(p.provider, "query_range", "error")
			p.metrics.RecordComponentResult(p.provider, "error", err.Error())
		}
		return contracts.MetricsResult{}, err
	}
	defer resp.Body.Close()

	var payload rangeQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		if p.metrics != nil {
			p.metrics.IncExternalProvider(p.provider, "query_range", "error")
			p.metrics.RecordComponentResult(p.provider, "error", err.Error())
		}
		return contracts.MetricsResult{}, err
	}
	if resp.StatusCode >= 400 || payload.Status != "success" {
		if p.metrics != nil {
			p.metrics.IncExternalProvider(p.provider, "query_range", "error")
			p.metrics.RecordComponentResult(p.provider, "error", fmt.Sprintf("status=%d", resp.StatusCode))
		}
		return contracts.MetricsResult{}, fmt.Errorf("victoriametrics query_range failed: status=%d", resp.StatusCode)
	}

	series := make([]map[string]interface{}, 0, len(payload.Data.Result))
	for _, item := range payload.Data.Result {
		row := map[string]interface{}{}
		for key, value := range item.Metric {
			row[key] = value
		}
		row["values"] = item.Values
		series = append(series, row)
	}

	if p.metrics != nil {
		p.metrics.IncExternalProvider(p.provider, "query_range", "success")
		p.metrics.RecordComponentResult(p.provider, "success", "query_range succeeded")
	}
	return contracts.MetricsResult{Series: series, Runtime: &contracts.RuntimeMetadata{
		Runtime:         "legacy_provider",
		Selection:       "fallback",
		Protocol:        p.provider,
		FallbackEnabled: true,
		FallbackTarget:  "stub",
	}}, nil
}

func (r *MetricsConnectorRuntime) CheckHealth(ctx context.Context, manifest connectors.Manifest) (string, string, error) {
	baseURL := strings.TrimSpace(manifest.Config.Values["base_url"])
	if baseURL == "" {
		return "unhealthy", "metrics connector base_url is not configured", fmt.Errorf("connector base_url is not configured")
	}
	checkURL := strings.TrimRight(baseURL, "/") + "/api/v1/query?query=up"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return "unhealthy", formatHTTPProbeRequestError("metrics connector", err), err
	}
	if bearerToken := strings.TrimSpace(manifest.Config.Values["bearer_token"]); bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	client := r.client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "unhealthy", formatHTTPProbeTransportError("metrics connector", err), err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "unhealthy", fmt.Sprintf("metrics connector health probe returned status=%d", resp.StatusCode), fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload instantQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "unhealthy", "metrics connector health probe returned invalid JSON", err
	}
	if payload.Status != "success" {
		return "unhealthy", fmt.Sprintf("metrics connector health probe failed: status=%s", payload.Status) + " (status should be 'success')", fmt.Errorf("api_status=%s", payload.Status)
	}

	return "healthy", "metrics connector health probe succeeded", nil
}


func buildMetricsQuery(query contracts.MetricsQuery) string {
	if strings.TrimSpace(query.Query) != "" {
		return strings.TrimSpace(query.Query)
	}
	switch {
	case query.Host != "":
		return fmt.Sprintf(`up{instance=%q}`, query.Host)
	case query.Service != "":
		return fmt.Sprintf(`up{job=~".*%s.*"}`, escapeRegexValue(query.Service))
	default:
		return "up"
	}
}

func isRangeMetricsQuery(query contracts.MetricsQuery) bool {
	mode := strings.ToLower(strings.TrimSpace(query.Mode))
	return mode == "range" || !query.Start.IsZero() || !query.End.IsZero() || strings.TrimSpace(query.Step) != "" || strings.TrimSpace(query.Window) != ""
}

func normalizeRangeQuery(query contracts.MetricsQuery) (time.Time, time.Time, string) {
	end := query.End.UTC()
	if end.IsZero() {
		end = time.Now().UTC()
	}
	start := query.Start.UTC()
	if start.IsZero() {
		window := strings.TrimSpace(query.Window)
		if window == "" {
			window = "1h"
		}
		if duration, err := time.ParseDuration(window); err == nil && duration > 0 {
			start = end.Add(-duration)
		}
	}
	if start.IsZero() || !start.Before(end) {
		start = end.Add(-time.Hour)
	}
	step := strings.TrimSpace(query.Step)
	if step == "" {
		step = "60s"
	}
	return start, end, step
}

func escapeRegexValue(value string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		".", `\.`,
		"+", `\+`,
		"*", `\*`,
		"?", `\?`,
		"(", `\(`,
		")", `\)`,
		"[", `\[`,
		"]", `\]`,
		"{", `\{`,
		"}", `\}`,
		"|", `\|`,
		"^", `\^`,
		"$", `\$`,
	)
	return replacer.Replace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type instantQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type rangeQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		} `json:"result"`
	} `json:"data"`
}
