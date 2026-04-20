package connectors

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSelectHealthyRuntimeManifestSkipsDegradedExecutionConnector(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "connectors.yaml")
	content := `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: jumpserver-main
        name: jumpserver
        display_name: JumpServer Main
        vendor: jumpserver
        version: 1.0.0
      spec:
        type: execution
        protocol: jumpserver_api
        import_export:
          exportable: true
          importable: true
          formats: ["yaml"]
      compatibility:
        tars_major_versions: ["1"]
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	manager, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if _, err := manager.RecordHealth("jumpserver-main", "degraded", "missing secrets", time.Now().UTC()); err != nil {
		t.Fatalf("record health: %v", err)
	}

	if _, ok := SelectHealthyRuntimeManifest(manager, "execution", "", map[string]struct{}{"jumpserver_api": {}}); ok {
		t.Fatalf("expected degraded execution connector to be skipped for healthy selection")
	}
}

func TestSelectRuntimeManifestPrefersHealthyMetricsConnector(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "connectors.yaml")
	content := `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: prometheus-main
        name: prometheus
        display_name: Prometheus Main
        vendor: prometheus
        version: 1.0.0
      spec:
        type: metrics
        protocol: prometheus_http
        import_export:
          exportable: true
          importable: true
          formats: ["yaml"]
      compatibility:
        tars_major_versions: ["1"]
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: victoriametrics-main
        name: victoriametrics
        display_name: VictoriaMetrics Main
        vendor: victoriametrics
        version: 1.0.0
      spec:
        type: metrics
        protocol: victoriametrics_http
        import_export:
          exportable: true
          importable: true
          formats: ["yaml"]
      compatibility:
        tars_major_versions: ["1"]
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	manager, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if _, err := manager.RecordHealth("prometheus-main", "degraded", "dial tcp refused", time.Now().UTC()); err != nil {
		t.Fatalf("record prometheus health: %v", err)
	}
	if _, err := manager.RecordHealth("victoriametrics-main", "healthy", "query succeeded", time.Now().UTC()); err != nil {
		t.Fatalf("record victoriametrics health: %v", err)
	}

	entry, ok := SelectRuntimeManifest(manager, "metrics", "", map[string]struct{}{"prometheus_http": {}, "victoriametrics_http": {}})
	if !ok {
		t.Fatalf("expected metrics connector selection to succeed")
	}
	if entry.Metadata.ID != "victoriametrics-main" {
		t.Fatalf("expected healthy metrics connector to be preferred, got %s", entry.Metadata.ID)
	}
}
