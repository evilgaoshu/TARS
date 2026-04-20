BEGIN;

ALTER TABLE execution_requests
  ADD COLUMN IF NOT EXISTS connector_id TEXT,
  ADD COLUMN IF NOT EXISTS connector_type TEXT,
  ADD COLUMN IF NOT EXISTS connector_vendor TEXT,
  ADD COLUMN IF NOT EXISTS protocol TEXT NOT NULL DEFAULT 'ssh',
  ADD COLUMN IF NOT EXISTS execution_mode TEXT NOT NULL DEFAULT 'ssh';

UPDATE execution_requests
SET protocol = COALESCE(NULLIF(protocol, ''), 'ssh'),
    execution_mode = COALESCE(NULLIF(execution_mode, ''), 'ssh');

COMMIT;
