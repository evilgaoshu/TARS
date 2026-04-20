package secrets

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

var ErrSecretRefRequired = errors.New("secret ref is required")

type Metadata struct {
	Ref       string    `json:"ref,omitempty" yaml:"ref,omitempty"`
	Set       bool      `json:"set" yaml:"set"`
	UpdatedAt time.Time `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	Source    string    `json:"source,omitempty" yaml:"source,omitempty"`
}

type Descriptor struct {
	Ref       string    `json:"ref,omitempty"`
	OwnerType string    `json:"owner_type,omitempty"`
	OwnerID   string    `json:"owner_id,omitempty"`
	Key       string    `json:"key,omitempty"`
	Set       bool      `json:"set"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Source    string    `json:"source,omitempty"`
}

type Entry struct {
	Ref       string    `yaml:"ref,omitempty"`
	Value     string    `yaml:"value,omitempty"`
	UpdatedAt time.Time `yaml:"updated_at,omitempty"`
	Source    string    `yaml:"source,omitempty"`
}

type Snapshot struct {
	Path      string
	UpdatedAt time.Time
	Entries   map[string]Metadata
	Loaded    bool
}

type fileConfig struct {
	Secrets struct {
		Entries []Entry `yaml:"entries,omitempty"`
	} `yaml:"secrets"`
}

type Store struct {
	mu        sync.RWMutex
	path      string
	entries   map[string]Entry
	updatedAt time.Time
	loaded    bool
}

func ResolveValues(store *Store, values map[string]string, refs map[string]string) map[string]string {
	if len(values) == 0 && len(refs) == 0 {
		return nil
	}
	out := make(map[string]string, len(values)+len(refs))
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		out[trimmedKey] = value
	}
	for key, ref := range refs {
		trimmedKey := strings.TrimSpace(key)
		trimmedRef := strings.TrimSpace(ref)
		if trimmedKey == "" || trimmedRef == "" || store == nil {
			continue
		}
		if value, ok := store.Get(trimmedRef); ok {
			out[trimmedKey] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func NewStore(path string) (*Store, error) {
	store := &Store{
		path:    strings.TrimSpace(path),
		entries: map[string]Entry{},
	}
	if store.path == "" {
		return store, nil
	}
	if err := store.Reload(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			store.loaded = true
			return store, nil
		}
		return nil, err
	}
	return store, nil
}

func (s *Store) Reload() error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil
	}
	content, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	entries, err := parse(content)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.entries = entries
	s.updatedAt = time.Now().UTC()
	s.loaded = true
	s.mu.Unlock()
	return nil
}

func (s *Store) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := Snapshot{
		Path:      s.path,
		UpdatedAt: s.updatedAt,
		Entries:   make(map[string]Metadata, len(s.entries)),
		Loaded:    s.loaded,
	}
	for ref, entry := range s.entries {
		out.Entries[ref] = Metadata{
			Ref:       ref,
			Set:       strings.TrimSpace(entry.Value) != "",
			UpdatedAt: entry.UpdatedAt,
			Source:    firstNonEmpty(entry.Source, "secret_store"),
		}
	}
	return out
}

func (s *Store) Get(ref string) (string, bool) {
	if s == nil {
		return "", false
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[ref]
	if !ok {
		return "", false
	}
	return entry.Value, true
}

func (s *Store) Metadata(ref string) (Metadata, bool) {
	if s == nil {
		return Metadata{}, false
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return Metadata{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[ref]
	if !ok {
		return Metadata{}, false
	}
	return Metadata{
		Ref:       ref,
		Set:       strings.TrimSpace(entry.Value) != "",
		UpdatedAt: entry.UpdatedAt,
		Source:    firstNonEmpty(entry.Source, "secret_store"),
	}, true
}

func (s *Store) Apply(upserts map[string]string, deletes []string, updatedAt time.Time) (map[string]Metadata, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return map[string]Metadata{}, nil
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = map[string]Entry{}
	}
	for _, ref := range deletes {
		delete(s.entries, strings.TrimSpace(ref))
	}
	metadata := make(map[string]Metadata, len(upserts))
	for ref, value := range upserts {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return nil, ErrSecretRefRequired
		}
		s.entries[ref] = Entry{
			Ref:       ref,
			Value:     value,
			UpdatedAt: updatedAt,
			Source:    "secret_store",
		}
		metadata[ref] = Metadata{
			Ref:       ref,
			Set:       strings.TrimSpace(value) != "",
			UpdatedAt: updatedAt,
			Source:    "secret_store",
		}
	}
	if err := save(s.path, s.entries); err != nil {
		return nil, err
	}
	s.updatedAt = updatedAt
	s.loaded = true
	return metadata, nil
}

func parse(content []byte) (map[string]Entry, error) {
	var raw fileConfig
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, err
	}
	entries := make(map[string]Entry, len(raw.Secrets.Entries))
	for _, entry := range raw.Secrets.Entries {
		ref := strings.TrimSpace(entry.Ref)
		if ref == "" {
			continue
		}
		entries[ref] = Entry{
			Ref:       ref,
			Value:     entry.Value,
			UpdatedAt: entry.UpdatedAt,
			Source:    firstNonEmpty(entry.Source, "secret_store"),
		}
	}
	return entries, nil
}

func save(path string, entries map[string]Entry) error {
	var raw fileConfig
	refs := make([]string, 0, len(entries))
	for ref := range entries {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		entry := entries[ref]
		raw.Secrets.Entries = append(raw.Secrets.Entries, Entry{
			Ref:       ref,
			Value:     entry.Value,
			UpdatedAt: entry.UpdatedAt,
			Source:    firstNonEmpty(entry.Source, "secret_store"),
		})
	}
	content, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return writeFileAtomically(path, string(content))
}

func writeFileAtomically(path string, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "secrets-*.yaml")
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
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
