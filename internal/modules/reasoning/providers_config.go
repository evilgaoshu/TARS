package reasoning

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"tars/internal/foundation/secrets"

	"gopkg.in/yaml.v3"
)

var ErrProviderNotConfigured = errors.New("provider is not configured")

type ProviderBinding struct {
	ProviderID string
	Model      string
}

type ProviderEntry struct {
	ID          string
	Vendor      string
	Protocol    string
	BaseURL     string
	APIKey      string
	APIKeyRef   string
	OrgID       string
	TenantID    string
	WorkspaceID string
	Enabled     bool
	Templates   []ProviderTemplate
}

type ProviderTemplate struct {
	ID          string
	Name        string
	Description string
	Values      map[string]string
	CreatedAt   time.Time
}

type SecretReference struct {
	Ref       string
	Set       bool
	UpdatedAt time.Time
	Source    string
}

type ProvidersConfig struct {
	Primary ProviderBinding
	Assist  ProviderBinding
	Entries []ProviderEntry
}

type ModelTarget struct {
	ProviderID string
	Vendor     string
	Protocol   string
	BaseURL    string
	APIKey     string
	Model      string
}

type ProvidersSnapshot struct {
	Path      string
	Content   string
	Config    ProvidersConfig
	UpdatedAt time.Time
	Loaded    bool
}

type ProviderRegistry interface {
	ResolvePrimaryModelTarget() *ModelTarget
	ResolveAssistModelTarget() *ModelTarget
	ResolvePrimaryModelTargetWithSecrets(store *secrets.Store) *ModelTarget
	ResolveAssistModelTargetWithSecrets(store *secrets.Store) *ModelTarget
	Snapshot() ProvidersSnapshot
}

type providersFileConfig struct {
	Providers struct {
		Primary struct {
			ProviderID string `yaml:"provider_id,omitempty"`
			Model      string `yaml:"model,omitempty"`
		} `yaml:"primary"`
		Assist struct {
			ProviderID string `yaml:"provider_id,omitempty"`
			Model      string `yaml:"model,omitempty"`
		} `yaml:"assist"`
		Entries []struct {
			ID          string             `yaml:"id,omitempty"`
			Vendor      string             `yaml:"vendor,omitempty"`
			Protocol    string             `yaml:"protocol,omitempty"`
			BaseURL     string             `yaml:"base_url,omitempty"`
			APIKey      string             `yaml:"api_key,omitempty"`
			APIKeyRef   string             `yaml:"api_key_ref,omitempty"`
			OrgID       string             `yaml:"org_id,omitempty"`
			TenantID    string             `yaml:"tenant_id,omitempty"`
			WorkspaceID string             `yaml:"workspace_id,omitempty"`
			Enabled     *bool              `yaml:"enabled,omitempty"`
			Templates   []ProviderTemplate `yaml:"templates,omitempty"`
		} `yaml:"entries,omitempty"`
	} `yaml:"providers"`
}

type ProviderManager struct {
	mu        sync.RWMutex
	path      string
	content   string
	config    *ProvidersConfig
	persist   func(ProvidersConfig) error
	updatedAt time.Time
}

func DefaultProvidersConfig() ProvidersConfig {
	return ProvidersConfig{}
}

func NewProviderManager(path string) (*ProviderManager, error) {
	manager := &ProviderManager{path: path}
	if path == "" {
		cfg := DefaultProvidersConfig()
		manager.config = &cfg
		return manager, nil
	}
	if err := manager.Reload(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *ProviderManager) ResolvePrimaryModelTarget() *ModelTarget {
	return m.ResolvePrimaryModelTargetWithSecrets(nil)
}

func (m *ProviderManager) ResolvePrimaryModelTargetWithSecrets(store *secrets.Store) *ModelTarget {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return resolveBoundTarget(m.config, m.config.Primary, store)
}

func (m *ProviderManager) ResolveAssistModelTarget() *ModelTarget {
	return m.ResolveAssistModelTargetWithSecrets(nil)
}

func (m *ProviderManager) ResolveAssistModelTargetWithSecrets(store *secrets.Store) *ModelTarget {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return resolveBoundTarget(m.config, m.config.Assist, store)
}

func (m *ProviderManager) Snapshot() ProvidersSnapshot {
	if m == nil {
		return ProvidersSnapshot{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := ProvidersSnapshot{
		Path:      m.path,
		Content:   m.content,
		UpdatedAt: m.updatedAt,
		Loaded:    m.config != nil,
	}
	if m.config != nil {
		snapshot.Config = *m.config
		snapshot.Config.Entries = cloneProviderEntries(snapshot.Config.Entries, true)
	}
	return snapshot
}

func (m *ProviderManager) SecretRefs() []SecretReference {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]SecretReference, 0)
	seen := make(map[string]struct{})
	if m.config == nil {
		return items
	}
	for _, entry := range m.config.Entries {
		ref := strings.TrimSpace(entry.APIKeyRef)
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		items = append(items, SecretReference{Ref: ref, Set: true, Source: "provider_registry"})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Ref < items[j].Ref
	})
	return items
}

func (m *ProviderManager) Reload() error {
	if m == nil || strings.TrimSpace(m.path) == "" {
		return nil
	}
	content, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	cfg, normalized, err := ParseProvidersConfig(content)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.content = normalized
	m.config = cfg
	m.updatedAt = time.Now().UTC()
	m.mu.Unlock()
	return nil
}

func (m *ProviderManager) SetPersistence(persist func(ProvidersConfig) error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.persist = persist
	m.mu.Unlock()
}

func (m *ProviderManager) SaveConfig(cfg ProvidersConfig) error {
	if m == nil {
		return nil
	}
	if strings.TrimSpace(m.path) == "" {
		normalized := normalizeProvidersConfig(cfg)
		content, err := EncodeProvidersConfig(&normalized)
		if err != nil {
			return err
		}
		m.mu.Lock()
		m.content = content
		m.config = &normalized
		m.updatedAt = time.Now().UTC()
		persist := m.persist
		m.mu.Unlock()
		if persist != nil {
			return persist(normalizeProvidersConfig(normalized))
		}
		return nil
	}
	m.mu.RLock()
	existing := m.config
	m.mu.RUnlock()
	merged := mergeProviderSecrets(existing, &cfg)
	content, err := EncodeProvidersConfig(&merged)
	if err != nil {
		return err
	}
	return m.Save(content)
}

func (m *ProviderManager) Save(content string) error {
	if m == nil {
		return nil
	}
	cfg, normalized, err := ParseProvidersConfig([]byte(content))
	if err != nil {
		return err
	}
	if strings.TrimSpace(m.path) == "" {
		m.mu.Lock()
		m.content = normalized
		m.config = cfg
		m.updatedAt = time.Now().UTC()
		persist := m.persist
		m.mu.Unlock()
		if persist != nil && cfg != nil {
			return persist(normalizeProvidersConfig(*cfg))
		}
		return nil
	}
	m.mu.RLock()
	existing := m.config
	m.mu.RUnlock()
	if existing != nil {
		merged := mergeProviderSecrets(existing, cfg)
		cfg = &merged
		normalized, err = EncodeProvidersConfig(cfg)
		if err != nil {
			return err
		}
	}
	if err := writeProvidersFileAtomically(m.path, normalized); err != nil {
		return err
	}
	m.mu.Lock()
	m.content = normalized
	m.config = cfg
	m.updatedAt = time.Now().UTC()
	persist := m.persist
	m.mu.Unlock()
	if persist != nil && cfg != nil {
		return persist(normalizeProvidersConfig(*cfg))
	}
	return nil
}

func ParseProvidersConfig(content []byte) (*ProvidersConfig, string, error) {
	var raw providersFileConfig
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, "", err
	}
	cfg := DefaultProvidersConfig()
	cfg.Primary = ProviderBinding{
		ProviderID: strings.TrimSpace(raw.Providers.Primary.ProviderID),
		Model:      strings.TrimSpace(raw.Providers.Primary.Model),
	}
	cfg.Assist = ProviderBinding{
		ProviderID: strings.TrimSpace(raw.Providers.Assist.ProviderID),
		Model:      strings.TrimSpace(raw.Providers.Assist.Model),
	}
	for _, item := range raw.Providers.Entries {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		cfg.Entries = append(cfg.Entries, ProviderEntry{
			ID:          strings.TrimSpace(item.ID),
			Vendor:      strings.TrimSpace(item.Vendor),
			Protocol:    normalizeProviderProtocol(item.Protocol, item.Vendor),
			BaseURL:     strings.TrimSpace(item.BaseURL),
			APIKey:      strings.TrimSpace(item.APIKey),
			APIKeyRef:   strings.TrimSpace(item.APIKeyRef),
			OrgID:       strings.TrimSpace(item.OrgID),
			TenantID:    strings.TrimSpace(item.TenantID),
			WorkspaceID: strings.TrimSpace(item.WorkspaceID),
			Enabled:     item.Enabled == nil || *item.Enabled,
			Templates:   cloneProviderTemplates(item.Templates),
		})
	}
	cfg = normalizeProvidersConfig(cfg)
	encoded, err := EncodeProvidersConfig(&cfg)
	if err != nil {
		return nil, "", err
	}
	return &cfg, encoded, nil
}

func EncodeProvidersConfig(cfg *ProvidersConfig) (string, error) {
	current := DefaultProvidersConfig()
	if cfg != nil {
		current = normalizeProvidersConfig(*cfg)
	}
	var raw providersFileConfig
	raw.Providers.Primary.ProviderID = current.Primary.ProviderID
	raw.Providers.Primary.Model = current.Primary.Model
	raw.Providers.Assist.ProviderID = current.Assist.ProviderID
	raw.Providers.Assist.Model = current.Assist.Model
	for _, item := range current.Entries {
		entry := struct {
			ID          string             `yaml:"id,omitempty"`
			Vendor      string             `yaml:"vendor,omitempty"`
			Protocol    string             `yaml:"protocol,omitempty"`
			BaseURL     string             `yaml:"base_url,omitempty"`
			APIKey      string             `yaml:"api_key,omitempty"`
			APIKeyRef   string             `yaml:"api_key_ref,omitempty"`
			OrgID       string             `yaml:"org_id,omitempty"`
			TenantID    string             `yaml:"tenant_id,omitempty"`
			WorkspaceID string             `yaml:"workspace_id,omitempty"`
			Enabled     *bool              `yaml:"enabled,omitempty"`
			Templates   []ProviderTemplate `yaml:"templates,omitempty"`
		}{
			ID:          item.ID,
			Vendor:      item.Vendor,
			Protocol:    item.Protocol,
			BaseURL:     item.BaseURL,
			APIKey:      item.APIKey,
			APIKeyRef:   item.APIKeyRef,
			OrgID:       item.OrgID,
			TenantID:    item.TenantID,
			WorkspaceID: item.WorkspaceID,
			Enabled:     boolPtr(item.Enabled),
			Templates:   cloneProviderTemplates(item.Templates),
		}
		raw.Providers.Entries = append(raw.Providers.Entries, entry)
	}
	content, err := yaml.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func normalizeProvidersConfig(cfg ProvidersConfig) ProvidersConfig {
	out := ProvidersConfig{
		Primary: ProviderBinding{
			ProviderID: strings.TrimSpace(cfg.Primary.ProviderID),
			Model:      strings.TrimSpace(cfg.Primary.Model),
		},
		Assist: ProviderBinding{
			ProviderID: strings.TrimSpace(cfg.Assist.ProviderID),
			Model:      strings.TrimSpace(cfg.Assist.Model),
		},
		Entries: make([]ProviderEntry, 0, len(cfg.Entries)),
	}
	seen := make(map[string]struct{}, len(cfg.Entries))
	for _, item := range cfg.Entries {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out.Entries = append(out.Entries, ProviderEntry{
			ID:          id,
			Vendor:      normalizeProviderVendor(item.Vendor, item.Protocol),
			Protocol:    normalizeProviderProtocol(item.Protocol, item.Vendor),
			BaseURL:     strings.TrimSpace(item.BaseURL),
			APIKey:      strings.TrimSpace(item.APIKey),
			APIKeyRef:   strings.TrimSpace(item.APIKeyRef),
			OrgID:       strings.TrimSpace(item.OrgID),
			TenantID:    strings.TrimSpace(item.TenantID),
			WorkspaceID: strings.TrimSpace(item.WorkspaceID),
			Enabled:     item.Enabled,
			Templates:   cloneProviderTemplates(item.Templates),
		})
	}
	sort.SliceStable(out.Entries, func(i, j int) bool {
		return out.Entries[i].ID < out.Entries[j].ID
	})
	return out
}

func normalizeProviderProtocol(protocol string, vendor string) string {
	switch strings.ToLower(strings.TrimSpace(firstNonEmpty(protocol, vendor))) {
	case "", "openai", "openai_compatible", "openai-compatible":
		return ModelProtocolOpenAICompatible
	case "openrouter":
		return "openrouter"
	case "anthropic", "claude":
		return ModelProtocolAnthropic
	case "ollama":
		return ModelProtocolOllama
	case "lmstudio", "lm_studio", "lm-studio":
		return ModelProtocolLMStudio
	case "gemini":
		return "gemini"
	default:
		return strings.ToLower(strings.TrimSpace(firstNonEmpty(protocol, vendor)))
	}
}

func normalizeProviderVendor(vendor string, protocol string) string {
	switch strings.ToLower(strings.TrimSpace(firstNonEmpty(vendor, protocol))) {
	case "", "openai_compatible", "openai-compatible":
		return "openai"
	case "anthropic":
		return "claude"
	default:
		return strings.ToLower(strings.TrimSpace(firstNonEmpty(vendor, protocol)))
	}
}

func resolveBoundTarget(cfg *ProvidersConfig, binding ProviderBinding, store *secrets.Store) *ModelTarget {
	if cfg == nil || strings.TrimSpace(binding.ProviderID) == "" {
		return nil
	}
	for _, entry := range cfg.Entries {
		if entry.ID == binding.ProviderID && entry.Enabled {
			model := strings.TrimSpace(binding.Model)
			if model == "" {
				return nil
			}
			return &ModelTarget{
				ProviderID: entry.ID,
				Vendor:     entry.Vendor,
				Protocol:   entry.Protocol,
				BaseURL:    entry.BaseURL,
				APIKey:     resolveProviderAPIKey(entry, store),
				Model:      model,
			}
		}
	}
	return nil
}

func resolveProviderAPIKey(entry ProviderEntry, store *secrets.Store) string {
	if ref := strings.TrimSpace(entry.APIKeyRef); ref != "" && store != nil {
		if value, ok := store.Get(ref); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return entry.APIKey
}

func cloneProviderEntries(input []ProviderEntry, includeSecrets bool) []ProviderEntry {
	if len(input) == 0 {
		return nil
	}
	output := make([]ProviderEntry, 0, len(input))
	for _, item := range input {
		copy := item
		if !includeSecrets {
			copy.APIKey = ""
		}
		copy.Templates = cloneProviderTemplates(item.Templates)
		output = append(output, copy)
	}
	return output
}

func mergeProviderSecrets(existing *ProvidersConfig, incoming *ProvidersConfig) ProvidersConfig {
	if incoming == nil {
		return DefaultProvidersConfig()
	}
	merged := normalizeProvidersConfig(*incoming)
	if existing == nil {
		return merged
	}
	secrets := make(map[string]string, len(existing.Entries))
	refs := make(map[string]string, len(existing.Entries))
	for _, item := range existing.Entries {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.APIKey) == "" {
			if strings.TrimSpace(item.ID) != "" && strings.TrimSpace(item.APIKeyRef) != "" {
				refs[item.ID] = item.APIKeyRef
			}
			continue
		}
		secrets[item.ID] = item.APIKey
		if strings.TrimSpace(item.APIKeyRef) != "" {
			refs[item.ID] = item.APIKeyRef
		}
	}
	for i := range merged.Entries {
		if strings.TrimSpace(merged.Entries[i].APIKey) == "" {
			if secret, ok := secrets[merged.Entries[i].ID]; ok {
				merged.Entries[i].APIKey = secret
			}
		}
		if strings.TrimSpace(merged.Entries[i].APIKeyRef) == "" {
			if ref, ok := refs[merged.Entries[i].ID]; ok {
				merged.Entries[i].APIKeyRef = ref
			}
		}
	}
	return merged
}

func cloneProviderTemplates(input []ProviderTemplate) []ProviderTemplate {
	if len(input) == 0 {
		return nil
	}
	out := make([]ProviderTemplate, 0, len(input))
	for _, item := range input {
		cloned := item
		cloned.Values = cloneProviderStringMap(item.Values)
		out = append(out, cloned)
	}
	return out
}

func cloneProviderStringMap(input map[string]string) map[string]string {
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

func writeProvidersFileAtomically(path string, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "providers-*.yaml")
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
