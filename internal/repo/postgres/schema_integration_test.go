package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestEnsureSchemaMigratesLegacyAgentRoleProviderPreferenceRows(t *testing.T) {
	t.Parallel()

	db := openIsolatedPostgresDB(t)
	defer db.close(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := db.conn.ExecContext(ctx, `
		CREATE TABLE runtime_config_documents (
			document_key TEXT PRIMARY KEY,
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE agent_roles (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			role_id TEXT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
			profile JSONB NOT NULL DEFAULT '{}',
			capability_binding JSONB NOT NULL DEFAULT '{}',
			policy_binding JSONB NOT NULL DEFAULT '{}',
			provider_preference JSONB NOT NULL DEFAULT '{}'::jsonb,
			org_id TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_agent_roles_tenant_role UNIQUE (tenant_id, role_id)
		);
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	if _, err := db.conn.ExecContext(ctx, `
		INSERT INTO runtime_config_documents (document_key, payload) VALUES
		('providers', $1::jsonb)
	`, `{
		"primary":{"provider_id":"openai-main","model":"gpt-4.1-mini"},
		"assist":{"provider_id":"openai-backup","model":"gpt-4o-mini"},
		"entries":[
			{"id":"openai-main","protocol":"openai_compatible","base_url":"https://primary.example.test","enabled":true},
			{"id":"openai-backup","protocol":"openai_compatible","base_url":"https://fallback.example.test","enabled":true}
		]
	}`); err != nil {
		t.Fatalf("seed providers config: %v", err)
	}

	if _, err := db.conn.ExecContext(ctx, `
		INSERT INTO agent_roles (role_id, display_name, provider_preference) VALUES
		('explicit-model', 'Explicit Model', '{"preferred_provider_id":"openai-main","preferred_model":"gpt-4.1-mini","preferred_model_role":"primary"}'::jsonb),
		('role-only', 'Role Only', '{"preferred_provider_id":"openai-main","preferred_model_role":"primary"}'::jsonb),
		('inherit-default', 'Inherit Default', '{}'::jsonb)
	`); err != nil {
		t.Fatalf("seed legacy agent roles: %v", err)
	}

	if err := EnsureSchema(ctx, db.conn); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	if hasColumn(t, ctx, db.conn, "agent_roles", "provider_preference") {
		t.Fatal("expected provider_preference column to be removed after migration")
	}
	if !hasColumn(t, ctx, db.conn, "agent_roles", "model_binding") {
		t.Fatal("expected model_binding column after migration")
	}

	assertModelBinding := func(roleID string, want map[string]any) {
		t.Helper()
		var raw []byte
		if err := db.conn.QueryRowContext(ctx, `SELECT model_binding FROM agent_roles WHERE role_id = $1`, roleID).Scan(&raw); err != nil {
			t.Fatalf("load model_binding for %s: %v", roleID, err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode model_binding for %s: %v", roleID, err)
		}
		if !jsonEqual(got, want) {
			t.Fatalf("unexpected model_binding for %s:\n got: %#v\nwant: %#v", roleID, got, want)
		}
	}

	assertModelBinding("explicit-model", map[string]any{
		"primary": map[string]any{
			"provider_id": "openai-main",
			"model":       "gpt-4.1-mini",
		},
		"inherit_platform_default": false,
	})
	assertModelBinding("role-only", map[string]any{
		"primary": map[string]any{
			"provider_id": "openai-main",
			"model":       "gpt-4.1-mini",
		},
		"inherit_platform_default": false,
	})
	assertModelBinding("inherit-default", map[string]any{
		"inherit_platform_default": true,
	})
}

func TestEnsureSchemaFailsWhenLegacyRoleOnlyBindingCannotResolveModel(t *testing.T) {
	t.Parallel()

	db := openIsolatedPostgresDB(t)
	defer db.close(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := db.conn.ExecContext(ctx, `
		CREATE TABLE runtime_config_documents (
			document_key TEXT PRIMARY KEY,
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE agent_roles (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			role_id TEXT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
			profile JSONB NOT NULL DEFAULT '{}',
			capability_binding JSONB NOT NULL DEFAULT '{}',
			policy_binding JSONB NOT NULL DEFAULT '{}',
			provider_preference JSONB NOT NULL DEFAULT '{}'::jsonb,
			org_id TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_agent_roles_tenant_role UNIQUE (tenant_id, role_id)
		);
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	if _, err := db.conn.ExecContext(ctx, `
		INSERT INTO runtime_config_documents (document_key, payload) VALUES
		('providers', '{"primary":{"provider_id":"different-provider","model":"gpt-4.1-mini"}}'::jsonb);
		INSERT INTO agent_roles (role_id, display_name, provider_preference) VALUES
		('broken-role', 'Broken Role', '{"preferred_provider_id":"openai-main","preferred_model_role":"primary"}'::jsonb);
	`); err != nil {
		t.Fatalf("seed unresolved legacy data: %v", err)
	}

	err := EnsureSchema(ctx, db.conn)
	if err == nil {
		t.Fatal("expected unresolved legacy role-only model binding migration to fail")
	}
	if !strings.Contains(err.Error(), "resolvable explicit model") {
		t.Fatalf("expected resolvable explicit model error, got %v", err)
	}
}

func TestEnsureSchemaSkipsLegacyMsgTemplateRenameWhenNotificationTemplatesAlreadyExists(t *testing.T) {
	t.Parallel()

	db := openIsolatedPostgresDB(t)
	defer db.close(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := db.conn.ExecContext(ctx, `
		CREATE TABLE notification_templates (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL DEFAULT '',
			locale TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			subject TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE msg_templates (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL DEFAULT '',
			locale TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`); err != nil {
		t.Fatalf("seed existing template tables: %v", err)
	}

	if err := EnsureSchema(ctx, db.conn); err != nil {
		t.Fatalf("ensure schema with existing notification_templates: %v", err)
	}
}

type isolatedPostgresDB struct {
	conn      *sql.DB
	adminConn *sql.DB
	name      string
}

func openIsolatedPostgresDB(t *testing.T) *isolatedPostgresDB {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("TARS_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set TARS_TEST_POSTGRES_DSN to run live Postgres schema integration tests")
	}
	adminDSN := mustSwapPostgresDatabase(t, dsn, "postgres")
	adminConn, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("open admin postgres connection: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := adminConn.PingContext(ctx); err != nil {
		t.Fatalf("ping admin postgres connection: %v", err)
	}

	name := "tars_schema_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := adminConn.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, name)); err != nil {
		t.Fatalf("create isolated database %s: %v", name, err)
	}
	conn, err := sql.Open("pgx", mustSwapPostgresDatabase(t, dsn, name))
	if err != nil {
		t.Fatalf("open isolated postgres connection: %v", err)
	}
	if err := conn.PingContext(ctx); err != nil {
		t.Fatalf("ping isolated postgres connection: %v", err)
	}
	return &isolatedPostgresDB{conn: conn, adminConn: adminConn, name: name}
}

func (db *isolatedPostgresDB) close(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if db.conn != nil {
		_ = db.conn.Close()
	}
	if db.adminConn != nil {
		if _, err := db.adminConn.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s" WITH (FORCE)`, db.name)); err != nil {
			t.Fatalf("drop isolated database %s: %v", db.name, err)
		}
		_ = db.adminConn.Close()
	}
}

func mustSwapPostgresDatabase(t *testing.T, dsn string, database string) string {
	t.Helper()

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse postgres dsn %q: %v", dsn, err)
	}
	parsed.Path = "/" + database
	return parsed.String()
}

func hasColumn(t *testing.T, ctx context.Context, db *sql.DB, table string, column string) bool {
	t.Helper()

	var exists bool
	if err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = $1
			  AND column_name = $2
		)
	`, table, column).Scan(&exists); err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	return exists
}

func jsonEqual(left map[string]any, right map[string]any) bool {
	leftRaw, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightRaw, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftRaw) == string(rightRaw)
}
