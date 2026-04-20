package connectors

import "testing"

func TestToolPlanCapabilitiesMapsRuntimeAndCatalogEntries(t *testing.T) {
	t.Parallel()

	manager := &Manager{
		config: &Config{
			Entries: []Manifest{
				{
					Metadata: Metadata{ID: "prometheus-main", Vendor: "prometheus"},
					Spec: Spec{
						Type:     "metrics",
						Protocol: "prometheus_http",
						Capabilities: []Capability{
							{ID: "query.range", Action: "query", ReadOnly: true, Description: "range query"},
						},
					},
				},
				{
					Metadata: Metadata{ID: "skill-source-main", Vendor: "tars"},
					Spec: Spec{
						Type:     "skill_source",
						Protocol: "http_index",
						Capabilities: []Capability{
							{ID: "source.sync", Action: "import", ReadOnly: true, Description: "sync skill metadata"},
						},
					},
				},
				{
					Metadata: Metadata{ID: "mcp-tool-main", Vendor: "acme"},
					Spec: Spec{
						Type:     "mcp_tool",
						Protocol: "mcp_stdio",
						Capabilities: []Capability{
							{ID: "tool.call", Action: "invoke", ReadOnly: true, Description: "invoke MCP tool"},
						},
					},
				},
			},
		},
	}

	items := ToolPlanCapabilities(manager)
	if len(items) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(items))
	}

	byConnector := map[string]ToolPlanCapability{}
	for _, item := range items {
		byConnector[item.ConnectorID] = item
	}
	if item := byConnector["prometheus-main"]; item.Tool != "metrics.query_range" || !item.Invocable {
		t.Fatalf("unexpected metrics capability: %+v", item)
	}
	if item := byConnector["mcp-tool-main"]; item.Tool != "connector.invoke_capability" || !item.Invocable {
		t.Fatalf("unexpected MCP capability: %+v", item)
	}
	if item := byConnector["skill-source-main"]; item.Tool != "connector.invoke_capability" || !item.Invocable {
		t.Fatalf("unexpected skill capability: %+v", item)
	}
}

func TestToolPlanCapabilitiesSkipsDisabledEntries(t *testing.T) {
	t.Parallel()

	manager := &Manager{
		config: &Config{
			Entries: []Manifest{
				{
					Disabled: true,
					Metadata: Metadata{ID: "prometheus-main", Vendor: "prometheus"},
					Spec: Spec{
						Type:     "metrics",
						Protocol: "prometheus_http",
						Capabilities: []Capability{
							{ID: "query.range", Action: "query", ReadOnly: true},
						},
					},
					Compatibility: Compatibility{TARSMajorVersions: []string{"1"}},
				},
				{
					Metadata: Metadata{ID: "victoriametrics-main", Vendor: "victoriametrics"},
					Spec: Spec{
						Type:     "metrics",
						Protocol: "victoriametrics_http",
						Capabilities: []Capability{
							{ID: "query.range", Action: "query", ReadOnly: true},
						},
					},
					Compatibility: Compatibility{TARSMajorVersions: []string{"1"}},
				},
			},
		},
		state: map[string]LifecycleState{
			"prometheus-main":      {ConnectorID: "prometheus-main", Health: HealthStatus{Status: "disabled"}},
			"victoriametrics-main": {ConnectorID: "victoriametrics-main", Health: HealthStatus{Status: "healthy"}},
		},
	}

	items := ToolPlanCapabilities(manager)
	if len(items) != 1 {
		t.Fatalf("expected only enabled connector capability, got %d", len(items))
	}
	if items[0].ConnectorID != "victoriametrics-main" {
		t.Fatalf("expected enabled connector to remain, got %+v", items[0])
	}
}

func TestToolPlanCapabilitiesPrefersHealthyRealMetricsConnectorBeforeStub(t *testing.T) {
	t.Parallel()

	manager := &Manager{
		config: &Config{
			Entries: []Manifest{
				{
					Metadata: Metadata{ID: "metrics-stub", Vendor: "tars"},
					Spec: Spec{
						Type:     "metrics",
						Protocol: "stub",
						Capabilities: []Capability{
							{ID: "query.range", Action: "query", ReadOnly: true},
						},
					},
					Compatibility: Compatibility{TARSMajorVersions: []string{"1"}},
				},
				{
					Metadata: Metadata{ID: "victoriametrics-main", Vendor: "victoriametrics"},
					Spec: Spec{
						Type:     "metrics",
						Protocol: "victoriametrics_http",
						Capabilities: []Capability{
							{ID: "query.range", Action: "query", ReadOnly: true},
						},
					},
					Compatibility: Compatibility{TARSMajorVersions: []string{"1"}},
				},
			},
		},
		state: map[string]LifecycleState{
			"metrics-stub":         {ConnectorID: "metrics-stub", Health: HealthStatus{Status: "healthy"}},
			"victoriametrics-main": {ConnectorID: "victoriametrics-main", Health: HealthStatus{Status: "healthy"}},
		},
	}

	items := ToolPlanCapabilities(manager)
	if len(items) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(items))
	}
	if items[0].ConnectorID != "victoriametrics-main" {
		t.Fatalf("expected real metrics connector to be preferred, got %+v", items[0])
	}
}

func TestToolPlanCapabilitiesPrefersHealthyRealObservabilityConnectorBeforeStub(t *testing.T) {
	t.Parallel()

	manager := &Manager{
		config: &Config{
			Entries: []Manifest{
				{
					Metadata: Metadata{ID: "observability-stub", Vendor: "tars"},
					Spec: Spec{
						Type:         "observability",
						Protocol:     "stub",
						Capabilities: []Capability{{ID: "observability.query", Action: "query", ReadOnly: true}},
					},
					Compatibility: Compatibility{TARSMajorVersions: []string{"1"}},
				},
				{
					Metadata: Metadata{ID: "observability-main", Vendor: "tars"},
					Spec: Spec{
						Type:         "observability",
						Protocol:     "observability_http",
						Capabilities: []Capability{{ID: "observability.query", Action: "query", ReadOnly: true}},
					},
					Compatibility: Compatibility{TARSMajorVersions: []string{"1"}},
				},
			},
		},
		state: map[string]LifecycleState{
			"observability-stub": {ConnectorID: "observability-stub", Health: HealthStatus{Status: "healthy"}},
			"observability-main": {ConnectorID: "observability-main", Health: HealthStatus{Status: "healthy"}},
		},
	}

	items := ToolPlanCapabilities(manager)
	if len(items) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(items))
	}
	if items[0].ConnectorID != "observability-main" {
		t.Fatalf("expected real observability connector to be preferred, got %+v", items[0])
	}
}
func TestToolPlanCapabilitiesVictoriaLogsFirstClass(t *testing.T) {
	t.Parallel()

	manager := &Manager{
		config: &Config{
			Entries: []Manifest{
				{
					Metadata: Metadata{ID: "victorialogs-main", Vendor: "victoriametrics"},
					Spec: Spec{
						Type:     "logs",
						Protocol: "victorialogs_http",
						Capabilities: []Capability{
							{ID: "logs.query", Action: "query", ReadOnly: true, Invocable: true, Description: "Query logs using LogsQL"},
							{ID: "victorialogs.query", Action: "query", ReadOnly: true, Invocable: true, Description: "Alias for logs.query"},
						},
					},
					Compatibility: Compatibility{TARSMajorVersions: []string{"1"}},
				},
			},
		},
		state: map[string]LifecycleState{
			"victorialogs-main": {ConnectorID: "victorialogs-main", Health: HealthStatus{Status: "healthy"}},
		},
	}

	items := ToolPlanCapabilities(manager)
	if len(items) != 2 {
		t.Fatalf("expected 2 capability entries (logs.query + victorialogs.query), got %d", len(items))
	}
	// All capabilities should map to logs.query tool
	for _, item := range items {
		if item.Tool != "logs.query" {
			t.Errorf("expected tool=logs.query for victorialogs, got %q (capability: %s)", item.Tool, item.CapabilityID)
		}
		if !item.Invocable {
			t.Errorf("expected invocable=true for logs.query, got false (capability: %s)", item.CapabilityID)
		}
		if item.Protocol != "victorialogs_http" {
			t.Errorf("expected protocol=victorialogs_http, got %q", item.Protocol)
		}
	}
}

func TestNormalizeCapabilityConnectorTypeVictoriaLogs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"logs", "logs"},
		{"victorialogs", "logs"},
		{"LOGS", "logs"},
		{"VictoriaLogs", "logs"},
		{"metrics", "metrics"},
		{"observability", "observability"},
		{"mcp_tool", "mcp"},
		{"skill_source", "skill"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := NormalizeCapabilityConnectorType(tc.input)
			if got != tc.want {
				t.Fatalf("NormalizeCapabilityConnectorType(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestCapabilityProtocolRankVictoriaLogsAndSSH(t *testing.T) {
	t.Parallel()

	// victorialogs_http and ssh_native must be at rank 0 (first-class)
	for _, proto := range []string{"victorialogs_http", "ssh_native", "victoriametrics_http"} {
		proto := proto
		t.Run(proto, func(t *testing.T) {
			t.Parallel()
			m := Manifest{Spec: Spec{Protocol: proto}}
			if got := capabilityProtocolRank(m); got != 0 {
				t.Fatalf("expected rank 0 for %s, got %d", proto, got)
			}
		})
	}
}
