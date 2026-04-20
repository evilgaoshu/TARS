package connectors

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func connectorManifest(id, name, displayName, vendor, version, connectorType, protocol string) Manifest {
	return Manifest{
		APIVersion: "tars.connector/v1alpha1",
		Kind:       "connector",
		Metadata: Metadata{
			ID:          id,
			Name:        name,
			DisplayName: displayName,
			Vendor:      vendor,
			Version:     version,
		},
		Spec: Spec{
			Type:     connectorType,
			Protocol: protocol,
			ImportExport: ImportExport{
				Exportable: true,
				Importable: true,
				Formats:    []string{"yaml"},
			},
		},
		Compatibility: Compatibility{
			TARSMajorVersions: []string{CurrentTARSMajorVersion},
		},
	}
}

func TestParseConfigNormalizesManifestDefaultsAndRoundTrips(t *testing.T) {
	t.Parallel()

	content := `connectors:
  entries:
    - metadata:
        id: victorialogs-main
        name: victorialogs
        display_name: " Victoria Logs "
        vendor: " victorialogs "
        version: " 1.0.0 "
      spec:
        type: observability
        protocol: " log_file "
        connection_form:
          - key: " path "
            label: " Path "
            type: " text "
            required: true
            options: [" /var/log/messages ", "", " /var/log/syslog "]
        import_export:
          exportable: true
          importable: true
          formats: [" yaml ", ""]
      config:
        values:
          " path ": " /var/log/messages "
          "": "ignored"
        secret_refs:
          " token ": " secret://victorialogs/token "
      compatibility:
        tars_major_versions: [" 1 ", ""]
        modes: [" managed ", ""]
    - metadata:
        id: ssh-main
        name: " ssh "
        display_name: " SSH Main "
        vendor: " ssh "
        version: " 1.1.0 "
      spec:
        type: execution
        protocol: " jumpserver_api "
        import_export:
          exportable: true
          importable: true
          formats: [" yaml "]
      config:
        values:
          " host ": " 127.0.0.1 "
      compatibility:
        tars_major_versions: [" 1 "]
`

	cfg, encoded, err := ParseConfig([]byte(content))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if len(cfg.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(cfg.Entries))
	}
	if cfg.Entries[0].Metadata.ID != "ssh-main" || cfg.Entries[1].Metadata.ID != "victorialogs-main" {
		t.Fatalf("expected entries sorted by id, got %+v", []string{cfg.Entries[0].Metadata.ID, cfg.Entries[1].Metadata.ID})
	}

	ssh := cfg.Entries[0]
	if ssh.APIVersion != "tars.connector/v1alpha1" || ssh.Kind != "connector" {
		t.Fatalf("expected manifest defaults to be applied, got api_version=%q kind=%q", ssh.APIVersion, ssh.Kind)
	}
	if ssh.Metadata.DisplayName != "SSH Main" || ssh.Metadata.Name != "ssh" {
		t.Fatalf("expected whitespace to be trimmed, got %+v", ssh.Metadata)
	}
	if ssh.Config.Values["host"] != "127.0.0.1" {
		t.Fatalf("expected config values to be trimmed, got %+v", ssh.Config.Values)
	}

	logs := cfg.Entries[1]
	if got := logs.Spec.ConnectionForm[0].Options; !reflect.DeepEqual(got, []string{"/var/log/messages", "/var/log/syslog"}) {
		t.Fatalf("expected connection form options to be trimmed, got %+v", got)
	}
	if got := logs.Config.SecretRefs["token"]; got != "secret://victorialogs/token" {
		t.Fatalf("expected secret refs to be trimmed, got %+v", logs.Config.SecretRefs)
	}
	if got := logs.Compatibility.Modes; !reflect.DeepEqual(got, []string{"managed"}) {
		t.Fatalf("expected compatibility modes to be trimmed, got %+v", got)
	}

	roundTripped, roundEncoded, err := ParseConfig([]byte(encoded))
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if len(roundTripped.Entries) != len(cfg.Entries) {
		t.Fatalf("expected round-trip entry count to stay stable, got %d want %d", len(roundTripped.Entries), len(cfg.Entries))
	}
	if roundTripped.Entries[0].Metadata.ID != cfg.Entries[0].Metadata.ID || roundTripped.Entries[1].Metadata.ID != cfg.Entries[1].Metadata.ID {
		t.Fatalf("expected round-trip entries to remain sorted, got %+v", []string{roundTripped.Entries[0].Metadata.ID, roundTripped.Entries[1].Metadata.ID})
	}
	if roundTripped.Entries[0].Config.Values["host"] != "127.0.0.1" || roundTripped.Entries[1].Config.SecretRefs["token"] != "secret://victorialogs/token" {
		t.Fatalf("expected round-trip normalization to keep trimmed values, got %+v", roundTripped.Entries)
	}
	if strings.Index(roundEncoded, "id: ssh-main") > strings.Index(roundEncoded, "id: victorialogs-main") {
		t.Fatalf("expected encoded config to preserve sorted ordering, got:\n%s", roundEncoded)
	}
}

func TestValidateManifestAndHealthHelpers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		m    Manifest
		want string
	}{
		{
			name: "invalid api version",
			m: func() Manifest {
				m := connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "jumpserver_api")
				m.APIVersion = "tars.connector/v2"
				return m
			}(),
			want: "manifest api_version must be tars.connector/v1alpha1",
		},
		{
			name: "invalid kind",
			m: func() Manifest {
				m := connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "jumpserver_api")
				m.Kind = "integration"
				return m
			}(),
			want: "manifest kind must be connector",
		},
		{
			name: "missing id",
			m: func() Manifest {
				m := connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "jumpserver_api")
				m.Metadata.ID = ""
				return m
			}(),
			want: "metadata.id is required",
		},
		{
			name: "missing type",
			m: func() Manifest {
				m := connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "", "jumpserver_api")
				return m
			}(),
			want: "spec.type is required",
		},
		{
			name: "missing protocol",
			m: func() Manifest {
				m := connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "")
				return m
			}(),
			want: "spec.protocol is required",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateManifest(tc.m)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("expected validation error")
				}
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}

	healthy := connectorManifest("victoriametrics-main", "victoriametrics", "Victoria Metrics", "victoriametrics", "1.0.0", "metrics", "victoriametrics_http")
	healthy.Spec.ConnectionForm = []Field{
		{Key: "base_url", Label: "Base URL", Type: "text", Required: true},
		{Key: "api_token", Label: "API Token", Type: "secret", Required: true, Secret: true},
	}
	healthy.Config.Values = map[string]string{"base_url": "https://victoriametrics.example.test"}
	healthy.Config.SecretRefs = map[string]string{"api_token": "secret://victoriametrics/token"}
	got := HealthStatusForManifest(healthy, CompatibilityReport{Compatible: true}, time.Unix(100, 0).UTC())
	if got.Status != "healthy" || got.CredentialStatus != "configured" {
		t.Fatalf("expected healthy status, got %+v", got)
	}

	disabled := healthy
	disabled.Disabled = true
	got = HealthStatusForManifest(disabled, CompatibilityReport{Compatible: true}, time.Unix(100, 0).UTC())
	if got.Status != "disabled" || got.Summary != "connector is disabled" {
		t.Fatalf("expected disabled status, got %+v", got)
	}

	incompatible := healthy
	incompatible.Compatibility.TARSMajorVersions = []string{"2"}
	got = HealthStatusForManifest(incompatible, CompatibilityReport{Compatible: false, Reasons: []string{"connector is not compatible with current TARS major version"}}, time.Unix(100, 0).UTC())
	if got.Status != "unhealthy" || !strings.Contains(got.Summary, "current TARS major version") {
		t.Fatalf("expected incompatible health summary, got %+v", got)
	}

	missingRequired := healthy
	missingRequired.Config.Values = map[string]string{}
	got = HealthStatusForManifest(missingRequired, CompatibilityReport{Compatible: true}, time.Unix(100, 0).UTC())
	if got.Status != "unhealthy" || !strings.Contains(got.Summary, "Base URL") {
		t.Fatalf("expected missing required fields summary, got %+v", got)
	}

	missingSecret := healthy
	missingSecret.Config.SecretRefs = map[string]string{}
	got = HealthStatusForManifest(missingSecret, CompatibilityReport{Compatible: true}, time.Unix(100, 0).UTC())
	if got.Status != "unhealthy" || got.CredentialStatus != "missing_credentials" || !strings.Contains(got.Summary, "API Token") {
		t.Fatalf("expected missing credentials summary, got %+v", got)
	}

	if got := joinNonEmpty([]string{" ", " ssh ", "", "victoriametrics"}, ", "); got != "ssh, victoriametrics" {
		t.Fatalf("expected joinNonEmpty to filter blank values, got %q", got)
	}
	if got := stringsJoin([]string{"ssh", "victoriametrics", "victorialogs"}, " -> "); got != "ssh -> victoriametrics -> victorialogs" {
		t.Fatalf("expected stringsJoin to preserve ordering, got %q", got)
	}
}

func TestValidateManifestRejectsInlineSSHSecretMaterial(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"password", "private_key", "passphrase"} {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			entry := connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "ssh_native")
			entry.Config.Values = map[string]string{
				"credential_id": "ops-key",
				key:             "inline-secret-material",
			}
			if err := ValidateManifest(entry); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("expected ValidateManifest to reject inline %s, got %v", key, err)
			}
		})
	}
}

func TestPendingRuntimeProbeHealth(t *testing.T) {
	t.Parallel()

	now := time.Unix(1234, 0).UTC()
	healthy := connectorManifest("jumpserver-main", "jumpserver", "JumpServer Main", "jumpserver", "1.0.0", "execution", "jumpserver_api")
	got := pendingRuntimeProbeHealth(healthy, CompatibilityReport{Compatible: true}, now)
	if got.Status != "unknown" || got.Summary != "runtime health check required after connector change" || !got.CheckedAt.Equal(now) {
		t.Fatalf("expected pending probe health for healthy connector change, got %+v", got)
	}

	healthy.Disabled = true
	got = pendingRuntimeProbeHealth(healthy, CompatibilityReport{Compatible: true}, now)
	if got.Status != "disabled" {
		t.Fatalf("expected disabled fallback health, got %+v", got)
	}

	healthy.Disabled = false
	got = pendingRuntimeProbeHealth(healthy, CompatibilityReport{Compatible: false, Reasons: []string{"connector is not compatible with current TARS major version"}}, now)
	if got.Status != "unhealthy" || !strings.Contains(got.Summary, "current TARS major version") {
		t.Fatalf("expected incompatible fallback health, got %+v", got)
	}
}

func TestLifecycleStateFileHelpersRoundTripAndMissingPaths(t *testing.T) {
	t.Parallel()

	if got := lifecycleStatePath(""); got != "" {
		t.Fatalf("expected blank lifecycle path to stay blank, got %q", got)
	}
	if got := lifecycleStatePath(" /tmp/connectors.yaml "); got != "/tmp/connectors.yaml.state.yaml" {
		t.Fatalf("expected lifecycle path suffix, got %q", got)
	}

	dir := t.TempDir()
	statePath := filepath.Join(dir, "connectors.state.yaml")
	state := map[string]LifecycleState{
		"victoria-logs-main": {
			ConnectorID: "victoria-logs-main",
			Health:      HealthStatus{Status: "healthy", Summary: "probe ok", CheckedAt: time.Unix(10, 0).UTC()},
			History:     []LifecycleEvent{{Type: "install", Version: "1.0.0", CreatedAt: time.Unix(10, 0).UTC()}},
		},
		"ssh-main": {
			ConnectorID: "ssh-main",
			Health:      HealthStatus{Status: "unknown", Summary: "pending probe", CheckedAt: time.Unix(20, 0).UTC()},
		},
	}
	if err := saveLifecycleStateFile(statePath, state); err != nil {
		t.Fatalf("save lifecycle state: %v", err)
	}

	loaded, err := loadLifecycleStateFile(statePath)
	if err != nil {
		t.Fatalf("load lifecycle state: %v", err)
	}
	if len(loaded) != 2 || loaded["ssh-main"].ConnectorID != "ssh-main" || loaded["victoria-logs-main"].Health.Status != "healthy" {
		t.Fatalf("unexpected loaded lifecycle state: %+v", loaded)
	}

	if got, err := loadLifecycleStateFile(""); err != nil || len(got) != 0 {
		t.Fatalf("expected blank lifecycle path to return empty map, got %+v err=%v", got, err)
	}
	if got, err := loadLifecycleStateFile(filepath.Join(dir, "missing.yaml")); err != nil || len(got) != 0 {
		t.Fatalf("expected missing lifecycle file to return empty map, got %+v err=%v", got, err)
	}

	if err := saveLifecycleStateFile("", state); err != nil {
		t.Fatalf("expected blank save path to be a no-op, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte("not: [valid"), 0o600); err != nil {
		t.Fatalf("write broken yaml fixture: %v", err)
	}
	if _, err := loadLifecycleStateFile(filepath.Join(dir, "broken.yaml")); err == nil {
		t.Fatalf("expected invalid lifecycle yaml to fail")
	}
}

func TestLoadRuntimeStateUsesPersistCallback(t *testing.T) {
	t.Parallel()

	manager, err := NewManager("")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	var persistedCfg Config
	var persistedState map[string]LifecycleState
	manager.SetPersistence(func(cfg Config, state map[string]LifecycleState) error {
		persistedCfg = cloneConfig(cfg)
		persistedState = cloneLifecycleMap(state)
		return nil
	})

	cfg := Config{
		Entries: []Manifest{
			connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "jumpserver_api"),
		},
	}
	lifecycle := map[string]LifecycleState{
		"ssh-main": {
			ConnectorID: "ssh-main",
			Health:      HealthStatus{Status: "healthy", Summary: "probe ok", CheckedAt: time.Unix(50, 0).UTC()},
		},
	}

	if err := manager.LoadRuntimeState(cfg, lifecycle); err != nil {
		t.Fatalf("load runtime state: %v", err)
	}
	if err := manager.SaveConfig(cfg); err != nil {
		t.Fatalf("save runtime state through callback path: %v", err)
	}

	snapshot := manager.Snapshot()
	if !snapshot.Loaded || len(snapshot.Config.Entries) != 1 || snapshot.Config.Entries[0].Metadata.ID != "ssh-main" {
		t.Fatalf("expected manager snapshot to be populated, got %+v", snapshot)
	}
	if persistedCfg.Entries[0].Metadata.ID != "ssh-main" || persistedState["ssh-main"].Health.Status != "healthy" {
		t.Fatalf("expected persistence callback to receive normalized state, got cfg=%+v state=%+v", persistedCfg, persistedState)
	}
}

func TestPathlessUpsertPersistsLifecycleAndHealthState(t *testing.T) {
	t.Parallel()

	manager, err := NewManager("")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	var persistedCfg Config
	var persistedState map[string]LifecycleState
	persistCalls := 0
	manager.SetPersistence(func(cfg Config, state map[string]LifecycleState) error {
		persistCalls++
		persistedCfg = cloneConfig(cfg)
		persistedState = cloneLifecycleMap(state)
		return nil
	})

	logs := connectorManifest("victorialogs-main", "victorialogs", "Victoria Logs", "victorialogs", "1.0.0", "observability", "victorialogs_http")
	logs.Config.Values = map[string]string{"base_url": "https://play-vmlogs.victoriametrics.com"}
	if err := manager.Upsert(logs); err != nil {
		t.Fatalf("upsert victorialogs: %v", err)
	}

	ssh := connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "ssh")
	ssh.Config.SecretRefs = map[string]string{"private_key": "secret://ssh/main/private_key"}
	if err := manager.Upsert(ssh); err != nil {
		t.Fatalf("upsert ssh: %v", err)
	}

	if persistCalls != 2 {
		t.Fatalf("expected persistence callback for each upsert, got %d", persistCalls)
	}
	if len(persistedCfg.Entries) != 2 || persistedCfg.Entries[0].Metadata.ID != "ssh-main" || persistedCfg.Entries[1].Metadata.ID != "victorialogs-main" {
		t.Fatalf("expected persisted config to be sorted by connector id, got %+v", persistedCfg.Entries)
	}
	if persistedState["victorialogs-main"].Health.Status != "healthy" || persistedState["ssh-main"].SecretRefs["private_key"] != "secret://ssh/main/private_key" {
		t.Fatalf("unexpected persisted lifecycle state: %+v", persistedState)
	}

	lifecycle := manager.ListLifecycle()
	if len(lifecycle) != 2 || lifecycle[0].ConnectorID != "ssh-main" || lifecycle[1].ConnectorID != "victorialogs-main" {
		t.Fatalf("expected lifecycle list to be sorted by connector id, got %+v", lifecycle)
	}
	lifecycle[1].Health.Status = "mutated"
	storedLogs, ok := manager.GetLifecycle("victorialogs-main")
	if !ok || storedLogs.Health.Status == "mutated" {
		t.Fatalf("expected lifecycle list items to be cloned, got ok=%v state=%+v", ok, storedLogs)
	}

	upgradedLogs := logs
	upgradedLogs.Metadata.Version = "1.1.0"
	if err := manager.Upsert(upgradedLogs); err != nil {
		t.Fatalf("replace victorialogs: %v", err)
	}
	replaced, ok := manager.GetLifecycle("victorialogs-main")
	if !ok {
		t.Fatalf("expected victorialogs lifecycle after replacement")
	}
	if replaced.CurrentVersion != "1.1.0" || len(replaced.Revisions) != 2 || replaced.History[len(replaced.History)-1].Type != "upgrade" {
		t.Fatalf("expected replacement to record upgrade lifecycle, got %+v", replaced)
	}

	available, err := manager.SetAvailableVersion("victorialogs-main", " 1.2.0 ")
	if err != nil {
		t.Fatalf("set available version: %v", err)
	}
	if available.AvailableVersion != "1.2.0" {
		t.Fatalf("expected available version to be trimmed, got %+v", available)
	}

	firstProbe := time.Unix(1000, 0).UTC()
	health, err := manager.RecordHealth(" victorialogs-main ", " healthy ", " probe ok ", firstProbe)
	if err != nil {
		t.Fatalf("record health: %v", err)
	}
	if health.Health.Status != "healthy" || health.Health.Summary != "probe ok" || !health.Health.CheckedAt.Equal(firstProbe) {
		t.Fatalf("expected recorded health to be normalized, got %+v", health.Health)
	}
	historyLen := len(health.HealthHistory)
	secondProbe := time.Unix(2000, 0).UTC()
	health, err = manager.RecordHealth("victorialogs-main", "healthy", "probe ok", secondProbe)
	if err != nil {
		t.Fatalf("record replacement health: %v", err)
	}
	if len(health.HealthHistory) != historyLen || !health.HealthHistory[len(health.HealthHistory)-1].CheckedAt.Equal(secondProbe) {
		t.Fatalf("expected duplicate health status+summary to replace last entry, got %+v", health.HealthHistory)
	}
}

func TestManagerLifecycleErrorBranches(t *testing.T) {
	t.Parallel()

	var nilManager *Manager
	if err := nilManager.Upsert(connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "ssh")); !errors.Is(err, ErrConfigPathNotSet) {
		t.Fatalf("expected nil upsert config path error, got %v", err)
	}
	if _, err := nilManager.RecordHealth("ssh-main", "healthy", "ok", time.Now()); !errors.Is(err, ErrConfigPathNotSet) {
		t.Fatalf("expected nil record health config path error, got %v", err)
	}
	if _, err := nilManager.SetAvailableVersion("ssh-main", "1.1.0"); !errors.Is(err, ErrConfigPathNotSet) {
		t.Fatalf("expected nil available version config path error, got %v", err)
	}

	manager, err := NewManager("")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if _, err := manager.RecordHealth("", "healthy", "ok", time.Now()); !errors.Is(err, ErrConnectorNotFound) {
		t.Fatalf("expected blank health connector not found, got %v", err)
	}
	if _, err := manager.RecordHealth("missing", "healthy", "ok", time.Now()); !errors.Is(err, ErrConnectorNotFound) {
		t.Fatalf("expected missing health connector not found, got %v", err)
	}
	if _, err := manager.SetAvailableVersion("missing", "1.1.0"); !errors.Is(err, ErrConnectorNotFound) {
		t.Fatalf("expected missing available version connector not found, got %v", err)
	}
	if _, _, err := manager.Rollback("missing", RollbackOptions{}); !errors.Is(err, ErrConnectorNotFound) {
		t.Fatalf("expected missing rollback connector not found, got %v", err)
	}
}

func TestCloneTemplateAssignmentsKeepsValuesIsolated(t *testing.T) {
	t.Parallel()

	assignments := []TemplateAssignment{{
		ID:     "ssh-default",
		Name:   "SSH Default",
		Values: map[string]string{"user": "sre", "auth": "secret_ref"},
	}}

	cloned := cloneTemplateAssignments(assignments)
	cloned[0].Values["user"] = "mutated"

	if assignments[0].Values["user"] != "sre" {
		t.Fatalf("expected clone mutation not to leak to source assignments: %+v", assignments)
	}
	if got := cloneTemplateAssignments(nil); got != nil {
		t.Fatalf("expected nil template assignments to stay nil, got %+v", got)
	}
}

func TestManagerSaveAndReloadAcrossConnectorTransitions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "connectors.yaml")

	initial := Config{
		Entries: []Manifest{
			connectorManifest("victorialogs-main", "victorialogs", "Victoria Logs", "victorialogs", "1.0.0", "observability", "log_file"),
			connectorManifest("victoriametrics-main", "victoriametrics", "Victoria Metrics", "victoriametrics", "1.0.0", "metrics", "victoriametrics_http"),
			connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "jumpserver_api"),
		},
	}
	initialContent, err := EncodeConfig(&initial)
	if err != nil {
		t.Fatalf("encode initial config: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(initialContent), 0o600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	manager, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	updated := cloneConfig(initial)
	updated.Entries[0].Config.Values = map[string]string{"path": "/var/log/messages"}
	updated.Entries[1].Disabled = true
	updatedContent, err := EncodeConfig(&updated)
	if err != nil {
		t.Fatalf("encode updated config: %v", err)
	}

	if err := manager.Save(updatedContent); err != nil {
		t.Fatalf("save updated config: %v", err)
	}

	ssh, ok := manager.GetLifecycle("ssh-main")
	if !ok {
		t.Fatalf("expected ssh lifecycle after save")
	}
	if ssh.Health.Status != "healthy" || len(ssh.Revisions) != 1 {
		t.Fatalf("expected unchanged ssh connector to remain healthy after save, got %+v", ssh)
	}

	vm, ok := manager.GetLifecycle("victoriametrics-main")
	if !ok {
		t.Fatalf("expected victoriametrics lifecycle after save")
	}
	if vm.Health.Status != "disabled" || len(vm.History) < 2 || vm.History[len(vm.History)-1].Type != "disable" {
		t.Fatalf("expected victoriametrics disable transition, got %+v", vm)
	}

	logs, ok := manager.GetLifecycle("victorialogs-main")
	if !ok {
		t.Fatalf("expected victorialogs lifecycle after save")
	}
	if logs.Health.Status != "healthy" || len(logs.Revisions) != 2 || logs.History[len(logs.History)-1].Type != "update" {
		t.Fatalf("expected victorialogs update transition, got %+v", logs)
	}

	upgradedSSH := connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.1.0", "execution", "jumpserver_api")
	_, upgradedState, err := manager.Upgrade("ssh-main", UpgradeOptions{
		Manifest:  upgradedSSH,
		Reason:    "upgrade ssh runtime",
		Available: "1.1.0",
	})
	if err != nil {
		t.Fatalf("upgrade ssh connector: %v", err)
	}
	if upgradedState.Health.Status != "unknown" || len(upgradedState.Revisions) != 2 {
		t.Fatalf("expected ssh upgrade to require a probe, got %+v", upgradedState)
	}

	if _, err := manager.SetEnabled("victoriametrics-main", true); err != nil {
		t.Fatalf("re-enable victoriametrics: %v", err)
	}
	if _, err := manager.SetAvailableVersion("ssh-main", "1.2.0"); err != nil {
		t.Fatalf("set ssh available version: %v", err)
	}

	firstProbe := time.Unix(1000, 0).UTC()
	secondProbe := time.Unix(2000, 0).UTC()
	if _, err := manager.RecordHealth("ssh-main", "healthy", "runtime probe passed", firstProbe); err != nil {
		t.Fatalf("record first health: %v", err)
	}
	if _, err := manager.RecordHealth("ssh-main", "healthy", "runtime probe passed", secondProbe); err != nil {
		t.Fatalf("record second health: %v", err)
	}

	sshAfterRecords, ok := manager.GetLifecycle("ssh-main")
	if !ok {
		t.Fatalf("expected ssh lifecycle after recorded health updates")
	}
	if !sshAfterRecords.Health.CheckedAt.Equal(secondProbe) || len(sshAfterRecords.HealthHistory) < 3 || sshAfterRecords.HealthHistory[len(sshAfterRecords.HealthHistory)-1].Summary != "runtime probe passed" {
		t.Fatalf("expected latest probe to be preserved in health history, got %+v", sshAfterRecords.HealthHistory)
	}

	reloaded, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("reload manager from disk: %v", err)
	}

	reloadedSSH, ok := reloaded.GetLifecycle("ssh-main")
	if !ok {
		t.Fatalf("expected ssh lifecycle after reload")
	}
	if reloadedSSH.AvailableVersion != "1.2.0" || reloadedSSH.Health.Status != "healthy" {
		t.Fatalf("expected ssh lifecycle state to persist, got %+v", reloadedSSH)
	}
	if len(reloadedSSH.Revisions) != 2 {
		t.Fatalf("expected ssh revisions to survive reload, got %+v", reloadedSSH.Revisions)
	}

	reloadedVM, ok := reloaded.GetLifecycle("victoriametrics-main")
	if !ok {
		t.Fatalf("expected victoriametrics lifecycle after reload")
	}
	if reloadedVM.Health.Status != "healthy" || len(reloadedVM.History) < 3 || reloadedVM.History[len(reloadedVM.History)-1].Type != "enable" {
		t.Fatalf("expected victoriametrics enable transition to persist, got %+v", reloadedVM)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	saved := string(content)
	if strings.Index(saved, "id: ssh-main") > strings.Index(saved, "id: victorialogs-main") || strings.Index(saved, "id: victorialogs-main") > strings.Index(saved, "id: victoriametrics-main") {
		t.Fatalf("expected saved config to be normalized and sorted, got:\n%s", saved)
	}
}

func TestSaveConfigPreservesRecordedHealthySummaryForUnchangedConnector(t *testing.T) {
	t.Parallel()

	manager, err := NewManager("")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	vmlogs := connectorManifest("victorialogs-main", "victorialogs", "Victoria Logs", "victoriametrics", "1.0.0", "logs", "victorialogs_http")
	vmlogs.Config.Values = map[string]string{"base_url": "http://127.0.0.1:9428"}
	if err := manager.Upsert(vmlogs); err != nil {
		t.Fatalf("upsert victorialogs: %v", err)
	}

	checkedAt := time.Unix(3000, 0).UTC()
	if _, err := manager.RecordHealth("victorialogs-main", "healthy", "victorialogs health probe succeeded (OK)", checkedAt); err != nil {
		t.Fatalf("record health: %v", err)
	}

	before, ok := manager.GetLifecycle("victorialogs-main")
	if !ok {
		t.Fatalf("expected lifecycle before save")
	}
	if before.Health.Summary != "victorialogs health probe succeeded (OK)" {
		t.Fatalf("expected recorded health summary before save, got %+v", before.Health)
	}

	snapshot := manager.Snapshot()
	if err := manager.SaveConfig(snapshot.Config); err != nil {
		t.Fatalf("save unchanged config: %v", err)
	}

	after, ok := manager.GetLifecycle("victorialogs-main")
	if !ok {
		t.Fatalf("expected lifecycle after save")
	}
	if after.Health.Status != "healthy" {
		t.Fatalf("expected healthy status after save, got %+v", after.Health)
	}
	if after.Health.Summary != "victorialogs health probe succeeded (OK)" {
		t.Fatalf("expected save to preserve runtime probe summary, got %+v", after.Health)
	}
	if !after.Health.CheckedAt.Equal(checkedAt) {
		t.Fatalf("expected save to preserve runtime probe checked_at, got %+v", after.Health)
	}
}
