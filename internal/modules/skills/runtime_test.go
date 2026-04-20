package skills

import (
	"os"
	"testing"

	"tars/internal/contracts"
)

func TestManagerMatchAndExpand(t *testing.T) {
	t.Parallel()

	configPath := t.TempDir() + "/skills.yaml"
	if err := os.WriteFile(configPath, []byte(`skills:
  entries:
    - api_version: tars.skill/v1alpha1
      kind: skill_package
      metadata:
        id: disk-space-incident
        name: disk-space-incident
        display_name: Disk Space Incident
        version: 1.0.0
        vendor: tars
        source: official
      spec:
        type: incident_skill
        triggers:
          alerts: ["DiskSpaceLow"]
        planner:
          summary: Metrics-first disk plan.
          preferred_tools: ["metrics.query_range","execution.run_command"]
          steps:
            - id: metrics_capacity
              tool: metrics.query_range
              required: true
              reason: Check metrics first.
              params:
                query: node_filesystem_avail_bytes
            - id: host_probe
              tool: execution.run_command
              reason: Inspect host only after metrics.
              params:
                command: df -h
              approval:
                default: require_approval
`), 0o600); err != nil {
		t.Fatalf("write skills config: %v", err)
	}
	manager, err := NewManager(configPath, "")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if _, _, err := manager.Promote("disk-space-incident", PromoteOptions{OperatorReason: "activate skill", ReviewState: "approved", RuntimeMode: "planner_visible"}); err != nil {
		t.Fatalf("promote: %v", err)
	}

	match := manager.Match(contracts.DiagnosisInput{Context: map[string]interface{}{"alert_name": "DiskSpaceLow", "host": "host-1", "service": "api"}})
	if match == nil || match.SkillID != "disk-space-incident" {
		t.Fatalf("expected disk skill match, got %+v", match)
	}

	steps := manager.Expand(*match, contracts.DiagnosisInput{Context: map[string]interface{}{"alert_name": "DiskSpaceLow", "host": "host-1", "service": "api"}}, []contracts.ToolCapabilityDescriptor{
		{Tool: "metrics.query_range", ConnectorID: "victoriametrics-main", Invocable: true},
		{Tool: "execution.run_command", ConnectorID: "jumpserver-main", Invocable: true},
	})
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %+v", steps)
	}
	if steps[0].Tool != "metrics.query_range" || steps[0].ConnectorID != "victoriametrics-main" {
		t.Fatalf("expected metrics step with preferred connector, got %+v", steps[0])
	}
	if steps[1].Tool != "execution.run_command" || steps[1].OnPendingApproval != "stop" {
		t.Fatalf("expected guarded execution step, got %+v", steps[1])
	}
	if got := steps[1].Input["host"]; got != "host-1" {
		t.Fatalf("expected host injected into execution step, got %+v", steps[1].Input)
	}
}

func TestManagerPlannerDescriptorsOnlyReturnEnabledSkills(t *testing.T) {
	t.Parallel()

	configPath := t.TempDir() + "/skills.yaml"
	if err := os.WriteFile(configPath, []byte(`skills:
  entries:
    - metadata:
        id: enabled-skill
        name: enabled-skill
        display_name: Enabled Skill
        version: 1.0.0
      spec:
        type: incident_skill
        planner:
          steps: []
    - metadata:
        id: disabled-skill
        name: disabled-skill
        display_name: Disabled Skill
        version: 1.0.0
      disabled: true
      spec:
        type: incident_skill
        planner:
          steps: []
`), 0o600); err != nil {
		t.Fatalf("write skills config: %v", err)
	}
	manager, err := NewManager(configPath, "")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	items := manager.PlannerDescriptors()
	if len(items) != 1 || items[0].CapabilityID != "enabled-skill" {
		t.Fatalf("expected only enabled skill descriptor, got %+v", items)
	}
}
