package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
	sshclient "tars/internal/modules/action/ssh"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/connectors"
)

func TestExecuteApprovedRejectsDisallowedHost(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor: &fakeExecutor{},
		AllowedHosts: []string{
			"127.0.0.1",
		},
		OutputSpoolDir: t.TempDir(),
	})

	_, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "192.168.3.106",
		Command:     "hostname && uptime",
	})
	if err == nil || !strings.Contains(err.Error(), "allowlist") {
		t.Fatalf("expected allowlist error, got %v", err)
	}
}

func TestExecuteApprovedRejectsBlockedCommand(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor: &fakeExecutor{},
		AllowedHosts: []string{
			"127.0.0.1",
		},
		OutputSpoolDir: t.TempDir(),
	})

	_, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "127.0.0.1",
		Command:     "hostname && rm -rf /tmp/demo",
	})
	if err == nil || !strings.Contains(err.Error(), "blocked fragment") {
		t.Fatalf("expected blocked fragment error, got %v", err)
	}
}

func TestExecuteApprovedAllowsServiceScopedCommand(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 0,
				Output:   "restarted",
			},
		},
		AllowedHosts:   []string{"127.0.0.1"},
		ApprovalRouter: approvalrouting.New(approvalrouting.Config{CommandAllowlist: map[string][]string{"sshd": {"systemctl restart sshd"}}}),
		OutputSpoolDir: t.TempDir(),
	})

	result, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "127.0.0.1",
		Service:     "sshd",
		Command:     "systemctl restart sshd",
	})
	if err != nil {
		t.Fatalf("execute approved: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed status, got %+v", result)
	}
}

func TestExecuteApprovedAllowsReadOnlyExitIPCommand(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 0,
				Output:   "203.0.113.10\n",
			},
		},
		AllowedHosts:   []string{"127.0.0.1"},
		OutputSpoolDir: t.TempDir(),
	})

	result, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-exit-ip",
		SessionID:   "ses-exit-ip",
		TargetHost:  "127.0.0.1",
		Command:     "curl -fsS https://api.ipify.org && echo",
	})
	if err != nil {
		t.Fatalf("execute approved: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed status, got %+v", result)
	}
	if !strings.Contains(result.OutputPreview, "203.0.113.10") {
		t.Fatalf("unexpected output preview: %+v", result)
	}
}

func TestExecuteApprovedAllowsPolicyApprovedCommandOutsideLegacyAllowlist(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 0,
				Output:   "restarted",
			},
		},
		AllowedHosts: []string{"127.0.0.1"},
		AuthorizationPolicy: authorization.New(authorization.Config{
			Defaults: authorization.Defaults{
				WhitelistAction: authorization.ActionDirectExecute,
				BlacklistAction: authorization.ActionSuggestOnly,
				UnmatchedAction: authorization.ActionRequireApproval,
			},
		}),
		OutputSpoolDir: t.TempDir(),
	})

	result, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-policy-approved",
		SessionID:   "ses-policy-approved",
		TargetHost:  "127.0.0.1",
		Service:     "sshd",
		Command:     "systemctl restart sshd",
	})
	if err != nil {
		t.Fatalf("execute approved: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed status, got %+v", result)
	}
}

func TestExecuteApprovedRejectsSuggestOnlyCommandByPolicy(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor:     &fakeExecutor{},
		AllowedHosts: []string{"127.0.0.1"},
		AuthorizationPolicy: authorization.New(authorization.Config{
			Defaults: authorization.Defaults{
				WhitelistAction: authorization.ActionDirectExecute,
				BlacklistAction: authorization.ActionSuggestOnly,
				UnmatchedAction: authorization.ActionRequireApproval,
			},
			SSH: authorization.SSHCommandConfig{
				NormalizeWhitespace: true,
				Blacklist:           []string{"systemctl restart *"},
			},
		}),
		OutputSpoolDir: t.TempDir(),
	})

	_, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-policy-blocked",
		SessionID:   "ses-policy-blocked",
		TargetHost:  "127.0.0.1",
		Command:     "systemctl restart sshd",
	})
	if err == nil || !strings.Contains(err.Error(), "manual handling") {
		t.Fatalf("expected suggest_only policy error, got %v", err)
	}
}

func TestExecuteApprovedRejectsCommandOutsideServiceAllowlist(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor:       &fakeExecutor{},
		AllowedHosts:   []string{"127.0.0.1"},
		ApprovalRouter: approvalrouting.New(approvalrouting.Config{CommandAllowlist: map[string][]string{"sshd": {"systemctl restart sshd"}}}),
		OutputSpoolDir: t.TempDir(),
	})

	_, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "127.0.0.1",
		Service:     "sshd",
		Command:     "systemctl restart nginx",
	})
	if err == nil || !strings.Contains(err.Error(), "service allowlist for sshd") {
		t.Fatalf("expected service allowlist error, got %v", err)
	}
}

func TestExecuteApprovedPersistsOutput(t *testing.T) {
	t.Parallel()

	spoolDir := t.TempDir()
	svc := NewService(Options{
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 0,
				Output:   "hostname\nup 1 day",
			},
		},
		AllowedHosts:   []string{"127.0.0.1"},
		OutputSpoolDir: spoolDir,
	})

	result, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "127.0.0.1",
		Command:     "hostname && uptime",
	})
	if err != nil {
		t.Fatalf("execute approved: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed status, got %s", result.Status)
	}
	if result.OutputRef == "" {
		t.Fatalf("expected output ref, got empty")
	}
	if filepath.Dir(result.OutputRef) != spoolDir {
		t.Fatalf("unexpected spool dir: %s", result.OutputRef)
	}
}

func TestQueryMetricsFallsBackWhenProviderErrors(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		MetricsProvider: failingMetricsProvider{err: errors.New("vm unavailable")},
	})

	result, err := svc.QueryMetrics(context.Background(), contracts.MetricsQuery{
		Service: "api",
		Host:    "host-1",
	})
	if err != nil {
		t.Fatalf("query metrics: %v", err)
	}
	if len(result.Series) != 1 {
		t.Fatalf("expected fallback series, got %+v", result.Series)
	}
	if result.Series[0]["source"] != "stub" {
		t.Fatalf("expected stub source, got %+v", result.Series[0])
	}
}

func TestQueryMetricsUsesConnectorRuntimeWhenConnectorProvided(t *testing.T) {
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
      config:
        values:
          base_url: https://prometheus.example.test
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

	svc := NewService(Options{
		Connectors: manager,
		QueryRuntimes: map[string]QueryRuntime{
			"prometheus_http": fakeQueryRuntime{},
		},
	})

	result, err := svc.QueryMetrics(context.Background(), contracts.MetricsQuery{
		Host:        "host-1",
		ConnectorID: "prometheus-main",
		Protocol:    "prometheus_http",
	})
	if err != nil {
		t.Fatalf("query metrics: %v", err)
	}
	if len(result.Series) != 1 || result.Series[0]["source"] != "connector-runtime" {
		t.Fatalf("unexpected metrics result: %+v", result)
	}
}

func TestQueryMetricsReturnsErrorWhenExplicitConnectorRuntimeFails(t *testing.T) {
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
      config:
        values:
          base_url: https://prometheus.example.test
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

	svc := NewService(Options{
		Connectors: manager,
		QueryRuntimes: map[string]QueryRuntime{
			"prometheus_http": failingQueryRuntime{err: errors.New("connector runtime unavailable")},
		},
		MetricsProvider: staticMetricsProvider{result: contracts.MetricsResult{Series: []map[string]interface{}{{"source": "legacy-provider"}}}},
	})

	_, err = svc.QueryMetrics(context.Background(), contracts.MetricsQuery{
		Host:        "host-1",
		ConnectorID: "prometheus-main",
		Protocol:    "prometheus_http",
	})
	if err == nil || !strings.Contains(err.Error(), "connector runtime unavailable") {
		t.Fatalf("expected explicit connector runtime error, got %v", err)
	}
}

func TestQueryMetricsResolvesExplicitConnectorAlias(t *testing.T) {
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
      config:
        values:
          base_url: https://prometheus.example.test
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

	svc := NewService(Options{
		Connectors: manager,
		QueryRuntimes: map[string]QueryRuntime{
			"prometheus_http": fakeQueryRuntime{},
		},
	})

	result, err := svc.QueryMetrics(context.Background(), contracts.MetricsQuery{
		Host:        "127.0.0.1:9100",
		ConnectorID: "prometheus",
	})
	if err != nil {
		t.Fatalf("query metrics with alias: %v", err)
	}
	if len(result.Series) != 1 || result.Series[0]["source"] != "connector-runtime" {
		t.Fatalf("unexpected metrics result: %+v", result)
	}
	if result.Runtime == nil || result.Runtime.ConnectorID != "prometheus-main" {
		t.Fatalf("expected resolved connector metadata, got %+v", result.Runtime)
	}
}

func TestQueryMetricsFallsBackWhenAutoSelectedConnectorRuntimeFails(t *testing.T) {
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
      config:
        values:
          base_url: https://prometheus.example.test
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

	svc := NewService(Options{
		Connectors: manager,
		QueryRuntimes: map[string]QueryRuntime{
			"prometheus_http": failingQueryRuntime{err: errors.New("connector runtime unavailable")},
		},
		MetricsProvider: staticMetricsProvider{result: contracts.MetricsResult{Series: []map[string]interface{}{{"source": "legacy-provider"}}}},
	})

	result, err := svc.QueryMetrics(context.Background(), contracts.MetricsQuery{
		Host:     "host-1",
		Protocol: "prometheus_http",
	})
	if err != nil {
		t.Fatalf("query metrics: %v", err)
	}
	if len(result.Series) != 1 || result.Series[0]["source"] != "legacy-provider" {
		t.Fatalf("expected legacy provider fallback, got %+v", result)
	}
	if result.Runtime == nil || !result.Runtime.FallbackUsed || result.Runtime.Runtime != "legacy_provider" {
		t.Fatalf("expected fallback runtime metadata, got %+v", result.Runtime)
	}
}

func TestExecuteApprovedMarksTruncatedOutput(t *testing.T) {
	t.Parallel()

	registry := foundationmetrics.New()
	svc := NewService(Options{
		Metrics: registry,
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 0,
				Output:   "0123456789",
			},
		},
		AllowedHosts:            []string{"127.0.0.1"},
		OutputSpoolDir:          t.TempDir(),
		MaxPersistedOutputBytes: 4,
	})

	result, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "127.0.0.1",
		Command:     "hostname",
	})
	if err != nil {
		t.Fatalf("execute approved: %v", err)
	}
	if result.OutputBytes != 10 {
		t.Fatalf("expected output bytes 10, got %d", result.OutputBytes)
	}
	if !result.OutputTruncated {
		t.Fatalf("expected output to be marked truncated")
	}
	if result.OutputPreview != "0123" {
		t.Fatalf("unexpected output preview: %q", result.OutputPreview)
	}

	var output strings.Builder
	if err := registry.WritePrometheus(&output); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	if !strings.Contains(output.String(), "tars_execution_output_truncated_total 1") {
		t.Fatalf("expected truncated metric, got:\n%s", output.String())
	}
}

func TestVerifyExecutionPassesWhenServiceIsActive(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 0,
				Output:   "active\n",
			},
		},
		AllowedHosts: []string{"127.0.0.1"},
	})

	result, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "127.0.0.1",
		Service:     "sshd",
	})
	if err != nil {
		t.Fatalf("verify execution: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("expected success, got %+v", result)
	}
	if !strings.Contains(result.Summary, "sshd is active") {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
}

func TestVerifyExecutionFallsBackToServiceAlias(t *testing.T) {
	t.Parallel()

	var commands []string
	svc := NewService(Options{
		Executor: &fakeExecutor{
			runFunc: func(_ context.Context, _ string, command string) (sshclient.Result, error) {
				commands = append(commands, command)
				switch command {
				case "systemctl is-active ssh":
					return sshclient.Result{ExitCode: 4, Output: "Unit ssh.service could not be found\n"}, sshclient.ErrRemoteCommandFailed
				case "systemctl is-active sshd":
					return sshclient.Result{ExitCode: 0, Output: "active\n"}, nil
				default:
					return sshclient.Result{}, nil
				}
			},
		},
		AllowedHosts: []string{"127.0.0.1"},
	})

	result, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "127.0.0.1",
		Service:     "ssh",
	})
	if err != nil {
		t.Fatalf("verify execution: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("expected success, got %+v", result)
	}
	if !strings.Contains(result.Summary, "sshd is active") {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if matched, _ := result.Details["matched_service"].(string); matched != "sshd" {
		t.Fatalf("expected matched service sshd, got %+v", result.Details)
	}
	if !slices.Equal(commands, []string{"systemctl is-active ssh", "systemctl is-active sshd"}) {
		t.Fatalf("unexpected commands: %+v", commands)
	}
}

func TestVerificationServiceCandidatesIncludesKnownAliases(t *testing.T) {
	t.Parallel()

	if got := VerificationServiceCandidates("ssh"); !slices.Equal(got, []string{"ssh", "sshd"}) {
		t.Fatalf("unexpected ssh candidates: %+v", got)
	}
	if got := VerificationServiceCandidates("postgresql"); !slices.Equal(got, []string{"postgresql", "postgres"}) {
		t.Fatalf("unexpected postgres candidates: %+v", got)
	}
}

func TestVerifyExecutionFailsWhenServiceIsInactive(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 3,
				Output:   "inactive\n",
			},
			err: sshclient.ErrRemoteCommandFailed,
		},
		AllowedHosts: []string{"127.0.0.1"},
	})

	result, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "127.0.0.1",
		Service:     "sshd",
	})
	if err != nil {
		t.Fatalf("verify execution: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed verification, got %+v", result)
	}
	if !strings.Contains(result.Summary, "not active") {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
}

func TestVerifyExecutionSkipsWithoutServiceHint(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor:     &fakeExecutor{},
		AllowedHosts: []string{"127.0.0.1"},
	})

	result, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-1",
		SessionID:   "ses-1",
		TargetHost:  "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("verify execution: %v", err)
	}
	if result.Status != "skipped" {
		t.Fatalf("expected skipped verification, got %+v", result)
	}
}

func TestExecuteApprovedRoutesToJumpServerRuntime(t *testing.T) {
	t.Parallel()

	manager := &connectors.Manager{}
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
      config:
        values:
          base_url: https://jumpserver.example.test
          access_key: ak-test
          secret_key: sk-test
      compatibility:
        tars_major_versions: ["1"]
`
	var err error
	if err := os.WriteFile(configPath, []byte("connectors:\n  entries: []\n"), 0o600); err != nil {
		t.Fatalf("seed connectors config: %v", err)
	}
	manager, err = connectors.NewManager(configPath)
	if err != nil {
		t.Fatalf("new connectors manager: %v", err)
	}
	if err := manager.Save(content); err != nil {
		t.Fatalf("save connectors config: %v", err)
	}

	svc := NewService(Options{
		Executor:   &fakeExecutor{},
		Connectors: manager,
		ExecutionRuntimes: map[string]ExecutionRuntime{
			"jumpserver_api": fakeExecutionRuntime{},
		},
		OutputSpoolDir: t.TempDir(),
	})

	result, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID:   "exe-jumpserver",
		SessionID:     "ses-jumpserver",
		TargetHost:    "192.168.3.106",
		Command:       "systemctl restart sshd",
		ConnectorID:   "jumpserver-main",
		Protocol:      "jumpserver_api",
		ExecutionMode: "jumpserver_job",
	})
	if err != nil {
		t.Fatalf("execute approved: %v", err)
	}
	if result.Status != "completed" || result.ConnectorID != "jumpserver-main" || result.ExecutionMode != "jumpserver_job" {
		t.Fatalf("unexpected execution result: %+v", result)
	}
}

func TestInvokeCapabilityReturnsPendingApprovalWithoutTransportError(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "connectors.yaml")
	content := `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: delivery-main
        name: delivery
        display_name: Delivery Main
        vendor: acme
        version: 1.0.0
      spec:
        type: delivery
        protocol: delivery_api
        capabilities:
          - id: deployment.promote
            action: invoke
            read_only: false
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

	svc := NewService(Options{
		Connectors:          manager,
		AuthorizationPolicy: mustAuthorizationManager(t, ""),
		CapabilityRuntimes: map[string]CapabilityRuntime{
			"delivery": fakeCapabilityRuntime{},
		},
	})

	result, err := svc.InvokeCapability(context.Background(), contracts.CapabilityRequest{
		ConnectorID:  "delivery-main",
		CapabilityID: "deployment.promote",
		Params:       map[string]interface{}{"service": "api"},
	})
	if err != nil {
		t.Fatalf("expected pending approval without transport error, got %v", err)
	}
	if result.Status != "pending_approval" {
		t.Fatalf("expected pending_approval, got %+v", result)
	}
	if result.Metadata["rule_id"] != "default" {
		t.Fatalf("expected default approval rule metadata, got %+v", result.Metadata)
	}
}

func TestCheckConnectorHealthUsesCapabilityRuntime(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "connectors.yaml")
	content := `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: observability-main
        name: observability
        display_name: Observability Main
        vendor: tars
        version: 1.0.0
      spec:
        type: observability
        protocol: observability_http
        capabilities:
          - id: observability.query
            action: query
            read_only: true
        connection_form:
          - key: base_url
            label: Base URL
            type: string
            required: true
      config:
        values:
          base_url: http://127.0.0.1:18080
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

	svc := NewService(Options{
		Connectors: manager,
		CapabilityRuntimes: map[string]CapabilityRuntime{
			"observability_http": fakeCapabilityRuntime{},
		},
	})

	state, err := svc.CheckConnectorHealth(context.Background(), "observability-main")
	if err != nil {
		t.Fatalf("check connector health: %v", err)
	}
	if state.Health.Status != "healthy" {
		t.Fatalf("expected healthy capability runtime health, got %+v", state.Health)
	}
}

func TestInvokeCapabilityNormalizesSkillSourceForRuntimeSelection(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "connectors.yaml")
	content := `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: skill-source-main
        name: skill-source
        display_name: Skill Source
        vendor: tars
        version: 1.0.0
      spec:
        type: skill_source
        protocol: http_index
        capabilities:
          - id: source.sync
            action: import
            read_only: true
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

	svc := NewService(Options{
		Connectors: manager,
		CapabilityRuntimes: map[string]CapabilityRuntime{
			"skill": fakeCapabilityRuntime{},
		},
	})

	result, err := svc.InvokeCapability(context.Background(), contracts.CapabilityRequest{
		ConnectorID:  "skill-source-main",
		CapabilityID: "source.sync",
		Params:       map[string]interface{}{"source": "default"},
	})
	if err != nil {
		t.Fatalf("invoke capability: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed result, got %+v", result)
	}
	if result.Runtime == nil || result.Runtime.ConnectorType != "skill_source" {
		t.Fatalf("expected runtime metadata for skill_source connector, got %+v", result.Runtime)
	}
	if result.Output["source"] != "fake-capability-runtime" {
		t.Fatalf("unexpected runtime output: %+v", result.Output)
	}
}

func TestInvokeCapabilityNormalizesSkillSourceForHardDeny(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "connectors.yaml")
	content := `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: skill-source-main
        name: skill-source
        display_name: Skill Source
        vendor: tars
        version: 1.0.0
      spec:
        type: skill_source
        protocol: http_index
        capabilities:
          - id: source.sync
            action: import
            read_only: true
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

	policy := authorization.New(authorization.Config{
		HardDeny: authorization.HardDenyConfig{
			MCPSkill: []string{"source.sync"},
		},
	})
	svc := NewService(Options{
		Connectors:          manager,
		AuthorizationPolicy: policy,
		CapabilityRuntimes:  map[string]CapabilityRuntime{"skill": fakeCapabilityRuntime{}},
	})

	result, err := svc.InvokeCapability(context.Background(), contracts.CapabilityRequest{
		ConnectorID:  "skill-source-main",
		CapabilityID: "source.sync",
	})
	if err != nil {
		t.Fatalf("expected denied result without transport error, got %v", err)
	}
	if result.Status != "denied" {
		t.Fatalf("expected denied result, got %+v", result)
	}
	if result.Metadata["matched_by"] != "hard_deny" {
		t.Fatalf("expected hard deny metadata, got %+v", result.Metadata)
	}
}

type fakeExecutor struct {
	result  sshclient.Result
	err     error
	runFunc func(context.Context, string, string) (sshclient.Result, error)
}

type fakeExecutionRuntime struct{}

type fakeQueryRuntime struct{}

type fakeCapabilityRuntime struct{}

type failingQueryRuntime struct {
	err error
}

type staticMetricsProvider struct {
	result contracts.MetricsResult
	err    error
}

func (fakeQueryRuntime) Query(_ context.Context, manifest connectors.Manifest, query contracts.MetricsQuery) (contracts.MetricsResult, error) {
	return contracts.MetricsResult{Series: []map[string]interface{}{{
		"connector_id": manifest.Metadata.ID,
		"host":         query.Host,
		"source":       "connector-runtime",
	}}}, nil
}

func (f failingQueryRuntime) Query(_ context.Context, _ connectors.Manifest, _ contracts.MetricsQuery) (contracts.MetricsResult, error) {
	return contracts.MetricsResult{}, f.err
}

func (fakeExecutionRuntime) Execute(_ context.Context, manifest connectors.Manifest, req contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
	return contracts.ExecutionResult{
		ExecutionID:   req.ExecutionID,
		SessionID:     req.SessionID,
		Status:        "completed",
		ConnectorID:   manifest.Metadata.ID,
		Protocol:      manifest.Spec.Protocol,
		ExecutionMode: req.ExecutionMode,
	}, nil
}

func (fakeExecutionRuntime) Verify(_ context.Context, _ connectors.Manifest, req contracts.VerificationRequest) (contracts.VerificationResult, error) {
	return contracts.VerificationResult{
		SessionID:   req.SessionID,
		ExecutionID: req.ExecutionID,
		Status:      "skipped",
	}, nil
}

func (fakeCapabilityRuntime) Invoke(_ context.Context, manifest connectors.Manifest, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	return contracts.CapabilityResult{
		Status: "completed",
		Output: map[string]interface{}{
			"source":        "fake-capability-runtime",
			"connector_id":  manifest.Metadata.ID,
			"capability_id": capabilityID,
			"params":        params,
		},
		Runtime: &contracts.RuntimeMetadata{
			Runtime:         "fake_capability_runtime",
			Selection:       "explicit_connector",
			ConnectorID:     manifest.Metadata.ID,
			ConnectorType:   manifest.Spec.Type,
			ConnectorVendor: manifest.Metadata.Vendor,
			Protocol:        manifest.Spec.Protocol,
		},
	}, nil
}

func (fakeCapabilityRuntime) CheckHealth(_ context.Context, manifest connectors.Manifest) (string, string, error) {
	return "healthy", fmt.Sprintf("capability runtime healthy for %s", manifest.Metadata.ID), nil
}

func (f *fakeExecutor) Run(ctx context.Context, host string, command string) (sshclient.Result, error) {
	if f.runFunc != nil {
		return f.runFunc(ctx, host, command)
	}
	return f.result, f.err
}

type failingMetricsProvider struct {
	err error
}

func (f failingMetricsProvider) Query(_ context.Context, _ contracts.MetricsQuery) (contracts.MetricsResult, error) {
	return contracts.MetricsResult{}, f.err
}

func (p staticMetricsProvider) Query(_ context.Context, _ contracts.MetricsQuery) (contracts.MetricsResult, error) {
	return p.result, p.err
}

func mustAuthorizationManager(t *testing.T, path string) *authorization.Manager {
	t.Helper()

	manager, err := authorization.NewManager(path)
	if err != nil {
		t.Fatalf("new authorization manager: %v", err)
	}
	return manager
}
