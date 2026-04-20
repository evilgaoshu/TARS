package reasoning

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"tars/internal/contracts"
)

func TestBuildDiagnosisUsesAnthropicProtocol(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://anthropic.example.test/v1/messages" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			if req.Header.Get("x-api-key") != "anthropic-secret" {
				t.Fatalf("unexpected anthropic key header: %s", req.Header.Get("x-api-key"))
			}
			if req.Header.Get("anthropic-version") != "2023-06-01" {
				t.Fatalf("unexpected anthropic version: %s", req.Header.Get("anthropic-version"))
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			payload := string(body)
			if !strings.Contains(payload, "\"system\"") || !strings.Contains(payload, "\"messages\"") {
				t.Fatalf("unexpected anthropic payload: %s", payload)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"content":[{"type":"text","text":"{\"summary\":\"anthropic summary\",\"execution_hint\":\"hostname && uptime\"}"}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Protocol: "anthropic",
		BaseURL:  "https://anthropic.example.test",
		APIKey:   "anthropic-secret",
		Model:    "claude-sonnet",
		Client:   client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-anthropic-1",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "anthropic summary" || result.ExecutionHint != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestBuildDiagnosisUsesOllamaProtocol(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "http://ollama.example.test:11434/api/chat" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			payload := string(body)
			if !strings.Contains(payload, "\"stream\":false") {
				t.Fatalf("unexpected ollama payload: %s", payload)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"message":{"content":"{\"summary\":\"ollama summary\",\"execution_hint\":\"hostname && uptime\"}"}
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Protocol: "ollama",
		BaseURL:  "http://ollama.example.test:11434",
		Model:    "qwen2.5",
		Client:   client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-ollama-1",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "ollama summary" || result.ExecutionHint != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestBuildDiagnosisUsesLMStudioWithoutExplicitV1Suffix(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "http://192.168.1.132:1234/v1/chat/completions" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"choices":[{"message":{"content":"{\"summary\":\"lmstudio summary\",\"execution_hint\":\"hostname && uptime\"}"}}]
				}`)),
			}, nil
		}),
	}

	svc := NewService(Options{
		Protocol: "lmstudio",
		BaseURL:  "http://192.168.1.132:1234",
		Model:    "qwen/qwen3-4b-2507",
		Client:   client,
	})
	result, err := svc.BuildDiagnosis(context.Background(), contracts.DiagnosisInput{
		SessionID: "ses-lmstudio-1",
		Context:   map[string]interface{}{"alert_name": "HighCPU"},
	})
	if err != nil {
		t.Fatalf("build diagnosis: %v", err)
	}
	if result.Summary != "lmstudio summary" || result.ExecutionHint != "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
