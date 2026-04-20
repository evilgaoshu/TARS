package postgres

import (
	"context"
	"database/sql"
)

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}

	// 0001_init.sql Baseline
	statements := []string{
		// Types
		`DO $$ BEGIN CREATE TYPE session_status AS ENUM ('open', 'analyzing', 'pending_approval', 'executing', 'verifying', 'resolved', 'failed'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN CREATE TYPE execution_status AS ENUM ('pending', 'approved', 'executing', 'completed', 'failed', 'timeout', 'rejected'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN CREATE TYPE risk_level AS ENUM ('info', 'warning', 'critical'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN CREATE TYPE outbox_status AS ENUM ('pending', 'processing', 'done', 'failed', 'blocked'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		// Tables
		`CREATE TABLE IF NOT EXISTS idempotency_keys (
			id UUID PRIMARY KEY,
			scope TEXT NOT NULL,
			idempotency_key TEXT NOT NULL,
			request_hash TEXT NOT NULL,
			resource_type TEXT,
			resource_id TEXT,
			status TEXT NOT NULL,
			response_payload JSONB,
			first_seen_at TIMESTAMPTZ NOT NULL,
			last_seen_at TIMESTAMPTZ NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT uniq_idempotency_scope_key UNIQUE (scope, idempotency_key)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_idempotency_expires_at ON idempotency_keys (expires_at)`,

		`CREATE TABLE IF NOT EXISTS alert_events (
			id UUID PRIMARY KEY,
			tenant_id TEXT NOT NULL DEFAULT 'default',
			external_alert_id TEXT,
			source TEXT NOT NULL,
			severity TEXT NOT NULL,
			labels JSONB NOT NULL,
			annotations JSONB NOT NULL,
			raw_payload JSONB NOT NULL,
			fingerprint TEXT NOT NULL,
			received_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_fingerprint ON alert_events (fingerprint)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_received_at ON alert_events (received_at)`,

		`CREATE TABLE IF NOT EXISTS alert_sessions (
			id UUID PRIMARY KEY,
			tenant_id TEXT NOT NULL DEFAULT 'default',
			alert_event_id UUID NOT NULL REFERENCES alert_events (id),
			status session_status NOT NULL,
			service_name TEXT,
			target_host TEXT,
			diagnosis_summary TEXT,
			tool_plan JSONB,
			attachments JSONB,
			verification_result JSONB,
			desense_map JSONB,
			version INTEGER NOT NULL DEFAULT 1,
			opened_at TIMESTAMPTZ NOT NULL,
			resolved_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_sessions_status ON alert_sessions (status)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_sessions_target_host ON alert_sessions (target_host)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_sessions_updated_at ON alert_sessions (updated_at)`,

		`CREATE TABLE IF NOT EXISTS session_events (
			id UUID PRIMARY KEY,
			session_id UUID NOT NULL REFERENCES alert_sessions (id),
			event_type TEXT NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS execution_requests (
			id UUID PRIMARY KEY,
			tenant_id TEXT NOT NULL DEFAULT 'default',
			session_id UUID NOT NULL REFERENCES alert_sessions (id),
			target_host TEXT NOT NULL,
			command TEXT NOT NULL,
			command_source TEXT NOT NULL,
			risk_level risk_level NOT NULL,
			requested_by TEXT NOT NULL,
			approved_by TEXT,
			approval_group TEXT,
			status execution_status NOT NULL,
			timeout_seconds INTEGER NOT NULL DEFAULT 300,
			output_ref TEXT,
			exit_code INTEGER NOT NULL DEFAULT 0,
			output_bytes BIGINT NOT NULL DEFAULT 0,
			output_truncated BOOLEAN NOT NULL DEFAULT FALSE,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL,
			approved_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_requests_session_id ON execution_requests (session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_requests_status ON execution_requests (status)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_requests_created_at ON execution_requests (created_at)`,

		`CREATE TABLE IF NOT EXISTS execution_approvals (
			id UUID PRIMARY KEY,
			execution_request_id UUID NOT NULL REFERENCES execution_requests (id),
			action TEXT NOT NULL,
			actor_id TEXT NOT NULL,
			actor_role TEXT,
			original_command TEXT,
			final_command TEXT,
			comment TEXT,
			created_at TIMESTAMPTZ NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS execution_output_chunks (
			id BIGSERIAL PRIMARY KEY,
			execution_request_id UUID NOT NULL REFERENCES execution_requests (id),
			seq INTEGER NOT NULL,
			stream_type TEXT NOT NULL,
			content TEXT NOT NULL,
			byte_size INTEGER NOT NULL,
			retention_until TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT uniq_execution_output_seq UNIQUE (execution_request_id, seq)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_output_retention ON execution_output_chunks (retention_until)`,

		`CREATE TABLE IF NOT EXISTS documents (
			id UUID PRIMARY KEY,
			tenant_id TEXT NOT NULL DEFAULT 'default',
			source_type TEXT NOT NULL,
			source_ref TEXT NOT NULL,
			title TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT uniq_documents_source UNIQUE (tenant_id, source_type, source_ref)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_documents_status ON documents (status, updated_at)`,

		`CREATE TABLE IF NOT EXISTS document_chunks (
			id UUID PRIMARY KEY,
			document_id UUID NOT NULL REFERENCES documents (id),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			chunk_index INTEGER NOT NULL,
			content TEXT NOT NULL,
			token_count INTEGER,
			citation JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT uniq_document_chunk_index UNIQUE (document_id, chunk_index)
		)`,

		`CREATE TABLE IF NOT EXISTS knowledge_records (
			id UUID PRIMARY KEY,
			tenant_id TEXT NOT NULL DEFAULT 'default',
			session_id UUID NOT NULL REFERENCES alert_sessions (id),
			document_id UUID NOT NULL REFERENCES documents (id),
			summary TEXT NOT NULL,
			content JSONB NOT NULL,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT uniq_knowledge_record_session UNIQUE (tenant_id, session_id)
		)`,

		`CREATE TABLE IF NOT EXISTS audit_logs (
			id BIGSERIAL PRIMARY KEY,
			tenant_id TEXT NOT NULL DEFAULT 'default',
			trace_id TEXT,
			actor_id TEXT,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			action TEXT NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS outbox_events (
			id UUID PRIMARY KEY,
			topic TEXT NOT NULL,
			aggregate_id TEXT NOT NULL,
			payload JSONB NOT NULL,
			status outbox_status NOT NULL DEFAULT 'pending',
			available_at TIMESTAMPTZ NOT NULL,
			retry_count INTEGER NOT NULL DEFAULT 0,
			last_error TEXT,
			blocked_reason TEXT,
			created_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_poll ON outbox_events (status, available_at)`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_failed_blocked ON outbox_events (status, created_at)`,

		// 0002_execution_connector_metadata.sql
		`ALTER TABLE execution_requests ADD COLUMN IF NOT EXISTS connector_id TEXT`,
		`ALTER TABLE execution_requests ADD COLUMN IF NOT EXISTS connector_type TEXT`,
		`ALTER TABLE execution_requests ADD COLUMN IF NOT EXISTS connector_vendor TEXT`,
		`ALTER TABLE execution_requests ADD COLUMN IF NOT EXISTS protocol TEXT NOT NULL DEFAULT 'ssh'`,
		`ALTER TABLE execution_requests ADD COLUMN IF NOT EXISTS execution_mode TEXT NOT NULL DEFAULT 'ssh'`,

		// 0003_capability_approval_fields.sql
		`ALTER TABLE execution_requests ADD COLUMN IF NOT EXISTS request_kind TEXT NOT NULL DEFAULT 'execution'`,
		`ALTER TABLE execution_requests ADD COLUMN IF NOT EXISTS step_id TEXT`,
		`ALTER TABLE execution_requests ADD COLUMN IF NOT EXISTS capability_id TEXT`,
		`ALTER TABLE execution_requests ADD COLUMN IF NOT EXISTS capability_params JSONB`,
		`CREATE INDEX IF NOT EXISTS idx_execution_requests_request_kind ON execution_requests (request_kind)`,

		// 0004_identity_access.sql
		`DO $$ BEGIN CREATE TYPE user_status AS ENUM ('active', 'disabled', 'invited'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN CREATE TYPE group_status AS ENUM ('active', 'disabled'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN CREATE TYPE auth_provider_type AS ENUM ('local_token', 'oidc', 'oauth2', 'ldap'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		`CREATE TABLE IF NOT EXISTS iam_users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			user_id TEXT NOT NULL,
			username TEXT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL DEFAULT '',
			status user_status NOT NULL DEFAULT 'active',
			source TEXT NOT NULL DEFAULT 'local',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_iam_users_tenant_user UNIQUE (tenant_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_users_tenant ON iam_users (tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_users_email ON iam_users (tenant_id, email)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_users_status ON iam_users (tenant_id, status)`,

		`CREATE TABLE IF NOT EXISTS iam_user_identities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			user_id TEXT NOT NULL,
			provider_type TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			external_subject TEXT NOT NULL,
			external_username TEXT NOT NULL DEFAULT '',
			external_email TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_iam_user_identity UNIQUE (tenant_id, provider_id, external_subject)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_user_identities_user ON iam_user_identities (tenant_id, user_id)`,

		`CREATE TABLE IF NOT EXISTS iam_groups (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			group_id TEXT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			status group_status NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_iam_groups_tenant_group UNIQUE (tenant_id, group_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_groups_tenant ON iam_groups (tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_groups_status ON iam_groups (tenant_id, status)`,

		`CREATE TABLE IF NOT EXISTS iam_group_memberships (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			group_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_iam_group_membership UNIQUE (tenant_id, group_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_group_memberships_group ON iam_group_memberships (tenant_id, group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_group_memberships_user ON iam_group_memberships (tenant_id, user_id)`,

		`CREATE TABLE IF NOT EXISTS iam_roles (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			role_id TEXT NOT NULL,
			display_name TEXT NOT NULL DEFAULT '',
			permissions JSONB NOT NULL DEFAULT '[]',
			is_system BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_iam_roles_tenant_role UNIQUE (tenant_id, role_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_roles_tenant ON iam_roles (tenant_id)`,

		`CREATE TABLE IF NOT EXISTS iam_role_bindings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			role_id TEXT NOT NULL,
			subject_type TEXT NOT NULL,
			subject_id TEXT NOT NULL,
			bound_by TEXT NOT NULL DEFAULT '',
			reason TEXT NOT NULL DEFAULT '',
			bound_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_iam_role_binding UNIQUE (tenant_id, role_id, subject_type, subject_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_role_bindings_role ON iam_role_bindings (tenant_id, role_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_role_bindings_subject ON iam_role_bindings (tenant_id, subject_type, subject_id)`,

		`CREATE TABLE IF NOT EXISTS iam_auth_providers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			provider_id TEXT NOT NULL,
			provider_type auth_provider_type NOT NULL DEFAULT 'oidc',
			name TEXT NOT NULL DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			client_id TEXT NOT NULL DEFAULT '',
			client_secret_ref TEXT NOT NULL DEFAULT '',
			auth_url TEXT NOT NULL DEFAULT '',
			token_url TEXT NOT NULL DEFAULT '',
			userinfo_url TEXT NOT NULL DEFAULT '',
			scopes JSONB NOT NULL DEFAULT '[]',
			redirect_path TEXT NOT NULL DEFAULT '',
			success_redirect TEXT NOT NULL DEFAULT '',
			userinfo_field_map JSONB NOT NULL DEFAULT '{}',
			allowed_domains JSONB NOT NULL DEFAULT '[]',
			default_roles JSONB NOT NULL DEFAULT '[]',
			allow_jit BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_iam_auth_providers_tenant_provider UNIQUE (tenant_id, provider_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_auth_providers_tenant ON iam_auth_providers (tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_auth_providers_enabled ON iam_auth_providers (tenant_id, enabled)`,

		`CREATE TABLE IF NOT EXISTS iam_sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			token_hash TEXT NOT NULL,
			user_id TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL,
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			revoked_at TIMESTAMPTZ,
			CONSTRAINT uq_iam_sessions_token UNIQUE (tenant_id, token_hash)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_sessions_user ON iam_sessions (tenant_id, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_sessions_expires ON iam_sessions (expires_at)`,

		`CREATE TABLE IF NOT EXISTS iam_audit_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id TEXT NOT NULL DEFAULT 'default',
			event_type TEXT NOT NULL,
			actor_id TEXT NOT NULL DEFAULT '',
			resource_type TEXT NOT NULL DEFAULT '',
			resource_id TEXT NOT NULL DEFAULT '',
			metadata JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			break_glass BOOLEAN NOT NULL DEFAULT FALSE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_audit_tenant ON iam_audit_events (tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_audit_event_type ON iam_audit_events (tenant_id, event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_audit_actor ON iam_audit_events (tenant_id, actor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_iam_audit_created ON iam_audit_events (created_at)`,

		// Other existing tables from EnsureSchema
		`CREATE TABLE IF NOT EXISTS runtime_config_documents (
			document_key TEXT PRIMARY KEY,
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS encrypted_secrets (
			ref TEXT PRIMARY KEY,
			ciphertext BYTEA NOT NULL,
			nonce BYTEA NOT NULL,
			key_id TEXT NOT NULL,
			algorithm TEXT NOT NULL,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_encrypted_secrets_updated_at ON encrypted_secrets (updated_at)`,
		`CREATE TABLE IF NOT EXISTS ssh_credentials (
			credential_id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL DEFAULT '',
			owner_type TEXT NOT NULL DEFAULT '',
			owner_id TEXT NOT NULL DEFAULT '',
			connector_id TEXT NOT NULL DEFAULT '',
			username TEXT NOT NULL DEFAULT '',
			auth_type TEXT NOT NULL,
			secret_ref TEXT NOT NULL,
			passphrase_secret_ref TEXT NOT NULL DEFAULT '',
			host_scope TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			created_by TEXT NOT NULL DEFAULT '',
			updated_by TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_rotated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ssh_credentials_connector ON ssh_credentials (connector_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ssh_credentials_status ON ssh_credentials (status)`,
		`CREATE TABLE IF NOT EXISTS setup_state (
			id BOOLEAN PRIMARY KEY DEFAULT TRUE,
			initialized BOOLEAN NOT NULL DEFAULT FALSE,
			current_step TEXT NOT NULL DEFAULT 'admin',
			admin_user_id TEXT NOT NULL DEFAULT '',
			auth_provider_id TEXT NOT NULL DEFAULT '',
			primary_provider_id TEXT NOT NULL DEFAULT '',
			primary_model TEXT NOT NULL DEFAULT '',
			default_channel_id TEXT NOT NULL DEFAULT '',
			provider_checked BOOLEAN NOT NULL DEFAULT FALSE,
			provider_check_ok BOOLEAN NOT NULL DEFAULT FALSE,
			provider_check_note TEXT NOT NULL DEFAULT '',
			login_hint JSONB NOT NULL DEFAULT '{}'::jsonb,
			completed_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (id = TRUE)
		)`,
		`ALTER TABLE setup_state ADD COLUMN IF NOT EXISTS provider_checked BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE setup_state ADD COLUMN IF NOT EXISTS provider_check_ok BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE setup_state ADD COLUMN IF NOT EXISTS provider_check_note TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE setup_state ADD COLUMN IF NOT EXISTS login_hint JSONB NOT NULL DEFAULT '{}'::jsonb`,

		`CREATE TABLE IF NOT EXISTS inbox_messages (
			id          TEXT PRIMARY KEY,
			tenant_id   TEXT NOT NULL DEFAULT 'default',
			subject     TEXT NOT NULL DEFAULT '',
			body        TEXT NOT NULL DEFAULT '',
			channel     TEXT NOT NULL DEFAULT 'in_app_inbox',
			ref_type    TEXT NOT NULL DEFAULT '',
			ref_id      TEXT NOT NULL DEFAULT '',
			source      TEXT NOT NULL DEFAULT 'system',
			actions     JSONB NOT NULL DEFAULT '[]'::jsonb,
			is_read     BOOLEAN NOT NULL DEFAULT FALSE,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			read_at     TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS inbox_messages_tenant_read_idx ON inbox_messages (tenant_id, is_read, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS triggers (
			id           TEXT PRIMARY KEY,
			tenant_id    TEXT NOT NULL DEFAULT 'default',
			display_name TEXT NOT NULL DEFAULT '',
			description  TEXT NOT NULL DEFAULT '',
			enabled      BOOLEAN NOT NULL DEFAULT TRUE,
			event_type   TEXT NOT NULL DEFAULT '',
			channel_id   TEXT NOT NULL DEFAULT 'in_app_inbox',
			automation_job_id TEXT NOT NULL DEFAULT '',
			governance   TEXT NOT NULL DEFAULT '',
			filter_expr  TEXT NOT NULL DEFAULT '',
			target_audience TEXT NOT NULL DEFAULT '',
			template_id  TEXT NOT NULL DEFAULT '',
			cooldown_sec INT NOT NULL DEFAULT 0,
			last_fired_at TIMESTAMPTZ,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS triggers_tenant_enabled_idx ON triggers (tenant_id, enabled, event_type)`,
		`ALTER TABLE triggers ADD COLUMN IF NOT EXISTS channel_id TEXT NOT NULL DEFAULT 'in_app_inbox'`,
		`DO $$ BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name='triggers' AND column_name='channel'
			) THEN
				UPDATE triggers SET channel_id = channel WHERE (channel_id = '' OR channel_id = 'in_app_inbox') AND channel != '' AND channel != 'in_app_inbox';
				ALTER TABLE triggers DROP COLUMN channel;
			END IF;
		END $$`,
		`ALTER TABLE triggers ADD COLUMN IF NOT EXISTS automation_job_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE triggers ADD COLUMN IF NOT EXISTS governance TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE triggers ADD COLUMN IF NOT EXISTS filter_expr TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE triggers ADD COLUMN IF NOT EXISTS target_audience TEXT NOT NULL DEFAULT ''`,

		`CREATE TABLE IF NOT EXISTS notification_templates (
			id         TEXT PRIMARY KEY,
			kind       TEXT NOT NULL DEFAULT '',
			locale     TEXT NOT NULL DEFAULT '',
			name       TEXT NOT NULL DEFAULT '',
			enabled    BOOLEAN NOT NULL DEFAULT TRUE,
			subject    TEXT NOT NULL DEFAULT '',
			body       TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`DO $$ BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = 'msg_templates'
			) AND NOT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = current_schema() AND table_name = 'notification_templates'
			) THEN
				ALTER TABLE msg_templates RENAME TO notification_templates;
			END IF;
		END $$`,
		`DO $$ BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = current_schema() AND table_name = 'notification_templates' AND column_name = 'type'
			) AND NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = current_schema() AND table_name = 'notification_templates' AND column_name = 'kind'
			) THEN
				ALTER TABLE notification_templates RENAME COLUMN type TO kind;
			END IF;
		END $$`,
		`ALTER TABLE notification_templates ADD COLUMN IF NOT EXISTS subject TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE notification_templates ADD COLUMN IF NOT EXISTS body TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS notification_templates_kind_locale_idx ON notification_templates (kind, locale)`,

		// 0005_agent_roles
		`CREATE TABLE IF NOT EXISTS agent_roles (
			id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id        TEXT NOT NULL DEFAULT 'default',
			role_id          TEXT NOT NULL,
			display_name     TEXT NOT NULL DEFAULT '',
			description      TEXT NOT NULL DEFAULT '',
			status           TEXT NOT NULL DEFAULT 'active',
			is_builtin       BOOLEAN NOT NULL DEFAULT FALSE,
			profile          JSONB NOT NULL DEFAULT '{}',
			capability_binding JSONB NOT NULL DEFAULT '{}',
			policy_binding   JSONB NOT NULL DEFAULT '{}',
			model_binding    JSONB NOT NULL DEFAULT '{}'::jsonb,
			org_id           TEXT NOT NULL DEFAULT '',
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_agent_roles_tenant_role UNIQUE (tenant_id, role_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_roles_tenant ON agent_roles (tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_roles_status ON agent_roles (tenant_id, status)`,
		`DO $$ BEGIN
			IF EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name='agent_roles' AND column_name='provider_preference'
			) AND NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name='agent_roles' AND column_name='model_binding'
			) THEN
				ALTER TABLE agent_roles RENAME COLUMN provider_preference TO model_binding;
			END IF;
		END $$`,
		`DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name='agent_roles' AND column_name='model_binding'
			) THEN
				ALTER TABLE agent_roles ADD COLUMN model_binding JSONB NOT NULL DEFAULT '{}'::jsonb;
			END IF;
		END $$`,
		`DO $$
		DECLARE unresolved_count INTEGER;
		BEGIN
			SELECT COUNT(*) INTO unresolved_count
			FROM agent_roles ar
			LEFT JOIN runtime_config_documents rcd ON rcd.document_key = 'providers'
			WHERE jsonb_typeof(COALESCE(ar.model_binding, '{}'::jsonb)) = 'object'
			  AND NOT (ar.model_binding ? 'primary' OR ar.model_binding ? 'fallback' OR ar.model_binding ? 'inherit_platform_default')
			  AND (
				(
					COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_provider_id'), ''), '') = ''
					AND (
						COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_model'), ''), '') <> ''
						OR COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_model_role'), ''), '') <> ''
					)
				)
				OR (
					COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_provider_id'), ''), '') <> ''
					AND COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_model'), ''), '') = ''
					AND COALESCE(
						CASE
							WHEN LOWER(BTRIM(COALESCE(ar.model_binding->>'preferred_model_role', ''))) = 'assist'
								AND COALESCE(NULLIF(BTRIM(COALESCE(
									rcd.payload #>> '{Assist,ProviderID}',
									rcd.payload #>> '{assist,provider_id}',
									rcd.payload #>> '{assist,ProviderID}',
									rcd.payload #>> '{Assist,provider_id}'
								)), ''), '') = COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_provider_id'), ''), '')
							THEN NULLIF(BTRIM(COALESCE(
								rcd.payload #>> '{Assist,Model}',
								rcd.payload #>> '{assist,model}',
								rcd.payload #>> '{assist,Model}',
								rcd.payload #>> '{Assist,model}'
							)), '')
							WHEN COALESCE(NULLIF(BTRIM(COALESCE(
								rcd.payload #>> '{Primary,ProviderID}',
								rcd.payload #>> '{primary,provider_id}',
								rcd.payload #>> '{primary,ProviderID}',
								rcd.payload #>> '{Primary,provider_id}'
							)), ''), '') = COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_provider_id'), ''), '')
							THEN NULLIF(BTRIM(COALESCE(
								rcd.payload #>> '{Primary,Model}',
								rcd.payload #>> '{primary,model}',
								rcd.payload #>> '{primary,Model}',
								rcd.payload #>> '{Primary,model}'
							)), '')
							ELSE NULL
						END,
						''
					) = ''
				)
			  );
			IF unresolved_count > 0 THEN
				RAISE EXCEPTION 'agent_roles model_binding migration failed: % rows still depend on preferred_model_role without a resolvable explicit model', unresolved_count;
			END IF;
		END $$`,
		`UPDATE agent_roles ar
		SET model_binding = CASE
			WHEN jsonb_typeof(COALESCE(ar.model_binding, '{}'::jsonb)) <> 'object' THEN
				jsonb_build_object('inherit_platform_default', TRUE)
			WHEN ar.model_binding ? 'primary' OR ar.model_binding ? 'fallback' OR ar.model_binding ? 'inherit_platform_default' THEN
				jsonb_strip_nulls(
					ar.model_binding || CASE
						WHEN ar.model_binding ? 'primary' OR ar.model_binding ? 'fallback' THEN '{}'::jsonb
						ELSE jsonb_build_object('inherit_platform_default', TRUE)
					END
				)
			WHEN COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_provider_id'), ''), '') = ''
				AND COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_model'), ''), '') = ''
				AND COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_model_role'), ''), '') = '' THEN
				jsonb_build_object('inherit_platform_default', TRUE)
			ELSE
				jsonb_strip_nulls(
					jsonb_build_object(
						'primary', jsonb_build_object(
							'provider_id', NULLIF(BTRIM(ar.model_binding->>'preferred_provider_id'), ''),
							'model', COALESCE(
								NULLIF(BTRIM(ar.model_binding->>'preferred_model'), ''),
								CASE
									WHEN LOWER(BTRIM(COALESCE(ar.model_binding->>'preferred_model_role', ''))) = 'assist'
										AND COALESCE(NULLIF(BTRIM(COALESCE(
											rcd.payload #>> '{Assist,ProviderID}',
											rcd.payload #>> '{assist,provider_id}',
											rcd.payload #>> '{assist,ProviderID}',
											rcd.payload #>> '{Assist,provider_id}'
										)), ''), '') = COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_provider_id'), ''), '')
									THEN NULLIF(BTRIM(COALESCE(
										rcd.payload #>> '{Assist,Model}',
										rcd.payload #>> '{assist,model}',
										rcd.payload #>> '{assist,Model}',
										rcd.payload #>> '{Assist,model}'
									)), '')
									WHEN COALESCE(NULLIF(BTRIM(COALESCE(
										rcd.payload #>> '{Primary,ProviderID}',
										rcd.payload #>> '{primary,provider_id}',
										rcd.payload #>> '{primary,ProviderID}',
										rcd.payload #>> '{Primary,provider_id}'
									)), ''), '') = COALESCE(NULLIF(BTRIM(ar.model_binding->>'preferred_provider_id'), ''), '')
									THEN NULLIF(BTRIM(COALESCE(
										rcd.payload #>> '{Primary,Model}',
										rcd.payload #>> '{primary,model}',
										rcd.payload #>> '{primary,Model}',
										rcd.payload #>> '{Primary,model}'
									)), '')
									ELSE NULL
								END
							)
						),
						'inherit_platform_default', FALSE
					)
				)
		END
		FROM (
			SELECT (SELECT payload FROM runtime_config_documents WHERE document_key = 'providers' LIMIT 1) AS payload
		) rcd`,

		// Add agent_role_id column to existing tables
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='alert_sessions' AND column_name='agent_role_id') THEN ALTER TABLE alert_sessions ADD COLUMN agent_role_id TEXT NOT NULL DEFAULT ''; END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='execution_requests' AND column_name='agent_role_id') THEN ALTER TABLE execution_requests ADD COLUMN agent_role_id TEXT NOT NULL DEFAULT ''; END IF; END $$`,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	return tx.Commit()
}
