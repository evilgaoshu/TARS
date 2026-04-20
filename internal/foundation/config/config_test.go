package config

import "testing"

func TestLoadFromEnvRolloutModeDiagnosisOnly(t *testing.T) {
	t.Setenv("TARS_ROLLOUT_MODE", "diagnosis_only")
	t.Setenv("TARS_FEATURES_DIAGNOSIS_ENABLED", "")
	t.Setenv("TARS_FEATURES_APPROVAL_ENABLED", "")
	t.Setenv("TARS_FEATURES_EXECUTION_ENABLED", "")
	t.Setenv("TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED", "")

	cfg := LoadFromEnv()

	if cfg.Features.RolloutMode != "diagnosis_only" {
		t.Fatalf("expected rollout mode diagnosis_only, got %q", cfg.Features.RolloutMode)
	}
	if !cfg.Features.DiagnosisEnabled {
		t.Fatalf("expected diagnosis to be enabled")
	}
	if cfg.Features.ApprovalEnabled {
		t.Fatalf("expected approval to be disabled")
	}
	if cfg.Features.ExecutionEnabled {
		t.Fatalf("expected execution to be disabled")
	}
	if cfg.Features.KnowledgeIngestEnabled {
		t.Fatalf("expected knowledge ingest to be disabled")
	}
}

func TestLoadFromEnvRolloutModeCanBeOverriddenPerFeature(t *testing.T) {
	t.Setenv("TARS_ROLLOUT_MODE", "approval_beta")
	t.Setenv("TARS_FEATURES_EXECUTION_ENABLED", "true")
	t.Setenv("TARS_FEATURES_KNOWLEDGE_INGEST_ENABLED", "true")

	cfg := LoadFromEnv()

	if cfg.Features.RolloutMode != "approval_beta" {
		t.Fatalf("expected rollout mode approval_beta, got %q", cfg.Features.RolloutMode)
	}
	if !cfg.Features.DiagnosisEnabled || !cfg.Features.ApprovalEnabled {
		t.Fatalf("expected diagnosis and approval to stay enabled")
	}
	if !cfg.Features.ExecutionEnabled {
		t.Fatalf("expected execution override to enable execution")
	}
	if !cfg.Features.KnowledgeIngestEnabled {
		t.Fatalf("expected knowledge ingest override to enable knowledge ingest")
	}
}

func TestLoadFromEnvInvalidRolloutModeFallsBackToCustom(t *testing.T) {
	t.Setenv("TARS_ROLLOUT_MODE", "unsupported")
	t.Setenv("TARS_FEATURES_DIAGNOSIS_ENABLED", "false")
	t.Setenv("TARS_FEATURES_APPROVAL_ENABLED", "true")

	cfg := LoadFromEnv()

	if cfg.Features.RolloutMode != "custom" {
		t.Fatalf("expected rollout mode custom, got %q", cfg.Features.RolloutMode)
	}
	if cfg.Features.DiagnosisEnabled {
		t.Fatalf("expected diagnosis override to be applied")
	}
	if !cfg.Features.ApprovalEnabled {
		t.Fatalf("expected approval override to be applied")
	}
	if cfg.Features.ExecutionEnabled {
		t.Fatalf("expected execution to remain disabled")
	}
}

func TestLoadFromEnvExtensionStatePath(t *testing.T) {
	t.Setenv("TARS_EXTENSIONS_STATE_PATH", "/tmp/extensions.state.yaml")

	cfg := LoadFromEnv()

	if cfg.Extensions.StatePath != "/tmp/extensions.state.yaml" {
		t.Fatalf("expected extensions state path to load, got %q", cfg.Extensions.StatePath)
	}
}

func TestLoadFromEnvDefaultsMainServerTo8081(t *testing.T) {
	t.Setenv("TARS_SERVER_LISTEN", "")
	t.Setenv("TARS_OPS_API_LISTEN", "")

	cfg := LoadFromEnv()

	if cfg.Server.Listen != ":8081" {
		t.Fatalf("expected main server listen default :8081, got %q", cfg.Server.Listen)
	}
}

func TestLoadFromEnvRuntimeConfigRequirePostgres(t *testing.T) {
	t.Setenv("TARS_RUNTIME_CONFIG_REQUIRE_POSTGRES", "true")

	cfg := LoadFromEnv()

	if !cfg.RuntimeConfig.RequirePostgres {
		t.Fatalf("expected runtime config postgres requirement to load")
	}
}

func TestLoadFromEnvKeepsHostKeyCheckingEnabledByDefault(t *testing.T) {
	t.Setenv("TARS_SSH_DISABLE_HOST_KEY_CHECKING", "")

	cfg := LoadFromEnv()

	if cfg.SSH.DisableHostKeyChecking {
		t.Fatalf("expected ssh host key checking to remain enabled by default")
	}
}
