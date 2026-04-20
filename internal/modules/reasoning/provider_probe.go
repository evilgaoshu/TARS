package reasoning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ProviderModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type ProviderCheckResult struct {
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}

func ListProviderModels(ctx context.Context, entry ProviderEntry) ([]ProviderModelInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	protocol := normalizeProviderProtocol(entry.Protocol, entry.Vendor)
	req, err := newProviderListRequest(ctx, protocol, entry)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("list models failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return decodeProviderModelList(protocol, resp.Body)
}

func CheckProviderAvailability(ctx context.Context, entry ProviderEntry, model string) (ProviderCheckResult, error) {
	models, err := ListProviderModels(ctx, entry)
	if err == nil {
		if strings.TrimSpace(model) == "" {
			return ProviderCheckResult{Available: true, Detail: fmt.Sprintf("list_models ok (%d models)", len(models))}, nil
		}
		for _, item := range models {
			if item.ID == model {
				return ProviderCheckResult{Available: true, Detail: "model found in provider listing"}, nil
			}
		}
		return ProviderCheckResult{Available: false, Detail: "model not found in provider listing"}, nil
	}

	if strings.TrimSpace(model) == "" {
		return ProviderCheckResult{Available: false, Detail: err.Error()}, nil
	}

	protocol := normalizeProviderProtocol(entry.Protocol, entry.Vendor)
	invocation, buildErr := buildModelInvocation(protocol, entry.BaseURL, entry.APIKey, model, "Reply with JSON.", `{"summary":"ok","execution_hint":""}`)
	if buildErr != nil {
		return ProviderCheckResult{}, buildErr
	}
	body, marshalErr := json.Marshal(invocation.Payload)
	if marshalErr != nil {
		return ProviderCheckResult{}, marshalErr
	}
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, invocation.Endpoint, bytes.NewReader(body))
	if reqErr != nil {
		return ProviderCheckResult{}, reqErr
	}
	for key, value := range invocation.Headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, doErr := client.Do(req)
	if doErr != nil {
		return ProviderCheckResult{Available: false, Detail: doErr.Error()}, nil
	}
	defer resp.Body.Close()
	_, parseErr := extractModelResponseContent(protocol, resp.StatusCode, resp.Body)
	if parseErr != nil {
		return ProviderCheckResult{Available: false, Detail: parseErr.Error()}, nil
	}
	return ProviderCheckResult{Available: true, Detail: "minimal inference succeeded"}, nil
}

func newProviderListRequest(ctx context.Context, protocol string, entry ProviderEntry) (*http.Request, error) {
	endpoint := ""
	headers := map[string]string{}
	switch normalizeModelProtocol(protocol) {
	case ModelProtocolOpenAICompatible, ModelProtocolLMStudio:
		endpoint = strings.TrimSuffix(normalizeOpenAIEndpoint(entry.BaseURL), "/chat/completions") + "/models"
		if strings.TrimSpace(entry.APIKey) != "" {
			headers["Authorization"] = "Bearer " + strings.TrimSpace(entry.APIKey)
		}
	case ModelProtocolOpenRouter:
		endpoint = strings.TrimSuffix(normalizeOpenRouterEndpoint(entry.BaseURL), "/chat/completions") + "/models"
		if strings.TrimSpace(entry.APIKey) != "" {
			headers["Authorization"] = "Bearer " + strings.TrimSpace(entry.APIKey)
		}
	case ModelProtocolAnthropic:
		endpoint = strings.TrimSuffix(normalizeAnthropicEndpoint(entry.BaseURL), "/messages") + "/models"
		headers["anthropic-version"] = "2023-06-01"
		if strings.TrimSpace(entry.APIKey) != "" {
			headers["x-api-key"] = strings.TrimSpace(entry.APIKey)
		}
	case ModelProtocolOllama:
		endpoint = strings.TrimSuffix(normalizeOllamaEndpoint(entry.BaseURL), "/chat") + "/tags"
	case ModelProtocolGemini:
		endpoint = normalizeGeminiModelsEndpoint(entry.BaseURL, entry.APIKey)
	default:
		return nil, fmt.Errorf("unsupported provider protocol: %s", protocol)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return req, nil
}

func decodeProviderModelList(protocol string, body io.Reader) ([]ProviderModelInfo, error) {
	switch normalizeModelProtocol(protocol) {
	case ModelProtocolOpenAICompatible, ModelProtocolLMStudio, ModelProtocolOpenRouter:
		var payload struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return nil, err
		}
		output := make([]ProviderModelInfo, 0, len(payload.Data))
		for _, item := range payload.Data {
			if strings.TrimSpace(item.ID) == "" {
				continue
			}
			output = append(output, ProviderModelInfo{ID: item.ID, Name: item.ID})
		}
		return output, nil
	case ModelProtocolAnthropic:
		var payload struct {
			Data []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			} `json:"data"`
		}
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return nil, err
		}
		output := make([]ProviderModelInfo, 0, len(payload.Data))
		for _, item := range payload.Data {
			if strings.TrimSpace(item.ID) == "" {
				continue
			}
			output = append(output, ProviderModelInfo{ID: item.ID, Name: firstNonEmpty(item.DisplayName, item.ID)})
		}
		return output, nil
	case ModelProtocolOllama:
		var payload struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return nil, err
		}
		output := make([]ProviderModelInfo, 0, len(payload.Models))
		for _, item := range payload.Models {
			if strings.TrimSpace(item.Name) == "" {
				continue
			}
			output = append(output, ProviderModelInfo{ID: item.Name, Name: item.Name})
		}
		return output, nil
	case ModelProtocolGemini:
		var payload struct {
			Models []struct {
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
			} `json:"models"`
		}
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return nil, err
		}
		output := make([]ProviderModelInfo, 0, len(payload.Models))
		for _, item := range payload.Models {
			id := strings.TrimPrefix(item.Name, "models/")
			if strings.TrimSpace(id) == "" {
				continue
			}
			output = append(output, ProviderModelInfo{ID: id, Name: firstNonEmpty(item.DisplayName, id)})
		}
		return output, nil
	default:
		return nil, fmt.Errorf("unsupported provider protocol: %s", protocol)
	}
}

func normalizeGeminiModelsEndpoint(baseURL string, apiKey string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	switch {
	case strings.HasSuffix(trimmed, "/v1beta/models"):
		if strings.TrimSpace(apiKey) == "" || strings.Contains(trimmed, "?key=") {
			return trimmed
		}
		return trimmed + "?key=" + apiKey
	case strings.HasSuffix(trimmed, "/v1beta"):
		trimmed = trimmed + "/models"
	default:
		trimmed = trimmed + "/v1beta/models"
	}
	if strings.TrimSpace(apiKey) == "" {
		return trimmed
	}
	return trimmed + "?key=" + apiKey
}
