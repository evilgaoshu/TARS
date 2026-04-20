package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"tars/internal/modules/connectors"
)

func TestDeliveryRuntimeInvokeUsesRemoteGitHubWhenConfigured(t *testing.T) {
	t.Parallel()

	var capturedURL string
	runtime := NewDeliveryRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedURL = req.URL.String()
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`[
					{
						"sha":"abc123",
						"html_url":"https://github.com/evilgaoshu/TARS/commit/abc123",
						"commit":{
							"message":"fix api release regression",
							"author":{"name":"dev","date":"2026-03-20T12:00:00Z"}
						}
					}
				]`)),
			}, nil
		}),
	})

	manifest := connectors.Manifest{}
	manifest.Metadata.ID = "delivery-main"
	manifest.Spec.Protocol = "delivery_github"
	manifest.Config.Values = map[string]string{
		"repo_url": "https://github.com/evilgaoshu/TARS.git",
	}

	result, err := runtime.Invoke(context.Background(), manifest, "delivery.query", map[string]interface{}{"service": "api"})
	if err != nil {
		t.Fatalf("invoke delivery github: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed result, got %+v", result)
	}
	if !strings.Contains(capturedURL, "/repos/evilgaoshu/TARS/commits") {
		t.Fatalf("expected GitHub commits endpoint, got %q", capturedURL)
	}
	if got := result.Output["source"]; got != "delivery_github" {
		t.Fatalf("expected github source, got %#v", got)
	}
	if got := result.Output["result_count"]; got != 1 {
		t.Fatalf("expected one commit, got %#v", got)
	}
}

func TestFilterLogLinesPrefersNonAuditMatches(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		`{"time":"2026-03-20T13:49:39Z","level":"INFO","msg":"audit event","component":"audit","metadata":{"service":"api"}}`,
		`{"time":"2026-03-20T13:49:40Z","level":"ERROR","service":"api","message":"api request failed with database timeout"}`,
	}, "\n")

	results := filterLogLines(content, []string{"api"}, 5)
	if len(results) != 1 {
		t.Fatalf("expected one preferred result, got %+v", results)
	}
	if !strings.Contains(results[0], "database timeout") {
		t.Fatalf("expected non-audit error line, got %q", results[0])
	}
}

func TestFilterLogLinesKeepsAuditWhenExplicitlyRequested(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		`{"time":"2026-03-20T13:49:39Z","level":"INFO","msg":"audit event","component":"audit","metadata":{"service":"api"}}`,
		`{"time":"2026-03-20T13:49:40Z","level":"ERROR","service":"api","message":"api request failed with database timeout"}`,
	}, "\n")

	results := filterLogLines(content, []string{"audit"}, 5)
	if len(results) == 0 {
		t.Fatalf("expected audit result")
	}
	if !strings.Contains(strings.ToLower(results[0]), "audit") {
		t.Fatalf("expected audit line when explicitly requested, got %q", results[0])
	}
}

func TestObservabilityHTTPRuntimeParsesVMAlertAlertsAndArtifacts(t *testing.T) {
	t.Parallel()

	runtime := NewObservabilityHTTPRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/v1/alerts" {
				t.Fatalf("unexpected path %q", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"status":"success",
					"data":{"alerts":[
						{"name":"DiskWillFillSoon","state":"firing","labels":{"service":"api"}},
						{"name":"HighCPU","state":"inactive","labels":{"service":"worker"}}
					]}
				}`)),
			}, nil
		}),
	})

	manifest := connectors.Manifest{}
	manifest.Metadata.ID = "observability-main"
	manifest.Spec.Protocol = "observability_http"
	manifest.Config.Values = map[string]string{
		"base_url":    "http://127.0.0.1:8880",
		"alerts_path": "/api/v1/alerts",
		"rules_path":  "/api/v1/rules",
	}

	result, err := runtime.Invoke(context.Background(), manifest, "observability.query", map[string]interface{}{"mode": "alerts", "query": "disk"})
	if err != nil {
		t.Fatalf("invoke observability http: %v", err)
	}
	if got := result.Output["result_count"]; got != 1 {
		t.Fatalf("expected filtered alert count, got %#v", got)
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected one artifact, got %+v", result.Artifacts)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.Artifacts[0].Content), &payload); err != nil {
		t.Fatalf("decode artifact payload: %v", err)
	}
	if strings.TrimSpace(payload["summary"].(string)) == "" {
		t.Fatalf("expected artifact summary, got %+v", payload)
	}
	if got := result.Output["source"]; got != "observability_http" {
		t.Fatalf("expected observability_http source, got %#v", got)
	}
}

func TestDeliveryRuntimeInvokeFallsBackToLatestGitHubCommitsWhenFilterMisses(t *testing.T) {
	t.Parallel()

	runtime := NewDeliveryRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`[
					{
						"sha":"abc123",
						"html_url":"https://github.com/VictoriaMetrics/VictoriaMetrics/commit/abc123",
						"commit":{"message":"release vmui changes","author":{"name":"dev","date":"2026-03-20T12:00:00Z"}}
					}
				]`)),
			}, nil
		}),
	})

	manifest := connectors.Manifest{}
	manifest.Metadata.ID = "delivery-main"
	manifest.Spec.Protocol = "delivery_github"
	manifest.Config.Values = map[string]string{
		"repo_url": "https://github.com/VictoriaMetrics/VictoriaMetrics.git",
	}

	result, err := runtime.Invoke(context.Background(), manifest, "delivery.query", map[string]interface{}{"service": "api"})
	if err != nil {
		t.Fatalf("invoke delivery github: %v", err)
	}
	if got := result.Output["result_count"]; got != 1 {
		t.Fatalf("expected fallback commit result, got %#v", got)
	}
	summary, ok := result.Output["summary"].(string)
	if !ok || !strings.Contains(summary, "fallback") {
		t.Fatalf("expected fallback summary, got %#v", result.Output["summary"])
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected delivery artifact, got %+v", result.Artifacts)
	}
}
