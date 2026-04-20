package authorization

import "testing"

func TestEvaluateSSHCommandHonorsOverrideBeforeBlacklist(t *testing.T) {
	t.Parallel()

	resolver := New(Config{
		Defaults: Defaults{
			WhitelistAction: ActionDirectExecute,
			BlacklistAction: ActionSuggestOnly,
			UnmatchedAction: ActionRequireApproval,
		},
		SSH: SSHCommandConfig{
			NormalizeWhitespace: true,
			Blacklist:           []string{"systemctl restart *"},
			Overrides: []OverrideConfig{
				{
					ID:           "sshd-restart-approved",
					Services:     []string{"sshd"},
					CommandGlobs: []string{"systemctl restart sshd*"},
					Action:       ActionRequireApproval,
				},
			},
		},
	})

	decision := resolver.EvaluateSSHCommand(SSHCommandInput{
		Command: "systemctl   restart   sshd",
		Service: "sshd",
	})
	if decision.Action != ActionRequireApproval {
		t.Fatalf("expected require_approval, got %+v", decision)
	}
	if decision.RuleID != "sshd-restart-approved" {
		t.Fatalf("expected override rule id, got %+v", decision)
	}
}

func TestEvaluateSSHCommandUsesWhitelistAndFallback(t *testing.T) {
	t.Parallel()

	resolver := New(Config{
		Defaults: Defaults{
			WhitelistAction: ActionDirectExecute,
			BlacklistAction: ActionSuggestOnly,
			UnmatchedAction: ActionRequireApproval,
		},
		SSH: SSHCommandConfig{
			NormalizeWhitespace: true,
			Whitelist:           []string{"uptime*", "df -h*"},
		},
	})

	decision := resolver.EvaluateSSHCommand(SSHCommandInput{Command: "uptime && cat /proc/loadavg"})
	if decision.Action != ActionDirectExecute {
		t.Fatalf("expected direct_execute for whitelist, got %+v", decision)
	}

	decision = resolver.EvaluateSSHCommand(SSHCommandInput{Command: "systemctl restart nginx"})
	if decision.Action != ActionRequireApproval {
		t.Fatalf("expected require_approval for unmatched command, got %+v", decision)
	}
}

func TestEvaluateSSHCommandHardDenyWins(t *testing.T) {
	t.Parallel()

	resolver := New(Config{
		Defaults: Defaults{
			WhitelistAction: ActionDirectExecute,
			BlacklistAction: ActionSuggestOnly,
			UnmatchedAction: ActionRequireApproval,
		},
		HardDeny: HardDenyConfig{
			SSHCommand: []string{"rm -rf /"},
		},
		SSH: SSHCommandConfig{
			NormalizeWhitespace: true,
			Whitelist:           []string{"rm -rf /"},
		},
	})

	decision := resolver.EvaluateSSHCommand(SSHCommandInput{Command: "rm -rf /"})
	if decision.Action != ActionDeny {
		t.Fatalf("expected deny, got %+v", decision)
	}
}
