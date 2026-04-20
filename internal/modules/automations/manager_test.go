package automations

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"tars/internal/contracts"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/connectors"
	"tars/internal/modules/skills"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/automations.yaml"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	return path
}

func minimalJob(id, jobType, schedule string) Job {
	job := Job{
		ID:       id,
		Type:     jobType,
		Schedule: schedule,
		Enabled:  true,
	}
	switch jobType {
	case jobTypeSkill:
		job.TargetRef = "my-skill"
		job.Skill = &SkillTarget{SkillID: "my-skill"}
	case jobTypeConnectorCapability:
		job.TargetRef = "conn/cap"
		job.ConnectorCapability = &ConnectorCapabilityJob{
			ConnectorID:  "conn",
			CapabilityID: "cap",
		}
	}
	return job
}

type fakeAutomationActionService struct {
	metricsCalls     int
	capabilityCalls  int
	lastMetricsQuery string
}

type captureAutomationReasoning struct {
	lastBuild    contracts.DiagnosisInput
	lastPlan     contracts.DiagnosisInput
	lastFinalize contracts.DiagnosisInput
	plan         contracts.DiagnosisPlan
	output       contracts.DiagnosisOutput
}

func (c *captureAutomationReasoning) BuildDiagnosis(_ context.Context, input contracts.DiagnosisInput) (contracts.DiagnosisOutput, error) {
	c.lastBuild = input
	return c.output, nil
}

func (c *captureAutomationReasoning) PlanDiagnosis(_ context.Context, input contracts.DiagnosisInput) (contracts.DiagnosisPlan, error) {
	c.lastPlan = input
	return c.plan, nil
}

func (c *captureAutomationReasoning) FinalizeDiagnosis(_ context.Context, input contracts.DiagnosisInput) (contracts.DiagnosisOutput, error) {
	c.lastFinalize = input
	return c.output, nil
}

func (f *fakeAutomationActionService) QueryMetrics(_ context.Context, query contracts.MetricsQuery) (contracts.MetricsResult, error) {
	f.metricsCalls++
	f.lastMetricsQuery = query.Query
	return contracts.MetricsResult{}, nil
}

func (f *fakeAutomationActionService) ExecuteApproved(_ context.Context, _ contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
	return contracts.ExecutionResult{}, nil
}

func (f *fakeAutomationActionService) InvokeApprovedCapability(_ context.Context, _ contracts.ApprovedCapabilityRequest) (contracts.CapabilityResult, error) {
	return contracts.CapabilityResult{}, nil
}

func (f *fakeAutomationActionService) VerifyExecution(_ context.Context, _ contracts.VerificationRequest) (contracts.VerificationResult, error) {
	return contracts.VerificationResult{}, nil
}

func (f *fakeAutomationActionService) CheckConnectorHealth(_ context.Context, _ string) (connectors.LifecycleState, error) {
	return connectors.LifecycleState{}, nil
}

func (f *fakeAutomationActionService) InvokeCapability(_ context.Context, _ contracts.CapabilityRequest) (contracts.CapabilityResult, error) {
	f.capabilityCalls++
	return contracts.CapabilityResult{
		Status: "completed",
		Output: map[string]interface{}{"summary": "capability completed"},
	}, nil
}

func newSkillManagerWithConfig(t *testing.T, content string) *skills.Manager {
	t.Helper()
	path := t.TempDir() + "/skills.yaml"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write skills config: %v", err)
	}
	manager, err := skills.NewManager(path, "")
	if err != nil {
		t.Fatalf("skills.NewManager: %v", err)
	}
	return manager
}

func newConnectorsManagerWithConfig(t *testing.T, content string) *connectors.Manager {
	t.Helper()
	path := t.TempDir() + "/connectors.yaml"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write connectors config: %v", err)
	}
	manager, err := connectors.NewManager(path)
	if err != nil {
		t.Fatalf("connectors.NewManager: %v", err)
	}
	return manager
}

// ---------------------------------------------------------------------------
// normalizeJob
// ---------------------------------------------------------------------------

func TestNormalizeJob_MissingID(t *testing.T) {
	t.Parallel()
	_, err := normalizeJob(Job{Type: jobTypeSkill, Schedule: "@every 1m", Skill: &SkillTarget{SkillID: "x"}})
	if err == nil {
		t.Fatal("expected error for missing ID, got nil")
	}
}

func TestNormalizeJob_MissingSchedule(t *testing.T) {
	t.Parallel()
	_, err := normalizeJob(Job{ID: "j1", Type: jobTypeSkill, Skill: &SkillTarget{SkillID: "x"}})
	if err == nil {
		t.Fatal("expected error for missing schedule, got nil")
	}
}

func TestNormalizeJob_InvalidSchedule(t *testing.T) {
	t.Parallel()
	_, err := normalizeJob(Job{ID: "j1", Type: jobTypeSkill, Schedule: "not-a-cron", Skill: &SkillTarget{SkillID: "x"}})
	if err == nil {
		t.Fatal("expected error for invalid schedule, got nil")
	}
}

func TestNormalizeJob_UnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := normalizeJob(Job{ID: "j1", Type: "unknown_type", Schedule: "@every 1m"})
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
}

func TestNormalizeJob_SkillType_Defaults(t *testing.T) {
	t.Parallel()
	job := Job{
		ID:        "j1",
		Type:      jobTypeSkill,
		Schedule:  "@every 5m",
		TargetRef: "my-skill",
	}
	normalized, err := normalizeJob(job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized.Skill == nil || normalized.Skill.SkillID != "my-skill" {
		t.Fatalf("expected Skill.SkillID = my-skill, got %+v", normalized.Skill)
	}
	if normalized.ConnectorCapability != nil {
		t.Fatal("ConnectorCapability should be nil for skill jobs")
	}
	if normalized.RuntimeMode != "managed" {
		t.Fatalf("expected default runtime_mode = managed, got %q", normalized.RuntimeMode)
	}
	if normalized.TimeoutSeconds != int(defaultRunTimeout.Seconds()) {
		t.Fatalf("expected default timeout = %d, got %d", int(defaultRunTimeout.Seconds()), normalized.TimeoutSeconds)
	}
	if normalized.GovernancePolicy != "auto" {
		t.Fatalf("expected default governance_policy = auto, got %q", normalized.GovernancePolicy)
	}
}

func TestNormalizeJob_ConnectorCapability_TargetRefParsed(t *testing.T) {
	t.Parallel()
	job := Job{
		ID:        "j2",
		Type:      jobTypeConnectorCapability,
		Schedule:  "0 * * * *",
		TargetRef: "myconn/mycap",
	}
	normalized, err := normalizeJob(job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized.ConnectorCapability == nil {
		t.Fatal("expected ConnectorCapability to be set")
	}
	if normalized.ConnectorCapability.ConnectorID != "myconn" {
		t.Fatalf("expected ConnectorID=myconn, got %q", normalized.ConnectorCapability.ConnectorID)
	}
	if normalized.ConnectorCapability.CapabilityID != "mycap" {
		t.Fatalf("expected CapabilityID=mycap, got %q", normalized.ConnectorCapability.CapabilityID)
	}
	if normalized.Skill != nil {
		t.Fatal("Skill should be nil for connector_capability jobs")
	}
}

func TestNormalizeJob_ConnectorCapability_MissingIDs(t *testing.T) {
	t.Parallel()
	job := Job{
		ID:                  "j3",
		Type:                jobTypeConnectorCapability,
		Schedule:            "@every 1m",
		ConnectorCapability: &ConnectorCapabilityJob{},
	}
	_, err := normalizeJob(job)
	if err == nil {
		t.Fatal("expected error for missing connector/capability IDs, got nil")
	}
}

func TestNormalizeJob_InvalidRetryBackoff(t *testing.T) {
	t.Parallel()
	job := Job{
		ID:                  "j4",
		Type:                jobTypeSkill,
		Schedule:            "@every 1m",
		Skill:               &SkillTarget{SkillID: "x"},
		RetryInitialBackoff: "not-a-duration",
	}
	_, err := normalizeJob(job)
	if err == nil {
		t.Fatal("expected error for invalid retry_initial_backoff, got nil")
	}
}

func TestNormalizeJob_InvalidGovernancePolicy(t *testing.T) {
	t.Parallel()
	job := Job{
		ID:               "j5",
		Type:             jobTypeSkill,
		Schedule:         "@every 1m",
		Skill:            &SkillTarget{SkillID: "x"},
		GovernancePolicy: "not-valid",
	}
	_, err := normalizeJob(job)
	if err == nil {
		t.Fatal("expected error for invalid governance_policy, got nil")
	}
}

func TestNormalizeJob_DisplayNameFallsBackToID(t *testing.T) {
	t.Parallel()
	job := Job{
		ID:       "my-job-id",
		Type:     jobTypeSkill,
		Schedule: "@every 1m",
		Skill:    &SkillTarget{SkillID: "x"},
	}
	normalized, err := normalizeJob(job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized.DisplayName != "my-job-id" {
		t.Fatalf("expected DisplayName=my-job-id, got %q", normalized.DisplayName)
	}
}

// ---------------------------------------------------------------------------
// parseSchedule
// ---------------------------------------------------------------------------

func TestParseSchedule_EveryDuration(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	next, err := parseSchedule("@every 10m", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := now.Add(10 * time.Minute)
	if !next.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, next)
	}
}

func TestParseSchedule_CronExpression(t *testing.T) {
	t.Parallel()
	// 0 * * * * = top of every hour
	now := time.Date(2026, 1, 1, 12, 30, 0, 0, time.UTC)
	next, err := parseSchedule("0 * * * *", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// next should be 13:00
	if next.Hour() != 13 || next.Minute() != 0 {
		t.Fatalf("expected 13:00, got %v", next)
	}
}

func TestParseSchedule_Descriptor_Daily(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next, err := parseSchedule("@daily", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next.Before(now) {
		t.Fatalf("next run must be after now, got %v", next)
	}
}

func TestParseSchedule_ZeroDuration(t *testing.T) {
	t.Parallel()
	_, err := parseSchedule("@every 0s", time.Now())
	if err == nil {
		t.Fatal("expected error for zero duration, got nil")
	}
}

func TestParseSchedule_NegativeDuration(t *testing.T) {
	t.Parallel()
	_, err := parseSchedule("@every -1m", time.Now())
	if err == nil {
		t.Fatal("expected error for negative duration, got nil")
	}
}

// ---------------------------------------------------------------------------
// syncState
// ---------------------------------------------------------------------------

func TestSyncState_NewJobAdded(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := &Config{
		Jobs: []Job{
			{ID: "j1", Type: jobTypeSkill, Schedule: "@every 1h", Enabled: true, Skill: &SkillTarget{SkillID: "x"}},
		},
	}
	state := syncState(nil, cfg, now)
	s, ok := state["j1"]
	if !ok {
		t.Fatal("expected state for j1")
	}
	if s.Status != "enabled" {
		t.Fatalf("expected status=enabled, got %q", s.Status)
	}
	if s.NextRunAt.IsZero() {
		t.Fatal("expected NextRunAt to be set for enabled job")
	}
}

func TestSyncState_DisabledJobHasNoNextRun(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := &Config{
		Jobs: []Job{
			{ID: "j1", Type: jobTypeSkill, Schedule: "@every 1h", Enabled: false, Skill: &SkillTarget{SkillID: "x"}},
		},
	}
	state := syncState(nil, cfg, now)
	s := state["j1"]
	if !s.NextRunAt.IsZero() {
		t.Fatalf("expected empty NextRunAt for disabled job, got %v", s.NextRunAt)
	}
	if s.Status != "disabled" {
		t.Fatalf("expected status=disabled, got %q", s.Status)
	}
}

func TestSyncState_RemovedJobPruned(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := map[string]JobState{
		"old-job": {JobID: "old-job", Status: "enabled"},
	}
	cfg := &Config{
		Jobs: []Job{
			{ID: "new-job", Type: jobTypeSkill, Schedule: "@every 1h", Enabled: true, Skill: &SkillTarget{SkillID: "x"}},
		},
	}
	state := syncState(existing, cfg, now)
	if _, ok := state["old-job"]; ok {
		t.Fatal("expected old-job to be pruned from state")
	}
	if _, ok := state["new-job"]; !ok {
		t.Fatal("expected new-job to exist in state")
	}
}

// ---------------------------------------------------------------------------
// shouldRetry / statusForJob helpers
// ---------------------------------------------------------------------------

func TestShouldRetry(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status string
		want   bool
	}{
		{"failed", true},
		{"FAILED", true},
		{"completed", false},
		{"blocked", false},
		{"running", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := shouldRetry(tc.status); got != tc.want {
			t.Errorf("shouldRetry(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestStatusForJob(t *testing.T) {
	t.Parallel()
	if statusForJob(Job{Enabled: true}) != "enabled" {
		t.Fatal("expected enabled")
	}
	if statusForJob(Job{Enabled: false}) != "disabled" {
		t.Fatal("expected disabled")
	}
}

// ---------------------------------------------------------------------------
// clampInt
// ---------------------------------------------------------------------------

func TestClampInt(t *testing.T) {
	t.Parallel()
	if clampInt(-1, 0, 5) != 0 {
		t.Fatal("expected 0")
	}
	if clampInt(10, 0, 5) != 5 {
		t.Fatal("expected 5")
	}
	if clampInt(3, 0, 5) != 3 {
		t.Fatal("expected 3")
	}
}

// ---------------------------------------------------------------------------
// Manager lifecycle via temp files (no external deps)
// ---------------------------------------------------------------------------

const minimalYAML = `automations:
  jobs:
    - id: job-a
      type: skill
      display_name: "Job A"
      schedule: "@every 10m"
      enabled: true
      skill:
        skill_id: my-skill
`

func TestManagerReloadAndGet(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	job, state, ok := mgr.Get("job-a")
	if !ok {
		t.Fatal("expected job-a to exist")
	}
	if job.ID != "job-a" {
		t.Fatalf("unexpected job ID: %s", job.ID)
	}
	if state.Status != "enabled" {
		t.Fatalf("expected status enabled, got %s", state.Status)
	}
	if state.NextRunAt.IsZero() {
		t.Fatal("expected NextRunAt to be set")
	}
}

func TestManagerGetMissingJob(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	_, _, ok := mgr.Get("does-not-exist")
	if ok {
		t.Fatal("expected false for missing job")
	}
}

func TestManagerSnapshot(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	snap := mgr.Snapshot()
	if !snap.Loaded {
		t.Fatal("expected Loaded=true")
	}
	if len(snap.Config.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(snap.Config.Jobs))
	}
}

func TestManagerUpsertNewJob(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	newJob := Job{
		ID:       "job-b",
		Type:     jobTypeSkill,
		Schedule: "@every 5m",
		Enabled:  true,
		Skill:    &SkillTarget{SkillID: "another-skill"},
	}
	got, _, err := mgr.Upsert(newJob)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if got.ID != "job-b" {
		t.Fatalf("expected job-b, got %s", got.ID)
	}
	snap := mgr.Snapshot()
	if len(snap.Config.Jobs) != 2 {
		t.Fatalf("expected 2 jobs after upsert, got %d", len(snap.Config.Jobs))
	}
}

func TestManagerSetEnabled(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	job, state, err := mgr.SetEnabled("job-a", false)
	if err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if job.Enabled {
		t.Fatal("expected job to be disabled")
	}
	if state.Status != "disabled" {
		t.Fatalf("expected state.Status=disabled, got %s", state.Status)
	}
	if !state.NextRunAt.IsZero() {
		t.Fatal("disabled job should have zero NextRunAt")
	}
}

func TestManagerSetEnabled_NotFound(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	_, _, err = mgr.SetEnabled("no-such-job", false)
	if err == nil {
		t.Fatal("expected error for missing job")
	}
}

func TestManagerRunNow_NotFound(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	_, err = mgr.RunNow(t.Context(), "no-such-job", RunRequest{Trigger: "manual"})
	if err == nil {
		t.Fatal("expected ErrJobNotFound, got nil")
	}
}

// ---------------------------------------------------------------------------
// Agent Role Runtime Semantics
// ---------------------------------------------------------------------------

// newAgentRoleManager creates an in-memory agentrole.Manager for testing.
func newAgentRoleManager(t *testing.T) *agentrole.Manager {
	t.Helper()
	mgr, err := agentrole.NewManager("", agentrole.Options{})
	if err != nil {
		t.Fatalf("agentrole.NewManager: %v", err)
	}
	return mgr
}

// TestAgentRole_FallbackToAutomationOperator verifies that when agent_role_id is empty,
// resolveJobRole returns the "automation_operator" builtin role.
func TestAgentRole_FallbackToAutomationOperator(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{AgentRoles: newAgentRoleManager(t)})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	job := Job{ID: "j1", AgentRoleID: ""}
	role := mgr.resolveJobRole(job)
	if role.RoleID != "automation_operator" {
		t.Fatalf("expected fallback to automation_operator, got %q", role.RoleID)
	}
}

// TestAgentRole_ExplicitRoleResolved verifies that a valid agent_role_id is resolved correctly.
func TestAgentRole_ExplicitRoleResolved(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{AgentRoles: newAgentRoleManager(t)})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// "diagnosis" is a builtin role with whitelist capability binding.
	job := Job{ID: "j1", AgentRoleID: "diagnosis"}
	role := mgr.resolveJobRole(job)
	if role.RoleID != "diagnosis" {
		t.Fatalf("expected diagnosis role, got %q", role.RoleID)
	}
	if role.CapabilityBinding.Mode != "whitelist" {
		t.Fatalf("expected whitelist mode for diagnosis role, got %q", role.CapabilityBinding.Mode)
	}
}

// TestAgentRole_AllowedSkillsWhitelistBlocks verifies that when a role has an AllowedSkills list,
// executeSkillJob blocks a skill not in the list, and allows one that is.
func TestAgentRole_AllowedSkillsWhitelistBlocks(t *testing.T) {
	t.Parallel()
	restrictedRole := agentrole.AgentRole{
		RoleID:      "restricted-automation",
		DisplayName: "Restricted Automation",
		Status:      "active",
		CapabilityBinding: agentrole.CapabilityBinding{
			Mode:          "whitelist",
			AllowedSkills: []string{"allowed-skill"},
		},
		PolicyBinding: agentrole.PolicyBinding{
			MaxRiskLevel: "critical",
			MaxAction:    "direct_execute",
		},
	}
	// isSkillAllowed replicates the AllowedSkills check logic from executeSkillJob.
	isSkillAllowed := func(role agentrole.AgentRole, skillID string) bool {
		if len(role.CapabilityBinding.AllowedSkills) == 0 {
			return true
		}
		for _, s := range role.CapabilityBinding.AllowedSkills {
			if strings.EqualFold(s, skillID) {
				return true
			}
		}
		return false
	}
	if isSkillAllowed(restrictedRole, "forbidden-skill") {
		t.Fatal("forbidden-skill should not be allowed by the restricted role")
	}
	if !isSkillAllowed(restrictedRole, "allowed-skill") {
		t.Fatal("allowed-skill should be allowed by the restricted role")
	}
	// Case-insensitive matching.
	if !isSkillAllowed(restrictedRole, "ALLOWED-SKILL") {
		t.Fatal("AllowedSkills check must be case-insensitive")
	}
	// Unrestricted role (empty AllowedSkills) allows any skill.
	unrestrictedRole := agentrole.AgentRole{
		RoleID:            "automation_operator",
		CapabilityBinding: agentrole.CapabilityBinding{Mode: "unrestricted"},
	}
	if !isSkillAllowed(unrestrictedRole, "any-skill") {
		t.Fatal("unrestricted role with no AllowedSkills should allow any skill")
	}
}

// TestAgentRole_CapabilityBindingBlocksStep verifies that EnforceCapabilityBinding in whitelist mode
// denies a capability not in the allowed list.
func TestAgentRole_CapabilityBindingBlocksStep(t *testing.T) {
	t.Parallel()
	role := agentrole.AgentRole{
		RoleID: "diagnosis",
		CapabilityBinding: agentrole.CapabilityBinding{
			Mode: "whitelist",
			AllowedConnectorCapabilities: []string{
				"metrics.query_instant",
				"observability.query",
			},
		},
	}
	// A capability that IS in the whitelist should not be denied.
	allowed := agentrole.EnforceCapabilityBinding(role, "direct_execute", "metrics.query_instant")
	if allowed == "deny" {
		t.Fatal("metrics.query_instant should be allowed by diagnosis role")
	}
	// A capability NOT in the whitelist must be denied.
	denied := agentrole.EnforceCapabilityBinding(role, "direct_execute", "execution.run_command")
	if denied != "deny" {
		t.Fatalf("execution.run_command should be denied by diagnosis role whitelist, got %q", denied)
	}
}

// TestAgentRole_PolicyHardDenyBlocksTool verifies that EnforcePolicy with a HardDeny entry
// returns "deny" regardless of current action.
func TestAgentRole_PolicyHardDenyBlocksTool(t *testing.T) {
	t.Parallel()
	role := agentrole.AgentRole{
		RoleID: "no-exec",
		PolicyBinding: agentrole.PolicyBinding{
			MaxRiskLevel: "critical",
			MaxAction:    "direct_execute",
			HardDeny:     []string{"execution.run_command"},
		},
	}
	result := agentrole.EnforcePolicy(role, "direct_execute", "execution.run_command")
	if result != "deny" {
		t.Fatalf("expected deny for hard-denied command, got %q", result)
	}
	// A non-denied command should pass through.
	result = agentrole.EnforcePolicy(role, "direct_execute", "metrics.query_instant")
	if result == "deny" {
		t.Fatalf("metrics.query_instant should not be denied, got %q", result)
	}
}

// TestAgentRole_RunMetadataContainsResolvedRoleID verifies that resolved_agent_role_id is written
// into run.Metadata when runJob starts.
func TestAgentRole_RunMetadataContainsResolvedRoleID(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{AgentRoles: newAgentRoleManager(t)})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// Simulate what runJob does: resolve role and check metadata is populated.
	job := Job{ID: "job-a", AgentRoleID: ""}
	role := mgr.resolveJobRole(job)

	run := Run{
		RunID:  "test-meta-run",
		JobID:  job.ID,
		Status: "running",
		Metadata: map[string]interface{}{
			"triggered_by": "test",
		},
	}
	run.Metadata["resolved_agent_role_id"] = role.RoleID
	run.Metadata["resolved_agent_role_display_name"] = role.DisplayName

	if run.Metadata["resolved_agent_role_id"] != "automation_operator" {
		t.Fatalf("expected resolved_agent_role_id=automation_operator, got %v", run.Metadata["resolved_agent_role_id"])
	}
	if run.Metadata["resolved_agent_role_display_name"] == "" {
		t.Fatal("expected resolved_agent_role_display_name to be non-empty")
	}
}

// TestAgentRole_PolicyRequireApprovalDowngradesAction verifies that RequireApprovalFor
// downgrades direct_execute to require_approval for matching commands.
func TestAgentRole_PolicyRequireApprovalDowngradesAction(t *testing.T) {
	t.Parallel()
	role := agentrole.AgentRole{
		RoleID: "safe-operator",
		PolicyBinding: agentrole.PolicyBinding{
			MaxRiskLevel:       "critical",
			MaxAction:          "direct_execute",
			RequireApprovalFor: []string{"execution.run_command"},
		},
	}
	result := agentrole.EnforcePolicy(role, "direct_execute", "execution.run_command")
	if result != "require_approval" {
		t.Fatalf("expected require_approval for guarded command, got %q", result)
	}
	// Unrelated command should still be direct_execute.
	result = agentrole.EnforcePolicy(role, "direct_execute", "metrics.query_instant")
	if result != "direct_execute" {
		t.Fatalf("expected direct_execute for unguarded command, got %q", result)
	}
}

// TestAgentRole_ExecuteSkillSteps_CapabilityBindingBlocks verifies that executeSkillSteps
// blocks a step when the role's capability binding denies the tool.
func TestAgentRole_ExecuteSkillSteps_CapabilityBindingBlocks(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, minimalYAML)
	mgr, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// Role with whitelist that does NOT include execution.run_command.
	role := agentrole.AgentRole{
		RoleID: "read-only",
		CapabilityBinding: agentrole.CapabilityBinding{
			Mode:                         "whitelist",
			AllowedConnectorCapabilities: []string{"metrics.query_instant"},
		},
		PolicyBinding: agentrole.PolicyBinding{
			MaxRiskLevel: "critical",
			MaxAction:    "direct_execute",
		},
	}
	// EnforceCapabilityBinding directly verifies the binding logic.
	decision := agentrole.EnforceCapabilityBinding(role, "direct_execute", "execution.run_command")
	if decision != "deny" {
		t.Fatalf("expected deny for execution.run_command under read-only role, got %q", decision)
	}
	decision2 := agentrole.EnforceCapabilityBinding(role, "direct_execute", "metrics.query_instant")
	if decision2 == "deny" {
		t.Fatal("metrics.query_instant should be allowed under this role")
	}

	// Build a minimal ToolPlanStep that will be denied by the role binding,
	// and run executeSkillSteps to confirm the integration path blocks before invocation.
	ctx := context.Background()
	steps := buildTestSteps("execution.run_command")
	result, execErr := mgr.executeSkillSteps(ctx, steps, nil, role)
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	if !result.Blocked {
		t.Fatal("expected Blocked=true when capability binding denies the step")
	}
	if result.BlockedTool != "execution.run_command" {
		t.Fatalf("expected BlockedTool=execution.run_command, got %q", result.BlockedTool)
	}
}

func TestExecuteSkillJob_AllowedSkillTagsRestrictByManifestTags(t *testing.T) {
	t.Parallel()

	skillManager := newSkillManagerWithConfig(t, `skills:
  entries:
    - api_version: tars.skill/v1alpha1
      kind: skill_package
      metadata:
        id: tagged-skill
        name: tagged-skill
        display_name: Tagged Skill
        version: 1.0.0
        vendor: tars
        source: official
        tags: ["storage", "ops"]
      spec:
        type: incident_skill
        planner:
          steps:
            - id: metrics_probe
              tool: metrics.query_instant
              params:
                query: up
`)
	manager, err := NewManager(writeConfig(t, minimalYAML), Options{
		Skills: skillManager,
		Action: &fakeAutomationActionService{},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	job := Job{
		ID: "tagged-job",
		Skill: &SkillTarget{
			SkillID: "tagged-skill",
			Context: map[string]interface{}{"host": "host-1"},
		},
	}
	baseRun := Run{RunID: "run-tagged", JobID: job.ID, Status: "running", Metadata: map[string]interface{}{}}

	t.Run("matching tag allows execution", func(t *testing.T) {
		action := &fakeAutomationActionService{}
		manager.action = action
		role := agentrole.AgentRole{
			RoleID: "tag-allowed",
			CapabilityBinding: agentrole.CapabilityBinding{
				Mode:             "unrestricted",
				AllowedSkillTags: []string{"storage"},
			},
			PolicyBinding: agentrole.PolicyBinding{
				MaxRiskLevel: "critical",
				MaxAction:    "direct_execute",
			},
		}
		run, execErr := manager.executeSkillJob(context.Background(), job, baseRun, role)
		if execErr != nil {
			t.Fatalf("executeSkillJob: %v", execErr)
		}
		if run.Status != "completed" {
			t.Fatalf("expected completed run for matching tag, got %+v", run)
		}
		if action.metricsCalls != 1 {
			t.Fatalf("expected one metrics execution for matching tag, got %d", action.metricsCalls)
		}
	})

	t.Run("non-matching tag blocks before execution", func(t *testing.T) {
		action := &fakeAutomationActionService{}
		manager.action = action
		role := agentrole.AgentRole{
			RoleID: "tag-blocked",
			CapabilityBinding: agentrole.CapabilityBinding{
				Mode:             "unrestricted",
				AllowedSkillTags: []string{"network"},
			},
			PolicyBinding: agentrole.PolicyBinding{
				MaxRiskLevel: "critical",
				MaxAction:    "direct_execute",
			},
		}
		run, execErr := manager.executeSkillJob(context.Background(), job, baseRun, role)
		if execErr != nil {
			t.Fatalf("executeSkillJob: %v", execErr)
		}
		if run.Status != "blocked" {
			t.Fatalf("expected blocked run for non-matching tag, got %+v", run)
		}
		if action.metricsCalls != 0 {
			t.Fatalf("expected no metrics execution when tag is blocked, got %d", action.metricsCalls)
		}
	})
}

func TestExecuteSkillJob_UsesRoleBoundReasoningPlannerAndFinalizer(t *testing.T) {
	t.Parallel()

	skillManager := newSkillManagerWithConfig(t, `skills:
  entries:
    - api_version: tars.skill/v1alpha1
      kind: skill_package
      metadata:
        id: tagged-skill
        name: tagged-skill
        display_name: Tagged Skill
        version: 1.0.0
        vendor: tars
        source: official
        tags: ["storage", "ops"]
      spec:
        type: incident_skill
        planner:
          steps:
            - id: metrics_probe
              tool: metrics.query_instant
              params:
                query: up
`)
	reasoning := &captureAutomationReasoning{
		plan: contracts.DiagnosisPlan{
			Summary: "planner selected a runtime-aware metrics probe",
			ToolPlan: []contracts.ToolPlanStep{
				{
					ID:    "planner_metrics_probe",
					Tool:  "metrics.query_instant",
					Input: map[string]interface{}{"query": "planner_up"},
				},
				{
					ID:    "planner_forbidden_step",
					Tool:  "execution.run_command",
					Input: map[string]interface{}{"command": "cat /etc/passwd"},
				},
			},
		},
		output: contracts.DiagnosisOutput{
			Summary:       "reasoned automation summary",
			ExecutionHint: "hostname",
		},
	}
	action := &fakeAutomationActionService{}
	manager, err := NewManager(writeConfig(t, minimalYAML), Options{
		Skills:    skillManager,
		Action:    action,
		Reasoning: reasoning,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	job := Job{
		ID:               "tagged-job",
		DisplayName:      "Tagged Job",
		GovernancePolicy: "auto",
		Skill: &SkillTarget{
			SkillID: "tagged-skill",
			Context: map[string]interface{}{"host": "host-1"},
		},
	}
	baseRun := Run{RunID: "run-tagged", JobID: job.ID, Status: "running", Metadata: map[string]interface{}{}}
	role := agentrole.AgentRole{
		RoleID:      "automation-operator",
		DisplayName: "Automation Operator",
		Profile: agentrole.Profile{
			SystemPrompt: "summarize automation results carefully",
		},
		CapabilityBinding: agentrole.CapabilityBinding{
			Mode:             "unrestricted",
			AllowedSkillTags: []string{"storage"},
		},
		PolicyBinding: agentrole.PolicyBinding{
			MaxRiskLevel: "critical",
			MaxAction:    "direct_execute",
		},
		ModelBinding: agentrole.ModelBinding{
			Primary: &agentrole.ModelTargetBinding{
				ProviderID: "openai-main",
				Model:      "gpt-4.1-mini",
			},
		},
	}

	run, execErr := manager.executeSkillJob(context.Background(), job, baseRun, role)
	if execErr != nil {
		t.Fatalf("executeSkillJob: %v", execErr)
	}
	if run.Status != "completed" {
		t.Fatalf("expected completed run, got %+v", run)
	}
	if run.Summary != "reasoned automation summary" {
		t.Fatalf("expected reasoning summary override, got %+v", run)
	}
	if got := reasoning.lastFinalize.RoleModelBinding; got == nil {
		t.Fatal("expected automation reasoning to receive role model binding")
	} else {
		if got.Primary == nil || got.Primary.ProviderID != "openai-main" || got.Primary.Model != "gpt-4.1-mini" {
			t.Fatalf("unexpected primary model binding: %+v", got)
		}
	}
	if got := reasoning.lastPlan.RoleModelBinding; got == nil {
		t.Fatal("expected planner to receive role model binding")
	} else if got.Primary == nil || got.Primary.Model != "gpt-4.1-mini" {
		t.Fatalf("unexpected planner primary model binding: %+v", got)
	}
	if got := action.lastMetricsQuery; got != "planner_up" {
		t.Fatalf("expected execution to follow planner-produced query, got %q", got)
	}
	if got := run.Metadata["automation_planner_source"]; got != "reasoning" {
		t.Fatalf("expected reasoning planner metadata, got %#v", got)
	}
	if got := reasoning.lastFinalize.Context["automation_job_id"]; got != "tagged-job" {
		t.Fatalf("expected automation_job_id in reasoning context, got %#v", got)
	}
	if got := reasoning.lastFinalize.Context["agent_role_system_prompt"]; got != "summarize automation results carefully" {
		t.Fatalf("expected role system prompt in reasoning context, got %#v", got)
	}
	if got := run.Metadata["automation_execution_hint"]; got != "hostname" {
		t.Fatalf("expected execution hint in run metadata, got %#v", got)
	}
	if action.metricsCalls != 1 {
		t.Fatalf("expected exactly one planner-selected metrics execution, got %d", action.metricsCalls)
	}
}

func TestExecuteConnectorCapabilityJob_UsesRoleBoundReasoningFinalizer(t *testing.T) {
	t.Parallel()

	connectorManager := newConnectorsManagerWithConfig(t, `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: metrics-main
        name: metrics-main
        display_name: Metrics Main
        vendor: tars
        version: 1.0.0
      spec:
        type: metrics
        protocol: stub
        capabilities:
          - id: metrics.query_instant
            action: query
            read_only: true
`)
	reasoning := &captureAutomationReasoning{
		output: contracts.DiagnosisOutput{
			Summary: "connector reasoning summary",
		},
	}
	manager, err := NewManager(writeConfig(t, minimalYAML), Options{
		Connectors: connectorManager,
		Action:     &fakeAutomationActionService{},
		Reasoning:  reasoning,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	job := Job{
		ID:               "connector-job",
		DisplayName:      "Connector Job",
		GovernancePolicy: "auto",
		ConnectorCapability: &ConnectorCapabilityJob{
			ConnectorID:  "metrics-main",
			CapabilityID: "metrics.query_instant",
			Params:       map[string]interface{}{"query": "up"},
		},
	}
	baseRun := Run{RunID: "run-connector", JobID: job.ID, Status: "running", Metadata: map[string]interface{}{}}
	role := agentrole.AgentRole{
		RoleID: "automation-operator",
		CapabilityBinding: agentrole.CapabilityBinding{
			Mode:                         "whitelist",
			AllowedConnectorCapabilities: []string{"metrics.query_instant"},
		},
		PolicyBinding: agentrole.PolicyBinding{
			MaxRiskLevel: "critical",
			MaxAction:    "direct_execute",
		},
		ModelBinding: agentrole.ModelBinding{
			Primary: &agentrole.ModelTargetBinding{
				ProviderID: "openai-main",
				Model:      "gpt-4.1-mini",
			},
		},
	}

	run, execErr := manager.executeConnectorCapabilityJob(context.Background(), job, baseRun, role)
	if execErr != nil {
		t.Fatalf("executeConnectorCapabilityJob: %v", execErr)
	}
	if run.Status != "completed" {
		t.Fatalf("expected completed run, got %+v", run)
	}
	if run.Summary != "connector reasoning summary" {
		t.Fatalf("expected reasoning summary override, got %+v", run)
	}
	if got := reasoning.lastFinalize.RoleModelBinding; got == nil || got.Primary == nil || got.Primary.Model != "gpt-4.1-mini" {
		t.Fatalf("expected direct model binding in reasoning context, got %+v", reasoning.lastFinalize.RoleModelBinding)
	}
	if got := reasoning.lastFinalize.Context["automation_job_type"]; got != jobTypeConnectorCapability {
		t.Fatalf("expected automation_job_type in reasoning context, got %#v", got)
	}
}

func TestExecuteConnectorCapabilityJob_BlocksNonDenyPolicyOutcomes(t *testing.T) {
	t.Parallel()

	connectorManager := newConnectorsManagerWithConfig(t, `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: metrics-main
        name: metrics-main
        display_name: Metrics Main
        vendor: tars
        version: 1.0.0
      spec:
        type: metrics
        protocol: stub
        capabilities:
          - id: metrics.query_instant
            action: query
            read_only: true
`)
	manager, err := NewManager(writeConfig(t, minimalYAML), Options{
		Connectors: connectorManager,
		Action:     &fakeAutomationActionService{},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	job := Job{
		ID: "connector-job",
		ConnectorCapability: &ConnectorCapabilityJob{
			ConnectorID:  "metrics-main",
			CapabilityID: "metrics.query_instant",
		},
	}
	baseRun := Run{RunID: "run-connector", JobID: job.ID, Status: "running", Metadata: map[string]interface{}{}}

	testCases := []struct {
		name string
		role agentrole.AgentRole
	}{
		{
			name: "require approval policy",
			role: agentrole.AgentRole{
				RoleID: "requires-approval",
				CapabilityBinding: agentrole.CapabilityBinding{
					Mode:                         "whitelist",
					AllowedConnectorCapabilities: []string{"metrics.query_instant"},
				},
				PolicyBinding: agentrole.PolicyBinding{
					MaxRiskLevel: "critical",
					MaxAction:    "require_approval",
				},
			},
		},
		{
			name: "suggest only policy",
			role: agentrole.AgentRole{
				RoleID: "suggest-only",
				CapabilityBinding: agentrole.CapabilityBinding{
					Mode:                         "whitelist",
					AllowedConnectorCapabilities: []string{"metrics.query_instant"},
				},
				PolicyBinding: agentrole.PolicyBinding{
					MaxRiskLevel: "info",
					MaxAction:    "suggest_only",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			action := &fakeAutomationActionService{}
			manager.action = action
			run, execErr := manager.executeConnectorCapabilityJob(context.Background(), job, baseRun, tc.role)
			if execErr != nil {
				t.Fatalf("executeConnectorCapabilityJob: %v", execErr)
			}
			if run.Status != "blocked" {
				t.Fatalf("expected blocked run for %s, got %+v", tc.name, run)
			}
			if action.capabilityCalls != 0 {
				t.Fatalf("expected no connector capability execution for %s, got %d", tc.name, action.capabilityCalls)
			}
		})
	}
}

// buildTestSteps creates a minimal []contracts.ToolPlanStep for testing.
func buildTestSteps(tool string) []contracts.ToolPlanStep {
	return []contracts.ToolPlanStep{
		{Tool: tool, Input: map[string]interface{}{}},
	}
}
