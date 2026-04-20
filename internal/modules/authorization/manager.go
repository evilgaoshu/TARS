package authorization

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrConfigPathNotSet = errors.New("authorization config path is not set")

type Snapshot struct {
	Path      string
	Content   string
	Config    Config
	UpdatedAt time.Time
	Loaded    bool
}

type Manager struct {
	mu        sync.RWMutex
	path      string
	content   string
	config    Config
	resolver  *Resolver
	persist   func(Config) error
	updatedAt time.Time
}

func NewManager(path string) (*Manager, error) {
	manager := &Manager{path: path}
	if path == "" {
		return manager, nil
	}
	if err := manager.Reload(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *Manager) EvaluateSSHCommand(input SSHCommandInput) Decision {
	if m == nil {
		return Decision{Action: ActionRequireApproval, RuleID: "default", MatchedBy: "default"}
	}

	m.mu.RLock()
	resolver := m.resolver
	m.mu.RUnlock()

	if resolver == nil {
		return Decision{Action: ActionRequireApproval, RuleID: "default", MatchedBy: "default"}
	}
	return resolver.EvaluateSSHCommand(input)
}

func (m *Manager) EvaluateCapability(input CapabilityInput) Decision {
	if m == nil {
		return Decision{Action: ActionRequireApproval, RuleID: "default", MatchedBy: "default"}
	}

	m.mu.RLock()
	resolver := m.resolver
	m.mu.RUnlock()

	if resolver == nil {
		if input.ReadOnly {
			return Decision{Action: ActionDirectExecute, RuleID: "capability_read_only", MatchedBy: "capability_read_only"}
		}
		return Decision{Action: ActionRequireApproval, RuleID: "default", MatchedBy: "default"}
	}
	return resolver.EvaluateCapability(input)
}

func (m *Manager) Snapshot() Snapshot {
	if m == nil {
		return Snapshot{}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return Snapshot{
		Path:      m.path,
		Content:   m.content,
		Config:    m.config,
		UpdatedAt: m.updatedAt,
		Loaded:    m.resolver != nil,
	}
}

func (m *Manager) Reload() error {
	if m == nil {
		return ErrConfigPathNotSet
	}

	m.mu.RLock()
	path := m.path
	m.mu.RUnlock()
	if path == "" {
		return ErrConfigPathNotSet
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return m.applyContent(string(content))
}

func (m *Manager) Save(content string) error {
	if m == nil {
		return ErrConfigPathNotSet
	}

	m.mu.RLock()
	path := m.path
	persist := m.persist
	m.mu.RUnlock()
	if path == "" && persist == nil {
		return ErrConfigPathNotSet
	}

	cfg, resolver, normalized, err := validateContent(content)
	if err != nil {
		return err
	}

	if path != "" {
		if err := writeFileAtomically(path, normalized); err != nil {
			return err
		}
	}

	m.mu.Lock()
	m.content = normalized
	m.config = cfg
	m.resolver = resolver
	m.updatedAt = time.Now().UTC()
	persist = m.persist
	m.mu.Unlock()
	if persist != nil {
		return persist(cfg)
	}
	return nil
}

func (m *Manager) SaveConfig(cfg Config) error {
	content, err := EncodeConfig(cfg)
	if err != nil {
		return err
	}
	return m.Save(content)
}

func (m *Manager) SetPersistence(persist func(Config) error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.persist = persist
	m.mu.Unlock()
}

func (m *Manager) LoadRuntimeConfig(cfg Config) error {
	content, err := EncodeConfig(cfg)
	if err != nil {
		return err
	}
	return m.applyContent(content)
}

func (m *Manager) applyContent(content string) error {
	cfg, resolver, normalized, err := validateContent(content)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.content = normalized
	m.config = cfg
	m.resolver = resolver
	m.updatedAt = time.Now().UTC()
	m.mu.Unlock()
	return nil
}

func validateContent(content string) (Config, *Resolver, string, error) {
	cfg, err := ParseConfig([]byte(content))
	if err != nil {
		return Config{}, nil, "", err
	}
	resolver := New(cfg)
	normalized, err := EncodeConfig(cfg)
	if err != nil {
		return Config{}, nil, "", err
	}
	return cfg, resolver, normalized, nil
}

func writeFileAtomically(path string, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "authorization-*.yaml")
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
