package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
	"tars/internal/modules/connectors"
)

func TestNewMetricsConnectorRuntime(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
	providedClient := &http.Client{Timeout: 3 * time.Second}

	runtime := NewMetricsConnectorRuntime(VictoriaMetricsConfig{
		Client:  providedClient,
		Metrics: registry,
	})
	if runtime.client != providedClient {
		t.Fatalf("expected provided client to be reused")
	}
	if runtime.metrics != registry {
		t.Fatalf("expected provided metrics registry to be reused")
	}

	fallback := NewMetricsConnectorRuntime(VictoriaMetricsConfig{})
	if fallback.client == nil {
		t.Fatalf("expected default client to be initialized")
	}
	if got := fallback.client.Timeout; got != 15*time.Second {
		t.Fatalf("expected default timeout 15s, got %s", got)
	}
}

func TestMetricsConnectorRuntimeQuery(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		protocol     string
		wantProtocol string
	}{
		{name: "prometheus", protocol: "prometheus_http", wantProtocol: "prometheus"},
		{name: "victoriametrics", protocol: "victoriametrics_http", wantProtocol: "victoriametrics"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/query" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				if got := r.URL.Query().Get("query"); got != `up{instance="node-1"}` {
					t.Fatalf("unexpected query: %s", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"status":"success",
					"data":{"result":[{"metric":{"instance":"node-1"},"value":[1710000000,"1"]}]}
				}`))
			}))
			t.Cleanup(server.Close)

			runtime := NewMetricsConnectorRuntime(VictoriaMetricsConfig{Client: server.Client()})
			result, err := runtime.Query(context.Background(), connectors.Manifest{
				Spec: connectors.Spec{Protocol: tc.protocol},
				Config: connectors.RuntimeConfig{
					Values: map[string]string{"base_url": server.URL},
				},
			}, contracts.MetricsQuery{Host: "node-1"})
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			if result.Runtime == nil || result.Runtime.Protocol != tc.wantProtocol {
				t.Fatalf("unexpected runtime metadata: %+v", result.Runtime)
			}
			if len(result.Series) != 1 {
				t.Fatalf("unexpected series: %+v", result.Series)
			}
			if got := result.Series[0]["instance"]; got != "node-1" {
				t.Fatalf("unexpected series row: %+v", result.Series[0])
			}
		})
	}
}

func TestMetricsConnectorRuntimeQueryForwardsCustomClient(t *testing.T) {
	t.Parallel()

	called := false
	runtime := NewMetricsConnectorRuntime(VictoriaMetricsConfig{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				called = true
				if req.URL.String() != "https://vm.example.test/api/v1/query?query=up" {
					t.Fatalf("unexpected request url: %s", req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"status":"success","data":{"result":[]}}`)),
				}, nil
			}),
		},
	})

	_, err := runtime.Query(context.Background(), connectors.Manifest{
		Spec: connectors.Spec{Protocol: "victoriametrics_http"},
		Config: connectors.RuntimeConfig{
			Values: map[string]string{"base_url": "https://vm.example.test"},
		},
	}, contracts.MetricsQuery{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !called {
		t.Fatalf("expected runtime custom client transport to be called")
	}
}

func TestMetricsConnectorRuntimeQueryErrors(t *testing.T) {
	t.Parallel()

	runtime := NewMetricsConnectorRuntime(VictoriaMetricsConfig{})

	if _, err := runtime.Query(context.Background(), connectors.Manifest{}, contracts.MetricsQuery{}); err == nil || !strings.Contains(err.Error(), "connector base_url is not configured") {
		t.Fatalf("expected missing base_url error, got %v", err)
	}

	if _, err := runtime.Query(context.Background(), connectors.Manifest{
		Spec: connectors.Spec{Protocol: "unsupported"},
		Config: connectors.RuntimeConfig{
			Values: map[string]string{"base_url": "https://vm.example.test"},
		},
	}, contracts.MetricsQuery{}); err == nil || !strings.Contains(err.Error(), "connector does not support metrics runtime") {
		t.Fatalf("expected unsupported protocol error, got %v", err)
	}
}

func TestVictoriaMetricsProviderQueryRange(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
	registry := foundationmetrics.New()
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query_range" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		values := r.URL.Query()
		if got := values.Get("query"); got != `up{job=~".*api\.v1\+\(core\).*"}` {
			t.Fatalf("unexpected query: %s", got)
		}
		if got := values.Get("start"); got != start.Format(time.RFC3339) {
			t.Fatalf("unexpected start: %s", got)
		}
		if got := values.Get("end"); got != end.Format(time.RFC3339) {
			t.Fatalf("unexpected end: %s", got)
		}
		if got := values.Get("step"); got != "30s" {
			t.Fatalf("unexpected step: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"success",
			"data":{"result":[{"metric":{"service":"api"},"values":[[1712044800,"1"],[1712044830,"2"]]}]}
		}`))
	}))
	t.Cleanup(server.Close)

	provider := NewVictoriaMetricsProvider(VictoriaMetricsConfig{
		BaseURL:     server.URL,
		BearerToken: "range-token",
		Client:      server.Client(),
		Metrics:     registry,
	})
	result, err := provider.Query(context.Background(), contracts.MetricsQuery{
		Service: "api.v1+(core)",
		Mode:    "range",
		Start:   start,
		End:     end,
		Step:    "30s",
	})
	if err != nil {
		t.Fatalf("query range: %v", err)
	}
	if result.Runtime == nil || result.Runtime.Runtime != "legacy_provider" {
		t.Fatalf("unexpected runtime metadata: %+v", result.Runtime)
	}
	if len(result.Series) != 1 {
		t.Fatalf("unexpected series: %+v", result.Series)
	}
	values, ok := result.Series[0]["values"].([][]interface{})
	if !ok {
		t.Fatalf("expected values to be preserved, got %#v", result.Series[0]["values"])
	}
	if len(values) != 2 {
		t.Fatalf("unexpected values: %+v", values)
	}
	if gotAuth != "Bearer range-token" {
		t.Fatalf("unexpected authorization header: %q", gotAuth)
	}

	var output strings.Builder
	if err := registry.WritePrometheus(&output); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	if !strings.Contains(output.String(), `tars_external_provider_requests_total{operation="query_range",provider="victoriametrics",result="success"} 1`) {
		t.Fatalf("expected query_range success metric, got:\n%s", output.String())
	}
}

func TestVictoriaMetricsProviderQueryRangeErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		cfg        VictoriaMetricsConfig
		query      contracts.MetricsQuery
		wantErr    string
		wantMetric string
	}{
		{
			name:       "invalid_request_url",
			cfg:        VictoriaMetricsConfig{BaseURL: "http://%"},
			query:      contracts.MetricsQuery{Mode: "range"},
			wantErr:    "invalid",
			wantMetric: `tars_external_provider_requests_total{operation="query_range",provider="victoriametrics",result="error"}`,
		},
		{
			name: "transport_error",
			cfg: VictoriaMetricsConfig{
				BaseURL: "https://vm.example.test",
				Client: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return nil, errors.New("boom")
					}),
				},
			},
			query:      contracts.MetricsQuery{Mode: "range"},
			wantErr:    "boom",
			wantMetric: `tars_external_provider_requests_total{operation="query_range",provider="victoriametrics",result="error"}`,
		},
		{
			name: "decode_error",
			cfg: VictoriaMetricsConfig{
				BaseURL: "https://vm.example.test",
				Client: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader("not-json")),
						}, nil
					}),
				},
			},
			query:      contracts.MetricsQuery{Mode: "range"},
			wantErr:    "invalid character",
			wantMetric: `tars_external_provider_requests_total{operation="query_range",provider="victoriametrics",result="error"}`,
		},
		{
			name: "status_error",
			cfg: VictoriaMetricsConfig{
				BaseURL: "https://vm.example.test",
				Client: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusServiceUnavailable,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader(`{"status":"success","data":{"result":[]}}`)),
						}, nil
					}),
				},
			},
			query:      contracts.MetricsQuery{Mode: "range"},
			wantErr:    "victoriametrics query_range failed",
			wantMetric: `tars_external_provider_requests_total{operation="query_range",provider="victoriametrics",result="error"}`,
		},
		{
			name: "payload_status_error",
			cfg: VictoriaMetricsConfig{
				BaseURL: "https://vm.example.test",
				Client: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader(`{"status":"error","data":{"result":[]}}`)),
						}, nil
					}),
				},
			},
			query:      contracts.MetricsQuery{Mode: "range"},
			wantErr:    "victoriametrics query_range failed",
			wantMetric: `tars_external_provider_requests_total{operation="query_range",provider="victoriametrics",result="error"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			registry := foundationmetrics.New()
			tc.cfg.Metrics = registry
			provider := NewVictoriaMetricsProvider(tc.cfg)
			_, err := provider.Query(context.Background(), tc.query)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q error, got %v", tc.wantErr, err)
			}
			var output strings.Builder
			if err := registry.WritePrometheus(&output); err != nil {
				t.Fatalf("write metrics: %v", err)
			}
			if !strings.Contains(output.String(), tc.wantMetric) {
				t.Fatalf("expected error metric, got:\n%s", output.String())
			}
		})
	}
}

func TestVictoriaMetricsProviderQueryStubRecordsMetrics(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
	provider := NewVictoriaMetricsProvider(VictoriaMetricsConfig{
		Metrics: registry,
	})

	result, err := provider.Query(context.Background(), contracts.MetricsQuery{
		Service: "api",
		Host:    "host-1",
	})
	if err != nil {
		t.Fatalf("query vm: %v", err)
	}
	if result.Runtime == nil || result.Runtime.Runtime != "stub" || !result.Runtime.FallbackUsed {
		t.Fatalf("unexpected runtime metadata: %+v", result.Runtime)
	}
	if len(result.Series) != 1 || result.Series[0]["source"] != "stub" {
		t.Fatalf("unexpected series: %+v", result.Series)
	}

	var output strings.Builder
	if err := registry.WritePrometheus(&output); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	if !strings.Contains(output.String(), `tars_external_provider_requests_total{operation="query",provider="victoriametrics",result="stub"} 1`) {
		t.Fatalf("expected stub provider metric, got:\n%s", output.String())
	}
}

func TestVictoriaMetricsProviderQueryErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		cfg        VictoriaMetricsConfig
		query      contracts.MetricsQuery
		wantErr    string
		wantMetric string
	}{
		{
			name:       "invalid_request_url",
			cfg:        VictoriaMetricsConfig{BaseURL: "http://%"},
			query:      contracts.MetricsQuery{Host: "node-1"},
			wantErr:    "invalid",
			wantMetric: `tars_external_provider_requests_total{operation="query",provider="victoriametrics",result="error"}`,
		},
		{
			name: "transport_error",
			cfg: VictoriaMetricsConfig{
				BaseURL: "https://vm.example.test",
				Client: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return nil, errors.New("boom")
					}),
				},
			},
			query:      contracts.MetricsQuery{Host: "node-1"},
			wantErr:    "boom",
			wantMetric: `tars_external_provider_requests_total{operation="query",provider="victoriametrics",result="error"}`,
		},
		{
			name: "decode_error",
			cfg: VictoriaMetricsConfig{
				BaseURL: "https://vm.example.test",
				Client: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader("not-json")),
						}, nil
					}),
				},
			},
			query:      contracts.MetricsQuery{Host: "node-1"},
			wantErr:    "invalid character",
			wantMetric: `tars_external_provider_requests_total{operation="query",provider="victoriametrics",result="error"}`,
		},
		{
			name: "status_error",
			cfg: VictoriaMetricsConfig{
				BaseURL: "https://vm.example.test",
				Client: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusServiceUnavailable,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader(`{"status":"success","data":{"result":[]}}`)),
						}, nil
					}),
				},
			},
			query:      contracts.MetricsQuery{Host: "node-1"},
			wantErr:    "victoriametrics query failed",
			wantMetric: `tars_external_provider_requests_total{operation="query",provider="victoriametrics",result="error"}`,
		},
		{
			name: "payload_status_error",
			cfg: VictoriaMetricsConfig{
				BaseURL: "https://vm.example.test",
				Client: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader(`{"status":"error","data":{"result":[]}}`)),
						}, nil
					}),
				},
			},
			query:      contracts.MetricsQuery{Host: "node-1"},
			wantErr:    "victoriametrics query failed",
			wantMetric: `tars_external_provider_requests_total{operation="query",provider="victoriametrics",result="error"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			registry := foundationmetrics.New()
			tc.cfg.Metrics = registry
			provider := NewVictoriaMetricsProvider(tc.cfg)
			_, err := provider.Query(context.Background(), tc.query)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q error, got %v", tc.wantErr, err)
			}
			var output strings.Builder
			if err := registry.WritePrometheus(&output); err != nil {
				t.Fatalf("write metrics: %v", err)
			}
			if !strings.Contains(output.String(), tc.wantMetric) {
				t.Fatalf("expected error metric, got:\n%s", output.String())
			}
		})
	}
}

func TestMetricsConnectorRuntimeCheckHealth(t *testing.T) {
	t.Parallel()

	t.Run("missing_base_url", func(t *testing.T) {
		runtime := NewMetricsConnectorRuntime(VictoriaMetricsConfig{})
		status, summary, err := runtime.CheckHealth(context.Background(), connectors.Manifest{})
		if status != "unhealthy" {
			t.Fatalf("unexpected status: %s", status)
		}
		if summary != "metrics connector base_url is not configured" {
			t.Fatalf("unexpected summary: %s", summary)
		}
		if err == nil || !strings.Contains(err.Error(), "connector base_url is not configured") {
			t.Fatalf("expected base_url error, got %v", err)
		}
	})

	t.Run("healthy", func(t *testing.T) {
		var gotAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			if r.URL.Path != "/api/v1/query" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if got := r.URL.Query().Get("query"); got != "up" {
				t.Fatalf("unexpected query: %s", got)
			}
			_, _ = w.Write([]byte(`{"status":"success","data":{"result":[]}}`))
		}))
		t.Cleanup(server.Close)

		runtime := NewMetricsConnectorRuntime(VictoriaMetricsConfig{Client: server.Client()})
		status, summary, err := runtime.CheckHealth(context.Background(), connectors.Manifest{
			Config: connectors.RuntimeConfig{
				Values: map[string]string{
					"base_url":     server.URL,
					"bearer_token": "secret",
				},
			},
		})
		if status != "healthy" {
			t.Fatalf("unexpected status: %s", status)
		}
		if summary != "metrics connector health probe succeeded" {
			t.Fatalf("unexpected summary: %s", summary)
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotAuth != "Bearer secret" {
			t.Fatalf("unexpected auth header: %q", gotAuth)
		}
	})

	t.Run("unhealthy_status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "down", http.StatusServiceUnavailable)
		}))
		t.Cleanup(server.Close)

		runtime := NewMetricsConnectorRuntime(VictoriaMetricsConfig{Client: server.Client()})
		status, summary, err := runtime.CheckHealth(context.Background(), connectors.Manifest{
			Config: connectors.RuntimeConfig{
				Values: map[string]string{"base_url": server.URL},
			},
		})
		if status != "unhealthy" {
			t.Fatalf("unexpected status: %s", status)
		}
		if !strings.Contains(summary, "status=503") {
			t.Fatalf("unexpected summary: %s", summary)
		}
		if err == nil || !strings.Contains(err.Error(), "status=503") {
			t.Fatalf("expected status error, got %v", err)
		}
	})

	t.Run("request_error", func(t *testing.T) {
		runtime := NewMetricsConnectorRuntime(VictoriaMetricsConfig{
			Client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("boom")
				}),
			},
		})
		status, summary, err := runtime.CheckHealth(context.Background(), connectors.Manifest{
			Config: connectors.RuntimeConfig{
				Values: map[string]string{"base_url": "https://vm.example.test"},
			},
		})
		if status != "unhealthy" {
			t.Fatalf("unexpected status: %s", status)
		}
		if summary != "metrics connector health probe failed: boom" {
			t.Fatalf("unexpected summary: %s", summary)
		}
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("expected transport error, got %v", err)
		}
	})

	t.Run("invalid_base_url", func(t *testing.T) {
		runtime := NewMetricsConnectorRuntime(VictoriaMetricsConfig{})
		status, summary, err := runtime.CheckHealth(context.Background(), connectors.Manifest{
			Config: connectors.RuntimeConfig{
				Values: map[string]string{"base_url": "http://[::1"},
			},
		})
		if status != "unhealthy" {
			t.Fatalf("unexpected status: %s", status)
		}
		if !strings.Contains(summary, "base_url is invalid") {
			t.Fatalf("unexpected summary: %s", summary)
		}
		if err == nil || !strings.Contains(err.Error(), "missing ']'") {
			t.Fatalf("expected invalid url error, got %v", err)
		}
	})
}

func TestBuildMetricsQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		query contracts.MetricsQuery
		want  string
	}{
		{name: "query", query: contracts.MetricsQuery{Query: "  sum(up)  "}, want: "sum(up)"},
		{name: "host", query: contracts.MetricsQuery{Host: "node-1"}, want: `up{instance="node-1"}`},
		{name: "service", query: contracts.MetricsQuery{Service: `api.v1+(core)`}, want: `up{job=~".*api\.v1\+\(core\).*"}`},
		{name: "default", query: contracts.MetricsQuery{}, want: "up"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildMetricsQuery(tc.query); got != tc.want {
				t.Fatalf("unexpected query: %s", got)
			}
		})
	}
}

func TestNormalizeRangeQuery(t *testing.T) {
	t.Parallel()

	t.Run("window_from_now", func(t *testing.T) {
		before := time.Now().UTC()
		start, end, step := normalizeRangeQuery(contracts.MetricsQuery{Window: "1h"})
		after := time.Now().UTC()

		if end.Before(before) || end.After(after) {
			t.Fatalf("expected end to be computed at call time, got %s not in [%s, %s]", end, before, after)
		}
		if !start.Add(time.Hour).Equal(end) {
			t.Fatalf("expected one hour window, got start=%s end=%s", start, end)
		}
		if step != "60s" {
			t.Fatalf("unexpected default step: %s", step)
		}
	})

	t.Run("fallback_when_window_invalid_or_start_after_end", func(t *testing.T) {
		end := time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
		start, gotEnd, step := normalizeRangeQuery(contracts.MetricsQuery{
			Start:  end.Add(time.Hour),
			End:    end,
			Window: "bogus",
			Step:   "15s",
		})

		if !gotEnd.Equal(end) {
			t.Fatalf("unexpected end: %s", gotEnd)
		}
		if !start.Equal(end.Add(-time.Hour)) {
			t.Fatalf("expected fallback to one hour, got %s", start)
		}
		if step != "15s" {
			t.Fatalf("unexpected step: %s", step)
		}
	})
}

func TestEscapeRegexValue(t *testing.T) {
	t.Parallel()

	got := escapeRegexValue(`a\\b.c+d*e?f(g)h[i]j{k}l|m^n$o`)
	want := `a\\\\b\.c\+d\*e\?f\(g\)h\[i\]j\{k\}l\|m\^n\$o`
	if got != want {
		t.Fatalf("unexpected escaped value: %s", got)
	}
}
