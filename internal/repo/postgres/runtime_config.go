package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"tars/internal/modules/access"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/connectors"
	"tars/internal/modules/org"
	"tars/internal/modules/reasoning"
)

const (
	runtimeConfigAccessDocKey           = "access"
	runtimeConfigAgentRolesDocKey       = "agent_roles"
	runtimeConfigApprovalRoutingDocKey  = "approval_routing"
	runtimeConfigAuthorizationDocKey    = "authorization"
	runtimeConfigConnectorsDocKey       = "connectors"
	runtimeConfigDesensitizationDocKey  = "desensitization"
	runtimeConfigOrgDocKey              = "org"
	runtimeConfigProvidersDocKey        = "providers"
	runtimeConfigReasoningPromptsDocKey = "reasoning_prompts"
)

type SetupLoginHint struct {
	Username string `json:"username,omitempty"`
	Provider string `json:"provider,omitempty"`
	LoginURL string `json:"login_url,omitempty"`
}

type SetupState struct {
	Initialized       bool           `json:"initialized"`
	CurrentStep       string         `json:"current_step,omitempty"`
	AdminUserID       string         `json:"admin_user_id,omitempty"`
	AuthProviderID    string         `json:"auth_provider_id,omitempty"`
	PrimaryProviderID string         `json:"primary_provider_id,omitempty"`
	PrimaryModel      string         `json:"primary_model,omitempty"`
	DefaultChannelID  string         `json:"default_channel_id,omitempty"`
	ProviderChecked   bool           `json:"provider_checked,omitempty"`
	ProviderCheckOK   bool           `json:"provider_check_ok,omitempty"`
	ProviderCheckNote string         `json:"provider_check_note,omitempty"`
	LoginHint         SetupLoginHint `json:"login_hint,omitempty"`
	CompletedAt       time.Time      `json:"completed_at,omitempty"`
	UpdatedAt         time.Time      `json:"updated_at,omitempty"`
}

type ConnectorsState struct {
	Config    connectors.Config                    `json:"config"`
	Lifecycle map[string]connectors.LifecycleState `json:"lifecycle,omitempty"`
	UpdatedAt time.Time                            `json:"updated_at,omitempty"`
}

type RuntimeConfigStore struct {
	db   *sql.DB
	mu   sync.RWMutex
	docs map[string]json.RawMessage
	set  SetupState
}

type setupStateScanner interface {
	Scan(dest ...any) error
}

func NewRuntimeConfigStore(db *sql.DB) *RuntimeConfigStore {
	return &RuntimeConfigStore{
		db:   db,
		docs: map[string]json.RawMessage{},
		set:  defaultSetupState(),
	}
}

func (s *RuntimeConfigStore) LoadAccessConfig(ctx context.Context) (access.Config, bool, error) {
	var cfg access.Config
	found, err := s.loadDocument(ctx, runtimeConfigAccessDocKey, &cfg)
	return cfg, found, err
}

func (s *RuntimeConfigStore) SaveAccessConfig(ctx context.Context, cfg access.Config) error {
	return s.saveDocument(ctx, runtimeConfigAccessDocKey, cfg)
}

func (s *RuntimeConfigStore) LoadAuthorizationConfig(ctx context.Context) (authorization.Config, bool, error) {
	var cfg authorization.Config
	found, err := s.loadDocument(ctx, runtimeConfigAuthorizationDocKey, &cfg)
	return cfg, found, err
}

func (s *RuntimeConfigStore) SaveAuthorizationConfig(ctx context.Context, cfg authorization.Config) error {
	return s.saveDocument(ctx, runtimeConfigAuthorizationDocKey, cfg)
}

func (s *RuntimeConfigStore) LoadApprovalRoutingConfig(ctx context.Context) (approvalrouting.Config, bool, error) {
	var cfg approvalrouting.Config
	found, err := s.loadDocument(ctx, runtimeConfigApprovalRoutingDocKey, &cfg)
	return cfg, found, err
}

func (s *RuntimeConfigStore) SaveApprovalRoutingConfig(ctx context.Context, cfg approvalrouting.Config) error {
	return s.saveDocument(ctx, runtimeConfigApprovalRoutingDocKey, cfg)
}

func (s *RuntimeConfigStore) LoadOrgConfig(ctx context.Context) (org.Config, bool, error) {
	var cfg org.Config
	found, err := s.loadDocument(ctx, runtimeConfigOrgDocKey, &cfg)
	return cfg, found, err
}

func (s *RuntimeConfigStore) SaveOrgConfig(ctx context.Context, cfg org.Config) error {
	return s.saveDocument(ctx, runtimeConfigOrgDocKey, cfg)
}

func (s *RuntimeConfigStore) LoadReasoningPromptsConfig(ctx context.Context) (reasoning.PromptSet, bool, error) {
	var cfg reasoning.PromptSet
	found, err := s.loadDocument(ctx, runtimeConfigReasoningPromptsDocKey, &cfg)
	return cfg, found, err
}

func (s *RuntimeConfigStore) SaveReasoningPromptsConfig(ctx context.Context, cfg reasoning.PromptSet) error {
	return s.saveDocument(ctx, runtimeConfigReasoningPromptsDocKey, cfg)
}

func (s *RuntimeConfigStore) LoadDesensitizationConfig(ctx context.Context) (reasoning.DesensitizationConfig, bool, error) {
	var cfg reasoning.DesensitizationConfig
	found, err := s.loadDocument(ctx, runtimeConfigDesensitizationDocKey, &cfg)
	return cfg, found, err
}

func (s *RuntimeConfigStore) SaveDesensitizationConfig(ctx context.Context, cfg reasoning.DesensitizationConfig) error {
	return s.saveDocument(ctx, runtimeConfigDesensitizationDocKey, cfg)
}

func (s *RuntimeConfigStore) LoadAgentRolesConfig(ctx context.Context) (agentrole.Config, bool, error) {
	var cfg agentrole.Config
	found, err := s.loadDocument(ctx, runtimeConfigAgentRolesDocKey, &cfg)
	return cfg, found, err
}

func (s *RuntimeConfigStore) SaveAgentRolesConfig(ctx context.Context, cfg agentrole.Config) error {
	return s.saveDocument(ctx, runtimeConfigAgentRolesDocKey, cfg)
}

func (s *RuntimeConfigStore) LoadProvidersConfig(ctx context.Context) (reasoning.ProvidersConfig, bool, error) {
	var cfg reasoning.ProvidersConfig
	found, err := s.loadDocument(ctx, runtimeConfigProvidersDocKey, &cfg)
	return cfg, found, err
}

func (s *RuntimeConfigStore) SaveProvidersConfig(ctx context.Context, cfg reasoning.ProvidersConfig) error {
	return s.saveDocument(ctx, runtimeConfigProvidersDocKey, cfg)
}

func (s *RuntimeConfigStore) LoadConnectorsState(ctx context.Context) (ConnectorsState, bool, error) {
	var state ConnectorsState
	found, err := s.loadDocument(ctx, runtimeConfigConnectorsDocKey, &state)
	if !found || err != nil {
		return state, found, err
	}
	return normalizeConnectorsState(state), true, nil
}

func (s *RuntimeConfigStore) SaveConnectorsState(ctx context.Context, state ConnectorsState) error {
	return s.saveDocument(ctx, runtimeConfigConnectorsDocKey, normalizeConnectorsState(state))
}

func (s *RuntimeConfigStore) LoadSetupState(ctx context.Context) (SetupState, error) {
	if s == nil {
		return defaultSetupState(), nil
	}
	if s.db == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return normalizeSetupState(s.set), nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT initialized, current_step, admin_user_id, auth_provider_id, primary_provider_id, primary_model, default_channel_id, provider_checked, provider_check_ok, provider_check_note, login_hint, completed_at, updated_at
		FROM setup_state
		WHERE id = TRUE
	`)
	state, err := scanSetupStateRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return defaultSetupState(), nil
		}
		return SetupState{}, err
	}
	return normalizeSetupState(state), nil
}

func (s *RuntimeConfigStore) SaveSetupState(ctx context.Context, state SetupState) error {
	if s == nil {
		return nil
	}
	state = normalizeSetupState(state)
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}
	if s.db == nil {
		s.mu.Lock()
		s.set = state
		s.mu.Unlock()
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO setup_state (
			id, initialized, current_step, admin_user_id, auth_provider_id, primary_provider_id, primary_model, default_channel_id, provider_checked, provider_check_ok, provider_check_note, login_hint, completed_at, updated_at
		) VALUES (
			TRUE, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
		ON CONFLICT (id) DO UPDATE SET
			initialized = EXCLUDED.initialized,
			current_step = EXCLUDED.current_step,
			admin_user_id = EXCLUDED.admin_user_id,
			auth_provider_id = EXCLUDED.auth_provider_id,
			primary_provider_id = EXCLUDED.primary_provider_id,
			primary_model = EXCLUDED.primary_model,
			default_channel_id = EXCLUDED.default_channel_id,
			provider_checked = EXCLUDED.provider_checked,
			provider_check_ok = EXCLUDED.provider_check_ok,
			provider_check_note = EXCLUDED.provider_check_note,
			login_hint = EXCLUDED.login_hint,
			completed_at = EXCLUDED.completed_at,
			updated_at = EXCLUDED.updated_at
	`,
		state.Initialized,
		state.CurrentStep,
		state.AdminUserID,
		state.AuthProviderID,
		state.PrimaryProviderID,
		state.PrimaryModel,
		state.DefaultChannelID,
		state.ProviderChecked,
		state.ProviderCheckOK,
		state.ProviderCheckNote,
		mustJSON(state.LoginHint),
		nullableTime(state.CompletedAt),
		state.UpdatedAt,
	)
	return err
}

func (s *RuntimeConfigStore) loadDocument(ctx context.Context, key string, target any) (bool, error) {
	if s == nil {
		return false, nil
	}
	if s.db == nil {
		s.mu.RLock()
		payload, ok := s.docs[key]
		s.mu.RUnlock()
		if !ok || len(payload) == 0 {
			return false, nil
		}
		if err := json.Unmarshal(payload, target); err != nil {
			return false, err
		}
		return true, nil
	}
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM runtime_config_documents WHERE document_key = $1`, key).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if len(payload) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return false, err
	}
	return true, nil
}

func (s *RuntimeConfigStore) saveDocument(ctx context.Context, key string, value any) error {
	if s == nil {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if s.db == nil {
		s.mu.Lock()
		s.docs[key] = append([]byte(nil), payload...)
		s.mu.Unlock()
		return nil
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO runtime_config_documents (document_key, payload, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (document_key) DO UPDATE SET
			payload = EXCLUDED.payload,
			updated_at = EXCLUDED.updated_at
	`, key, payload)
	return err
}

func defaultSetupState() SetupState {
	return SetupState{CurrentStep: "admin"}
}

func normalizeSetupState(state SetupState) SetupState {
	if state.Initialized {
		state.CurrentStep = ""
		return state
	}
	if state.CurrentStep == "" {
		state.CurrentStep = "admin"
	}
	return state
}

func normalizeConnectorsState(state ConnectorsState) ConnectorsState {
	state.Config.Entries = cloneConnectorManifests(state.Config.Entries)
	sort.SliceStable(state.Config.Entries, func(i, j int) bool {
		return state.Config.Entries[i].Metadata.ID < state.Config.Entries[j].Metadata.ID
	})
	if len(state.Lifecycle) == 0 {
		state.Lifecycle = map[string]connectors.LifecycleState{}
		return state
	}
	normalized := make(map[string]connectors.LifecycleState, len(state.Lifecycle))
	for key, value := range state.Lifecycle {
		id := value.ConnectorID
		if id == "" {
			id = key
		}
		if id == "" {
			continue
		}
		normalized[id] = cloneConnectorLifecycleState(value)
	}
	state.Lifecycle = normalized
	return state
}

func cloneConnectorManifests(input []connectors.Manifest) []connectors.Manifest {
	if len(input) == 0 {
		return nil
	}
	bytes, _ := json.Marshal(input)
	var out []connectors.Manifest
	_ = json.Unmarshal(bytes, &out)
	return out
}

func cloneConnectorLifecycleState(input connectors.LifecycleState) connectors.LifecycleState {
	bytes, _ := json.Marshal(input)
	var out connectors.LifecycleState
	_ = json.Unmarshal(bytes, &out)
	return out
}

func nullableTime(ts time.Time) any {
	if ts.IsZero() {
		return nil
	}
	return ts
}

func mustJSON(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return payload
}

func scanSetupStateRow(row setupStateScanner) (SetupState, error) {
	var state SetupState
	var loginHintPayload []byte
	var completedAt sql.NullTime
	if err := row.Scan(
		&state.Initialized,
		&state.CurrentStep,
		&state.AdminUserID,
		&state.AuthProviderID,
		&state.PrimaryProviderID,
		&state.PrimaryModel,
		&state.DefaultChannelID,
		&state.ProviderChecked,
		&state.ProviderCheckOK,
		&state.ProviderCheckNote,
		&loginHintPayload,
		&completedAt,
		&state.UpdatedAt,
	); err != nil {
		return SetupState{}, err
	}
	if len(loginHintPayload) > 0 {
		_ = json.Unmarshal(loginHintPayload, &state.LoginHint)
	}
	if completedAt.Valid {
		state.CompletedAt = completedAt.Time
	}
	return state, nil
}
