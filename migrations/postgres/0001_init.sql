BEGIN;

CREATE TYPE session_status AS ENUM (
  'open',
  'analyzing',
  'pending_approval',
  'executing',
  'verifying',
  'resolved',
  'failed'
);

CREATE TYPE execution_status AS ENUM (
  'pending',
  'approved',
  'executing',
  'completed',
  'failed',
  'timeout',
  'rejected'
);

CREATE TYPE risk_level AS ENUM ('info', 'warning', 'critical');

CREATE TYPE outbox_status AS ENUM (
  'pending',
  'processing',
  'done',
  'failed',
  'blocked'
);

CREATE TABLE idempotency_keys (
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
);

CREATE INDEX idx_idempotency_expires_at ON idempotency_keys (expires_at);

CREATE TABLE alert_events (
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
);

CREATE INDEX idx_alert_events_fingerprint ON alert_events (fingerprint);
CREATE INDEX idx_alert_events_received_at ON alert_events (received_at);

CREATE TABLE alert_sessions (
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
);

CREATE INDEX idx_alert_sessions_status ON alert_sessions (status);
CREATE INDEX idx_alert_sessions_target_host ON alert_sessions (target_host);
CREATE INDEX idx_alert_sessions_updated_at ON alert_sessions (updated_at);

CREATE TABLE session_events (
  id UUID PRIMARY KEY,
  session_id UUID NOT NULL REFERENCES alert_sessions (id),
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE execution_requests (
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
);

CREATE INDEX idx_execution_requests_session_id ON execution_requests (session_id);
CREATE INDEX idx_execution_requests_status ON execution_requests (status);
CREATE INDEX idx_execution_requests_created_at ON execution_requests (created_at);

CREATE TABLE execution_approvals (
  id UUID PRIMARY KEY,
  execution_request_id UUID NOT NULL REFERENCES execution_requests (id),
  action TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  actor_role TEXT,
  original_command TEXT,
  final_command TEXT,
  comment TEXT,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE execution_output_chunks (
  id BIGSERIAL PRIMARY KEY,
  execution_request_id UUID NOT NULL REFERENCES execution_requests (id),
  seq INTEGER NOT NULL,
  stream_type TEXT NOT NULL,
  content TEXT NOT NULL,
  byte_size INTEGER NOT NULL,
  retention_until TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT uniq_execution_output_seq UNIQUE (execution_request_id, seq)
);

CREATE INDEX idx_execution_output_retention ON execution_output_chunks (retention_until);

CREATE TABLE documents (
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
);

CREATE INDEX idx_documents_status ON documents (status, updated_at);

CREATE TABLE document_chunks (
  id UUID PRIMARY KEY,
  document_id UUID NOT NULL REFERENCES documents (id),
  tenant_id TEXT NOT NULL DEFAULT 'default',
  chunk_index INTEGER NOT NULL,
  content TEXT NOT NULL,
  token_count INTEGER,
  citation JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT uniq_document_chunk_index UNIQUE (document_id, chunk_index)
);

CREATE TABLE knowledge_records (
  id UUID PRIMARY KEY,
  tenant_id TEXT NOT NULL DEFAULT 'default',
  session_id UUID NOT NULL REFERENCES alert_sessions (id),
  document_id UUID NOT NULL REFERENCES documents (id),
  summary TEXT NOT NULL,
  content JSONB NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT uniq_knowledge_record_session UNIQUE (tenant_id, session_id)
);

CREATE TABLE audit_logs (
  id BIGSERIAL PRIMARY KEY,
  tenant_id TEXT NOT NULL DEFAULT 'default',
  trace_id TEXT,
  actor_id TEXT,
  resource_type TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  action TEXT NOT NULL,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE outbox_events (
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
);

CREATE INDEX idx_outbox_poll ON outbox_events (status, available_at);
CREATE INDEX idx_outbox_failed_blocked ON outbox_events (status, created_at);

COMMIT;
