package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	LogLevel      string
	Server        ServerConfig
	Web           WebConfig
	Postgres      PostgresConfig
	RuntimeConfig RuntimeConfigConfig
	SecretCustody SecretCustodyConfig
	Observability ObservabilityConfig
	Vector        VectorConfig
	OpsAPI        OpsAPIConfig
	GC            GCConfig
	Telegram      TelegramConfig
	VMAlert       VMAlertConfig
	Model         ModelConfig
	Reasoning     ReasoningConfig
	VM            VictoriaMetricsConfig
	SSH           SSHConfig
	Approval      ApprovalConfig
	Connectors    ConnectorConfig
	Extensions    ExtensionConfig
	Skills        SkillConfig
	Automations   AutomationConfig
	AgentRoles    AgentRoleConfig
	Authorization AuthorizationConfig
	Access        AccessConfig
	Output        ExecutionOutputConfig
	Features      FeatureFlags
	Org           OrgConfig
}

type ServerConfig struct {
	Listen        string
	PublicBaseURL string
}

type WebConfig struct {
	DistDir string
}

type PostgresConfig struct {
	DSN string
}

type RuntimeConfigConfig struct {
	RequirePostgres bool
}

type SecretCustodyConfig struct {
	EncryptionKey   string
	EncryptionKeyID string
}

type VectorConfig struct {
	SQLitePath string
}

type ObservabilityConfig struct {
	DataDir string
	Metrics ObservabilitySignalConfig
	Logs    ObservabilitySignalConfig
	Traces  ObservabilitySignalConfig
	OTLP    ObservabilityOTLPConfig
}

type ObservabilitySignalConfig struct {
	Retention    time.Duration
	MaxSizeBytes int64
}

type ObservabilityOTLPConfig struct {
	Endpoint      string
	Protocol      string
	Insecure      bool
	MetricsEnable bool
	LogsEnable    bool
	TracesEnable  bool
}

type OpsAPIConfig struct {
	Enabled bool
	Listen  string
	Token   string
}

type GCConfig struct {
	Enabled               bool
	Interval              time.Duration
	ExecutionOutputRetain time.Duration
}

type TelegramConfig struct {
	BotToken       string
	WebhookSecret  string
	BaseURL        string
	PollingEnabled bool
	PollTimeout    time.Duration
	PollInterval   time.Duration
}

type VMAlertConfig struct {
	WebhookSecret string
}

type ModelConfig struct {
	Protocol string
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
}

type ReasoningConfig struct {
	PromptsConfigPath          string
	DesensitizationConfigPath  string
	ProvidersConfigPath        string
	SecretsConfigPath          string
	LocalCommandFallbackEnable bool
}

type VictoriaMetricsConfig struct {
	BaseURL string
	Timeout time.Duration
}

type ApprovalConfig struct {
	Timeout    time.Duration
	ConfigPath string
}

type AuthorizationConfig struct {
	ConfigPath string
}

type AccessConfig struct {
	ConfigPath string
}

type OrgConfig struct {
	ConfigPath string
}

type ConnectorConfig struct {
	ConfigPath    string
	SecretsPath   string
	TemplatesPath string
}

type SkillConfig struct {
	ConfigPath      string
	MarketplacePath string
}

type ExtensionConfig struct {
	StatePath string
}

type AutomationConfig struct {
	ConfigPath string
}

type AgentRoleConfig struct {
	ConfigPath string
}

type SSHConfig struct {
	User                    string
	PrivateKeyPath          string
	ConnectTimeout          time.Duration
	CommandTimeout          time.Duration
	AllowedHosts            []string
	AllowedCommandPrefixes  []string
	BlockedCommandFragments []string
	DisableHostKeyChecking  bool
}

type ExecutionOutputConfig struct {
	SpoolDir          string
	MaxPersistedBytes int
	ChunkBytes        int
	Retention         time.Duration
}

type FeatureFlags struct {
	RolloutMode            string
	DiagnosisEnabled       bool
	ApprovalEnabled        bool
	ExecutionEnabled       bool
	KnowledgeIngestEnabled bool
}

func LoadFromEnv() Config {
	features := loadFeatureFlagsFromEnv()

	return Config{
		LogLevel: envString("TARS_LOG_LEVEL", "INFO"),
		Server: ServerConfig{
			Listen:        envString("TARS_SERVER_LISTEN", ":8081"),
			PublicBaseURL: envString("TARS_SERVER_PUBLIC_BASE_URL", "https://tars.example.com"),
		},
		Web: WebConfig{
			DistDir: envString("TARS_WEB_DIST_DIR", "./web/dist"),
		},
		Postgres: PostgresConfig{
			DSN: envString("TARS_POSTGRES_DSN", ""),
		},
		RuntimeConfig: RuntimeConfigConfig{
			RequirePostgres: envBool("TARS_RUNTIME_CONFIG_REQUIRE_POSTGRES", false),
		},
		SecretCustody: SecretCustodyConfig{
			EncryptionKey:   envString("TARS_SECRET_CUSTODY_KEY", ""),
			EncryptionKeyID: envString("TARS_SECRET_CUSTODY_KEY_ID", "local"),
		},
		Observability: ObservabilityConfig{
			DataDir: envString("TARS_OBSERVABILITY_DATA_DIR", "./data/observability"),
			Metrics: ObservabilitySignalConfig{
				Retention:    envDuration("TARS_OBSERVABILITY_METRICS_RETENTION", 168*time.Hour),
				MaxSizeBytes: envInt64("TARS_OBSERVABILITY_METRICS_MAX_SIZE_BYTES", 10*1024*1024*1024),
			},
			Logs: ObservabilitySignalConfig{
				Retention:    envDuration("TARS_OBSERVABILITY_LOGS_RETENTION", 168*time.Hour),
				MaxSizeBytes: envInt64("TARS_OBSERVABILITY_LOGS_MAX_SIZE_BYTES", 10*1024*1024*1024),
			},
			Traces: ObservabilitySignalConfig{
				Retention:    envDuration("TARS_OBSERVABILITY_TRACES_RETENTION", 168*time.Hour),
				MaxSizeBytes: envInt64("TARS_OBSERVABILITY_TRACES_MAX_SIZE_BYTES", 10*1024*1024*1024),
			},
			OTLP: ObservabilityOTLPConfig{
				Endpoint:      envString("TARS_OBSERVABILITY_OTLP_ENDPOINT", ""),
				Protocol:      envString("TARS_OBSERVABILITY_OTLP_PROTOCOL", "grpc"),
				Insecure:      envBool("TARS_OBSERVABILITY_OTLP_INSECURE", false),
				MetricsEnable: envBool("TARS_OBSERVABILITY_OTLP_METRICS_ENABLED", false),
				LogsEnable:    envBool("TARS_OBSERVABILITY_OTLP_LOGS_ENABLED", false),
				TracesEnable:  envBool("TARS_OBSERVABILITY_OTLP_TRACES_ENABLED", false),
			},
		},
		Vector: VectorConfig{
			SQLitePath: envString("TARS_VECTOR_SQLITE_PATH", ""),
		},
		OpsAPI: OpsAPIConfig{
			Enabled: envBool("TARS_OPS_API_ENABLED", false),
			Listen:  envString("TARS_OPS_API_LISTEN", "127.0.0.1:8081"),
			Token:   envString("TARS_OPS_API_TOKEN", ""),
		},
		GC: GCConfig{
			Enabled:               envBool("TARS_GC_ENABLED", true),
			Interval:              envDuration("TARS_GC_INTERVAL", 1*time.Hour),
			ExecutionOutputRetain: envDuration("TARS_EXECUTION_OUTPUT_RETENTION", 168*time.Hour),
		},
		Telegram: TelegramConfig{
			BotToken:       envString("TARS_TELEGRAM_BOT_TOKEN", ""),
			WebhookSecret:  envString("TARS_TELEGRAM_WEBHOOK_SECRET", ""),
			BaseURL:        envString("TARS_TELEGRAM_BASE_URL", "https://api.telegram.org"),
			PollingEnabled: envBool("TARS_TELEGRAM_POLLING_ENABLED", false),
			PollTimeout:    envDuration("TARS_TELEGRAM_POLL_TIMEOUT", 30*time.Second),
			PollInterval:   envDuration("TARS_TELEGRAM_POLL_INTERVAL", 1*time.Second),
		},
		VMAlert: VMAlertConfig{
			WebhookSecret: envString("TARS_VMALERT_WEBHOOK_SECRET", ""),
		},
		Model: ModelConfig{
			Protocol: envString("TARS_MODEL_PROTOCOL", "openai_compatible"),
			BaseURL:  envString("TARS_MODEL_BASE_URL", ""),
			APIKey:   envString("TARS_MODEL_API_KEY", ""),
			Model:    envString("TARS_MODEL_NAME", "gpt-4o-mini"),
			Timeout:  envDuration("TARS_MODEL_TIMEOUT", 30*time.Second),
		},
		Reasoning: ReasoningConfig{
			PromptsConfigPath:          envString("TARS_REASONING_PROMPTS_CONFIG_PATH", ""),
			DesensitizationConfigPath:  envString("TARS_DESENSITIZATION_CONFIG_PATH", ""),
			ProvidersConfigPath:        envString("TARS_PROVIDERS_CONFIG_PATH", ""),
			SecretsConfigPath:          envString("TARS_SECRETS_CONFIG_PATH", ""),
			LocalCommandFallbackEnable: envBool("TARS_REASONING_LOCAL_COMMAND_FALLBACK_ENABLED", false),
		},
		VM: VictoriaMetricsConfig{
			BaseURL: envString("TARS_VM_BASE_URL", ""),
			Timeout: envDuration("TARS_VM_TIMEOUT", 15*time.Second),
		},
		SSH: SSHConfig{
			User:                    envString("TARS_SSH_USER", ""),
			PrivateKeyPath:          envString("TARS_SSH_PRIVATE_KEY_PATH", ""),
			ConnectTimeout:          envDuration("TARS_SSH_CONNECT_TIMEOUT", 10*time.Second),
			CommandTimeout:          envDuration("TARS_SSH_COMMAND_TIMEOUT", 5*time.Minute),
			AllowedHosts:            envCSV("TARS_SSH_ALLOWED_HOSTS"),
			AllowedCommandPrefixes:  envCSV("TARS_SSH_ALLOWED_COMMAND_PREFIXES"),
			BlockedCommandFragments: envCSV("TARS_SSH_BLOCKED_COMMAND_FRAGMENTS"),
			DisableHostKeyChecking:  envBool("TARS_SSH_DISABLE_HOST_KEY_CHECKING", false),
		},
		Output: ExecutionOutputConfig{
			SpoolDir:          envString("TARS_EXECUTION_OUTPUT_SPOOL_DIR", "./data/execution_output"),
			MaxPersistedBytes: envInt("TARS_EXECUTION_OUTPUT_MAX_PERSISTED_BYTES", 262144),
			ChunkBytes:        envInt("TARS_EXECUTION_OUTPUT_CHUNK_BYTES", 16384),
			Retention:         envDuration("TARS_EXECUTION_OUTPUT_RETENTION", 168*time.Hour),
		},
		Approval: ApprovalConfig{
			Timeout:    envDuration("TARS_APPROVAL_TIMEOUT", 15*time.Minute),
			ConfigPath: envString("TARS_APPROVALS_CONFIG_PATH", ""),
		},
		Connectors: ConnectorConfig{
			ConfigPath:    envString("TARS_CONNECTORS_CONFIG_PATH", ""),
			SecretsPath:   envString("TARS_CONNECTORS_SECRETS_PATH", ""),
			TemplatesPath: envString("TARS_CONNECTORS_TEMPLATES_PATH", ""),
		},
		Extensions: ExtensionConfig{
			StatePath: envString("TARS_EXTENSIONS_STATE_PATH", ""),
		},
		Skills: SkillConfig{
			ConfigPath:      envString("TARS_SKILLS_CONFIG_PATH", ""),
			MarketplacePath: envString("TARS_SKILLS_MARKETPLACE_PATH", "./configs/marketplace/skills"),
		},
		Automations: AutomationConfig{
			ConfigPath: envString("TARS_AUTOMATIONS_CONFIG_PATH", "./configs/automations.yaml"),
		},
		AgentRoles: AgentRoleConfig{
			ConfigPath: envString("TARS_AGENT_ROLES_CONFIG_PATH", ""),
		},
		Authorization: AuthorizationConfig{
			ConfigPath: envString("TARS_AUTHORIZATION_CONFIG_PATH", ""),
		},
		Access: AccessConfig{
			ConfigPath: envString("TARS_ACCESS_CONFIG_PATH", ""),
		},
		Org: OrgConfig{
			ConfigPath: envString("TARS_ORG_CONFIG_PATH", ""),
		},
		Features: features,
	}
}

func loadFeatureFlagsFromEnv() FeatureFlags {
	mode := normalizeRolloutMode(envString("TARS_ROLLOUT_MODE", ""))
	flags := featureFlagsForRolloutMode(mode)

	if value, ok := envBoolWithPresence("TARS_FEATURES_DIAGNOSIS_ENABLED"); ok {
		flags.DiagnosisEnabled = value
	}
	if value, ok := envBoolWithPresence("TARS_FEATURES_APPROVAL_ENABLED"); ok {
		flags.ApprovalEnabled = value
	}
	if value, ok := envBoolWithPresence("TARS_FEATURES_EXECUTION_ENABLED"); ok {
		flags.ExecutionEnabled = value
	}
	if value, ok := envBoolWithPresence("TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED"); ok {
		flags.KnowledgeIngestEnabled = value
	}

	return flags
}

func normalizeRolloutMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "diagnosis_only", "approval_beta", "execution_beta", "knowledge_on":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return "custom"
	}
}

func featureFlagsForRolloutMode(mode string) FeatureFlags {
	flags := FeatureFlags{
		RolloutMode:            mode,
		DiagnosisEnabled:       true,
		ApprovalEnabled:        false,
		ExecutionEnabled:       false,
		KnowledgeIngestEnabled: false,
	}

	switch mode {
	case "approval_beta":
		flags.ApprovalEnabled = true
	case "execution_beta":
		flags.ApprovalEnabled = true
		flags.ExecutionEnabled = true
	case "knowledge_on":
		flags.ApprovalEnabled = true
		flags.ExecutionEnabled = true
		flags.KnowledgeIngestEnabled = true
	}

	return flags
}

func envString(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	if value, ok := envBoolWithPresence(key); ok {
		return value
	}
	return fallback
}

func envBoolWithPresence(key string) (bool, bool) {
	value := os.Getenv(key)
	switch value {
	case "1", "true", "TRUE", "yes", "YES":
		return true, true
	case "0", "false", "FALSE", "no", "NO":
		return false, true
	default:
		return false, false
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envCSV(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}

	items := strings.Split(value, ",")
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
