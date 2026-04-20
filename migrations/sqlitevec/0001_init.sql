CREATE TABLE chunk_vectors (
  chunk_id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  document_id TEXT NOT NULL,
  embedding BLOB NOT NULL
);
