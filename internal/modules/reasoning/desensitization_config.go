package reasoning

import (
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type DesensitizationConfig struct {
	Enabled        bool
	Secrets        SecretRedactionConfig
	Placeholders   PlaceholderConfig
	Rehydration    RehydrationConfig
	LocalLLMAssist LocalLLMAssistConfig
}

type SecretRedactionConfig struct {
	KeyNames           []string
	QueryKeyNames      []string
	AdditionalPatterns []string
	RedactBearer       bool
	RedactBasicAuthURL bool
	RedactSKTokens     bool
}

type PlaceholderConfig struct {
	HostKeyFragments  []string
	PathKeyFragments  []string
	ReplaceInlineIP   bool
	ReplaceInlineHost bool
	ReplaceInlinePath bool
}

type RehydrationConfig struct {
	Host bool
	IP   bool
	Path bool
}

type LocalLLMAssistConfig struct {
	Enabled  bool
	Provider string
	BaseURL  string
	Model    string
	Mode     string
}

type desensitizationFileConfig struct {
	Desensitization struct {
		Enabled *bool `yaml:"enabled,omitempty"`
		Secrets struct {
			KeyNames           []string `yaml:"key_names,omitempty"`
			QueryKeyNames      []string `yaml:"query_key_names,omitempty"`
			AdditionalPatterns []string `yaml:"additional_patterns,omitempty"`
			RedactBearer       *bool    `yaml:"redact_bearer,omitempty"`
			RedactBasicAuthURL *bool    `yaml:"redact_basic_auth_url,omitempty"`
			RedactSKTokens     *bool    `yaml:"redact_sk_tokens,omitempty"`
		} `yaml:"secrets"`
		Placeholders struct {
			HostKeyFragments  []string `yaml:"host_key_fragments,omitempty"`
			PathKeyFragments  []string `yaml:"path_key_fragments,omitempty"`
			ReplaceInlineIP   *bool    `yaml:"replace_inline_ip,omitempty"`
			ReplaceInlineHost *bool    `yaml:"replace_inline_host,omitempty"`
			ReplaceInlinePath *bool    `yaml:"replace_inline_path,omitempty"`
		} `yaml:"placeholders"`
		Rehydration struct {
			Host *bool `yaml:"host,omitempty"`
			IP   *bool `yaml:"ip,omitempty"`
			Path *bool `yaml:"path,omitempty"`
		} `yaml:"rehydration"`
		LocalLLMAssist struct {
			Enabled  *bool  `yaml:"enabled,omitempty"`
			Provider string `yaml:"provider,omitempty"`
			BaseURL  string `yaml:"base_url,omitempty"`
			Model    string `yaml:"model,omitempty"`
			Mode     string `yaml:"mode,omitempty"`
		} `yaml:"local_llm_assist,omitempty"`
	} `yaml:"desensitization"`
}

func DefaultDesensitizationConfig() DesensitizationConfig {
	return DesensitizationConfig{
		Enabled: true,
		Secrets: SecretRedactionConfig{
			KeyNames:           []string{"password", "passwd", "token", "secret", "api_key"},
			QueryKeyNames:      []string{"access_token", "refresh_token", "token", "secret", "api_key"},
			RedactBearer:       true,
			RedactBasicAuthURL: true,
			RedactSKTokens:     true,
		},
		Placeholders: PlaceholderConfig{
			HostKeyFragments:  []string{"host", "hostname", "instance", "node", "address"},
			PathKeyFragments:  []string{"path", "file", "filename", "dir", "directory"},
			ReplaceInlineIP:   true,
			ReplaceInlineHost: true,
			ReplaceInlinePath: true,
		},
		Rehydration: RehydrationConfig{
			Host: true,
			IP:   true,
			Path: true,
		},
		LocalLLMAssist: LocalLLMAssistConfig{
			Enabled:  false,
			Provider: "openai_compatible",
			Mode:     "detect_only",
		},
	}
}

func LoadDesensitizationConfig(path string) (*DesensitizationConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseDesensitizationConfig(content)
}

func ParseDesensitizationConfig(content []byte) (*DesensitizationConfig, error) {
	var cfg desensitizationFileConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}

	out := DefaultDesensitizationConfig()
	if cfg.Desensitization.Enabled != nil {
		out.Enabled = *cfg.Desensitization.Enabled
	}
	if len(cfg.Desensitization.Secrets.KeyNames) > 0 {
		out.Secrets.KeyNames = cloneStrings(cfg.Desensitization.Secrets.KeyNames)
	}
	if len(cfg.Desensitization.Secrets.QueryKeyNames) > 0 {
		out.Secrets.QueryKeyNames = cloneStrings(cfg.Desensitization.Secrets.QueryKeyNames)
	}
	if len(cfg.Desensitization.Secrets.AdditionalPatterns) > 0 {
		for _, pattern := range cfg.Desensitization.Secrets.AdditionalPatterns {
			if _, err := regexp.Compile(pattern); err != nil {
				return nil, err
			}
		}
		out.Secrets.AdditionalPatterns = cloneStrings(cfg.Desensitization.Secrets.AdditionalPatterns)
	}
	if cfg.Desensitization.Secrets.RedactBearer != nil {
		out.Secrets.RedactBearer = *cfg.Desensitization.Secrets.RedactBearer
	}
	if cfg.Desensitization.Secrets.RedactBasicAuthURL != nil {
		out.Secrets.RedactBasicAuthURL = *cfg.Desensitization.Secrets.RedactBasicAuthURL
	}
	if cfg.Desensitization.Secrets.RedactSKTokens != nil {
		out.Secrets.RedactSKTokens = *cfg.Desensitization.Secrets.RedactSKTokens
	}
	if len(cfg.Desensitization.Placeholders.HostKeyFragments) > 0 {
		out.Placeholders.HostKeyFragments = cloneStrings(cfg.Desensitization.Placeholders.HostKeyFragments)
	}
	if len(cfg.Desensitization.Placeholders.PathKeyFragments) > 0 {
		out.Placeholders.PathKeyFragments = cloneStrings(cfg.Desensitization.Placeholders.PathKeyFragments)
	}
	if cfg.Desensitization.Placeholders.ReplaceInlineIP != nil {
		out.Placeholders.ReplaceInlineIP = *cfg.Desensitization.Placeholders.ReplaceInlineIP
	}
	if cfg.Desensitization.Placeholders.ReplaceInlineHost != nil {
		out.Placeholders.ReplaceInlineHost = *cfg.Desensitization.Placeholders.ReplaceInlineHost
	}
	if cfg.Desensitization.Placeholders.ReplaceInlinePath != nil {
		out.Placeholders.ReplaceInlinePath = *cfg.Desensitization.Placeholders.ReplaceInlinePath
	}
	if cfg.Desensitization.Rehydration.Host != nil {
		out.Rehydration.Host = *cfg.Desensitization.Rehydration.Host
	}
	if cfg.Desensitization.Rehydration.IP != nil {
		out.Rehydration.IP = *cfg.Desensitization.Rehydration.IP
	}
	if cfg.Desensitization.Rehydration.Path != nil {
		out.Rehydration.Path = *cfg.Desensitization.Rehydration.Path
	}
	if cfg.Desensitization.LocalLLMAssist.Enabled != nil {
		out.LocalLLMAssist.Enabled = *cfg.Desensitization.LocalLLMAssist.Enabled
	}
	if strings.TrimSpace(cfg.Desensitization.LocalLLMAssist.Provider) != "" {
		out.LocalLLMAssist.Provider = strings.TrimSpace(cfg.Desensitization.LocalLLMAssist.Provider)
	}
	out.LocalLLMAssist.BaseURL = strings.TrimSpace(cfg.Desensitization.LocalLLMAssist.BaseURL)
	out.LocalLLMAssist.Model = strings.TrimSpace(cfg.Desensitization.LocalLLMAssist.Model)
	if strings.TrimSpace(cfg.Desensitization.LocalLLMAssist.Mode) != "" {
		out.LocalLLMAssist.Mode = strings.TrimSpace(cfg.Desensitization.LocalLLMAssist.Mode)
	}
	return &out, nil
}

func EncodeDesensitizationConfig(cfg *DesensitizationConfig) (string, error) {
	current := DefaultDesensitizationConfig()
	if cfg != nil {
		current = normalizeDesensitizationConfig(*cfg)
	}

	var out desensitizationFileConfig
	out.Desensitization.Enabled = boolPtr(current.Enabled)
	out.Desensitization.Secrets.KeyNames = cloneStrings(current.Secrets.KeyNames)
	out.Desensitization.Secrets.QueryKeyNames = cloneStrings(current.Secrets.QueryKeyNames)
	out.Desensitization.Secrets.AdditionalPatterns = cloneStrings(current.Secrets.AdditionalPatterns)
	out.Desensitization.Secrets.RedactBearer = boolPtr(current.Secrets.RedactBearer)
	out.Desensitization.Secrets.RedactBasicAuthURL = boolPtr(current.Secrets.RedactBasicAuthURL)
	out.Desensitization.Secrets.RedactSKTokens = boolPtr(current.Secrets.RedactSKTokens)
	out.Desensitization.Placeholders.HostKeyFragments = cloneStrings(current.Placeholders.HostKeyFragments)
	out.Desensitization.Placeholders.PathKeyFragments = cloneStrings(current.Placeholders.PathKeyFragments)
	out.Desensitization.Placeholders.ReplaceInlineIP = boolPtr(current.Placeholders.ReplaceInlineIP)
	out.Desensitization.Placeholders.ReplaceInlineHost = boolPtr(current.Placeholders.ReplaceInlineHost)
	out.Desensitization.Placeholders.ReplaceInlinePath = boolPtr(current.Placeholders.ReplaceInlinePath)
	out.Desensitization.Rehydration.Host = boolPtr(current.Rehydration.Host)
	out.Desensitization.Rehydration.IP = boolPtr(current.Rehydration.IP)
	out.Desensitization.Rehydration.Path = boolPtr(current.Rehydration.Path)
	out.Desensitization.LocalLLMAssist.Enabled = boolPtr(current.LocalLLMAssist.Enabled)
	out.Desensitization.LocalLLMAssist.Provider = current.LocalLLMAssist.Provider
	out.Desensitization.LocalLLMAssist.BaseURL = current.LocalLLMAssist.BaseURL
	out.Desensitization.LocalLLMAssist.Model = current.LocalLLMAssist.Model
	out.Desensitization.LocalLLMAssist.Mode = current.LocalLLMAssist.Mode

	content, err := yaml.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func normalizeDesensitizationConfig(cfg DesensitizationConfig) DesensitizationConfig {
	out := DefaultDesensitizationConfig()
	out.Enabled = cfg.Enabled
	if len(cfg.Secrets.KeyNames) > 0 {
		out.Secrets.KeyNames = cloneStrings(cfg.Secrets.KeyNames)
	}
	if len(cfg.Secrets.QueryKeyNames) > 0 {
		out.Secrets.QueryKeyNames = cloneStrings(cfg.Secrets.QueryKeyNames)
	}
	out.Secrets.AdditionalPatterns = cloneStrings(cfg.Secrets.AdditionalPatterns)
	out.Secrets.RedactBearer = cfg.Secrets.RedactBearer
	out.Secrets.RedactBasicAuthURL = cfg.Secrets.RedactBasicAuthURL
	out.Secrets.RedactSKTokens = cfg.Secrets.RedactSKTokens
	if len(cfg.Placeholders.HostKeyFragments) > 0 {
		out.Placeholders.HostKeyFragments = cloneStrings(cfg.Placeholders.HostKeyFragments)
	}
	if len(cfg.Placeholders.PathKeyFragments) > 0 {
		out.Placeholders.PathKeyFragments = cloneStrings(cfg.Placeholders.PathKeyFragments)
	}
	out.Placeholders.ReplaceInlineIP = cfg.Placeholders.ReplaceInlineIP
	out.Placeholders.ReplaceInlineHost = cfg.Placeholders.ReplaceInlineHost
	out.Placeholders.ReplaceInlinePath = cfg.Placeholders.ReplaceInlinePath
	out.Rehydration.Host = cfg.Rehydration.Host
	out.Rehydration.IP = cfg.Rehydration.IP
	out.Rehydration.Path = cfg.Rehydration.Path
	out.LocalLLMAssist.Enabled = cfg.LocalLLMAssist.Enabled
	if strings.TrimSpace(cfg.LocalLLMAssist.Provider) != "" {
		out.LocalLLMAssist.Provider = strings.TrimSpace(cfg.LocalLLMAssist.Provider)
	}
	out.LocalLLMAssist.BaseURL = strings.TrimSpace(cfg.LocalLLMAssist.BaseURL)
	out.LocalLLMAssist.Model = strings.TrimSpace(cfg.LocalLLMAssist.Model)
	if strings.TrimSpace(cfg.LocalLLMAssist.Mode) != "" {
		out.LocalLLMAssist.Mode = strings.TrimSpace(cfg.LocalLLMAssist.Mode)
	}
	return out
}

func boolPtr(value bool) *bool {
	return &value
}

func cloneStrings(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	out := make([]string, 0, len(input))
	for _, item := range input {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
