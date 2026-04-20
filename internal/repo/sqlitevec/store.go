package sqlitevec

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"

	"tars/internal/modules/knowledge"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return &Store{}, nil
	}

	if err := os.MkdirAll(filepath.Dir(trimmed), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", trimmed)
	if err != nil {
		return nil, err
	}
	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) ReplaceDocument(ctx context.Context, tenantID string, documentID string, chunks []knowledge.VectorChunk) error {
	if s.db == nil {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM chunk_vectors WHERE document_id = ?`, documentID); err != nil {
		return err
	}

	for _, chunk := range chunks {
		if len(chunk.Embedding) == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO chunk_vectors (chunk_id, tenant_id, document_id, embedding)
			VALUES (?, ?, ?, ?)
		`, chunk.ChunkID, tenantID, documentID, encodeEmbedding(chunk.Embedding)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) Search(ctx context.Context, tenantID string, queryEmbedding []float32, limit int) ([]knowledge.VectorSearchHit, error) {
	if s.db == nil || len(queryEmbedding) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT chunk_id, document_id, embedding
		FROM chunk_vectors
		WHERE tenant_id = ?
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]knowledge.VectorSearchHit, 0)
	for rows.Next() {
		var chunkID string
		var documentID string
		var blob []byte
		if err := rows.Scan(&chunkID, &documentID, &blob); err != nil {
			return nil, err
		}
		embedding, err := decodeEmbedding(blob)
		if err != nil {
			return nil, err
		}
		score := cosineSimilarity(queryEmbedding, embedding)
		if score <= 0 {
			continue
		}
		results = append(results, knowledge.VectorSearchHit{
			ChunkID:    chunkID,
			DocumentID: documentID,
			Score:      score,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ChunkID < results[j].ChunkID
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func encodeEmbedding(values []float32) []byte {
	output := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(output[i*4:], math.Float32bits(value))
	}
	return output
}

func decodeEmbedding(blob []byte) ([]float32, error) {
	if len(blob)%4 != 0 {
		return nil, fmt.Errorf("invalid embedding blob length: %d", len(blob))
	}
	values := make([]float32, len(blob)/4)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	return values, nil
}

func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS chunk_vectors (
			chunk_id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			document_id TEXT NOT NULL,
			embedding BLOB NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_chunk_vectors_tenant_document
			ON chunk_vectors (tenant_id, document_id);
	`)
	return err
}

func cosineSimilarity(left []float32, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var score float64
	for i := range left {
		score += float64(left[i] * right[i])
	}
	return score
}
