package connectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpgradeAndRollbackPreserveRevisionHistoryAndRequireProbe(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "connectors.yaml")
	initial := `connectors:
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
      config:
        values:
          base_url: https://jumpserver.example.test
      compatibility:
        tars_major_versions: ["1"]
`
	if err := os.WriteFile(configPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}

	manager, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	upgradedManifest := Manifest{
		APIVersion: "tars.connector/v1alpha1",
		Kind:       "connector",
		Metadata: Metadata{
			ID:          "jumpserver-main",
			Name:        "jumpserver",
			DisplayName: "JumpServer Main",
			Vendor:      "jumpserver",
			Version:     "1.1.0",
		},
		Spec: Spec{
			Type:     "execution",
			Protocol: "jumpserver_api",
			ImportExport: ImportExport{
				Exportable: true,
				Importable: true,
				Formats:    []string{"yaml"},
			},
		},
		Config: RuntimeConfig{
			Values: map[string]string{"base_url": "https://jumpserver.example.test"},
		},
		Compatibility: Compatibility{
			TARSMajorVersions: []string{"1"},
		},
	}

	_, upgradedState, err := manager.Upgrade("jumpserver-main", UpgradeOptions{
		Manifest:  upgradedManifest,
		Reason:    "upgrade jumpserver connector",
		Available: "1.1.0",
	})
	if err != nil {
		t.Fatalf("upgrade connector: %v", err)
	}
	if upgradedState.Health.Status != "unknown" || upgradedState.Health.Summary != "runtime health check required after connector change" {
		t.Fatalf("expected pending probe health after upgrade, got %+v", upgradedState.Health)
	}

	_, rolledBackState, err := manager.Rollback("jumpserver-main", RollbackOptions{
		TargetVersion: "1.0.0",
		Reason:        "rollback after validation",
	})
	if err != nil {
		t.Fatalf("rollback connector: %v", err)
	}
	if rolledBackState.Health.Status != "unknown" || rolledBackState.Health.Summary != "runtime health check required after connector change" {
		t.Fatalf("expected pending probe health after rollback, got %+v", rolledBackState.Health)
	}
	if len(rolledBackState.Revisions) != 3 {
		t.Fatalf("expected install, upgrade, rollback revisions, got %+v", rolledBackState.Revisions)
	}
	if rolledBackState.Revisions[0].Version != "1.0.0" || rolledBackState.Revisions[0].Reason != "install" {
		t.Fatalf("expected original install revision to be preserved, got %+v", rolledBackState.Revisions[0])
	}
	if rolledBackState.Revisions[1].Version != "1.1.0" || rolledBackState.Revisions[1].Reason != "upgrade" {
		t.Fatalf("expected upgrade revision to be preserved, got %+v", rolledBackState.Revisions[1])
	}
	if rolledBackState.Revisions[2].Version != "1.0.0" || rolledBackState.Revisions[2].Reason != "rollback after validation" {
		t.Fatalf("expected rollback revision to be preserved, got %+v", rolledBackState.Revisions[2])
	}
	if len(rolledBackState.HealthHistory) == 0 || rolledBackState.HealthHistory[len(rolledBackState.HealthHistory)-1].Status != "unknown" {
		t.Fatalf("expected pending probe health in history, got %+v", rolledBackState.HealthHistory)
	}
}
