package reasoning

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"tars/internal/foundation/secrets"
)

func TestEncodePromptSetUsesDefaultsForNilInput(t *testing.T) {
	t.Parallel()

	content, err := EncodePromptSet(nil)
	if err != nil {
		t.Fatalf("encode default prompt set: %v", err)
	}

	parsed, err := ParsePromptSet([]byte(content))
	if err != nil {
		t.Fatalf("parse encoded prompt set: %v", err)
	}

	want := DefaultPromptSet()
	if parsed.SystemPrompt != want.SystemPrompt {
		t.Fatalf("unexpected system prompt: got %q want %q", parsed.SystemPrompt, want.SystemPrompt)
	}
	if parsed.UserPromptTemplate != want.UserPromptTemplate {
		t.Fatalf("unexpected user prompt template: got %q want %q", parsed.UserPromptTemplate, want.UserPromptTemplate)
	}
}

func TestPromptManagerDefaultsSnapshotAndCopy(t *testing.T) {
	t.Parallel()

	m, err := NewPromptManager("")
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}

	want := DefaultPromptSet()
	current := m.CurrentPromptSet()
	if current.SystemPrompt != want.SystemPrompt {
		t.Fatalf("unexpected default system prompt: got %q want %q", current.SystemPrompt, want.SystemPrompt)
	}
	if current.UserPromptTemplate != want.UserPromptTemplate {
		t.Fatalf("unexpected default user prompt template: got %q want %q", current.UserPromptTemplate, want.UserPromptTemplate)
	}

	snapshot := m.Snapshot()
	if !snapshot.Loaded || snapshot.Path != "" || snapshot.Content != "" || !reflect.DeepEqual(snapshot.PromptSet, *want) {
		t.Fatalf("unexpected default snapshot: %+v", snapshot)
	}
	snapshot.PromptSet.SystemPrompt = "mutated"
	if got := m.CurrentPromptSet().SystemPrompt; got != want.SystemPrompt {
		t.Fatalf("manager prompt set was mutated through snapshot: got %q want %q", got, want.SystemPrompt)
	}
}

func TestPromptManagerWithoutPathPersistsRuntimePromptSet(t *testing.T) {
	t.Parallel()

	m, err := NewPromptManager("")
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}

	var persisted PromptSet
	m.SetPersistence(func(cfg PromptSet) error {
		persisted = cfg
		return nil
	})

	next := PromptSet{
		SystemPrompt:       "  runtime system  ",
		UserPromptTemplate: "  session={{ .SessionID }}  ",
	}
	if err := m.SavePromptSet(next); err != nil {
		t.Fatalf("save runtime prompt set: %v", err)
	}
	if got := m.CurrentPromptSet(); got.SystemPrompt != "runtime system" || got.UserPromptTemplate != "session={{ .SessionID }}" {
		t.Fatalf("unexpected current prompt set: %+v", got)
	}
	if persisted.SystemPrompt != "runtime system" || persisted.UserPromptTemplate != "session={{ .SessionID }}" {
		t.Fatalf("unexpected persisted prompt set: %+v", persisted)
	}

	if err := m.LoadRuntimePromptSet(PromptSet{SystemPrompt: "loaded system", UserPromptTemplate: "loaded={{ .SessionID }}"}); err != nil {
		t.Fatalf("load runtime prompt set: %v", err)
	}
	if got := m.CurrentPromptSet(); got.SystemPrompt != "loaded system" || got.UserPromptTemplate != "loaded={{ .SessionID }}" {
		t.Fatalf("unexpected loaded runtime prompt set: %+v", got)
	}
}

func TestPromptManagerSaveAndReloadRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.yaml")
	initial := strings.TrimSpace(`
reasoning:
  system_prompt: |
    initial system
  user_prompt_template: |
    sid={{ .SessionID }}
`)
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("write prompt config: %v", err)
	}

	m, err := NewPromptManager(path)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}

	if got := m.CurrentPromptSet(); got.SystemPrompt != "initial system" {
		t.Fatalf("unexpected loaded prompt set: %+v", got)
	}
	if got := m.CurrentPromptSet(); got.UserPromptTemplate != "sid={{ .SessionID }}" {
		t.Fatalf("unexpected loaded prompt template: %+v", got)
	}
	if got := m.Snapshot(); got.Content != initial {
		t.Fatalf("unexpected snapshot content: got %q want %q", got.Content, initial)
	}

	next := PromptSet{
		SystemPrompt:       "  updated system  ",
		UserPromptTemplate: "  session={{ .SessionID }}  ",
	}
	if err := m.SavePromptSet(next); err != nil {
		t.Fatalf("save prompt set: %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved prompt config: %v", err)
	}
	if !strings.Contains(string(saved), "updated system") {
		t.Fatalf("saved prompt config was not written: %s", string(saved))
	}
	if !strings.Contains(string(saved), "session={{ .SessionID }}") {
		t.Fatalf("saved prompt config was not normalized: %s", string(saved))
	}

	snapshot := m.Snapshot()
	if snapshot.Content != string(saved) {
		t.Fatalf("snapshot content did not track save: got %q want %q", snapshot.Content, string(saved))
	}

	rewritten := strings.TrimSpace(`
reasoning:
  system_prompt: |
    reloaded system
  user_prompt_template: |
    sid={{ .SessionID }}
    ctx={{ .ContextJSON }}
`)
	if err := os.WriteFile(path, []byte(rewritten), 0o600); err != nil {
		t.Fatalf("rewrite prompt config: %v", err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("reload prompt config: %v", err)
	}

	if got := m.CurrentPromptSet(); got.SystemPrompt != "reloaded system" {
		t.Fatalf("unexpected reloaded prompt set: %+v", got)
	}
	if got := m.CurrentPromptSet(); got.UserPromptTemplate != "sid={{ .SessionID }}\nctx={{ .ContextJSON }}" {
		t.Fatalf("unexpected reloaded prompt template: %+v", got)
	}
	if got := m.Snapshot(); got.Content != rewritten {
		t.Fatalf("reload did not update snapshot content: got %q want %q", got.Content, rewritten)
	}
}

func TestPromptManagerNilReceiverReturnsDefaults(t *testing.T) {
	t.Parallel()

	var m *PromptManager
	if got := m.CurrentPromptSet(); got.SystemPrompt != DefaultPromptSet().SystemPrompt {
		t.Fatalf("nil prompt manager did not return defaults: %+v", got)
	}
	if got := m.CurrentPromptSet(); got.UserPromptTemplate != DefaultPromptSet().UserPromptTemplate {
		t.Fatalf("nil prompt manager did not return default template: %+v", got)
	}
	if got := m.Snapshot(); got != (PromptSnapshot{}) {
		t.Fatalf("nil prompt manager snapshot should be zero, got %+v", got)
	}
	if err := m.Reload(); !errors.Is(err, ErrPromptsConfigPathNotSet) {
		t.Fatalf("unexpected reload error: %v", err)
	}
	if err := m.Save(""); !errors.Is(err, ErrPromptsConfigPathNotSet) {
		t.Fatalf("unexpected save error: %v", err)
	}
}

func TestDesensitizationManagerDefaultsSnapshotAndCopy(t *testing.T) {
	t.Parallel()

	m, err := NewDesensitizationManager("")
	if err != nil {
		t.Fatalf("new desensitization manager: %v", err)
	}

	current := m.CurrentDesensitizationConfig()
	if !current.Enabled {
		t.Fatalf("expected default desensitization to be enabled")
	}

	snapshot := m.Snapshot()
	if !snapshot.Loaded || snapshot.Path != "" {
		t.Fatalf("unexpected default snapshot: %+v", snapshot)
	}
	snapshot.Config.Enabled = false
	if got := m.CurrentDesensitizationConfig(); !got.Enabled {
		t.Fatalf("manager config was mutated through snapshot")
	}
	current.Enabled = false
	if got := m.CurrentDesensitizationConfig(); !got.Enabled {
		t.Fatalf("manager config was mutated through current copy")
	}
}

func TestDesensitizationManagerWithoutPathPersistsRuntimeConfig(t *testing.T) {
	t.Parallel()

	m, err := NewDesensitizationManager("")
	if err != nil {
		t.Fatalf("new desensitization manager: %v", err)
	}

	var persisted DesensitizationConfig
	m.SetPersistence(func(cfg DesensitizationConfig) error {
		persisted = cfg
		return nil
	})

	next := DefaultDesensitizationConfig()
	next.Enabled = false
	next.Placeholders.ReplaceInlineHost = false
	if err := m.SaveConfig(next); err != nil {
		t.Fatalf("save runtime desensitization config: %v", err)
	}
	if got := m.CurrentDesensitizationConfig(); got.Enabled || got.Placeholders.ReplaceInlineHost {
		t.Fatalf("unexpected current desensitization config: %+v", got)
	}
	if persisted.Enabled || persisted.Placeholders.ReplaceInlineHost {
		t.Fatalf("unexpected persisted desensitization config: %+v", persisted)
	}

	loaded := DefaultDesensitizationConfig()
	loaded.Secrets.RedactBearer = false
	if err := m.LoadRuntimeConfig(loaded); err != nil {
		t.Fatalf("load runtime desensitization config: %v", err)
	}
	if got := m.CurrentDesensitizationConfig(); got.Secrets.RedactBearer {
		t.Fatalf("unexpected loaded desensitization config: %+v", got)
	}
}

func TestDesensitizationManagerSaveAndReloadRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "desensitization.yaml")
	initialCfg := DefaultDesensitizationConfig()
	initialCfg.Enabled = false
	initialCfg.LocalLLMAssist.Enabled = true
	initialCfg.LocalLLMAssist.Provider = " openai_compatible "
	initialCfg.LocalLLMAssist.BaseURL = " http://127.0.0.1:11434/v1 "
	initialCfg.LocalLLMAssist.Model = " qwen2.5 "
	initialContent, err := EncodeDesensitizationConfig(&initialCfg)
	if err != nil {
		t.Fatalf("encode initial desensitization config: %v", err)
	}
	if err := os.WriteFile(path, []byte(initialContent), 0o600); err != nil {
		t.Fatalf("write desensitization config: %v", err)
	}

	m, err := NewDesensitizationManager(path)
	if err != nil {
		t.Fatalf("new desensitization manager: %v", err)
	}
	if got := m.Snapshot(); got.Content != initialContent {
		t.Fatalf("unexpected snapshot content: got %q want %q", got.Content, initialContent)
	}

	next := DefaultDesensitizationConfig()
	next.Rehydration.Path = false
	next.Secrets.AdditionalPatterns = []string{`corp-[A-Z0-9]{6}`}
	next.LocalLLMAssist.Enabled = true
	next.LocalLLMAssist.Provider = " openai_compatible "
	next.LocalLLMAssist.Model = " qwen2.5 "
	if err := m.SaveConfig(next); err != nil {
		t.Fatalf("save desensitization config: %v", err)
	}
	expectedSaved := DefaultDesensitizationConfig()
	expectedSaved.LocalLLMAssist.Enabled = true
	expectedSaved.LocalLLMAssist.Provider = "openai_compatible"
	expectedSaved.LocalLLMAssist.Model = "qwen2.5"
	expectedSaved.Secrets.AdditionalPatterns = []string{`corp-[A-Z0-9]{6}`}
	expectedSaved.Rehydration.Path = false
	if got := m.Snapshot().Config; !reflect.DeepEqual(got, expectedSaved) {
		t.Fatalf("unexpected saved desensitization config: got %+v want %+v", got, expectedSaved)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved desensitization config: %v", err)
	}
	parsedSaved, err := ParseDesensitizationConfig(saved)
	if err != nil {
		t.Fatalf("parse saved desensitization config: %v", err)
	}
	if got := parsedSaved; !reflect.DeepEqual(*got, expectedSaved) {
		t.Fatalf("saved file did not normalize as expected: got %+v want %+v", got, expectedSaved)
	}

	rewritten := strings.TrimSpace(`
desensitization:
  enabled: true
  secrets:
    key_names:
      - token
  placeholders:
    replace_inline_path: false
  rehydration:
    path: false
`)
	if err := os.WriteFile(path, []byte(rewritten), 0o600); err != nil {
		t.Fatalf("rewrite desensitization config: %v", err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("reload desensitization config: %v", err)
	}

	expectedReloaded := DefaultDesensitizationConfig()
	expectedReloaded.Secrets.KeyNames = []string{"token"}
	expectedReloaded.Placeholders.ReplaceInlinePath = false
	expectedReloaded.Rehydration.Path = false
	if got := m.CurrentDesensitizationConfig(); !reflect.DeepEqual(*got, expectedReloaded) {
		t.Fatalf("unexpected reloaded desensitization config: got %+v want %+v", got, expectedReloaded)
	}
	expectedReloadedContent, err := EncodeDesensitizationConfig(&expectedReloaded)
	if err != nil {
		t.Fatalf("encode expected reloaded desensitization config: %v", err)
	}
	if got := m.Snapshot(); got.Content != expectedReloadedContent {
		t.Fatalf("reload did not update snapshot content: got %q want %q", got.Content, expectedReloadedContent)
	}
}

func TestDesensitizationManagerNilReceiverReturnsDefaults(t *testing.T) {
	t.Parallel()

	var m *DesensitizationManager
	if got := m.CurrentDesensitizationConfig(); !got.Enabled {
		t.Fatalf("nil desensitization manager did not return defaults: %+v", got)
	}
	if got := m.Snapshot(); got.Path != "" || got.Content != "" || got.Loaded || !reflect.DeepEqual(got.Config, DesensitizationConfig{}) {
		t.Fatalf("nil desensitization manager snapshot should be zero, got %+v", got)
	}
	if err := m.Reload(); !errors.Is(err, ErrDesensitizationConfigPathNotSet) {
		t.Fatalf("unexpected reload error: %v", err)
	}
	if err := m.Save(""); !errors.Is(err, ErrDesensitizationConfigPathNotSet) {
		t.Fatalf("unexpected save error: %v", err)
	}
}

func TestProviderManagerDefaultSnapshotAndCopy(t *testing.T) {
	t.Parallel()

	m, err := NewProviderManager("")
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}

	snapshot := m.Snapshot()
	if !snapshot.Loaded || snapshot.Path != "" {
		t.Fatalf("unexpected default snapshot: %+v", snapshot)
	}
	if !reflect.DeepEqual(snapshot.Config, DefaultProvidersConfig()) {
		t.Fatalf("unexpected default providers config: %+v", snapshot.Config)
	}
	if refs := m.SecretRefs(); len(refs) != 0 {
		t.Fatalf("expected no secret refs for default config, got %+v", refs)
	}

	snapshot.Config.Primary.ProviderID = "changed"
	if got := m.Snapshot(); got.Config.Primary.ProviderID != "" {
		t.Fatalf("manager config was mutated through snapshot: %+v", got.Config)
	}
}

func TestProviderManagerReloadSaveResolveAndPersistence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "providers.yaml")
	raw := strings.TrimSpace(`
providers:
  primary:
    provider_id: " primary "
    model: " gpt-4o "
  assist:
    provider_id: " assist "
    model: " claude-3.5-sonnet "
  entries:
    - id: "assist"
      vendor: " anthropic "
      base_url: " https://assist.example "
      api_key_ref: " secret/assist "
      enabled: true
    - id: "primary"
      vendor: " openai_compatible "
      protocol: " openai-compatible "
      base_url: " https://primary.example "
      api_key_ref: " secret/primary "
      enabled: true
    - id: "primary"
      vendor: "ignored"
      base_url: "https://ignored.example"
      enabled: false
`)
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write providers config: %v", err)
	}

	storeDir := filepath.Join(dir, "secrets")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("create secrets dir: %v", err)
	}
	store, err := secrets.NewStore(filepath.Join(storeDir, "store.yaml"))
	if err != nil {
		t.Fatalf("new secrets store: %v", err)
	}
	if _, err := store.Apply(map[string]string{
		"secret/assist":  "assist-key",
		"secret/primary": "primary-key",
	}, nil, time.Now().UTC()); err != nil {
		t.Fatalf("seed secret store: %v", err)
	}

	m, err := NewProviderManager(path)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}

	snapshot := m.Snapshot()
	if snapshot.Config.Primary.ProviderID != "primary" || snapshot.Config.Assist.ProviderID != "assist" {
		t.Fatalf("unexpected loaded provider bindings: %+v", snapshot.Config)
	}
	if got := m.SecretRefs(); !reflect.DeepEqual(got, []SecretReference{
		{Ref: "secret/assist", Set: true, Source: "provider_registry"},
		{Ref: "secret/primary", Set: true, Source: "provider_registry"},
	}) {
		t.Fatalf("unexpected secret refs: %+v", got)
	}

	primary := m.ResolvePrimaryModelTarget()
	if primary == nil || primary.ProviderID != "primary" || primary.APIKey != "" {
		t.Fatalf("unexpected primary target without secrets: %+v", primary)
	}
	primaryWithSecrets := m.ResolvePrimaryModelTargetWithSecrets(store)
	if primaryWithSecrets == nil || primaryWithSecrets.APIKey != "primary-key" {
		t.Fatalf("unexpected primary target with secrets: %+v", primaryWithSecrets)
	}
	assist := m.ResolveAssistModelTarget()
	if assist == nil || assist.ProviderID != "assist" || assist.APIKey != "" {
		t.Fatalf("unexpected assist target without secrets: %+v", assist)
	}
	assistWithSecrets := m.ResolveAssistModelTargetWithSecrets(store)
	if assistWithSecrets == nil || assistWithSecrets.APIKey != "assist-key" {
		t.Fatalf("unexpected assist target with secrets: %+v", assistWithSecrets)
	}

	var persisted ProvidersConfig
	var persistedCalled bool
	m.SetPersistence(func(cfg ProvidersConfig) error {
		persistedCalled = true
		persisted = cfg
		return nil
	})

	update := ProvidersConfig{
		Primary: ProviderBinding{ProviderID: " primary ", Model: " gpt-4o.1 "},
		Assist:  ProviderBinding{ProviderID: " assist ", Model: " claude-3.7 "},
		Entries: []ProviderEntry{
			{
				ID:        "primary",
				Vendor:    " openai_compatible ",
				BaseURL:   " https://primary.example ",
				APIKeyRef: "secret/primary",
				Enabled:   true,
			},
			{
				ID:        "assist",
				Vendor:    " anthropic ",
				BaseURL:   " https://assist.example ",
				APIKeyRef: "secret/assist",
				Enabled:   true,
			},
		},
	}
	if err := m.SaveConfig(update); err != nil {
		t.Fatalf("save providers config: %v", err)
	}
	if !persistedCalled {
		t.Fatalf("expected provider persistence callback to be invoked")
	}
	if persisted.Primary.ProviderID != "primary" || persisted.Primary.Model != "gpt-4o.1" {
		t.Fatalf("unexpected persisted primary binding: %+v", persisted.Primary)
	}
	if len(persisted.Entries) != 2 || persisted.Entries[0].ID != "assist" || persisted.Entries[1].ID != "primary" {
		t.Fatalf("unexpected persisted providers config ordering: %+v", persisted.Entries)
	}

	savedContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved providers config: %v", err)
	}
	if got := m.Snapshot(); got.Content != string(savedContent) {
		t.Fatalf("snapshot content did not track provider save")
	}
	if got := m.ResolvePrimaryModelTargetWithSecrets(store); got == nil || got.APIKey != "primary-key" {
		t.Fatalf("expected merged secret to survive save: %+v", got)
	}

	rewritten := strings.TrimSpace(`
providers:
  primary:
    provider_id: primary
    model: gpt-5
  assist:
    provider_id: assist
    model: claude-4
  entries:
    - id: primary
      vendor: openai
      base_url: https://primary.example
      enabled: true
`)
	if err := os.WriteFile(path, []byte(rewritten), 0o600); err != nil {
		t.Fatalf("rewrite providers config: %v", err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("reload providers config: %v", err)
	}
	if got := m.Snapshot(); got.Config.Primary.Model != "gpt-5" || got.Config.Assist.Model != "claude-4" {
		t.Fatalf("reload did not refresh provider config: %+v", got.Config)
	}
}

func TestProviderManagerNilReceiverReturnsZeroValues(t *testing.T) {
	t.Parallel()

	var m *ProviderManager
	if got := m.ResolvePrimaryModelTarget(); got != nil {
		t.Fatalf("expected nil primary target, got %+v", got)
	}
	if got := m.ResolveAssistModelTarget(); got != nil {
		t.Fatalf("expected nil assist target, got %+v", got)
	}
	if got := m.ResolvePrimaryModelTargetWithSecrets(nil); got != nil {
		t.Fatalf("expected nil primary target with secrets, got %+v", got)
	}
	if got := m.ResolveAssistModelTargetWithSecrets(nil); got != nil {
		t.Fatalf("expected nil assist target with secrets, got %+v", got)
	}
	if got := m.Snapshot(); got.Path != "" || got.Content != "" || got.Loaded || !reflect.DeepEqual(got.Config, ProvidersConfig{}) {
		t.Fatalf("expected zero snapshot, got %+v", got)
	}
	if got := m.SecretRefs(); got != nil {
		t.Fatalf("expected nil secret refs, got %+v", got)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("expected nil reload to succeed, got %v", err)
	}
	if err := m.SaveConfig(ProvidersConfig{}); err != nil {
		t.Fatalf("expected nil save config to succeed, got %v", err)
	}
	if err := m.Save(""); err != nil {
		t.Fatalf("expected nil save to succeed, got %v", err)
	}
}
