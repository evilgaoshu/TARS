package authorization

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleAuthorizationConfig = `authorization:
  defaults:
    whitelist_action: direct_execute
    blacklist_action: suggest_only
    unmatched_action: require_approval
  hard_deny:
    ssh_command:
      - "rm -rf /"
  ssh_command:
    normalize_whitespace: true
    whitelist:
      - "uptime*"
    blacklist:
      - "systemctl restart *"
`

func TestManagerSaveReloadsResolver(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "authorization.yaml")
	if err := os.WriteFile(path, []byte(sampleAuthorizationConfig), 0o600); err != nil {
		t.Fatalf("write initial policy: %v", err)
	}

	manager, err := NewManager(path)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	decision := manager.EvaluateSSHCommand(SSHCommandInput{Command: "uptime && cat /proc/loadavg"})
	if decision.Action != ActionDirectExecute {
		t.Fatalf("expected direct_execute, got %+v", decision)
	}

	updated := `authorization:
  defaults:
    whitelist_action: direct_execute
    blacklist_action: require_approval
    unmatched_action: require_approval
  ssh_command:
    normalize_whitespace: true
    whitelist:
      - "uptime*"
    blacklist:
      - "systemctl restart *"
`
	if err := manager.Save(updated); err != nil {
		t.Fatalf("save updated policy: %v", err)
	}

	decision = manager.EvaluateSSHCommand(SSHCommandInput{Command: "systemctl restart nginx"})
	if decision.Action != ActionRequireApproval {
		t.Fatalf("expected require_approval after save, got %+v", decision)
	}

	snapshot := manager.Snapshot()
	if !snapshot.Loaded {
		t.Fatalf("expected loaded snapshot, got %+v", snapshot)
	}
	if snapshot.Config.Defaults.BlacklistAction != ActionRequireApproval {
		t.Fatalf("unexpected snapshot config: %+v", snapshot.Config)
	}
}

func TestManagerSaveRejectsInvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "authorization.yaml")
	if err := os.WriteFile(path, []byte(sampleAuthorizationConfig), 0o600); err != nil {
		t.Fatalf("write initial policy: %v", err)
	}

	manager, err := NewManager(path)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	if err := manager.Save("authorization:\n  defaults: ["); err == nil {
		t.Fatalf("expected invalid yaml error")
	}
}
