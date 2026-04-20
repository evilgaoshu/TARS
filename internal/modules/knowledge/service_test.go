package knowledge

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"tars/internal/contracts"
)

func TestBuildSessionKnowledgeArtifactsIncludesCoreFields(t *testing.T) {
	t.Parallel()

	session := contracts.SessionDetail{
		SessionID:        "ses-123",
		Status:           "resolved",
		DiagnosisSummary: "cpu pressure caused by noisy neighbor",
		Alert: map[string]interface{}{
			"source":   "telegram_chat",
			"host":     "host-1",
			"severity": "critical",
			"labels": map[string]string{
				"alertname": "HighCPU",
				"instance":  "host-1",
				"service":   "api",
			},
			"annotations": map[string]string{
				"summary":      "cpu too high",
				"user_request": "看系统负载",
				"requested_by": "alice",
			},
		},
		Executions: []contracts.ExecutionDetail{
			{
				ExecutionID:     "exe-1",
				Status:          "completed",
				Command:         "hostname && uptime",
				TargetHost:      "host-1",
				ConnectorID:     "jumpserver-main",
				ConnectorType:   "execution",
				ConnectorVendor: "jumpserver",
				Protocol:        "jumpserver_api",
				ExecutionMode:   "jumpserver_job",
				Runtime: &contracts.RuntimeMetadata{
					Runtime:         "connector",
					ConnectorID:     "jumpserver-main",
					ConnectorType:   "execution",
					ConnectorVendor: "jumpserver",
					Protocol:        "jumpserver_api",
					ExecutionMode:   "jumpserver_job",
					FallbackEnabled: true,
					FallbackUsed:    false,
					FallbackTarget:  "ssh",
				},
				ExitCode:  0,
				OutputRef: "/tmp/exe-1.log",
			},
		},
		Timeline: []contracts.TimelineEvent{
			{
				Event:     "execution_completed",
				Message:   "execution completed",
				CreatedAt: time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	title, content, payload, err := buildSessionKnowledgeArtifacts(session, map[string][]contracts.ExecutionOutputChunk{
		"exe-1": {
			{Seq: 0, Content: "08:19:40 up 13 days\n0.16 0.04 0.01\n"},
		},
	})
	if err != nil {
		t.Fatalf("build artifacts: %v", err)
	}

	if title != "HighCPU on host-1" {
		t.Fatalf("unexpected title: %q", title)
	}
	for _, expected := range []string{
		"Session: ses-123",
		"Diagnosis: cpu pressure caused by noisy neighbor",
		"AlertName: HighCPU",
		"Service: api",
		"execution_id=exe-1",
		"connector_id=jumpserver-main",
		"connector_type=execution",
		"connector_vendor=jumpserver",
		"protocol=jumpserver_api",
		"execution_mode=jumpserver_job",
		"output_preview=08:19:40 up 13 days",
		"operator(alice): 看系统负载",
		"tars(command): host=host-1 command=hostname && uptime connector_id=jumpserver-main connector_type=execution connector_vendor=jumpserver protocol=jumpserver_api execution_mode=jumpserver_job runtime=connector fallback_used=false fallback_target=ssh",
		"tars(output): 08:19:40 up 13 days",
		"execution_completed",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected content to contain %q, got:\n%s", expected, content)
		}
	}
	if !strings.Contains(string(payload), `"session_id":"ses-123"`) || !strings.Contains(string(payload), `"conversation"`) {
		t.Fatalf("unexpected payload: %s", string(payload))
	}
}

func TestChunkTextSplitsLongContent(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("a", 720) + "\n\n" + strings.Repeat("b", 720)
	chunks := chunkText(content, 700)
	if len(chunks) < 3 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if len(chunk) > 700 {
			t.Fatalf("chunk exceeded limit: %d", len(chunk))
		}
	}
}

func TestChunkTextAndSnippetPreserveUTF8(t *testing.T) {
	t.Parallel()

	content := strings.Repeat("看系统负载并检查磁盘使用情况。", 80)
	chunks := chunkText(content, 32)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if !utf8.ValidString(chunk) {
			t.Fatalf("chunk is not valid utf8: %q", chunk)
		}
		if len([]rune(chunk)) > 32 {
			t.Fatalf("chunk exceeded rune limit: %d", len([]rune(chunk)))
		}
	}

	snippet := makeSnippet(content, 17)
	if !utf8.ValidString(snippet) {
		t.Fatalf("snippet is not valid utf8: %q", snippet)
	}
	if !strings.HasSuffix(snippet, "...") {
		t.Fatalf("expected snippet to be truncated with ellipsis, got %q", snippet)
	}
}

func TestSearchTermsAndScore(t *testing.T) {
	t.Parallel()

	terms := searchTerms("HighCPU api host-1")
	if strings.Join(terms, ",") != "highcpu,api,host-1" {
		t.Fatalf("unexpected terms: %v", terms)
	}

	score := scoreKnowledgeContent("AlertName: HighCPU\nHost: host-1\nService: api", terms)
	if score != 3 {
		t.Fatalf("unexpected score: %d", score)
	}
}

func TestBuildEmbeddingAndCosineSimilarity(t *testing.T) {
	t.Parallel()

	left := buildEmbedding("HighCPU api host-1")
	right := buildEmbedding("api host-1 highcpu")
	other := buildEmbedding("DiskFull worker host-9")

	if len(left) != embeddingDimensions {
		t.Fatalf("unexpected embedding length: %d", len(left))
	}
	if score := cosineSimilarity(left, right); score < 0.99 {
		t.Fatalf("expected similar embeddings, got %f", score)
	}
	if score := cosineSimilarity(left, other); score >= 0.99 {
		t.Fatalf("expected different embeddings, got %f", score)
	}
}

func TestMergeKnowledgeHitsPrefersPrimaryAndDeduplicates(t *testing.T) {
	t.Parallel()

	merged := mergeKnowledgeHits(
		[]contracts.KnowledgeHit{
			{ChunkID: "chunk-1", DocumentID: "doc-1"},
			{ChunkID: "chunk-2", DocumentID: "doc-2"},
		},
		[]contracts.KnowledgeHit{
			{ChunkID: "chunk-2", DocumentID: "doc-2"},
			{ChunkID: "chunk-3", DocumentID: "doc-3"},
		},
		3,
	)
	if len(merged) != 3 {
		t.Fatalf("unexpected merged length: %d", len(merged))
	}
	if merged[0].ChunkID != "chunk-1" || merged[1].ChunkID != "chunk-2" || merged[2].ChunkID != "chunk-3" {
		t.Fatalf("unexpected merge order: %+v", merged)
	}
}

func TestBuildSessionKnowledgeArtifactsPayloadStable(t *testing.T) {
	t.Parallel()

	session := contracts.SessionDetail{
		SessionID:        "ses-stable",
		Status:           "resolved",
		DiagnosisSummary: "stable diagnosis",
		Alert: map[string]interface{}{
			"labels": map[string]string{
				"alertname": "StableAlert",
				"instance":  "host-1",
			},
		},
	}

	_, _, firstPayload, err := buildSessionKnowledgeArtifacts(session, nil)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	_, _, secondPayload, err := buildSessionKnowledgeArtifacts(session, nil)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if !bytes.Equal(firstPayload, secondPayload) {
		t.Fatalf("expected stable payloads, got %s vs %s", string(firstPayload), string(secondPayload))
	}
}

func TestDeterministicChunkIDStable(t *testing.T) {
	t.Parallel()

	first := deterministicChunkID("doc-1", 0)
	second := deterministicChunkID("doc-1", 0)
	other := deterministicChunkID("doc-1", 1)

	if first != second {
		t.Fatalf("expected stable chunk id, got %s vs %s", first, second)
	}
	if first == other {
		t.Fatalf("expected different chunk ids for different chunk index")
	}
}
