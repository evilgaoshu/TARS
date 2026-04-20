package app

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"tars/internal/foundation/audit"
	"tars/internal/foundation/config"
	"tars/internal/foundation/logger"
	foundationmetrics "tars/internal/foundation/metrics"
	"tars/internal/foundation/observability"
	"tars/internal/foundation/secrets"
	"tars/internal/modules/access"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/connectors"
	"tars/internal/modules/extensions"
	"tars/internal/modules/knowledge"
	"tars/internal/modules/org"
	"tars/internal/modules/reasoning"
	"tars/internal/modules/skills"
	"tars/internal/modules/sshcredentials"
	postgresrepo "tars/internal/repo/postgres"
	sqlitevecrepo "tars/internal/repo/sqlitevec"
)

var ErrRuntimeConfigPostgresRequired = errors.New("runtime config requires postgres")

type bootstrapShared struct {
	cfg                    config.Config
	logger                 *slog.Logger
	metrics                *foundationmetrics.Registry
	observability          *observability.Store
	auditLogger            audit.Logger
	db                     *sql.DB
	vectorStore            knowledge.VectorStore
	runtimeConfigStore     *postgresrepo.RuntimeConfigStore
	authorizationManager   *authorization.Manager
	approvalManager        *approvalrouting.Manager
	accessManager          *access.Manager
	orgManager             *org.Manager
	promptManager          *reasoning.PromptManager
	desensitizationManager *reasoning.DesensitizationManager
	providerManager        *reasoning.ProviderManager
	secretStore            *secrets.Store
	sshCredentialManager   *sshcredentials.Manager
	connectorManager       *connectors.Manager
	skillManager           *skills.Manager
	extensionManager       *extensions.Manager
	agentRoleManager       *agentrole.Manager
}

type postgresBootstrap struct {
	db                 *sql.DB
	auditLogger        audit.Logger
	runtimeConfigStore *postgresrepo.RuntimeConfigStore
}

var openBootstrapPostgres = func(ctx context.Context, cfg config.Config, log *slog.Logger, observabilityStore *observability.Store, auditLogger audit.Logger) (postgresBootstrap, error) {
	openedDB, err := sql.Open("pgx", cfg.Postgres.DSN)
	if err != nil {
		return postgresBootstrap{}, err
	}
	if err := openedDB.Ping(); err != nil {
		_ = openedDB.Close()
		return postgresBootstrap{}, err
	}
	if err := postgresrepo.EnsureSchema(ctx, openedDB); err != nil {
		_ = openedDB.Close()
		return postgresBootstrap{}, err
	}
	return postgresBootstrap{
		db:                 openedDB,
		auditLogger:        audit.NewComposite(auditLogger, audit.NewPostgres(openedDB, log)),
		runtimeConfigStore: postgresrepo.NewRuntimeConfigStore(openedDB),
	}, nil
}

var openBootstrapVectorStore = func(path string) (knowledge.VectorStore, error) {
	return sqlitevecrepo.Open(path)
}

var sshSecretVaultFactory = func(db *sql.DB, masterKey string, keyID string) (sshcredentials.SecretBackend, error) {
	return postgresrepo.NewEncryptedSecretVault(db, masterKey, keyID)
}

var sshCredentialRepoFactory = func(db *sql.DB) sshcredentials.Repository {
	return postgresrepo.NewSSHCredentialRepository(db)
}

func buildSharedBootstrap(cfg config.Config) (*bootstrapShared, error) {
	metricsRegistry := foundationmetrics.New()
	observabilityStore, err := observability.NewStore(cfg.Observability)
	if err != nil {
		return nil, err
	}
	observabilityStore.SetMetrics(metricsRegistry)
	log := logger.NewWithOptions(cfg, observabilityStore, metricsRegistry)
	auditLogger := audit.NewComposite(audit.NewSlog(log), observability.NewAuditLogger(observabilityStore))
	metricsRegistry.SetRolloutMode(cfg.Features.RolloutMode)
	metricsRegistry.SetFeatureFlag("diagnosis_enabled", cfg.Features.DiagnosisEnabled)
	metricsRegistry.SetFeatureFlag("approval_enabled", cfg.Features.ApprovalEnabled)
	metricsRegistry.SetFeatureFlag("execution_enabled", cfg.Features.ExecutionEnabled)
	metricsRegistry.SetFeatureFlag("knowledge_ingest_enabled", cfg.Features.KnowledgeIngestEnabled)

	shared := &bootstrapShared{
		cfg:                cfg,
		logger:             log,
		metrics:            metricsRegistry,
		observability:      observabilityStore,
		auditLogger:        auditLogger,
		runtimeConfigStore: postgresrepo.NewRuntimeConfigStore(nil),
	}

	if cfg.Postgres.DSN == "" {
		if cfg.RuntimeConfig.RequirePostgres {
			return nil, ErrRuntimeConfigPostgresRequired
		}
		log.Warn("postgres DSN not configured: workflow service is using in-memory storage; data will NOT persist across restarts — set TARS_POSTGRES_DSN for production use")
	} else {
		postgresState, err := openBootstrapPostgres(context.Background(), cfg, log, observabilityStore, auditLogger)
		if err != nil {
			return nil, err
		}
		shared.db = postgresState.db
		shared.auditLogger = postgresState.auditLogger
		if postgresState.runtimeConfigStore != nil {
			shared.runtimeConfigStore = postgresState.runtimeConfigStore
		}
	}

	if strings.TrimSpace(cfg.Vector.SQLitePath) != "" {
		store, err := openBootstrapVectorStore(cfg.Vector.SQLitePath)
		if err != nil {
			return nil, err
		}
		shared.vectorStore = store
	}

	shared.authorizationManager, err = authorization.NewManager(cfg.Authorization.ConfigPath)
	if err != nil {
		return nil, err
	}
	shared.approvalManager, err = approvalrouting.NewManager(cfg.Approval.ConfigPath)
	if err != nil {
		return nil, err
	}
	shared.accessManager, err = access.NewManager(cfg.Access.ConfigPath)
	if err != nil {
		return nil, err
	}
	shared.orgManager, err = org.NewManager(cfg.Org.ConfigPath)
	if err != nil {
		return nil, err
	}
	shared.promptManager, err = reasoning.NewPromptManager(cfg.Reasoning.PromptsConfigPath)
	if err != nil {
		return nil, err
	}
	shared.desensitizationManager, err = reasoning.NewDesensitizationManager(cfg.Reasoning.DesensitizationConfigPath)
	if err != nil {
		return nil, err
	}
	shared.providerManager, err = reasoning.NewProviderManager(cfg.Reasoning.ProvidersConfigPath)
	if err != nil {
		return nil, err
	}
	shared.secretStore, err = secrets.NewStore(firstNonEmpty(cfg.Connectors.SecretsPath, cfg.Reasoning.SecretsConfigPath))
	if err != nil {
		return nil, err
	}
	if shared.db != nil && strings.TrimSpace(cfg.SecretCustody.EncryptionKey) != "" {
		vault, err := sshSecretVaultFactory(shared.db, cfg.SecretCustody.EncryptionKey, cfg.SecretCustody.EncryptionKeyID)
		if err != nil {
			return nil, err
		}
		shared.sshCredentialManager = sshcredentials.NewManager(sshCredentialRepoFactory(shared.db), vault)
		shared.sshCredentialManager.SetAudit(shared.auditLogger)
	}
	shared.connectorManager, err = connectors.NewManager(cfg.Connectors.ConfigPath)
	if err != nil {
		return nil, err
	}
	shared.skillManager, err = skills.NewManager(cfg.Skills.ConfigPath, cfg.Skills.MarketplacePath)
	if err != nil {
		return nil, err
	}

	extensionStatePath := cfg.Extensions.StatePath
	if strings.TrimSpace(extensionStatePath) == "" {
		extensionStatePath = extensions.DefaultStatePath(cfg.Skills.ConfigPath)
	}
	shared.extensionManager, err = extensions.NewManager(extensionStatePath, shared.skillManager)
	if err != nil {
		return nil, err
	}
	shared.agentRoleManager, err = agentrole.NewManager(cfg.AgentRoles.ConfigPath, agentrole.Options{
		Logger: log,
		Audit:  shared.auditLogger,
	})
	if err != nil {
		return nil, err
	}

	if err := shared.restoreRuntimeConfig(context.Background()); err != nil {
		return nil, err
	}

	return shared, nil
}

func (b *bootstrapShared) restoreRuntimeConfig(ctx context.Context) error {
	if b.runtimeConfigStore == nil {
		return nil
	}
	b.accessManager.SetPersistence(func(cfg access.Config) error {
		return b.runtimeConfigStore.SaveAccessConfig(context.Background(), cfg)
	})
	b.providerManager.SetPersistence(func(cfg reasoning.ProvidersConfig) error {
		return b.runtimeConfigStore.SaveProvidersConfig(context.Background(), cfg)
	})
	b.connectorManager.SetPersistence(func(cfg connectors.Config, lifecycle map[string]connectors.LifecycleState) error {
		return b.runtimeConfigStore.SaveConnectorsState(context.Background(), postgresrepo.ConnectorsState{
			Config:    cfg,
			Lifecycle: lifecycle,
			UpdatedAt: time.Now().UTC(),
		})
	})
	b.authorizationManager.SetPersistence(func(cfg authorization.Config) error {
		return b.runtimeConfigStore.SaveAuthorizationConfig(context.Background(), cfg)
	})
	b.approvalManager.SetPersistence(func(cfg approvalrouting.Config) error {
		return b.runtimeConfigStore.SaveApprovalRoutingConfig(context.Background(), cfg)
	})
	b.orgManager.SetPersistence(func(cfg org.Config) error {
		return b.runtimeConfigStore.SaveOrgConfig(context.Background(), cfg)
	})
	b.promptManager.SetPersistence(func(cfg reasoning.PromptSet) error {
		return b.runtimeConfigStore.SaveReasoningPromptsConfig(context.Background(), cfg)
	})
	b.desensitizationManager.SetPersistence(func(cfg reasoning.DesensitizationConfig) error {
		return b.runtimeConfigStore.SaveDesensitizationConfig(context.Background(), cfg)
	})
	b.agentRoleManager.SetPersistence(func(cfg agentrole.Config) error {
		return b.runtimeConfigStore.SaveAgentRolesConfig(context.Background(), cfg)
	})

	if persistedAccess, found, err := b.runtimeConfigStore.LoadAccessConfig(ctx); err != nil {
		return err
	} else if found {
		if err := b.accessManager.SaveConfig(persistedAccess); err != nil {
			return err
		}
	}
	if persistedProviders, found, err := b.runtimeConfigStore.LoadProvidersConfig(ctx); err != nil {
		return err
	} else if found {
		if err := b.providerManager.SaveConfig(persistedProviders); err != nil {
			return err
		}
	}
	if persistedConnectors, found, err := b.runtimeConfigStore.LoadConnectorsState(ctx); err != nil {
		return err
	} else if found {
		b.connectorManager.SetPersistence(nil)
		if err := b.connectorManager.LoadRuntimeState(persistedConnectors.Config, persistedConnectors.Lifecycle); err != nil {
			return err
		}
		b.connectorManager.SetPersistence(func(cfg connectors.Config, lifecycle map[string]connectors.LifecycleState) error {
			return b.runtimeConfigStore.SaveConnectorsState(context.Background(), postgresrepo.ConnectorsState{
				Config:    cfg,
				Lifecycle: lifecycle,
				UpdatedAt: time.Now().UTC(),
			})
		})
	}
	if persistedAuthz, found, err := b.runtimeConfigStore.LoadAuthorizationConfig(ctx); err != nil {
		return err
	} else if found {
		if err := b.authorizationManager.LoadRuntimeConfig(persistedAuthz); err != nil {
			return err
		}
	}
	if persistedApproval, found, err := b.runtimeConfigStore.LoadApprovalRoutingConfig(ctx); err != nil {
		return err
	} else if found {
		if err := b.approvalManager.LoadRuntimeConfig(persistedApproval); err != nil {
			return err
		}
	}
	if persistedOrg, found, err := b.runtimeConfigStore.LoadOrgConfig(ctx); err != nil {
		return err
	} else if found {
		if err := b.orgManager.LoadRuntimeConfig(persistedOrg); err != nil {
			return err
		}
	}
	if persistedPrompts, found, err := b.runtimeConfigStore.LoadReasoningPromptsConfig(ctx); err != nil {
		return err
	} else if found {
		if err := b.promptManager.LoadRuntimePromptSet(persistedPrompts); err != nil {
			return err
		}
	}
	if persistedDesense, found, err := b.runtimeConfigStore.LoadDesensitizationConfig(ctx); err != nil {
		return err
	} else if found {
		if err := b.desensitizationManager.LoadRuntimeConfig(persistedDesense); err != nil {
			return err
		}
	}
	if persistedRoles, found, err := b.runtimeConfigStore.LoadAgentRolesConfig(ctx); err != nil {
		return err
	} else if found {
		if err := b.agentRoleManager.LoadRuntimeConfig(persistedRoles); err != nil {
			return err
		}
	}
	return nil
}
