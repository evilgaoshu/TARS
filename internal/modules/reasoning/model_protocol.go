package reasoning

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	ModelProtocolOpenAICompatible = "openai_compatible"
	ModelProtocolOpenRouter       = "openrouter"
	ModelProtocolAnthropic        = "anthropic"
	ModelProtocolOllama           = "ollama"
	ModelProtocolLMStudio         = "lmstudio"
	ModelProtocolGemini           = "gemini"
)

type modelInvocation struct {
	Protocol string
	Endpoint string
	Headers  map[string]string
	Payload  map[string]interface{}
}

type anthropicMessageResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type ollamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error,omitempty"`
}

type geminiGenerateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func normalizeModelProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", "openai", "openai_compatible", "openai-compatible":
		return ModelProtocolOpenAICompatible
	case "openrouter":
		return ModelProtocolOpenRouter
	case "anthropic":
		return ModelProtocolAnthropic
	case "ollama":
		return ModelProtocolOllama
	case "lmstudio", "lm_studio", "lm-studio":
		return ModelProtocolLMStudio
	case "gemini":
		return ModelProtocolGemini
	default:
		return strings.ToLower(strings.TrimSpace(protocol))
	}
}

func buildModelInvocation(protocol string, baseURL string, apiKey string, model string, systemPrompt string, userPrompt string) (modelInvocation, error) {
	normalized := normalizeModelProtocol(protocol)
	switch normalized {
	case ModelProtocolOpenAICompatible, ModelProtocolLMStudio:
		endpoint := normalizeOpenAIEndpoint(baseURL)
		headers := map[string]string{
			"Content-Type": "application/json",
		}
		if strings.TrimSpace(apiKey) != "" {
			headers["Authorization"] = "Bearer " + strings.TrimSpace(apiKey)
		}
		return modelInvocation{
			Protocol: normalized,
			Endpoint: endpoint,
			Headers:  headers,
			Payload: map[string]interface{}{
				"model":       model,
				"temperature": 0.1,
				"messages": []map[string]string{
					{
						"role":    "system",
						"content": systemPrompt,
					},
					{
						"role":    "user",
						"content": userPrompt,
					},
				},
			},
		}, nil
	case ModelProtocolOpenRouter:
		endpoint := normalizeOpenRouterEndpoint(baseURL)
		headers := map[string]string{
			"Content-Type": "application/json",
		}
		if strings.TrimSpace(apiKey) != "" {
			headers["Authorization"] = "Bearer " + strings.TrimSpace(apiKey)
		}
		return modelInvocation{
			Protocol: normalized,
			Endpoint: endpoint,
			Headers:  headers,
			Payload: map[string]interface{}{
				"model":       model,
				"temperature": 0.1,
				"messages": []map[string]string{
					{
						"role":    "system",
						"content": systemPrompt,
					},
					{
						"role":    "user",
						"content": userPrompt,
					},
				},
			},
		}, nil
	case ModelProtocolAnthropic:
		endpoint := normalizeAnthropicEndpoint(baseURL)
		headers := map[string]string{
			"Content-Type":      "application/json",
			"anthropic-version": "2023-06-01",
		}
		if strings.TrimSpace(apiKey) != "" {
			headers["x-api-key"] = strings.TrimSpace(apiKey)
		}
		return modelInvocation{
			Protocol: normalized,
			Endpoint: endpoint,
			Headers:  headers,
			Payload: map[string]interface{}{
				"model":       model,
				"temperature": 0.1,
				"max_tokens":  1024,
				"system":      systemPrompt,
				"messages": []map[string]string{
					{
						"role":    "user",
						"content": userPrompt,
					},
				},
			},
		}, nil
	case ModelProtocolOllama:
		endpoint := normalizeOllamaEndpoint(baseURL)
		headers := map[string]string{
			"Content-Type": "application/json",
		}
		if strings.TrimSpace(apiKey) != "" {
			headers["Authorization"] = "Bearer " + strings.TrimSpace(apiKey)
		}
		return modelInvocation{
			Protocol: normalized,
			Endpoint: endpoint,
			Headers:  headers,
			Payload: map[string]interface{}{
				"model":  model,
				"stream": false,
				"messages": []map[string]string{
					{
						"role":    "system",
						"content": systemPrompt,
					},
					{
						"role":    "user",
						"content": userPrompt,
					},
				},
			},
		}, nil
	case ModelProtocolGemini:
		endpoint := normalizeGeminiEndpoint(baseURL, model, apiKey)
		return modelInvocation{
			Protocol: normalized,
			Endpoint: endpoint,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Payload: map[string]interface{}{
				"system_instruction": map[string]any{
					"parts": []map[string]string{
						{"text": systemPrompt},
					},
				},
				"contents": []map[string]any{
					{
						"role": "user",
						"parts": []map[string]string{
							{"text": userPrompt},
						},
					},
				},
				"generationConfig": map[string]any{
					"temperature": 0.1,
				},
			},
		}, nil
	default:
		return modelInvocation{}, fmt.Errorf("unsupported model protocol: %s", normalized)
	}
}

func extractModelResponseContent(protocol string, statusCode int, body io.Reader) (string, error) {
	switch normalizeModelProtocol(protocol) {
	case ModelProtocolOpenAICompatible, ModelProtocolLMStudio, ModelProtocolOpenRouter:
		var payload chatCompletionResponse
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return "", err
		}
		if statusCode >= 400 || len(payload.Choices) == 0 {
			return "", fmt.Errorf("model completion failed: status=%d", statusCode)
		}
		return payload.Choices[0].Message.Content, nil
	case ModelProtocolAnthropic:
		var payload anthropicMessageResponse
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return "", err
		}
		if statusCode >= 400 {
			if payload.Error != nil && strings.TrimSpace(payload.Error.Message) != "" {
				return "", fmt.Errorf("model completion failed: status=%d message=%s", statusCode, payload.Error.Message)
			}
			return "", fmt.Errorf("model completion failed: status=%d", statusCode)
		}
		for _, item := range payload.Content {
			if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
				return item.Text, nil
			}
		}
		return "", fmt.Errorf("model response does not contain text content")
	case ModelProtocolOllama:
		var payload ollamaChatResponse
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return "", err
		}
		if statusCode >= 400 {
			if strings.TrimSpace(payload.Error) != "" {
				return "", fmt.Errorf("model completion failed: status=%d message=%s", statusCode, payload.Error)
			}
			return "", fmt.Errorf("model completion failed: status=%d", statusCode)
		}
		if strings.TrimSpace(payload.Message.Content) == "" {
			return "", fmt.Errorf("model response does not contain message content")
		}
		return payload.Message.Content, nil
	case ModelProtocolGemini:
		var payload geminiGenerateContentResponse
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			return "", err
		}
		if statusCode >= 400 {
			if payload.Error != nil && strings.TrimSpace(payload.Error.Message) != "" {
				return "", fmt.Errorf("model completion failed: status=%d message=%s", statusCode, payload.Error.Message)
			}
			return "", fmt.Errorf("model completion failed: status=%d", statusCode)
		}
		for _, candidate := range payload.Candidates {
			for _, part := range candidate.Content.Parts {
				if strings.TrimSpace(part.Text) != "" {
					return part.Text, nil
				}
			}
		}
		return "", fmt.Errorf("model response does not contain text content")
	default:
		return "", fmt.Errorf("unsupported model protocol: %s", normalizeModelProtocol(protocol))
	}
}

func normalizeOpenAIEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	switch {
	case strings.HasSuffix(trimmed, "/chat/completions"):
		return trimmed
	case strings.HasSuffix(trimmed, "/v1"):
		return trimmed + "/chat/completions"
	default:
		return trimmed + "/v1/chat/completions"
	}
}

func normalizeAnthropicEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	switch {
	case strings.HasSuffix(trimmed, "/messages"):
		return trimmed
	case strings.HasSuffix(trimmed, "/v1"):
		return trimmed + "/messages"
	default:
		return trimmed + "/v1/messages"
	}
}

func normalizeOpenRouterEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	switch {
	case strings.HasSuffix(trimmed, "/chat/completions"):
		return trimmed
	case strings.HasSuffix(trimmed, "/api/v1"):
		return trimmed + "/chat/completions"
	default:
		return trimmed + "/api/v1/chat/completions"
	}
}

func normalizeOllamaEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	switch {
	case strings.HasSuffix(trimmed, "/api/chat"):
		return trimmed
	default:
		return trimmed + "/api/chat"
	}
}

func normalizeGeminiEndpoint(baseURL string, model string, apiKey string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	switch {
	case strings.Contains(trimmed, ":generateContent"):
		if strings.TrimSpace(apiKey) == "" || strings.Contains(trimmed, "?key=") {
			return trimmed
		}
		return trimmed + "?key=" + apiKey
	case strings.HasSuffix(trimmed, "/v1beta"):
		trimmed = trimmed + "/models/" + model + ":generateContent"
	default:
		trimmed = trimmed + "/v1beta/models/" + model + ":generateContent"
	}
	if strings.TrimSpace(apiKey) == "" {
		return trimmed
	}
	return trimmed + "?key=" + apiKey
}
