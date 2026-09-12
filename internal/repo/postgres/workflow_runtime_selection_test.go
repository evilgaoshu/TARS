package postgres

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"tars/internal/modules/connectors"
)

func TestSelectExecutionRuntimeFallsBackToHealthySSHNativeConnector(t *testing.T) {
	t.Parallel()

	manager := writePostgresWorkflowRuntimeSelectionConnectorConfig(t)
	if _, err := manager.RecordHealth("jumpserver-main", "degraded", "jumpserver command execution denied", time.Now().UTC()); err != nil {
		t.Fatalf("record jumpserver health: %v", err)
	}
	if _, err := manager.RecordHealth("ssh-main", "healthy", "ssh credential probe passed", time.Now().UTC()); err != nil {
		t.Fatalf("record ssh health: %v", err)
	}

	got := selectExecutionRuntime(manager)
	if got.ConnectorID != "ssh-main" || got.ConnectorType != "execution" || got.ConnectorVendor != "openssh" || got.Protocol != "ssh_native" || got.ExecutionMode != "ssh_native" {
		t.Fatalf("expected ssh-main fallback connector, got %+v", got)
	}
	if got.Runtime == nil || got.Runtime.Runtime != "connector" || got.Runtime.FallbackUsed {
		t.Fatalf("expected connector runtime metadata without legacy fallback, got %+v", got.Runtime)
	}
}

func TestSelectExecutionRuntimeSkipsJumpServerAPIOnlyHealth(t *testing.T) {
	t.Parallel()

	manager := writePostgresWorkflowRuntimeSelectionConnectorConfig(t)
	if _, err := manager.RecordHealth("jumpserver-main", "healthy", "jumpserver API probe succeeded", time.Now().UTC()); err != nil {
		t.Fatalf("record jumpserver health: %v", err)
	}
	if _, err := manager.RecordHealth("ssh-main", "healthy", "ssh credential probe passed", time.Now().UTC()); err != nil {
		t.Fatalf("record ssh health: %v", err)
	}

	got := selectExecutionRuntime(manager)
	if got.ConnectorID != "ssh-main" || got.Protocol != "ssh_native" || got.ExecutionMode != "ssh_native" {
		t.Fatalf("expected ssh-main to remain selected when jumpserver only has API health, got %+v", got)
	}
	if got.Runtime == nil || got.Runtime.ConnectorID != "ssh-main" {
		t.Fatalf("expected runtime metadata to stay on ssh-main, got %+v", got.Runtime)
	}
}

func writePostgresWorkflowRuntimeSelectionConnectorConfig(t *testing.T) *connectors.Manager {
	t.Helper()
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
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: ssh-main
        name: ssh
        display_name: SSH Main
        vendor: openssh
        version: 1.0.0
      spec:
        type: execution
        protocol: ssh_native
        import_export:
          exportable: true
          importable: true
          formats: ["yaml"]
      config:
        values:
          host: 192.168.3.9
          username: root
          credential_id: evi19-ssh-root-3-9
      compatibility:
        tars_major_versions: ["1"]
`
	if err := os.WriteFile(configPath, []byte("connectors:\n  entries: []\n"), 0o600); err != nil {
		t.Fatalf("seed connectors config: %v", err)
	}
	manager, err := connectors.NewManager(configPath)
	if err != nil {
		t.Fatalf("new connectors manager: %v", err)
	}
	if err := manager.Save(content); err != nil {
		t.Fatalf("save connectors config: %v", err)
	}
	return manager
}
