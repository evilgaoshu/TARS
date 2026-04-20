package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tars/internal/foundation/audit"
	"tars/internal/foundation/config"
	"tars/internal/foundation/logger"
	foundationmetrics "tars/internal/foundation/metrics"
	"tars/internal/foundation/observability"
	"tars/internal/modules/alertintake"
	"tars/internal/modules/knowledge"
	"tars/internal/modules/workflow"
)

func TestRegisterPublicRoutesReturns404ForUnknownAPIPaths(t *testing.T) {
	t.Parallel()

	deps := Dependencies{
		Config: config.Config{
			OpsAPI: config.OpsAPIConfig{
				Enabled: true,
				Token:   "ops-token",
			},
		},
		Logger:      logger.New("INFO"),
		AlertIngest: alertintake.NewService(),
		Workflow:    workflow.NewService(workflow.Options{DiagnosisEnabled: true}),
		Knowledge:   knowledge.NewService(),
		Audit:       audit.NewNoop(),
	}

	mux := http.NewServeMux()
	RegisterPublicRoutes(mux, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected public router to return 404 for unknown api path, got %d", recorder.Code)
	}
}

func TestRegisterOpsRoutesExposesOpsEndpoints(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	mux := http.NewServeMux()
	RegisterOpsRoutes(mux, Dependencies{
		Config:    defaultTestConfig(),
		Logger:    logger.New("INFO"),
		Workflow:  system.workflow,
		Knowledge: knowledge.NewService(),
		Audit:     audit.NewNoop(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer ops-token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ops router to expose sessions endpoint, got %d", recorder.Code)
	}
}

func TestRegisterOpsRoutesExposePlatformDiscovery(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	mux := http.NewServeMux()
	RegisterOpsRoutes(mux, Dependencies{
		Config:    defaultTestConfig(),
		Logger:    logger.New("INFO"),
		Workflow:  system.workflow,
		Knowledge: knowledge.NewService(),
		Audit:     audit.NewNoop(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/discovery", nil)
	req.Header.Set("Authorization", "Bearer ops-token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ops router to expose platform discovery, got %d", recorder.Code)
	}
}

func TestRegisterOpsRoutesExposeSkillsEndpoint(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, false, false)
	mux := http.NewServeMux()
	RegisterOpsRoutes(mux, Dependencies{
		Config:    defaultTestConfig(),
		Logger:    logger.New("INFO"),
		Workflow:  system.workflow,
		Knowledge: knowledge.NewService(),
		Audit:     audit.NewNoop(),
		Skills:    system.skills,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/skills", nil)
	req.Header.Set("Authorization", "Bearer ops-token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK && recorder.Code != http.StatusMovedPermanently && recorder.Code != http.StatusTemporaryRedirect && recorder.Code != http.StatusSeeOther {
		t.Fatalf("expected ops router to expose skills endpoint, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMetricsEndpointRendersRegistry(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
	registry.AddAlertEvents("vmalert", "accepted", 1)

	deps := Dependencies{
		Config:      config.Config{},
		Logger:      logger.New("INFO"),
		Metrics:     registry,
		AlertIngest: alertintake.NewService(),
		Workflow:    workflow.NewService(workflow.Options{DiagnosisEnabled: true}),
		Knowledge:   knowledge.NewService(),
		Audit:       audit.NewNoop(),
	}

	mux := http.NewServeMux()
	RegisterPublicRoutes(mux, deps)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected metrics endpoint to succeed, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "tars_alert_events_total") {
		t.Fatalf("expected metrics output, got:\n%s", recorder.Body.String())
	}
}

func TestMetricsEndpointRendersObservabilityStoreMetrics(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
	store, err := observability.NewStore(config.ObservabilityConfig{
		DataDir: t.TempDir(),
		Metrics: config.ObservabilitySignalConfig{Retention: 24 * time.Hour, MaxSizeBytes: 1 << 20},
		Logs:    config.ObservabilitySignalConfig{Retention: 24 * time.Hour, MaxSizeBytes: 1 << 20},
		Traces:  config.ObservabilitySignalConfig{Retention: 24 * time.Hour, MaxSizeBytes: 1 << 20},
	})
	if err != nil {
		t.Fatalf("new observability store: %v", err)
	}
	store.SetMetrics(registry)
	if err := store.AppendLog(observability.SignalRecord{
		Timestamp: time.Now().UTC(),
		Level:     "error",
		Component: "telegram",
		Message:   "telegram getUpdates failed",
		TraceID:   "trace-1",
	}); err != nil {
		t.Fatalf("append log: %v", err)
	}

	deps := Dependencies{
		Config:        config.Config{},
		Logger:        logger.New("INFO"),
		Metrics:       registry,
		Observability: store,
		AlertIngest:   alertintake.NewService(),
		Workflow:      workflow.NewService(workflow.Options{DiagnosisEnabled: true}),
		Knowledge:     knowledge.NewService(),
		Audit:         audit.NewNoop(),
	}

	mux := http.NewServeMux()
	RegisterPublicRoutes(mux, deps)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected metrics endpoint to succeed, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, want := range []string{
		`tars_observability_store_append_duration_seconds_count{signal="logs"} 1`,
		`tars_observability_store_records_total{signal="logs"} 1`,
		`tars_observability_store_file_bytes{signal="logs"}`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected metrics output to contain %q, got:\n%s", want, body)
		}
	}
}
