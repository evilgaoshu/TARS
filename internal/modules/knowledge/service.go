package knowledge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
)

type Options struct {
	DB       *sql.DB
	Vector   VectorStore
	Workflow contracts.WorkflowService
	Metrics  *foundationmetrics.Registry
}

type Service struct {
	db       *sql.DB
	vector   VectorStore
	workflow contracts.WorkflowService
	metrics  *foundationmetrics.Registry
}

func NewService() *Service {
	return &Service{}
}

func NewServiceWithOptions(opts Options) *Service {
	return &Service{
		db:       opts.DB,
		vector:   opts.Vector,
		workflow: opts.Workflow,
		metrics:  opts.Metrics,
	}
}

func (s *Service) Search(ctx context.Context, query contracts.KnowledgeQuery) ([]contracts.KnowledgeHit, error) {
	if s.db == nil {
		if s.metrics != nil {
			s.metrics.IncKnowledgeSearch("stub")
		}
		return []contracts.KnowledgeHit{}, nil
	}

	tenantID := strings.TrimSpace(query.TenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	terms := searchTerms(query.Query)
	if len(terms) == 0 {
		if s.metrics != nil {
			s.metrics.IncKnowledgeSearch("empty")
		}
		return []contracts.KnowledgeHit{}, nil
	}

	vectorHits := make([]contracts.KnowledgeHit, 0)
	if s.vector != nil {
		ranked, err := s.vector.Search(ctx, tenantID, buildEmbedding(query.Query), 5)
		if err != nil {
			return nil, err
		}
		vectorHits, err = s.loadVectorHits(ctx, tenantID, ranked)
		if err != nil {
			return nil, err
		}
	}

	lexicalHits, err := s.searchLexical(ctx, tenantID, terms, 5)
	if err != nil {
		return nil, err
	}

	hits := mergeKnowledgeHits(vectorHits, lexicalHits, 5)
	if s.metrics != nil {
		result := "miss"
		if len(hits) > 0 {
			result = "hit"
		}
		s.metrics.IncKnowledgeSearch(result)
	}
	return hits, nil
}

func (s *Service) searchLexical(ctx context.Context, tenantID string, terms []string, limit int) ([]contracts.KnowledgeHit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.document_id, c.id, c.content, d.updated_at
		FROM document_chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE d.tenant_id = $1
		  AND d.status = 'active'
		ORDER BY d.updated_at DESC, c.chunk_index ASC
		LIMIT 200
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		hit       contracts.KnowledgeHit
		score     int
		updatedAt time.Time
	}

	candidates := make([]candidate, 0)
	for rows.Next() {
		var documentID string
		var chunkID string
		var content string
		var updatedAt time.Time
		if err := rows.Scan(&documentID, &chunkID, &content, &updatedAt); err != nil {
			return nil, err
		}
		score := scoreKnowledgeContent(content, terms)
		if score == 0 {
			continue
		}
		candidates = append(candidates, candidate{
			hit: contracts.KnowledgeHit{
				DocumentID: documentID,
				ChunkID:    chunkID,
				Snippet:    makeSnippet(content, 240),
			},
			score:     score,
			updatedAt: updatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].updatedAt.After(candidates[j].updatedAt)
	})

	hits := make([]contracts.KnowledgeHit, 0, min(len(candidates), limit))
	for _, item := range candidates {
		hits = append(hits, item.hit)
		if len(hits) == limit {
			break
		}
	}
	return hits, nil
}

func (s *Service) loadVectorHits(ctx context.Context, tenantID string, ranked []VectorSearchHit) ([]contracts.KnowledgeHit, error) {
	if len(ranked) == 0 {
		return nil, nil
	}

	chunkIDs := make([]string, 0, len(ranked))
	for _, item := range ranked {
		chunkIDs = append(chunkIDs, item.ChunkID)
	}

	placeholders := make([]string, 0, len(chunkIDs))
	args := make([]interface{}, 0, len(chunkIDs)+1)
	args = append(args, tenantID)
	for i, chunkID := range chunkIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+2))
		args = append(args, chunkID)
	}

	query := fmt.Sprintf(`
		SELECT c.id, c.document_id, c.content
		FROM document_chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE d.tenant_id = $1
		  AND d.status = 'active'
		  AND c.id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexed := make(map[string]contracts.KnowledgeHit, len(chunkIDs))
	for rows.Next() {
		var chunkID string
		var documentID string
		var content string
		if err := rows.Scan(&chunkID, &documentID, &content); err != nil {
			return nil, err
		}
		indexed[chunkID] = contracts.KnowledgeHit{
			DocumentID: documentID,
			ChunkID:    chunkID,
			Snippet:    makeSnippet(content, 240),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	hits := make([]contracts.KnowledgeHit, 0, len(ranked))
	for _, item := range ranked {
		hit, ok := indexed[item.ChunkID]
		if !ok {
			continue
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

func mergeKnowledgeHits(primary []contracts.KnowledgeHit, fallback []contracts.KnowledgeHit, limit int) []contracts.KnowledgeHit {
	if limit <= 0 {
		limit = 5
	}

	seen := make(map[string]struct{}, limit)
	merged := make([]contracts.KnowledgeHit, 0, limit)
	appendHits := func(items []contracts.KnowledgeHit) {
		for _, item := range items {
			if _, ok := seen[item.ChunkID]; ok {
				continue
			}
			seen[item.ChunkID] = struct{}{}
			merged = append(merged, item)
			if len(merged) == limit {
				return
			}
		}
	}

	appendHits(primary)
	if len(merged) < limit {
		appendHits(fallback)
	}
	return merged
}

func (s *Service) IngestResolvedSession(ctx context.Context, event contracts.SessionClosedEvent) (contracts.KnowledgeIngestResult, error) {
	if s.db == nil || s.workflow == nil {
		if s.metrics != nil {
			s.metrics.IncKnowledgeIngest("stub")
		}
		return contracts.KnowledgeIngestResult{
			DocumentID: "doc-" + event.SessionID,
			Chunks:     0,
		}, nil
	}

	sessionDetail, err := s.workflow.GetSession(ctx, event.SessionID)
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncKnowledgeIngest("error")
		}
		return contracts.KnowledgeIngestResult{}, err
	}

	tenantID := strings.TrimSpace(event.TenantID)
	if tenantID == "" {
		tenantID = "default"
	}

	executionOutputs := collectExecutionOutputs(ctx, s.workflow, sessionDetail)
	title, content, recordPayload, err := buildSessionKnowledgeArtifacts(sessionDetail, executionOutputs)
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncKnowledgeIngest("error")
		}
		return contracts.KnowledgeIngestResult{}, err
	}
	contentHash := sha256Hex(content)
	chunks := chunkText(content, 700)
	now := event.ResolvedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncKnowledgeIngest("error")
		}
		return contracts.KnowledgeIngestResult{}, err
	}
	defer tx.Rollback()

	documentID, contentChanged, err := s.upsertDocument(ctx, tx, tenantID, event.SessionID, title, contentHash, now)
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncKnowledgeIngest("error")
		}
		return contracts.KnowledgeIngestResult{}, err
	}
	indexedChunks := make([]indexedChunk, 0, len(chunks))
	if contentChanged {
		indexedChunks, err = s.replaceChunks(ctx, tx, tenantID, documentID, event.SessionID, chunks, now)
		if err != nil {
			if s.metrics != nil {
				s.metrics.IncKnowledgeIngest("error")
			}
			return contracts.KnowledgeIngestResult{}, err
		}
	} else if s.vector != nil {
		indexedChunks, err = s.loadExistingChunks(ctx, tx, documentID)
		if err != nil {
			if s.metrics != nil {
				s.metrics.IncKnowledgeIngest("error")
			}
			return contracts.KnowledgeIngestResult{}, err
		}
	}
	if err := s.upsertKnowledgeRecord(ctx, tx, tenantID, event.SessionID, documentID, sessionDetail.DiagnosisSummary, recordPayload, now); err != nil {
		if s.metrics != nil {
			s.metrics.IncKnowledgeIngest("error")
		}
		return contracts.KnowledgeIngestResult{}, err
	}

	if err := tx.Commit(); err != nil {
		if s.metrics != nil {
			s.metrics.IncKnowledgeIngest("error")
		}
		return contracts.KnowledgeIngestResult{}, err
	}
	if contentChanged {
		if err := s.replaceChunkVectors(ctx, tenantID, documentID, indexedChunks); err != nil {
			if s.metrics != nil {
				s.metrics.IncKnowledgeIngest("error")
			}
			return contracts.KnowledgeIngestResult{}, err
		}
	}
	if s.metrics != nil {
		s.metrics.IncKnowledgeIngest("success")
	}
	return contracts.KnowledgeIngestResult{
		DocumentID: documentID,
		Chunks:     len(chunks),
	}, nil
}

func (s *Service) upsertDocument(ctx context.Context, tx *sql.Tx, tenantID string, sessionID string, title string, contentHash string, now time.Time) (string, bool, error) {
	var currentDocumentID string
	var currentContentHash string
	var currentTitle string
	var currentStatus string
	err := tx.QueryRowContext(ctx, `
		SELECT id, content_hash, title, status
		FROM documents
		WHERE tenant_id = $1
		  AND source_type = 'session'
		  AND source_ref = $2
	`, tenantID, sessionID).Scan(&currentDocumentID, &currentContentHash, &currentTitle, &currentStatus)
	switch {
	case err == nil:
		contentChanged := currentContentHash != contentHash
		if contentChanged || currentTitle != title || currentStatus != "active" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE documents
				SET title = $2,
				    content_hash = $3,
				    status = 'active',
				    updated_at = $4
				WHERE id = $1
			`, currentDocumentID, title, contentHash, now); err != nil {
				return "", false, err
			}
		}
		return currentDocumentID, contentChanged, nil
	case err != nil && err != sql.ErrNoRows:
		return "", false, err
	}

	documentID := randomUUID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO documents (
			id, tenant_id, source_type, source_ref, title, content_hash, status, created_at, updated_at
		) VALUES ($1, $2, 'session', $3, $4, $5, 'active', $6, $6)
	`, documentID, tenantID, sessionID, title, contentHash, now); err != nil {
		return "", false, err
	}
	return documentID, true, nil
}

func deterministicChunkID(documentID string, chunkIndex int) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", documentID, chunkIndex)))
	raw := hash[:16]
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

func (s *Service) replaceChunks(ctx context.Context, tx *sql.Tx, tenantID string, documentID string, sessionID string, chunks []string, now time.Time) ([]indexedChunk, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM document_chunks WHERE document_id = $1`, documentID); err != nil {
		return nil, err
	}

	indexed := make([]indexedChunk, 0, len(chunks))
	for i, chunk := range chunks {
		citationJSON, err := json.Marshal(map[string]string{
			"source_type": "session",
			"source_ref":  sessionID,
		})
		if err != nil {
			return nil, err
		}
		chunkID := deterministicChunkID(documentID, i)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO document_chunks (
				id, document_id, tenant_id, chunk_index, content, token_count, citation, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, chunkID, documentID, tenantID, i, chunk, estimateTokenCount(chunk), citationJSON, now); err != nil {
			return nil, err
		}
		indexed = append(indexed, indexedChunk{
			ChunkID: chunkID,
			Content: chunk,
		})
	}
	return indexed, nil
}

func (s *Service) loadExistingChunks(ctx context.Context, tx *sql.Tx, documentID string) ([]indexedChunk, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, content
		FROM document_chunks
		WHERE document_id = $1
		ORDER BY chunk_index ASC
	`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]indexedChunk, 0)
	for rows.Next() {
		var item indexedChunk
		if err := rows.Scan(&item.ChunkID, &item.Content); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) replaceChunkVectors(ctx context.Context, tenantID string, documentID string, chunks []indexedChunk) error {
	if s.vector == nil {
		return nil
	}

	items := make([]VectorChunk, 0, len(chunks))
	for _, chunk := range chunks {
		items = append(items, VectorChunk{
			ChunkID:    chunk.ChunkID,
			DocumentID: documentID,
			Embedding:  buildEmbedding(chunk.Content),
		})
	}
	return s.vector.ReplaceDocument(ctx, tenantID, documentID, items)
}

func (s *Service) ReindexDocuments(ctx context.Context, _ string) error {
	if s.db == nil || s.workflow == nil {
		return nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT source_ref
		FROM documents
		WHERE source_type = 'session'
		UNION
		SELECT session_id::text
		FROM knowledge_records
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	sessionIDs := make([]string, 0)
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return err
		}
		sessionIDs = append(sessionIDs, sessionID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, sessionID := range sessionIDs {
		if _, err := s.IngestResolvedSession(ctx, contracts.SessionClosedEvent{
			SessionID:  sessionID,
			TenantID:   "default",
			ResolvedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) GetSessionKnowledge(ctx context.Context, sessionID string) (contracts.SessionKnowledgeTrace, error) {
	if s.db == nil || strings.TrimSpace(sessionID) == "" {
		return contracts.SessionKnowledgeTrace{}, nil
	}

	var trace contracts.SessionKnowledgeTrace
	var payload []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT d.id, d.title, COALESCE(kr.summary, ''), kr.content, d.updated_at
		FROM knowledge_records kr
		JOIN documents d ON d.id = kr.document_id
		WHERE kr.tenant_id = 'default'
		  AND kr.session_id = $1
		  AND kr.status = 'active'
		LIMIT 1
	`, sessionID).Scan(&trace.DocumentID, &trace.Title, &trace.Summary, &payload, &trace.UpdatedAt)
	switch {
	case err == sql.ErrNoRows:
		return contracts.SessionKnowledgeTrace{}, nil
	case err != nil:
		return contracts.SessionKnowledgeTrace{}, err
	}

	trace.Available = true
	trace.Conversation = extractKnowledgeConversation(payload)
	trace.Runtime = latestKnowledgeRuntime(trace.Conversation)

	rows, err := s.db.QueryContext(ctx, `
		SELECT content
		FROM document_chunks
		WHERE document_id = $1
		ORDER BY chunk_index ASC
		LIMIT 3
	`, trace.DocumentID)
	if err != nil {
		return contracts.SessionKnowledgeTrace{}, err
	}
	defer rows.Close()

	parts := make([]string, 0, 3)
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return contracts.SessionKnowledgeTrace{}, err
		}
		parts = append(parts, strings.TrimSpace(content))
	}
	if err := rows.Err(); err != nil {
		return contracts.SessionKnowledgeTrace{}, err
	}
	trace.ContentPreview = makeSnippet(strings.Join(parts, "\n\n"), 1200)
	return trace, nil
}

func (s *Service) ListKnowledgeRecords(ctx context.Context, filter contracts.ListKnowledgeFilter) ([]contracts.KnowledgeRecordDetail, error) {
	if s.db == nil {
		return nil, nil
	}

	sortBy := strings.TrimSpace(filter.SortBy)
	switch sortBy {
	case "title", "session_id", "updated_at":
	default:
		sortBy = "updated_at"
	}
	sortOrder := strings.ToLower(strings.TrimSpace(filter.SortOrder))
	if sortOrder != "asc" {
		sortOrder = "desc"
	}

	args := []any{"default"}
	conditions := []string{"kr.tenant_id = $1", "kr.status = 'active'"}
	if value := strings.TrimSpace(filter.Query); value != "" {
		args = append(args, "%"+value+"%")
		conditions = append(conditions, "(kr.session_id ILIKE $2 OR d.id::text ILIKE $2 OR d.title ILIKE $2 OR COALESCE(kr.summary, '') ILIKE $2)")
	}

	query := `
		SELECT d.id, kr.session_id, d.title, COALESCE(kr.summary, ''), d.updated_at
		FROM knowledge_records kr
		JOIN documents d ON d.id = kr.document_id
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY ` + sortBy + ` ` + sortOrder + `
	`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]contracts.KnowledgeRecordDetail, 0, 64)
	for rows.Next() {
		var item contracts.KnowledgeRecordDetail
		if err := rows.Scan(&item.DocumentID, &item.SessionID, &item.Title, &item.Summary, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) ListKnowledgeRecordsByIDs(ctx context.Context, ids []string) ([]contracts.KnowledgeRecordDetail, error) {
	if s.db == nil || len(ids) == 0 {
		return nil, nil
	}
	trimmedIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			trimmedIDs = append(trimmedIDs, id)
		}
	}
	if len(trimmedIDs) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(trimmedIDs)+1)
	args = append(args, "default")
	placeholders := make([]string, 0, len(trimmedIDs))
	for i, id := range trimmedIDs {
		args = append(args, id)
		placeholders = append(placeholders, "$"+strconv.Itoa(i+2))
	}
	query := `
		SELECT d.id, kr.session_id, d.title, COALESCE(kr.summary, ''), d.updated_at
		FROM knowledge_records kr
		JOIN documents d ON d.id = kr.document_id
		WHERE kr.tenant_id = $1
		  AND kr.status = 'active'
		  AND d.id::text IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY d.updated_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]contracts.KnowledgeRecordDetail, 0, len(trimmedIDs))
	for rows.Next() {
		var item contracts.KnowledgeRecordDetail
		if err := rows.Scan(&item.DocumentID, &item.SessionID, &item.Title, &item.Summary, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type indexedChunk struct {
	ChunkID string
	Content string
}

func (s *Service) upsertKnowledgeRecord(ctx context.Context, tx *sql.Tx, tenantID string, sessionID string, documentID string, summary string, payload []byte, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO knowledge_records (
			id, tenant_id, session_id, document_id, summary, content, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'active', $7)
		ON CONFLICT (tenant_id, session_id)
		DO UPDATE SET
			document_id = EXCLUDED.document_id,
			summary = EXCLUDED.summary,
			content = EXCLUDED.content,
			status = 'active'
	`, randomUUID(), tenantID, sessionID, documentID, nullableSummary(summary), payload, now)
	return err
}

func buildSessionKnowledgeArtifacts(sessionDetail contracts.SessionDetail, executionOutputs map[string][]contracts.ExecutionOutputChunk) (string, string, []byte, error) {
	conversation := buildSessionConversation(sessionDetail, executionOutputs)
	record := map[string]interface{}{
		"session_id":        sessionDetail.SessionID,
		"status":            sessionDetail.Status,
		"diagnosis_summary": sessionDetail.DiagnosisSummary,
		"alert":             sessionDetail.Alert,
		"executions":        sessionDetail.Executions,
		"conversation":      conversation,
		"timeline":          sessionDetail.Timeline,
	}
	recordPayload, err := json.Marshal(record)
	if err != nil {
		return "", "", nil, err
	}

	alertName := alertLabel(sessionDetail.Alert, "alertname")
	host := alertLabel(sessionDetail.Alert, "instance")
	if host == "" {
		host = alertString(sessionDetail.Alert, "host")
	}
	title := fmt.Sprintf("session %s", sessionDetail.SessionID)
	if alertName != "" || host != "" {
		title = fmt.Sprintf("%s on %s", fallback(alertName, "alert"), fallback(host, "unknown-host"))
	}

	builder := &strings.Builder{}
	fmt.Fprintf(builder, "Session: %s\n", sessionDetail.SessionID)
	fmt.Fprintf(builder, "Title: %s\n", title)
	fmt.Fprintf(builder, "Status: %s\n", sessionDetail.Status)
	if sessionDetail.DiagnosisSummary != "" {
		fmt.Fprintf(builder, "Diagnosis: %s\n", sessionDetail.DiagnosisSummary)
	}
	if alertName != "" {
		fmt.Fprintf(builder, "AlertName: %s\n", alertName)
	}
	if host != "" {
		fmt.Fprintf(builder, "Host: %s\n", host)
	}
	if service := alertLabel(sessionDetail.Alert, "service"); service != "" {
		fmt.Fprintf(builder, "Service: %s\n", service)
	}
	if severity := alertString(sessionDetail.Alert, "severity"); severity != "" {
		fmt.Fprintf(builder, "Severity: %s\n", severity)
	}

	if labels := alertStringMap(sessionDetail.Alert, "labels"); len(labels) > 0 {
		builder.WriteString("\nLabels:\n")
		for _, key := range sortedKeys(labels) {
			value := labels[key]
			fmt.Fprintf(builder, "- %s=%s\n", key, value)
		}
	}
	if annotations := alertStringMap(sessionDetail.Alert, "annotations"); len(annotations) > 0 {
		builder.WriteString("\nAnnotations:\n")
		for _, key := range sortedKeys(annotations) {
			value := annotations[key]
			fmt.Fprintf(builder, "- %s=%s\n", key, value)
		}
	}
	if len(sessionDetail.Executions) > 0 {
		builder.WriteString("\nExecutions:\n")
		for _, execution := range sessionDetail.Executions {
			outputPreview := compactKnowledgeOutput(executionOutputs[execution.ExecutionID], 240)
			runtimeFields := knowledgeExecutionRuntimeFields(execution)
			fmt.Fprintf(
				builder,
				"- execution_id=%s status=%s host=%s command=%s %s exit_code=%d output_ref=%s output_preview=%s\n",
				execution.ExecutionID,
				execution.Status,
				execution.TargetHost,
				execution.Command,
				runtimeFields,
				execution.ExitCode,
				execution.OutputRef,
				outputPreview,
			)
		}
	}
	if len(conversation) > 0 {
		builder.WriteString("\nConversation:\n")
		for _, line := range conversation {
			fmt.Fprintf(builder, "- %s\n", line)
		}
	}
	if len(sessionDetail.Timeline) > 0 {
		builder.WriteString("\nTimeline:\n")
		for _, item := range sessionDetail.Timeline {
			fmt.Fprintf(builder, "- %s %s %s\n", item.CreatedAt.UTC().Format(time.RFC3339), item.Event, item.Message)
		}
	}

	return title, builder.String(), recordPayload, nil
}

func collectExecutionOutputs(ctx context.Context, workflow contracts.WorkflowService, sessionDetail contracts.SessionDetail) map[string][]contracts.ExecutionOutputChunk {
	if workflow == nil || len(sessionDetail.Executions) == 0 {
		return map[string][]contracts.ExecutionOutputChunk{}
	}

	outputs := make(map[string][]contracts.ExecutionOutputChunk, len(sessionDetail.Executions))
	for _, execution := range sessionDetail.Executions {
		chunks, err := workflow.GetExecutionOutput(ctx, execution.ExecutionID)
		if err != nil {
			continue
		}
		outputs[execution.ExecutionID] = chunks
	}
	return outputs
}

func buildSessionConversation(sessionDetail contracts.SessionDetail, executionOutputs map[string][]contracts.ExecutionOutputChunk) []string {
	lines := make([]string, 0, 8)
	source := alertString(sessionDetail.Alert, "source")
	userRequest := annotationString(sessionDetail.Alert, "user_request")
	requestedBy := annotationString(sessionDetail.Alert, "requested_by")

	if source == "telegram_chat" && strings.TrimSpace(userRequest) != "" {
		lines = append(lines, fmt.Sprintf("operator(%s): %s", fallback(requestedBy, "unknown"), userRequest))
	}
	if strings.TrimSpace(sessionDetail.DiagnosisSummary) != "" {
		lines = append(lines, "tars(diagnosis): "+sessionDetail.DiagnosisSummary)
	}
	for _, execution := range sessionDetail.Executions {
		lines = append(lines, fmt.Sprintf(
			"tars(command): host=%s command=%s %s",
			fallback(execution.TargetHost, "unknown-host"),
			fallback(execution.Command, "n/a"),
			knowledgeExecutionRuntimeFields(execution),
		))
		if preview := compactKnowledgeOutput(executionOutputs[execution.ExecutionID], 240); preview != "" {
			lines = append(lines, "tars(output): "+preview)
		}
	}
	if sessionDetail.Verification != nil && strings.TrimSpace(sessionDetail.Verification.Summary) != "" {
		lines = append(lines, "tars(verification): "+sessionDetail.Verification.Summary)
	}
	return lines
}

func knowledgeExecutionRuntimeFields(execution contracts.ExecutionDetail) string {
	return fmt.Sprintf(
		"connector_id=%s connector_type=%s connector_vendor=%s protocol=%s execution_mode=%s runtime=%s fallback_used=%t fallback_target=%s",
		fallback(execution.ConnectorID, "n/a"),
		fallback(execution.ConnectorType, "n/a"),
		fallback(execution.ConnectorVendor, "n/a"),
		fallback(execution.Protocol, "n/a"),
		fallback(execution.ExecutionMode, "n/a"),
		fallback(runtimeMetadataValue(execution.Runtime, "runtime"), "n/a"),
		runtimeMetadataBool(execution.Runtime),
		fallback(runtimeMetadataValue(execution.Runtime, "fallback_target"), "n/a"),
	)
}

func latestKnowledgeRuntime(conversation []string) *contracts.RuntimeMetadata {
	for i := len(conversation) - 1; i >= 0; i-- {
		line := strings.TrimSpace(conversation[i])
		if !strings.Contains(line, "runtime=") {
			continue
		}
		runtime := &contracts.RuntimeMetadata{}
		for _, part := range strings.Fields(line) {
			key, value, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			switch key {
			case "runtime":
				runtime.Runtime = value
			case "connector_id":
				runtime.ConnectorID = value
			case "connector_type":
				runtime.ConnectorType = value
			case "connector_vendor":
				runtime.ConnectorVendor = value
			case "protocol":
				runtime.Protocol = value
			case "execution_mode":
				runtime.ExecutionMode = value
			case "fallback_target":
				runtime.FallbackTarget = value
			case "fallback_used":
				runtime.FallbackUsed = value == "true"
			}
		}
		if runtime.Runtime != "" || runtime.ConnectorID != "" || runtime.Protocol != "" {
			return runtime
		}
	}
	return nil
}

func runtimeMetadataValue(runtime *contracts.RuntimeMetadata, field string) string {
	if runtime == nil {
		return ""
	}
	switch field {
	case "runtime":
		return runtime.Runtime
	case "fallback_target":
		return runtime.FallbackTarget
	default:
		return ""
	}
}

func runtimeMetadataBool(runtime *contracts.RuntimeMetadata) bool {
	return runtime != nil && runtime.FallbackUsed
}

func compactKnowledgeOutput(chunks []contracts.ExecutionOutputChunk, maxLen int) string {
	if len(chunks) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, chunk := range chunks {
		content := strings.TrimSpace(strings.ToValidUTF8(chunk.Content, ""))
		if content == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(content)
		if maxLen > 0 && utf8.RuneCountInString(builder.String()) >= maxLen {
			break
		}
	}

	return safeSnippet(builder.String(), maxLen)
}

func extractKnowledgeConversation(payload []byte) []string {
	if len(payload) == 0 {
		return nil
	}

	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil
	}

	raw, ok := body["conversation"].([]any)
	if !ok {
		return nil
	}

	lines := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		lines = append(lines, text)
	}
	return lines
}

func chunkText(content string, maxLen int) []string {
	trimmed := strings.TrimSpace(strings.ToValidUTF8(content, ""))
	if trimmed == "" {
		return []string{""}
	}
	if maxLen <= 0 {
		maxLen = 700
	}

	paragraphs := strings.Split(trimmed, "\n\n")
	chunks := make([]string, 0, len(paragraphs))
	current := &strings.Builder{}
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(strings.ToValidUTF8(paragraph, ""))
		if paragraph == "" {
			continue
		}
		if current.Len() > 0 && utf8.RuneCountInString(current.String())+2+utf8.RuneCountInString(paragraph) > maxLen {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		if utf8.RuneCountInString(paragraph) > maxLen {
			if current.Len() > 0 {
				chunks = append(chunks, current.String())
				current.Reset()
			}
			runes := []rune(paragraph)
			for len(runes) > maxLen {
				chunks = append(chunks, string(runes[:maxLen]))
				runes = runes[maxLen:]
			}
			paragraph = strings.TrimSpace(string(runes))
			if paragraph == "" {
				continue
			}
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(paragraph)
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	if len(chunks) == 0 {
		return []string{trimmed}
	}
	return chunks
}

func makeSnippet(content string, maxLen int) string {
	return safeSnippet(content, maxLen)
}

func safeSnippet(content string, maxLen int) string {
	content = strings.TrimSpace(strings.ToValidUTF8(content, ""))
	if maxLen <= 0 {
		return content
	}
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return strings.TrimSpace(string(runes[:maxLen])) + "..."
}

func searchTerms(query string) []string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(parts) == 0 {
		return nil
	}
	terms := make([]string, 0, len(parts))
	for _, item := range parts {
		if len(item) >= 2 {
			terms = append(terms, item)
		}
	}
	return terms
}

func scoreKnowledgeContent(content string, terms []string) int {
	if len(terms) == 0 {
		return 0
	}
	normalized := strings.ToLower(content)
	score := 0
	for _, term := range terms {
		if strings.Contains(normalized, term) {
			score++
		}
	}
	return score
}

func estimateTokenCount(content string) int {
	if content == "" {
		return 0
	}
	return len(strings.Fields(content))
}

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func randomUUID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

func alertString(alert map[string]interface{}, key string) string {
	if value, ok := alert[key].(string); ok {
		return value
	}
	return ""
}

func alertLabel(alert map[string]interface{}, key string) string {
	labels := alertStringMap(alert, "labels")
	return labels[key]
}

func annotationString(alert map[string]interface{}, key string) string {
	annotations := alertStringMap(alert, "annotations")
	return annotations[key]
}

func alertStringMap(alert map[string]interface{}, key string) map[string]string {
	raw, ok := alert[key]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case map[string]string:
		return typed
	case map[string]interface{}:
		out := make(map[string]string, len(typed))
		for mapKey, value := range typed {
			if typedValue, ok := value.(string); ok {
				out[mapKey] = typedValue
			}
		}
		return out
	default:
		return nil
	}
}

func nullableSummary(summary string) string {
	if strings.TrimSpace(summary) == "" {
		return "session closed"
	}
	return summary
}

func fallback(value string, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func sortedKeys(items map[string]string) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
