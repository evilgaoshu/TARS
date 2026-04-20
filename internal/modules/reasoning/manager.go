package reasoning

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrPromptsConfigPathNotSet = errors.New("reasoning prompts config path is not set")

type PromptSnapshot struct {
	Path      string
	Content   string
	PromptSet PromptSet
	UpdatedAt time.Time
	Loaded    bool
}

type PromptManager struct {
	mu        sync.RWMutex
	path      string
	content   string
	promptSet *PromptSet
	persist   func(PromptSet) error
	updatedAt time.Time
}

func NewPromptManager(path string) (*PromptManager, error) {
	manager := &PromptManager{path: path}
	if path == "" {
		manager.promptSet = DefaultPromptSet()
		return manager, nil
	}
	if err := manager.Reload(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (m *PromptManager) CurrentPromptSet() *PromptSet {
	if m == nil {
		return DefaultPromptSet()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return defaultPromptSet(m.promptSet)
}

func (m *PromptManager) Snapshot() PromptSnapshot {
	if m == nil {
		return PromptSnapshot{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := PromptSnapshot{
		Path:      m.path,
		Content:   m.content,
		UpdatedAt: m.updatedAt,
		Loaded:    m.promptSet != nil,
	}
	if m.promptSet != nil {
		snapshot.PromptSet = *m.promptSet
	}
	return snapshot
}

func (m *PromptManager) Reload() error {
	if m == nil {
		return ErrPromptsConfigPathNotSet
	}
	m.mu.RLock()
	path := m.path
	m.mu.RUnlock()
	if path == "" {
		return ErrPromptsConfigPathNotSet
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return m.applyContent(string(content))
}

func (m *PromptManager) SavePromptSet(promptSet PromptSet) error {
	content, err := EncodePromptSet(&promptSet)
	if err != nil {
		return err
	}
	return m.Save(content)
}

func (m *PromptManager) Save(content string) error {
	if m == nil {
		return ErrPromptsConfigPathNotSet
	}
	m.mu.RLock()
	path := m.path
	persist := m.persist
	m.mu.RUnlock()
	if path == "" && persist == nil {
		return ErrPromptsConfigPathNotSet
	}
	promptSet, err := ParsePromptSet([]byte(content))
	if err != nil {
		return err
	}
	normalized, err := EncodePromptSet(promptSet)
	if err != nil {
		return err
	}
	if path != "" {
		if err := writePromptFileAtomically(path, normalized); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.content = normalized
	m.promptSet = promptSet
	m.updatedAt = time.Now().UTC()
	persist = m.persist
	m.mu.Unlock()
	if persist != nil {
		return persist(*promptSet)
	}
	return nil
}

func (m *PromptManager) applyContent(content string) error {
	promptSet, err := ParsePromptSet([]byte(content))
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.content = content
	m.promptSet = promptSet
	m.updatedAt = time.Now().UTC()
	m.mu.Unlock()
	return nil
}

func (m *PromptManager) SetPersistence(persist func(PromptSet) error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.persist = persist
	m.mu.Unlock()
}

func (m *PromptManager) LoadRuntimePromptSet(promptSet PromptSet) error {
	content, err := EncodePromptSet(&promptSet)
	if err != nil {
		return err
	}
	return m.applyContent(content)
}

func writePromptFileAtomically(path string, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "reasoning-prompts-*.yaml")
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
