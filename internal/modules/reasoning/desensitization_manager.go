package reasoning

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrDesensitizationConfigPathNotSet = errors.New("desensitization config path is not set")

type DesensitizationSnapshot struct {
	Path      string
	Content   string
	Config    DesensitizationConfig
	UpdatedAt time.Time
	Loaded    bool
}

type DesensitizationProvider interface {
	CurrentDesensitizationConfig() *DesensitizationConfig
}

type DesensitizationManager struct {
	mu        sync.RWMutex
	path      string
	content   string
	config    *DesensitizationConfig
	persist   func(DesensitizationConfig) error
	updatedAt time.Time
}

func NewDesensitizationManager(path string) (*DesensitizationManager, error) {
	manager := &DesensitizationManager{path: path}
	if path == "" {
		cfg := DefaultDesensitizationConfig()
		manager.config = &cfg
		return manager, nil
	}
	if err := manager.Reload(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *DesensitizationManager) CurrentDesensitizationConfig() *DesensitizationConfig {
	if m == nil {
		cfg := DefaultDesensitizationConfig()
		return &cfg
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config == nil {
		cfg := DefaultDesensitizationConfig()
		return &cfg
	}
	cfg := *m.config
	return &cfg
}

func (m *DesensitizationManager) Snapshot() DesensitizationSnapshot {
	if m == nil {
		return DesensitizationSnapshot{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := DesensitizationSnapshot{
		Path:      m.path,
		Content:   m.content,
		UpdatedAt: m.updatedAt,
		Loaded:    m.config != nil,
	}
	if m.config != nil {
		snapshot.Config = *m.config
	}
	return snapshot
}

func (m *DesensitizationManager) Reload() error {
	if m == nil {
		return ErrDesensitizationConfigPathNotSet
	}
	m.mu.RLock()
	path := m.path
	m.mu.RUnlock()
	if path == "" {
		return ErrDesensitizationConfigPathNotSet
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return m.applyContent(string(content))
}

func (m *DesensitizationManager) SaveConfig(cfg DesensitizationConfig) error {
	content, err := EncodeDesensitizationConfig(&cfg)
	if err != nil {
		return err
	}
	return m.Save(content)
}

func (m *DesensitizationManager) Save(content string) error {
	if m == nil {
		return ErrDesensitizationConfigPathNotSet
	}
	m.mu.RLock()
	path := m.path
	persist := m.persist
	m.mu.RUnlock()
	if path == "" && persist == nil {
		return ErrDesensitizationConfigPathNotSet
	}
	cfg, err := ParseDesensitizationConfig([]byte(content))
	if err != nil {
		return err
	}
	normalized, err := EncodeDesensitizationConfig(cfg)
	if err != nil {
		return err
	}
	if path != "" {
		if err := writeDesensitizationFileAtomically(path, normalized); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.content = normalized
	m.config = cfg
	m.updatedAt = time.Now().UTC()
	persist = m.persist
	m.mu.Unlock()
	if persist != nil {
		return persist(*cfg)
	}
	return nil
}

func (m *DesensitizationManager) applyContent(content string) error {
	cfg, err := ParseDesensitizationConfig([]byte(content))
	if err != nil {
		return err
	}
	normalized, err := EncodeDesensitizationConfig(cfg)
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

func (m *DesensitizationManager) SetPersistence(persist func(DesensitizationConfig) error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.persist = persist
	m.mu.Unlock()
}

func (m *DesensitizationManager) LoadRuntimeConfig(cfg DesensitizationConfig) error {
	content, err := EncodeDesensitizationConfig(&cfg)
	if err != nil {
		return err
	}
	return m.applyContent(content)
}

func writeDesensitizationFileAtomically(path string, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "desensitization-*.yaml")
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
