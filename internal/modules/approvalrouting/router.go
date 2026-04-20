package approvalrouting

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Router interface {
	Resolve(alert map[string]interface{}, requester string, fallbackTarget string) Route
	AllowedCommandPrefixes(service string) []string
	ServiceCommandAllowlist() map[string][]string
}

type Route struct {
	GroupKey    string
	SourceLabel string
	Targets     []string
}

type Resolver struct {
	prohibitSelfApproval bool
	serviceOwners        map[string][]string
	oncallGroups         map[string][]string
	commandAllowlist     map[string][]string
}

type Config struct {
	ProhibitSelfApproval bool
	ServiceOwners        map[string][]string
	OncallGroups         map[string][]string
	CommandAllowlist     map[string][]string
}

type fileConfig struct {
	Approval struct {
		ProhibitSelfApproval bool `yaml:"prohibit_self_approval"`
		Routing              struct {
			ServiceOwner map[string][]string `yaml:"service_owner"`
			OncallGroup  map[string][]string `yaml:"oncall_group"`
		} `yaml:"routing"`
		Execution struct {
			CommandAllowlist map[string][]string `yaml:"command_allowlist"`
		} `yaml:"execution"`
	} `yaml:"approval"`
}

func Load(path string) (*Resolver, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg, err := Parse(content)
	if err != nil {
		return nil, err
	}

	return New(cfg), nil
}

func Parse(content []byte) (Config, error) {

	var cfg fileConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return Config{}, err
	}

	return Config{
		ProhibitSelfApproval: cfg.Approval.ProhibitSelfApproval,
		ServiceOwners:        cfg.Approval.Routing.ServiceOwner,
		OncallGroups:         cfg.Approval.Routing.OncallGroup,
		CommandAllowlist:     cfg.Approval.Execution.CommandAllowlist,
	}, nil
}

func Encode(cfg Config) (string, error) {
	out := fileConfig{}
	out.Approval.ProhibitSelfApproval = cfg.ProhibitSelfApproval
	out.Approval.Routing.ServiceOwner = cloneRoutes(cfg.ServiceOwners)
	out.Approval.Routing.OncallGroup = cloneRoutes(cfg.OncallGroups)
	out.Approval.Execution.CommandAllowlist = cloneRoutes(cfg.CommandAllowlist)

	content, err := yaml.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func New(cfg Config) *Resolver {
	return &Resolver{
		prohibitSelfApproval: cfg.ProhibitSelfApproval,
		serviceOwners:        normalizeRoutes(cfg.ServiceOwners),
		oncallGroups:         normalizeRoutes(cfg.OncallGroups),
		commandAllowlist:     normalizeRoutes(cfg.CommandAllowlist),
	}
}

func (r *Resolver) AllowedCommandPrefixes(service string) []string {
	key := strings.TrimSpace(service)
	if key == "" {
		return nil
	}

	items := r.commandAllowlist[key]
	if len(items) == 0 {
		return nil
	}

	out := make([]string, len(items))
	copy(out, items)
	return out
}

func (r *Resolver) ServiceCommandAllowlist() map[string][]string {
	if len(r.commandAllowlist) == 0 {
		return map[string][]string{}
	}

	out := make(map[string][]string, len(r.commandAllowlist))
	for key, items := range r.commandAllowlist {
		cloned := make([]string, len(items))
		copy(cloned, items)
		out[key] = cloned
	}
	return out
}

func (r *Resolver) Resolve(alert map[string]interface{}, requester string, fallbackTarget string) Route {
	service := alertLabel(alert, "service")
	env := alertLabel(alert, "env")

	if route := r.buildRoute("service_owner", service, requester); len(route.Targets) > 0 {
		return route
	}
	if route := r.buildRoute("oncall", service, requester); len(route.Targets) > 0 {
		return route
	}
	if route := r.buildRoute("oncall", env, requester); len(route.Targets) > 0 {
		return route
	}
	if route := r.buildRoute("oncall", "default", requester); len(route.Targets) > 0 {
		return route
	}

	fallbackTarget = strings.TrimSpace(fallbackTarget)
	if fallbackTarget == "" {
		fallbackTarget = "ops-room"
	}
	return Route{
		GroupKey:    "fallback:default",
		SourceLabel: "fallback(default)",
		Targets:     []string{fallbackTarget},
	}
}

func (r *Resolver) buildRoute(kind string, key string, requester string) Route {
	key = strings.TrimSpace(key)
	if key == "" {
		return Route{}
	}

	var candidates []string
	switch kind {
	case "service_owner":
		candidates = r.serviceOwners[key]
	case "oncall":
		candidates = r.oncallGroups[key]
	}
	if len(candidates) == 0 {
		return Route{}
	}

	targets := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, item := range candidates {
		target := strings.TrimSpace(item)
		if target == "" {
			continue
		}
		if r.prohibitSelfApproval && requester != "" && requester == target {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	if len(targets) == 0 {
		return Route{}
	}

	return Route{
		GroupKey:    fmt.Sprintf("%s:%s", kind, key),
		SourceLabel: formatSourceLabel(kind, key),
		Targets:     targets,
	}
}

func normalizeRoutes(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return map[string][]string{}
	}

	out := make(map[string][]string, len(in))
	for key, items := range in {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}

		normalized := make([]string, 0, len(items))
		for _, item := range items {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				normalized = append(normalized, trimmed)
			}
		}
		if len(normalized) > 0 {
			out[trimmedKey] = normalized
		}
	}
	return out
}

func cloneRoutes(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(in))
	for key, items := range in {
		cloned := make([]string, len(items))
		copy(cloned, items)
		out[key] = cloned
	}
	return out
}

func alertLabel(alert map[string]interface{}, key string) string {
	labels, ok := alert["labels"]
	if !ok {
		return ""
	}

	switch typed := labels.(type) {
	case map[string]string:
		return strings.TrimSpace(typed[key])
	case map[string]interface{}:
		if value, ok := typed[key].(string); ok {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func formatSourceLabel(kind string, key string) string {
	switch kind {
	case "service_owner":
		return fmt.Sprintf("service owner(%s)", key)
	case "oncall":
		return fmt.Sprintf("oncall(%s)", key)
	default:
		return fmt.Sprintf("%s(%s)", kind, key)
	}
}
