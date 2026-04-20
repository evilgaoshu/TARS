package reasoning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"tars/internal/foundation/audit"
	foundationmetrics "tars/internal/foundation/metrics"
)

type SensitiveDetections struct {
	Secrets []string `json:"secrets"`
	Hosts   []string `json:"hosts"`
	IPs     []string `json:"ips"`
	Paths   []string `json:"paths"`
}

type localLLMDetector struct {
	logger  *slog.Logger
	metrics *foundationmetrics.Registry
	audit   audit.Logger
	client  *http.Client
}

func newLocalLLMDetector(logger *slog.Logger, metrics *foundationmetrics.Registry, auditLogger audit.Logger, client *http.Client) *localLLMDetector {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &localLLMDetector{
		logger:  fallbackLogger(logger),
		metrics: metrics,
		audit:   auditLogger,
		client:  client,
	}
}

func (d *localLLMDetector) DetectSensitiveValues(ctx context.Context, sessionID string, input map[string]interface{}, cfg LocalLLMAssistConfig, apiKey string) (*SensitiveDetections, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("local llm assist requires base_url and model")
	}
	if strings.TrimSpace(cfg.Mode) != "" && !strings.EqualFold(strings.TrimSpace(cfg.Mode), "detect_only") {
		return nil, fmt.Errorf("unsupported local llm assist mode: %s", strings.TrimSpace(cfg.Mode))
	}

	requestContext, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return nil, err
	}

	systemPrompt := strings.TrimSpace(`
You are a security-focused assistant helping detect sensitive literals in structured operational context.
Return only JSON with four arrays: secrets, hosts, ips, paths.
Rules:
- Return exact substrings that appear in the provided context. Do not normalize or invent values.
- "secrets" is for passwords, tokens, API keys, bearer values, credentials, cookies, or other secret material.
- "hosts" is for hostnames, domain names, node names, or internal service names that should be masked.
- "ips" is for IPv4/IPv6 literals that should be masked.
- "paths" is for filesystem paths and file names that should be masked.
- If a value is already "[REDACTED]" or a placeholder like "[HOST_1]", do not return it.
- Be conservative. If uncertain, omit the value.
`)
	userPrompt := "Detect sensitive literals in this JSON context and return the required JSON object only.\n\n" + string(requestContext)
	protocol := normalizeModelProtocol(cfg.Provider)
	invocation, err := buildModelInvocation(protocol, cfg.BaseURL, apiKey, cfg.Model, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(invocation.Payload)
	if err != nil {
		return nil, err
	}

	d.auditRequest(ctx, sessionID, cfg, invocation.Endpoint, systemPrompt, userPrompt, invocation.Payload, protocol)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, invocation.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, value := range invocation.Headers {
		req.Header.Set(key, value)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		d.recordResult("error", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	var detections SensitiveDetections
	content, err := extractModelResponseContent(protocol, resp.StatusCode, resp.Body)
	if err != nil {
		d.recordResult("error", err.Error())
		return nil, err
	}
	if err := decodeDiagnosisJSON(content, &detections); err != nil {
		d.recordResult("error", err.Error())
		return nil, err
	}
	normalized := normalizeSensitiveDetections(&detections)
	d.recordResult("success", "detect_only completed")
	d.auditResponse(ctx, sessionID, cfg, normalized, protocol)
	return normalized, nil
}

func (d *localLLMDetector) recordResult(result string, detail string) {
	if d.metrics != nil {
		d.metrics.IncExternalProvider("local_llm_desensitizer", "detect_sensitive_values", result)
		d.metrics.RecordComponentResult("desensitization_local_llm", result, detail)
	}
}

func (d *localLLMDetector) auditRequest(ctx context.Context, sessionID string, cfg LocalLLMAssistConfig, endpoint string, systemPrompt string, userPrompt string, requestPayload map[string]interface{}, protocol string) {
	if d == nil || d.audit == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	d.audit.Log(ctx, audit.Entry{
		ResourceType: "llm_request",
		ResourceID:   sessionID,
		Action:       "local_llm_desensitization_detect_send",
		Actor:        "tars_desensitization",
		Metadata: map[string]any{
			"session_id":        sessionID,
			"provider":          fallbackLocalAssistString(protocol, ModelProtocolOpenAICompatible),
			"model":             strings.TrimSpace(cfg.Model),
			"mode":              fallbackLocalAssistString(strings.TrimSpace(cfg.Mode), "detect_only"),
			"endpoint":          endpoint,
			"system_prompt_raw": systemPrompt,
			"user_prompt_raw":   userPrompt,
			"user_prompt_sent":  userPrompt,
			"request_raw":       requestPayload,
			"request_sent":      requestPayload,
			"contains_raw":      true,
			"contains_sent":     true,
			"trust_boundary":    "local_llm_only",
		},
	})
}

func (d *localLLMDetector) auditResponse(ctx context.Context, sessionID string, cfg LocalLLMAssistConfig, detections *SensitiveDetections, protocol string) {
	if d == nil || d.audit == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	d.audit.Log(ctx, audit.Entry{
		ResourceType: "llm_request",
		ResourceID:   sessionID,
		Action:       "local_llm_desensitization_detect_result",
		Actor:        "tars_desensitization",
		Metadata: map[string]any{
			"session_id":     sessionID,
			"provider":       fallbackLocalAssistString(protocol, ModelProtocolOpenAICompatible),
			"model":          strings.TrimSpace(cfg.Model),
			"mode":           fallbackLocalAssistString(strings.TrimSpace(cfg.Mode), "detect_only"),
			"detections":     detections,
			"trust_boundary": "local_llm_only",
		},
	})
}

func fallbackLocalAssistString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
