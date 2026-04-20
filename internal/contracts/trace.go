package contracts

import (
	"context"
	"time"
)

type SessionKnowledgeTrace struct {
	Available      bool
	DocumentID     string
	Title          string
	Summary        string
	ContentPreview string
	Conversation   []string
	Runtime        *RuntimeMetadata
	UpdatedAt      time.Time
}

type SessionKnowledgeReader interface {
	GetSessionKnowledge(ctx context.Context, sessionID string) (SessionKnowledgeTrace, error)
}

type KnowledgeListReader interface {
	ListKnowledgeRecords(ctx context.Context, filter ListKnowledgeFilter) ([]KnowledgeRecordDetail, error)
}

type KnowledgeBulkReader interface {
	ListKnowledgeRecordsByIDs(ctx context.Context, ids []string) ([]KnowledgeRecordDetail, error)
}
