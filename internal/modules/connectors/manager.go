package connectors

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

var ErrConfigPathNotSet = errors.New("connectors config path is not set")
var ErrConnectorNotFound = errors.New("connector not found")

type Config struct {
	Entries []Manifest
}

type Snapshot struct {
	Path      string
	Content   string
	Config    Config
	Lifecycle map[string]LifecycleState
	UpdatedAt time.Time
	Loaded    bool
}

type fileConfig struct {
	Connectors struct {
		Entries []Manifest `yaml:"entries,omitempty"`
	} `yaml:"connectors"`
}

type lifecycleStateFile struct {
	Connectors struct {
		Entries []LifecycleState `yaml:"entries,omitempty"`
	} `yaml:"connectors"`
}

type Manager struct {
	mu        sync.RWMutex
	path      string
	statePath string
	content   string
	config    *Config
	state     map[string]LifecycleState
	persist   func(Config, map[string]LifecycleState) error
	updatedAt time.Time
}

type UpgradeOptions struct {
	Manifest  Manifest
	Reason    string
	Available string
}

type RollbackOptions struct {
	TargetVersion string
	Reason        string
}

func DefaultConfig() Config {
	return Config{}
}

func NewManager(path string) (*Manager, error) {
	manager := &Manager{path: path, statePath: lifecycleStatePath(path)}
	if strings.TrimSpace(path) == "" {
		cfg := DefaultConfig()
		manager.config = &cfg
		manager.state = map[string]LifecycleState{}
		return manager, nil
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
		Lifecycle: cloneLifecycleMap(m.state),
	}
	if m.config != nil {
		snapshot.Config = cloneConfig(*m.config)
	}
	return snapshot
}

func (m *Manager) Reload() error {
	if m == nil || strings.TrimSpace(m.path) == "" {
		return nil
	}
	content, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	cfg, normalized, err := ParseConfig(content)
	if err != nil {
		return err
	}
	state, err := loadLifecycleStateFile(m.statePath)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	state = syncLifecycleState(state, nil, cfg, now)
	m.mu.Lock()
	m.content = normalized
	m.config = cfg
	m.state = state
	m.updatedAt = now
	m.mu.Unlock()
	return nil
}

func (m *Manager) SetPersistence(persist func(Config, map[string]LifecycleState) error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.persist = persist
	m.mu.Unlock()
}

func (m *Manager) LoadRuntimeState(cfg Config, lifecycle map[string]LifecycleState) error {
	if m == nil {
		return ErrConfigPathNotSet
	}
	if err := ValidateConfigCompatibility(cfg); err != nil {
		return err
	}
	content, err := EncodeConfig(&cfg)
	if err != nil {
		return err
	}
	cfg = normalizeConfig(cfg)
	content, err = EncodeConfig(&cfg)
	if err != nil {
		return err
	}
	state := cloneLifecycleMap(lifecycle)
	if len(state) == 0 {
		state = syncLifecycleState(state, nil, &cfg, time.Now().UTC())
	}
	m.mu.Lock()
	m.content = content
	m.config = &cfg
	m.state = state
	m.updatedAt = time.Now().UTC()
	m.mu.Unlock()
	return nil
}

func (m *Manager) SaveConfig(cfg Config) error {
	if m == nil {
		return ErrConfigPathNotSet
	}
	if err := ValidateConfigCompatibility(cfg); err != nil {
		return err
	}
	content, err := EncodeConfig(&cfg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(m.path) == "" {
		m.mu.RLock()
		var previous *Config
		if m.config != nil {
			cloned := cloneConfig(*m.config)
			previous = &cloned
		}
		state := cloneLifecycleMap(m.state)
		m.mu.RUnlock()
		now := time.Now().UTC()
		cfg = normalizeConfig(cfg)
		content, err = EncodeConfig(&cfg)
		if err != nil {
			return err
		}
		state = syncLifecycleState(state, previous, &cfg, now)
		m.mu.Lock()
		m.content = content
		m.config = &cfg
		m.state = state
		m.updatedAt = now
		persist := m.persist
		m.mu.Unlock()
		if persist != nil {
			return persist(cloneConfig(cfg), cloneLifecycleMap(state))
		}
		return nil
	}
	return m.saveConfigAndState(&cfg, content)
}

func (m *Manager) Save(content string) error {
	if m == nil {
		return ErrConfigPathNotSet
	}
	cfg, normalized, err := ParseConfig([]byte(content))
	if err != nil {
		return err
	}
	if err := ValidateConfigCompatibility(*cfg); err != nil {
		return err
	}
	if strings.TrimSpace(m.path) == "" {
		return m.SaveConfig(*cfg)
	}
	return m.saveConfigAndState(cfg, normalized)
}

func (m *Manager) saveConfigAndState(cfg *Config, normalized string) error {
	if m == nil || strings.TrimSpace(m.path) == "" {
		return ErrConfigPathNotSet
	}
	m.mu.RLock()
	var previous *Config
	if m.config != nil {
		cloned := cloneConfig(*m.config)
		previous = &cloned
	}
	state := cloneLifecycleMap(m.state)
	m.mu.RUnlock()

	now := time.Now().UTC()
	state = syncLifecycleState(state, previous, cfg, now)
	if err := writeFileAtomically(m.path, normalized); err != nil {
		return err
	}
	if err := saveLifecycleStateFile(m.statePath, state); err != nil {
		return err
	}
	m.mu.Lock()
	m.content = normalized
	m.config = cfg
	m.state = state
	m.updatedAt = now
	persist := m.persist
	m.mu.Unlock()
	if persist != nil {
		return persist(cloneConfig(*cfg), cloneLifecycleMap(state))
	}
	return nil
}

func (m *Manager) Upsert(entry Manifest) error {
	if m == nil {
		return ErrConfigPathNotSet
	}
	m.mu.RLock()
	current := DefaultConfig()
	if m.config != nil {
		current = cloneConfig(*m.config)
	}
	m.mu.RUnlock()

	normalized, err := normalizeManifest(entry)
	if err != nil {
		return err
	}

	replaced := false
	for i := range current.Entries {
		if current.Entries[i].Metadata.ID == normalized.Metadata.ID {
			current.Entries[i] = normalized
			replaced = true
			break
		}
	}
	if !replaced {
		current.Entries = append(current.Entries, normalized)
	}
	return m.SaveConfig(current)
}

func (m *Manager) Get(id string) (Manifest, bool) {
	if m == nil {
		return Manifest{}, false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Manifest{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config == nil {
		return Manifest{}, false
	}
	for _, entry := range m.config.Entries {
		if entry.Metadata.ID == id {
			cloned := cloneManifests([]Manifest{entry})
			return cloned[0], true
		}
	}
	return Manifest{}, false
}

func (m *Manager) GetLifecycle(id string) (LifecycleState, bool) {
	if m == nil {
		return LifecycleState{}, false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return LifecycleState{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.state[id]
	if !ok {
		return LifecycleState{}, false
	}
	return cloneLifecycleState(state), true
}

func (m *Manager) ListLifecycle() []LifecycleState {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]LifecycleState, 0, len(m.state))
	for _, state := range m.state {
		items = append(items, cloneLifecycleState(state))
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].ConnectorID < items[j].ConnectorID
	})
	return items
}

func (m *Manager) RecordHealth(id string, status string, summary string, checkedAt time.Time) (LifecycleState, error) {
	if m == nil {
		return LifecycleState{}, ErrConfigPathNotSet
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return LifecycleState{}, ErrConnectorNotFound
	}
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}

	m.mu.Lock()
	state, ok := m.state[id]
	if !ok {
		m.mu.Unlock()
		return LifecycleState{}, ErrConnectorNotFound
	}
	state.Health = HealthStatus{
		Status:    strings.TrimSpace(status),
		Summary:   strings.TrimSpace(summary),
		CheckedAt: checkedAt,
	}
	state.UpdatedAt = checkedAt
	state.HealthHistory = appendHealthHistory(state.HealthHistory, state.Health)
	m.state[id] = state
	persist := m.persist
	stateMap := cloneLifecycleMap(m.state)
	m.mu.Unlock()
	if strings.TrimSpace(m.path) != "" {
		if err := saveLifecycleStateFile(m.statePath, stateMap); err != nil {
			return LifecycleState{}, err
		}
	}
	if persist != nil {
		cfg := DefaultConfig()
		m.mu.RLock()
		if m.config != nil {
			cfg = cloneConfig(*m.config)
		}
		m.mu.RUnlock()
		if err := persist(cfg, stateMap); err != nil {
			return LifecycleState{}, err
		}
	}
	return cloneLifecycleState(state), nil
}

func (m *Manager) SetEnabled(id string, enabled bool) (Manifest, error) {
	if m == nil {
		return Manifest{}, ErrConfigPathNotSet
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Manifest{}, ErrConnectorNotFound
	}

	m.mu.RLock()
	current := DefaultConfig()
	if m.config != nil {
		current = cloneConfig(*m.config)
	}
	m.mu.RUnlock()

	for i := range current.Entries {
		if current.Entries[i].Metadata.ID == id {
			current.Entries[i].Disabled = !enabled
			if err := m.SaveConfig(current); err != nil {
				return Manifest{}, err
			}
			entry, ok := m.Get(id)
			if !ok {
				return Manifest{}, ErrConnectorNotFound
			}
			return entry, nil
		}
	}
	return Manifest{}, ErrConnectorNotFound
}

func (m *Manager) Upgrade(id string, opts UpgradeOptions) (Manifest, LifecycleState, error) {
	if m == nil {
		return Manifest{}, LifecycleState{}, ErrConfigPathNotSet
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Manifest{}, LifecycleState{}, ErrConnectorNotFound
	}

	normalized, err := normalizeManifest(opts.Manifest)
	if err != nil {
		return Manifest{}, LifecycleState{}, err
	}
	if normalized.Metadata.ID != id {
		return Manifest{}, LifecycleState{}, errors.New("manifest metadata.id must match connector id")
	}
	if !SupportsCurrentTARSMajor(normalized) {
		return Manifest{}, LifecycleState{}, errors.New("connector is not compatible with current TARS major version")
	}

	m.mu.RLock()
	current := DefaultConfig()
	if m.config != nil {
		current = cloneConfig(*m.config)
	}
	m.mu.RUnlock()

	index := -1
	for i := range current.Entries {
		if current.Entries[i].Metadata.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return Manifest{}, LifecycleState{}, ErrConnectorNotFound
	}
	previous := current.Entries[index]
	current.Entries[index] = normalized
	if err := m.SaveConfig(current); err != nil {
		return Manifest{}, LifecycleState{}, err
	}

	updatedState, ok := m.GetLifecycle(id)
	if !ok {
		return Manifest{}, LifecycleState{}, ErrConnectorNotFound
	}
	probeRequiredAt := time.Now().UTC()
	updatedState.Health = pendingRuntimeProbeHealth(normalized, updatedState.Compatibility, probeRequiredAt)
	updatedState.HealthHistory = appendHealthHistory(updatedState.HealthHistory, updatedState.Health)
	updatedState.UpdatedAt = probeRequiredAt
	if strings.TrimSpace(opts.Available) != "" {
		updatedState.AvailableVersion = strings.TrimSpace(opts.Available)
	}
	reason := strings.TrimSpace(opts.Reason)
	if reason != "" && previous.Metadata.Version == normalized.Metadata.Version {
		updatedState.History = append(updatedState.History, LifecycleEvent{
			Type:      "update_plan",
			Summary:   reason,
			Version:   normalized.Metadata.Version,
			ToVersion: normalized.Metadata.Version,
			Enabled:   boolPtr(normalized.Enabled()),
			Metadata: map[string]string{
				"reason": reason,
			},
			CreatedAt: time.Now().UTC(),
		})
	}
	if err := m.persistLifecycleState(id, updatedState); err != nil {
		return Manifest{}, LifecycleState{}, err
	}
	return normalized, updatedState, nil
}

func (m *Manager) Rollback(id string, opts RollbackOptions) (Manifest, LifecycleState, error) {
	if m == nil {
		return Manifest{}, LifecycleState{}, ErrConfigPathNotSet
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Manifest{}, LifecycleState{}, ErrConnectorNotFound
	}

	m.mu.RLock()
	state := cloneLifecycleMap(m.state)
	m.mu.RUnlock()
	currentState, ok := state[id]
	if !ok {
		return Manifest{}, LifecycleState{}, ErrConnectorNotFound
	}

	targetVersion := strings.TrimSpace(opts.TargetVersion)
	var target *Manifest
	for _, revision := range currentState.Revisions {
		if revision.Manifest == nil {
			continue
		}
		if targetVersion == "" || revision.Version == targetVersion {
			cloned := cloneManifests([]Manifest{*revision.Manifest})
			target = &cloned[0]
		}
	}
	if target == nil {
		return Manifest{}, LifecycleState{}, ErrConnectorNotFound
	}

	m.mu.RLock()
	current := DefaultConfig()
	if m.config != nil {
		current = cloneConfig(*m.config)
	}
	m.mu.RUnlock()
	index := -1
	var previous Manifest
	for i := range current.Entries {
		if current.Entries[i].Metadata.ID == id {
			index = i
			previous = current.Entries[i]
			break
		}
	}
	if index < 0 {
		return Manifest{}, LifecycleState{}, ErrConnectorNotFound
	}
	current.Entries[index] = *target
	if err := m.SaveConfig(current); err != nil {
		return Manifest{}, LifecycleState{}, err
	}

	updatedState, ok := m.GetLifecycle(id)
	if !ok {
		return Manifest{}, LifecycleState{}, ErrConnectorNotFound
	}
	updatedState.History = pruneRollbackAutoTransition(updatedState.History, previous.Metadata.Version, target.Metadata.Version)
	updatedState.Revisions = pruneRollbackAutoRevision(updatedState.Revisions, *target)
	probeRequiredAt := time.Now().UTC()
	updatedState.Health = pendingRuntimeProbeHealth(*target, updatedState.Compatibility, probeRequiredAt)
	updatedState.HealthHistory = appendHealthHistory(updatedState.HealthHistory, updatedState.Health)
	updatedState.UpdatedAt = probeRequiredAt
	updatedState.History = append(updatedState.History, LifecycleEvent{
		Type:        "rollback",
		Summary:     firstNonEmpty(strings.TrimSpace(opts.Reason), "connector rolled back"),
		Version:     target.Metadata.Version,
		FromVersion: previous.Metadata.Version,
		ToVersion:   target.Metadata.Version,
		Enabled:     boolPtr(target.Enabled()),
		Metadata: map[string]string{
			"reason": firstNonEmpty(strings.TrimSpace(opts.Reason), "unspecified"),
		},
		CreatedAt: time.Now().UTC(),
	})
	updatedState.Revisions = appendRevision(updatedState.Revisions, *target, time.Now().UTC(), firstNonEmpty(strings.TrimSpace(opts.Reason), "rollback"))
	if err := m.persistLifecycleState(id, updatedState); err != nil {
		return Manifest{}, LifecycleState{}, err
	}
	return *target, updatedState, nil
}

func (m *Manager) SetAvailableVersion(id string, version string) (LifecycleState, error) {
	if m == nil {
		return LifecycleState{}, ErrConfigPathNotSet
	}
	state, ok := m.GetLifecycle(strings.TrimSpace(id))
	if !ok {
		return LifecycleState{}, ErrConnectorNotFound
	}
	state.AvailableVersion = strings.TrimSpace(version)
	state.UpdatedAt = time.Now().UTC()
	if err := m.persistLifecycleState(state.ConnectorID, state); err != nil {
		return LifecycleState{}, err
	}
	return state, nil
}

func (m *Manager) persistLifecycleState(id string, state LifecycleState) error {
	if m == nil {
		return ErrConfigPathNotSet
	}
	m.mu.Lock()
	if m.state == nil {
		m.state = map[string]LifecycleState{}
	}
	m.state[id] = cloneLifecycleState(state)
	stateMap := cloneLifecycleMap(m.state)
	cfg := DefaultConfig()
	if m.config != nil {
		cfg = cloneConfig(*m.config)
	}
	persist := m.persist
	m.mu.Unlock()
	if strings.TrimSpace(m.path) != "" {
		if err := saveLifecycleStateFile(m.statePath, stateMap); err != nil {
			return err
		}
	}
	if persist != nil {
		return persist(cfg, stateMap)
	}
	return nil
}

func ParseConfig(content []byte) (*Config, string, error) {
	var raw fileConfig
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, "", err
	}
	cfg := DefaultConfig()
	for _, entry := range raw.Connectors.Entries {
		normalized, err := normalizeManifest(entry)
		if err != nil {
			return nil, "", err
		}
		cfg.Entries = append(cfg.Entries, normalized)
	}
	cfg = normalizeConfig(cfg)
	encoded, err := EncodeConfig(&cfg)
	if err != nil {
		return nil, "", err
	}
	return &cfg, encoded, nil
}

func SupportsCurrentTARSMajor(entry Manifest) bool {
	return CompatibilityReportForManifest(entry).Compatible
}

func EncodeConfig(cfg *Config) (string, error) {
	current := DefaultConfig()
	if cfg != nil {
		current = normalizeConfig(*cfg)
	}
	var raw fileConfig
	raw.Connectors.Entries = cloneManifests(current.Entries)
	bytes, err := yaml.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func normalizeConfig(cfg Config) Config {
	cfg.Entries = cloneManifests(cfg.Entries)
	sort.SliceStable(cfg.Entries, func(i, j int) bool {
		return cfg.Entries[i].Metadata.ID < cfg.Entries[j].Metadata.ID
	})
	return cfg
}

func normalizeManifest(entry Manifest) (Manifest, error) {
	entry.APIVersion = strings.TrimSpace(entry.APIVersion)
	if entry.APIVersion == "" {
		entry.APIVersion = "tars.connector/v1alpha1"
	}
	entry.Kind = strings.TrimSpace(entry.Kind)
	if entry.Kind == "" {
		entry.Kind = "connector"
	}
	entry.Metadata.ID = strings.TrimSpace(entry.Metadata.ID)
	entry.Metadata.Name = strings.TrimSpace(entry.Metadata.Name)
	entry.Metadata.DisplayName = strings.TrimSpace(entry.Metadata.DisplayName)
	entry.Metadata.Vendor = strings.TrimSpace(entry.Metadata.Vendor)
	entry.Metadata.Version = strings.TrimSpace(entry.Metadata.Version)
	entry.Metadata.Description = strings.TrimSpace(entry.Metadata.Description)
	entry.Metadata.OrgID = strings.TrimSpace(entry.Metadata.OrgID)
	entry.Metadata.TenantID = strings.TrimSpace(entry.Metadata.TenantID)
	entry.Metadata.WorkspaceID = strings.TrimSpace(entry.Metadata.WorkspaceID)
	entry.Spec.Type = strings.TrimSpace(entry.Spec.Type)
	entry.Spec.Protocol = strings.TrimSpace(entry.Spec.Protocol)
	entry.Spec.Capabilities = cloneCapabilities(entry.Spec.Capabilities)
	entry.Spec.ConnectionForm = cloneFields(entry.Spec.ConnectionForm)
	entry.Spec.ImportExport.Formats = cloneStrings(entry.Spec.ImportExport.Formats)
	entry.Config.Values = cloneStringMap(entry.Config.Values)
	entry.Config.SecretRefs = cloneStringMap(entry.Config.SecretRefs)
	entry.Compatibility.TARSMajorVersions = cloneStrings(entry.Compatibility.TARSMajorVersions)
	entry.Compatibility.UpstreamMajorVersions = cloneStrings(entry.Compatibility.UpstreamMajorVersions)
	entry.Compatibility.Modes = cloneStrings(entry.Compatibility.Modes)
	entry.Marketplace.Category = strings.TrimSpace(entry.Marketplace.Category)
	entry.Marketplace.Tags = cloneStrings(entry.Marketplace.Tags)
	entry.Marketplace.Source = strings.TrimSpace(entry.Marketplace.Source)

	for i := range entry.Spec.Capabilities {
		entry.Spec.Capabilities[i].ID = strings.TrimSpace(entry.Spec.Capabilities[i].ID)
		entry.Spec.Capabilities[i].Action = strings.TrimSpace(entry.Spec.Capabilities[i].Action)
		entry.Spec.Capabilities[i].Scopes = cloneStrings(entry.Spec.Capabilities[i].Scopes)
		entry.Spec.Capabilities[i].Description = strings.TrimSpace(entry.Spec.Capabilities[i].Description)
	}
	for i := range entry.Spec.ConnectionForm {
		entry.Spec.ConnectionForm[i].Key = strings.TrimSpace(entry.Spec.ConnectionForm[i].Key)
		entry.Spec.ConnectionForm[i].Label = strings.TrimSpace(entry.Spec.ConnectionForm[i].Label)
		entry.Spec.ConnectionForm[i].Type = strings.TrimSpace(entry.Spec.ConnectionForm[i].Type)
		entry.Spec.ConnectionForm[i].Default = strings.TrimSpace(entry.Spec.ConnectionForm[i].Default)
		entry.Spec.ConnectionForm[i].Options = cloneStrings(entry.Spec.ConnectionForm[i].Options)
		entry.Spec.ConnectionForm[i].Description = strings.TrimSpace(entry.Spec.ConnectionForm[i].Description)
	}

	if entry.Metadata.DisplayName == "" {
		entry.Metadata.DisplayName = firstNonEmpty(entry.Metadata.Name, entry.Metadata.ID)
	}
	if entry.Metadata.Name == "" {
		entry.Metadata.Name = firstNonEmpty(entry.Metadata.ID, entry.Metadata.DisplayName)
	}

	if err := ValidateManifest(entry); err != nil {
		return Manifest{}, err
	}
	return entry, nil
}

func ValidateManifest(entry Manifest) error {
	switch strings.TrimSpace(entry.APIVersion) {
	case "", "tars.connector/v1alpha1":
	default:
		return errors.New("manifest api_version must be tars.connector/v1alpha1")
	}
	if strings.TrimSpace(entry.Kind) != "connector" {
		return errors.New("manifest kind must be connector")
	}
	if strings.TrimSpace(entry.Metadata.ID) == "" {
		return errors.New("metadata.id is required")
	}
	if strings.TrimSpace(entry.Spec.Type) == "" {
		return errors.New("spec.type is required")
	}
	if strings.TrimSpace(entry.Spec.Protocol) == "" {
		return errors.New("spec.protocol is required")
	}
	if err := validateSSHSecretCustody(entry); err != nil {
		return err
	}
	return nil
}

func validateSSHSecretCustody(entry Manifest) error {
	protocol := strings.TrimSpace(entry.Spec.Protocol)
	if protocol != "ssh_native" && protocol != "ssh_shell" {
		return nil
	}
	for _, key := range []string{"password", "private_key", "passphrase"} {
		if value := strings.TrimSpace(entry.Config.Values[key]); value != "" {
			return errors.New("ssh connector secret material must be stored via credential custody; inline " + key + " is not allowed")
		}
	}
	return nil
}

func cloneConfig(cfg Config) Config {
	return Config{
		Entries: cloneManifests(cfg.Entries),
	}
}

func cloneLifecycleMap(input map[string]LifecycleState) map[string]LifecycleState {
	if len(input) == 0 {
		return map[string]LifecycleState{}
	}
	out := make(map[string]LifecycleState, len(input))
	for key, value := range input {
		out[key] = cloneLifecycleState(value)
	}
	return out
}

func cloneLifecycleState(input LifecycleState) LifecycleState {
	output := input
	output.Compatibility.Reasons = cloneStrings(input.Compatibility.Reasons)
	output.History = cloneLifecycleEvents(input.History)
	output.HealthHistory = cloneHealthStatuses(input.HealthHistory)
	output.Revisions = cloneRevisionSnapshots(input.Revisions)
	output.SecretRefs = cloneStringMap(input.SecretRefs)
	output.Templates = cloneTemplateAssignments(input.Templates)
	return output
}

func cloneRevisionSnapshots(input []RevisionSnapshot) []RevisionSnapshot {
	if len(input) == 0 {
		return nil
	}
	out := make([]RevisionSnapshot, 0, len(input))
	for _, item := range input {
		cloned := item
		if item.Manifest != nil {
			manifests := cloneManifests([]Manifest{*item.Manifest})
			cloned.Manifest = &manifests[0]
		}
		out = append(out, cloned)
	}
	return out
}

func cloneTemplateAssignments(input []TemplateAssignment) []TemplateAssignment {
	if len(input) == 0 {
		return nil
	}
	out := make([]TemplateAssignment, 0, len(input))
	for _, item := range input {
		cloned := item
		cloned.Values = cloneStringMap(item.Values)
		out = append(out, cloned)
	}
	return out
}

func cloneLifecycleEvents(input []LifecycleEvent) []LifecycleEvent {
	if len(input) == 0 {
		return nil
	}
	out := make([]LifecycleEvent, 0, len(input))
	for _, item := range input {
		cloned := item
		cloned.Metadata = cloneStringMap(item.Metadata)
		out = append(out, cloned)
	}
	return out
}

func cloneHealthStatuses(input []HealthStatus) []HealthStatus {
	if len(input) == 0 {
		return nil
	}
	out := make([]HealthStatus, len(input))
	copy(out, input)
	return out
}

func cloneManifests(input []Manifest) []Manifest {
	if len(input) == 0 {
		return nil
	}
	out := make([]Manifest, 0, len(input))
	for _, item := range input {
		cloned := item
		cloned.Spec.Capabilities = cloneCapabilities(item.Spec.Capabilities)
		cloned.Spec.ConnectionForm = cloneFields(item.Spec.ConnectionForm)
		cloned.Spec.ImportExport.Formats = cloneStrings(item.Spec.ImportExport.Formats)
		cloned.Config.Values = cloneStringMap(item.Config.Values)
		cloned.Config.SecretRefs = cloneStringMap(item.Config.SecretRefs)
		cloned.Compatibility.TARSMajorVersions = cloneStrings(item.Compatibility.TARSMajorVersions)
		cloned.Compatibility.UpstreamMajorVersions = cloneStrings(item.Compatibility.UpstreamMajorVersions)
		cloned.Compatibility.Modes = cloneStrings(item.Compatibility.Modes)
		cloned.Marketplace.Tags = cloneStrings(item.Marketplace.Tags)
		out = append(out, cloned)
	}
	return out
}

func cloneCapabilities(input []Capability) []Capability {
	if len(input) == 0 {
		return nil
	}
	out := make([]Capability, 0, len(input))
	for _, item := range input {
		out = append(out, Capability{
			ID:          item.ID,
			Action:      item.Action,
			ReadOnly:    item.ReadOnly,
			Invocable:   item.Invocable,
			Scopes:      cloneStrings(item.Scopes),
			Description: item.Description,
		})
	}
	return out
}

func cloneFields(input []Field) []Field {
	if len(input) == 0 {
		return nil
	}
	out := make([]Field, 0, len(input))
	for _, item := range input {
		out = append(out, Field{
			Key:         item.Key,
			Label:       item.Label,
			Type:        item.Type,
			Required:    item.Required,
			Secret:      item.Secret,
			Default:     item.Default,
			Options:     cloneStrings(item.Options),
			Description: item.Description,
		})
	}
	return out
}

func cloneStrings(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	out := make([]string, 0, len(input))
	for _, item := range input {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		out[trimmedKey] = strings.TrimSpace(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func lifecycleStatePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return path + ".state.yaml"
}

func loadLifecycleStateFile(path string) (map[string]LifecycleState, error) {
	if strings.TrimSpace(path) == "" {
		return map[string]LifecycleState{}, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]LifecycleState{}, nil
		}
		return nil, err
	}
	var raw lifecycleStateFile
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]LifecycleState, len(raw.Connectors.Entries))
	for _, entry := range raw.Connectors.Entries {
		id := strings.TrimSpace(entry.ConnectorID)
		if id == "" {
			continue
		}
		out[id] = cloneLifecycleState(entry)
	}
	return out, nil
}

func saveLifecycleStateFile(path string, state map[string]LifecycleState) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	var raw lifecycleStateFile
	ids := make([]string, 0, len(state))
	for id := range state {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		raw.Connectors.Entries = append(raw.Connectors.Entries, cloneLifecycleState(state[id]))
	}
	content, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return writeFileAtomically(path, string(content))
}

func syncLifecycleState(existing map[string]LifecycleState, previous *Config, current *Config, now time.Time) map[string]LifecycleState {
	state := cloneLifecycleMap(existing)
	currentByID := manifestMap(current)
	previousByID := manifestMap(previous)
	for id, entry := range currentByID {
		compatibility := CompatibilityReportForManifest(entry)
		compatibility.CheckedAt = now
		health := HealthStatusForManifest(entry, compatibility, now)
		fingerprint := manifestFingerprint(entry)
		item, found := state[id]
		if !found {
			item = LifecycleState{
				ConnectorID: id,
				InstalledAt: now,
			}
		}
		previousHealth := item.Health
		item.ConnectorID = id
		item.DisplayName = firstNonEmpty(entry.Metadata.DisplayName, entry.Metadata.Name, entry.Metadata.ID)
		item.CurrentVersion = entry.Metadata.Version
		item.AvailableVersion = firstNonEmpty(item.AvailableVersion, entry.Metadata.Version)
		item.Enabled = entry.Enabled()
		item.Runtime = RuntimeMetadataForManifest(entry)
		item.Compatibility = compatibility
		item.SecretRefs = cloneStringMap(entry.Config.SecretRefs)
		item.UpdatedAt = now
		if item.InstalledAt.IsZero() {
			item.InstalledAt = now
		}
		if !found {
			item.History = append(item.History, LifecycleEvent{
				Type:      "install",
				Summary:   "connector installed",
				Version:   entry.Metadata.Version,
				ToVersion: entry.Metadata.Version,
				Enabled:   boolPtr(entry.Enabled()),
				CreatedAt: now,
			})
			item.Revisions = appendRevision(item.Revisions, entry, now, "install")
		} else if previousEntry, ok := previousByID[id]; ok {
			switch {
			case previousEntry.Metadata.Version != entry.Metadata.Version:
				item.History = append(item.History, LifecycleEvent{
					Type:        lifecycleEventTypeForVersion(previousEntry.Metadata.Version, entry.Metadata.Version),
					Summary:     previousEntry.Metadata.Version + " -> " + entry.Metadata.Version,
					Version:     entry.Metadata.Version,
					FromVersion: previousEntry.Metadata.Version,
					ToVersion:   entry.Metadata.Version,
					Enabled:     boolPtr(entry.Enabled()),
					CreatedAt:   now,
				})
				item.Revisions = appendRevision(item.Revisions, entry, now, "upgrade")
			case previousEntry.Disabled != entry.Disabled:
				eventType := "disable"
				summary := "connector disabled"
				if entry.Enabled() {
					eventType = "enable"
					summary = "connector enabled"
				}
				item.History = append(item.History, LifecycleEvent{
					Type:      eventType,
					Summary:   summary,
					Version:   entry.Metadata.Version,
					Enabled:   boolPtr(entry.Enabled()),
					CreatedAt: now,
				})
			case item.LastFingerprint != "" && item.LastFingerprint != fingerprint:
				item.History = append(item.History, LifecycleEvent{
					Type:      "update",
					Summary:   "connector manifest updated",
					Version:   entry.Metadata.Version,
					ToVersion: entry.Metadata.Version,
					Enabled:   boolPtr(entry.Enabled()),
					CreatedAt: now,
				})
				item.Revisions = appendRevision(item.Revisions, entry, now, "update")
			}
		}
		healthChanged := previousHealth.Status != health.Status || previousHealth.Summary != health.Summary
		if shouldPreserveRecordedHealth(previousHealth, health, entry.Enabled(), compatibility.Compatible, item.LastFingerprint, fingerprint) {
			health = previousHealth
			healthChanged = false
		}
		item.Health = health
		if !found || healthChanged {
			item.HealthHistory = appendHealthHistory(item.HealthHistory, health)
		}
		item.LastFingerprint = fingerprint
		state[id] = item
	}
	for id := range state {
		if _, ok := currentByID[id]; !ok {
			delete(state, id)
		}
	}
	return state
}

func appendHealthHistory(history []HealthStatus, health HealthStatus) []HealthStatus {
	if health.Status == "" {
		return history
	}
	if len(history) > 0 {
		last := history[len(history)-1]
		if last.Status == health.Status && last.Summary == health.Summary {
			history[len(history)-1] = health
			return history
		}
	}
	history = append(history, health)
	if len(history) > 20 {
		return append([]HealthStatus(nil), history[len(history)-20:]...)
	}
	return history
}

func shouldPreserveRecordedHealth(previous HealthStatus, fallback HealthStatus, enabled bool, compatible bool, previousFingerprint string, currentFingerprint string) bool {
	if previous.Status == "" {
		return false
	}
	if !enabled || !compatible {
		return false
	}
	if previousFingerprint == "" || previousFingerprint != currentFingerprint {
		return false
	}
	if previous.Status != fallback.Status {
		return false
	}
	if previous.Summary == "" || previous.Summary == fallback.Summary {
		return false
	}
	if previous.CheckedAt.IsZero() {
		return false
	}
	return true
}

func manifestMap(cfg *Config) map[string]Manifest {
	if cfg == nil || len(cfg.Entries) == 0 {
		return map[string]Manifest{}
	}
	out := make(map[string]Manifest, len(cfg.Entries))
	for _, entry := range cfg.Entries {
		out[entry.Metadata.ID] = entry
	}
	return out
}

func lifecycleEventTypeForVersion(previous string, current string) string {
	if previous == "" {
		return "install"
	}
	if current == "" {
		return "update"
	}
	return "upgrade"
}

func manifestFingerprint(entry Manifest) string {
	encoded, err := EncodeConfig(&Config{Entries: []Manifest{entry}})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(encoded))
	return hex.EncodeToString(sum[:])
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}

func appendRevision(history []RevisionSnapshot, manifest Manifest, createdAt time.Time, reason string) []RevisionSnapshot {
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	for i := range history {
		if history[i].Version == manifest.Metadata.Version && history[i].Reason == reason && history[i].Manifest != nil && manifestFingerprint(*history[i].Manifest) == manifestFingerprint(manifest) {
			history[i].CreatedAt = createdAt
			history[i].Reason = reason
			return history
		}
	}
	cloned := cloneManifests([]Manifest{manifest})
	history = append(history, RevisionSnapshot{
		Version:   manifest.Metadata.Version,
		Manifest:  &cloned[0],
		CreatedAt: createdAt,
		Reason:    reason,
	})
	sort.SliceStable(history, func(i, j int) bool {
		return history[i].CreatedAt.Before(history[j].CreatedAt)
	})
	if len(history) > 10 {
		return append([]RevisionSnapshot(nil), history[len(history)-10:]...)
	}
	return history
}

func pendingRuntimeProbeHealth(entry Manifest, compatibility CompatibilityReport, checkedAt time.Time) HealthStatus {
	fallback := HealthStatusForManifest(entry, compatibility, checkedAt)
	if !entry.Enabled() || !compatibility.Compatible {
		return fallback
	}
	return HealthStatus{
		Status:    "unknown",
		Summary:   "runtime health check required after connector change",
		CheckedAt: checkedAt,
	}
}

func pruneRollbackAutoTransition(history []LifecycleEvent, fromVersion string, toVersion string) []LifecycleEvent {
	for i := len(history) - 1; i >= 0; i-- {
		item := history[i]
		if item.Type == "upgrade" && item.FromVersion == fromVersion && item.ToVersion == toVersion {
			return append(append([]LifecycleEvent(nil), history[:i]...), history[i+1:]...)
		}
	}
	return history
}

func pruneRollbackAutoRevision(history []RevisionSnapshot, target Manifest) []RevisionSnapshot {
	fingerprint := manifestFingerprint(target)
	for i := len(history) - 1; i >= 0; i-- {
		item := history[i]
		if item.Version != target.Metadata.Version || item.Reason != "upgrade" || item.Manifest == nil {
			continue
		}
		if manifestFingerprint(*item.Manifest) != fingerprint {
			continue
		}
		return append(append([]RevisionSnapshot(nil), history[:i]...), history[i+1:]...)
	}
	return history
}

func writeFileAtomically(path string, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "connectors-*.yaml")
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
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
