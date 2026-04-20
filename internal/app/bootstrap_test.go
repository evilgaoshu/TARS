package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"tars/internal/foundation/audit"
	"tars/internal/foundation/config"
	"tars/internal/foundation/observability"
	"tars/internal/modules/access"
	"tars/internal/modules/action"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/connectors"
	"tars/internal/modules/org"
	"tars/internal/modules/reasoning"
	"tars/internal/modules/sshcredentials"
	postgresrepo "tars/internal/repo/postgres"
)

func testBootstrapConfig(t *testing.T) config.Config {
	t.Helper()

	cfg := config.LoadFromEnv()
	cfg.Postgres.DSN = ""
	cfg.Vector.SQLitePath = ""
	cfg.Observability.DataDir = t.TempDir()
	cfg.Web.DistDir = t.TempDir()
	cfg.Output.SpoolDir = t.TempDir()
	cfg.Telegram.BotToken = ""
	cfg.Telegram.PollingEnabled = false
	cfg.Skills.ConfigPath = ""
	cfg.Skills.MarketplacePath = ""
	cfg.Extensions.StatePath = ""
	cfg.Automations.ConfigPath = ""
	cfg.Access.ConfigPath = ""
	cfg.Authorization.ConfigPath = ""
	cfg.Approval.ConfigPath = ""
	cfg.Org.ConfigPath = ""
	cfg.Connectors.ConfigPath = ""
	cfg.Connectors.SecretsPath = ""
	cfg.AgentRoles.ConfigPath = ""
	cfg.Reasoning.PromptsConfigPath = ""
	cfg.Reasoning.DesensitizationConfigPath = ""
	cfg.Reasoning.ProvidersConfigPath = ""
	cfg.Reasoning.SecretsConfigPath = ""
	return cfg
}

func TestNewWithConfigWithoutPostgresBuildsPilotLoop(t *testing.T) {
	t.Parallel()

	app, err := newWithConfig(testBootstrapConfig(t))
	if err != nil {
		t.Fatalf("newWithConfig: %v", err)
	}
	if app.services.AlertIngest == nil || app.services.Workflow == nil || app.services.Reasoning == nil || app.services.Action == nil || app.services.Channel == nil {
		t.Fatalf("expected pilot loop services to be present: %+v", app.services)
	}
}

func TestNewWithConfigRequiresPostgresWhenRuntimeConfigIsStrict(t *testing.T) {
	t.Parallel()

	cfg := testBootstrapConfig(t)
	cfg.RuntimeConfig.RequirePostgres = true

	_, err := newWithConfig(cfg)
	if !errors.Is(err, ErrRuntimeConfigPostgresRequired) {
		t.Fatalf("expected ErrRuntimeConfigPostgresRequired, got %v", err)
	}
}

func TestOptionalServicesAttachSeparatelyFromPilotLoop(t *testing.T) {
	t.Parallel()

	shared, err := buildSharedBootstrap(testBootstrapConfig(t))
	if err != nil {
		t.Fatalf("buildSharedBootstrap: %v", err)
	}
	core, err := buildPilotCoreServices(shared)
	if err != nil {
		t.Fatalf("buildPilotCoreServices: %v", err)
	}

	coreOnly := assembleServices(shared, core, optionalServices{})
	if coreOnly.AlertIngest == nil || coreOnly.Workflow == nil || coreOnly.Reasoning == nil || coreOnly.Action == nil || coreOnly.Channel == nil {
		t.Fatalf("expected core services in core-only assembly: %+v", coreOnly)
	}
	if coreOnly.Knowledge != nil || coreOnly.Inbox != nil || coreOnly.Trigger != nil || coreOnly.NotificationTemplates != nil || coreOnly.Skills != nil || coreOnly.Extensions != nil || coreOnly.Automations != nil || coreOnly.AgentRoles != nil || coreOnly.RuntimeConfig != nil {
		t.Fatalf("expected optional services to stay detached from core-only assembly: %+v", coreOnly)
	}

	optional, err := buildOptionalServices(shared, core)
	if err != nil {
		t.Fatalf("buildOptionalServices: %v", err)
	}
	full := assembleServices(shared, core, optional)
	if full.Knowledge == nil || full.Inbox == nil || full.Trigger == nil || full.NotificationTemplates == nil || full.Skills == nil || full.Extensions == nil || full.Automations == nil || full.AgentRoles == nil || full.RuntimeConfig == nil {
		t.Fatalf("expected optional services to be attached in full assembly: %+v", full)
	}
}

func TestBuildPilotCoreServicesRegistersVictoriaLogsCapabilityRuntime(t *testing.T) {
	t.Parallel()

	shared, err := buildSharedBootstrap(testBootstrapConfig(t))
	if err != nil {
		t.Fatalf("buildSharedBootstrap: %v", err)
	}
	core, err := buildPilotCoreServices(shared)
	if err != nil {
		t.Fatalf("buildPilotCoreServices: %v", err)
	}

	actionSvc, ok := core.Action.(*action.Service)
	if !ok {
		t.Fatalf("expected action service, got %T", core.Action)
	}
	state, err := actionSvc.CheckManifestHealth(context.Background(), connectors.Manifest{
		APIVersion: "tars.connector/v1alpha1",
		Kind:       "connector",
		Metadata: connectors.Metadata{
			ID:          "victorialogs-main",
			Name:        "victorialogs",
			DisplayName: "VictoriaLogs Main",
			Vendor:      "victoriametrics",
			Version:     "1.0.0",
		},
		Spec: connectors.Spec{
			Type:     "logs",
			Protocol: "victorialogs_http",
			Capabilities: []connectors.Capability{
				{ID: "logs.query", Action: "query", ReadOnly: true, Invocable: true},
			},
			ConnectionForm: []connectors.Field{
				{Key: "base_url", Label: "Base URL", Type: "string", Required: true},
			},
			ImportExport: connectors.ImportExport{Exportable: true, Importable: true, Formats: []string{"yaml", "json"}},
		},
		Config:        connectors.RuntimeConfig{Values: map[string]string{"base_url": ""}},
		Compatibility: connectors.Compatibility{TARSMajorVersions: []string{"1"}},
	})
	if err != nil {
		t.Fatalf("expected app bootstrap to support victorialogs_http health probes, got %v", err)
	}
	if state.Health.Status != "unhealthy" || !strings.Contains(state.Health.Summary, "base_url") {
		t.Fatalf("expected registered runtime to report missing base_url as unhealthy, got %+v", state.Health)
	}
}

func TestNewWithConfigPostgresEnablesRuntimePersistence(t *testing.T) {
	cfg := testBootstrapConfig(t)
	cfg.Postgres.DSN = "postgres://bootstrap-test"

	fakeRuntimeConfig := postgresrepo.NewRuntimeConfigStore(nil)
	fakeDB := sql.OpenDB(noopConnector{})
	t.Cleanup(func() { _ = fakeDB.Close() })

	previousOpen := openBootstrapPostgres
	t.Cleanup(func() { openBootstrapPostgres = previousOpen })
	openBootstrapPostgres = func(_ context.Context, _ config.Config, _ *slog.Logger, _ *observability.Store, auditLogger audit.Logger) (postgresBootstrap, error) {
		return postgresBootstrap{
			db:                 fakeDB,
			auditLogger:        auditLogger,
			runtimeConfigStore: fakeRuntimeConfig,
		}, nil
	}

	app, err := newWithConfig(cfg)
	if err != nil {
		t.Fatalf("newWithConfig: %v", err)
	}
	if app.services.RuntimeConfig != fakeRuntimeConfig {
		t.Fatalf("expected runtime config store to be wired")
	}

	err = app.services.Access.SaveConfig(access.Config{
		Users: []access.User{{UserID: "u-1", Username: "alice"}},
	})
	if err != nil {
		t.Fatalf("save access config: %v", err)
	}
	saved, found, err := fakeRuntimeConfig.LoadAccessConfig(context.Background())
	if err != nil {
		t.Fatalf("load access config: %v", err)
	}
	if !found || len(saved.Users) != 1 || saved.Users[0].UserID != "u-1" {
		t.Fatalf("expected persisted access config, got found=%v cfg=%+v", found, saved)
	}

	if err := app.services.Authz.SaveConfig(authorization.Config{
		Defaults: authorization.Defaults{WhitelistAction: authorization.ActionDirectExecute},
	}); err != nil {
		t.Fatalf("save authorization config: %v", err)
	}
	savedAuthz, found, err := fakeRuntimeConfig.LoadAuthorizationConfig(context.Background())
	if err != nil {
		t.Fatalf("load authorization config: %v", err)
	}
	if !found || savedAuthz.Defaults.WhitelistAction != authorization.ActionDirectExecute {
		t.Fatalf("expected persisted authorization config, got found=%v cfg=%+v", found, savedAuthz)
	}

	if err := app.services.Approval.SaveConfig(approvalrouting.Config{
		ProhibitSelfApproval: true,
	}); err != nil {
		t.Fatalf("save approval routing config: %v", err)
	}
	savedApproval, found, err := fakeRuntimeConfig.LoadApprovalRoutingConfig(context.Background())
	if err != nil {
		t.Fatalf("load approval routing config: %v", err)
	}
	if !found || !savedApproval.ProhibitSelfApproval {
		t.Fatalf("expected persisted approval routing config, got found=%v cfg=%+v", found, savedApproval)
	}

	prompts := *reasoning.DefaultPromptSet()
	prompts.SystemPrompt = "persisted diagnosis prompt"
	if err := app.services.Prompts.SavePromptSet(prompts); err != nil {
		t.Fatalf("save prompts config: %v", err)
	}
	savedPrompts, found, err := fakeRuntimeConfig.LoadReasoningPromptsConfig(context.Background())
	if err != nil {
		t.Fatalf("load prompts config: %v", err)
	}
	if !found || savedPrompts.SystemPrompt != "persisted diagnosis prompt" {
		t.Fatalf("expected persisted prompts config, got found=%v cfg=%+v", found, savedPrompts)
	}

	desense := reasoning.DefaultDesensitizationConfig()
	desense.Enabled = true
	if err := app.services.Desense.SaveConfig(desense); err != nil {
		t.Fatalf("save desensitization config: %v", err)
	}
	savedDesense, found, err := fakeRuntimeConfig.LoadDesensitizationConfig(context.Background())
	if err != nil {
		t.Fatalf("load desensitization config: %v", err)
	}
	if !found || !savedDesense.Enabled {
		t.Fatalf("expected persisted desensitization config, got found=%v cfg=%+v", found, savedDesense)
	}

	createdOrg, err := app.services.Org.CreateOrganization(org.Organization{
		ID:   "engineering",
		Name: "Engineering",
		Slug: "engineering",
	})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	savedOrg, found, err := fakeRuntimeConfig.LoadOrgConfig(context.Background())
	if err != nil {
		t.Fatalf("load org config: %v", err)
	}
	if !found || len(savedOrg.Organizations) == 0 || savedOrg.Organizations[len(savedOrg.Organizations)-1].ID != createdOrg.ID {
		t.Fatalf("expected persisted org config, got found=%v cfg=%+v", found, savedOrg)
	}

	createdRole, err := app.services.AgentRoles.Create(agentrole.AgentRole{
		RoleID:      "incident_commander",
		DisplayName: "Incident Commander",
		Description: "Coordinates incident response.",
		ModelBinding: agentrole.ModelBinding{
			InheritPlatformDefault: true,
		},
	})
	if err != nil {
		t.Fatalf("create agent role: %v", err)
	}
	savedRoles, found, err := fakeRuntimeConfig.LoadAgentRolesConfig(context.Background())
	if err != nil {
		t.Fatalf("load agent roles config: %v", err)
	}
	if !found {
		t.Fatalf("expected persisted agent role config")
	}
	roleFound := false
	for _, role := range savedRoles.AgentRoles {
		if role.RoleID == createdRole.RoleID {
			roleFound = true
			break
		}
	}
	if !roleFound {
		t.Fatalf("expected persisted agent role %q in %+v", createdRole.RoleID, savedRoles)
	}
}

func TestBuildSharedBootstrapWiresAuditIntoSSHCredentialManager(t *testing.T) {
	cfg := testBootstrapConfig(t)
	cfg.Postgres.DSN = "postgres://bootstrap-test"
	cfg.SecretCustody.EncryptionKey = "0123456789abcdef0123456789abcdef"

	fakeRuntimeConfig := postgresrepo.NewRuntimeConfigStore(nil)
	fakeDB := sql.OpenDB(noopConnector{})
	captureAudit := &testAuditLogger{}
	t.Cleanup(func() { _ = fakeDB.Close() })

	previousOpen := openBootstrapPostgres
	previousVaultFactory := sshSecretVaultFactory
	previousRepoFactory := sshCredentialRepoFactory
	t.Cleanup(func() {
		openBootstrapPostgres = previousOpen
		sshSecretVaultFactory = previousVaultFactory
		sshCredentialRepoFactory = previousRepoFactory
	})
	openBootstrapPostgres = func(_ context.Context, _ config.Config, _ *slog.Logger, _ *observability.Store, auditLogger audit.Logger) (postgresBootstrap, error) {
		return postgresBootstrap{
			db:                 fakeDB,
			auditLogger:        audit.NewComposite(auditLogger, captureAudit),
			runtimeConfigStore: fakeRuntimeConfig,
		}, nil
	}
	sshSecretVaultFactory = func(_ *sql.DB, _ string, _ string) (sshcredentials.SecretBackend, error) {
		return sshcredentials.NewMemorySecretBackend(), nil
	}
	sshCredentialRepoFactory = func(_ *sql.DB) sshcredentials.Repository {
		return sshcredentials.NewMemoryRepository()
	}

	shared, err := buildSharedBootstrap(cfg)
	if err != nil {
		t.Fatalf("buildSharedBootstrap: %v", err)
	}
	if shared.sshCredentialManager == nil {
		t.Fatalf("expected ssh credential manager to be configured")
	}

	if _, err := shared.sshCredentialManager.Create(context.Background(), sshcredentials.CreateInput{
		CredentialID:   "ops-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       sshcredentials.AuthTypePassword,
		Password:       "pw",
		HostScope:      "192.168.3.100",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := shared.sshCredentialManager.Resolve(context.Background(), "ops-key", "192.168.3.100"); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	entries, err := captureAudit.List(context.Background(), audit.ListFilter{Action: "ssh_credential.used"})
	if err != nil {
		t.Fatalf("List() audit entries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected ssh credential use audit entry to be recorded")
	}
}

type testAuditLogger struct {
	entries []audit.Record
}

func (l *testAuditLogger) Log(_ context.Context, entry audit.Entry) {
	l.entries = append(l.entries, audit.Record{
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		Action:       entry.Action,
		Actor:        entry.Actor,
		TraceID:      entry.TraceID,
		Metadata:     entry.Metadata,
	})
}

func (l *testAuditLogger) List(_ context.Context, filter audit.ListFilter) ([]audit.Record, error) {
	if strings.TrimSpace(filter.Action) == "" {
		return append([]audit.Record(nil), l.entries...), nil
	}
	out := make([]audit.Record, 0, len(l.entries))
	for _, entry := range l.entries {
		if entry.Action == filter.Action {
			out = append(out, entry)
		}
	}
	return out, nil
}

type noopConnector struct{}

func (noopConnector) Connect(context.Context) (driver.Conn, error) { return noopConn{}, nil }
func (noopConnector) Driver() driver.Driver                        { return noopDriver{} }

type noopDriver struct{}

func (noopDriver) Open(string) (driver.Conn, error) { return noopConn{}, nil }

type noopConn struct{}

func (noopConn) Prepare(string) (driver.Stmt, error) { return noopStmt{}, nil }
func (noopConn) Close() error                        { return nil }
func (noopConn) Begin() (driver.Tx, error)           { return noopTx{}, nil }
func (noopConn) Ping(context.Context) error          { return nil }

type noopStmt struct{}

func (noopStmt) Close() error                               { return nil }
func (noopStmt) NumInput() int                              { return -1 }
func (noopStmt) Exec([]driver.Value) (driver.Result, error) { return noopResult(0), nil }
func (noopStmt) Query([]driver.Value) (driver.Rows, error)  { return noopRows{}, nil }
func (noopStmt) ExecContext(context.Context, []driver.NamedValue) (driver.Result, error) {
	return noopResult(0), nil
}
func (noopStmt) QueryContext(context.Context, []driver.NamedValue) (driver.Rows, error) {
	return noopRows{}, nil
}

type noopTx struct{}

func (noopTx) Commit() error   { return nil }
func (noopTx) Rollback() error { return nil }

type noopResult int64

func (r noopResult) LastInsertId() (int64, error) { return int64(r), nil }
func (r noopResult) RowsAffected() (int64, error) { return int64(r), nil }

type noopRows struct{}

func (noopRows) Columns() []string              { return []string{"ok"} }
func (noopRows) Close() error                   { return nil }
func (noopRows) Next(dest []driver.Value) error { return io.EOF }

var _ driver.Pinger = noopConn{}
var _ driver.StmtExecContext = noopStmt{}
var _ driver.StmtQueryContext = noopStmt{}
