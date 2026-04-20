ALTER TABLE execution_requests
  ADD COLUMN request_kind TEXT NOT NULL DEFAULT 'execution',
  ADD COLUMN step_id TEXT,
  ADD COLUMN capability_id TEXT,
  ADD COLUMN capability_params JSONB;

UPDATE execution_requests
SET request_kind = 'execution'
WHERE request_kind IS NULL OR request_kind = '';

CREATE INDEX idx_execution_requests_request_kind ON execution_requests (request_kind);
