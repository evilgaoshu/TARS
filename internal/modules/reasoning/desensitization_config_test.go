package reasoning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesensitizeContextCanDisablePathRehydration(t *testing.T) {
	t.Parallel()

	cfg := DefaultDesensitizationConfig()
	cfg.Rehydration.Path = false

	sanitized, mapping := desensitizeContextWithConfig(map[string]interface{}{
		"host":         "192.168.3.106",
		"user_request": "读取 /tmp/app.log token=abc123",
		"file_path":    "/tmp/app.log",
	}, &cfg)

	userRequest, _ := sanitized["user_request"].(string)
	if strings.Contains(userRequest, "/tmp/app.log") {
		t.Fatalf("expected path to be desensitized, got %q", userRequest)
	}
	if !strings.Contains(userRequest, "[PATH_1]") {
		t.Fatalf("expected PATH placeholder, got %q", userRequest)
	}

	rehydrated := rehydratePlaceholdersWithConfig("inspect [PATH_1] on [IP_1]", mapping, &cfg)
	if strings.Contains(rehydrated, "/tmp/app.log") {
		t.Fatalf("expected path rehydration to remain disabled, got %q", rehydrated)
	}
	if !strings.Contains(rehydrated, "192.168.3.106") {
		t.Fatalf("expected IP to still rehydrate, got %q", rehydrated)
	}
}

func TestDesensitizeContextUsesAdditionalSecretPatterns(t *testing.T) {
	t.Parallel()

	cfg := DefaultDesensitizationConfig()
	cfg.Secrets.AdditionalPatterns = []string{`corp-[A-Z0-9]{6}`}

	sanitized, _ := desensitizeContextWithConfig(map[string]interface{}{
		"user_request": "使用 corp-ABC123 调用内部接口",
	}, &cfg)
	userRequest, _ := sanitized["user_request"].(string)
	if strings.Contains(userRequest, "corp-ABC123") {
		t.Fatalf("expected additional secret pattern to be redacted, got %q", userRequest)
	}
	if !strings.Contains(userRequest, "[REDACTED]") {
		t.Fatalf("expected redacted marker, got %q", userRequest)
	}
}

func TestEncodeParseDesensitizationConfigRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := DefaultDesensitizationConfig()
	cfg.Enabled = true
	cfg.Secrets.KeyNames = []string{"password", "cookie"}
	cfg.Placeholders.HostKeyFragments = []string{"host", "peer"}
	cfg.Rehydration.Path = false
	cfg.LocalLLMAssist.Enabled = true
	cfg.LocalLLMAssist.BaseURL = "http://127.0.0.1:11434/v1"
	cfg.LocalLLMAssist.Model = "qwen2.5"

	content, err := EncodeDesensitizationConfig(&cfg)
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}

	parsed, err := ParseDesensitizationConfig([]byte(content))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if !parsed.LocalLLMAssist.Enabled || parsed.LocalLLMAssist.Model != "qwen2.5" {
		t.Fatalf("unexpected parsed local llm assist config: %+v", parsed.LocalLLMAssist)
	}
	if parsed.Rehydration.Path {
		t.Fatalf("expected path rehydration to remain disabled after round trip")
	}
	if len(parsed.Secrets.KeyNames) != 2 || parsed.Secrets.KeyNames[1] != "cookie" {
		t.Fatalf("unexpected secret key names: %+v", parsed.Secrets.KeyNames)
	}
}

func TestLoadDesensitizationConfigAndWrapperHelpers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "desensitization.yaml")
	content := `
desensitization:
  enabled: true
  rehydration:
    path: false
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadDesensitizationConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Rehydration.Path {
		t.Fatalf("expected loaded config to disable path rehydration")
	}

	sanitized, mapping := desensitizeContext(map[string]interface{}{
		"host":         "example.internal",
		"user_request": "read /var/log/app.log from 10.0.0.8 token=abc123",
	})
	userRequest, _ := sanitized["user_request"].(string)
	if strings.Contains(userRequest, "example.internal") || strings.Contains(userRequest, "/var/log/app.log") || strings.Contains(userRequest, "10.0.0.8") {
		t.Fatalf("expected wrapper desensitization to replace sensitive values, got %q", userRequest)
	}

	rehydrated := rehydratePlaceholders("inspect [HOST_1] [IP_1] [PATH_1]", mapping)
	if !strings.Contains(rehydrated, "example.internal") || !strings.Contains(rehydrated, "10.0.0.8") || !strings.Contains(rehydrated, "/var/log/app.log") {
		t.Fatalf("expected wrapper rehydration to restore values, got %q", rehydrated)
	}
}

func TestDesensitizeContextWithConfigAndDetectionsSupportsDisabledAndHintedModes(t *testing.T) {
	t.Parallel()

	disabled := DefaultDesensitizationConfig()
	disabled.Enabled = false

	input := map[string]interface{}{
		"user_request": "read /tmp/app.log from host-1",
		"nested":       map[string]interface{}{"token": "abc123"},
	}
	sanitized, mapping := desensitizeContextWithConfigAndDetections(input, &disabled, nil)
	if mapping != nil {
		t.Fatalf("expected disabled config to skip placeholder mapping, got %+v", mapping)
	}
	if sanitized["user_request"] != input["user_request"] {
		t.Fatalf("expected disabled config to preserve original content, got %+v", sanitized)
	}

	cfg := DefaultDesensitizationConfig()
	sanitized, mapping = desensitizeContextWithConfigAndDetections(map[string]interface{}{
		"user_request": "connect to phoenix-cluster at 192.168.0.12 and inspect /srv/app/config.yaml token=topsecret",
	}, &cfg, &SensitiveDetections{
		Secrets: []string{" topsecret ", "topsecret"},
		Hosts:   []string{"phoenix-cluster", "phoenix-cluster"},
		IPs:     []string{"192.168.0.12"},
		Paths:   []string{"/srv/app/config.yaml"},
	})

	userRequest, _ := sanitized["user_request"].(string)
	if strings.Contains(userRequest, "phoenix-cluster") || strings.Contains(userRequest, "192.168.0.12") || strings.Contains(userRequest, "/srv/app/config.yaml") || strings.Contains(userRequest, "topsecret") {
		t.Fatalf("expected detection hints to be applied before fallback rules, got %q", userRequest)
	}
	if mapping["[HOST_1]"] != "phoenix-cluster" || mapping["[IP_1]"] != "192.168.0.12" || mapping["[PATH_1]"] != "/srv/app/config.yaml" {
		t.Fatalf("unexpected detection-based mapping: %+v", mapping)
	}
}

func TestWriteDesensitizationFileAtomically(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "desensitization.yaml")
	if err := writeDesensitizationFileAtomically(path, "desensitization:\n  enabled: true\n"); err != nil {
		t.Fatalf("write desensitization file: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read desensitization file: %v", err)
	}
	if !strings.Contains(string(content), "enabled: true") {
		t.Fatalf("expected written content, got %q", string(content))
	}
	if err := writeDesensitizationFileAtomically(filepath.Join(dir, "missing", "desensitization.yaml"), "x"); err == nil {
		t.Fatalf("expected write helper to fail for missing directory")
	}
}

func TestPromptAndDesensitizationManagerErrorBranches(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	promptPath := filepath.Join(dir, "prompts.yaml")
	if err := os.WriteFile(promptPath, []byte("reasoning:\n  system_prompt: ok\n"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}
	promptManager, err := NewPromptManager(promptPath)
	if err != nil {
		t.Fatalf("new prompt manager: %v", err)
	}
	if err := promptManager.Save(":\n"); err == nil {
		t.Fatalf("expected invalid prompt yaml to fail")
	}
	if err := (&PromptManager{path: filepath.Join(dir, "missing", "prompts.yaml")}).Reload(); err == nil {
		t.Fatalf("expected reload from missing prompt file to fail")
	}

	desensePath := filepath.Join(dir, "desensitization.yaml")
	if err := os.WriteFile(desensePath, []byte("desensitization:\n  enabled: true\n"), 0o600); err != nil {
		t.Fatalf("write desensitization file: %v", err)
	}
	desenseManager, err := NewDesensitizationManager(desensePath)
	if err != nil {
		t.Fatalf("new desensitization manager: %v", err)
	}
	if err := desenseManager.Save(":\n"); err == nil {
		t.Fatalf("expected invalid desensitization yaml to fail")
	}
	if err := (&DesensitizationManager{path: filepath.Join(dir, "missing", "desensitization.yaml")}).Reload(); err == nil {
		t.Fatalf("expected reload from missing desensitization file to fail")
	}

	var nilPromptManager *PromptManager
	if err := nilPromptManager.SavePromptSet(PromptSet{}); err == nil {
		t.Fatalf("expected nil prompt manager SavePromptSet to fail")
	}
	if _, err := LoadPromptSet(filepath.Join(dir, "missing-prompts.yaml")); err == nil {
		t.Fatalf("expected LoadPromptSet to fail for missing file")
	}

	var nilDesenseManager *DesensitizationManager
	if err := nilDesenseManager.SaveConfig(DefaultDesensitizationConfig()); err == nil {
		t.Fatalf("expected nil desensitization manager SaveConfig to fail")
	}
}
