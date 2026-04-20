package automations

import "time"

type Config struct {
	Jobs []Job `yaml:"jobs,omitempty"`
}

type Snapshot struct {
	Path      string
	Content   string
	Config    Config
	State     map[string]JobState
	UpdatedAt time.Time
	Loaded    bool
}

type fileConfig struct {
	Automations Config `yaml:"automations"`
}

type stateFile struct {
	Automations struct {
		Jobs []JobState `yaml:"jobs,omitempty"`
	} `yaml:"automations"`
}

type Job struct {
	ID                  string                  `json:"id" yaml:"id"`
	DisplayName         string                  `json:"display_name" yaml:"display_name"`
	Description         string                  `json:"description,omitempty" yaml:"description,omitempty"`
	AgentRoleID         string                  `json:"agent_role_id,omitempty" yaml:"agent_role_id,omitempty"`
	GovernancePolicy    string                  `json:"governance_policy,omitempty" yaml:"governance_policy,omitempty"`
	Type                string                  `json:"type" yaml:"type"`
	TargetRef           string                  `json:"target_ref" yaml:"target_ref"`
	Schedule            string                  `json:"schedule" yaml:"schedule"`
	Enabled             bool                    `json:"enabled" yaml:"enabled"`
	Owner               string                  `json:"owner,omitempty" yaml:"owner,omitempty"`
	RuntimeMode         string                  `json:"runtime_mode,omitempty" yaml:"runtime_mode,omitempty"`
	TimeoutSeconds      int                     `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
	RetryMaxAttempts    int                     `json:"retry_max_attempts,omitempty" yaml:"retry_max_attempts,omitempty"`
	RetryInitialBackoff string                  `json:"retry_initial_backoff,omitempty" yaml:"retry_initial_backoff,omitempty"`
	Labels              map[string]string       `json:"labels,omitempty" yaml:"labels,omitempty"`
	Skill               *SkillTarget            `json:"skill,omitempty" yaml:"skill,omitempty"`
	ConnectorCapability *ConnectorCapabilityJob `json:"connector_capability,omitempty" yaml:"connector_capability,omitempty"`
}

type SkillTarget struct {
	SkillID string                 `json:"skill_id" yaml:"skill_id"`
	Context map[string]interface{} `json:"context,omitempty" yaml:"context,omitempty"`
}

type ConnectorCapabilityJob struct {
	ConnectorID  string                 `json:"connector_id" yaml:"connector_id"`
	CapabilityID string                 `json:"capability_id" yaml:"capability_id"`
	Params       map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
}

type JobState struct {
	JobID               string    `json:"job_id" yaml:"job_id"`
	Status              string    `json:"status,omitempty" yaml:"status,omitempty"`
	LastRunAt           time.Time `json:"last_run_at,omitempty" yaml:"last_run_at,omitempty"`
	NextRunAt           time.Time `json:"next_run_at,omitempty" yaml:"next_run_at,omitempty"`
	LastOutcome         string    `json:"last_outcome,omitempty" yaml:"last_outcome,omitempty"`
	LastError           string    `json:"last_error,omitempty" yaml:"last_error,omitempty"`
	ConsecutiveFailures int       `json:"consecutive_failures,omitempty" yaml:"consecutive_failures,omitempty"`
	Runs                []Run     `json:"runs,omitempty" yaml:"runs,omitempty"`
	UpdatedAt           time.Time `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

type Run struct {
	RunID        string                 `json:"run_id" yaml:"run_id"`
	JobID        string                 `json:"job_id" yaml:"job_id"`
	Trigger      string                 `json:"trigger" yaml:"trigger"`
	Status       string                 `json:"status" yaml:"status"`
	StartedAt    time.Time              `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt  time.Time              `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
	AttemptCount int                    `json:"attempt_count,omitempty" yaml:"attempt_count,omitempty"`
	Summary      string                 `json:"summary,omitempty" yaml:"summary,omitempty"`
	Error        string                 `json:"error,omitempty" yaml:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type RunRequest struct {
	Trigger     string
	TriggeredBy string
}
