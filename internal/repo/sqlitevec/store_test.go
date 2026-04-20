package sqlitevec

import (
	"context"
	"path/filepath"
	"testing"

	"tars/internal/modules/knowledge"
)

func TestStoreReplaceDocumentAndSearch(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "vectors.db"))
	if err != nil {
		t.Fatalf("open sqlitevec store: %v", err)
	}

	ctx := context.Background()
	if err := store.ReplaceDocument(ctx, "default", "doc-1", []knowledge.VectorChunk{
		{
			ChunkID:    "chunk-1",
			DocumentID: "doc-1",
			Embedding:  []float32{1, 0, 0},
		},
		{
			ChunkID:    "chunk-2",
			DocumentID: "doc-1",
			Embedding:  []float32{0, 1, 0},
		},
	}); err != nil {
		t.Fatalf("replace document: %v", err)
	}

	hits, err := store.Search(ctx, "default", []float32{0.9, 0.1, 0}, 2)
	if err != nil {
		t.Fatalf("search vectors: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("unexpected hit count: %d", len(hits))
	}
	if hits[0].ChunkID != "chunk-1" {
		t.Fatalf("expected chunk-1 first, got %+v", hits)
	}
}

func TestStoreReplaceDocumentRemovesOldVectors(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "vectors.db"))
	if err != nil {
		t.Fatalf("open sqlitevec store: %v", err)
	}

	ctx := context.Background()
	if err := store.ReplaceDocument(ctx, "default", "doc-1", []knowledge.VectorChunk{
		{
			ChunkID:    "chunk-old",
			DocumentID: "doc-1",
			Embedding:  []float32{1, 0},
		},
	}); err != nil {
		t.Fatalf("replace document: %v", err)
	}
	if err := store.ReplaceDocument(ctx, "default", "doc-1", []knowledge.VectorChunk{
		{
			ChunkID:    "chunk-new",
			DocumentID: "doc-1",
			Embedding:  []float32{0, 1},
		},
	}); err != nil {
		t.Fatalf("replace document: %v", err)
	}

	hits, err := store.Search(ctx, "default", []float32{1, 0}, 5)
	if err != nil {
		t.Fatalf("search vectors: %v", err)
	}
	for _, hit := range hits {
		if hit.ChunkID == "chunk-old" {
			t.Fatalf("expected old chunk to be removed, got %+v", hits)
		}
	}
}
