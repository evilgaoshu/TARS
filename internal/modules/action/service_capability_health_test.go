package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tars/internal/contracts"
	sshclient "tars/internal/modules/action/ssh"
	"tars/internal/modules/authorization"
	"tars/internal/modules/connectors"
)

func TestAppendCompatibilitySummary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		runtimeSummary string
		compatSummary  string
		want           string
	}{
		{
			name:          "runtime empty",
			compatSummary: "compatibility reason",
			want:          "compatibility reason",
		},
		{
			name:           "compatibility empty",
			runtimeSummary: "runtime summary",
			want:           "runtime summary",
		},
		{
			name:           "same summary",
			runtimeSummary: "runtime summary",
			compatSummary:  "runtime summary",
			want:           "runtime summary",
		},
		{
			name:           "compatibility already present",
			runtimeSummary: "runtime summary; compatibility reason",
			compatSummary:  "compatibility reason",
			want:           "runtime summary; compatibility reason",
		},
		{
			name:           "append compatibility summary",
			runtimeSummary: "runtime summary",
			compatSummary:  "compatibility reason",
			want:           "runtime summary; compatibility reason",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := appendCompatibilitySummary(tc.runtimeSummary, tc.compatSummary)
			if got != tc.want {
				t.Fatalf("appendCompatibilitySummary(%q, %q) = %q, want %q", tc.runtimeSummary, tc.compatSummary, got, tc.want)
			}
		})
	}
}

func TestCheckConnectorHealthQueryRuntime(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"prometheus-main",
		"Prometheus Main",
		"prometheus",
		"metrics",
		"prometheus_http",
		[]string{"1"},
	))

	svc := NewService(Options{
		Connectors: manager,
		QueryRuntimes: map[string]QueryRuntime{
			"prometheus_http": testConnectorRuntime{
				healthFn: func(context.Context, connectors.Manifest) (string, string, error) {
					return "healthy", "query checker ran", nil
				},
			},
		},
	})

	state, err := svc.CheckConnectorHealth(context.Background(), "prometheus-main")
	if err != nil {
		t.Fatalf("check connector health: %v", err)
	}
	if state.Health.Status != "healthy" {
		t.Fatalf("expected healthy connector health, got %+v", state.Health)
	}
	if state.Health.Summary != "query checker ran" {
		t.Fatalf("expected query checker summary, got %+v", state.Health)
	}
}

func TestCheckConnectorHealthExecutionRuntimeError(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"jumpserver-main",
		"JumpServer Main",
		"jumpserver",
		"execution",
		"jumpserver_api",
		[]string{"1"},
	))

	svc := NewService(Options{
		Connectors: manager,
		ExecutionRuntimes: map[string]ExecutionRuntime{
			"jumpserver_api": testConnectorRuntime{
				healthFn: func(context.Context, connectors.Manifest) (string, string, error) {
					return "unhealthy", "", errors.New("draft health probe failed")
				},
			},
		},
	})

	state, err := svc.CheckConnectorHealth(context.Background(), "jumpserver-main")
	if err != nil {
		t.Fatalf("check connector health: %v", err)
	}
	if state.Health.Status != "unhealthy" {
		t.Fatalf("expected unhealthy connector health, got %+v", state.Health)
	}
	if !strings.Contains(state.Health.Summary, "draft health probe failed") {
		t.Fatalf("expected runtime error summary, got %+v", state.Health)
	}
}

func TestCheckConnectorHealthQueryRuntimeError(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"prometheus-main",
		"Prometheus Main",
		"prometheus",
		"metrics",
		"prometheus_http",
		[]string{"1"},
	))

	svc := NewService(Options{
		Connectors: manager,
		QueryRuntimes: map[string]QueryRuntime{
			"prometheus_http": testConnectorRuntime{
				healthFn: func(context.Context, connectors.Manifest) (string, string, error) {
					return "degraded", "query runtime failed", errors.New("query runtime unavailable")
				},
			},
		},
	})

	state, err := svc.CheckConnectorHealth(context.Background(), "prometheus-main")
	if err != nil {
		t.Fatalf("check connector health: %v", err)
	}
	if state.Health.Status != "degraded" || !strings.Contains(state.Health.Summary, "query runtime failed") {
		t.Fatalf("unexpected query runtime health result: %+v", state.Health)
	}
}

func TestCheckConnectorHealthCapabilityRuntime(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"observability-main",
		"Observability Main",
		"tars",
		"observability",
		"observability_http",
		[]string{"1"},
	))

	svc := NewService(Options{
		Connectors: manager,
		CapabilityRuntimes: map[string]CapabilityRuntime{
			"observability_http": testConnectorRuntime{
				healthFn: func(context.Context, connectors.Manifest) (string, string, error) {
					return "healthy", "capability runtime healthy", nil
				},
			},
		},
	})

	state, err := svc.CheckConnectorHealth(context.Background(), "observability-main")
	if err != nil {
		t.Fatalf("check connector health: %v", err)
	}
	if state.Health.Status != "healthy" {
		t.Fatalf("expected healthy connector health, got %+v", state.Health)
	}
	if state.Health.Summary != "capability runtime healthy" {
		t.Fatalf("unexpected capability runtime summary: %+v", state.Health)
	}
}

func TestCheckConnectorHealthFallsBackWhenNoRuntimeCheckerIsRegistered(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"fallback-main",
		"Fallback Main",
		"prometheus",
		"metrics",
		"prometheus_http",
		[]string{"1"},
	))

	svc := NewService(Options{Connectors: manager})

	state, err := svc.CheckConnectorHealth(context.Background(), "fallback-main")
	if err != nil {
		t.Fatalf("check connector health: %v", err)
	}
	if state.Health.Status != "healthy" {
		t.Fatalf("expected fallback health to remain healthy, got %+v", state.Health)
	}
	if state.Health.Summary != "connector is enabled and compatible" {
		t.Fatalf("unexpected fallback summary: %+v", state.Health)
	}
}

func TestCheckManifestHealthQueryRuntime(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		QueryRuntimes: map[string]QueryRuntime{
			"prometheus_http": testConnectorRuntime{
				healthFn: func(context.Context, connectors.Manifest) (string, string, error) {
					return "healthy", "draft query checker ran", nil
				},
			},
		},
	})

	state, err := svc.CheckManifestHealth(context.Background(), newConnectorManifest(
		"prometheus-draft",
		"Prometheus Draft",
		"prometheus",
		"metrics",
		"prometheus_http",
		[]string{"1"},
	))
	if err != nil {
		t.Fatalf("check manifest health: %v", err)
	}
	if state.Health.Status != "healthy" {
		t.Fatalf("expected healthy draft health, got %+v", state.Health)
	}
	if state.Health.Summary != "draft query checker ran" {
		t.Fatalf("expected draft query checker summary, got %+v", state.Health)
	}
}

func TestCheckManifestHealthExecutionRuntimeError(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		ExecutionRuntimes: map[string]ExecutionRuntime{
			"jumpserver_api": testConnectorRuntime{
				healthFn: func(context.Context, connectors.Manifest) (string, string, error) {
					return "degraded", "execution draft probe failed", errors.New("execution draft probe failed")
				},
			},
		},
	})

	state, err := svc.CheckManifestHealth(context.Background(), newConnectorManifest(
		"jumpserver-draft",
		"JumpServer Draft",
		"jumpserver",
		"execution",
		"jumpserver_api",
		[]string{"1"},
	))
	if err != nil {
		t.Fatalf("check manifest health: %v", err)
	}
	if state.Health.Status != "degraded" {
		t.Fatalf("expected degraded draft health, got %+v", state.Health)
	}
	if !strings.Contains(state.Health.Summary, "execution draft probe failed") {
		t.Fatalf("expected execution probe error summary, got %+v", state.Health)
	}
}

func TestCheckManifestHealthCapabilityRuntime(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		CapabilityRuntimes: map[string]CapabilityRuntime{
			"observability_http": testConnectorRuntime{
				healthFn: func(context.Context, connectors.Manifest) (string, string, error) {
					return "healthy", "capability draft probe healthy", nil
				},
			},
		},
	})

	state, err := svc.CheckManifestHealth(context.Background(), newConnectorManifest(
		"observability-draft",
		"Observability Draft",
		"tars",
		"observability",
		"observability_http",
		[]string{"1"},
	))
	if err != nil {
		t.Fatalf("check manifest health: %v", err)
	}
	if state.Health.Status != "healthy" {
		t.Fatalf("expected healthy draft health, got %+v", state.Health)
	}
	if state.Health.Summary != "capability draft probe healthy" {
		t.Fatalf("unexpected capability draft summary: %+v", state.Health)
	}
}

func TestCheckManifestHealthAppendsCompatibilitySummary(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		QueryRuntimes: map[string]QueryRuntime{
			"prometheus_http": testConnectorRuntime{
				healthFn: func(context.Context, connectors.Manifest) (string, string, error) {
					return "healthy", "runtime probe healthy", nil
				},
			},
		},
	})

	state, err := svc.CheckManifestHealth(context.Background(), newConnectorManifest(
		"prometheus-incompatible",
		"Prometheus Incompatible",
		"prometheus",
		"metrics",
		"prometheus_http",
		[]string{"99"},
	))
	if err != nil {
		t.Fatalf("check manifest health: %v", err)
	}
	if state.Health.Status != "unhealthy" {
		t.Fatalf("expected unhealthy incompatible health, got %+v", state.Health)
	}
	if !strings.Contains(state.Health.Summary, "runtime probe healthy") || !strings.Contains(state.Health.Summary, "not compatible with current TARS major version") {
		t.Fatalf("expected combined compatibility summary, got %+v", state.Health)
	}
}

func TestCheckManifestHealthRejectsUnsupportedDraftProtocol(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{})

	_, err := svc.CheckManifestHealth(context.Background(), newConnectorManifest(
		"unknown-draft",
		"Unknown Draft",
		"tars",
		"observability",
		"unsupported_proto",
		[]string{"1"},
	))
	if err == nil || !strings.Contains(err.Error(), "does not support draft health probe") {
		t.Fatalf("expected unsupported draft protocol error, got %v", err)
	}
}

func TestInvokeCapabilityValidationAndMissingRuntime(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, capabilityTestManifest())

	cases := []struct {
		name string
		svc  *Service
		req  contracts.CapabilityRequest
		want string
	}{
		{
			name: "missing connector id",
			svc:  NewService(Options{}),
			req:  contracts.CapabilityRequest{CapabilityID: "source.sync"},
			want: "connector_id is required",
		},
		{
			name: "missing capability id",
			svc:  NewService(Options{}),
			req:  contracts.CapabilityRequest{ConnectorID: "skill-source-main"},
			want: "capability_id is required",
		},
		{
			name: "missing connector manager",
			svc:  NewService(Options{}),
			req:  contracts.CapabilityRequest{ConnectorID: "skill-source-main", CapabilityID: "source.sync"},
			want: "connector manager not available",
		},
		{
			name: "capability not found",
			svc:  NewService(Options{Connectors: manager}),
			req:  contracts.CapabilityRequest{ConnectorID: "skill-source-main", CapabilityID: "missing.capability"},
			want: "capability \"missing.capability\" not found on connector \"skill-source-main\"",
		},
		{
			name: "runtime not found",
			svc:  NewService(Options{Connectors: manager}),
			req:  contracts.CapabilityRequest{ConnectorID: "skill-source-main", CapabilityID: "source.sync"},
			want: "no capability runtime for connector type=skill_source protocol=http_index",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := tc.svc.InvokeCapability(context.Background(), tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
			if result.Status != "failed" {
				t.Fatalf("expected failed result, got %+v", result)
			}
			if !strings.Contains(result.Error, tc.want) {
				t.Fatalf("expected result error containing %q, got %+v", tc.want, result)
			}
		})
	}
}

func TestInvokeApprovedCapabilityClonesParamsAndBypassesAuthorization(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, capabilityTestManifest())
	originalParams := map[string]interface{}{"mode": "original"}

	svc := NewService(Options{
		Connectors: manager,
		AuthorizationPolicy: authorization.New(authorization.Config{
			HardDeny: authorization.HardDenyConfig{
				MCPSkill: []string{"source.sync"},
			},
		}),
		CapabilityRuntimes: map[string]CapabilityRuntime{
			"skill": testConnectorRuntime{
				invokeFn: func(_ context.Context, _ connectors.Manifest, _ string, params map[string]interface{}) (contracts.CapabilityResult, error) {
					params["mode"] = "mutated"
					return contracts.CapabilityResult{
						Status: "completed",
						Output: map[string]interface{}{
							"params": params,
						},
					}, nil
				},
			},
		},
	})

	result, err := svc.InvokeApprovedCapability(context.Background(), contracts.ApprovedCapabilityRequest{
		ConnectorID:  "skill-source-main",
		CapabilityID: "source.sync",
		Params:       originalParams,
		RequestedBy:  "reviewer",
	})
	if err != nil {
		t.Fatalf("invoke approved capability: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed approved capability result, got %+v", result)
	}
	if originalParams["mode"] != "original" {
		t.Fatalf("expected original params to remain unchanged, got %+v", originalParams)
	}
	params, ok := result.Output["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected params output, got %+v", result.Output)
	}
	if params["mode"] != "mutated" {
		t.Fatalf("expected runtime to mutate cloned params, got %+v", params)
	}
}

func TestInvokeCapabilitySetsRuntimeMetadataWhenRuntimeReturnsNil(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, capabilityTestManifest())
	svc := NewService(Options{
		Connectors: manager,
		CapabilityRuntimes: map[string]CapabilityRuntime{
			"skill": testConnectorRuntime{
				invokeFn: func(_ context.Context, _ connectors.Manifest, _ string, params map[string]interface{}) (contracts.CapabilityResult, error) {
					return contracts.CapabilityResult{
						Status: "completed",
						Output: map[string]interface{}{
							"params": params,
						},
					}, nil
				},
			},
		},
	})

	result, err := svc.InvokeCapability(context.Background(), contracts.CapabilityRequest{
		ConnectorID:  "skill-source-main",
		CapabilityID: "source.sync",
		Params:       map[string]interface{}{"mode": "runtime"},
	})
	if err != nil {
		t.Fatalf("invoke capability: %v", err)
	}
	if result.Runtime == nil || result.Runtime.Runtime != "capability" || result.Runtime.ConnectorID != "skill-source-main" {
		t.Fatalf("expected runtime metadata fallback to be populated, got %+v", result.Runtime)
	}
}

func TestActionHelpers(t *testing.T) {
	t.Parallel()

	if got := withDefaultPrefixes(nil); len(got) == 0 || got[0] != "hostname" {
		t.Fatalf("expected default prefixes, got %+v", got)
	}
	if got := withDefaultPrefixes([]string{"custom"}); len(got) != 1 || got[0] != "custom" {
		t.Fatalf("expected custom prefixes to be preserved, got %+v", got)
	}

	if got := withDefaultBlockedFragments(nil); len(got) == 0 || got[0] != "rm -rf" {
		t.Fatalf("expected default blocked fragments, got %+v", got)
	}
	if got := withDefaultBlockedFragments([]string{"custom"}); len(got) != 1 || got[0] != "custom" {
		t.Fatalf("expected custom blocked fragments to be preserved, got %+v", got)
	}

	if got := firstNonEmpty(" ", "alpha", "beta"); got != "alpha" {
		t.Fatalf("unexpected firstNonEmpty result: %q", got)
	}
	if got := runtimeSelectionMode(""); got != "auto_selector" {
		t.Fatalf("unexpected runtime selection mode: %q", got)
	}
	if got := runtimeSelectionMode("prometheus-main"); got != "explicit_connector" {
		t.Fatalf("unexpected runtime selection mode for connector: %q", got)
	}
	if got := queryRuntimeFallbackReason(true, connectors.Manifest{}); got != "connector_runtime_failed" {
		t.Fatalf("unexpected query fallback reason: %q", got)
	}
	if got := queryRuntimeFallbackReason(false, connectors.Manifest{Metadata: connectors.Metadata{ID: "prometheus-main"}}); got != "connector_runtime_unavailable" {
		t.Fatalf("unexpected query fallback reason for connector: %q", got)
	}
	if got := queryRuntimeFallbackReason(false, connectors.Manifest{}); got != "no_compatible_connector_selected" {
		t.Fatalf("unexpected query fallback reason for selector: %q", got)
	}
	if got := executionRuntimeFallbackReason(true, connectors.Manifest{}); got != "connector_runtime_unavailable" {
		t.Fatalf("unexpected execution fallback reason: %q", got)
	}
	if got := executionRuntimeFallbackReason(false, connectors.Manifest{Metadata: connectors.Metadata{ID: "jumpserver-main"}}); got != "connector_runtime_unavailable" {
		t.Fatalf("unexpected execution fallback reason for connector: %q", got)
	}
	if got := executionRuntimeFallbackReason(false, connectors.Manifest{}); got != "no_compatible_connector_selected" {
		t.Fatalf("unexpected execution fallback reason for selector: %q", got)
	}
	if got := capabilityRuntimeKey(newConnectorManifest("mcp-main", "MCP Main", "tars", "mcp", "mcp_api", []string{"1"})); got != "mcp" {
		t.Fatalf("unexpected capability runtime key for mcp: %q", got)
	}
	if got := capabilityRuntimeKey(newConnectorManifest("skill-main", "Skill Main", "tars", "skill_source", "http_index", []string{"1"})); got != "skill" {
		t.Fatalf("unexpected capability runtime key for skill: %q", got)
	}
	if got := capabilityRuntimeKey(newConnectorManifest("obs-main", "Obs Main", "tars", "observability", "observability_http", []string{"1"})); got != "observability_http" {
		t.Fatalf("unexpected capability runtime key for observability: %q", got)
	}
	if got := capabilityRuntimeKey(connectors.Manifest{Spec: connectors.Spec{Type: "delivery"}}); got != "delivery" {
		t.Fatalf("unexpected capability runtime key for normalized type fallback: %q", got)
	}
	if got := connectorSourceType(newConnectorManifest("mcp-main", "MCP Main", "tars", "mcp", "mcp_api", []string{"1"})); got != "mcp" {
		t.Fatalf("unexpected connector source type for mcp: %q", got)
	}
	if got := connectorSourceType(newConnectorManifest("skill-main", "Skill Main", "tars", "skill_source", "http_index", []string{"1"})); got != "skill" {
		t.Fatalf("unexpected connector source type for skill: %q", got)
	}
	if got := connectorSourceType(newConnectorManifest("obs-main", "Obs Main", "tars", "observability", "observability_http", []string{"1"})); got != "connector" {
		t.Fatalf("unexpected connector source type for connector: %q", got)
	}

	queryCloned := cloneQueryRuntimes(map[string]QueryRuntime{" prometheus_http ": testConnectorRuntime{}, "": nil})
	if len(queryCloned) != 1 {
		t.Fatalf("expected one cloned query runtime, got %+v", queryCloned)
	}
	execCloned := cloneExecutionRuntimes(map[string]ExecutionRuntime{" jumpserver_api ": testConnectorRuntime{}, "": nil})
	if len(execCloned) != 1 {
		t.Fatalf("expected one cloned execution runtime, got %+v", execCloned)
	}
	capCloned := cloneCapabilityRuntimes(map[string]CapabilityRuntime{" observability_http ": testConnectorRuntime{}, "": nil})
	if len(capCloned) != 1 {
		t.Fatalf("expected one cloned capability runtime, got %+v", capCloned)
	}

	params := map[string]interface{}{"mode": "original"}
	clonedParams := cloneCapabilityParams(params)
	clonedParams["mode"] = "mutated"
	if params["mode"] != "original" {
		t.Fatalf("expected cloneCapabilityParams to copy the map, got %+v", params)
	}
	if cloneCapabilityParams(nil) != nil {
		t.Fatalf("expected nil capability params to remain nil")
	}

	preview, bytes, truncated := prepareExecutionOutput("0123456789", 4)
	if preview != "0123" || bytes != 10 || !truncated {
		t.Fatalf("unexpected prepared execution output: %q %d %v", preview, bytes, truncated)
	}
	if got, truncated := truncateUTF8ByBytes("hello", 8); got != "hello" || truncated {
		t.Fatalf("unexpected utf8 truncation result: %q %v", got, truncated)
	}
	if got := truncateVerificationOutput("abcdefghijkl", 4); got != "abcd" {
		t.Fatalf("unexpected verification truncation result: %q", got)
	}
	if got := withDefaultMaxPersistedOutputBytes(0); got != 262144 {
		t.Fatalf("unexpected default output byte limit: %d", got)
	}
	if got, bytes, truncated := prepareExecutionOutput("", 4); got != "" || bytes != 0 || truncated {
		t.Fatalf("unexpected empty execution output handling: %q %d %v", got, bytes, truncated)
	}
	if got, bytes, truncated := prepareExecutionOutput("0123456789", 0); got != "0123456789" || bytes != 10 || truncated {
		t.Fatalf("unexpected zero-limit execution output handling: %q %d %v", got, bytes, truncated)
	}
	if got, truncated := truncateUTF8ByBytes("0123456789", 4); got != "0123" || !truncated {
		t.Fatalf("unexpected truncated utf8 output: %q %v", got, truncated)
	}
}

func TestResolveRuntimeHelpers(t *testing.T) {
	t.Parallel()

	metricsManager := mustTestConnectorManager(t, newConnectorManifest(
		"prometheus-main",
		"Prometheus Main",
		"prometheus",
		"metrics",
		"prometheus_http",
		[]string{"1"},
	))
	executionManager := mustTestConnectorManager(t, newConnectorManifest(
		"jumpserver-main",
		"JumpServer Main",
		"jumpserver",
		"execution",
		"jumpserver_api",
		[]string{"1"},
	))
	svc := NewService(Options{
		Connectors: metricsManager,
	})

	if _, _, ok, err := svc.resolveQueryRuntime("", ""); err != nil || ok {
		t.Fatalf("expected empty query runtime resolution, got ok=%v err=%v", ok, err)
	}
	svc = NewService(Options{
		Connectors: executionManager,
	})
	if _, _, ok, err := svc.resolveExecutionRuntime("", ""); err != nil || ok {
		t.Fatalf("expected empty execution runtime resolution, got ok=%v err=%v", ok, err)
	}

	svc = NewService(Options{
		Connectors: metricsManager,
	})
	if _, _, _, err := svc.resolveQueryRuntime("prometheus-main", "unsupported"); err == nil || !strings.Contains(err.Error(), "protocol unsupported") {
		t.Fatalf("expected unsupported query runtime protocol error, got %v", err)
	}
	if _, _, _, err := svc.resolveQueryRuntime("prometheus-main", "prometheus_http"); err == nil || !strings.Contains(err.Error(), "unsupported metrics connector protocol prometheus_http") {
		t.Fatalf("expected missing query runtime error, got %v", err)
	}
	if _, _, _, err := svc.resolveExecutionRuntime("jumpserver-main", "unsupported"); err == nil || !strings.Contains(err.Error(), "connector not found") {
		t.Fatalf("expected unresolved execution runtime error, got %v", err)
	}
	svc = NewService(Options{
		Connectors: executionManager,
	})
	if _, _, _, err := svc.resolveExecutionRuntime("jumpserver-main", "jumpserver_api"); err == nil || !strings.Contains(err.Error(), "unsupported execution connector protocol jumpserver_api") {
		t.Fatalf("expected missing execution runtime error, got %v", err)
	}
}

func TestExecuteApprovedRuntimeAndSSHBranches(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"jumpserver-main",
		"JumpServer Main",
		"jumpserver",
		"execution",
		"jumpserver_api",
		[]string{"1"},
	))

	svc := NewService(Options{
		Connectors: manager,
		ExecutionRuntimes: map[string]ExecutionRuntime{
			"jumpserver_api": testConnectorRuntime{
				executeFn: func(_ context.Context, _ connectors.Manifest, req contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
					return contracts.ExecutionResult{
						ExitCode: 3,
						Output:   "runtime output",
					}, errors.New("execution runtime unavailable")
				},
			},
		},
		AllowedHosts:   []string{"127.0.0.1"},
		OutputSpoolDir: t.TempDir(),
	})

	result, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-runtime-error",
		SessionID:   "ses-runtime-error",
		TargetHost:  "127.0.0.1",
		Command:     "systemctl restart sshd",
		ConnectorID: "jumpserver-main",
		Protocol:    "jumpserver_api",
	})
	if err == nil || !strings.Contains(err.Error(), "execution runtime unavailable") {
		t.Fatalf("expected execution runtime error, got %v", err)
	}
	if result.ExitCode != 3 {
		t.Fatalf("expected runtime result to be returned, got %+v", result)
	}

	svc = NewService(Options{
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 0,
				Output:   "timed out",
				TimedOut: true,
			},
			err: sshclient.ErrCommandTimedOut,
		},
		AllowedHosts:   []string{"127.0.0.1"},
		OutputSpoolDir: t.TempDir(),
	})
	timeoutResult, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-timeout",
		SessionID:   "ses-timeout",
		TargetHost:  "127.0.0.1",
		Command:     "hostname",
	})
	if err != nil {
		t.Fatalf("execute approved timeout path: %v", err)
	}
	if timeoutResult.Status != "timeout" {
		t.Fatalf("expected timeout status, got %+v", timeoutResult)
	}

	svc = NewService(Options{
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 7,
				Output:   "failed",
			},
			err: sshclient.ErrRemoteCommandFailed,
		},
		AllowedHosts:   []string{"127.0.0.1"},
		OutputSpoolDir: t.TempDir(),
	})
	failedResult, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-failed",
		SessionID:   "ses-failed",
		TargetHost:  "127.0.0.1",
		Command:     "hostname",
	})
	if err != nil {
		t.Fatalf("execute approved failed path: %v", err)
	}
	if failedResult.Status != "failed" {
		t.Fatalf("expected failed status, got %+v", failedResult)
	}

	if _, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		SessionID:  "ses-incomplete",
		TargetHost: "127.0.0.1",
		Command:    "hostname",
	}); err == nil || !strings.Contains(err.Error(), "execution request is incomplete") {
		t.Fatalf("expected incomplete request validation error, got %v", err)
	}
}

func TestExecuteApprovedReportsUnexpectedSSHError(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor: &fakeExecutor{
			err: errors.New("network down"),
		},
		AllowedHosts: []string{"127.0.0.1"},
	})

	if _, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-ssh-error",
		SessionID:   "ses-ssh-error",
		TargetHost:  "127.0.0.1",
		Command:     "hostname",
	}); err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("expected unexpected ssh error, got %v", err)
	}
}

func TestExecuteApprovedSkipsPersistWhenRuntimeAlreadySetsOutputMetadata(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"jumpserver-main",
		"JumpServer Main",
		"jumpserver",
		"execution",
		"jumpserver_api",
		[]string{"1"},
	))

	svc := NewService(Options{
		Connectors: manager,
		ExecutionRuntimes: map[string]ExecutionRuntime{
			"jumpserver_api": testConnectorRuntime{
				executeFn: func(_ context.Context, _ connectors.Manifest, _ contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
					return contracts.ExecutionResult{
						Output:        "already captured",
						OutputBytes:   17,
						OutputRef:     "existing-ref",
						OutputPreview: "existing-preview",
					}, nil
				},
			},
		},
		AllowedHosts:   []string{"127.0.0.1"},
		OutputSpoolDir: t.TempDir(),
	})

	result, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-output-metadata",
		SessionID:   "ses-output-metadata",
		TargetHost:  "127.0.0.1",
		Command:     "systemctl restart sshd",
		ConnectorID: "jumpserver-main",
		Protocol:    "jumpserver_api",
	})
	if err != nil {
		t.Fatalf("execute approved output metadata path: %v", err)
	}
	if result.OutputRef != "existing-ref" || result.OutputPreview != "existing-preview" || result.OutputBytes != 17 {
		t.Fatalf("unexpected runtime output metadata handling: %+v", result)
	}
}

func TestExecuteApprovedPersistsRuntimeOutput(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"jumpserver-main",
		"JumpServer Main",
		"jumpserver",
		"execution",
		"jumpserver_api",
		[]string{"1"},
	))

	spoolDir := t.TempDir()
	svc := NewService(Options{
		Connectors: manager,
		ExecutionRuntimes: map[string]ExecutionRuntime{
			"jumpserver_api": testConnectorRuntime{
				executeFn: func(_ context.Context, _ connectors.Manifest, _ contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
					return contracts.ExecutionResult{
						Output: "runtime output",
					}, nil
				},
			},
		},
		AllowedHosts:   []string{"127.0.0.1"},
		OutputSpoolDir: spoolDir,
	})

	result, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-runtime-output",
		SessionID:   "ses-runtime-output",
		TargetHost:  "127.0.0.1",
		Command:     "systemctl restart sshd",
		ConnectorID: "jumpserver-main",
		Protocol:    "jumpserver_api",
	})
	if err != nil {
		t.Fatalf("execute approved runtime output path: %v", err)
	}
	if result.OutputRef == "" || result.OutputPreview == "" || result.OutputBytes == 0 {
		t.Fatalf("expected persisted runtime output metadata, got %+v", result)
	}
	if !strings.Contains(result.OutputRef, spoolDir) {
		t.Fatalf("expected output ref inside spool dir, got %+v", result.OutputRef)
	}
}

func TestExecuteApprovedWarnsWhenPersistingOutputFails(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(outputPath, []byte("seed"), 0o600); err != nil {
		t.Fatalf("seed output path: %v", err)
	}

	svc := NewService(Options{
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 0,
				Output:   "persist me",
			},
		},
		AllowedHosts:   []string{"127.0.0.1"},
		OutputSpoolDir: outputPath,
	})

	result, err := svc.ExecuteApproved(context.Background(), contracts.ApprovedExecutionRequest{
		ExecutionID: "exe-persist-warn",
		SessionID:   "ses-persist-warn",
		TargetHost:  "127.0.0.1",
		Command:     "hostname",
	})
	if err != nil {
		t.Fatalf("execute approved persist warning path: %v", err)
	}
	if result.Status != "completed" || result.OutputPreview == "" {
		t.Fatalf("unexpected persist warning result: %+v", result)
	}
}

func TestVerifyExecutionRuntimeAndSSHBranches(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"jumpserver-main",
		"JumpServer Main",
		"jumpserver",
		"execution",
		"jumpserver_api",
		[]string{"1"},
	))

	svc := NewService(Options{
		Connectors: manager,
		ExecutionRuntimes: map[string]ExecutionRuntime{
			"jumpserver_api": testConnectorRuntime{
				verifyFn: func(_ context.Context, _ connectors.Manifest, req contracts.VerificationRequest) (contracts.VerificationResult, error) {
					return contracts.VerificationResult{
						SessionID:   req.SessionID,
						ExecutionID: req.ExecutionID,
						Status:      "success",
						Summary:     "runtime verification passed",
					}, nil
				},
			},
		},
	})

	result, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-runtime",
		SessionID:   "ses-runtime",
		TargetHost:  "127.0.0.1",
		Service:     "sshd",
		ConnectorID: "jumpserver-main",
		Protocol:    "jumpserver_api",
	})
	if err != nil {
		t.Fatalf("verify execution runtime path: %v", err)
	}
	if result.Status != "success" || result.Summary != "runtime verification passed" {
		t.Fatalf("unexpected runtime verification result: %+v", result)
	}

	svc = NewService(Options{
		Executor: &fakeExecutor{
			result: sshclient.Result{
				ExitCode: 0,
				Output:   "timeout",
				TimedOut: true,
			},
			err: sshclient.ErrCommandTimedOut,
		},
		AllowedHosts: []string{"127.0.0.1"},
	})
	timeoutResult, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-timeout",
		SessionID:   "ses-timeout",
		TargetHost:  "127.0.0.1",
		Service:     "sshd",
	})
	if err != nil {
		t.Fatalf("verify execution timeout path: %v", err)
	}
	if timeoutResult.Status != "failed" || !strings.Contains(timeoutResult.Summary, "timed out") {
		t.Fatalf("unexpected timeout verification result: %+v", timeoutResult)
	}

	skipped, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-empty-host",
		SessionID:   "ses-empty-host",
		Service:     "sshd",
	})
	if err != nil {
		t.Fatalf("verify execution empty host should skip, got %v", err)
	}
	if skipped.Status != "skipped" || !strings.Contains(skipped.Summary, "target host is empty") {
		t.Fatalf("unexpected empty-host verification result: %+v", skipped)
	}
}

func TestVerifyExecutionReportsRuntimeError(t *testing.T) {
	t.Parallel()

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"jumpserver-main",
		"JumpServer Main",
		"jumpserver",
		"execution",
		"jumpserver_api",
		[]string{"1"},
	))

	svc := NewService(Options{
		Connectors: manager,
		ExecutionRuntimes: map[string]ExecutionRuntime{
			"jumpserver_api": testConnectorRuntime{
				verifyFn: func(context.Context, connectors.Manifest, contracts.VerificationRequest) (contracts.VerificationResult, error) {
					return contracts.VerificationResult{Status: "failed", Summary: "runtime verification failed"}, errors.New("verification runtime unavailable")
				},
			},
		},
	})

	result, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-runtime-error",
		SessionID:   "ses-runtime-error",
		TargetHost:  "127.0.0.1",
		Service:     "sshd",
		ConnectorID: "jumpserver-main",
		Protocol:    "jumpserver_api",
	})
	if err == nil || !strings.Contains(err.Error(), "verification runtime unavailable") {
		t.Fatalf("expected verification runtime error, got %v", err)
	}
	if result.Status != "failed" || !strings.Contains(result.Summary, "runtime verification failed") {
		t.Fatalf("unexpected runtime verification error result: %+v", result)
	}
}

func TestVerifyExecutionReportsValidationErrorAndDegradedRuntimeStatus(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		AllowedHosts: []string{"127.0.0.1"},
	})

	result, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-validation",
		SessionID:   "ses-validation",
		TargetHost:  "192.0.2.1",
		Service:     "sshd",
	})
	if err != nil {
		t.Fatalf("verify execution validation error path: %v", err)
	}
	if result.Status != "failed" || !strings.Contains(result.Summary, "allowlist") {
		t.Fatalf("unexpected validation error result: %+v", result)
	}

	manager := mustTestConnectorManager(t, newConnectorManifest(
		"jumpserver-main",
		"JumpServer Main",
		"jumpserver",
		"execution",
		"jumpserver_api",
		[]string{"1"},
	))
	svc = NewService(Options{
		Connectors: manager,
		ExecutionRuntimes: map[string]ExecutionRuntime{
			"jumpserver_api": testConnectorRuntime{
				verifyFn: func(context.Context, connectors.Manifest, contracts.VerificationRequest) (contracts.VerificationResult, error) {
					return contracts.VerificationResult{
						Status:  "failed",
						Summary: "runtime verification degraded",
					}, nil
				},
			},
		},
	})
	degraded, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-degraded",
		SessionID:   "ses-degraded",
		TargetHost:  "127.0.0.1",
		Service:     "sshd",
		ConnectorID: "jumpserver-main",
		Protocol:    "jumpserver_api",
	})
	if err != nil {
		t.Fatalf("verify execution degraded runtime path: %v", err)
	}
	if degraded.Status != "failed" || !strings.Contains(degraded.Summary, "runtime verification degraded") {
		t.Fatalf("unexpected degraded verification result: %+v", degraded)
	}
}

func TestVerifyExecutionReportsUnexpectedSSHError(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		Executor: &fakeExecutor{
			err: errors.New("network down"),
		},
		AllowedHosts: []string{"127.0.0.1"},
	})

	result, err := svc.VerifyExecution(context.Background(), contracts.VerificationRequest{
		ExecutionID: "exe-ssh-error",
		SessionID:   "ses-ssh-error",
		TargetHost:  "127.0.0.1",
		Service:     "sshd",
	})
	if err != nil {
		t.Fatalf("verify execution unexpected ssh error: %v", err)
	}
	if result.Status != "failed" || !strings.Contains(result.Summary, "could not confirm sshd status") {
		t.Fatalf("unexpected ssh error verification result: %+v", result)
	}
}

func TestRecordConnectorHealthBranches(t *testing.T) {
	t.Parallel()

	var nilService *Service
	nilService.recordConnectorHealth("connector-id", "healthy", "summary")

	svc := NewService(Options{Connectors: &connectors.Manager{}})
	svc.recordConnectorHealth("", "healthy", "summary")
	svc.recordConnectorHealth("missing", "healthy", "summary")
}

func TestCheckConnectorHealthNilServiceAndEmptyConnectors(t *testing.T) {
	t.Parallel()

	var nilService *Service
	if _, err := nilService.CheckConnectorHealth(context.Background(), "connector-id"); err == nil {
		t.Fatalf("expected nil service connector health error")
	}

	svc := NewService(Options{})
	if _, err := svc.CheckConnectorHealth(context.Background(), "connector-id"); err == nil {
		t.Fatalf("expected empty connectors manager error")
	}
}

func TestResolveRuntimeManifestNilBranches(t *testing.T) {
	t.Parallel()

	var nilService *Service
	if _, ok, err := nilService.resolveRuntimeManifest("connector-id", "metrics", "prometheus_http", nil); err != nil || ok {
		t.Fatalf("expected nil service runtime resolution to short-circuit, got ok=%v err=%v", ok, err)
	}

	svc := NewService(Options{})
	if _, ok, err := svc.resolveRuntimeManifest("connector-id", "metrics", "prometheus_http", nil); err != nil || ok {
		t.Fatalf("expected empty connector manager runtime resolution to short-circuit, got ok=%v err=%v", ok, err)
	}
}

func TestPersistOutputBranches(t *testing.T) {
	t.Parallel()

	emptySvc := NewService(Options{})
	if got, err := emptySvc.persistOutput("exe-empty-dir", "output"); err != nil || got != "" {
		t.Fatalf("expected empty spool dir to skip persistence, got %q err=%v", got, err)
	}

	svc := NewService(Options{OutputSpoolDir: t.TempDir()})
	if got, err := svc.persistOutput("exe-empty", ""); err != nil || got != "" {
		t.Fatalf("expected empty output to be ignored, got %q err=%v", got, err)
	}

	filePath := filepath.Join(t.TempDir(), "spool")
	if err := os.WriteFile(filePath, []byte("seed"), 0o600); err != nil {
		t.Fatalf("seed spool path: %v", err)
	}
	svc = NewService(Options{OutputSpoolDir: filePath})
	if got, err := svc.persistOutput("exe-error", "persist me"); err == nil || got != "" {
		t.Fatalf("expected persist error for non-directory spool path, got %q err=%v", got, err)
	}
}

func TestLowLevelStringAndCandidateHelpers(t *testing.T) {
	t.Parallel()

	if got := firstNonEmpty(" ", "alpha"); got != "alpha" {
		t.Fatalf("unexpected firstNonEmpty result: %q", got)
	}
	if got := firstNonEmpty(" ", ""); got != "" {
		t.Fatalf("expected empty result from firstNonEmpty, got %q", got)
	}
	if err := validateBlockedFragments("hostname", []string{"", "rm -rf"}); err != nil {
		t.Fatalf("expected clean command to pass blocked fragment validation, got %v", err)
	}
	if err := validateBlockedFragments("rm -rf /tmp/demo", []string{"", "rm -rf"}); err == nil {
		t.Fatalf("expected blocked fragment validation error")
	}
	if !matchesCommandPrefix("hostname", []string{"", "host"}) {
		t.Fatalf("expected prefix match to succeed")
	}
	if matchesCommandPrefix("uptime", []string{"", "host"}) {
		t.Fatalf("expected prefix match to fail")
	}
	if got := VerificationServiceCandidates(""); got != nil {
		t.Fatalf("expected empty service candidates, got %+v", got)
	}
	if got, truncated := truncateUTF8ByBytes("éé", 1); got != "" || !truncated {
		t.Fatalf("expected utf8 truncation at rune boundary, got %q truncated=%v", got, truncated)
	}
}

func TestValidateCommandBranches(t *testing.T) {
	t.Parallel()

	if err := validateCommand("", nil, nil, "", nil); err == nil || !strings.Contains(err.Error(), "command is empty") {
		t.Fatalf("expected empty command error, got %v", err)
	}
	if err := validateCommand("rm -rf /tmp/demo", nil, []string{"rm -rf"}, "", nil); err == nil || !strings.Contains(err.Error(), "blocked fragment") {
		t.Fatalf("expected blocked fragment error, got %v", err)
	}
	if err := validateCommand("hostname", []string{"hostname"}, nil, "", nil); err != nil {
		t.Fatalf("expected allowed prefix to pass, got %v", err)
	}
	if err := validateCommand("systemctl restart sshd", nil, nil, "sshd", map[string][]string{"sshd": {"systemctl restart sshd"}}); err != nil {
		t.Fatalf("expected service allowlist command to pass, got %v", err)
	}
	if err := validateCommand("systemctl restart nginx", nil, nil, "sshd", map[string][]string{"sshd": {"systemctl restart sshd"}}); err == nil || !strings.Contains(err.Error(), "service allowlist for sshd") {
		t.Fatalf("expected service allowlist denial, got %v", err)
	}
	if err := validateCommand("echo hello", nil, nil, "", nil); err == nil || !strings.Contains(err.Error(), "not in allowlist") {
		t.Fatalf("expected default allowlist denial, got %v", err)
	}
}

func mustTestConnectorManager(t *testing.T, entries ...connectors.Manifest) *connectors.Manager {
	t.Helper()

	path := filepath.Join(t.TempDir(), "connectors.yaml")
	content, err := connectors.EncodeConfig(&connectors.Config{Entries: entries})
	if err != nil {
		t.Fatalf("encode connectors config: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}
	manager, err := connectors.NewManager(path)
	if err != nil {
		t.Fatalf("new connectors manager: %v", err)
	}
	return manager
}

func newConnectorManifest(id, displayName, vendor, connectorType, protocol string, tarsVersions []string) connectors.Manifest {
	return connectors.Manifest{
		APIVersion: "tars.connector/v1alpha1",
		Kind:       "connector",
		Metadata: connectors.Metadata{
			ID:          id,
			Name:        id,
			DisplayName: displayName,
			Vendor:      vendor,
			Version:     "1.0.0",
		},
		Spec: connectors.Spec{
			Type:     connectorType,
			Protocol: protocol,
			ImportExport: connectors.ImportExport{
				Exportable: true,
				Importable: true,
				Formats:    []string{"yaml"},
			},
		},
		Compatibility: connectors.Compatibility{
			TARSMajorVersions: tarsVersions,
		},
	}
}

func capabilityTestManifest() connectors.Manifest {
	manifest := newConnectorManifest(
		"skill-source-main",
		"Skill Source",
		"tars",
		"skill_source",
		"http_index",
		[]string{"1"},
	)
	manifest.Spec.Capabilities = []connectors.Capability{
		{
			ID:       "source.sync",
			Action:   "import",
			ReadOnly: false,
		},
	}
	return manifest
}

type testConnectorRuntime struct {
	healthFn  func(context.Context, connectors.Manifest) (string, string, error)
	executeFn func(context.Context, connectors.Manifest, contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error)
	verifyFn  func(context.Context, connectors.Manifest, contracts.VerificationRequest) (contracts.VerificationResult, error)
	invokeFn  func(context.Context, connectors.Manifest, string, map[string]interface{}) (contracts.CapabilityResult, error)
}

func (r testConnectorRuntime) Query(_ context.Context, _ connectors.Manifest, _ contracts.MetricsQuery) (contracts.MetricsResult, error) {
	return contracts.MetricsResult{}, nil
}

func (r testConnectorRuntime) Execute(ctx context.Context, manifest connectors.Manifest, req contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
	if r.executeFn != nil {
		return r.executeFn(ctx, manifest, req)
	}
	return contracts.ExecutionResult{}, nil
}

func (r testConnectorRuntime) Verify(ctx context.Context, manifest connectors.Manifest, req contracts.VerificationRequest) (contracts.VerificationResult, error) {
	if r.verifyFn != nil {
		return r.verifyFn(ctx, manifest, req)
	}
	return contracts.VerificationResult{}, nil
}

func (r testConnectorRuntime) Invoke(ctx context.Context, manifest connectors.Manifest, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	if r.invokeFn != nil {
		return r.invokeFn(ctx, manifest, capabilityID, params)
	}
	return contracts.CapabilityResult{Status: "completed"}, nil
}

func (r testConnectorRuntime) CheckHealth(ctx context.Context, manifest connectors.Manifest) (string, string, error) {
	if r.healthFn != nil {
		return r.healthFn(ctx, manifest)
	}
	return "healthy", fmt.Sprintf("runtime healthy for %s", manifest.Metadata.ID), nil
}
