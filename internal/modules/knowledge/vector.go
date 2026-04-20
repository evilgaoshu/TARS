package knowledge

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
)

const embeddingDimensions = 64

type VectorStore interface {
	ReplaceDocument(ctx context.Context, tenantID string, documentID string, chunks []VectorChunk) error
	Search(ctx context.Context, tenantID string, queryEmbedding []float32, limit int) ([]VectorSearchHit, error)
}

type VectorChunk struct {
	ChunkID    string
	DocumentID string
	Embedding  []float32
}

type VectorSearchHit struct {
	ChunkID    string
	DocumentID string
	Score      float64
}

func buildEmbedding(text string) []float32 {
	terms := searchTerms(strings.ToLower(strings.TrimSpace(text)))
	if len(terms) == 0 {
		return nil
	}

	vector := make([]float32, embeddingDimensions)
	for _, term := range terms {
		if strings.TrimSpace(term) == "" {
			continue
		}
		hash := hashToken(term)
		primary := int(hash % embeddingDimensions)
		secondary := int((hash >> 8) % embeddingDimensions)
		sign := float32(1)
		if (hash>>16)&1 == 1 {
			sign = -1
		}
		vector[primary] += sign
		if secondary != primary {
			vector[secondary] += sign * 0.5
		}
	}

	return normalizeEmbedding(vector)
}

func normalizeEmbedding(input []float32) []float32 {
	if len(input) == 0 {
		return nil
	}

	var sum float64
	for _, value := range input {
		sum += float64(value * value)
	}
	if sum == 0 {
		return nil
	}

	norm := float32(math.Sqrt(sum))
	output := make([]float32, len(input))
	for i, value := range input {
		output[i] = value / norm
	}
	return output
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

func hashToken(token string) uint32 {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(token))
	return hasher.Sum32()
}
