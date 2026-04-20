package skills

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

var ErrConfigPathNotSet = errors.New("skills config path is not set")
var ErrSkillNotFound = errors.New("skill not found")

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
	Skills struct {
		Entries []Manifest `yaml:"entries,omitempty"`
	} `yaml:"skills"`
}

type lifecycleStateFile struct {
	Skills struct {
		Entries []LifecycleState `yaml:"entries,omitempty"`
	} `yaml:"skills"`
}

type Manager struct {
	mu             sync.RWMutex
	path           string
	statePath      string
	marketplaceDir string
	content        string
	config         *Config
	state          map[string]LifecycleState
	updatedAt      time.Time
}

type UpsertOptions struct {
	Manifest Manifest
	Reason   string
	Action   string
	Source   string
	Status   string
}

type PromoteOptions struct {
	OperatorReason string
	ReviewState    string
	RuntimeMode    string
}

type RollbackOptions struct {
	OperatorReason string
}

func DefaultConfig() Config {
	return Config{}
}

func NewManager(path string, marketplaceDir string) (*Manager, error) {
	manager := &Manager{path: strings.TrimSpace(path), statePath: lifecycleStatePath(path), marketplaceDir: strings.TrimSpace(marketplaceDir)}
	if manager.path == "" {
		cfg, err := loadMarketplaceConfig(manager.marketplaceDir)
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		state := syncLifecycleState(map[string]LifecycleState{}, nil, cfg, now)
		manager.config = cfg
		manager.state = state
		manager.updatedAt = now
		manager.content, _ = EncodeConfig(cfg)
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
	if m == nil {
		return nil
	}
	if strings.TrimSpace(m.path) == "" {
		cfg, err := loadMarketplaceConfig(m.marketplaceDir)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		state := syncLifecycleState(m.state, m.config, cfg, now)
		content, _ := EncodeConfig(cfg)
		m.mu.Lock()
		m.content = content
		m.config = cfg
		m.state = state
		m.updatedAt = now
		m.mu.Unlock()
		return nil
	}
	content, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg, loadErr := loadMarketplaceConfig(m.marketplaceDir)
			if loadErr != nil {
				return loadErr
			}
			now := time.Now().UTC()
			state := syncLifecycleState(m.state, m.config, cfg, now)
			encoded, _ := EncodeConfig(cfg)
			m.mu.Lock()
			m.content = encoded
			m.config = cfg
			m.state = state
			m.updatedAt = now
			m.mu.Unlock()
			return nil
		}
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

func (m *Manager) SaveConfig(cfg Config) error {
	if m == nil || strings.TrimSpace(m.path) == "" {
		return ErrConfigPathNotSet
	}
	content, err := EncodeConfig(&cfg)
	if err != nil {
		return err
	}
	return m.saveConfigAndState(&cfg, content)
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
	m.mu.Unlock()
	return nil
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
		return items[i].SkillID < items[j].SkillID
	})
	return items
}

func (m *Manager) Upsert(opts UpsertOptions) (Manifest, LifecycleState, error) {
	normalized, err := normalizeManifest(opts.Manifest)
	if err != nil {
		return Manifest{}, LifecycleState{}, err
	}
	if opts.Source != "" {
		normalized.Metadata.Source = strings.TrimSpace(opts.Source)
	}
	m.mu.RLock()
	current := DefaultConfig()
	if m.config != nil {
		current = cloneConfig(*m.config)
	}
	previousState := cloneLifecycleMap(m.state)
	m.mu.RUnlock()
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
	if strings.TrimSpace(m.path) != "" {
		if err := m.SaveConfig(current); err != nil {
			return Manifest{}, LifecycleState{}, err
		}
	} else {
		content, _ := EncodeConfig(&current)
		now := time.Now().UTC()
		state := syncLifecycleState(previousState, m.config, &current, now)
		m.mu.Lock()
		m.content = content
		m.config = &current
		m.state = state
		m.updatedAt = now
		m.mu.Unlock()
	}
	state, ok := m.GetLifecycle(normalized.Metadata.ID)
	if !ok {
		return Manifest{}, LifecycleState{}, ErrSkillNotFound
	}
	now := time.Now().UTC()
	state.Source = firstNonEmpty(strings.TrimSpace(opts.Source), state.Source, normalized.Metadata.Source, "custom")
	state.Status = firstNonEmpty(strings.TrimSpace(opts.Status), state.Status, "draft")
	state.ReviewState = firstNonEmpty(state.ReviewState, "pending")
	state.RuntimeMode = firstNonEmpty(state.RuntimeMode, "planner_visible")
	action := firstNonEmpty(strings.TrimSpace(opts.Action), "skill_updated")
	reason := firstNonEmpty(strings.TrimSpace(opts.Reason), "unspecified")
	state.History = append(state.History, LifecycleEvent{
		Type:      action,
		Summary:   reason,
		Metadata: map[string]string{
			"reason": reason,
			"source": state.Source,
		},
		CreatedAt: now,
	})
	state.UpdatedAt = now
	if !state.InstalledAt.IsZero() {
		// keep existing installed_at
	} else {
		state.InstalledAt = now
	}
	if err := m.persistLifecycleState(normalized.Metadata.ID, state); err != nil {
		return Manifest{}, LifecycleState{}, err
	}
	return normalized, state, nil
}

func (m *Manager) SetEnabled(id string, enabled bool, reason string) (Manifest, LifecycleState, error) {
	m.mu.RLock()
	current := DefaultConfig()
	if m.config != nil {
		current = cloneConfig(*m.config)
	}
	m.mu.RUnlock()
	for i := range current.Entries {
		if current.Entries[i].Metadata.ID == strings.TrimSpace(id) {
			current.Entries[i].Disabled = !enabled
			if strings.TrimSpace(m.path) != "" {
				if err := m.SaveConfig(current); err != nil {
					return Manifest{}, LifecycleState{}, err
				}
			} else {
				content, _ := EncodeConfig(&current)
				now := time.Now().UTC()
				m.mu.Lock()
				m.content = content
				m.config = &current
				m.updatedAt = now
				m.mu.Unlock()
			}
			entry, _ := m.Get(id)
			state, ok := m.GetLifecycle(id)
			if !ok {
				return Manifest{}, LifecycleState{}, ErrSkillNotFound
			}
			now := time.Now().UTC()
			state.Enabled = enabled
			if enabled {
				state.Status = "active"
			} else {
				state.Status = "disabled"
			}
			state.UpdatedAt = now
			state.History = append(state.History, LifecycleEvent{
				Type:      map[bool]string{true: "skill_enabled", false: "skill_disabled"}[enabled],
				Summary:   firstNonEmpty(strings.TrimSpace(reason), "unspecified"),
				Metadata: map[string]string{
					"reason": firstNonEmpty(strings.TrimSpace(reason), "unspecified"),
				},
				CreatedAt: now,
			})
			if err := m.persistLifecycleState(id, state); err != nil {
				return Manifest{}, LifecycleState{}, err
			}
			return entry, state, nil
		}
	}
	return Manifest{}, LifecycleState{}, ErrSkillNotFound
}

func (m *Manager) Promote(id string, opts PromoteOptions) (Manifest, LifecycleState, error) {
	entry, ok := m.Get(id)
	if !ok {
		return Manifest{}, LifecycleState{}, ErrSkillNotFound
	}
	state, ok := m.GetLifecycle(id)
	if !ok {
		return Manifest{}, LifecycleState{}, ErrSkillNotFound
	}
	now := time.Now().UTC()
	state.Status = "active"
	state.Enabled = true
	state.ReviewState = firstNonEmpty(strings.TrimSpace(opts.ReviewState), "approved")
	state.RuntimeMode = firstNonEmpty(strings.TrimSpace(opts.RuntimeMode), state.RuntimeMode, "planner_visible")
	state.PublishedAt = now
	state.UpdatedAt = now
	state.History = append(state.History, LifecycleEvent{
		Type:      "skill_promoted",
		Summary:   firstNonEmpty(strings.TrimSpace(opts.OperatorReason), "unspecified"),
		Metadata: map[string]string{
			"review_state": state.ReviewState,
			"runtime_mode": state.RuntimeMode,
			"reason":       firstNonEmpty(strings.TrimSpace(opts.OperatorReason), "unspecified"),
		},
		CreatedAt: now,
	})
	if err := m.persistLifecycleState(id, state); err != nil {
		return Manifest{}, LifecycleState{}, err
	}
	return entry, state, nil
}

func (m *Manager) Delete(id string, reason string) error {
	m.mu.Lock()
	if m.config == nil {
		m.mu.Unlock()
		return ErrSkillNotFound
	}

	found := false
	newEntries := make([]Manifest, 0, len(m.config.Entries))
	for _, e := range m.config.Entries {
		if e.Metadata.ID == id {
			found = true
			continue
		}
		newEntries = append(newEntries, e)
	}

	if !found {
		m.mu.Unlock()
		return ErrSkillNotFound
	}

	cfg := cloneConfig(*m.config)
	cfg.Entries = newEntries
	m.mu.Unlock()

	return m.saveConfigAndState(&cfg, "skill_deleted: "+reason)
}

func (m *Manager) Rollback(id string, opts RollbackOptions) (Manifest, LifecycleState, error) {
	currentEntry, ok := m.Get(id)
	if !ok {
		return Manifest{}, LifecycleState{}, ErrSkillNotFound
	}
	state, ok := m.GetLifecycle(id)
	if !ok {
		return Manifest{}, LifecycleState{}, ErrSkillNotFound
	}
	currentSignature := manifestSignature(currentEntry)
	var target *Manifest
	if len(state.Revisions) >= 2 {
		for idx := len(state.Revisions) - 2; idx >= 0; idx-- {
			revision := state.Revisions[idx]
			if revision.Manifest == nil {
				continue
			}
			// Roll back to the nearest material change
			if manifestSignature(*revision.Manifest) == currentSignature {
				continue
			}
			cloned := cloneManifests([]Manifest{*revision.Manifest})
			target = &cloned[0]
			break
		}
	}
	if target == nil {
		return Manifest{}, LifecycleState{}, ErrSkillNotFound
	}
	_, _, err := m.Upsert(UpsertOptions{
		Manifest: *target,
		Reason:   firstNonEmpty(strings.TrimSpace(opts.OperatorReason), "rollback"),
		Action:   "skill_rolled_back",
		Source:   firstNonEmpty(target.Metadata.Source, state.Source),
		Status:   firstNonEmpty(state.Status, "active"),
	})
	if err != nil {
		return Manifest{}, LifecycleState{}, err
	}
	updated, _ := m.Get(id)
	updatedState, _ := m.GetLifecycle(id)
	updatedState.PublishedAt = time.Now().UTC()
	if err := m.persistLifecycleState(id, updatedState); err != nil {
		return Manifest{}, LifecycleState{}, err
	}
	return updated, updatedState, nil
}

func (m *Manager) persistLifecycleState(id string, state LifecycleState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		m.state = map[string]LifecycleState{}
	}
	m.state[id] = cloneLifecycleState(state)
	if strings.TrimSpace(m.statePath) == "" {
		return nil
	}
	return saveLifecycleStateFile(m.statePath, m.state)
}

func ParseConfig(content []byte) (*Config, string, error) {
	var raw fileConfig
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, "", err
	}
	cfg := DefaultConfig()
	for _, entry := range raw.Skills.Entries {
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

func EncodeConfig(cfg *Config) (string, error) {
	current := DefaultConfig()
	if cfg != nil {
		current = normalizeConfig(*cfg)
	}
	var raw fileConfig
	raw.Skills.Entries = cloneManifests(current.Entries)
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
		entry.APIVersion = "tars.skill/v1alpha1"
	}
	entry.Kind = strings.TrimSpace(entry.Kind)
	if entry.Kind == "" {
		entry.Kind = "skill_package"
	}
	entry.Metadata.ID = strings.TrimSpace(entry.Metadata.ID)
	entry.Metadata.Name = strings.TrimSpace(entry.Metadata.Name)
	entry.Metadata.DisplayName = strings.TrimSpace(entry.Metadata.DisplayName)
	entry.Metadata.Description = strings.TrimSpace(entry.Metadata.Description)
	entry.Metadata.Tags = cloneStrings(entry.Metadata.Tags)
	entry.Metadata.Content = strings.TrimSpace(entry.Metadata.Content)
	entry.Spec.Governance.ExecutionPolicy = strings.TrimSpace(entry.Spec.Governance.ExecutionPolicy)
	entry.Compatibility = CompatibilityReportForManifest(entry)
	entry.Spec.Governance.ConnectorPreference.Metrics = cloneStrings(entry.Spec.Governance.ConnectorPreference.Metrics)
	entry.Spec.Governance.ConnectorPreference.Execution = cloneStrings(entry.Spec.Governance.ConnectorPreference.Execution)
	entry.Spec.Governance.ConnectorPreference.Observability = cloneStrings(entry.Spec.Governance.ConnectorPreference.Observability)
	entry.Spec.Governance.ConnectorPreference.Delivery = cloneStrings(entry.Spec.Governance.ConnectorPreference.Delivery)
	entry.Marketplace.Tags = cloneStrings(entry.Marketplace.Tags)
	entry.Marketplace.Source = strings.TrimSpace(entry.Marketplace.Source)
	if entry.Metadata.DisplayName == "" {
		entry.Metadata.DisplayName = firstNonEmpty(entry.Metadata.Name, entry.Metadata.ID)
	}
	if entry.Metadata.Name == "" {
		entry.Metadata.Name = firstNonEmpty(entry.Metadata.ID, entry.Metadata.DisplayName)
	}
	if entry.Metadata.Source == "" {
		entry.Metadata.Source = firstNonEmpty(entry.Marketplace.Source, "official")
	}
	if err := ValidateManifest(entry); err != nil {
		return Manifest{}, err
	}
	return entry, nil
}

func ValidateManifest(entry Manifest) error {
	switch strings.TrimSpace(entry.APIVersion) {
	case "", "tars.skill/v1alpha1", "tars.marketplace/v1alpha1":
	default:
		return errors.New("skill api_version must be tars.skill/v1alpha1 or tars.marketplace/v1alpha1")
	}
	if kind := strings.TrimSpace(entry.Kind); kind != "skill_package" {
		return errors.New("skill kind must be skill_package")
	}
	if strings.TrimSpace(entry.Metadata.ID) == "" {
		return errors.New("metadata.id is required")
	}
	return nil
}

func loadMarketplaceConfig(dir string) (*Config, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(dir) == "" {
		return &cfg, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		packagePath := filepath.Join(dir, entry.Name(), "package.yaml")
		content, err := os.ReadFile(packagePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		manifest, _, err := ParseManifest(content)
		if err != nil {
			return nil, err
		}
		cfg.Entries = append(cfg.Entries, *manifest)
	}
	normalized := normalizeConfig(cfg)
	return &normalized, nil
}

func ParseManifest(content []byte) (*Manifest, string, error) {
	var manifest Manifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return nil, "", err
	}
	normalized, err := normalizeManifest(manifest)
	if err != nil {
		return nil, "", err
	}
	bytes, err := yaml.Marshal(normalized)
	if err != nil {
		return nil, "", err
	}
	return &normalized, string(bytes), nil
}

func cloneConfig(cfg Config) Config {
	return Config{Entries: cloneManifests(cfg.Entries)}
}

func cloneManifests(input []Manifest) []Manifest {
	if len(input) == 0 {
		return nil
	}
	out := make([]Manifest, 0, len(input))
	for _, item := range input {
		cloned := item
		cloned.Metadata.Tags = cloneStrings(item.Metadata.Tags)
		cloned.Metadata.Content = item.Metadata.Content
		cloned.Spec.Governance.ConnectorPreference.Metrics = cloneStrings(item.Spec.Governance.ConnectorPreference.Metrics)
		cloned.Spec.Governance.ConnectorPreference.Execution = cloneStrings(item.Spec.Governance.ConnectorPreference.Execution)
		cloned.Spec.Governance.ConnectorPreference.Observability = cloneStrings(item.Spec.Governance.ConnectorPreference.Observability)
		cloned.Spec.Governance.ConnectorPreference.Delivery = cloneStrings(item.Spec.Governance.ConnectorPreference.Delivery)
		cloned.Metadata.Tags = cloneStrings(item.Metadata.Tags)
		cloned.Marketplace.Tags = cloneStrings(item.Marketplace.Tags)
		out = append(out, cloned)
	}
	return out
}

func cloneInterfaceMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			out[trimmed] = value
		}
	}
	return out
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
	output.History = cloneLifecycleEvents(input.History)
	output.Revisions = cloneRevisionSnapshots(input.Revisions)
	return output
}

func cloneLifecycleEvents(input []LifecycleEvent) []LifecycleEvent {
	if len(input) == 0 {
		return nil
	}
	out := make([]LifecycleEvent, 0, len(input))
	for _, item := range input {
		cloned := item
		if len(item.Metadata) > 0 {
			cloned.Metadata = make(map[string]string, len(item.Metadata))
			for key, value := range item.Metadata {
				cloned.Metadata[key] = value
			}
		}
		out = append(out, cloned)
	}
	return out
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

func manifestSignature(manifest Manifest) string {
	normalized, err := normalizeManifest(manifest)
	if err != nil {
		normalized = manifest
	}
	payload, err := yaml.Marshal(normalized)
	if err != nil {
		return normalized.Metadata.ID
	}
	return string(payload)
}

func appendRevision(existing []RevisionSnapshot, manifest Manifest, createdAt time.Time, action string, reason string) []RevisionSnapshot {
	updated := cloneRevisionSnapshots(existing)
	updated = append(updated, RevisionSnapshot{
		Manifest:  &cloneManifests([]Manifest{manifest})[0],
		CreatedAt: createdAt,
		Reason:    reason,
		Action:    action,
	})
	if len(updated) > 20 {
		updated = updated[len(updated)-20:]
	}
	return updated
}

func syncLifecycleState(existing map[string]LifecycleState, previous *Config, current *Config, now time.Time) map[string]LifecycleState {
	out := cloneLifecycleMap(existing)
	if out == nil {
		out = map[string]LifecycleState{}
	}
	prevByID := map[string]Manifest{}
	if previous != nil {
		for _, item := range previous.Entries {
			prevByID[item.Metadata.ID] = item
		}
	}
	if current == nil {
		return out
	}
	for _, item := range current.Entries {
		state, ok := out[item.Metadata.ID]
		if !ok {
			state = LifecycleState{
				SkillID:        item.Metadata.ID,
				DisplayName:    firstNonEmpty(item.Metadata.DisplayName, item.Metadata.Name, item.Metadata.ID),
				Source:         firstNonEmpty(item.Metadata.Source, item.Marketplace.Source, "official"),
				Status:         lifecycleStatusForManifest(item),
				ReviewState:    "pending",
				RuntimeMode:    "planner_visible",
				Enabled:        item.Enabled(),
				InstalledAt:    now,
				UpdatedAt:      now,
				History: []LifecycleEvent{{
					Type:    "skill_installed",
					Summary: "skill registered",
					CreatedAt: now,
				}},
				Revisions: appendRevision(nil, item, now, "skill_installed", "skill registered"),
			}
			out[item.Metadata.ID] = state
			continue
		}
		state.DisplayName = firstNonEmpty(item.Metadata.DisplayName, item.Metadata.Name, item.Metadata.ID)
		state.Source = firstNonEmpty(item.Metadata.Source, item.Marketplace.Source, state.Source, "official")
		state.Enabled = item.Enabled()
		if strings.TrimSpace(state.Status) == "" {
			state.Status = lifecycleStatusForManifest(item)
		}
		state.UpdatedAt = now
		if previousItem, existed := prevByID[item.Metadata.ID]; existed {
			// Detection of changes via fingerprint instead of version
			if manifestSignature(previousItem) != manifestSignature(item) {
				state.History = append(state.History, LifecycleEvent{
					Type:      "skill_revision_loaded",
					Summary:   "skill content updated",
					CreatedAt: now,
				})
				state.Revisions = appendRevision(state.Revisions, item, now, "skill_revision_loaded", "skill content updated")
			}
		}
		out[item.Metadata.ID] = state
	}
	return out
}

func lifecycleStatusForManifest(item Manifest) string {
	if item.Enabled() {
		return "active"
	}
	return "disabled"
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
		if os.IsNotExist(err) {
			return map[string]LifecycleState{}, nil
		}
		return nil, err
	}
	var raw lifecycleStateFile
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]LifecycleState, len(raw.Skills.Entries))
	for _, item := range raw.Skills.Entries {
		out[item.SkillID] = cloneLifecycleState(item)
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
		raw.Skills.Entries = append(raw.Skills.Entries, cloneLifecycleState(state[id]))
	}
	content, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return writeFileAtomically(path, string(content))
}

func writeFileAtomically(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".skills-*.tmp")
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
