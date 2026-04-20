package reasoning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPromptSet(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "prompts.yaml")
	content := `
reasoning:
  system_prompt: |
    test system
  user_prompt_template: |
    sid={{ .SessionID }}
    host={{ index .Context "host" }}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	prompts, err := LoadPromptSet(path)
	if err != nil {
		t.Fatalf("load prompt set: %v", err)
	}

	rendered, err := prompts.RenderUserPrompt("ses-1", map[string]interface{}{"host": "node-1"})
	if err != nil {
		t.Fatalf("render user prompt: %v", err)
	}

	if strings.TrimSpace(prompts.SystemPrompt) != "test system" {
		t.Fatalf("unexpected system prompt: %q", prompts.SystemPrompt)
	}
	if !strings.Contains(rendered, "sid=ses-1") || !strings.Contains(rendered, "host=node-1") {
		t.Fatalf("unexpected rendered prompt: %s", rendered)
	}
}

func TestDefaultPromptSetProvidesDefaults(t *testing.T) {
	t.Parallel()

	prompts := NewPromptSet("", "")
	rendered, err := prompts.RenderUserPrompt("ses-2", map[string]interface{}{"host": "node-2"})
	if err != nil {
		t.Fatalf("render default prompt: %v", err)
	}
	if !strings.Contains(prompts.SystemPrompt, "Return ONLY strict JSON") {
		t.Fatalf("unexpected default system prompt: %s", prompts.SystemPrompt)
	}
	if !strings.Contains(rendered, "session_id=ses-2") || !strings.Contains(rendered, `"host":"node-2"`) {
		t.Fatalf("unexpected default rendered prompt: %s", rendered)
	}
}
