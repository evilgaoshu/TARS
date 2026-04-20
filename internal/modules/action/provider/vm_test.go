package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
)

func TestQueryBuildsVMRequest(t *testing.T) {
	t.Parallel()

	var capturedURL string
	var capturedAuth string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedURL = req.URL.String()
			capturedAuth = req.Header.Get("Authorization")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"status":"success",
					"data":{"result":[{"metric":{"instance":"host-1"},"value":[1710000000,"1"]}]}
				}`)),
			}, nil
		}),
	}

	provider := NewVictoriaMetricsProvider(VictoriaMetricsConfig{
		BaseURL:     "https://vm.example.test",
		BearerToken: "secret-token",
		Provider:    "prometheus",
		Client:      client,
	})

	result, err := provider.Query(context.Background(), contracts.MetricsQuery{Host: "host-1"})
	if err != nil {
		t.Fatalf("query vm: %v", err)
	}
	if !strings.Contains(capturedURL, "query=up%7Binstance%3D%22host-1%22%7D") {
		t.Fatalf("unexpected request url: %s", capturedURL)
	}
	if capturedAuth != "Bearer secret-token" {
		t.Fatalf("unexpected authorization header: %q", capturedAuth)
	}
	if len(result.Series) != 1 || result.Series[0]["instance"] != "host-1" {
		t.Fatalf("unexpected series: %+v", result.Series)
	}
}

func TestQueryFallsBackToStubWhenBaseURLMissing(t *testing.T) {
	t.Parallel()

	provider := NewVictoriaMetricsProvider(VictoriaMetricsConfig{})
	result, err := provider.Query(context.Background(), contracts.MetricsQuery{
		Service: "api",
		Host:    "host-1",
	})
	if err != nil {
		t.Fatalf("query vm: %v", err)
	}
	if len(result.Series) != 1 || result.Series[0]["source"] != "stub" {
		t.Fatalf("unexpected series: %+v", result.Series)
	}
}

func TestQueryRecordsMetrics(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"status":"success",
					"data":{"result":[{"metric":{"instance":"host-1"},"value":[1710000000,"1"]}]}
				}`)),
			}, nil
		}),
	}

	provider := NewVictoriaMetricsProvider(VictoriaMetricsConfig{
		BaseURL: "https://vm.example.test",
		Client:  client,
		Metrics: registry,
	})

	if _, err := provider.Query(context.Background(), contracts.MetricsQuery{Host: "host-1"}); err != nil {
		t.Fatalf("query vm: %v", err)
	}

	var output strings.Builder
	if err := registry.WritePrometheus(&output); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	if !strings.Contains(output.String(), `tars_external_provider_requests_total{operation="query",provider="victoriametrics",result="success"} 1`) {
		t.Fatalf("expected provider metric, got:\n%s", output.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
