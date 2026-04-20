package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"tars/internal/contracts"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/connectors"
	"tars/internal/modules/reasoning"
)

type Options struct {
	Logger                  *slog.Logger
	DiagnosisEnabled        bool
	ApprovalEnabled         bool
	ExecutionEnabled        bool
	KnowledgeIngestEnabled  bool
	ApprovalTimeout         time.Duration
	ApprovalRouter          approvalrouting.Router
	AuthorizationPolicy     authorization.Evaluator
	AgentRoleManager        *agentrole.Manager
	DesensitizationProvider reasoning.DesensitizationProvider
	Connectors              *connectors.Manager
	OutputChunkBytes        int
	OutputRetention         time.Duration
}

type Store struct {
	db     *sql.DB
	logger *slog.Logger
	opts   Options
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

func NewStore(db *sql.DB, opts Options) *Store {
	if opts.ApprovalTimeout <= 0 {
		opts.ApprovalTimeout = 15 * time.Minute
	}
	if opts.OutputChunkBytes <= 0 {
		opts.OutputChunkBytes = 16384
	}
	if opts.OutputRetention <= 0 {
		opts.OutputRetention = 168 * time.Hour
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Store{db: db, logger: logger, opts: opts}
}

func (s *Store) HandleAlertEvent(ctx context.Context, event contracts.AlertEvent) (contracts.SessionMutationResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contracts.SessionMutationResult{}, err
	}
	defer tx.Rollback()

	fingerprint := strings.TrimSpace(event.Fingerprint)
	if fingerprint == "" {
		fingerprint = randomID("fp")
	}

	now := event.ReceivedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	if key := strings.TrimSpace(event.IdempotencyKey); key != "" {
		requestHash := strings.TrimSpace(event.RequestHash)
		if requestHash == "" {
			requestHash = key
		}
		duplicate, err := s.ensureIdempotency(ctx, tx, "vmalert_alert", key, requestHash, now)
		if err != nil {
			return contracts.SessionMutationResult{}, err
		}
		if duplicate {
			var existingSessionID string
			var existingStatus string
			err = tx.QueryRowContext(ctx, `
				SELECT s.id, s.status
				FROM alert_sessions s
				JOIN alert_events e ON e.id = s.alert_event_id
				WHERE e.fingerprint = $1
				ORDER BY s.opened_at DESC
				LIMIT 1
			`, fingerprint).Scan(&existingSessionID, &existingStatus)
			if err != nil {
				return contracts.SessionMutationResult{}, err
			}
			if err := tx.Commit(); err != nil {
				return contracts.SessionMutationResult{}, err
			}
			return contracts.SessionMutationResult{
				SessionID:  existingSessionID,
				Status:     existingStatus,
				Duplicated: true,
			}, nil
		}
	}

	var duplicateSessionID string
	var duplicateStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT s.id, s.status
		FROM alert_sessions s
		JOIN alert_events e ON e.id = s.alert_event_id
		WHERE e.fingerprint = $1
		ORDER BY s.opened_at DESC
		LIMIT 1
	`, fingerprint).Scan(&duplicateSessionID, &duplicateStatus)
	switch {
	case err == nil:
		if err := s.insertSessionEvent(ctx, tx, duplicateSessionID, "alert_repeated", "received duplicate alert for existing session", now); err != nil {
			return contracts.SessionMutationResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return contracts.SessionMutationResult{}, err
		}
		return contracts.SessionMutationResult{
			SessionID:  duplicateSessionID,
			Status:     duplicateStatus,
			Duplicated: true,
		}, nil
	case !errors.Is(err, sql.ErrNoRows):
		return contracts.SessionMutationResult{}, err
	}

	alertEventID := randomUUID()
	sessionID := randomUUID()
	host := pickHost(event.Labels)
	status := "analyzing"
	diagnosisSummary := "Diagnosis queued"
	outboxStatus := "pending"
	blockedReason := ""
	if !s.opts.DiagnosisEnabled {
		status = "open"
		diagnosisSummary = "Diagnosis feature disabled"
		outboxStatus = "blocked"
		blockedReason = "diagnosis_disabled"
	}

	labelsJSON, err := json.Marshal(event.Labels)
	if err != nil {
		return contracts.SessionMutationResult{}, err
	}
	annotationsJSON, err := json.Marshal(event.Annotations)
	if err != nil {
		return contracts.SessionMutationResult{}, err
	}
	rawPayloadJSON, err := json.Marshal(map[string]interface{}{
		"labels":      event.Labels,
		"annotations": event.Annotations,
		"source":      event.Source,
		"severity":    event.Severity,
	})
	if err != nil {
		return contracts.SessionMutationResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO alert_events (
			id, tenant_id, source, severity, labels, annotations, raw_payload, fingerprint, received_at
		) VALUES ($1, 'default', $2, $3, $4, $5, $6, $7, $8)
	`, alertEventID, event.Source, event.Severity, labelsJSON, annotationsJSON, rawPayloadJSON, fingerprint, now); err != nil {
		return contracts.SessionMutationResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO alert_sessions (
			id, tenant_id, alert_event_id, status, service_name, target_host, diagnosis_summary, agent_role_id, opened_at, updated_at
		) VALUES ($1, 'default', $2, $3, $4, $5, $6, 'diagnosis', $7, $7)
	`, sessionID, alertEventID, status, event.Labels["service"], host, diagnosisSummary, now); err != nil {
		return contracts.SessionMutationResult{}, err
	}

	if err := s.insertSessionEvent(ctx, tx, sessionID, "alert_received", "alert ingested from vmalert webhook", now); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	if outboxStatus == "pending" {
		if err := s.insertSessionEvent(ctx, tx, sessionID, "diagnosis_requested", "queued diagnosis request for reasoning pipeline", now); err != nil {
			return contracts.SessionMutationResult{}, err
		}
	} else {
		if err := s.insertSessionEvent(ctx, tx, sessionID, "diagnosis_blocked", "diagnosis request blocked by feature flag", now); err != nil {
			return contracts.SessionMutationResult{}, err
		}
	}

	if _, err := s.publishEventTx(ctx, tx, contracts.EventPublishRequest{
		Topic:         "session.analyze_requested",
		AggregateID:   sessionID,
		Payload:       []byte(`{}`),
		Status:        outboxStatus,
		BlockedReason: blockedReason,
		CreatedAt:     now,
		AvailableAt:   now,
	}); err != nil {
		return contracts.SessionMutationResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return contracts.SessionMutationResult{}, err
	}

	return contracts.SessionMutationResult{
		SessionID: sessionID,
		Status:    status,
	}, nil
}

func (s *Store) HandleChannelEvent(ctx context.Context, event contracts.ChannelEvent) (contracts.WorkflowDispatchResult, error) {
	if strings.TrimSpace(event.ExecutionID) == "" {
		return contracts.WorkflowDispatchResult{}, contracts.ErrNotFound
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contracts.WorkflowDispatchResult{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if event.IdempotencyKey != "" {
		duplicate, err := s.ensureIdempotency(ctx, tx, "telegram_update", event.IdempotencyKey, fmt.Sprintf("%s:%s:%s", event.ExecutionID, event.Action, event.Command), now)
		if err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if duplicate {
			if err := tx.Commit(); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			return contracts.WorkflowDispatchResult{}, nil
		}
	}

	executionDetail, sessionID, err := s.lockExecution(ctx, tx, event.ExecutionID)
	if err != nil {
		return contracts.WorkflowDispatchResult{}, err
	}
	if executionDetail.Status != "pending" {
		return contracts.WorkflowDispatchResult{}, contracts.ErrInvalidState
	}

	if _, err := s.lockSession(ctx, tx, sessionID); err != nil {
		return contracts.WorkflowDispatchResult{}, err
	}

	switch event.Action {
	case "approve", "modify_approve":
		if executionDetail.RequestKind == "capability" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE execution_requests
				SET status = 'executing', approved_by = $2, approved_at = $3, version = version + 1
				WHERE id = $1
			`, executionDetail.ExecutionID, event.UserID, now); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			if err := s.updateSessionStatus(ctx, tx, sessionID, "executing", now, false); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			if err := s.insertApprovalRecord(ctx, tx, executionDetail.ExecutionID, event.Action, event.UserID, executionDetail.CapabilityID, executionDetail.CapabilityID, now); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			if err := s.insertSessionEvent(ctx, tx, sessionID, "capability_approval_accepted", fmt.Sprintf("capability approval %s approved by %s capability=%s connector=%s", executionDetail.ExecutionID, event.UserID, executionDetail.CapabilityID, executionDetail.ConnectorID), now); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			if err := tx.Commit(); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			return contracts.WorkflowDispatchResult{
				SessionID: sessionID,
				Notifications: []contracts.ChannelMessage{{
					Channel: event.Channel,
					Target:  event.ChatID,
					Body:    fmt.Sprintf("[%s] capability approved, invoking %s on connector %s", executionDetail.ExecutionID, executionDetail.CapabilityID, executionDetail.ConnectorID),
				}},
				Capabilities: []contracts.ApprovedCapabilityRequest{{
					ApprovalID:    executionDetail.ExecutionID,
					SessionID:     sessionID,
					StepID:        executionDetail.StepID,
					ConnectorID:   executionDetail.ConnectorID,
					CapabilityID:  executionDetail.CapabilityID,
					Params:        cloneInterfaceMap(executionDetail.CapabilityParams),
					RequestedBy:   executionDetail.RequestedBy,
					ApprovalGroup: executionDetail.ApprovalGroup,
					Runtime:       contracts.CloneRuntimeMetadata(executionDetail.Runtime),
				}},
			}, nil
		}
		command := executionDetail.Command
		if event.Action == "modify_approve" && strings.TrimSpace(event.Command) != "" {
			command = strings.TrimSpace(event.Command)
		}
		if !s.opts.ExecutionEnabled {
			if _, err := tx.ExecContext(ctx, `
				UPDATE execution_requests
				SET status = 'approved', approved_by = $2, approved_at = $3, command = $4, version = version + 1
				WHERE id = $1
			`, executionDetail.ExecutionID, event.UserID, now, command); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			if err := s.updateSessionStatus(ctx, tx, sessionID, "open", now, false); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			if err := s.insertApprovalRecord(ctx, tx, executionDetail.ExecutionID, event.Action, event.UserID, executionDetail.Command, command, now); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			if err := s.insertSessionEvent(ctx, tx, sessionID, "approval_accepted_manual", fmt.Sprintf("execution %s approved by %s but execution is disabled command=%s %s", executionDetail.ExecutionID, event.UserID, command, postgresExecutionRuntimeSummary(executionDetail)), now); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			if err := tx.Commit(); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			return contracts.WorkflowDispatchResult{
				SessionID: sessionID,
				Notifications: []contracts.ChannelMessage{{
					Channel: event.Channel,
					Target:  event.ChatID,
					Body:    fmt.Sprintf("[%s] execution approved by %s but execution is disabled; please handle manually", executionDetail.ExecutionID, event.UserID),
				}},
			}, nil
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE execution_requests
			SET status = 'executing', approved_by = $2, approved_at = $3, command = $4, version = version + 1
			WHERE id = $1
		`, executionDetail.ExecutionID, event.UserID, now, command); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := s.updateSessionStatus(ctx, tx, sessionID, "executing", now, false); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := s.insertApprovalRecord(ctx, tx, executionDetail.ExecutionID, event.Action, event.UserID, executionDetail.Command, command, now); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := s.insertSessionEvent(ctx, tx, sessionID, "approval_accepted", fmt.Sprintf("execution %s approved by %s command=%s %s", executionDetail.ExecutionID, event.UserID, command, postgresExecutionRuntimeSummary(executionDetail)), now); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		var serviceName string
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(service_name, '') FROM alert_sessions WHERE id = $1`, sessionID).Scan(&serviceName); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		return contracts.WorkflowDispatchResult{
			SessionID: sessionID,
			Notifications: []contracts.ChannelMessage{{
				Channel: event.Channel,
				Target:  event.ChatID,
				Body:    fmt.Sprintf("[%s] execution approved, starting command on %s", executionDetail.ExecutionID, executionDetail.TargetHost),
			}},
			Executions: []contracts.ApprovedExecutionRequest{{
				ExecutionID:     executionDetail.ExecutionID,
				SessionID:       sessionID,
				TargetHost:      executionDetail.TargetHost,
				Command:         command,
				Service:         serviceName,
				ConnectorID:     executionDetail.ConnectorID,
				ConnectorType:   executionDetail.ConnectorType,
				ConnectorVendor: executionDetail.ConnectorVendor,
				Protocol:        executionDetail.Protocol,
				ExecutionMode:   executionDetail.ExecutionMode,
			}},
		}, nil

	case "reject":
		eventType := "approval_rejected"
		message := fmt.Sprintf("execution %s rejected by %s", executionDetail.ExecutionID, event.UserID)
		originalCommand := executionDetail.Command
		if executionDetail.RequestKind == "capability" {
			eventType = "capability_approval_rejected"
			message = fmt.Sprintf("capability approval %s rejected by %s", executionDetail.ExecutionID, event.UserID)
			originalCommand = executionDetail.CapabilityID
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE execution_requests
			SET status = 'rejected', version = version + 1
			WHERE id = $1
		`, executionDetail.ExecutionID); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := s.updateSessionStatus(ctx, tx, sessionID, "analyzing", now, false); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := s.insertApprovalRecord(ctx, tx, executionDetail.ExecutionID, event.Action, event.UserID, originalCommand, originalCommand, now); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := s.insertSessionEvent(ctx, tx, sessionID, eventType, message, now); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		return contracts.WorkflowDispatchResult{
			SessionID: sessionID,
			Notifications: []contracts.ChannelMessage{{
				Channel: event.Channel,
				Target:  event.ChatID,
				Body:    fmt.Sprintf("[%s] execution rejected by %s", executionDetail.ExecutionID, event.UserID),
			}},
		}, nil

	case "request_context":
		eventType := "approval_requested_context"
		message := fmt.Sprintf("additional context requested by %s", event.UserID)
		originalCommand := executionDetail.Command
		if executionDetail.RequestKind == "capability" {
			eventType = "capability_approval_requested_context"
			message = fmt.Sprintf("additional capability context requested by %s", event.UserID)
			originalCommand = executionDetail.CapabilityID
		}
		if err := s.updateSessionStatus(ctx, tx, sessionID, "analyzing", now, false); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := s.insertApprovalRecord(ctx, tx, executionDetail.ExecutionID, event.Action, event.UserID, originalCommand, originalCommand, now); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := s.insertSessionEvent(ctx, tx, sessionID, eventType, message, now); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		return contracts.WorkflowDispatchResult{
			SessionID: sessionID,
			Notifications: []contracts.ChannelMessage{{
				Channel: event.Channel,
				Target:  event.ChatID,
				Body:    fmt.Sprintf("[%s] additional context requested", executionDetail.ExecutionID),
			}},
		}, nil

	default:
		return contracts.WorkflowDispatchResult{}, fmt.Errorf("unsupported channel action: %s", event.Action)
	}
}

func (s *Store) HandleExecutionResult(ctx context.Context, result contracts.ExecutionResult) (contracts.SessionMutationResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contracts.SessionMutationResult{}, err
	}
	defer tx.Rollback()

	executionDetail, sessionID, err := s.lockExecution(ctx, tx, result.ExecutionID)
	if err != nil {
		return contracts.SessionMutationResult{}, err
	}
	if result.SessionID != "" {
		sessionID = result.SessionID
	}

	now := time.Now().UTC()
	completedAt := now
	if _, err := tx.ExecContext(ctx, `
		UPDATE execution_requests
		SET status = $2, output_ref = $3, exit_code = $4, output_bytes = $5, output_truncated = $6, completed_at = $7, version = version + 1
		WHERE id = $1
	`, executionDetail.ExecutionID, result.Status, nullableString(result.OutputRef), result.ExitCode, result.OutputBytes, result.OutputTruncated, completedAt); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	executionDetail.Runtime = contracts.CloneRuntimeMetadata(result.Runtime)
	if err := s.replaceExecutionOutputChunks(ctx, tx, executionDetail.ExecutionID, result.OutputPreview, now); err != nil {
		return contracts.SessionMutationResult{}, err
	}

	sessionStatus := "verifying"
	eventType := "execution_result_received"
	message := fmt.Sprintf("execution %s returned status %s %s", result.ExecutionID, result.Status, postgresExecutionRuntimeSummary(executionDetail))
	switch result.Status {
	case "completed":
		eventType = "execution_completed"
		message = fmt.Sprintf("execution %s completed successfully %s", result.ExecutionID, postgresExecutionRuntimeSummary(executionDetail))
	case "failed", "timeout":
		sessionStatus = "failed"
		eventType = "execution_failed"
		message = fmt.Sprintf("execution %s ended with status %s %s", result.ExecutionID, result.Status, postgresExecutionRuntimeSummary(executionDetail))
	}

	if err := s.updateSessionStatus(ctx, tx, sessionID, sessionStatus, now, sessionStatus == "resolved"); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	if err := s.insertSessionEvent(ctx, tx, sessionID, eventType, message, now); err != nil {
		return contracts.SessionMutationResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	return contracts.SessionMutationResult{
		SessionID: sessionID,
		Status:    sessionStatus,
	}, nil
}

func (s *Store) HandleCapabilityResult(ctx context.Context, result contracts.CapabilityExecutionResult) (contracts.SessionMutationResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contracts.SessionMutationResult{}, err
	}
	defer tx.Rollback()

	executionDetail, sessionID, err := s.lockExecution(ctx, tx, result.ApprovalID)
	if err != nil {
		return contracts.SessionMutationResult{}, err
	}
	if result.SessionID != "" {
		sessionID = result.SessionID
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE execution_requests
		SET status = $2, capability_params = COALESCE(capability_params, '{}'::jsonb), completed_at = $3, version = version + 1
		WHERE id = $1
	`, result.ApprovalID, result.Status, now); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	status := "open"
	eventType := "capability_completed"
	message := fmt.Sprintf("capability approval %s completed capability=%s connector=%s", result.ApprovalID, executionDetail.CapabilityID, executionDetail.ConnectorID)
	if result.Status != "completed" {
		status = "failed"
		eventType = "capability_failed"
		message = fmt.Sprintf("capability approval %s failed: %s", result.ApprovalID, firstNonEmpty(result.Error, result.Status))
	}
	if err := s.updateSessionStatus(ctx, tx, sessionID, status, now, false); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	if err := s.insertSessionEvent(ctx, tx, sessionID, eventType, message, now); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	if len(result.Artifacts) > 0 || len(result.Output) > 0 || len(result.Metadata) > 0 || result.Error != "" {
		sessionDetail, _, err := s.loadSessionForMutation(ctx, tx, sessionID)
		if err != nil {
			return contracts.SessionMutationResult{}, err
		}
		updateSessionToolPlanStep(&sessionDetail, executionDetail.StepID, func(step *contracts.ToolPlanStep) {
			step.Status = result.Status
			step.Runtime = contracts.CloneRuntimeMetadata(result.Runtime)
			step.CompletedAt = now
			step.Output = map[string]interface{}{"approval_id": result.ApprovalID, "status": result.Status, "result": cloneInterfaceMap(result.Output), "metadata": cloneInterfaceMap(result.Metadata), "error": result.Error}
		})
		toolPlanJSON, err := json.Marshal(sessionDetail.ToolPlan)
		if err != nil {
			return contracts.SessionMutationResult{}, err
		}
		attachments := append([]contracts.MessageAttachment(nil), sessionDetail.Attachments...)
		attachments = append(attachments, result.Artifacts...)
		attachmentsJSON, err := json.Marshal(attachments)
		if err != nil {
			return contracts.SessionMutationResult{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE alert_sessions
			SET tool_plan = $2, attachments = $3, updated_at = $4, version = version + 1
			WHERE id = $1
		`, sessionID, nullableJSON(toolPlanJSON), nullableJSON(attachmentsJSON), now); err != nil {
			return contracts.SessionMutationResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	return contracts.SessionMutationResult{SessionID: sessionID, Status: status}, nil
}

func (s *Store) HandleVerificationResult(ctx context.Context, result contracts.VerificationResult) (contracts.SessionMutationResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contracts.SessionMutationResult{}, err
	}
	defer tx.Rollback()

	now := result.CheckedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	verificationJSON, err := json.Marshal(map[string]interface{}{
		"status":     result.Status,
		"summary":    result.Summary,
		"details":    result.Details,
		"runtime":    result.Runtime,
		"checked_at": now,
	})
	if err != nil {
		return contracts.SessionMutationResult{}, err
	}

	sessionStatus := "analyzing"
	eventType := "verify_failed"
	switch result.Status {
	case "success", "skipped":
		sessionStatus = "resolved"
		eventType = "verify_success"
	case "failed":
		sessionStatus = "analyzing"
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE alert_sessions
		SET status = $2::session_status,
		    verification_result = $3,
		    updated_at = $4,
		    resolved_at = CASE WHEN $2::text = 'resolved' THEN $4 ELSE resolved_at END,
		    version = version + 1
		WHERE id = $1
	`, result.SessionID, sessionStatus, verificationJSON, now); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	if err := s.insertSessionEvent(ctx, tx, result.SessionID, eventType, result.Summary, now); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	if sessionStatus == "resolved" {
		if err := s.enqueueSessionClosed(ctx, tx, result.SessionID, now); err != nil {
			return contracts.SessionMutationResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return contracts.SessionMutationResult{}, err
	}
	return contracts.SessionMutationResult{
		SessionID: result.SessionID,
		Status:    sessionStatus,
	}, nil
}

func (s *Store) ListSessions(ctx context.Context, filter contracts.ListSessionsFilter) ([]contracts.SessionDetail, error) {
	query := `
		SELECT s.id
		FROM alert_sessions s
		JOIN alert_events e ON e.id = s.alert_event_id
		WHERE ($1 = '' OR s.status::text = $1)
		  AND ($2 = '' OR s.target_host = $2)
		  AND (
		    $3 = ''
		    OR s.id::text ILIKE '%' || $3 || '%'
		    OR s.target_host ILIKE '%' || $3 || '%'
		    OR s.service_name ILIKE '%' || $3 || '%'
		    OR COALESCE(e.labels->>'alertname', '') ILIKE '%' || $3 || '%'
		    OR COALESCE(e.annotations->>'summary', '') ILIKE '%' || $3 || '%'
		  )
	`
	query += " ORDER BY " + sessionSortColumn(filter.SortBy) + " " + sortDirection(filter.SortOrder)

	rows, err := s.db.QueryContext(ctx, query, filter.Status, filter.Host, strings.TrimSpace(filter.Query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]contracts.SessionDetail, 0)
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, err
		}
		item, err := s.GetSession(ctx, sessionID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(filter.SortBy), "triage") {
		contracts.SortSessionsForTriage(items, filter.SortOrder)
	}
	return items, nil
}

func (s *Store) ListExecutions(ctx context.Context, filter contracts.ListExecutionsFilter) ([]contracts.ExecutionDetail, error) {
	query := `
		SELECT id
		FROM execution_requests
		WHERE ($1 = '' OR status::text = $1)
		  AND (
		    $2 = ''
		    OR id::text ILIKE '%' || $2 || '%'
		    OR target_host ILIKE '%' || $2 || '%'
		    OR command ILIKE '%' || $2 || '%'
		    OR requested_by ILIKE '%' || $2 || '%'
		    OR approval_group ILIKE '%' || $2 || '%'
		  )
	`
	query += " ORDER BY " + executionSortColumn(filter.SortBy) + " " + sortDirection(filter.SortOrder)

	rows, err := s.db.QueryContext(ctx, query, filter.Status, strings.TrimSpace(filter.Query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]contracts.ExecutionDetail, 0)
	for rows.Next() {
		var executionID string
		if err := rows.Scan(&executionID); err != nil {
			return nil, err
		}
		item, err := s.GetExecution(ctx, executionID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(filter.SortBy), "triage") {
		contracts.SortExecutionsForTriage(items, filter.SortOrder)
	}
	return items, nil
}

func (s *Store) GetSession(ctx context.Context, sessionID string) (contracts.SessionDetail, error) {
	return s.loadSessionDetail(ctx, s.db, sessionID)
}

func (s *Store) GetExecution(ctx context.Context, executionID string) (contracts.ExecutionDetail, error) {
	return s.loadExecutionDetail(ctx, s.db, executionID)
}

func (s *Store) GetExecutionOutput(ctx context.Context, executionID string) ([]contracts.ExecutionOutputChunk, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT seq, stream_type, content, byte_size, created_at
		FROM execution_output_chunks
		WHERE execution_request_id = $1
		ORDER BY seq ASC
	`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]contracts.ExecutionOutputChunk, 0)
	for rows.Next() {
		var item contracts.ExecutionOutputChunk
		if err := rows.Scan(
			&item.Seq,
			&item.StreamType,
			&item.Content,
			&item.ByteSize,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) > 0 {
		return items, nil
	}
	if _, err := s.loadExecutionDetail(ctx, s.db, executionID); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListOutbox(ctx context.Context, filter contracts.ListOutboxFilter) ([]contracts.OutboxEvent, error) {
	query := `
		SELECT id, topic, status, aggregate_id, retry_count, COALESCE(last_error, ''), COALESCE(blocked_reason, ''), created_at
		FROM outbox_events
	`
	args := []interface{}{}
	if filter.Status != "" {
		query += ` WHERE status::text = $1`
		args = append(args, filter.Status)
	} else {
		query += ` WHERE status::text IN ('blocked', 'failed')`
	}
	if strings.TrimSpace(filter.Query) != "" {
		args = append(args, strings.TrimSpace(filter.Query))
		query += fmt.Sprintf(` AND (
			id ILIKE '%%' || $%d || '%%'
			OR topic ILIKE '%%' || $%d || '%%'
			OR aggregate_id ILIKE '%%' || $%d || '%%'
			OR COALESCE(last_error, '') ILIKE '%%' || $%d || '%%'
			OR COALESCE(blocked_reason, '') ILIKE '%%' || $%d || '%%'
		)`, len(args), len(args), len(args), len(args), len(args))
	}
	query += ` ORDER BY ` + outboxSortColumn(filter.SortBy) + ` ` + sortDirection(filter.SortOrder)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]contracts.OutboxEvent, 0)
	for rows.Next() {
		var item contracts.OutboxEvent
		if err := rows.Scan(&item.ID, &item.Topic, &item.Status, &item.AggregateID, &item.RetryCount, &item.LastError, &item.BlockedReason, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ReplayOutbox(ctx context.Context, eventID string, operatorReason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var topic string
	var status string
	var aggregateID string
	err = tx.QueryRowContext(ctx, `
		SELECT topic, status, aggregate_id
		FROM outbox_events
		WHERE id = $1
		FOR UPDATE
	`, eventID).Scan(&topic, &status, &aggregateID)
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status == "blocked" && !s.topicEnabled(topic) {
		return contracts.ErrBlockedByFeatureFlag
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE outbox_events
		SET status = 'pending', last_error = NULL, blocked_reason = NULL, retry_count = retry_count + 1, available_at = $2
		WHERE id = $1
	`, eventID, time.Now().UTC()); err != nil {
		return err
	}
	if err := s.insertSessionEvent(ctx, tx, aggregateID, "outbox_replayed", fmt.Sprintf("operator replayed outbox event: %s", operatorReason), time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteOutbox(ctx context.Context, eventID string, operatorReason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	var aggregateID string
	err = tx.QueryRowContext(ctx, `
		SELECT status, aggregate_id
		FROM outbox_events
		WHERE id = $1
		FOR UPDATE
	`, eventID).Scan(&status, &aggregateID)
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "failed" && status != "blocked" {
		return contracts.ErrInvalidState
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM outbox_events WHERE id = $1`, eventID); err != nil {
		return err
	}
	if err := s.insertSessionEvent(ctx, tx, aggregateID, "outbox_deleted", fmt.Sprintf("operator deleted outbox event: %s", operatorReason), time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RecoverProcessingOutbox(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status = 'pending'
		WHERE status = 'processing'
	`)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (s *Store) SweepApprovalTimeouts(ctx context.Context, now time.Time) ([]contracts.ChannelMessage, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, session_id, requested_by
		FROM execution_requests
		WHERE status = 'pending'
		  AND created_at <= $1
		FOR UPDATE SKIP LOCKED
	`, now.Add(-s.opts.ApprovalTimeout))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type expiredApproval struct {
		executionID string
		sessionID   string
		requestedBy string
	}
	expired := make([]expiredApproval, 0)
	for rows.Next() {
		var item expiredApproval
		if err := rows.Scan(&item.executionID, &item.sessionID, &item.requestedBy); err != nil {
			return nil, err
		}
		expired = append(expired, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	notifications := make([]contracts.ChannelMessage, 0)
	for _, item := range expired {
		if _, err := tx.ExecContext(ctx, `
			UPDATE execution_requests
			SET status = 'rejected', version = version + 1
			WHERE id = $1
		`, item.executionID); err != nil {
			return nil, err
		}
		if err := s.updateSessionStatus(ctx, tx, item.sessionID, "open", now, false); err != nil {
			return nil, err
		}
		if err := s.insertApprovalRecord(ctx, tx, item.executionID, "reject", "system:approval_timeout", "", "", now); err != nil {
			return nil, err
		}
		if err := s.insertSessionEvent(ctx, tx, item.sessionID, "approval_timed_out", fmt.Sprintf("execution %s timed out waiting for approval", item.executionID), now); err != nil {
			return nil, err
		}
		sessionDetail, err := s.loadSessionDetail(ctx, tx, item.sessionID)
		if err != nil {
			return nil, err
		}
		route := resolveApprovalRoute(s.opts.ApprovalRouter, sessionDetail.Alert, item.requestedBy)
		for _, target := range route.Targets {
			notifications = append(notifications, contracts.ChannelMessage{
				Channel: notificationChannelForAlert(sessionDetail.Alert),
				Target:  notificationTargetForAlert(sessionDetail.Alert, target),
				Body:    fmt.Sprintf("[%s] approval timed out for execution %s; request was auto-rejected", item.sessionID, item.executionID),
			})
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return notifications, nil
}

func (s *Store) ClaimOutboxBatch(ctx context.Context, limit int) ([]contracts.DispatchableOutboxEvent, error) {
	events, err := s.ClaimEvents(ctx, limit)
	if err != nil {
		return nil, err
	}
	items := make([]contracts.DispatchableOutboxEvent, 0, len(events))
	for _, event := range events {
		items = append(items, toDispatchableOutboxEvent(event))
	}
	return items, nil
}

func (s *Store) ClaimEvents(ctx context.Context, limit int) ([]contracts.EventEnvelope, error) {
	if limit <= 0 {
		limit = 1
	}

	rows, err := s.db.QueryContext(ctx, `
		WITH claimed AS (
			SELECT id
			FROM outbox_events
			WHERE status = 'pending'
			  AND available_at <= NOW()
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE outbox_events o
		SET status = 'processing'
		FROM claimed c
		WHERE o.id = c.id
		RETURNING o.id, o.topic, o.aggregate_id, o.payload, o.status, o.retry_count, COALESCE(o.last_error, ''), o.available_at, o.created_at
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]contracts.EventEnvelope, 0)
	for rows.Next() {
		var item contracts.EventEnvelope
		var retryCount int
		if err := rows.Scan(&item.EventID, &item.Topic, &item.AggregateID, &item.Payload, &item.Status, &retryCount, &item.LastError, &item.AvailableAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Attempt = retryCount + 1
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) EnqueueNotifications(ctx context.Context, sessionID string, messages []contracts.ChannelMessage) error {
	if len(messages) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	for _, message := range messages {
		payload, err := contracts.EncodeChannelMessage(message)
		if err != nil {
			return err
		}
		if _, err := s.publishEventTx(ctx, tx, contracts.EventPublishRequest{
			Topic:       "telegram.send",
			AggregateID: sessionID,
			Payload:     payload,
			Status:      "pending",
			CreatedAt:   now,
			AvailableAt: now,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ApplyDiagnosis(ctx context.Context, eventID string, diagnosis contracts.DiagnosisOutput) (contracts.WorkflowDispatchResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contracts.WorkflowDispatchResult{}, err
	}
	defer tx.Rollback()

	var sessionID string
	err = tx.QueryRowContext(ctx, `
		SELECT aggregate_id
		FROM outbox_events
		WHERE id = $1
		FOR UPDATE
	`, eventID).Scan(&sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.WorkflowDispatchResult{}, contracts.ErrNotFound
	}
	if err != nil {
		return contracts.WorkflowDispatchResult{}, err
	}

	sessionDetail, severity, err := s.loadSessionForMutation(ctx, tx, sessionID)
	if err != nil {
		return contracts.WorkflowDispatchResult{}, err
	}

	status := "open"
	now := time.Now().UTC()
	diagnosis = rehydrateDiagnosisOutput(diagnosis, s.currentDesensitizationConfig())
	var desenseJSON []byte
	if len(diagnosis.DesenseMap) > 0 {
		desenseJSON, err = json.Marshal(diagnosis.DesenseMap)
		if err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
	}
	var toolPlanJSON []byte
	if len(diagnosis.ToolPlan) > 0 {
		toolPlanJSON, err = json.Marshal(diagnosis.ToolPlan)
		if err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
	}
	var attachmentsJSON []byte
	if len(diagnosis.Attachments) > 0 {
		attachmentsJSON, err = json.Marshal(diagnosis.Attachments)
		if err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE alert_sessions
		SET diagnosis_summary = $2, status = $3, updated_at = $4, desense_map = $5, tool_plan = $6, attachments = $7, version = version + 1
		WHERE id = $1
	`, sessionID, diagnosis.Summary, status, now, desenseJSON, nullableJSON(toolPlanJSON), nullableJSON(attachmentsJSON)); err != nil {
		return contracts.WorkflowDispatchResult{}, err
	}
	sessionDetail.DiagnosisSummary = diagnosis.Summary
	sessionDetail.Status = status
	sessionDetail.ToolPlan = cloneToolPlanSteps(diagnosis.ToolPlan)
	sessionDetail.Attachments = cloneMessageAttachments(diagnosis.Attachments)

	route := approvalrouting.Route{}
	if s.opts.ApprovalEnabled {
		route = resolveApprovalRoute(s.opts.ApprovalRouter, sessionDetail.Alert, "tars")
	}
	diagnosisTarget := selectDiagnosisNotificationTarget(sessionDetail.Alert, route)
	notifications := []contracts.ChannelMessage{{
		Channel:     notificationChannelForAlert(sessionDetail.Alert),
		Target:      notificationTargetForAlert(sessionDetail.Alert, diagnosisTarget),
		Subject:     "诊断结果",
		RefType:     "session",
		RefID:       sessionID,
		Body:        formatDiagnosisMessage(sessionDetail, diagnosis),
		Attachments: cloneMessageAttachments(diagnosis.Attachments),
	}}
	immediateExecutions := make([]contracts.ApprovedExecutionRequest, 0, 1)
	if err := s.insertSessionEvent(ctx, tx, sessionID, "diagnosis_message_prepared", fmt.Sprintf("target=%s body=%s", notifications[0].Target, compactSnippet(notifications[0].Body)), now); err != nil {
		return contracts.WorkflowDispatchResult{}, err
	}
	source := stringFromAlert(sessionDetail.Alert, "source")
	if diagnosis.ExecutionHint != "" && s.opts.DiagnosisEnabled {
		authDecision := s.resolveAuthorizationDecision(sessionDetail.Alert, diagnosis.ExecutionHint)
		if err := s.insertSessionEvent(ctx, tx, sessionID, "authorization_decided", fmt.Sprintf("action=%s rule=%s matched_by=%s command=%s", authDecision.Action, fallbackString(authDecision.RuleID, "default"), fallbackString(authDecision.MatchedBy, "default"), diagnosis.ExecutionHint), now); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
		switch authDecision.Action {
		case authorization.ActionDirectExecute:
			if s.opts.ExecutionEnabled {
				executionID := randomUUID()
				runtimeSelection := selectExecutionRuntime(s.opts.Connectors)
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO execution_requests (
						id, tenant_id, session_id, target_host, command, command_source, risk_level, requested_by, approved_by, approval_group, status, timeout_seconds, request_kind, connector_id, connector_type, connector_vendor, protocol, execution_mode, created_at, approved_at
					) VALUES ($1, 'default', $2, $3, $4, 'ai_diagnosis', $5, 'tars', 'authorization_policy', 'policy:direct_execute', 'executing', 300, 'execution', $6, $7, $8, $9, $10, $11, $11)
				`, executionID, sessionID, stringFromAlert(sessionDetail.Alert, "host"), diagnosis.ExecutionHint, riskLevelForSeverity(severity), nullableString(runtimeSelection.ConnectorID), nullableString(runtimeSelection.ConnectorType), nullableString(runtimeSelection.ConnectorVendor), runtimeSelection.Protocol, runtimeSelection.ExecutionMode, now); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
				if err := s.updateSessionStatus(ctx, tx, sessionID, "executing", now, false); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
				sessionDetail.Status = "executing"
				if err := s.insertSessionEvent(ctx, tx, sessionID, "execution_direct_ready", fmt.Sprintf("execution %s started directly by authorization policy command=%s %s", executionID, diagnosis.ExecutionHint, postgresExecutionRuntimeSummary(contracts.ExecutionDetail{ConnectorID: runtimeSelection.ConnectorID, Protocol: runtimeSelection.Protocol, ExecutionMode: runtimeSelection.ExecutionMode, Runtime: contracts.CloneRuntimeMetadata(runtimeSelection.Runtime)})), now); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
				notifications = append(notifications, buildAuthorizationDecisionMessage(sessionDetail, diagnosis.ExecutionHint, authDecision, diagnosisTarget, "命中白名单，开始直接执行。"))
				immediateExecutions = append(immediateExecutions, contracts.ApprovedExecutionRequest{
					ExecutionID:     executionID,
					SessionID:       sessionID,
					TargetHost:      stringFromAlert(sessionDetail.Alert, "host"),
					Command:         diagnosis.ExecutionHint,
					Service:         alertLabel(sessionDetail.Alert, "service"),
					ConnectorID:     runtimeSelection.ConnectorID,
					ConnectorType:   runtimeSelection.ConnectorType,
					ConnectorVendor: runtimeSelection.ConnectorVendor,
					Protocol:        runtimeSelection.Protocol,
					ExecutionMode:   runtimeSelection.ExecutionMode,
				})
			} else {
				if err := s.insertSessionEvent(ctx, tx, sessionID, "execution_disabled", "authorization policy allows direct execution but execution is disabled", now); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
				notifications = append(notifications, buildAuthorizationDecisionMessage(sessionDetail, diagnosis.ExecutionHint, authDecision, diagnosisTarget, "命中白名单，但当前执行功能关闭，请手动处理。"))
				if err := s.resolveChatWithoutExecution(ctx, tx, sessionID, &sessionDetail, source, now, "diagnosis answered without execution because execution is disabled"); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
			}
		case authorization.ActionRequireApproval:
			if s.opts.ApprovalEnabled {
				executionID := randomUUID()
				runtimeSelection := selectExecutionRuntime(s.opts.Connectors)
				s.logger.Info(
					"apply diagnosis prepared approval route",
					"session_id", sessionID,
					"execution_id", executionID,
					"route_group", route.GroupKey,
					"route_targets", strings.Join(route.Targets, ","),
					"command", diagnosis.ExecutionHint,
				)
				if err := s.insertSessionEvent(ctx, tx, sessionID, "approval_route_selected", fmt.Sprintf("approval route=%s targets=%s", fallbackString(route.GroupKey, "fallback:unknown"), strings.Join(route.Targets, ",")), now); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO execution_requests (
						id, tenant_id, session_id, target_host, command, command_source, risk_level, requested_by, approval_group, status, timeout_seconds, request_kind, connector_id, connector_type, connector_vendor, protocol, execution_mode, created_at
					) VALUES ($1, 'default', $2, $3, $4, 'ai_diagnosis', $5, 'tars', $6, 'pending', 300, 'execution', $7, $8, $9, $10, $11, $12)
				`, executionID, sessionID, stringFromAlert(sessionDetail.Alert, "host"), diagnosis.ExecutionHint, riskLevelForSeverity(severity), route.GroupKey, nullableString(runtimeSelection.ConnectorID), nullableString(runtimeSelection.ConnectorType), nullableString(runtimeSelection.ConnectorVendor), runtimeSelection.Protocol, runtimeSelection.ExecutionMode, now); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
				if err := s.updateSessionStatus(ctx, tx, sessionID, "pending_approval", now, false); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
				if err := s.insertSessionEvent(ctx, tx, sessionID, "execution_draft_ready", fmt.Sprintf("execution draft %s created command=%s %s", executionID, diagnosis.ExecutionHint, postgresExecutionRuntimeSummary(contracts.ExecutionDetail{ConnectorID: runtimeSelection.ConnectorID, Protocol: runtimeSelection.Protocol, ExecutionMode: runtimeSelection.ExecutionMode, Runtime: contracts.CloneRuntimeMetadata(runtimeSelection.Runtime)})), now); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
				sessionDetail.Status = "pending_approval"
				approvalMessages := buildApprovalMessages(sessionDetail, contracts.ExecutionDetail{
					ExecutionID:     executionID,
					Status:          "pending",
					RiskLevel:       riskLevelForSeverity(severity),
					Command:         diagnosis.ExecutionHint,
					TargetHost:      stringFromAlert(sessionDetail.Alert, "host"),
					ConnectorID:     runtimeSelection.ConnectorID,
					ConnectorType:   runtimeSelection.ConnectorType,
					ConnectorVendor: runtimeSelection.ConnectorVendor,
					Protocol:        runtimeSelection.Protocol,
					ExecutionMode:   runtimeSelection.ExecutionMode,
					RequestedBy:     "tars",
					ApprovalGroup:   route.GroupKey,
					Runtime:         contracts.CloneRuntimeMetadata(runtimeSelection.Runtime),
					CreatedAt:       now,
				}, route, s.opts.ApprovalTimeout)
				notifications = append(notifications, approvalMessages...)
				for _, approvalMessage := range approvalMessages {
					if err := s.insertSessionEvent(ctx, tx, sessionID, "approval_message_prepared", fmt.Sprintf("target=%s body=%s", approvalMessage.Target, compactSnippet(approvalMessage.Body)), now); err != nil {
						return contracts.WorkflowDispatchResult{}, err
					}
				}
			} else {
				if err := s.insertSessionEvent(ctx, tx, sessionID, "approval_disabled", "execution hint produced but approval flow is disabled", now); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
				notifications = append(notifications, buildAuthorizationDecisionMessage(sessionDetail, diagnosis.ExecutionHint, authDecision, diagnosisTarget, "命令需要审批，但当前审批功能关闭，请手动处理。"))
				if err := s.resolveChatWithoutExecution(ctx, tx, sessionID, &sessionDetail, source, now, "diagnosis answered without execution because approval is disabled"); err != nil {
					return contracts.WorkflowDispatchResult{}, err
				}
			}
		case authorization.ActionSuggestOnly:
			if err := s.insertSessionEvent(ctx, tx, sessionID, "execution_suggested_only", fmt.Sprintf("authorization policy marked command as suggest_only: %s", diagnosis.ExecutionHint), now); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			notifications = append(notifications, buildAuthorizationDecisionMessage(sessionDetail, diagnosis.ExecutionHint, authDecision, diagnosisTarget, "命中手动处理策略，当前只展示建议命令，不会自动执行。"))
			if err := s.resolveChatWithoutExecution(ctx, tx, sessionID, &sessionDetail, source, now, "diagnosis answered with suggest-only command"); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
		case authorization.ActionDeny:
			if err := s.insertSessionEvent(ctx, tx, sessionID, "execution_denied", fmt.Sprintf("authorization policy denied command: %s", diagnosis.ExecutionHint), now); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
			notifications = append(notifications, buildAuthorizationDecisionMessage(sessionDetail, diagnosis.ExecutionHint, authDecision, diagnosisTarget, "命中禁止策略，当前不允许执行这条命令。"))
			if err := s.resolveChatWithoutExecution(ctx, tx, sessionID, &sessionDetail, source, now, "diagnosis answered with denied command"); err != nil {
				return contracts.WorkflowDispatchResult{}, err
			}
		}
	}
	if diagnosis.ExecutionHint == "" {
		if err := s.resolveChatWithoutExecution(ctx, tx, sessionID, &sessionDetail, source, now, "diagnosis answered without execution request"); err != nil {
			return contracts.WorkflowDispatchResult{}, err
		}
	}
	if err := s.insertSessionEvent(ctx, tx, sessionID, "diagnosis_completed", diagnosis.Summary, now); err != nil {
		return contracts.WorkflowDispatchResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return contracts.WorkflowDispatchResult{}, err
	}

	return contracts.WorkflowDispatchResult{
		SessionID:     sessionID,
		Notifications: notifications,
		Executions:    immediateExecutions,
	}, nil
}

func (s *Store) CreateCapabilityApproval(ctx context.Context, req contracts.ApprovedCapabilityRequest) (contracts.ExecutionDetail, []contracts.ChannelMessage, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contracts.ExecutionDetail{}, nil, err
	}
	defer tx.Rollback()

	sessionDetail, _, err := s.loadSessionForMutation(ctx, tx, req.SessionID)
	if err != nil {
		return contracts.ExecutionDetail{}, nil, err
	}
	now := time.Now().UTC()
	route := approvalrouting.Route{}
	if s.opts.ApprovalEnabled {
		route = resolveApprovalRoute(s.opts.ApprovalRouter, sessionDetail.Alert, fallbackString(req.RequestedBy, "tars"))
	}
	approvalID := strings.TrimSpace(req.ApprovalID)
	if approvalID == "" {
		approvalID = randomUUID()
	}
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return contracts.ExecutionDetail{}, nil, err
	}
	runtime := contracts.CloneRuntimeMetadata(req.Runtime)
	detail := contracts.ExecutionDetail{
		ExecutionID:      approvalID,
		RequestKind:      "capability",
		Status:           "pending",
		RiskLevel:        riskLevelForSeverity(stringFromAlert(sessionDetail.Alert, "severity")),
		StepID:           strings.TrimSpace(req.StepID),
		CapabilityID:     strings.TrimSpace(req.CapabilityID),
		CapabilityParams: cloneInterfaceMap(req.Params),
		ConnectorID:      strings.TrimSpace(req.ConnectorID),
		ConnectorType:    runtimeField(runtime, "connector_type"),
		ConnectorVendor:  runtimeField(runtime, "connector_vendor"),
		Protocol:         runtimeField(runtime, "protocol"),
		ExecutionMode:    runtimeField(runtime, "execution_mode"),
		RequestedBy:      fallbackString(req.RequestedBy, "tars"),
		ApprovalGroup:    route.GroupKey,
		Runtime:          runtime,
		CreatedAt:        now,
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO execution_requests (
			id, tenant_id, session_id, target_host, command, command_source, risk_level, requested_by, approval_group, status, timeout_seconds, request_kind, step_id, capability_id, capability_params, connector_id, connector_type, connector_vendor, protocol, execution_mode, created_at
		) VALUES ($1, 'default', $2, '', '', 'tool_plan_capability', $3, $4, $5, 'pending', 300, 'capability', $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, detail.ExecutionID, req.SessionID, detail.RiskLevel, detail.RequestedBy, detail.ApprovalGroup, nullableString(detail.StepID), nullableString(detail.CapabilityID), nullableJSON(paramsJSON), nullableString(detail.ConnectorID), nullableString(detail.ConnectorType), nullableString(detail.ConnectorVendor), nullableString(detail.Protocol), nullableString(detail.ExecutionMode), now); err != nil {
		return contracts.ExecutionDetail{}, nil, err
	}
	if err := s.updateSessionStatus(ctx, tx, req.SessionID, "pending_approval", now, false); err != nil {
		return contracts.ExecutionDetail{}, nil, err
	}
	if err := s.insertSessionEvent(ctx, tx, req.SessionID, "capability_approval_requested", fmt.Sprintf("capability approval %s created connector=%s capability=%s", detail.ExecutionID, detail.ConnectorID, detail.CapabilityID), now); err != nil {
		return contracts.ExecutionDetail{}, nil, err
	}
	messages := buildCapabilityApprovalMessages(sessionDetail, detail, route, s.opts.ApprovalTimeout)
	for _, message := range messages {
		if err := s.insertSessionEvent(ctx, tx, req.SessionID, "approval_message_prepared", fmt.Sprintf("target=%s body=%s", message.Target, compactSnippet(message.Body)), now); err != nil {
			return contracts.ExecutionDetail{}, nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return contracts.ExecutionDetail{}, nil, err
	}
	return detail, messages, nil
}

func rehydrateDiagnosisOutput(diagnosis contracts.DiagnosisOutput, cfg *reasoning.DesensitizationConfig) contracts.DiagnosisOutput {
	if len(diagnosis.DesenseMap) == 0 {
		return diagnosis
	}
	diagnosis.Summary = rehydratePlaceholders(diagnosis.Summary, diagnosis.DesenseMap, cfg)
	diagnosis.ExecutionHint = rehydratePlaceholders(diagnosis.ExecutionHint, diagnosis.DesenseMap, cfg)
	return diagnosis
}

func rehydratePlaceholders(input string, mapping map[string]string, cfg *reasoning.DesensitizationConfig) string {
	if input == "" || len(mapping) == 0 {
		return input
	}
	if cfg == nil {
		defaultCfg := reasoning.DefaultDesensitizationConfig()
		cfg = &defaultCfg
	}

	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		switch {
		case strings.HasPrefix(key, "[HOST_") && !cfg.Rehydration.Host:
			continue
		case strings.HasPrefix(key, "[IP_") && !cfg.Rehydration.IP:
			continue
		case strings.HasPrefix(key, "[PATH_") && !cfg.Rehydration.Path:
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j])
	})

	output := input
	for _, placeholder := range keys {
		output = strings.ReplaceAll(output, placeholder, mapping[placeholder])
	}
	return output
}

func (s *Store) currentDesensitizationConfig() *reasoning.DesensitizationConfig {
	if s == nil || s.opts.DesensitizationProvider == nil {
		cfg := reasoning.DefaultDesensitizationConfig()
		return &cfg
	}
	return s.opts.DesensitizationProvider.CurrentDesensitizationConfig()
}

func (s *Store) CompleteOutbox(ctx context.Context, eventID string) error {
	return s.ResolveEvent(ctx, eventID, contracts.DeliveryResult{Decision: contracts.DeliveryDecisionAck})
}

func (s *Store) MarkOutboxFailed(ctx context.Context, eventID string, lastError string) error {
	var topic string
	var retryCount int
	err := s.db.QueryRowContext(ctx, `
		SELECT topic, retry_count
		FROM outbox_events
		WHERE id = $1
	`, eventID).Scan(&topic, &retryCount)
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.ErrNotFound
	}
	if err != nil {
		return err
	}
	decision := contracts.DefaultDeliveryPolicy(topic).Decide(retryCount+1, fmt.Errorf("%s", strings.TrimSpace(lastError)))
	return s.ResolveEvent(ctx, eventID, decision)
}

func (s *Store) ResolveEvent(ctx context.Context, eventID string, result contracts.DeliveryResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var aggregateID string
	var topic string
	var retryCount int
	err = tx.QueryRowContext(ctx, `
		SELECT aggregate_id, topic, retry_count
		FROM outbox_events
		WHERE id = $1
		FOR UPDATE
	`, eventID).Scan(&aggregateID, &topic, &retryCount)
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.ErrNotFound
	}
	if err != nil {
		return err
	}

	nextRetryCount := retryCount
	now := time.Now().UTC()
	result.LastError = strings.TrimSpace(result.LastError)
	switch result.Decision {
	case contracts.DeliveryDecisionAck:
		updateResult, err := tx.ExecContext(ctx, `
			UPDATE outbox_events
			SET status = 'done', blocked_reason = NULL, last_error = NULL
			WHERE id = $1
		`, eventID)
		if err != nil {
			return err
		}
		affected, _ := updateResult.RowsAffected()
		if affected == 0 {
			return contracts.ErrNotFound
		}
		return tx.Commit()
	case contracts.DeliveryDecisionRetry:
		nextRetryCount++
		if _, err := tx.ExecContext(ctx, `
			UPDATE outbox_events
			SET status = 'pending',
			    last_error = $2,
			    retry_count = $3,
			    available_at = $4
			WHERE id = $1
		`, eventID, result.LastError, nextRetryCount, now.Add(result.Delay)); err != nil {
			return err
		}
		message := fmt.Sprintf("event retry scheduled after delivery failure: %s", result.LastError)
		if strings.TrimSpace(result.Reason) != "" {
			message = fmt.Sprintf("%s (%s)", message, result.Reason)
		}
		if err := s.insertSessionEvent(ctx, tx, aggregateID, "outbox_retry_scheduled", message, now); err != nil {
			return err
		}
		return tx.Commit()
	case contracts.DeliveryDecisionDeadLetter:
		nextRetryCount++
		err = tx.QueryRowContext(ctx, `
			UPDATE outbox_events
			SET status = 'failed', last_error = $2, retry_count = $3
			WHERE id = $1
			RETURNING aggregate_id
		`, eventID, result.LastError, nextRetryCount).Scan(&aggregateID)
		if err != nil {
			return err
		}
		message := result.LastError
		if strings.TrimSpace(result.Reason) != "" {
			message = fmt.Sprintf("%s (%s)", message, result.Reason)
		}
		if err := s.insertSessionEvent(ctx, tx, aggregateID, "outbox_failed", message, now); err != nil {
			return err
		}
		return tx.Commit()
	default:
		return fmt.Errorf("unsupported delivery decision: %s", result.Decision)
	}
}

func (s *Store) loadSessionDetail(ctx context.Context, q queryer, sessionID string) (contracts.SessionDetail, error) {
	var (
		status           string
		diagnosisSummary sql.NullString
		toolPlanJSON     []byte
		attachmentsJSON  []byte
		verificationJSON []byte
		source           string
		severity         string
		fingerprint      string
		targetHost       sql.NullString
		agentRoleID      sql.NullString
		labelsJSON       []byte
		annotationsJSON  []byte
		receivedAt       time.Time
	)
	err := q.QueryRowContext(ctx, `
		SELECT s.status, s.diagnosis_summary, COALESCE(s.tool_plan, '[]'::jsonb), COALESCE(s.attachments, '[]'::jsonb), COALESCE(s.verification_result, '{}'::jsonb), e.source, e.severity, e.fingerprint, s.target_host, COALESCE(s.agent_role_id, ''), e.labels, e.annotations, e.received_at
		FROM alert_sessions s
		JOIN alert_events e ON e.id = s.alert_event_id
		WHERE s.id = $1
	`, sessionID).Scan(&status, &diagnosisSummary, &toolPlanJSON, &attachmentsJSON, &verificationJSON, &source, &severity, &fingerprint, &targetHost, &agentRoleID, &labelsJSON, &annotationsJSON, &receivedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.SessionDetail{}, contracts.ErrNotFound
	}
	if err != nil {
		return contracts.SessionDetail{}, err
	}

	labels, err := unmarshalStringMap(labelsJSON)
	if err != nil {
		return contracts.SessionDetail{}, err
	}
	annotations, err := unmarshalStringMap(annotationsJSON)
	if err != nil {
		return contracts.SessionDetail{}, err
	}

	detail := contracts.SessionDetail{
		SessionID:        sessionID,
		AgentRoleID:      agentRoleID.String,
		Status:           status,
		DiagnosisSummary: diagnosisSummary.String,
		Alert: map[string]interface{}{
			"source":      source,
			"severity":    severity,
			"fingerprint": fingerprint,
			"host":        targetHost.String,
			"labels":      labels,
			"annotations": annotations,
			"received_at": receivedAt,
		},
		Executions: []contracts.ExecutionDetail{},
		Timeline:   []contracts.TimelineEvent{},
	}
	if len(toolPlanJSON) > 0 {
		if err := json.Unmarshal(toolPlanJSON, &detail.ToolPlan); err != nil {
			return contracts.SessionDetail{}, err
		}
	}
	if len(attachmentsJSON) > 0 {
		if err := json.Unmarshal(attachmentsJSON, &detail.Attachments); err != nil {
			return contracts.SessionDetail{}, err
		}
	}
	if verification, err := unmarshalVerification(verificationJSON); err != nil {
		return contracts.SessionDetail{}, err
	} else {
		detail.Verification = verification
	}

	executionRows, err := q.QueryContext(ctx, `
		SELECT id, COALESCE(request_kind, 'execution'), status, risk_level, command, target_host, COALESCE(step_id, ''), COALESCE(capability_id, ''), COALESCE(capability_params, '{}'::jsonb), COALESCE(connector_id, ''), COALESCE(connector_type, ''), COALESCE(connector_vendor, ''), COALESCE(protocol, ''), COALESCE(execution_mode, ''), requested_by, COALESCE(approval_group, ''), exit_code, COALESCE(output_ref, ''), output_bytes, output_truncated, created_at, approved_at, completed_at
		FROM execution_requests
		WHERE session_id = $1
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return contracts.SessionDetail{}, err
	}
	defer executionRows.Close()

	for executionRows.Next() {
		var item contracts.ExecutionDetail
		var approvedAt sql.NullTime
		var completedAt sql.NullTime
		var capabilityParamsJSON []byte
		if err := executionRows.Scan(
			&item.ExecutionID,
			&item.RequestKind,
			&item.Status,
			&item.RiskLevel,
			&item.Command,
			&item.TargetHost,
			&item.StepID,
			&item.CapabilityID,
			&capabilityParamsJSON,
			&item.ConnectorID,
			&item.ConnectorType,
			&item.ConnectorVendor,
			&item.Protocol,
			&item.ExecutionMode,
			&item.RequestedBy,
			&item.ApprovalGroup,
			&item.ExitCode,
			&item.OutputRef,
			&item.OutputBytes,
			&item.OutputTruncated,
			&item.CreatedAt,
			&approvedAt,
			&completedAt,
		); err != nil {
			return contracts.SessionDetail{}, err
		}
		if approvedAt.Valid {
			item.ApprovedAt = approvedAt.Time
		}
		if completedAt.Valid {
			item.CompletedAt = completedAt.Time
		}
		if len(capabilityParamsJSON) > 0 {
			if err := json.Unmarshal(capabilityParamsJSON, &item.CapabilityParams); err != nil {
				return contracts.SessionDetail{}, err
			}
		}
		item.Runtime = inferExecutionRuntimeMetadata(item)
		item.SessionID = sessionID
		detail.Executions = append(detail.Executions, item)
	}
	if err := executionRows.Err(); err != nil {
		return contracts.SessionDetail{}, err
	}

	eventRows, err := q.QueryContext(ctx, `
		SELECT event_type, payload, created_at
		FROM session_events
		WHERE session_id = $1
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return contracts.SessionDetail{}, err
	}
	defer eventRows.Close()

	for eventRows.Next() {
		var eventType string
		var payloadJSON []byte
		var createdAt time.Time
		if err := eventRows.Scan(&eventType, &payloadJSON, &createdAt); err != nil {
			return contracts.SessionDetail{}, err
		}
		message, err := eventMessage(payloadJSON)
		if err != nil {
			return contracts.SessionDetail{}, err
		}
		detail.Timeline = append(detail.Timeline, contracts.TimelineEvent{
			Event:     eventType,
			Message:   message,
			CreatedAt: createdAt,
		})
	}
	if err := eventRows.Err(); err != nil {
		return contracts.SessionDetail{}, err
	}
	contracts.PopulateSessionGoldenPath(&detail)
	return detail, nil
}

func (s *Store) loadExecutionDetail(ctx context.Context, q queryer, executionID string) (contracts.ExecutionDetail, error) {
	var item contracts.ExecutionDetail
	var approvedAt sql.NullTime
	var completedAt sql.NullTime
	var capabilityParamsJSON []byte
	err := q.QueryRowContext(ctx, `
		SELECT id, COALESCE(request_kind, 'execution'), status, risk_level, command, target_host, COALESCE(step_id, ''), COALESCE(capability_id, ''), COALESCE(capability_params, '{}'::jsonb), COALESCE(connector_id, ''), COALESCE(connector_type, ''), COALESCE(connector_vendor, ''), COALESCE(protocol, ''), COALESCE(execution_mode, ''), requested_by, COALESCE(approval_group, ''), exit_code, COALESCE(output_ref, ''), output_bytes, output_truncated, created_at, approved_at, completed_at
		FROM execution_requests
		WHERE id = $1
	`, executionID).Scan(
		&item.ExecutionID,
		&item.RequestKind,
		&item.Status,
		&item.RiskLevel,
		&item.Command,
		&item.TargetHost,
		&item.StepID,
		&item.CapabilityID,
		&capabilityParamsJSON,
		&item.ConnectorID,
		&item.ConnectorType,
		&item.ConnectorVendor,
		&item.Protocol,
		&item.ExecutionMode,
		&item.RequestedBy,
		&item.ApprovalGroup,
		&item.ExitCode,
		&item.OutputRef,
		&item.OutputBytes,
		&item.OutputTruncated,
		&item.CreatedAt,
		&approvedAt,
		&completedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.ExecutionDetail{}, contracts.ErrNotFound
	}
	if err != nil {
		return contracts.ExecutionDetail{}, err
	}
	if approvedAt.Valid {
		item.ApprovedAt = approvedAt.Time
	}
	if completedAt.Valid {
		item.CompletedAt = completedAt.Time
	}
	if len(capabilityParamsJSON) > 0 {
		if err := json.Unmarshal(capabilityParamsJSON, &item.CapabilityParams); err != nil {
			return contracts.ExecutionDetail{}, err
		}
	}
	item.Runtime = inferExecutionRuntimeMetadata(item)
	if err := q.QueryRowContext(ctx, `SELECT session_id FROM execution_requests WHERE id = $1`, executionID).Scan(&item.SessionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return contracts.ExecutionDetail{}, contracts.ErrNotFound
		}
		return contracts.ExecutionDetail{}, err
	}
	var sessionDetail *contracts.SessionDetail
	if item.SessionID != "" {
		detail, err := s.loadSessionDetail(ctx, q, item.SessionID)
		if err != nil && !errors.Is(err, contracts.ErrNotFound) {
			return contracts.ExecutionDetail{}, err
		}
		if err == nil {
			sessionDetail = &detail
		}
	}
	contracts.PopulateExecutionGoldenPath(&item, sessionDetail)
	return item, nil
}

func (s *Store) loadSessionForMutation(ctx context.Context, q queryer, sessionID string) (contracts.SessionDetail, string, error) {
	detail, err := s.loadSessionDetail(ctx, q, sessionID)
	if err != nil {
		return contracts.SessionDetail{}, "", err
	}
	severity, _ := detail.Alert["severity"].(string)
	return detail, severity, nil
}

func (s *Store) lockExecution(ctx context.Context, tx *sql.Tx, executionID string) (contracts.ExecutionDetail, string, error) {
	var item contracts.ExecutionDetail
	var sessionID string
	var approvedAt sql.NullTime
	var completedAt sql.NullTime
	var capabilityParamsJSON []byte
	err := tx.QueryRowContext(ctx, `
		SELECT id, session_id, COALESCE(request_kind, 'execution'), status, risk_level, command, target_host, COALESCE(step_id, ''), COALESCE(capability_id, ''), COALESCE(capability_params, '{}'::jsonb), COALESCE(connector_id, ''), COALESCE(connector_type, ''), COALESCE(connector_vendor, ''), COALESCE(protocol, ''), COALESCE(execution_mode, ''), requested_by, COALESCE(approval_group, ''), exit_code, COALESCE(output_ref, ''), output_bytes, output_truncated, created_at, approved_at, completed_at
		FROM execution_requests
		WHERE id = $1
		FOR UPDATE
	`, executionID).Scan(
		&item.ExecutionID,
		&sessionID,
		&item.RequestKind,
		&item.Status,
		&item.RiskLevel,
		&item.Command,
		&item.TargetHost,
		&item.StepID,
		&item.CapabilityID,
		&capabilityParamsJSON,
		&item.ConnectorID,
		&item.ConnectorType,
		&item.ConnectorVendor,
		&item.Protocol,
		&item.ExecutionMode,
		&item.RequestedBy,
		&item.ApprovalGroup,
		&item.ExitCode,
		&item.OutputRef,
		&item.OutputBytes,
		&item.OutputTruncated,
		&item.CreatedAt,
		&approvedAt,
		&completedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return contracts.ExecutionDetail{}, "", contracts.ErrNotFound
	}
	if err != nil {
		return contracts.ExecutionDetail{}, "", err
	}
	if approvedAt.Valid {
		item.ApprovedAt = approvedAt.Time
	}
	if completedAt.Valid {
		item.CompletedAt = completedAt.Time
	}
	if len(capabilityParamsJSON) > 0 {
		if err := json.Unmarshal(capabilityParamsJSON, &item.CapabilityParams); err != nil {
			return contracts.ExecutionDetail{}, "", err
		}
	}
	item.Runtime = inferExecutionRuntimeMetadata(item)
	return item, sessionID, nil
}

func (s *Store) lockSession(ctx context.Context, tx *sql.Tx, sessionID string) (string, error) {
	var status string
	err := tx.QueryRowContext(ctx, `
		SELECT status
		FROM alert_sessions
		WHERE id = $1
		FOR UPDATE
	`, sessionID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", contracts.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return status, nil
}

func (s *Store) updateSessionStatus(ctx context.Context, tx *sql.Tx, sessionID string, status string, now time.Time, resolved bool) error {
	if resolved {
		_, err := tx.ExecContext(ctx, `
			UPDATE alert_sessions
			SET status = $2, resolved_at = $3, updated_at = $3, version = version + 1
			WHERE id = $1
		`, sessionID, status, now)
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE alert_sessions
		SET status = $2, updated_at = $3, version = version + 1
		WHERE id = $1
	`, sessionID, status, now)
	return err
}

func (s *Store) enqueueSessionClosed(ctx context.Context, tx *sql.Tx, sessionID string, now time.Time) error {
	status := "pending"
	blockedReason := ""
	if !s.opts.KnowledgeIngestEnabled {
		status = "blocked"
		blockedReason = "knowledge_ingest_disabled"
	}
	_, err := s.publishEventTx(ctx, tx, contracts.EventPublishRequest{
		Topic:         "session.closed",
		AggregateID:   sessionID,
		Payload:       []byte(`{}`),
		Status:        status,
		BlockedReason: blockedReason,
		CreatedAt:     now,
		AvailableAt:   now,
	})
	return err
}

func (s *Store) PublishEvent(ctx context.Context, event contracts.EventPublishRequest) (contracts.EventEnvelope, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return contracts.EventEnvelope{}, err
	}
	defer tx.Rollback()
	envelope, err := s.publishEventTx(ctx, tx, event)
	if err != nil {
		return contracts.EventEnvelope{}, err
	}
	if err := tx.Commit(); err != nil {
		return contracts.EventEnvelope{}, err
	}
	return envelope, nil
}

func (s *Store) RecoverPendingEvents(ctx context.Context) (int, error) {
	return s.RecoverProcessingOutbox(ctx)
}

func (s *Store) publishEventTx(ctx context.Context, tx *sql.Tx, event contracts.EventPublishRequest) (contracts.EventEnvelope, error) {
	now := event.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	status := strings.TrimSpace(event.Status)
	if status == "" {
		status = "pending"
	}
	availableAt := event.AvailableAt
	if availableAt.IsZero() && status == "pending" {
		availableAt = now
	}
	id := randomUUID()
	_, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (
			id, topic, aggregate_id, payload, status, available_at, retry_count, blocked_reason, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8)
	`, id, strings.TrimSpace(event.Topic), strings.TrimSpace(event.AggregateID), event.Payload, status, availableAt, nullableString(strings.TrimSpace(event.BlockedReason)), now)
	if err != nil {
		return contracts.EventEnvelope{}, err
	}
	return contracts.EventEnvelope{
		EventID:       id,
		Topic:         strings.TrimSpace(event.Topic),
		AggregateID:   strings.TrimSpace(event.AggregateID),
		Payload:       append([]byte(nil), event.Payload...),
		Headers:       cloneStringMapLocal(event.Headers),
		Metadata:      event.Metadata,
		Attempt:       1,
		Status:        status,
		BlockedReason: strings.TrimSpace(event.BlockedReason),
		AvailableAt:   availableAt,
		CreatedAt:     now,
	}, nil
}

func toDispatchableOutboxEvent(event contracts.EventEnvelope) contracts.DispatchableOutboxEvent {
	return contracts.DispatchableOutboxEvent{
		EventID:     event.EventID,
		Topic:       event.Topic,
		AggregateID: event.AggregateID,
		Headers:     cloneStringMapLocal(event.Headers),
		Metadata:    event.Metadata,
		Attempt:     event.Attempt,
		Status:      event.Status,
		LastError:   event.LastError,
		AvailableAt: event.AvailableAt,
		CreatedAt:   event.CreatedAt,
		Payload:     append([]byte(nil), event.Payload...),
	}
}

func cloneStringMapLocal(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (s *Store) ensureIdempotency(ctx context.Context, tx *sql.Tx, scope string, key string, requestHash string, now time.Time) (bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO idempotency_keys (
			id, scope, idempotency_key, request_hash, status, first_seen_at, last_seen_at, expires_at
		) VALUES ($1, $2, $3, $4, 'accepted', $5, $5, $6)
		ON CONFLICT (scope, idempotency_key) DO NOTHING
	`, randomUUID(), scope, key, requestHash, now, now.Add(24*time.Hour))
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	if affected > 0 {
		return false, nil
	}

	var existingHash string
	err = tx.QueryRowContext(ctx, `
		SELECT request_hash
		FROM idempotency_keys
		WHERE scope = $1 AND idempotency_key = $2
	`, scope, key).Scan(&existingHash)
	if err != nil {
		return false, err
	}
	if existingHash == requestHash {
		return true, nil
	}
	return false, fmt.Errorf("idempotency conflict for %s/%s", scope, key)
}

func (s *Store) DeleteExpiredIdempotencyKeys(ctx context.Context, now time.Time) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM idempotency_keys
		WHERE expires_at < $1
	`, now)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (s *Store) DeleteExpiredExecutionOutputChunks(ctx context.Context, now time.Time) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM execution_output_chunks
		WHERE retention_until < $1
	`, now)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (s *Store) insertApprovalRecord(ctx context.Context, tx *sql.Tx, executionID string, action string, actorID string, originalCommand string, finalCommand string, createdAt time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO execution_approvals (
			id, execution_request_id, action, actor_id, original_command, final_command, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, randomUUID(), executionID, action, actorID, nullableString(originalCommand), nullableString(finalCommand), createdAt)
	return err
}

func (s *Store) insertSessionEvent(ctx context.Context, tx *sql.Tx, sessionID string, eventType string, message string, createdAt time.Time) error {
	payload, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO session_events (id, session_id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, randomUUID(), sessionID, eventType, payload, createdAt)
	return err
}

func (s *Store) replaceExecutionOutputChunks(ctx context.Context, tx *sql.Tx, executionID string, preview string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM execution_output_chunks WHERE execution_request_id = $1`, executionID); err != nil {
		return err
	}
	if preview == "" {
		return nil
	}

	chunks := splitStringByBytes(preview, s.opts.OutputChunkBytes)
	retentionUntil := now.Add(s.opts.OutputRetention)
	for index, chunk := range chunks {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO execution_output_chunks (
				execution_request_id, seq, stream_type, content, byte_size, retention_until, created_at
			) VALUES ($1, $2, 'combined', $3, $4, $5, $6)
		`, executionID, index, chunk, len([]byte(chunk)), retentionUntil, now); err != nil {
			return err
		}
	}
	return nil
}

func nullableString(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func unmarshalStringMap(input []byte) (map[string]string, error) {
	if len(input) == 0 {
		return map[string]string{}, nil
	}
	var out map[string]string
	if err := json.Unmarshal(input, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}

func unmarshalVerification(input []byte) (*contracts.SessionVerification, error) {
	if len(input) == 0 || string(input) == "{}" {
		return nil, nil
	}
	var payload struct {
		Status    string                     `json:"status"`
		Summary   string                     `json:"summary"`
		Details   map[string]interface{}     `json:"details"`
		Runtime   *contracts.RuntimeMetadata `json:"runtime"`
		CheckedAt time.Time                  `json:"checked_at"`
	}
	if err := json.Unmarshal(input, &payload); err != nil {
		return nil, err
	}
	if payload.Status == "" && payload.Summary == "" && len(payload.Details) == 0 && payload.CheckedAt.IsZero() {
		return nil, nil
	}
	return &contracts.SessionVerification{
		Status:    payload.Status,
		Summary:   payload.Summary,
		Details:   payload.Details,
		Runtime:   contracts.CloneRuntimeMetadata(payload.Runtime),
		CheckedAt: payload.CheckedAt,
	}, nil
}

func eventMessage(payloadJSON []byte) (string, error) {
	if len(payloadJSON) == 0 {
		return "", nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return "", err
	}
	if message, ok := payload["message"].(string); ok {
		return message, nil
	}
	return "", nil
}

func postgresExecutionRuntimeSummary(execution contracts.ExecutionDetail) string {
	return fmt.Sprintf(
		"runtime=%s connector=%s protocol=%s mode=%s fallback_used=%t fallback_target=%s",
		fallbackString(runtimeMetadataField(execution.Runtime, "runtime"), "n/a"),
		fallbackString(execution.ConnectorID, "n/a"),
		fallbackString(execution.Protocol, "n/a"),
		fallbackString(execution.ExecutionMode, "n/a"),
		runtimeMetadataFallbackUsed(execution.Runtime),
		fallbackString(runtimeMetadataField(execution.Runtime, "fallback_target"), "n/a"),
	)
}

func inferExecutionRuntimeMetadata(item contracts.ExecutionDetail) *contracts.RuntimeMetadata {
	runtime := "connector"
	fallbackUsed := false
	fallbackReason := ""
	fallbackTarget := "ssh"
	if strings.TrimSpace(item.ConnectorID) == "" && strings.EqualFold(strings.TrimSpace(item.Protocol), "ssh") {
		runtime = "ssh"
		fallbackUsed = true
		fallbackReason = "no_healthy_connector_selected"
		fallbackTarget = "ssh"
	}
	selection := "auto_selector"
	if strings.TrimSpace(item.ConnectorID) != "" {
		selection = "auto_selector"
	}
	return &contracts.RuntimeMetadata{
		Runtime:         runtime,
		Selection:       selection,
		ConnectorID:     item.ConnectorID,
		ConnectorType:   item.ConnectorType,
		ConnectorVendor: item.ConnectorVendor,
		Protocol:        item.Protocol,
		ExecutionMode:   item.ExecutionMode,
		FallbackEnabled: true,
		FallbackUsed:    fallbackUsed,
		FallbackReason:  fallbackReason,
		FallbackTarget:  fallbackTarget,
	}
}

func runtimeMetadataField(runtime *contracts.RuntimeMetadata, field string) string {
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

func runtimeMetadataFallbackUsed(runtime *contracts.RuntimeMetadata) bool {
	return runtime != nil && runtime.FallbackUsed
}

func runtimeField(runtime *contracts.RuntimeMetadata, field string) string {
	if runtime == nil {
		return ""
	}
	switch field {
	case "connector_type":
		return runtime.ConnectorType
	case "connector_vendor":
		return runtime.ConnectorVendor
	case "protocol":
		return runtime.Protocol
	case "execution_mode":
		return runtime.ExecutionMode
	default:
		return ""
	}
}

func randomUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

func randomID(prefix string) string {
	return prefix + "-" + randomUUID()
}

type executionRuntimeSelection struct {
	ConnectorID     string
	ConnectorType   string
	ConnectorVendor string
	Protocol        string
	ExecutionMode   string
	Runtime         *contracts.RuntimeMetadata
}

func selectExecutionRuntime(manager *connectors.Manager) executionRuntimeSelection {
	selection := executionRuntimeSelection{
		Protocol:      "ssh",
		ExecutionMode: "ssh",
		Runtime: &contracts.RuntimeMetadata{
			Runtime:         "ssh",
			Selection:       "auto_selector",
			Protocol:        "ssh",
			ExecutionMode:   "ssh",
			FallbackEnabled: true,
			FallbackUsed:    true,
			FallbackReason:  "no_healthy_connector_selected",
			FallbackTarget:  "ssh",
		},
	}
	entry, ok := connectors.SelectHealthyRuntimeManifest(manager, "execution", "", map[string]struct{}{"jumpserver_api": {}})
	if !ok {
		return selection
	}
	selection.ConnectorID = entry.Metadata.ID
	selection.ConnectorType = entry.Spec.Type
	selection.ConnectorVendor = entry.Metadata.Vendor
	selection.Protocol = entry.Spec.Protocol
	selection.ExecutionMode = connectors.DefaultExecutionMode(entry.Spec.Protocol)
	selection.Runtime = &contracts.RuntimeMetadata{
		Runtime:         "connector",
		Selection:       "auto_selector",
		ConnectorID:     entry.Metadata.ID,
		ConnectorType:   entry.Spec.Type,
		ConnectorVendor: entry.Metadata.Vendor,
		Protocol:        entry.Spec.Protocol,
		ExecutionMode:   selection.ExecutionMode,
		FallbackEnabled: true,
		FallbackUsed:    false,
		FallbackTarget:  "ssh",
	}
	return selection
}

func (s *Store) topicEnabled(topic string) bool {
	switch topic {
	case "session.analyze_requested":
		return s.opts.DiagnosisEnabled
	case "session.closed":
		return s.opts.KnowledgeIngestEnabled
	default:
		return true
	}
}

func pickHost(labels map[string]string) string {
	for _, key := range []string{"instance", "host", "node", "pod"} {
		if labels[key] != "" {
			return labels[key]
		}
	}
	return ""
}

func pickNotificationTarget(alert map[string]interface{}) string {
	for _, key := range []string{"telegram_target", "chat_id", "room"} {
		if value, ok := alert[key].(string); ok && value != "" {
			return value
		}
	}
	for _, key := range []string{"telegram_target", "chat_id", "room"} {
		if value := alertLabel(alert, key); strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "ops-room"
}

func buildCapabilityApprovalMessages(detail contracts.SessionDetail, execution contracts.ExecutionDetail, route approvalrouting.Route, timeout time.Duration) []contracts.ChannelMessage {
	targets := route.Targets
	if len(targets) == 0 {
		targets = []string{pickNotificationTarget(detail.Alert)}
	}
	items := make([]contracts.ChannelMessage, 0, len(targets))
	for _, target := range targets {
		items = append(items, contracts.ChannelMessage{
			Channel: notificationChannelForAlert(detail.Alert),
			Target:  notificationTargetForAlert(detail.Alert, target),
			Body:    formatCapabilityApprovalMessage(detail, execution, route, timeout),
			Actions: []contracts.ChannelAction{
				{Label: "批准能力", Value: fmt.Sprintf("approve:%s", execution.ExecutionID)},
				{Label: "拒绝", Value: fmt.Sprintf("reject:%s", execution.ExecutionID)},
				{Label: "补充信息", Value: fmt.Sprintf("request_context:%s", execution.ExecutionID)},
			},
		})
	}
	return items
}

func formatCapabilityApprovalMessage(detail contracts.SessionDetail, execution contracts.ExecutionDetail, route approvalrouting.Route, timeout time.Duration) string {
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	return fmt.Sprintf(
		"[TARS 能力审批]\nsession: %s\napproval: %s\nconnector: %s\ncapability: %s\nparams: %s\n原因: %s\n时限: %s\n来源: %s",
		detail.SessionID,
		execution.ExecutionID,
		fallbackString(execution.ConnectorID, "unknown-connector"),
		fallbackString(execution.CapabilityID, "unknown-capability"),
		compactSnippet(fmt.Sprintf("%v", execution.CapabilityParams)),
		fallbackString(detail.DiagnosisSummary, "tool plan requested approval"),
		timeout.Round(time.Second),
		route.SourceLabel,
	)
}

func resolveApprovalRoute(router approvalrouting.Router, alert map[string]interface{}, requester string) approvalrouting.Route {
	if alertLabel(alert, "tars_chat") == "true" {
		return approvalrouting.Route{
			GroupKey:    "chat_request:direct",
			SourceLabel: "chat request(direct)",
			Targets:     []string{pickNotificationTarget(alert)},
		}
	}
	if router == nil {
		return approvalrouting.Route{
			GroupKey:    "fallback:default",
			SourceLabel: "fallback(default)",
			Targets:     []string{pickNotificationTarget(alert)},
		}
	}
	return router.Resolve(alert, requester, pickNotificationTarget(alert))
}

func (s *Store) resolveAuthorizationDecision(alert map[string]interface{}, command string) authorization.Decision {
	if s.opts.AuthorizationPolicy == nil {
		return authorization.Decision{
			Action:    authorization.ActionRequireApproval,
			RuleID:    "mvp_default",
			MatchedBy: "default",
		}
	}
	decision := s.opts.AuthorizationPolicy.EvaluateSSHCommand(authorization.SSHCommandInput{
		Command: command,
		Service: alertLabel(alert, "service"),
		Host:    stringFromAlert(alert, "host"),
		Channel: stringFromAlert(alert, "source"),
	})
	// Apply agent role policy enforcement (take the more restrictive result).
	if s.opts.AgentRoleManager != nil {
		role := s.opts.AgentRoleManager.ResolveForSession("diagnosis")
		enforced := agentrole.EnforcePolicy(role, string(decision.Action), command)
		decision.Action = authorization.Action(enforced)
	}
	return decision
}

func (s *Store) resolveChatWithoutExecution(ctx context.Context, tx *sql.Tx, sessionID string, sessionDetail *contracts.SessionDetail, source string, now time.Time, message string) error {
	if sessionDetail.Status == "resolved" {
		return nil
	}
	switch source {
	case "telegram_chat", "web_chat", "ops_api":
		// chat-originated sessions should close after diagnosis when no execution is needed.
	default:
		if alertLabel(sessionDetail.Alert, "tars_generated") != "ops_setup" {
			return nil
		}
	}
	if err := s.updateSessionStatus(ctx, tx, sessionID, "resolved", now, true); err != nil {
		return err
	}
	sessionDetail.Status = "resolved"
	if err := s.insertSessionEvent(ctx, tx, sessionID, "chat_answer_completed", message, now); err != nil {
		return err
	}
	return s.enqueueSessionClosed(ctx, tx, sessionID, now)
}

func formatDiagnosisMessage(detail contracts.SessionDetail, diagnosis contracts.DiagnosisOutput) string {
	host := stringFromAlert(detail.Alert, "host")
	fingerprint := stringFromAlert(detail.Alert, "fingerprint")
	alertName := fallbackString(alertLabel(detail.Alert, "alertname"), "unknown-alert")
	service := alertLabel(detail.Alert, "service")
	severity := fallbackString(stringFromAlert(detail.Alert, "severity"), "info")
	source := fallbackString(stringFromAlert(detail.Alert, "source"), "vmalert")
	userRequest := annotationString(detail.Alert, "user_request")

	lines := []string{"[TARS] 诊断"}
	if source == "telegram_chat" && strings.TrimSpace(userRequest) != "" {
		lines = append(lines, fmt.Sprintf("请求: %s", compactSnippet(userRequest)))
		lines = append(lines, fmt.Sprintf("目标: %s", fallbackString(host, "unknown-host")))
	} else {
		lines = append(lines, fmt.Sprintf("告警: %s @ %s", alertName, fallbackString(host, "unknown-host")))
	}
	if service != "" || severity != "info" {
		context := []string{}
		if service != "" {
			context = append(context, fmt.Sprintf("服务 %s", service))
		}
		if severity != "info" {
			context = append(context, fmt.Sprintf("级别 %s", severity))
		}
		lines = append(lines, strings.Join(context, " · "))
	}
	lines = append(lines, fmt.Sprintf("结论: %s", diagnosis.Summary))
	if fingerprint != "" && source != "telegram_chat" {
		lines = append(lines, fmt.Sprintf("指纹: %s", fingerprint))
	}
	if diagnosis.ExecutionHint != "" {
		lines = append(lines, fmt.Sprintf("下一步: %s", diagnosis.ExecutionHint))
	}
	if len(diagnosis.Citations) > 0 {
		lines = append(lines, fmt.Sprintf("参考: %d 条知识", minInt(len(diagnosis.Citations), 3)))
	}
	lines = append(lines, fmt.Sprintf("会话: %s", detail.SessionID))
	return strings.Join(lines, "\n")
}

func diagnosisPrivacyNote() string {
	return "摘要已做脱敏处理；主机/IP/路径可能按策略回填展示，密码、token、API key 等敏感值仍显示为 [REDACTED]。"
}

func buildAuthorizationDecisionMessage(detail contracts.SessionDetail, command string, decision authorization.Decision, target string, summary string) contracts.ChannelMessage {
	lines := []string{
		"[TARS] 策略",
		fmt.Sprintf("结果: %s", authorizationActionLabel(decision.Action)),
	}
	if strings.TrimSpace(command) != "" {
		lines = append(lines, fmt.Sprintf("命令: %s", command))
	}
	if strings.TrimSpace(summary) != "" {
		lines = append(lines, fmt.Sprintf("说明: %s", summary))
	}
	if strings.TrimSpace(decision.RuleID) != "" {
		lines = append(lines, fmt.Sprintf("规则: %s", decision.RuleID))
	}
	lines = append(lines, fmt.Sprintf("会话: %s", detail.SessionID))
	return contracts.ChannelMessage{
		Channel: notificationChannelForAlert(detail.Alert),
		Target:  notificationTargetForAlert(detail.Alert, target),
		Body:    strings.Join(lines, "\n"),
	}
}

func buildApprovalMessages(detail contracts.SessionDetail, execution contracts.ExecutionDetail, route approvalrouting.Route, timeout time.Duration) []contracts.ChannelMessage {
	targets := route.Targets
	if len(targets) == 0 {
		targets = []string{pickNotificationTarget(detail.Alert)}
	}

	items := make([]contracts.ChannelMessage, 0, len(targets))
	for _, target := range targets {
		items = append(items, contracts.ChannelMessage{
			Channel: notificationChannelForAlert(detail.Alert),
			Target:  notificationTargetForAlert(detail.Alert, target),
			Body:    formatApprovalMessage(detail, execution, route, timeout),
			Actions: []contracts.ChannelAction{
				{Label: "批准执行", Value: fmt.Sprintf("approve:%s", execution.ExecutionID)},
				{Label: "拒绝", Value: fmt.Sprintf("reject:%s", execution.ExecutionID)},
				{Label: "要求补充信息", Value: fmt.Sprintf("request_context:%s", execution.ExecutionID)},
			},
		})
	}
	return items
}

func selectDiagnosisNotificationTarget(alert map[string]interface{}, route approvalrouting.Route) string {
	if target := pickNotificationTarget(alert); strings.TrimSpace(target) != "" && target != "ops-room" {
		return target
	}
	if len(route.Targets) > 0 && strings.TrimSpace(route.Targets[0]) != "" {
		return strings.TrimSpace(route.Targets[0])
	}
	return pickNotificationTarget(alert)
}

func notificationChannelForAlert(alert map[string]interface{}) string {
	switch strings.TrimSpace(stringFromAlert(alert, "source")) {
	case "web_chat", "ops_api":
		return "in_app_inbox"
	default:
		return "telegram"
	}
}

func notificationTargetForAlert(alert map[string]interface{}, fallbackTarget string) string {
	target := strings.TrimSpace(pickNotificationTarget(alert))
	switch strings.TrimSpace(stringFromAlert(alert, "source")) {
	case "telegram_chat", "web_chat", "ops_api":
		if target != "" {
			return target
		}
	}
	if fallback := strings.TrimSpace(fallbackTarget); fallback != "" {
		return fallback
	}
	return target
}

func formatApprovalMessage(detail contracts.SessionDetail, execution contracts.ExecutionDetail, route approvalrouting.Route, timeout time.Duration) string {
	_ = route
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	if stringFromAlert(detail.Alert, "source") == "telegram_chat" && strings.TrimSpace(annotationString(detail.Alert, "user_request")) != "" {
		return fmt.Sprintf(
			"[TARS] 待审批\n请求: %s\n目标: %s\n风险: %s\n命令: %s\n原因: %s\n时限: %s\n会话: %s",
			compactSnippet(annotationString(detail.Alert, "user_request")),
			fallbackString(stringFromAlert(detail.Alert, "host"), "unknown-host"),
			execution.RiskLevel,
			execution.Command,
			compactSnippet(fallbackString(detail.DiagnosisSummary, "diagnosis pending")),
			formatApprovalTimeout(timeout),
			detail.SessionID,
		)
	}
	return fmt.Sprintf(
		"[TARS] 待审批\n告警: %s @ %s\n风险: %s\n命令: %s\n原因: %s\n时限: %s\n会话: %s",
		fallbackString(alertLabel(detail.Alert, "alertname"), "unknown-alert"),
		fallbackString(stringFromAlert(detail.Alert, "host"), "unknown-host"),
		execution.RiskLevel,
		execution.Command,
		compactSnippet(fallbackString(detail.DiagnosisSummary, "diagnosis pending")),
		formatApprovalTimeout(timeout),
		detail.SessionID,
	)
}

func authorizationActionLabel(action authorization.Action) string {
	switch action {
	case authorization.ActionDirectExecute:
		return "直接执行"
	case authorization.ActionRequireApproval:
		return "需要审批"
	case authorization.ActionSuggestOnly:
		return "仅建议"
	case authorization.ActionDeny:
		return "已拒绝"
	default:
		return string(action)
	}
}

func formatApprovalTimeout(timeout time.Duration) string {
	if timeout <= 0 {
		return "15m"
	}
	if timeout%time.Minute == 0 {
		return timeout.Round(time.Minute).String()
	}
	return timeout.Round(time.Second).String()
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func stringFromAlert(alert map[string]interface{}, key string) string {
	if value, ok := alert[key].(string); ok {
		return value
	}
	return ""
}

func alertLabel(alert map[string]interface{}, key string) string {
	labels, ok := alert["labels"]
	if !ok {
		return ""
	}

	switch typed := labels.(type) {
	case map[string]string:
		return typed[key]
	case map[string]interface{}:
		if value, ok := typed[key].(string); ok {
			return value
		}
	}
	return ""
}

func annotationString(alert map[string]interface{}, key string) string {
	annotations, ok := alert["annotations"]
	if !ok {
		return ""
	}

	switch typed := annotations.(type) {
	case map[string]string:
		return typed[key]
	case map[string]interface{}:
		if value, ok := typed[key].(string); ok {
			return value
		}
	}
	return ""
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func compactSnippet(value string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len(trimmed) <= 96 {
		return trimmed
	}
	return trimmed[:93] + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func updateSessionToolPlanStep(detail *contracts.SessionDetail, stepID string, mutate func(step *contracts.ToolPlanStep)) {
	if detail == nil || strings.TrimSpace(stepID) == "" || mutate == nil {
		return
	}
	for idx := range detail.ToolPlan {
		if strings.TrimSpace(detail.ToolPlan[idx].ID) != strings.TrimSpace(stepID) {
			continue
		}
		mutate(&detail.ToolPlan[idx])
		return
	}
}

func riskLevelForSeverity(severity string) string {
	switch severity {
	case "critical":
		return "critical"
	case "warning":
		return "warning"
	default:
		return "info"
	}
}

func notificationRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return time.Second
	case 2:
		return 5 * time.Second
	default:
		return 15 * time.Second
	}
}

func sortDirection(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "asc") {
		return "ASC"
	}
	return "DESC"
}

func sessionSortColumn(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "status":
		return "s.status"
	case "session_id":
		return "s.id"
	case "created_at":
		return "s.opened_at"
	case "updated_at":
		fallthrough
	default:
		return "s.updated_at"
	}
}

func executionSortColumn(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "triage":
		return "created_at"
	case "status":
		return "status"
	case "target_host":
		return "target_host"
	case "execution_id":
		return "id"
	case "completed_at":
		return "completed_at"
	case "created_at":
		fallthrough
	default:
		return "created_at"
	}
}

func outboxSortColumn(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "status":
		return "status"
	case "topic":
		return "topic"
	case "created_at":
		fallthrough
	default:
		return "created_at"
	}
}

func nullableJSON(value []byte) interface{} {
	if len(value) == 0 {
		return nil
	}
	return value
}

func cloneInterfaceMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneToolPlanSteps(items []contracts.ToolPlanStep) []contracts.ToolPlanStep {
	if len(items) == 0 {
		return nil
	}
	out := make([]contracts.ToolPlanStep, len(items))
	copy(out, items)
	for i := range out {
		out[i].Runtime = contracts.CloneRuntimeMetadata(items[i].Runtime)
		out[i].Input = cloneInterfaceMap(items[i].Input)
		out[i].ResolvedInput = cloneInterfaceMap(items[i].ResolvedInput)
		out[i].Output = cloneInterfaceMap(items[i].Output)
	}
	return out
}

func cloneMessageAttachments(items []contracts.MessageAttachment) []contracts.MessageAttachment {
	if len(items) == 0 {
		return nil
	}
	out := make([]contracts.MessageAttachment, len(items))
	copy(out, items)
	for i := range out {
		out[i].Metadata = cloneInterfaceMap(items[i].Metadata)
	}
	return out
}

func splitStringByBytes(input string, chunkBytes int) []string {
	if chunkBytes <= 0 {
		chunkBytes = 16384
	}
	if input == "" {
		return nil
	}

	items := make([]string, 0, max(1, len(input)/chunkBytes))
	remaining := input
	for len(remaining) > 0 {
		chunk, truncated := truncateStringByBytes(remaining, chunkBytes)
		if chunk == "" && truncated {
			break
		}
		if chunk == "" {
			chunk = remaining
			truncated = false
		}
		items = append(items, chunk)
		if !truncated {
			break
		}
		remaining = remaining[len(chunk):]
	}
	return items
}

func truncateStringByBytes(input string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || input == "" {
		return "", input != ""
	}
	if len(input) <= maxBytes {
		return input, false
	}

	lastBoundary := 0
	for index := range input {
		if index > maxBytes {
			break
		}
		lastBoundary = index
	}
	if lastBoundary == 0 {
		return "", true
	}
	return input[:lastBoundary], true
}
