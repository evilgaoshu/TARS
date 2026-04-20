package authorization

import (
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Action string

const (
	ActionDirectExecute   Action = "direct_execute"
	ActionRequireApproval Action = "require_approval"
	ActionSuggestOnly     Action = "suggest_only"
	ActionDeny            Action = "deny"
)

type SSHCommandInput struct {
	Command string
	Service string
	Host    string
	Channel string
}

// CapabilityInput represents a connector capability invocation to be authorized.
type CapabilityInput struct {
	ConnectorID  string
	CapabilityID string
	ReadOnly     bool
	Source       string // "connector", "mcp", "skill", "builtin"
}

type Evaluator interface {
	EvaluateSSHCommand(input SSHCommandInput) Decision
	EvaluateCapability(input CapabilityInput) Decision
}

type Decision struct {
	Action    Action
	RuleID    string
	MatchedBy string
}

type Resolver struct {
	defaults Defaults
	hardDeny hardDenyPolicy
	ssh      sshCommandPolicy
}

type Defaults struct {
	WhitelistAction Action
	BlacklistAction Action
	UnmatchedAction Action
}

type Config struct {
	Defaults Defaults
	HardDeny HardDenyConfig
	SSH      SSHCommandConfig
}

type HardDenyConfig struct {
	SSHCommand []string
	MCPSkill   []string
}

type SSHCommandConfig struct {
	NormalizeWhitespace bool
	Whitelist           []string
	Blacklist           []string
	Overrides           []OverrideConfig
}

type OverrideConfig struct {
	ID           string
	Services     []string
	Hosts        []string
	Channels     []string
	CommandGlobs []string
	Action       Action
}

type hardDenyPolicy struct {
	sshCommand []string
	mcpSkill   []string
}

type sshCommandPolicy struct {
	normalizeWhitespace bool
	whitelist           []string
	blacklist           []string
	overrides           []overrideRule
}

type overrideRule struct {
	id           string
	services     []string
	hosts        []string
	channels     []string
	commandGlobs []string
	action       Action
}

type fileConfig struct {
	Authorization struct {
		Defaults struct {
			WhitelistAction string `yaml:"whitelist_action"`
			BlacklistAction string `yaml:"blacklist_action"`
			UnmatchedAction string `yaml:"unmatched_action"`
		} `yaml:"defaults"`
		HardDeny struct {
			SSHCommand []string `yaml:"ssh_command,omitempty"`
			MCPSkill   []string `yaml:"mcp_skill,omitempty"`
		} `yaml:"hard_deny,omitempty"`
		SSHCommand struct {
			NormalizeWhitespace bool     `yaml:"normalize_whitespace,omitempty"`
			Whitelist           []string `yaml:"whitelist,omitempty"`
			Blacklist           []string `yaml:"blacklist,omitempty"`
			Overrides           []struct {
				ID           string   `yaml:"id,omitempty"`
				Services     []string `yaml:"services,omitempty"`
				Hosts        []string `yaml:"hosts,omitempty"`
				Channels     []string `yaml:"channels,omitempty"`
				CommandGlobs []string `yaml:"command_globs,omitempty"`
				Action       string   `yaml:"action,omitempty"`
			} `yaml:"overrides,omitempty"`
		} `yaml:"ssh_command"`
	} `yaml:"authorization"`
}

func Load(path string) (*Resolver, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return Parse(content)
}

func Parse(content []byte) (*Resolver, error) {
	cfg, err := ParseConfig(content)
	if err != nil {
		return nil, err
	}
	return New(cfg), nil
}

func ParseConfig(content []byte) (Config, error) {
	var cfg fileConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return Config{}, err
	}

	overrides := make([]OverrideConfig, 0, len(cfg.Authorization.SSHCommand.Overrides))
	for _, item := range cfg.Authorization.SSHCommand.Overrides {
		overrides = append(overrides, OverrideConfig{
			ID:           item.ID,
			Services:     item.Services,
			Hosts:        item.Hosts,
			Channels:     item.Channels,
			CommandGlobs: item.CommandGlobs,
			Action:       parseAction(item.Action, ActionRequireApproval),
		})
	}

	return Config{
		Defaults: Defaults{
			WhitelistAction: parseAction(cfg.Authorization.Defaults.WhitelistAction, ActionDirectExecute),
			BlacklistAction: parseAction(cfg.Authorization.Defaults.BlacklistAction, ActionSuggestOnly),
			UnmatchedAction: parseAction(cfg.Authorization.Defaults.UnmatchedAction, ActionRequireApproval),
		},
		HardDeny: HardDenyConfig{
			SSHCommand: cfg.Authorization.HardDeny.SSHCommand,
			MCPSkill:   cfg.Authorization.HardDeny.MCPSkill,
		},
		SSH: SSHCommandConfig{
			NormalizeWhitespace: cfg.Authorization.SSHCommand.NormalizeWhitespace,
			Whitelist:           cfg.Authorization.SSHCommand.Whitelist,
			Blacklist:           cfg.Authorization.SSHCommand.Blacklist,
			Overrides:           overrides,
		},
	}, nil
}

func EncodeConfig(cfg Config) (string, error) {
	var out fileConfig
	out.Authorization.Defaults.WhitelistAction = string(parseAction(string(cfg.Defaults.WhitelistAction), ActionDirectExecute))
	out.Authorization.Defaults.BlacklistAction = string(parseAction(string(cfg.Defaults.BlacklistAction), ActionSuggestOnly))
	out.Authorization.Defaults.UnmatchedAction = string(parseAction(string(cfg.Defaults.UnmatchedAction), ActionRequireApproval))
	out.Authorization.HardDeny.SSHCommand = cloneStrings(cfg.HardDeny.SSHCommand)
	out.Authorization.HardDeny.MCPSkill = cloneStrings(cfg.HardDeny.MCPSkill)
	out.Authorization.SSHCommand.NormalizeWhitespace = cfg.SSH.NormalizeWhitespace
	out.Authorization.SSHCommand.Whitelist = cloneStrings(cfg.SSH.Whitelist)
	out.Authorization.SSHCommand.Blacklist = cloneStrings(cfg.SSH.Blacklist)
	out.Authorization.SSHCommand.Overrides = make([]struct {
		ID           string   `yaml:"id,omitempty"`
		Services     []string `yaml:"services,omitempty"`
		Hosts        []string `yaml:"hosts,omitempty"`
		Channels     []string `yaml:"channels,omitempty"`
		CommandGlobs []string `yaml:"command_globs,omitempty"`
		Action       string   `yaml:"action,omitempty"`
	}, 0, len(cfg.SSH.Overrides))
	for _, item := range cfg.SSH.Overrides {
		out.Authorization.SSHCommand.Overrides = append(out.Authorization.SSHCommand.Overrides, struct {
			ID           string   `yaml:"id,omitempty"`
			Services     []string `yaml:"services,omitempty"`
			Hosts        []string `yaml:"hosts,omitempty"`
			Channels     []string `yaml:"channels,omitempty"`
			CommandGlobs []string `yaml:"command_globs,omitempty"`
			Action       string   `yaml:"action,omitempty"`
		}{
			ID:           strings.TrimSpace(item.ID),
			Services:     cloneStrings(item.Services),
			Hosts:        cloneStrings(item.Hosts),
			Channels:     cloneStrings(item.Channels),
			CommandGlobs: cloneStrings(item.CommandGlobs),
			Action:       string(parseAction(string(item.Action), ActionRequireApproval)),
		})
	}

	content, err := yaml.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func New(cfg Config) *Resolver {
	defaults := cfg.Defaults
	if defaults.WhitelistAction == "" {
		defaults.WhitelistAction = ActionDirectExecute
	}
	if defaults.BlacklistAction == "" {
		defaults.BlacklistAction = ActionSuggestOnly
	}
	if defaults.UnmatchedAction == "" {
		defaults.UnmatchedAction = ActionRequireApproval
	}

	policy := &Resolver{
		defaults: defaults,
		hardDeny: hardDenyPolicy{
			sshCommand: normalizePatterns(cfg.HardDeny.SSHCommand, true),
			mcpSkill:   normalizePatterns(cfg.HardDeny.MCPSkill, true),
		},
		ssh: sshCommandPolicy{
			normalizeWhitespace: cfg.SSH.NormalizeWhitespace,
			whitelist:           normalizePatterns(cfg.SSH.Whitelist, cfg.SSH.NormalizeWhitespace),
			blacklist:           normalizePatterns(cfg.SSH.Blacklist, cfg.SSH.NormalizeWhitespace),
		},
	}

	for _, item := range cfg.SSH.Overrides {
		action := item.Action
		if action == "" {
			action = ActionRequireApproval
		}
		policy.ssh.overrides = append(policy.ssh.overrides, overrideRule{
			id:           strings.TrimSpace(item.ID),
			services:     normalizePatterns(item.Services, true),
			hosts:        normalizePatterns(item.Hosts, true),
			channels:     normalizePatterns(item.Channels, true),
			commandGlobs: normalizePatterns(item.CommandGlobs, cfg.SSH.NormalizeWhitespace),
			action:       action,
		})
	}

	return policy
}

func (r *Resolver) EvaluateSSHCommand(input SSHCommandInput) Decision {
	if r == nil {
		return Decision{Action: ActionRequireApproval, MatchedBy: "default"}
	}

	command := normalizeValue(input.Command, r.ssh.normalizeWhitespace)
	service := normalizeValue(input.Service, true)
	host := normalizeValue(input.Host, true)
	channel := normalizeValue(input.Channel, true)

	for index, pattern := range r.hardDeny.sshCommand {
		if matchesGlob(command, pattern) {
			return Decision{
				Action:    ActionDeny,
				RuleID:    joinRuleID("hard_deny", index, pattern),
				MatchedBy: "hard_deny",
			}
		}
	}

	for index, item := range r.ssh.overrides {
		if !matchesAnyOrEmpty(service, item.services) {
			continue
		}
		if !matchesAnyOrEmpty(host, item.hosts) {
			continue
		}
		if !matchesAnyOrEmpty(channel, item.channels) {
			continue
		}
		if !matchesAnyOrEmpty(command, item.commandGlobs) {
			continue
		}
		ruleID := item.id
		if ruleID == "" {
			ruleID = joinRuleID("override", index, "")
		}
		return Decision{
			Action:    item.action,
			RuleID:    ruleID,
			MatchedBy: "override",
		}
	}

	if matchesAny(command, r.ssh.whitelist) {
		return Decision{
			Action:    r.defaults.WhitelistAction,
			RuleID:    "whitelist",
			MatchedBy: "whitelist",
		}
	}
	if matchesAny(command, r.ssh.blacklist) {
		return Decision{
			Action:    r.defaults.BlacklistAction,
			RuleID:    "blacklist",
			MatchedBy: "blacklist",
		}
	}

	return Decision{
		Action:    r.defaults.UnmatchedAction,
		RuleID:    "default",
		MatchedBy: "default",
	}
}

// EvaluateCapability authorizes a connector capability invocation.
// Read-only capabilities are directly executed; non-read-only default to require_approval.
// MCP/skill capabilities also check hard_deny.mcp_skill patterns.
func (r *Resolver) EvaluateCapability(input CapabilityInput) Decision {
	if r == nil {
		return Decision{Action: ActionRequireApproval, MatchedBy: "default"}
	}

	capID := normalizeValue(input.CapabilityID, true)
	source := normalizeValue(input.Source, true)

	// Check MCP/skill hard deny
	if source == "mcp" || source == "skill" {
		for index, pattern := range r.hardDeny.mcpSkill {
			if matchesGlob(capID, pattern) {
				return Decision{
					Action:    ActionDeny,
					RuleID:    joinRuleID("hard_deny_mcp_skill", index, pattern),
					MatchedBy: "hard_deny",
				}
			}
		}
	}

	// Read-only capabilities are directly executed
	if input.ReadOnly {
		return Decision{
			Action:    ActionDirectExecute,
			RuleID:    "capability_read_only",
			MatchedBy: "capability_read_only",
		}
	}

	// Non-read-only capabilities require approval by default
	return Decision{
		Action:    ActionRequireApproval,
		RuleID:    "capability_default",
		MatchedBy: "capability_default",
	}
}

func normalizePatterns(values []string, normalizeWhitespace bool) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeValue(value, normalizeWhitespace)
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}

func normalizeValue(value string, normalizeWhitespace bool) string {
	trimmed := strings.TrimSpace(value)
	if normalizeWhitespace {
		trimmed = strings.Join(strings.Fields(trimmed), " ")
	}
	return strings.ToLower(trimmed)
}

func matchesAny(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchesGlob(value, pattern) {
			return true
		}
	}
	return false
}

func matchesAnyOrEmpty(value string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	return matchesAny(value, patterns)
}

func matchesGlob(value string, pattern string) bool {
	if strings.TrimSpace(pattern) == "" {
		return false
	}

	var builder strings.Builder
	builder.WriteString("^")
	for _, char := range pattern {
		switch char {
		case '*':
			builder.WriteString(".*")
		case '?':
			builder.WriteString(".")
		default:
			builder.WriteString(regexp.QuoteMeta(string(char)))
		}
	}
	builder.WriteString("$")

	matcher, err := regexp.Compile(builder.String())
	if err != nil {
		return false
	}
	return matcher.MatchString(value)
}

func joinRuleID(prefix string, _ int, pattern string) string {
	if strings.TrimSpace(pattern) == "" {
		return prefix
	}
	return prefix + ":" + strings.TrimSpace(pattern)
}

func parseAction(raw string, fallback Action) Action {
	switch Action(strings.TrimSpace(raw)) {
	case ActionDirectExecute, ActionRequireApproval, ActionSuggestOnly, ActionDeny:
		return Action(strings.TrimSpace(raw))
	default:
		return fallback
	}
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, item := range in {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
