-- Migration 0004: Identity & Access Baseline Tables
-- These tables provide a Postgres-native backing store for the IAM platform.
-- The current default implementation uses YAML file persistence (access.Manager).
-- These tables are designed to be the long-term home for IAM data when
-- the platform grows to multi-tenant or high-availability deployments.
-- Status: baseline schema — not yet wired to the default runtime (YAML remains primary).

-- ─── Enum types ────────────────────────────────────────────────────────────

DO $$ BEGIN
  CREATE TYPE user_status AS ENUM ('active', 'disabled', 'invited');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE group_status AS ENUM ('active', 'disabled');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE auth_provider_type AS ENUM ('local_token', 'oidc', 'oauth2', 'ldap');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ─── Users ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS iam_users (
  id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      TEXT         NOT NULL DEFAULT 'default',
  user_id        TEXT         NOT NULL,           -- platform user identifier
  username       TEXT         NOT NULL,
  display_name   TEXT         NOT NULL DEFAULT '',
  email          TEXT         NOT NULL DEFAULT '',
  status         user_status  NOT NULL DEFAULT 'active',
  source         TEXT         NOT NULL DEFAULT 'local',  -- local / oidc / oauth2 / ldap / ops_token
  created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_iam_users_tenant_user UNIQUE (tenant_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_iam_users_tenant   ON iam_users (tenant_id);
CREATE INDEX IF NOT EXISTS idx_iam_users_email    ON iam_users (tenant_id, email);
CREATE INDEX IF NOT EXISTS idx_iam_users_status   ON iam_users (tenant_id, status);

-- ─── User identities (user_identities) ────────────────────────────────────

CREATE TABLE IF NOT EXISTS iam_user_identities (
  id                UUID   PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id         TEXT   NOT NULL DEFAULT 'default',
  user_id           TEXT   NOT NULL,              -- FK -> iam_users.user_id (within tenant)
  provider_type     TEXT   NOT NULL,              -- oidc / oauth2 / ldap / telegram / local_token
  provider_id       TEXT   NOT NULL,              -- auth_provider.id
  external_subject  TEXT   NOT NULL,              -- sub / uid / dn from external IdP
  external_username TEXT   NOT NULL DEFAULT '',
  external_email    TEXT   NOT NULL DEFAULT '',
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_iam_user_identity UNIQUE (tenant_id, provider_id, external_subject)
);

CREATE INDEX IF NOT EXISTS idx_iam_user_identities_user ON iam_user_identities (tenant_id, user_id);

-- ─── Groups ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS iam_groups (
  id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    TEXT         NOT NULL DEFAULT 'default',
  group_id     TEXT         NOT NULL,
  display_name TEXT         NOT NULL DEFAULT '',
  description  TEXT         NOT NULL DEFAULT '',
  status       group_status NOT NULL DEFAULT 'active',
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_iam_groups_tenant_group UNIQUE (tenant_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_iam_groups_tenant  ON iam_groups (tenant_id);
CREATE INDEX IF NOT EXISTS idx_iam_groups_status  ON iam_groups (tenant_id, status);

-- ─── Group memberships ─────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS iam_group_memberships (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  TEXT        NOT NULL DEFAULT 'default',
  group_id   TEXT        NOT NULL,    -- FK -> iam_groups.group_id (within tenant)
  user_id    TEXT        NOT NULL,    -- FK -> iam_users.user_id (within tenant)
  added_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_iam_group_membership UNIQUE (tenant_id, group_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_iam_group_memberships_group ON iam_group_memberships (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_iam_group_memberships_user  ON iam_group_memberships (tenant_id, user_id);

-- ─── Roles ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS iam_roles (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    TEXT        NOT NULL DEFAULT 'default',
  role_id      TEXT        NOT NULL,
  display_name TEXT        NOT NULL DEFAULT '',
  permissions  JSONB       NOT NULL DEFAULT '[]',
  is_system    BOOLEAN     NOT NULL DEFAULT FALSE,  -- system roles cannot be deleted
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_iam_roles_tenant_role UNIQUE (tenant_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_iam_roles_tenant ON iam_roles (tenant_id);

-- ─── Role bindings ─────────────────────────────────────────────────────────
-- Binds a role to either a user or a group (at most one of user_id / group_id is set)

CREATE TABLE IF NOT EXISTS iam_role_bindings (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   TEXT        NOT NULL DEFAULT 'default',
  role_id     TEXT        NOT NULL,   -- FK -> iam_roles.role_id (within tenant)
  subject_type TEXT        NOT NULL,  -- 'user' | 'group'
  subject_id  TEXT        NOT NULL,   -- user_id or group_id
  bound_by    TEXT        NOT NULL DEFAULT '',   -- operator who created this binding
  reason      TEXT        NOT NULL DEFAULT '',   -- operator reason (audit)
  bound_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_iam_role_binding UNIQUE (tenant_id, role_id, subject_type, subject_id)
);

CREATE INDEX IF NOT EXISTS idx_iam_role_bindings_role    ON iam_role_bindings (tenant_id, role_id);
CREATE INDEX IF NOT EXISTS idx_iam_role_bindings_subject ON iam_role_bindings (tenant_id, subject_type, subject_id);

-- ─── Auth providers ────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS iam_auth_providers (
  id                  UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           TEXT              NOT NULL DEFAULT 'default',
  provider_id         TEXT              NOT NULL,
  provider_type       auth_provider_type NOT NULL DEFAULT 'oidc',
  name                TEXT              NOT NULL DEFAULT '',
  enabled             BOOLEAN           NOT NULL DEFAULT TRUE,
  client_id           TEXT              NOT NULL DEFAULT '',
  -- client_secret is intentionally NOT stored in Postgres in plaintext;
  -- use client_secret_ref to point at a secret manager entry
  client_secret_ref   TEXT              NOT NULL DEFAULT '',
  auth_url            TEXT              NOT NULL DEFAULT '',
  token_url           TEXT              NOT NULL DEFAULT '',
  userinfo_url        TEXT              NOT NULL DEFAULT '',
  scopes              JSONB             NOT NULL DEFAULT '[]',
  redirect_path       TEXT              NOT NULL DEFAULT '',
  success_redirect    TEXT              NOT NULL DEFAULT '',
  userinfo_field_map  JSONB             NOT NULL DEFAULT '{}',  -- {user_id, username, display_name, email}
  allowed_domains     JSONB             NOT NULL DEFAULT '[]',
  default_roles       JSONB             NOT NULL DEFAULT '[]',
  allow_jit           BOOLEAN           NOT NULL DEFAULT FALSE,
  created_at          TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_iam_auth_providers_tenant_provider UNIQUE (tenant_id, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_iam_auth_providers_tenant  ON iam_auth_providers (tenant_id);
CREATE INDEX IF NOT EXISTS idx_iam_auth_providers_enabled ON iam_auth_providers (tenant_id, enabled);

-- ─── Platform sessions ─────────────────────────────────────────────────────
-- Short-lived platform sessions issued after successful authentication.
-- token_hash stores a SHA-256 hash of the bearer token (never the raw token).

CREATE TABLE IF NOT EXISTS iam_sessions (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    TEXT        NOT NULL DEFAULT 'default',
  token_hash   TEXT        NOT NULL,   -- SHA-256(token) — raw token never stored
  user_id      TEXT        NOT NULL,   -- FK -> iam_users.user_id
  provider_id  TEXT        NOT NULL,   -- which auth_provider issued this session
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at   TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at   TIMESTAMPTZ,            -- NULL means active; set on logout
  CONSTRAINT uq_iam_sessions_token UNIQUE (tenant_id, token_hash)
);

CREATE INDEX IF NOT EXISTS idx_iam_sessions_user    ON iam_sessions (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_iam_sessions_expires ON iam_sessions (expires_at);

-- ─── IAM audit log ─────────────────────────────────────────────────────────
-- Dedicated audit table for IAM events (separate from workflow audit_logs).
-- Required by spec: auth_login_succeeded/failed, role_binding_changed etc.

CREATE TABLE IF NOT EXISTS iam_audit_events (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    TEXT        NOT NULL DEFAULT 'default',
  event_type   TEXT        NOT NULL,   -- auth_login_succeeded / role_binding_changed / ...
  actor_id     TEXT        NOT NULL DEFAULT '',   -- who triggered the event
  resource_type TEXT       NOT NULL DEFAULT '',   -- user / group / role / auth_provider
  resource_id  TEXT        NOT NULL DEFAULT '',
  metadata     JSONB       NOT NULL DEFAULT '{}',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  break_glass  BOOLEAN     NOT NULL DEFAULT FALSE  -- flagged ops-token events
);

CREATE INDEX IF NOT EXISTS idx_iam_audit_tenant    ON iam_audit_events (tenant_id);
CREATE INDEX IF NOT EXISTS idx_iam_audit_event_type ON iam_audit_events (tenant_id, event_type);
CREATE INDEX IF NOT EXISTS idx_iam_audit_actor     ON iam_audit_events (tenant_id, actor_id);
CREATE INDEX IF NOT EXISTS idx_iam_audit_created   ON iam_audit_events (created_at);

COMMENT ON TABLE iam_users             IS 'Platform user accounts (separate from People business profiles)';
COMMENT ON TABLE iam_user_identities   IS 'External identity links per user (OIDC sub, LDAP dn, Telegram id)';
COMMENT ON TABLE iam_groups            IS 'Platform groups for membership and role binding';
COMMENT ON TABLE iam_group_memberships IS 'User-to-group membership records';
COMMENT ON TABLE iam_roles             IS 'RBAC roles with permission arrays';
COMMENT ON TABLE iam_role_bindings     IS 'Role assignments to users and groups (with audit fields)';
COMMENT ON TABLE iam_auth_providers    IS 'Authentication provider configurations (OIDC/OAuth2/LDAP/local_token)';
COMMENT ON TABLE iam_sessions          IS 'Active platform sessions; token is stored as hash only';
COMMENT ON TABLE iam_audit_events      IS 'IAM-specific audit log (login, role changes, provider changes)';
