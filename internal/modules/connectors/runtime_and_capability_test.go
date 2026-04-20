package connectors

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestResolveRuntimeManifestValidateRuntimeManifestAndExecutionMode(t *testing.T) {
	t.Parallel()

	ssh := connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "jumpserver_api")
	victoriaLogs := connectorManifest("victorialogs-main", "victorialogs", "Victoria Logs", "victorialogs", "1.0.0", "observability", "log_file")
	victoriaMetrics := connectorManifest("victoriametrics-main", "victoriametrics", "Victoria Metrics", "victoriametrics", "1.0.0", "metrics", "victoriametrics_http")
	customMetrics := connectorManifest("custom-metrics-main", "custom-metrics", "Custom Metrics", "custom", "1.0.0", "metrics", "custom_http")

	manager := &Manager{
		config: &Config{Entries: []Manifest{ssh, victoriaLogs, victoriaMetrics, customMetrics}},
		state: map[string]LifecycleState{
			"ssh-main":             {ConnectorID: "ssh-main", Health: HealthStatus{Status: "healthy"}},
			"victorialogs-main":    {ConnectorID: "victorialogs-main", Health: HealthStatus{Status: "healthy"}},
			"victoriametrics-main": {ConnectorID: "victoriametrics-main", Health: HealthStatus{Status: "healthy"}},
			"custom-metrics-main":  {ConnectorID: "custom-metrics-main", Health: HealthStatus{Status: "healthy"}},
		},
	}

	if _, err := ResolveRuntimeManifest(nil, "victoria logs", "observability", "log_file", map[string]struct{}{"log_file": {}}); !errors.Is(err, ErrConnectorNotFound) {
		t.Fatalf("expected nil manager lookup to fail with not found, got %v", err)
	}
	if _, err := ResolveRuntimeManifest(manager, " ", "observability", "log_file", map[string]struct{}{"log_file": {}}); !errors.Is(err, ErrConnectorNotFound) {
		t.Fatalf("expected blank connector id lookup to fail with not found, got %v", err)
	}

	resolved, err := ResolveRuntimeManifest(manager, "victorialogs", "observability", "log_file", map[string]struct{}{"log_file": {}})
	if err != nil {
		t.Fatalf("resolve runtime manifest by alias: %v", err)
	}
	if resolved.Metadata.ID != "victorialogs-main" {
		t.Fatalf("expected alias lookup to resolve victoria logs connector, got %+v", resolved.Metadata)
	}

	validateCases := []struct {
		name      string
		m         Manifest
		exp       string
		proto     string
		supported map[string]struct{}
		wantErr   error
	}{
		{
			name:    "type mismatch",
			m:       ssh,
			exp:     "metrics",
			proto:   "jumpserver_api",
			wantErr: ErrConnectorRuntimeUnsupported,
		},
		{
			name: "disabled",
			m: func() Manifest {
				m := ssh
				m.Disabled = true
				return m
			}(),
			exp:       "execution",
			proto:     "jumpserver_api",
			supported: map[string]struct{}{"jumpserver_api": {}},
			wantErr:   ErrConnectorDisabled,
		},
		{
			name: "incompatible",
			m: func() Manifest {
				m := ssh
				m.Compatibility.TARSMajorVersions = []string{"2"}
				return m
			}(),
			exp:       "execution",
			proto:     "jumpserver_api",
			supported: map[string]struct{}{"jumpserver_api": {}},
			wantErr:   ErrConnectorIncompatible,
		},
		{
			name:      "unsupported protocol",
			m:         ssh,
			exp:       "execution",
			proto:     "jumpserver_api",
			supported: map[string]struct{}{"other_protocol": {}},
			wantErr:   ErrConnectorRuntimeUnsupported,
		},
		{
			name:      "success",
			m:         ssh,
			exp:       "execution",
			proto:     " jumpserver_api ",
			supported: map[string]struct{}{"jumpserver_api": {}},
		},
	}
	for _, tc := range validateCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateRuntimeManifest(tc.m, tc.exp, tc.proto, tc.supported)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("expected validation success, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}

	if got := DefaultExecutionMode("jumpserver_api"); got != "jumpserver_job" {
		t.Fatalf("expected jumpserver_api to map to jumpserver_job, got %q", got)
	}
	if got := DefaultExecutionMode(""); got != "ssh" {
		t.Fatalf("expected blank execution mode to default to ssh, got %q", got)
	}
	if got := DefaultExecutionMode(" victoriametrics_http "); got != "victoriametrics_http" {
		t.Fatalf("expected non-special execution mode to be trimmed, got %q", got)
	}

	if err := ValidateConfigCompatibility(Config{Entries: []Manifest{ssh, func() Manifest {
		m := victoriaMetrics
		m.Compatibility.TARSMajorVersions = []string{"2"}
		return m
	}()}}); err == nil || !strings.Contains(err.Error(), "victoriametrics-main") {
		t.Fatalf("expected config compatibility to reject the incompatible connector, got %v", err)
	}
}

func TestCapabilityToolMappingBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		entry      Manifest
		capability Capability
		want       []mappedToolCapability
	}{
		{
			name:       "metrics range",
			entry:      connectorManifest("victoriametrics-main", "victoriametrics", "Victoria Metrics", "victoriametrics", "1.0.0", "metrics", "victoriametrics_http"),
			capability: Capability{ID: "query.range", Action: "query", ReadOnly: true},
			want:       []mappedToolCapability{{tool: "metrics.query_range", invocable: true}},
		},
		{
			name:       "metrics query action fallback",
			entry:      connectorManifest("victoriametrics-main", "victoriametrics", "Victoria Metrics", "victoriametrics", "1.0.0", "metrics", "victoriametrics_http"),
			capability: Capability{ID: "custom.query", Action: "query", ReadOnly: true},
			want:       []mappedToolCapability{{tool: "metrics.query_instant", invocable: true}, {tool: "metrics.query_range", invocable: true}},
		},
		{
			name:       "execution command",
			entry:      connectorManifest("ssh-main", "ssh", "SSH Main", "ssh", "1.0.0", "execution", "jumpserver_api"),
			capability: Capability{ID: "command.execute", Action: "invoke", ReadOnly: false},
			want:       []mappedToolCapability{{tool: "execution.run_command", invocable: true}},
		},
		{
			name:       "observability query",
			entry:      connectorManifest("victorialogs-main", "victorialogs", "Victoria Logs", "victorialogs", "1.0.0", "observability", "log_file"),
			capability: Capability{ID: "trace.query", Action: "query", ReadOnly: true},
			want:       []mappedToolCapability{{tool: "observability.query", invocable: true}},
		},
		{
			name:       "delivery query fallback",
			entry:      connectorManifest("delivery-main", "delivery", "Delivery Main", "acme", "1.0.0", "delivery", "delivery_git"),
			capability: Capability{ID: "custom.status", Action: "query", ReadOnly: true},
			want:       []mappedToolCapability{{tool: "delivery.query", invocable: true}},
		},
		{
			name:       "mcp tool",
			entry:      connectorManifest("mcp-main", "mcp", "MCP Main", "acme", "1.0.0", "mcp_tool", "mcp_stdio"),
			capability: Capability{ID: "tool.call", Action: "invoke", ReadOnly: true},
			want:       []mappedToolCapability{{tool: "connector.invoke_capability", invocable: true}},
		},
		{
			name:       "skill source",
			entry:      connectorManifest("skill-main", "skill", "Skill Main", "acme", "1.0.0", "skill_source", "http_index"),
			capability: Capability{ID: "source.sync", Action: "import", ReadOnly: true},
			want:       []mappedToolCapability{{tool: "connector.invoke_capability", invocable: true}},
		},
		{
			name:       "fallback",
			entry:      connectorManifest("custom-main", "custom", "Custom Main", "acme", "1.0.0", "custom", "custom_http"),
			capability: Capability{ID: "anything", Action: "build", ReadOnly: false},
			want:       []mappedToolCapability{{tool: "connector.invoke_capability", invocable: false}},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mapCapabilityToTools(tc.entry, tc.capability)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected tool mapping: got %+v want %+v", got, tc.want)
			}
		})
	}

	manager := &Manager{
		config: &Config{
			Entries: []Manifest{
				func() Manifest {
					m := connectorManifest("custom-metrics-main", "custom-metrics", "Custom Metrics", "custom", "1.0.0", "metrics", "custom_http")
					m.Spec.Capabilities = []Capability{{ID: "query.range", Action: "query", ReadOnly: true}}
					return m
				}(),
				func() Manifest {
					m := connectorManifest("victoriametrics-main", "victoriametrics", "Victoria Metrics", "victoriametrics", "1.0.0", "metrics", "victoriametrics_http")
					m.Spec.Capabilities = []Capability{{ID: "query.range", Action: "query", ReadOnly: true}}
					return m
				}(),
			},
		},
		state: map[string]LifecycleState{
			"custom-metrics-main":  {ConnectorID: "custom-metrics-main", Health: HealthStatus{Status: "healthy"}},
			"victoriametrics-main": {ConnectorID: "victoriametrics-main", Health: HealthStatus{Status: "healthy"}},
		},
	}

	items := ToolPlanCapabilities(manager)
	if len(items) != 2 {
		t.Fatalf("expected 2 tool-plan capabilities, got %d", len(items))
	}
	if items[0].ConnectorID != "victoriametrics-main" {
		t.Fatalf("expected victoriametrics connector to outrank unknown protocol entries, got %+v", items)
	}
}
