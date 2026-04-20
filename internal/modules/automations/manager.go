package automations

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"

	"tars/internal/contracts"
	"tars/internal/foundation/audit"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/connectors"
	"tars/internal/modules/skills"

	actionmod "tars/internal/modules/action"
	knowledgemod "tars/internal/modules/knowledge"
)

var ErrConfigPathNotSet = errors.New("automations config path is not set")
var ErrJobNotFound = errors.New("automation job not found")

const (
	jobTypeSkill               = "skill"
	jobTypeConnectorCapability = "connector_capability"
	maxRunHistory              = 20
	defaultSchedulerTick       = 5 * time.Second
	defaultRunTimeout          = 30 * time.Second
)

type Manager struct {
	mu          sync.RWMutex
	runSeq      atomic.Uint64
	path        string
	statePath   string
	content     string
	config      *Config
	state       map[string]JobState
	updatedAt   time.Time
	logger      *slog.Logger
	audit       audit.Logger
	action      contracts.ActionService
	knowledge   contracts.KnowledgeService
	reasoning   contracts.ReasoningService
	connectors  *connectors.Manager
	skills      *skills.Manager
	agentRoles  *agentrole.Manager
	runNotifier func(context.Context, Job, Run)
	runningJobs map[string]struct{}
}

type Options struct {
	Logger      *slog.Logger
	Audit       audit.Logger
	Action      contracts.ActionService
	Knowledge   contracts.KnowledgeService
	Reasoning   contracts.ReasoningService
	Connectors  *connectors.Manager
	Skills      *skills.Manager
	AgentRoles  *agentrole.Manager
	RunNotifier func(context.Context, Job, Run)
}

func NewManager(path string, opts Options) (*Manager, error) {
	manager := &Manager{
		path:        strings.TrimSpace(path),
		statePath:   lifecycleStatePath(path),
		logger:      fallbackLogger(opts.Logger),
		audit:       opts.Audit,
		action:      opts.Action,
		knowledge:   opts.Knowledge,
		reasoning:   opts.Reasoning,
		connectors:  opts.Connectors,
		skills:      opts.Skills,
		agentRoles:  opts.AgentRoles,
		runNotifier: opts.RunNotifier,
		runningJobs: map[string]struct{}{},
	}
	if err := manager.Reload(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) Snapshot() Snapshot {
	if m == nil {
		return Snapshot{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := Snapshot{
		Path:      m.path,
		Content:   m.content,
		UpdatedAt: m.updatedAt,
		Loaded:    m.config != nil,
		State:     cloneStateMap(m.state),
	}
	if m.config != nil {
		snapshot.Config = cloneConfig(*m.config)
	}
	return snapshot
}

func (m *Manager) Reload() error {
	if m == nil {
		return nil
	}
	cfg, normalized, err := loadConfigFile(m.path)
	if err != nil {
		return err
	}
	state, err := loadStateFile(m.statePath)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	state = syncState(state, cfg, now)
	m.mu.Lock()
	m.content = normalized
	m.config = cfg
	m.state = state
	m.updatedAt = now
	m.mu.Unlock()
	return nil
}

func (m *Manager) Get(id string) (Job, JobState, bool) {
	if m == nil {
		return Job{}, JobState{}, false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Job{}, JobState{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := getJobLocked(m.config, id)
	if !ok {
		return Job{}, JobState{}, false
	}
	state, ok := m.state[id]
	if !ok {
		state = defaultJobState(job, time.Now().UTC())
	}
	return cloneJob(job), cloneState(state), true
}

func (m *Manager) Upsert(job Job) (Job, JobState, error) {
	if m == nil {
		return Job{}, JobState{}, ErrConfigPathNotSet
	}
	normalized, err := normalizeJob(job)
	if err != nil {
		return Job{}, JobState{}, err
	}
	m.mu.RLock()
	current := Config{}
	if m.config != nil {
		current = cloneConfig(*m.config)
	}
	state := cloneStateMap(m.state)
	m.mu.RUnlock()
	replaced := false
	for i := range current.Jobs {
		if current.Jobs[i].ID == normalized.ID {
			current.Jobs[i] = normalized
			replaced = true
			break
		}
	}
	if !replaced {
		current.Jobs = append(current.Jobs, normalized)
	}
	if err := m.saveConfigAndState(&current, state); err != nil {
		return Job{}, JobState{}, err
	}
	updated, updatedState, ok := m.Get(normalized.ID)
	if !ok {
		return Job{}, JobState{}, ErrJobNotFound
	}
	return updated, updatedState, nil
}

func (m *Manager) SetEnabled(id string, enabled bool) (Job, JobState, error) {
	m.mu.RLock()
	current := Config{}
	if m.config != nil {
		current = cloneConfig(*m.config)
	}
	state := cloneStateMap(m.state)
	m.mu.RUnlock()
	id = strings.TrimSpace(id)
	found := false
	for i := range current.Jobs {
		if current.Jobs[i].ID != id {
			continue
		}
		current.Jobs[i].Enabled = enabled
		found = true
		break
	}
	if !found {
		return Job{}, JobState{}, ErrJobNotFound
	}
	if err := m.saveConfigAndState(&current, state); err != nil {
		return Job{}, JobState{}, err
	}
	updated, updatedState, ok := m.Get(id)
	if !ok {
		return Job{}, JobState{}, ErrJobNotFound
	}
	return updated, updatedState, nil
}

func (m *Manager) RunNow(ctx context.Context, id string, req RunRequest) (Run, error) {
	job, _, ok := m.Get(id)
	if !ok {
		return Run{}, ErrJobNotFound
	}
	return m.runJob(ctx, job, firstNonEmpty(strings.TrimSpace(req.Trigger), "manual"), strings.TrimSpace(req.TriggeredBy))
}

func (m *Manager) RunDue(ctx context.Context, now time.Time) {
	snapshot := m.Snapshot()
	for _, job := range snapshot.Config.Jobs {
		state := snapshot.State[job.ID]
		if !job.Enabled || state.NextRunAt.IsZero() || state.NextRunAt.After(now) {
			continue
		}
		jobCopy := job
		go func() {
			_, err := m.runJob(ctx, jobCopy, "schedule", "scheduler")
			if err != nil {
				m.logger.Warn("automation scheduled run failed", "job_id", jobCopy.ID, "error", err)
			}
		}()
	}
}

func (m *Manager) StartScheduler(ctx context.Context) {
	if m == nil {
		return
	}
	ticker := time.NewTicker(defaultSchedulerTick)
	defer ticker.Stop()
	m.logger.Info("automation scheduler started")
	for {
		m.RunDue(ctx, time.Now().UTC())
		select {
		case <-ctx.Done():
			m.logger.Info("automation scheduler stopped")
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) saveConfigAndState(cfg *Config, previousState map[string]JobState) error {
	if strings.TrimSpace(m.path) == "" {
		return ErrConfigPathNotSet
	}
	content, err := encodeConfig(cfg)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	state := syncState(previousState, cfg, now)
	if err := writeFileAtomically(m.path, content, ".automations-"); err != nil {
		return err
	}
	if err := saveStateFile(m.statePath, state); err != nil {
		return err
	}
	m.mu.Lock()
	m.content = content
	m.config = cfg
	m.state = state
	m.updatedAt = now
	m.mu.Unlock()
	return nil
}

func (m *Manager) runJob(ctx context.Context, job Job, trigger string, actor string) (Run, error) {
	if m == nil {
		return Run{}, ErrJobNotFound
	}
	jobID := strings.TrimSpace(job.ID)
	if jobID == "" {
		return Run{}, ErrJobNotFound
	}
	if !m.tryStartRun(jobID) {
		return Run{}, fmt.Errorf("automation job %s is already running", jobID)
	}
	defer m.finishRun(jobID)

	startedAt := time.Now().UTC()
	run := Run{
		RunID:     m.nextRunID(),
		JobID:     jobID,
		Trigger:   trigger,
		Status:    "running",
		StartedAt: startedAt,
		Metadata: map[string]interface{}{
			"triggered_by": actor,
		},
	}

	// Resolve the agent role for this job and record it in run metadata.
	resolvedRole := m.resolveJobRole(job)
	run.Metadata["resolved_agent_role_id"] = resolvedRole.RoleID
	run.Metadata["resolved_agent_role_display_name"] = resolvedRole.DisplayName
	// Record the resolved role model binding for auditability and downstream runtime use.
	if resolvedRole.ModelBinding.Primary != nil {
		run.Metadata["role_model_binding_primary_provider_id"] = resolvedRole.ModelBinding.Primary.ProviderID
		run.Metadata["role_model_binding_primary_model"] = resolvedRole.ModelBinding.Primary.Model
	}
	if resolvedRole.ModelBinding.Fallback != nil {
		run.Metadata["role_model_binding_fallback_provider_id"] = resolvedRole.ModelBinding.Fallback.ProviderID
		run.Metadata["role_model_binding_fallback_model"] = resolvedRole.ModelBinding.Fallback.Model
	}
	run.Metadata["role_model_binding_inherit_platform_default"] = resolvedRole.ModelBinding.InheritPlatformDefault

	m.markRunStarted(job, run)
	m.auditRun(context.Background(), job, run, "automation_run_started", nil)

	timeout := durationOrDefault(job.TimeoutSeconds, defaultRunTimeout)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	finalRun := run
	var execErr error
	attempts := normalizedAttempts(job.RetryMaxAttempts)
	policy := backoff.NewExponentialBackOff()
	policy.InitialInterval = retryInitialInterval(job.RetryInitialBackoff)
	policy.MaxInterval = 30 * time.Second
	policy.MaxElapsedTime = 0
	for attempt := 1; attempt <= attempts; attempt++ {
		finalRun.AttemptCount = attempt
		finalRun, execErr = m.executeOnce(runCtx, job, finalRun, resolvedRole)
		if execErr == nil || !shouldRetry(finalRun.Status) || attempt == attempts {
			break
		}
		wait := policy.NextBackOff()
		if wait == backoff.Stop {
			break
		}
		select {
		case <-runCtx.Done():
			execErr = runCtx.Err()
			finalRun.Status = "failed"
			finalRun.Error = runCtx.Err().Error()
		case <-time.After(wait):
		}
	}
	if finalRun.CompletedAt.IsZero() {
		finalRun.CompletedAt = time.Now().UTC()
	}
	if finalRun.Status == "running" {
		finalRun.Status = "failed"
	}
	m.markRunFinished(job, finalRun)
	m.auditRun(context.Background(), job, finalRun, "automation_run_"+finalRun.Status, map[string]any{"attempt_count": finalRun.AttemptCount})
	if m.runNotifier != nil {
		m.runNotifier(context.Background(), cloneJob(job), cloneRun(finalRun))
	}
	return finalRun, execErr
}

func (m *Manager) executeOnce(ctx context.Context, job Job, run Run, role agentrole.AgentRole) (Run, error) {
	if blocked, summary := governancePolicyBlock(job); blocked {
		run.Status = "blocked"
		run.CompletedAt = time.Now().UTC()
		run.Summary = summary
		run = m.finalizeAutomationRun(ctx, job, run, role, map[string]interface{}{
			"automation_governance_blocked": true,
			"automation_governance_reason":  summary,
		})
		return run, nil
	}
	switch strings.ToLower(strings.TrimSpace(job.Type)) {
	case jobTypeSkill:
		return m.executeSkillJob(ctx, job, run, role)
	case jobTypeConnectorCapability:
		return m.executeConnectorCapabilityJob(ctx, job, run, role)
	default:
		run.Status = "failed"
		run.CompletedAt = time.Now().UTC()
		run.Error = fmt.Sprintf("unsupported automation type %q", job.Type)
		return run, errors.New(run.Error)
	}
}

func (m *Manager) executeSkillJob(ctx context.Context, job Job, run Run, role agentrole.AgentRole) (Run, error) {
	if m.skills == nil {
		run.Status = "failed"
		run.CompletedAt = time.Now().UTC()
		run.Error = "skills manager is not configured"
		return run, errors.New(run.Error)
	}
	target := job.Skill
	if target == nil {
		run.Status = "failed"
		run.CompletedAt = time.Now().UTC()
		run.Error = "skill target is required"
		return run, errors.New(run.Error)
	}
	manifest, ok := m.skills.Get(target.SkillID)
	if !ok {
		run.Status = "failed"
		run.CompletedAt = time.Now().UTC()
		run.Error = "skill not found"
		return run, errors.New(run.Error)
	}
	state, ok := m.skills.GetLifecycle(target.SkillID)
	if !ok || !state.Enabled || strings.EqualFold(state.Status, "disabled") || strings.EqualFold(state.RuntimeMode, "disabled") {
		run.Status = "blocked"
		run.CompletedAt = time.Now().UTC()
		run.Summary = "skill is not enabled for automation"
		return run, nil
	}
	if !isSkillAllowedByRole(role, manifest) {
		run.Status = "blocked"
		run.CompletedAt = time.Now().UTC()
		run.Summary = fmt.Sprintf("skill %q is not permitted by agent role %q", target.SkillID, role.RoleID)
		return run, nil
	}
	match := contracts.SkillMatch{
		SkillID:     manifest.Metadata.ID,
		DisplayName: firstNonEmpty(manifest.Metadata.DisplayName, manifest.Metadata.Name, manifest.Metadata.ID),
		Summary:     manifest.Metadata.Description,
		ReviewState: state.ReviewState,
		RuntimeMode: state.RuntimeMode,
		Source:      firstNonEmpty(state.Source, manifest.Metadata.Source, manifest.Marketplace.Source),
		Manifest: map[string]interface{}{
			"skill_id": manifest.Metadata.ID,
			"content":  manifest.Metadata.Content,
		},
	}
	capabilities := automationToolCapabilities(m.connectors)
	staticSteps := m.skills.Expand(match, contracts.DiagnosisInput{Context: cloneInterfaceMap(target.Context)}, capabilities)
	steps, plannerMetadata := m.planAutomationSkill(ctx, job, run, target, manifest, role, staticSteps, capabilities)
	result, execErr := m.executeSkillSteps(ctx, steps, cloneInterfaceMap(target.Context), role)
	runMetadata := map[string]interface{}{
		"skill_id":      manifest.Metadata.ID,
		"skill_tags":    append([]string(nil), manifest.Metadata.Tags...),
		"step_count":    len(steps),
		"executed":      result.ExecutedSteps,
		"blocked_tool":  result.BlockedTool,
		"tool_plan":     steps,
		"planner_state": plannerMetadata["automation_planner_source"],
	}
	for key, value := range plannerMetadata {
		runMetadata[key] = value
	}
	run.CompletedAt = time.Now().UTC()
	run.Metadata = mergeMetadata(run.Metadata, runMetadata)
	if execErr != nil {
		run.Status = "failed"
		run.Error = execErr.Error()
		run.Summary = "skill automation failed"
		run = m.finalizeAutomationRun(ctx, job, run, role, runMetadata)
		return run, execErr
	}
	if result.PendingApproval {
		run.Status = "blocked"
		run.Summary = firstNonEmpty(result.Summary, "skill automation requires approval and was not auto-executed")
		run = m.finalizeAutomationRun(ctx, job, run, role, mergeMetadata(runMetadata, map[string]interface{}{
			"automation_pending_approval": true,
		}))
		return run, nil
	}
	if result.Blocked {
		run.Status = "blocked"
		run.Summary = firstNonEmpty(result.Summary, "skill automation hit a guarded step and stopped")
		run = m.finalizeAutomationRun(ctx, job, run, role, runMetadata)
		return run, nil
	}
	run.Status = "completed"
	run.Summary = firstNonEmpty(result.Summary, fmt.Sprintf("skill automation completed with %d step(s)", result.ExecutedSteps))
	run = m.finalizeAutomationRun(ctx, job, run, role, runMetadata)
	return run, nil
}

func (m *Manager) planAutomationSkill(
	ctx context.Context,
	job Job,
	run Run,
	target *SkillTarget,
	manifest skills.Manifest,
	role agentrole.AgentRole,
	staticSteps []contracts.ToolPlanStep,
	capabilities []contracts.ToolCapabilityDescriptor,
) ([]contracts.ToolPlanStep, map[string]interface{}) {
	metadata := map[string]interface{}{
		"automation_planner_source": "skill_manifest",
	}
	if len(staticSteps) == 0 {
		metadata["automation_planner_reason"] = "skill manifest did not define planner steps"
		return staticSteps, metadata
	}
	if m == nil || m.reasoning == nil {
		metadata["automation_planner_reason"] = "reasoning service is not configured"
		return staticSteps, metadata
	}

	plannerContext := cloneInterfaceMap(target.Context)
	if plannerContext == nil {
		plannerContext = map[string]interface{}{}
	}
	plannerContext["summary"] = firstNonEmpty(strings.TrimSpace(job.Description), strings.TrimSpace(manifest.Metadata.Description), fmt.Sprintf("automation job %s", job.ID))
	plannerContext["user_request"] = fmt.Sprintf("Run automation job %s using skill %s and produce a safe executable tool plan.", firstNonEmpty(job.DisplayName, job.ID), manifest.Metadata.ID)
	plannerContext["automation_run"] = true
	plannerContext["automation_job_id"] = strings.TrimSpace(job.ID)
	plannerContext["automation_display_name"] = firstNonEmpty(strings.TrimSpace(job.DisplayName), strings.TrimSpace(job.ID))
	plannerContext["automation_job_type"] = jobTypeSkill
	plannerContext["automation_target_ref"] = firstNonEmpty(strings.TrimSpace(job.TargetRef), strings.TrimSpace(target.SkillID))
	plannerContext["automation_governance_policy"] = firstNonEmpty(strings.TrimSpace(job.GovernancePolicy), "auto")
	plannerContext["skill_id"] = strings.TrimSpace(manifest.Metadata.ID)
	plannerContext["skill_display_name"] = firstNonEmpty(strings.TrimSpace(manifest.Metadata.DisplayName), strings.TrimSpace(manifest.Metadata.Name), strings.TrimSpace(manifest.Metadata.ID))
	plannerContext["skill_tags"] = append([]string(nil), manifest.Metadata.Tags...)
	plannerContext["skill_manifest_steps"] = staticSteps
	plannerContext["tool_capabilities"] = capabilities
	plannerContext["tool_capabilities_summary"] = automationToolCapabilitySummary(capabilities)
	if strings.TrimSpace(role.Profile.SystemPrompt) != "" {
		plannerContext["agent_role_system_prompt"] = strings.TrimSpace(role.Profile.SystemPrompt)
	}

	plan, err := m.reasoning.PlanDiagnosis(ctx, contracts.DiagnosisInput{
		SessionID:        fmt.Sprintf("automation/%s/%s", job.ID, run.RunID),
		Context:          plannerContext,
		RoleModelBinding: roleModelBinding(role),
	})
	if err != nil {
		m.logger.Warn("automation skill planner failed, falling back to manifest steps", "job_id", job.ID, "skill_id", manifest.Metadata.ID, "error", err)
		metadata["automation_planner_reason"] = err.Error()
		return staticSteps, metadata
	}
	plannedSteps := filterAutomationSkillPlan(plan.ToolPlan, staticSteps)
	if len(plannedSteps) == 0 {
		metadata["automation_planner_reason"] = "planner returned no executable manifest-compatible steps"
		return staticSteps, metadata
	}
	metadata["automation_planner_source"] = "reasoning"
	metadata["automation_planner_summary"] = strings.TrimSpace(plan.Summary)
	metadata["automation_static_step_count"] = len(staticSteps)
	return plannedSteps, metadata
}

func (m *Manager) executeConnectorCapabilityJob(ctx context.Context, job Job, run Run, role agentrole.AgentRole) (Run, error) {
	target := job.ConnectorCapability
	if target == nil {
		run.Status = "failed"
		run.CompletedAt = time.Now().UTC()
		run.Error = "connector capability target is required"
		return run, errors.New(run.Error)
	}
	descriptor, ok := m.findCapabilityDescriptor(target.ConnectorID, target.CapabilityID, "connector.invoke_capability")
	if !ok {
		run.Status = "failed"
		run.CompletedAt = time.Now().UTC()
		run.Error = "connector capability not found"
		return run, errors.New(run.Error)
	}
	// Enforce role capability binding against the connector capability.
	if decision := agentrole.EnforceCapabilityBinding(role, "direct_execute", descriptor.CapabilityID); decision == "deny" {
		run.Status = "blocked"
		run.CompletedAt = time.Now().UTC()
		run.Summary = fmt.Sprintf("connector capability %q is denied by agent role %q", descriptor.CapabilityID, role.RoleID)
		return run, nil
	}
	// Enforce role policy binding.
	switch decision := agentrole.EnforcePolicy(role, "direct_execute", descriptor.CapabilityID); decision {
	case "deny":
		run.Status = "blocked"
		run.CompletedAt = time.Now().UTC()
		run.Summary = fmt.Sprintf("connector capability %q is hard-denied by policy in agent role %q", descriptor.CapabilityID, role.RoleID)
		return run, nil
	case "require_approval":
		run.Status = "blocked"
		run.CompletedAt = time.Now().UTC()
		run.Summary = fmt.Sprintf("connector capability %q requires approval per agent role %q policy", descriptor.CapabilityID, role.RoleID)
		return run, nil
	case "suggest_only":
		run.Status = "blocked"
		run.CompletedAt = time.Now().UTC()
		run.Summary = fmt.Sprintf("connector capability %q is suggest-only under agent role %q and cannot be auto-executed", descriptor.CapabilityID, role.RoleID)
		return run, nil
	}
	if !descriptor.ReadOnly {
		run.Status = "blocked"
		run.CompletedAt = time.Now().UTC()
		run.Summary = "non-read-only connector capability cannot be auto-executed"
		return run, nil
	}
	capResult, err := m.invokeCapability(ctx, descriptor.ConnectorID, descriptor.CapabilityID, cloneInterfaceMap(target.Params))
	run.CompletedAt = time.Now().UTC()
	run.Metadata = mergeMetadata(run.Metadata, map[string]interface{}{
		"connector_id":  descriptor.ConnectorID,
		"capability_id": descriptor.CapabilityID,
	})
	if err != nil {
		run.Status = "failed"
		run.Error = err.Error()
		run.Summary = "connector capability automation failed"
		run = m.finalizeAutomationRun(ctx, job, run, role, map[string]interface{}{
			"connector_id":         descriptor.ConnectorID,
			"connector_capability": descriptor.CapabilityID,
			"connector_params":     cloneInterfaceMap(target.Params),
		})
		return run, err
	}
	if capResult.Status == "pending_approval" {
		run.Status = "blocked"
		run.Summary = "connector capability requires approval and was not auto-executed"
		run = m.finalizeAutomationRun(ctx, job, run, role, map[string]interface{}{
			"connector_id":                descriptor.ConnectorID,
			"connector_capability":        descriptor.CapabilityID,
			"connector_params":            cloneInterfaceMap(target.Params),
			"automation_pending_approval": true,
		})
		return run, nil
	}
	run.Status = "completed"
	run.Summary = firstNonEmpty(stringMapValue(capResult.Output, "summary"), "connector capability completed")
	run = m.finalizeAutomationRun(ctx, job, run, role, map[string]interface{}{
		"connector_id":         descriptor.ConnectorID,
		"connector_capability": descriptor.CapabilityID,
		"connector_params":     cloneInterfaceMap(target.Params),
		"capability_output":    cloneInterfaceMap(capResult.Output),
	})
	return run, nil
}

type skillExecutionResult struct {
	ExecutedSteps   int
	Blocked         bool
	PendingApproval bool
	BlockedTool     string
	Summary         string
}

func (m *Manager) executeSkillSteps(ctx context.Context, steps []contracts.ToolPlanStep, inputContext map[string]interface{}, role agentrole.AgentRole) (skillExecutionResult, error) {
	result := skillExecutionResult{}
	contextData := cloneInterfaceMap(inputContext)
	for _, step := range steps {
		result.ExecutedSteps++
		resolved := cloneInterfaceMap(step.Input)
		if resolved == nil {
			resolved = map[string]interface{}{}
		}
		for key, value := range contextData {
			if _, exists := resolved[key]; !exists {
				resolved[key] = value
			}
		}
		// Enforce role capability binding and policy for this step's tool/capability.
		capabilityRef := firstNonEmpty(interfaceString(resolved["capability_id"]), step.Tool)
		if decision := agentrole.EnforceCapabilityBinding(role, "direct_execute", capabilityRef); decision == "deny" {
			result.Blocked = true
			result.BlockedTool = step.Tool
			result.Summary = fmt.Sprintf("tool %s is denied by agent role %q capability binding", step.Tool, role.RoleID)
			return result, nil
		}
		switch decision := agentrole.EnforcePolicy(role, "direct_execute", step.Tool); decision {
		case "deny":
			result.Blocked = true
			result.BlockedTool = step.Tool
			result.Summary = fmt.Sprintf("tool %s is hard-denied by policy in agent role %q", step.Tool, role.RoleID)
			return result, nil
		case "require_approval":
			result.PendingApproval = true
			result.BlockedTool = step.Tool
			result.Summary = fmt.Sprintf("tool %s requires approval per agent role %q policy", step.Tool, role.RoleID)
			return result, nil
		case "suggest_only":
			result.Blocked = true
			result.BlockedTool = step.Tool
			result.Summary = fmt.Sprintf("tool %s is suggest-only under agent role %q and cannot be auto-executed", step.Tool, role.RoleID)
			return result, nil
		}
		switch strings.TrimSpace(step.Tool) {
		case "metrics.query_range", "metrics.query_instant":
			if m.action == nil {
				return result, errors.New("action service is not configured")
			}
			_, err := m.action.QueryMetrics(ctx, contracts.MetricsQuery{
				Service:     interfaceString(resolved["service"]),
				Host:        interfaceString(resolved["host"]),
				Query:       interfaceString(resolved["query"]),
				Mode:        defaultMetricsMode(step.Tool),
				Step:        firstNonEmpty(interfaceString(resolved["step"]), "5m"),
				Window:      firstNonEmpty(interfaceString(resolved["window"]), "1h"),
				ConnectorID: firstNonEmpty(step.ConnectorID, interfaceString(resolved["connector_id"]), m.resolveConnectorIDByTool(step.Tool)),
			})
			if err != nil {
				return result, err
			}
		case "knowledge.search":
			if m.knowledge == nil {
				return result, errors.New("knowledge service is not configured")
			}
			_, err := m.knowledge.Search(ctx, contracts.KnowledgeQuery{Query: firstNonEmpty(interfaceString(resolved["query"]), interfaceString(resolved["summary"]), interfaceString(resolved["user_request"]))})
			if err != nil {
				return result, err
			}
		case "observability.query", "delivery.query", "connector.invoke_capability":
			descriptor, ok := m.selectDescriptorForTool(step.Tool, step.ConnectorID, resolved)
			if !ok {
				return result, fmt.Errorf("no invocable connector capability for tool %s", step.Tool)
			}
			if !descriptor.ReadOnly {
				result.Blocked = true
				result.BlockedTool = step.Tool
				result.Summary = fmt.Sprintf("tool %s resolved to non-read-only capability %s", step.Tool, descriptor.CapabilityID)
				return result, nil
			}
			capResult, err := m.invokeCapability(ctx, descriptor.ConnectorID, descriptor.CapabilityID, resolved)
			if err != nil {
				return result, err
			}
			if capResult.Status == "pending_approval" {
				result.PendingApproval = true
				result.BlockedTool = step.Tool
				result.Summary = fmt.Sprintf("tool %s requires approval and was not auto-executed", step.Tool)
				return result, nil
			}
		case "execution.run_command":
			result.Blocked = true
			result.BlockedTool = step.Tool
			result.Summary = "execution.run_command remains guarded and is not auto-executed by automations"
			return result, nil
		default:
			result.Blocked = true
			result.BlockedTool = step.Tool
			result.Summary = fmt.Sprintf("tool %s is not supported by automation mvp", step.Tool)
			return result, nil
		}
	}
	return result, nil
}

func (m *Manager) invokeCapability(ctx context.Context, connectorID string, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	if m.action == nil {
		return contracts.CapabilityResult{}, errors.New("action service is not configured")
	}
	return m.action.InvokeCapability(ctx, contracts.CapabilityRequest{
		ConnectorID:  connectorID,
		CapabilityID: capabilityID,
		Params:       cloneInterfaceMap(params),
		Caller:       "automation_job",
		SessionID:    "",
	})
}

func filterAutomationSkillPlan(planned []contracts.ToolPlanStep, static []contracts.ToolPlanStep) []contracts.ToolPlanStep {
	if len(planned) == 0 {
		return nil
	}
	allowedTools := map[string]struct{}{}
	for _, step := range static {
		tool := strings.TrimSpace(step.Tool)
		if tool == "" {
			continue
		}
		allowedTools[tool] = struct{}{}
	}
	if len(allowedTools) == 0 {
		return append([]contracts.ToolPlanStep(nil), planned...)
	}
	filtered := make([]contracts.ToolPlanStep, 0, len(planned))
	for _, step := range planned {
		if _, ok := allowedTools[strings.TrimSpace(step.Tool)]; !ok {
			continue
		}
		filtered = append(filtered, step)
	}
	return filtered
}

func (m *Manager) resolveConnectorIDByTool(tool string) string {
	for _, item := range connectors.ToolPlanCapabilities(m.connectors) {
		if item.Tool == tool && item.Invocable {
			return item.ConnectorID
		}
	}
	return ""
}

func (m *Manager) selectDescriptorForTool(tool string, connectorID string, params map[string]interface{}) (connectors.ToolPlanCapability, bool) {
	capabilityID := interfaceString(params["capability_id"])
	if strings.TrimSpace(tool) == "connector.invoke_capability" {
		return m.findCapabilityDescriptor(firstNonEmpty(connectorID, interfaceString(params["connector_id"])), capabilityID, tool)
	}
	wantedConnector := firstNonEmpty(connectorID, interfaceString(params["connector_id"]))
	for _, item := range connectors.ToolPlanCapabilities(m.connectors) {
		if item.Tool != tool || !item.Invocable {
			continue
		}
		if wantedConnector != "" && item.ConnectorID != wantedConnector {
			continue
		}
		return item, true
	}
	return connectors.ToolPlanCapability{}, false
}

func (m *Manager) findCapabilityDescriptor(connectorID string, capabilityID string, tool string) (connectors.ToolPlanCapability, bool) {
	connectorID = strings.TrimSpace(connectorID)
	capabilityID = strings.TrimSpace(capabilityID)
	for _, item := range connectors.ToolPlanCapabilities(m.connectors) {
		if !item.Invocable {
			continue
		}
		if connectorID != "" && item.ConnectorID != connectorID {
			continue
		}
		if capabilityID != "" && item.CapabilityID != capabilityID {
			continue
		}
		if tool != "" && item.Tool != tool && tool != "connector.invoke_capability" {
			continue
		}
		return item, true
	}
	return connectors.ToolPlanCapability{}, false
}

func (m *Manager) tryStartRun(jobID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.runningJobs[jobID]; ok {
		return false
	}
	m.runningJobs[jobID] = struct{}{}
	return true
}

func (m *Manager) finishRun(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runningJobs, jobID)
}

func (m *Manager) markRunStarted(job Job, run Run) {
	m.updateState(job.ID, func(state *JobState) {
		state.Status = "running"
		state.LastRunAt = run.StartedAt
		state.LastOutcome = "running"
		state.LastError = ""
		state.UpdatedAt = run.StartedAt
		state.NextRunAt = nextScheduleAfter(job.Schedule, run.StartedAt)
	})
}

func (m *Manager) markRunFinished(job Job, run Run) {
	m.updateState(job.ID, func(state *JobState) {
		state.Status = run.Status
		state.LastRunAt = run.StartedAt
		state.LastOutcome = run.Status
		state.LastError = strings.TrimSpace(run.Error)
		state.UpdatedAt = run.CompletedAt
		if run.Status == "completed" {
			state.ConsecutiveFailures = 0
		} else if run.Status == "failed" {
			state.ConsecutiveFailures++
		}
		state.Runs = append([]Run{cloneRun(run)}, state.Runs...)
		if len(state.Runs) > maxRunHistory {
			state.Runs = state.Runs[:maxRunHistory]
		}
		state.NextRunAt = nextScheduleAfter(job.Schedule, run.StartedAt)
	})
}

func (m *Manager) updateState(jobID string, mutate func(*JobState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		m.state = map[string]JobState{}
	}
	state, ok := m.state[jobID]
	if !ok {
		state = JobState{JobID: jobID}
	}
	mutate(&state)
	m.state[jobID] = state
	if strings.TrimSpace(m.statePath) != "" {
		_ = saveStateFile(m.statePath, m.state)
	}
	if !m.updatedAt.After(state.UpdatedAt) {
		m.updatedAt = state.UpdatedAt
	}
}

func isSkillAllowedByRole(role agentrole.AgentRole, manifest skills.Manifest) bool {
	allowedSkills := role.CapabilityBinding.AllowedSkills
	allowedTags := role.CapabilityBinding.AllowedSkillTags
	if len(allowedSkills) == 0 && len(allowedTags) == 0 {
		return true
	}
	if containsFold(allowedSkills, manifest.Metadata.ID) {
		return true
	}
	for _, tag := range manifest.Metadata.Tags {
		if containsFold(allowedTags, tag) {
			return true
		}
	}
	return false
}

func governancePolicyBlock(job Job) (bool, string) {
	switch strings.ToLower(strings.TrimSpace(job.GovernancePolicy)) {
	case "", "auto":
		return false, ""
	case "approval_required":
		return true, "automation governance policy requires approval before execution"
	case "disabled":
		return true, "automation governance policy is disabled"
	default:
		return false, ""
	}
}

func containsFold(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func roleModelBinding(role agentrole.AgentRole) *contracts.RoleModelBinding {
	primary := roleModelTargetBinding(role.ModelBinding.Primary)
	fallback := roleModelTargetBinding(role.ModelBinding.Fallback)
	if primary == nil && fallback == nil && !role.ModelBinding.InheritPlatformDefault {
		return nil
	}
	return &contracts.RoleModelBinding{
		Primary:                primary,
		Fallback:               fallback,
		InheritPlatformDefault: role.ModelBinding.InheritPlatformDefault,
	}
}

func roleModelTargetBinding(binding *agentrole.ModelTargetBinding) *contracts.RoleModelTargetBinding {
	if binding == nil {
		return nil
	}
	target := &contracts.RoleModelTargetBinding{
		ProviderID: strings.TrimSpace(binding.ProviderID),
		Model:      strings.TrimSpace(binding.Model),
	}
	if target.ProviderID == "" && target.Model == "" {
		return nil
	}
	return target
}

func (m *Manager) finalizeAutomationRun(ctx context.Context, job Job, run Run, role agentrole.AgentRole, extra map[string]interface{}) Run {
	if m == nil || m.reasoning == nil {
		return run
	}
	contextData := map[string]interface{}{
		"automation_run":                   true,
		"automation_job_id":                job.ID,
		"automation_display_name":          job.DisplayName,
		"automation_job_type":              firstNonEmpty(strings.TrimSpace(job.Type), inferAutomationJobType(job)),
		"automation_target_ref":            job.TargetRef,
		"automation_governance_policy":     firstNonEmpty(strings.TrimSpace(job.GovernancePolicy), "auto"),
		"resolved_agent_role_id":           role.RoleID,
		"resolved_agent_role_display_name": role.DisplayName,
		"run_status":                       run.Status,
		"run_summary":                      run.Summary,
		"run_error":                        run.Error,
		"run_metadata":                     cloneInterfaceMap(run.Metadata),
		"user_request":                     "Summarize this automation run and only produce execution_hint if a human still needs to take an action.",
	}
	if strings.TrimSpace(role.Profile.SystemPrompt) != "" {
		contextData["agent_role_system_prompt"] = strings.TrimSpace(role.Profile.SystemPrompt)
	}
	for key, value := range extra {
		contextData[key] = value
	}
	output, err := m.reasoning.FinalizeDiagnosis(ctx, contracts.DiagnosisInput{
		SessionID:        firstNonEmpty(strings.TrimSpace(run.RunID), strings.TrimSpace(job.ID)),
		Context:          contextData,
		RoleModelBinding: roleModelBinding(role),
	})
	if err != nil {
		m.logger.Warn("automation reasoning finalizer failed", "job_id", job.ID, "run_id", run.RunID, "error", err)
		return run
	}
	if strings.TrimSpace(output.Summary) != "" {
		run.Summary = strings.TrimSpace(output.Summary)
	}
	if strings.TrimSpace(output.ExecutionHint) != "" {
		if run.Metadata == nil {
			run.Metadata = map[string]interface{}{}
		}
		run.Metadata["automation_execution_hint"] = strings.TrimSpace(output.ExecutionHint)
	}
	return run
}

func inferAutomationJobType(job Job) string {
	if job.Skill != nil {
		return jobTypeSkill
	}
	if job.ConnectorCapability != nil {
		return jobTypeConnectorCapability
	}
	return ""
}

// resolveJobRole returns the agent role to use for this job.
// Precedence: job.AgentRoleID → automation_operator builtin.
func (m *Manager) resolveJobRole(job Job) agentrole.AgentRole {
	if m.agentRoles != nil {
		return m.agentRoles.ResolveForAutomation(strings.TrimSpace(job.AgentRoleID))
	}
	// No agentRoles manager configured: synthesize the builtin automation_operator role inline.
	return agentrole.AgentRole{
		RoleID:            "automation_operator",
		DisplayName:       "Automation Operator",
		Status:            "active",
		CapabilityBinding: agentrole.CapabilityBinding{Mode: "unrestricted"},
		PolicyBinding: agentrole.PolicyBinding{
			MaxRiskLevel: "critical",
			MaxAction:    "direct_execute",
		},
	}
}

func (m *Manager) auditRun(ctx context.Context, job Job, run Run, action string, extra map[string]any) {
	if m == nil || m.audit == nil {
		return
	}
	metadata := map[string]any{
		"job_id":        job.ID,
		"job_type":      job.Type,
		"target_ref":    job.TargetRef,
		"run_id":        run.RunID,
		"trigger":       run.Trigger,
		"status":        run.Status,
		"summary":       run.Summary,
		"error":         run.Error,
		"attempt_count": run.AttemptCount,
	}
	for key, value := range run.Metadata {
		metadata[key] = value
	}
	for key, value := range extra {
		metadata[key] = value
	}
	m.audit.Log(ctx, audit.Entry{
		ResourceType: "automation_run",
		ResourceID:   run.RunID,
		Action:       action,
		Actor:        "automation_scheduler",
		Metadata:     metadata,
	})
}

func loadConfigFile(path string) (*Config, string, error) {
	if strings.TrimSpace(path) == "" {
		cfg := Config{}
		content, _ := encodeConfig(&cfg)
		return &cfg, content, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := Config{}
			encoded, _ := encodeConfig(&cfg)
			return &cfg, encoded, nil
		}
		return nil, "", err
	}
	var raw fileConfig
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, "", err
	}
	for i := range raw.Automations.Jobs {
		normalized, err := normalizeJob(raw.Automations.Jobs[i])
		if err != nil {
			return nil, "", err
		}
		raw.Automations.Jobs[i] = normalized
	}
	encoded, err := encodeConfig(&raw.Automations)
	if err != nil {
		return nil, "", err
	}
	return &raw.Automations, encoded, nil
}

func encodeConfig(cfg *Config) (string, error) {
	raw := fileConfig{}
	raw.Automations = cloneConfig(derefConfig(cfg))
	content, err := yaml.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func loadStateFile(path string) (map[string]JobState, error) {
	if strings.TrimSpace(path) == "" {
		return map[string]JobState{}, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]JobState{}, nil
		}
		return nil, err
	}
	var raw stateFile
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]JobState, len(raw.Automations.Jobs))
	for _, item := range raw.Automations.Jobs {
		if strings.TrimSpace(item.JobID) == "" {
			continue
		}
		out[item.JobID] = cloneState(item)
	}
	return out, nil
}

func saveStateFile(path string, state map[string]JobState) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	var raw stateFile
	ids := make([]string, 0, len(state))
	for id := range state {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		raw.Automations.Jobs = append(raw.Automations.Jobs, cloneState(state[id]))
	}
	content, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return writeFileAtomically(path, string(content), ".automations-state-")
}

func syncState(existing map[string]JobState, cfg *Config, now time.Time) map[string]JobState {
	state := cloneStateMap(existing)
	jobs := derefConfig(cfg).Jobs
	active := map[string]struct{}{}
	for _, job := range jobs {
		active[job.ID] = struct{}{}
		current, ok := state[job.ID]
		if !ok {
			current = defaultJobState(job, now)
		}
		current.JobID = job.ID
		current.Status = statusForJob(job)
		current.UpdatedAt = now
		current.NextRunAt = nextScheduleAfter(job.Schedule, now)
		if !job.Enabled {
			current.NextRunAt = time.Time{}
		}
		state[job.ID] = current
	}
	for id := range state {
		if _, ok := active[id]; !ok {
			delete(state, id)
		}
	}
	return state
}

func normalizeJob(job Job) (Job, error) {
	job.ID = strings.TrimSpace(job.ID)
	job.DisplayName = strings.TrimSpace(job.DisplayName)
	job.Type = strings.ToLower(strings.TrimSpace(job.Type))
	job.TargetRef = strings.TrimSpace(job.TargetRef)
	job.Schedule = strings.TrimSpace(job.Schedule)
	job.Owner = strings.TrimSpace(job.Owner)
	job.GovernancePolicy = firstNonEmpty(strings.ToLower(strings.TrimSpace(job.GovernancePolicy)), "auto")
	job.RuntimeMode = firstNonEmpty(strings.TrimSpace(job.RuntimeMode), "managed")
	if job.ID == "" {
		return Job{}, errors.New("id is required")
	}
	if job.DisplayName == "" {
		job.DisplayName = job.ID
	}
	if job.Schedule == "" {
		return Job{}, errors.New("schedule is required")
	}
	if _, err := parseSchedule(job.Schedule, time.Now().UTC()); err != nil {
		return Job{}, fmt.Errorf("invalid schedule: %w", err)
	}
	switch job.GovernancePolicy {
	case "auto", "approval_required", "disabled":
	default:
		return Job{}, fmt.Errorf("invalid governance_policy %q", job.GovernancePolicy)
	}
	job.TimeoutSeconds = clampInt(job.TimeoutSeconds, 0, 3600)
	if job.TimeoutSeconds == 0 {
		job.TimeoutSeconds = int(defaultRunTimeout.Seconds())
	}
	job.RetryMaxAttempts = clampInt(job.RetryMaxAttempts, 0, 5)
	if job.RetryInitialBackoff != "" {
		if _, err := time.ParseDuration(job.RetryInitialBackoff); err != nil {
			return Job{}, fmt.Errorf("invalid retry_initial_backoff: %w", err)
		}
	}
	job.Labels = cloneStringMap(job.Labels)
	job.Description = strings.TrimSpace(job.Description)
	switch job.Type {
	case jobTypeSkill:
		if job.Skill == nil {
			job.Skill = &SkillTarget{SkillID: job.TargetRef}
		}
		job.Skill.SkillID = firstNonEmpty(strings.TrimSpace(job.Skill.SkillID), job.TargetRef)
		job.TargetRef = job.Skill.SkillID
		job.Skill.Context = cloneInterfaceMap(job.Skill.Context)
		if job.TargetRef == "" {
			return Job{}, errors.New("skill.skill_id is required")
		}
		job.ConnectorCapability = nil
	case jobTypeConnectorCapability:
		if job.ConnectorCapability == nil {
			job.ConnectorCapability = &ConnectorCapabilityJob{}
		}
		if job.TargetRef != "" {
			parts := strings.Split(job.TargetRef, "/")
			if len(parts) == 2 {
				job.ConnectorCapability.ConnectorID = firstNonEmpty(strings.TrimSpace(job.ConnectorCapability.ConnectorID), strings.TrimSpace(parts[0]))
				job.ConnectorCapability.CapabilityID = firstNonEmpty(strings.TrimSpace(job.ConnectorCapability.CapabilityID), strings.TrimSpace(parts[1]))
			}
		}
		job.ConnectorCapability.ConnectorID = strings.TrimSpace(job.ConnectorCapability.ConnectorID)
		job.ConnectorCapability.CapabilityID = strings.TrimSpace(job.ConnectorCapability.CapabilityID)
		job.ConnectorCapability.Params = cloneInterfaceMap(job.ConnectorCapability.Params)
		if job.ConnectorCapability.ConnectorID == "" || job.ConnectorCapability.CapabilityID == "" {
			return Job{}, errors.New("connector_capability.connector_id and connector_capability.capability_id are required")
		}
		job.TargetRef = job.ConnectorCapability.ConnectorID + "/" + job.ConnectorCapability.CapabilityID
		job.Skill = nil
	default:
		return Job{}, fmt.Errorf("unsupported job type %q", job.Type)
	}
	return job, nil
}

func statusForJob(job Job) string {
	if job.Enabled {
		return "enabled"
	}
	return "disabled"
}

func defaultJobState(job Job, now time.Time) JobState {
	state := JobState{
		JobID:     job.ID,
		Status:    statusForJob(job),
		UpdatedAt: now,
	}
	if job.Enabled {
		state.NextRunAt = nextScheduleAfter(job.Schedule, now)
	}
	return state
}

func parseSchedule(schedule string, now time.Time) (time.Time, error) {
	schedule = strings.TrimSpace(schedule)
	if strings.HasPrefix(schedule, "@every ") {
		d, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(schedule, "@every ")))
		if err != nil {
			return time.Time{}, err
		}
		if d <= 0 {
			return time.Time{}, errors.New("duration must be positive")
		}
		return now.Add(d), nil
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	parsed, err := parser.Parse(schedule)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.Next(now), nil
}

func nextScheduleAfter(schedule string, now time.Time) time.Time {
	next, err := parseSchedule(schedule, now)
	if err != nil {
		return time.Time{}
	}
	return next.UTC()
}

func writeFileAtomically(path string, content string, pattern string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), pattern+"*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func lifecycleStatePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return path + ".state.yaml"
}

func getJobLocked(cfg *Config, id string) (Job, bool) {
	if cfg == nil {
		return Job{}, false
	}
	for _, job := range cfg.Jobs {
		if job.ID == id {
			return job, true
		}
	}
	return Job{}, false
}

func normalizedAttempts(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func retryInitialInterval(raw string) time.Duration {
	if d, err := time.ParseDuration(strings.TrimSpace(raw)); err == nil && d > 0 {
		return d
	}
	return 2 * time.Second
}

func durationOrDefault(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func shouldRetry(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "failed")
}

func defaultMetricsMode(tool string) string {
	if strings.HasSuffix(strings.TrimSpace(tool), "query_range") {
		return "range"
	}
	return "instant"
}

func (m *Manager) nextRunID() string {
	if id := uuid.NewString(); id != "" {
		return id
	}
	return fmt.Sprintf("aut-run-%06d", m.runSeq.Add(1))
}

func derefConfig(cfg *Config) Config {
	if cfg == nil {
		return Config{}
	}
	return *cfg
}

func cloneConfig(cfg Config) Config {
	out := Config{Jobs: make([]Job, 0, len(cfg.Jobs))}
	for _, job := range cfg.Jobs {
		out.Jobs = append(out.Jobs, cloneJob(job))
	}
	return out
}

func cloneJob(job Job) Job {
	out := job
	out.Labels = cloneStringMap(job.Labels)
	if job.Skill != nil {
		copySkill := *job.Skill
		copySkill.Context = cloneInterfaceMap(job.Skill.Context)
		out.Skill = &copySkill
	}
	if job.ConnectorCapability != nil {
		copyCapability := *job.ConnectorCapability
		copyCapability.Params = cloneInterfaceMap(job.ConnectorCapability.Params)
		out.ConnectorCapability = &copyCapability
	}
	return out
}

func cloneStateMap(input map[string]JobState) map[string]JobState {
	if len(input) == 0 {
		return map[string]JobState{}
	}
	out := make(map[string]JobState, len(input))
	for key, value := range input {
		out[key] = cloneState(value)
	}
	return out
}

func cloneState(in JobState) JobState {
	out := in
	if len(in.Runs) > 0 {
		out.Runs = make([]Run, 0, len(in.Runs))
		for _, run := range in.Runs {
			out.Runs = append(out.Runs, cloneRun(run))
		}
	}
	return out
}

func cloneRun(in Run) Run {
	out := in
	out.Metadata = cloneInterfaceMap(in.Metadata)
	return out
}

func cloneInterfaceMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		out[trimmed] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		out[trimmed] = strings.TrimSpace(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func interfaceString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func mergeMetadata(base map[string]interface{}, extra map[string]interface{}) map[string]interface{} {
	out := cloneInterfaceMap(base)
	if out == nil {
		out = map[string]interface{}{}
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func stringMapValue(input map[string]interface{}, key string) string {
	if len(input) == 0 {
		return ""
	}
	return interfaceString(input[key])
}

func fallbackLogger(log *slog.Logger) *slog.Logger {
	if log != nil {
		return log
	}
	return slog.Default()
}

func automationToolCapabilities(manager *connectors.Manager) []contracts.ToolCapabilityDescriptor {
	out := []contracts.ToolCapabilityDescriptor{
		{
			Tool:        "knowledge.search",
			ReadOnly:    true,
			Invocable:   true,
			Source:      "builtin",
			Description: "Search TARS knowledge memory, incident summaries, and imported references.",
			Scopes:      []string{"knowledge.read"},
		},
	}
	out = append(out, toToolCapabilityDescriptors(connectors.ToolPlanCapabilities(manager))...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Invocable != out[j].Invocable {
			return out[i].Invocable
		}
		if out[i].Tool != out[j].Tool {
			return out[i].Tool < out[j].Tool
		}
		if out[i].ConnectorType != out[j].ConnectorType {
			return out[i].ConnectorType < out[j].ConnectorType
		}
		if out[i].ConnectorID != out[j].ConnectorID {
			return out[i].ConnectorID < out[j].ConnectorID
		}
		return out[i].CapabilityID < out[j].CapabilityID
	})
	return out
}

func automationToolCapabilitySummary(items []contracts.ToolCapabilityDescriptor) string {
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		label := item.Tool
		if strings.TrimSpace(item.ConnectorID) != "" {
			label += fmt.Sprintf(" via %s", item.ConnectorID)
		}
		if strings.TrimSpace(item.CapabilityID) != "" {
			label += fmt.Sprintf(" [%s]", item.CapabilityID)
		}
		if item.Invocable {
			label += " (invocable)"
		} else {
			label += " (catalog-only)"
		}
		if strings.TrimSpace(item.Description) != "" {
			label += ": " + item.Description
		}
		lines = append(lines, "- "+label)
	}
	return strings.Join(lines, "\n")
}

func toToolCapabilityDescriptors(input []connectors.ToolPlanCapability) []contracts.ToolCapabilityDescriptor {
	if len(input) == 0 {
		return nil
	}
	out := make([]contracts.ToolCapabilityDescriptor, 0, len(input))
	for _, item := range input {
		out = append(out, contracts.ToolCapabilityDescriptor{
			Tool:            item.Tool,
			ConnectorID:     item.ConnectorID,
			ConnectorType:   item.ConnectorType,
			ConnectorVendor: item.ConnectorVendor,
			Protocol:        item.Protocol,
			CapabilityID:    item.CapabilityID,
			Action:          item.Action,
			Scopes:          append([]string(nil), item.Scopes...),
			ReadOnly:        item.ReadOnly,
			Invocable:       item.Invocable,
			Source:          item.Source,
			Description:     item.Description,
		})
	}
	return out
}

var _ = actionmod.NewService
var _ = knowledgemod.NewService
