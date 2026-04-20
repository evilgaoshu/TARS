package workflow

import (
	"context"
	"strings"
	"testing"
	"time"

	"tars/internal/contracts"
)

func TestListSessionsFiltersQueriesAndSorts(t *testing.T) {
	t.Parallel()

	service := NewService(Options{})
	service.sessions["ses-001"] = &sessionRecord{
		detail: contracts.SessionDetail{
			SessionID:        "ses-001",
			Status:           "resolved",
			DiagnosisSummary: "database saturation cleared",
			Alert: map[string]interface{}{
				"host":     "db-1",
				"severity": "critical",
				"labels": map[string]interface{}{
					"alertname": "DBCPUHigh",
					"instance":  "db-1",
					"service":   "payments",
				},
				"annotations": map[string]interface{}{
					"summary": "payments database cpu high",
				},
			},
			Timeline: []contracts.TimelineEvent{{Event: "verify_success", CreatedAt: time.Date(2026, time.April, 2, 11, 0, 0, 0, time.UTC)}},
		},
		host: "db-1",
	}
	service.sessions["ses-002"] = &sessionRecord{
		detail: contracts.SessionDetail{
			SessionID:        "ses-002",
			Status:           "open",
			DiagnosisSummary: "api latency stabilized",
			Alert: map[string]interface{}{
				"host":     "api-1",
				"severity": "warning",
				"labels": map[string]interface{}{
					"alertname": "APILatency",
					"instance":  "api-1",
					"service":   "payments",
				},
				"annotations": map[string]interface{}{
					"summary": "payments api latency spike",
				},
			},
			Timeline: []contracts.TimelineEvent{{Event: "diagnosis_completed", CreatedAt: time.Date(2026, time.April, 2, 10, 0, 0, 0, time.UTC)}},
		},
		host: "api-1",
	}
	service.sessions["ses-003"] = &sessionRecord{
		detail: contracts.SessionDetail{
			SessionID:        "ses-003",
			Status:           "failed",
			DiagnosisSummary: "worker queue backed up",
			Alert: map[string]interface{}{
				"host":     "worker-1",
				"severity": "critical",
				"labels": map[string]interface{}{
					"alertname": "QueueDepth",
					"instance":  "worker-1",
					"service":   "jobs",
				},
				"annotations": map[string]interface{}{
					"summary": "job queue delay rising",
				},
			},
			Timeline: []contracts.TimelineEvent{{Event: "execution_failed", CreatedAt: time.Date(2026, time.April, 2, 12, 0, 0, 0, time.UTC)}},
		},
		host: "worker-1",
	}
	service.sessionOrder = []string{"ses-001", "ses-002", "ses-003"}

	items, err := service.ListSessions(context.Background(), contracts.ListSessionsFilter{
		Query:     "payments",
		SortBy:    "session_id",
		SortOrder: "asc",
	})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two payments sessions, got %+v", items)
	}
	if items[0].SessionID != "ses-001" || items[1].SessionID != "ses-002" {
		t.Fatalf("expected session_id ascending sort, got %+v", items)
	}
	if items[0].GoldenSummary == nil || items[1].GoldenSummary == nil {
		t.Fatalf("expected golden summaries to be populated, got %+v", items)
	}

	hostItems, err := service.ListSessions(context.Background(), contracts.ListSessionsFilter{
		Status: "open",
		Host:   "api-1",
		Query:  "latency",
	})
	if err != nil {
		t.Fatalf("list sessions by host/status: %v", err)
	}
	if len(hostItems) != 1 || hostItems[0].SessionID != "ses-002" {
		t.Fatalf("expected filtered session ses-002, got %+v", hostItems)
	}
}

func TestListSessionsTriageSortsBeforePaginationConsumers(t *testing.T) {
	t.Parallel()

	service := NewService(Options{})
	service.sessions["ses-critical-old"] = &sessionRecord{
		detail: contracts.SessionDetail{
			SessionID: "ses-critical-old",
			Status:    "pending_approval",
			Alert: map[string]interface{}{
				"host":     "api-1",
				"severity": "critical",
				"labels": map[string]interface{}{
					"alertname": "CriticalApproval",
				},
			},
			Executions: []contracts.ExecutionDetail{{ExecutionID: "exe-critical", Status: "pending", RiskLevel: "critical"}},
			Timeline:   []contracts.TimelineEvent{{Event: "approval_requested", CreatedAt: time.Date(2026, time.April, 2, 8, 0, 0, 0, time.UTC)}},
		},
		host: "api-1",
	}
	service.sessions["ses-warning-new"] = &sessionRecord{
		detail: contracts.SessionDetail{
			SessionID: "ses-warning-new",
			Status:    "open",
			Alert: map[string]interface{}{
				"host":     "api-2",
				"severity": "warning",
				"labels": map[string]interface{}{
					"alertname": "WarningRecent",
				},
			},
			Timeline: []contracts.TimelineEvent{{Event: "diagnosis_started", CreatedAt: time.Date(2026, time.April, 2, 12, 0, 0, 0, time.UTC)}},
		},
		host: "api-2",
	}
	service.sessionOrder = []string{"ses-critical-old", "ses-warning-new"}

	items, err := service.ListSessions(context.Background(), contracts.ListSessionsFilter{
		SortBy:    "triage",
		SortOrder: "desc",
	})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two sessions, got %+v", items)
	}
	if items[0].SessionID != "ses-critical-old" {
		t.Fatalf("expected triage sort to rank older critical pending action first before API pagination, got %+v", items)
	}
}

func TestListExecutionsFiltersQueriesAndSorts(t *testing.T) {
	t.Parallel()

	service := NewService(Options{})
	service.executions["exe-001"] = contracts.ExecutionDetail{
		ExecutionID:   "exe-001",
		Status:        "completed",
		RiskLevel:     "low",
		Command:       "uptime",
		TargetHost:    "api-1",
		RequestedBy:   "tars",
		ApprovalGroup: "policy:auto",
		OutputRef:     "s3://outputs/exe-001",
		CreatedAt:     time.Date(2026, time.April, 2, 8, 0, 0, 0, time.UTC),
		CompletedAt:   time.Date(2026, time.April, 2, 8, 5, 0, 0, time.UTC),
	}
	service.executions["exe-002"] = contracts.ExecutionDetail{
		ExecutionID:   "exe-002",
		Status:        "pending",
		RiskLevel:     "high",
		Command:       "systemctl restart payments",
		TargetHost:    "db-1",
		RequestedBy:   "alice",
		ApprovalGroup: "team:payments",
		OutputRef:     "s3://outputs/exe-002",
		CreatedAt:     time.Date(2026, time.April, 2, 9, 0, 0, 0, time.UTC),
		CompletedAt:   time.Date(2026, time.April, 2, 9, 10, 0, 0, time.UTC),
	}
	service.executionSession["exe-001"] = "ses-001"
	service.executionSession["exe-002"] = "ses-002"

	items, err := service.ListExecutions(context.Background(), contracts.ListExecutionsFilter{
		Query:     "s3://outputs",
		SortBy:    "target_host",
		SortOrder: "asc",
	})
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected both executions to match output_ref query, got %+v", items)
	}
	if items[0].TargetHost != "api-1" || items[1].TargetHost != "db-1" {
		t.Fatalf("expected target_host sort, got %+v", items)
	}
	if items[0].SessionID != "ses-001" || items[1].SessionID != "ses-002" {
		t.Fatalf("expected session ids to be attached, got %+v", items)
	}
	if items[0].GoldenSummary == nil || items[1].GoldenSummary == nil {
		t.Fatalf("expected execution golden summaries, got %+v", items)
	}

	pending, err := service.ListExecutions(context.Background(), contracts.ListExecutionsFilter{
		Status: "pending",
		Query:  "team:payments",
	})
	if err != nil {
		t.Fatalf("list pending executions: %v", err)
	}
	if len(pending) != 1 || pending[0].ExecutionID != "exe-002" {
		t.Fatalf("expected pending payments approval, got %+v", pending)
	}
}

func TestListExecutionsTriageSortsBeforePaginationConsumers(t *testing.T) {
	t.Parallel()

	service := NewService(Options{})
	service.executions["exe-critical-old"] = contracts.ExecutionDetail{
		ExecutionID: "exe-critical-old",
		Status:      "pending",
		RiskLevel:   "critical",
		TargetHost:  "db-1",
		CreatedAt:   time.Date(2026, time.April, 2, 8, 0, 0, 0, time.UTC),
	}
	service.executions["exe-warning-new"] = contracts.ExecutionDetail{
		ExecutionID: "exe-warning-new",
		Status:      "pending",
		RiskLevel:   "warning",
		TargetHost:  "api-1",
		CreatedAt:   time.Date(2026, time.April, 2, 12, 0, 0, 0, time.UTC),
	}
	service.executions["exe-completed-newest"] = contracts.ExecutionDetail{
		ExecutionID: "exe-completed-newest",
		Status:      "completed",
		RiskLevel:   "critical",
		TargetHost:  "worker-1",
		CreatedAt:   time.Date(2026, time.April, 2, 13, 0, 0, 0, time.UTC),
		CompletedAt: time.Date(2026, time.April, 2, 14, 0, 0, 0, time.UTC),
	}
	service.executionSession["exe-critical-old"] = "ses-critical-old"
	service.executionSession["exe-warning-new"] = "ses-warning-new"
	service.executionSession["exe-completed-newest"] = "ses-completed-newest"

	items, err := service.ListExecutions(context.Background(), contracts.ListExecutionsFilter{
		SortBy:    "triage",
		SortOrder: "desc",
	})
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected three executions, got %+v", items)
	}
	if items[0].ExecutionID != "exe-critical-old" {
		t.Fatalf("expected triage sort to rank critical pending execution ahead of newer warning/completed rows before API pagination, got %+v", items)
	}
}

func TestSessionAndExecutionQueryHelpers(t *testing.T) {
	t.Parallel()

	session := contracts.SessionDetail{
		SessionID:        "ses-query",
		Status:           "open",
		DiagnosisSummary: "payments api latency spike",
		Alert: map[string]interface{}{
			"host":     "api-9",
			"severity": "critical",
			"labels": map[string]interface{}{
				"alertname": "APILatency",
				"instance":  "api-9",
				"service":   "payments",
			},
			"annotations": map[string]interface{}{
				"summary": "latency rose above threshold",
			},
		},
	}
	if !matchesSessionQuery(session, "payments") {
		t.Fatalf("expected service label to match session query")
	}
	if !matchesSessionQuery(session, "LATENCY ROSE") {
		t.Fatalf("expected annotation summary to match session query case-insensitively")
	}
	if matchesSessionQuery(session, "cache") {
		t.Fatalf("did not expect unrelated session query match")
	}

	execution := contracts.ExecutionDetail{
		ExecutionID:   "exe-query",
		Status:        "pending",
		RiskLevel:     "high",
		Command:       "systemctl restart payments",
		TargetHost:    "api-9",
		RequestedBy:   "alice",
		ApprovalGroup: "team:payments",
		OutputRef:     "s3://outputs/exe-query",
	}
	if !matchesExecutionQuery(execution, "restart payments") {
		t.Fatalf("expected command to match execution query")
	}
	if !matchesExecutionQuery(execution, "TEAM:PAYMENTS") {
		t.Fatalf("expected approval group to match execution query case-insensitively")
	}
	if matchesExecutionQuery(execution, "worker") {
		t.Fatalf("did not expect unrelated execution query match")
	}
}

func TestGetExecutionOutputBuildsChunksAndReturnsCopy(t *testing.T) {
	t.Parallel()

	service := NewService(Options{})
	sessionID := "ses-output"
	service.sessions[sessionID] = &sessionRecord{
		detail: contracts.SessionDetail{
			SessionID: sessionID,
			Status:    "executing",
			Alert: map[string]interface{}{
				"host":     "api-4",
				"severity": "warning",
				"labels":   map[string]interface{}{"service": "payments"},
			},
		},
		host: "api-4",
	}
	service.executions["exe-output"] = contracts.ExecutionDetail{ExecutionID: "exe-output", Status: "executing"}
	service.executionSession["exe-output"] = sessionID

	preview := strings.Repeat("界", 6000)
	_, err := service.HandleExecutionResult(context.Background(), contracts.ExecutionResult{
		ExecutionID:     "exe-output",
		SessionID:       sessionID,
		Status:          "completed",
		OutputPreview:   preview,
		OutputBytes:     int64(len([]byte(preview))),
		OutputTruncated: true,
	})
	if err != nil {
		t.Fatalf("handle execution result: %v", err)
	}

	chunks, err := service.GetExecutionOutput(context.Background(), "exe-output")
	if err != nil {
		t.Fatalf("get execution output: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected output to split into two chunks, got %d", len(chunks))
	}
	if chunks[0].Seq != 0 || chunks[1].Seq != 1 {
		t.Fatalf("expected chunk sequence numbers, got %+v", chunks)
	}

	var rebuilt strings.Builder
	for _, chunk := range chunks {
		rebuilt.WriteString(chunk.Content)
		if chunk.ByteSize != len([]byte(chunk.Content)) {
			t.Fatalf("expected byte size to match chunk content, got %+v", chunk)
		}
	}
	if rebuilt.String() != preview {
		t.Fatalf("expected output preview to be preserved across chunks")
	}

	chunks[0].Content = "mutated"
	fresh, err := service.GetExecutionOutput(context.Background(), "exe-output")
	if err != nil {
		t.Fatalf("get execution output again: %v", err)
	}
	if fresh[0].Content == "mutated" {
		t.Fatalf("expected GetExecutionOutput to return a copy, got %+v", fresh)
	}

	if _, err := service.GetExecutionOutput(context.Background(), "missing-execution"); err != contracts.ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing execution, got %v", err)
	}
}

func TestExecutionOutputChunkHelpers(t *testing.T) {
	t.Parallel()

	if chunks := buildExecutionOutputChunks("", time.Time{}); chunks != nil {
		t.Fatalf("expected empty preview to produce no chunks, got %+v", chunks)
	}

	chunks := buildExecutionOutputChunks("hello world", time.Time{})
	if len(chunks) != 1 || chunks[0].Content != "hello world" || chunks[0].CreatedAt.IsZero() {
		t.Fatalf("expected default timestamped chunk, got %+v", chunks)
	}

	chunk, truncated := truncateStringByBytes("界a", 1)
	if chunk != "" || !truncated {
		t.Fatalf("expected truncation to stop at rune boundary, got chunk=%q truncated=%v", chunk, truncated)
	}

	items := splitStringByBytes("abcdef", 2)
	if len(items) != 3 || items[0] != "ab" || items[2] != "ef" {
		t.Fatalf("expected splitStringByBytes to chunk by byte limit, got %+v", items)
	}
}

func TestSortHelpersAndExecutionResultBranches(t *testing.T) {
	t.Parallel()

	t.Run("session and execution sort helpers", func(t *testing.T) {
		t.Parallel()

		sessions := []contracts.SessionDetail{
			{SessionID: "ses-2", Status: "open", Timeline: []contracts.TimelineEvent{{CreatedAt: time.Date(2026, time.April, 2, 9, 0, 0, 0, time.UTC)}}},
			{SessionID: "ses-1", Status: "failed", Timeline: []contracts.TimelineEvent{{CreatedAt: time.Date(2026, time.April, 2, 10, 0, 0, 0, time.UTC)}}},
			{SessionID: "ses-3", Status: "resolved"},
		}
		sortSessionDetails(sessions, "status", "asc")
		if sessions[0].Status != "failed" || sessions[2].Status != "resolved" {
			t.Fatalf("expected status ascending sort, got %+v", sessions)
		}
		sortSessionDetails(sessions, "created_at", "asc")
		if sessions[0].SessionID != "ses-3" {
			t.Fatalf("expected zero-timeline session to sort first in ascending created_at order, got %+v", sessions)
		}
		sortSessionDetails(sessions, "updated_at", "desc")
		if sessions[0].SessionID != "ses-1" {
			t.Fatalf("expected latest timeline first, got %+v", sessions)
		}
		sortSessionDetails(sessions, "status", "desc")
		if sessions[0].Status != "resolved" {
			t.Fatalf("expected status descending sort, got %+v", sessions)
		}
		if got := sessionSortTime(contracts.SessionDetail{}); !got.IsZero() {
			t.Fatalf("expected zero time for session without timeline, got %v", got)
		}

		executions := []contracts.ExecutionDetail{
			{ExecutionID: "exe-2", Status: "pending", TargetHost: "worker-1", CreatedAt: time.Date(2026, time.April, 2, 9, 0, 0, 0, time.UTC), CompletedAt: time.Date(2026, time.April, 2, 9, 5, 0, 0, time.UTC)},
			{ExecutionID: "exe-1", Status: "completed", TargetHost: "api-1", CreatedAt: time.Date(2026, time.April, 2, 8, 0, 0, 0, time.UTC), CompletedAt: time.Date(2026, time.April, 2, 8, 5, 0, 0, time.UTC)},
		}
		sortExecutionDetails(executions, "execution_id", "asc")
		if executions[0].ExecutionID != "exe-1" {
			t.Fatalf("expected execution_id ascending sort, got %+v", executions)
		}
		sortExecutionDetails(executions, "status", "desc")
		if executions[0].Status != "pending" {
			t.Fatalf("expected status descending sort, got %+v", executions)
		}
		sortExecutionDetails(executions, "target_host", "asc")
		if executions[0].TargetHost != "api-1" {
			t.Fatalf("expected target_host ascending sort, got %+v", executions)
		}
		sortExecutionDetails(executions, "created_at", "asc")
		if executions[0].ExecutionID != "exe-1" {
			t.Fatalf("expected created_at ascending sort, got %+v", executions)
		}
		sortExecutionDetails(executions, "completed_at", "desc")
		if executions[0].ExecutionID != "exe-2" {
			t.Fatalf("expected completed_at descending sort, got %+v", executions)
		}
	})

	t.Run("handle execution result failed and unknown statuses", func(t *testing.T) {
		t.Parallel()

		service := NewService(Options{})
		sessionID := "ses-execution-branches"
		service.sessions[sessionID] = &sessionRecord{
			detail: contracts.SessionDetail{
				SessionID: sessionID,
				Status:    "executing",
				Alert: map[string]interface{}{
					"host":     "api-7",
					"severity": "warning",
					"labels":   map[string]interface{}{"service": "payments"},
				},
			},
			host: "api-7",
		}

		service.executions["exe-failed"] = contracts.ExecutionDetail{ExecutionID: "exe-failed", Status: "executing"}
		service.executionSession["exe-failed"] = sessionID
		if result, err := service.HandleExecutionResult(context.Background(), contracts.ExecutionResult{
			ExecutionID: "exe-failed",
			SessionID:   sessionID,
			Status:      "failed",
			ExitCode:    1,
			OutputRef:   "s3://outputs/exe-failed",
		}); err != nil {
			t.Fatalf("handle failed execution result: %v", err)
		} else if result.Status != "failed" {
			t.Fatalf("expected failed execution result, got %+v", result)
		}
		if !hasTimelineEvent(service.sessions[sessionID].detail.Timeline, "execution_failed") {
			t.Fatalf("expected execution_failed timeline, got %+v", service.sessions[sessionID].detail.Timeline)
		}

		service.executions["exe-other"] = contracts.ExecutionDetail{ExecutionID: "exe-other", Status: "executing"}
		service.executionSession["exe-other"] = sessionID
		if result, err := service.HandleExecutionResult(context.Background(), contracts.ExecutionResult{
			ExecutionID: "exe-other",
			SessionID:   sessionID,
			Status:      "cancelled",
			ExitCode:    130,
		}); err != nil {
			t.Fatalf("handle unknown execution result: %v", err)
		} else if result.Status != "verifying" {
			t.Fatalf("expected verifying execution result, got %+v", result)
		}
		if !hasTimelineEvent(service.sessions[sessionID].detail.Timeline, "execution_result_received") {
			t.Fatalf("expected execution_result_received timeline, got %+v", service.sessions[sessionID].detail.Timeline)
		}
	})
}
