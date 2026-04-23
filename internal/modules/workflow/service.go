package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tars/internal/contracts"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/connectors"
	"tars/internal/modules/reasoning"
)

type Options struct {
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
}

type Service struct {
	mu                      sync.RWMutex
	seq                     atomic.Uint64
	outboxSeq               atomic.Uint64
	executionSeq            atomic.Uint64
	opts                    Options
	sessions                map[string]*sessionRecord
	sessionOrder            []string
	sessionByFingerprint    map[string]string
	executions              map[string]contracts.ExecutionDetail
	executionOutputs        map[string][]contracts.ExecutionOutputChunk
	executionSession        map[string]string
	processedEvents         map[string]struct{}
	processedAlerts         map[string]string
	outbox                  map[string]*contracts.OutboxEvent
	outboxPayloads          map[string][]byte
	outboxOrder             []string
	approvalRouter          approvalrouting.Router
	authorizationPolicy     authorization.Evaluator
	desensitizationProvider reasoning.DesensitizationProvider
	connectors              *connectors.Manager
}

type sessionRecord struct {
	detail      contracts.SessionDetail
	fingerprint string
	host        string
	desenseMap  map[string]string
}

func NewService(opts Options) *Service {
	if opts.ApprovalTimeout <= 0 {
		opts.ApprovalTimeout = 15 * time.Minute
	}
	return &Service{
		opts:                    opts,
		sessions:                make(map[string]*sessionRecord),
		sessionByFingerprint:    make(map[string]string),
		executions:              make(map[string]contracts.ExecutionDetail),
		executionOutputs:        make(map[string][]contracts.ExecutionOutputChunk),
		executionSession:        make(map[string]string),
		processedEvents:         make(map[string]struct{}),
		processedAlerts:         make(map[string]string),
		outbox:                  make(map[string]*contracts.OutboxEvent),
		outboxPayloads:          make(map[string][]byte),
		approvalRouter:          opts.ApprovalRouter,
		authorizationPolicy:     opts.AuthorizationPolicy,
		desensitizationProvider: opts.DesensitizationProvider,
		connectors:              opts.Connectors,
	}
}

func (s *Service) HandleAlertEvent(_ context.Context, event contracts.AlertEvent) (contracts.SessionMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key := strings.TrimSpace(event.IdempotencyKey); key != "" {
		if sessionID, ok := s.processedAlerts[key]; ok {
			if record, found := s.sessions[sessionID]; found {
				return contracts.SessionMutationResult{
					SessionID:  sessionID,
					Status:     record.detail.Status,
					Duplicated: true,
				}, nil
			}
		}
	}

	fingerprint := event.Fingerprint
	if fingerprint == "" {
		fingerprint = fmt.Sprintf("fp-%06d", s.seq.Load()+1)
	}

	now := event.ReceivedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	if sessionID, ok := s.sessionByFingerprint[fingerprint]; ok {
		record := s.sessions[sessionID]
		if key := strings.TrimSpace(event.IdempotencyKey); key != "" {
			s.processedAlerts[key] = sessionID
		}
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "alert_repeated",
			Message:   "received duplicate alert for existing session",
			CreatedAt: now,
		})
		return contracts.SessionMutationResult{
			SessionID:  sessionID,
			Status:     record.detail.Status,
			Duplicated: true,
		}, nil
	}

	sessionID := s.nextID("ses", &s.seq)
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

	alertPayload := map[string]interface{}{
		"source":      event.Source,
		"severity":    event.Severity,
		"fingerprint": fingerprint,
		"host":        host,
		"labels":      cloneStringMap(event.Labels),
		"annotations": cloneStringMap(event.Annotations),
		"received_at": now,
	}

	detail := contracts.SessionDetail{
		SessionID:        sessionID,
		AgentRoleID:      "diagnosis",
		Status:           status,
		DiagnosisSummary: diagnosisSummary,
		Alert:            alertPayload,
		Executions:       []contracts.ExecutionDetail{},
		Timeline: []contracts.TimelineEvent{
			{
				Event:     "alert_received",
				Message:   "alert ingested from vmalert webhook",
				CreatedAt: now,
			},
		},
	}
	if outboxStatus == "pending" {
		detail.Timeline = append(detail.Timeline, contracts.TimelineEvent{
			Event:     "diagnosis_requested",
			Message:   "queued diagnosis request for reasoning pipeline",
			CreatedAt: now,
		})
	} else {
		detail.Timeline = append(detail.Timeline, contracts.TimelineEvent{
			Event:     "diagnosis_blocked",
			Message:   "diagnosis request blocked by feature flag",
			CreatedAt: now,
		})
	}

	record := &sessionRecord{
		detail:      detail,
		fingerprint: fingerprint,
		host:        host,
	}
	s.sessions[sessionID] = record
	s.sessionOrder = append(s.sessionOrder, sessionID)
	s.sessionByFingerprint[fingerprint] = sessionID
	if key := strings.TrimSpace(event.IdempotencyKey); key != "" {
		s.processedAlerts[key] = sessionID
	}

	if _, err := s.publishEventLocked(contracts.EventPublishRequest{
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

	return contracts.SessionMutationResult{
		SessionID: sessionID,
		Status:    status,
	}, nil
}

func (s *Service) HandleChannelEvent(_ context.Context, event contracts.ChannelEvent) (contracts.WorkflowDispatchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.ExecutionID == "" {
		return contracts.WorkflowDispatchResult{}, contracts.ErrNotFound
	}
	if event.IdempotencyKey != "" {
		if _, ok := s.processedEvents[event.IdempotencyKey]; ok {
			return contracts.WorkflowDispatchResult{}, nil
		}
	}

	executionDetail, ok := s.executions[event.ExecutionID]
	if !ok {
		return contracts.WorkflowDispatchResult{}, contracts.ErrNotFound
	}

	sessionID := s.executionSession[event.ExecutionID]
	record, ok := s.sessions[sessionID]
	if !ok {
		return contracts.WorkflowDispatchResult{}, contracts.ErrNotFound
	}

	now := time.Now().UTC()
	requestKind := strings.TrimSpace(executionDetail.RequestKind)
	if requestKind == "" {
		requestKind = "execution"
	}
	switch event.Action {
	case "approve", "modify_approve":
		if executionDetail.Status != "pending" {
			return contracts.WorkflowDispatchResult{}, contracts.ErrInvalidState
		}
		if requestKind == "capability" {
			executionDetail.Status = "executing"
			executionDetail.ApprovedAt = now
			s.executions[event.ExecutionID] = executionDetail
			record.detail.Executions = upsertExecutionDetail(record.detail.Executions, executionDetail)
			record.detail.Status = "executing"
			record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
				Event:     "capability_approval_accepted",
				Message:   fmt.Sprintf("capability approval %s approved by %s capability=%s connector=%s", event.ExecutionID, event.UserID, executionDetail.CapabilityID, executionDetail.ConnectorID),
				CreatedAt: now,
			})
			if event.IdempotencyKey != "" {
				s.processedEvents[event.IdempotencyKey] = struct{}{}
			}
			return contracts.WorkflowDispatchResult{
				SessionID: record.detail.SessionID,
				Notifications: []contracts.ChannelMessage{{
					Channel: event.Channel,
					Target:  event.ChatID,
					Body:    fmt.Sprintf("[%s] capability approved, invoking %s on connector %s", event.ExecutionID, executionDetail.CapabilityID, executionDetail.ConnectorID),
				}},
				Capabilities: []contracts.ApprovedCapabilityRequest{{
					ApprovalID:    executionDetail.ExecutionID,
					SessionID:     record.detail.SessionID,
					StepID:        executionDetail.StepID,
					ConnectorID:   executionDetail.ConnectorID,
					CapabilityID:  executionDetail.CapabilityID,
					Params:        cloneInterfaceMap(executionDetail.CapabilityParams),
					RequestedBy:   firstNonEmpty(event.UserID, executionDetail.RequestedBy),
					ApprovalGroup: executionDetail.ApprovalGroup,
					Runtime:       contracts.CloneRuntimeMetadata(executionDetail.Runtime),
				}},
			}, nil
		}
		if event.Action == "modify_approve" && event.Command != "" {
			executionDetail.Command = event.Command
		}
		if !s.opts.ExecutionEnabled {
			executionDetail.Status = "approved"
			executionDetail.ApprovedAt = now
			s.executions[event.ExecutionID] = executionDetail
			record.detail.Executions = upsertExecutionDetail(record.detail.Executions, executionDetail)
			record.detail.Status = "open"
			record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
				Event:     "approval_accepted_manual",
				Message:   fmt.Sprintf("execution %s approved by %s but execution is disabled command=%s %s", event.ExecutionID, event.UserID, executionDetail.Command, workflowExecutionRuntimeSummary(executionDetail)),
				CreatedAt: now,
			})
			if event.IdempotencyKey != "" {
				s.processedEvents[event.IdempotencyKey] = struct{}{}
			}
			return contracts.WorkflowDispatchResult{
				SessionID: record.detail.SessionID,
				Notifications: []contracts.ChannelMessage{
					{
						Channel: event.Channel,
						Target:  event.ChatID,
						Body:    fmt.Sprintf("[%s] execution approved by %s but execution is disabled; please handle manually", event.ExecutionID, event.UserID),
					},
				},
			}, nil
		}
		if event.IdempotencyKey != "" {
			s.processedEvents[event.IdempotencyKey] = struct{}{}
		}

		executionDetail.Status = "executing"
		executionDetail.ApprovedAt = now
		s.executions[event.ExecutionID] = executionDetail
		record.detail.Executions = upsertExecutionDetail(record.detail.Executions, executionDetail)
		record.detail.Status = "executing"
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "approval_accepted",
			Message:   fmt.Sprintf("execution %s approved by %s command=%s %s", event.ExecutionID, event.UserID, executionDetail.Command, workflowExecutionRuntimeSummary(executionDetail)),
			CreatedAt: now,
		})
		return contracts.WorkflowDispatchResult{
			SessionID: record.detail.SessionID,
			Notifications: []contracts.ChannelMessage{
				{
					Channel: event.Channel,
					Target:  event.ChatID,
					Body:    fmt.Sprintf("[%s] execution approved, starting command on %s", event.ExecutionID, executionDetail.TargetHost),
				},
			},
			Executions: []contracts.ApprovedExecutionRequest{
				{
					ExecutionID:     executionDetail.ExecutionID,
					SessionID:       record.detail.SessionID,
					TargetHost:      executionDetail.TargetHost,
					Command:         executionDetail.Command,
					Service:         alertLabel(record.detail.Alert, "service"),
					ConnectorID:     executionDetail.ConnectorID,
					ConnectorType:   executionDetail.ConnectorType,
					ConnectorVendor: executionDetail.ConnectorVendor,
					Protocol:        executionDetail.Protocol,
					ExecutionMode:   executionDetail.ExecutionMode,
				},
			},
		}, nil
	case "reject":
		if executionDetail.Status != "pending" {
			return contracts.WorkflowDispatchResult{}, contracts.ErrInvalidState
		}
		executionDetail.Status = "rejected"
		s.executions[event.ExecutionID] = executionDetail
		record.detail.Executions = upsertExecutionDetail(record.detail.Executions, executionDetail)
		record.detail.Status = "analyzing"
		eventType := "approval_rejected"
		message := fmt.Sprintf("execution %s rejected by %s", event.ExecutionID, event.UserID)
		if requestKind == "capability" {
			eventType = "capability_approval_rejected"
			message = fmt.Sprintf("capability approval %s rejected by %s", event.ExecutionID, event.UserID)
			updateToolPlanStep(record, executionDetail.StepID, func(step *contracts.ToolPlanStep) {
				step.Status = "rejected"
				ensureMap(&step.Output)["approval_id"] = executionDetail.ExecutionID
				ensureMap(&step.Output)["status"] = "rejected"
			})
		}
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     eventType,
			Message:   message,
			CreatedAt: now,
		})
		if event.IdempotencyKey != "" {
			s.processedEvents[event.IdempotencyKey] = struct{}{}
		}
		return contracts.WorkflowDispatchResult{
			SessionID: record.detail.SessionID,
			Notifications: []contracts.ChannelMessage{
				{
					Channel: event.Channel,
					Target:  event.ChatID,
					Body:    fmt.Sprintf("[%s] execution rejected by %s", event.ExecutionID, event.UserID),
				},
			},
		}, nil
	case "request_context":
		if executionDetail.Status != "pending" {
			return contracts.WorkflowDispatchResult{}, contracts.ErrInvalidState
		}
		record.detail.Status = "analyzing"
		eventType := "approval_requested_context"
		message := fmt.Sprintf("additional context requested by %s", event.UserID)
		if requestKind == "capability" {
			eventType = "capability_approval_requested_context"
			message = fmt.Sprintf("additional capability context requested by %s", event.UserID)
		}
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     eventType,
			Message:   message,
			CreatedAt: now,
		})
		if event.IdempotencyKey != "" {
			s.processedEvents[event.IdempotencyKey] = struct{}{}
		}
		return contracts.WorkflowDispatchResult{
			SessionID: record.detail.SessionID,
			Notifications: []contracts.ChannelMessage{
				{
					Channel: event.Channel,
					Target:  event.ChatID,
					Body:    fmt.Sprintf("[%s] additional context requested", event.ExecutionID),
				},
			},
		}, nil
	default:
		return contracts.WorkflowDispatchResult{}, fmt.Errorf("unsupported channel action: %s", event.Action)
	}
}

func (s *Service) HandleExecutionResult(_ context.Context, result contracts.ExecutionResult) (contracts.SessionMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	executionDetail, ok := s.executions[result.ExecutionID]
	if !ok {
		executionDetail = contracts.ExecutionDetail{
			ExecutionID: result.ExecutionID,
			Status:      result.Status,
		}
	}
	executionDetail.Status = result.Status
	executionDetail.ExitCode = result.ExitCode
	executionDetail.OutputRef = result.OutputRef
	executionDetail.OutputBytes = result.OutputBytes
	executionDetail.OutputTruncated = result.OutputTruncated
	executionDetail.Runtime = contracts.CloneRuntimeMetadata(result.Runtime)
	completedAt := time.Now().UTC()
	executionDetail.CompletedAt = completedAt
	s.executions[result.ExecutionID] = executionDetail
	s.executionOutputs[result.ExecutionID] = buildExecutionOutputChunks(result.OutputPreview, completedAt)

	sessionID := result.SessionID
	if sessionID == "" {
		sessionID = s.executionSession[result.ExecutionID]
	}

	record, ok := s.sessions[sessionID]
	if !ok {
		return contracts.SessionMutationResult{}, contracts.ErrNotFound
	}

	record.detail.Executions = upsertExecutionDetail(record.detail.Executions, executionDetail)
	now := completedAt
	switch result.Status {
	case "completed":
		record.detail.Status = "verifying"
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "execution_completed",
			Message:   fmt.Sprintf("execution %s completed successfully %s", result.ExecutionID, workflowExecutionRuntimeSummary(executionDetail)),
			CreatedAt: now,
		})
	case "failed", "timeout":
		record.detail.Status = "failed"
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "execution_failed",
			Message:   fmt.Sprintf("execution %s ended with status %s %s", result.ExecutionID, result.Status, workflowExecutionRuntimeSummary(executionDetail)),
			CreatedAt: now,
		})
	default:
		record.detail.Status = "verifying"
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "execution_result_received",
			Message:   fmt.Sprintf("execution %s returned status %s %s", result.ExecutionID, result.Status, workflowExecutionRuntimeSummary(executionDetail)),
			CreatedAt: now,
		})
	}

	return contracts.SessionMutationResult{
		SessionID: sessionID,
		Status:    record.detail.Status,
	}, nil
}

func (s *Service) HandleCapabilityResult(_ context.Context, result contracts.CapabilityExecutionResult) (contracts.SessionMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	executionDetail, ok := s.executions[result.ApprovalID]
	if !ok {
		executionDetail = contracts.ExecutionDetail{ExecutionID: result.ApprovalID, RequestKind: "capability"}
	}
	executionDetail.Status = result.Status
	executionDetail.StepID = firstNonEmpty(result.StepID, executionDetail.StepID)
	executionDetail.CapabilityID = firstNonEmpty(result.CapabilityID, executionDetail.CapabilityID)
	executionDetail.ConnectorID = firstNonEmpty(result.ConnectorID, executionDetail.ConnectorID)
	executionDetail.Runtime = contracts.CloneRuntimeMetadata(result.Runtime)
	executionDetail.CompletedAt = time.Now().UTC()
	s.executions[result.ApprovalID] = executionDetail

	sessionID := firstNonEmpty(result.SessionID, s.executionSession[result.ApprovalID])
	record, ok := s.sessions[sessionID]
	if !ok {
		return contracts.SessionMutationResult{}, contracts.ErrNotFound
	}
	record.detail.Executions = upsertExecutionDetail(record.detail.Executions, executionDetail)
	updateToolPlanStep(record, executionDetail.StepID, func(step *contracts.ToolPlanStep) {
		step.Status = result.Status
		step.Runtime = contracts.CloneRuntimeMetadata(result.Runtime)
		step.CompletedAt = executionDetail.CompletedAt
		output := ensureMap(&step.Output)
		output["approval_id"] = result.ApprovalID
		output["status"] = result.Status
		if len(result.Output) > 0 {
			output["result"] = cloneInterfaceMap(result.Output)
		}
		if len(result.Metadata) > 0 {
			output["metadata"] = cloneInterfaceMap(result.Metadata)
		}
		if result.Error != "" {
			output["error"] = result.Error
		}
	})
	if len(result.Artifacts) > 0 {
		record.detail.Attachments = append(record.detail.Attachments, cloneMessageAttachments(result.Artifacts)...)
	}
	now := executionDetail.CompletedAt
	if result.Status == "completed" {
		record.detail.Status = "open"
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "capability_completed",
			Message:   fmt.Sprintf("capability approval %s completed capability=%s connector=%s", result.ApprovalID, executionDetail.CapabilityID, executionDetail.ConnectorID),
			CreatedAt: now,
		})
	} else {
		record.detail.Status = "failed"
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "capability_failed",
			Message:   fmt.Sprintf("capability approval %s failed: %s", result.ApprovalID, firstNonEmpty(result.Error, result.Status)),
			CreatedAt: now,
		})
	}
	return contracts.SessionMutationResult{SessionID: sessionID, Status: record.detail.Status}, nil
}

func (s *Service) HandleVerificationResult(_ context.Context, result contracts.VerificationResult) (contracts.SessionMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.sessions[result.SessionID]
	if !ok {
		return contracts.SessionMutationResult{}, contracts.ErrNotFound
	}

	now := result.CheckedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	record.detail.Verification = &contracts.SessionVerification{
		Status:    result.Status,
		Summary:   result.Summary,
		Details:   cloneInterfaceMap(result.Details),
		Runtime:   contracts.CloneRuntimeMetadata(result.Runtime),
		CheckedAt: now,
	}

	eventType := "verify_failed"
	switch result.Status {
	case "success", "skipped":
		record.detail.Status = "resolved"
		eventType = "verify_success"
		s.enqueueSessionClosed(record.detail.SessionID, now)
	case "failed":
		record.detail.Status = "analyzing"
	default:
		record.detail.Status = "analyzing"
	}

	record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
		Event:     eventType,
		Message:   result.Summary,
		CreatedAt: now,
	})

	return contracts.SessionMutationResult{
		SessionID: result.SessionID,
		Status:    record.detail.Status,
	}, nil
}

func (s *Service) SweepApprovalTimeouts(_ context.Context, now time.Time) ([]contracts.ChannelMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if now.IsZero() {
		now = time.Now().UTC()
	}

	notifications := make([]contracts.ChannelMessage, 0)
	for executionID, executionDetail := range s.executions {
		if executionDetail.Status != "pending" || executionDetail.CreatedAt.IsZero() {
			continue
		}
		if now.Sub(executionDetail.CreatedAt) < s.opts.ApprovalTimeout {
			continue
		}

		executionDetail.Status = "rejected"
		s.executions[executionID] = executionDetail

		sessionID := s.executionSession[executionID]
		record, ok := s.sessions[sessionID]
		if !ok {
			continue
		}

		record.detail.Executions = upsertExecutionDetail(record.detail.Executions, executionDetail)
		record.detail.Status = "open"
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "approval_timed_out",
			Message:   fmt.Sprintf("execution %s timed out waiting for approval", executionID),
			CreatedAt: now,
		})
		route := s.resolveApprovalRoute(record.detail.Alert, executionDetail.RequestedBy)
		for _, target := range route.Targets {
			notifications = append(notifications, contracts.ChannelMessage{
				Channel: notificationChannelForAlert(record.detail.Alert),
				Target:  notificationTargetForAlert(record.detail.Alert, target),
				Body:    fmt.Sprintf("[%s] approval timed out for execution %s; request was auto-rejected", record.detail.SessionID, executionID),
			})
		}
	}

	return notifications, nil
}

func (s *Service) ListSessions(_ context.Context, filter contracts.ListSessionsFilter) ([]contracts.SessionDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]contracts.SessionDetail, 0, len(s.sessionOrder))
	for i := len(s.sessionOrder) - 1; i >= 0; i-- {
		record := s.sessions[s.sessionOrder[i]]
		if filter.Status != "" && record.detail.Status != filter.Status {
			continue
		}
		if filter.Host != "" && record.host != filter.Host {
			continue
		}
		detail := cloneSessionDetail(record.detail)
		if !matchesSessionQuery(detail, filter.Query) {
			continue
		}
		contracts.PopulateSessionGoldenPath(&detail)
		items = append(items, detail)
	}
	sortSessionDetails(items, filter.SortBy, filter.SortOrder)
	return items, nil
}

func (s *Service) ListExecutions(_ context.Context, filter contracts.ListExecutionsFilter) ([]contracts.ExecutionDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]contracts.ExecutionDetail, 0, len(s.executions))
	for _, execution := range s.executions {
		if filter.Status != "" && execution.Status != filter.Status {
			continue
		}
		if !matchesExecutionQuery(execution, filter.Query) {
			continue
		}
		execution.SessionID = s.executionSession[execution.ExecutionID]
		contracts.PopulateExecutionGoldenPath(&execution, nil)
		items = append(items, execution)
	}
	sortExecutionDetails(items, filter.SortBy, filter.SortOrder)
	return items, nil
}

func (s *Service) GetSession(_ context.Context, sessionID string) (contracts.SessionDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.sessions[sessionID]
	if !ok {
		return contracts.SessionDetail{}, contracts.ErrNotFound
	}
	detail := cloneSessionDetail(record.detail)
	contracts.PopulateSessionGoldenPath(&detail)
	return detail, nil
}

func (s *Service) GetExecution(_ context.Context, executionID string) (contracts.ExecutionDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	execution, ok := s.executions[executionID]
	if !ok {
		return contracts.ExecutionDetail{}, contracts.ErrNotFound
	}
	execution.SessionID = s.executionSession[execution.ExecutionID]
	var session *contracts.SessionDetail
	if record, found := s.sessions[execution.SessionID]; found {
		detail := cloneSessionDetail(record.detail)
		contracts.PopulateSessionGoldenPath(&detail)
		session = &detail
	}
	contracts.PopulateExecutionGoldenPath(&execution, session)
	return execution, nil
}

func (s *Service) GetExecutionOutput(_ context.Context, executionID string) ([]contracts.ExecutionOutputChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.executions[executionID]; !ok {
		return nil, contracts.ErrNotFound
	}

	items := s.executionOutputs[executionID]
	out := make([]contracts.ExecutionOutputChunk, len(items))
	copy(out, items)
	return out, nil
}

func (s *Service) ApplyDiagnosis(_ context.Context, eventID string, diagnosis contracts.DiagnosisOutput) (contracts.WorkflowDispatchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.outbox[eventID]
	if !ok {
		return contracts.WorkflowDispatchResult{}, contracts.ErrNotFound
	}
	record, ok := s.sessions[event.AggregateID]
	if !ok {
		return contracts.WorkflowDispatchResult{}, contracts.ErrNotFound
	}

	record.detail.DiagnosisSummary = diagnosis.Summary
	record.detail.ToolPlan = cloneToolPlanSteps(diagnosis.ToolPlan)
	record.detail.Attachments = cloneMessageAttachments(diagnosis.Attachments)
	record.detail.Status = "open"
	record.desenseMap = cloneStringMap(diagnosis.DesenseMap)
	diagnosis = rehydrateDiagnosisOutput(diagnosis, s.currentDesensitizationConfig())
	if diagnosis.ExecutionHint == "" {
		diagnosis.ExecutionHint = firstPlannedExecutionCommand(diagnosis.ToolPlan)
	}
	now := time.Now().UTC()
	route := approvalrouting.Route{}
	if s.opts.ApprovalEnabled {
		route = s.resolveApprovalRoute(record.detail.Alert, "tars")
	}
	diagnosisTarget := selectDiagnosisNotificationTarget(record.detail.Alert, route)
	notifications := []contracts.ChannelMessage{
		{
			Channel:     notificationChannelForAlert(record.detail.Alert),
			Target:      notificationTargetForAlert(record.detail.Alert, diagnosisTarget),
			Subject:     "诊断结果",
			Body:        formatDiagnosisMessage(record.detail, diagnosis),
			RefType:     "session",
			RefID:       record.detail.SessionID,
			Attachments: cloneMessageAttachments(diagnosis.Attachments),
		},
	}
	immediateExecutions := make([]contracts.ApprovedExecutionRequest, 0, 1)
	record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
		Event:     "diagnosis_message_prepared",
		Message:   fmt.Sprintf("target=%s body=%s", notifications[0].Target, compactSnippet(notifications[0].Body)),
		CreatedAt: now,
	})
	source := stringFromAlert(record.detail.Alert, "source")
	if diagnosis.ExecutionHint != "" && s.opts.DiagnosisEnabled {
		authDecision := s.resolveAuthorizationDecision(record.detail.Alert, diagnosis.ExecutionHint)
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "authorization_decided",
			Message:   fmt.Sprintf("action=%s rule=%s matched_by=%s command=%s", authDecision.Action, fallbackString(authDecision.RuleID, "default"), fallbackString(authDecision.MatchedBy, "default"), diagnosis.ExecutionHint),
			CreatedAt: now,
		})

		switch authDecision.Action {
		case authorization.ActionDirectExecute:
			if s.opts.ExecutionEnabled {
				executionID := s.nextID("exe", &s.executionSeq)
				runtimeSelection := s.selectExecutionRuntime(stringFromAlert(record.detail.Alert, "host"))
				executionDetail := contracts.ExecutionDetail{
					ExecutionID:     executionID,
					RequestKind:     "execution",
					Status:          "executing",
					RiskLevel:       riskLevelForAlert(record.detail.Alert),
					Command:         diagnosis.ExecutionHint,
					TargetHost:      stringFromAlert(record.detail.Alert, "host"),
					ConnectorID:     runtimeSelection.ConnectorID,
					ConnectorType:   runtimeSelection.ConnectorType,
					ConnectorVendor: runtimeSelection.ConnectorVendor,
					Protocol:        runtimeSelection.Protocol,
					ExecutionMode:   runtimeSelection.ExecutionMode,
					RequestedBy:     "tars",
					ApprovalGroup:   "policy:direct_execute",
					Runtime:         contracts.CloneRuntimeMetadata(runtimeSelection.Runtime),
					CreatedAt:       now,
					ApprovedAt:      now,
				}
				s.executions[executionID] = executionDetail
				s.executionSession[executionID] = record.detail.SessionID
				record.detail.Executions = upsertExecutionDetail(record.detail.Executions, executionDetail)
				record.detail.Status = "executing"
				record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
					Event:     "execution_direct_ready",
					Message:   fmt.Sprintf("execution %s started directly by authorization policy command=%s %s", executionID, diagnosis.ExecutionHint, workflowExecutionRuntimeSummary(executionDetail)),
					CreatedAt: now,
				})
				notifications = append(notifications, buildAuthorizationDecisionMessage(record.detail, diagnosis.ExecutionHint, authDecision, diagnosisTarget, "命中白名单，开始直接执行。"))
				immediateExecutions = append(immediateExecutions, contracts.ApprovedExecutionRequest{
					ExecutionID:     executionDetail.ExecutionID,
					SessionID:       record.detail.SessionID,
					TargetHost:      executionDetail.TargetHost,
					Command:         executionDetail.Command,
					Service:         alertLabel(record.detail.Alert, "service"),
					ConnectorID:     executionDetail.ConnectorID,
					ConnectorType:   executionDetail.ConnectorType,
					ConnectorVendor: executionDetail.ConnectorVendor,
					Protocol:        executionDetail.Protocol,
					ExecutionMode:   executionDetail.ExecutionMode,
				})
			} else {
				record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
					Event:     "execution_disabled",
					Message:   "authorization policy allows direct execution but execution is disabled",
					CreatedAt: now,
				})
				notifications = append(notifications, buildAuthorizationDecisionMessage(record.detail, diagnosis.ExecutionHint, authDecision, diagnosisTarget, "命中白名单，但当前执行功能关闭，请手动处理。"))
				s.resolveChatWithoutExecution(record, now, source, "diagnosis answered without execution because execution is disabled")
			}
		case authorization.ActionRequireApproval:
			if s.opts.ApprovalEnabled {
				executionID := s.nextID("exe", &s.executionSeq)
				runtimeSelection := s.selectExecutionRuntime(stringFromAlert(record.detail.Alert, "host"))
				record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
					Event:     "approval_route_selected",
					Message:   fmt.Sprintf("approval route=%s targets=%s", fallbackString(route.GroupKey, "fallback:unknown"), strings.Join(route.Targets, ",")),
					CreatedAt: now,
				})
				executionDetail := contracts.ExecutionDetail{
					ExecutionID:     executionID,
					RequestKind:     "execution",
					Status:          "pending",
					RiskLevel:       riskLevelForAlert(record.detail.Alert),
					Command:         diagnosis.ExecutionHint,
					TargetHost:      stringFromAlert(record.detail.Alert, "host"),
					ConnectorID:     runtimeSelection.ConnectorID,
					ConnectorType:   runtimeSelection.ConnectorType,
					ConnectorVendor: runtimeSelection.ConnectorVendor,
					Protocol:        runtimeSelection.Protocol,
					ExecutionMode:   runtimeSelection.ExecutionMode,
					RequestedBy:     "tars",
					ApprovalGroup:   route.GroupKey,
					Runtime:         contracts.CloneRuntimeMetadata(runtimeSelection.Runtime),
					CreatedAt:       now,
				}
				s.executions[executionID] = executionDetail
				s.executionSession[executionID] = record.detail.SessionID
				record.detail.Executions = upsertExecutionDetail(record.detail.Executions, executionDetail)
				record.detail.Status = "pending_approval"
				record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
					Event:     "execution_draft_ready",
					Message:   fmt.Sprintf("execution draft %s created command=%s %s", executionID, diagnosis.ExecutionHint, workflowExecutionRuntimeSummary(executionDetail)),
					CreatedAt: now,
				})
				approvalMessages := buildApprovalMessages(record.detail, executionDetail, route, s.opts.ApprovalTimeout)
				notifications = append(notifications, approvalMessages...)
				for _, approvalMessage := range approvalMessages {
					record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
						Event:     "approval_message_prepared",
						Message:   fmt.Sprintf("target=%s body=%s", approvalMessage.Target, compactSnippet(approvalMessage.Body)),
						CreatedAt: now,
					})
				}
			} else {
				record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
					Event:     "approval_disabled",
					Message:   "execution hint produced but approval flow is disabled",
					CreatedAt: now,
				})
				notifications = append(notifications, buildAuthorizationDecisionMessage(record.detail, diagnosis.ExecutionHint, authDecision, diagnosisTarget, "命令需要审批，但当前审批功能关闭，请手动处理。"))
				s.resolveChatWithoutExecution(record, now, source, "diagnosis answered without execution because approval is disabled")
			}
		case authorization.ActionSuggestOnly:
			record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
				Event:     "execution_suggested_only",
				Message:   fmt.Sprintf("authorization policy marked command as suggest_only: %s", diagnosis.ExecutionHint),
				CreatedAt: now,
			})
			notifications = append(notifications, buildAuthorizationDecisionMessage(record.detail, diagnosis.ExecutionHint, authDecision, diagnosisTarget, "命中手动处理策略，当前只展示建议命令，不会自动执行。"))
			s.resolveChatWithoutExecution(record, now, source, "diagnosis answered with suggest-only command")
		case authorization.ActionDeny:
			record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
				Event:     "execution_denied",
				Message:   fmt.Sprintf("authorization policy denied command: %s", diagnosis.ExecutionHint),
				CreatedAt: now,
			})
			notifications = append(notifications, buildAuthorizationDecisionMessage(record.detail, diagnosis.ExecutionHint, authDecision, diagnosisTarget, "命中禁止策略，当前不允许执行这条命令。"))
			s.resolveChatWithoutExecution(record, now, source, "diagnosis answered with denied command")
		}
	}
	if diagnosis.ExecutionHint == "" {
		s.resolveChatWithoutExecution(record, now, source, "diagnosis answered without execution request")
	}
	record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
		Event:     "diagnosis_completed",
		Message:   diagnosis.Summary,
		CreatedAt: now,
	})

	return contracts.WorkflowDispatchResult{
		SessionID:     record.detail.SessionID,
		Notifications: notifications,
		Executions:    immediateExecutions,
	}, nil
}

func (s *Service) CreateCapabilityApproval(_ context.Context, req contracts.ApprovedCapabilityRequest) (contracts.ExecutionDetail, []contracts.ChannelMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.sessions[req.SessionID]
	if !ok {
		return contracts.ExecutionDetail{}, nil, contracts.ErrNotFound
	}
	now := time.Now().UTC()
	route := approvalrouting.Route{}
	if s.opts.ApprovalEnabled {
		route = s.resolveApprovalRoute(record.detail.Alert, fallbackString(req.RequestedBy, "tars"))
	}
	approvalID := strings.TrimSpace(req.ApprovalID)
	if approvalID == "" {
		approvalID = s.nextID("cap", &s.executionSeq)
	}
	detail := contracts.ExecutionDetail{
		ExecutionID:      approvalID,
		RequestKind:      "capability",
		Status:           "pending",
		RiskLevel:        riskLevelForAlert(record.detail.Alert),
		StepID:           strings.TrimSpace(req.StepID),
		CapabilityID:     strings.TrimSpace(req.CapabilityID),
		CapabilityParams: cloneInterfaceMap(req.Params),
		ConnectorID:      strings.TrimSpace(req.ConnectorID),
		ApprovalGroup:    route.GroupKey,
		RequestedBy:      fallbackString(req.RequestedBy, "tars"),
		Runtime:          contracts.CloneRuntimeMetadata(req.Runtime),
		CreatedAt:        now,
	}
	if detail.Runtime != nil {
		detail.ConnectorType = detail.Runtime.ConnectorType
		detail.ConnectorVendor = detail.Runtime.ConnectorVendor
		detail.Protocol = detail.Runtime.Protocol
		detail.ExecutionMode = detail.Runtime.ExecutionMode
	}
	s.executions[approvalID] = detail
	s.executionSession[approvalID] = record.detail.SessionID
	record.detail.Executions = upsertExecutionDetail(record.detail.Executions, detail)
	record.detail.Status = "pending_approval"
	updateToolPlanStep(record, detail.StepID, func(step *contracts.ToolPlanStep) {
		step.Status = "pending_approval"
		step.CompletedAt = now
		ensureMap(&step.Output)["approval_id"] = approvalID
		ensureMap(&step.Output)["status"] = "pending_approval"
	})
	record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
		Event:     "capability_approval_requested",
		Message:   fmt.Sprintf("capability approval %s created connector=%s capability=%s", approvalID, detail.ConnectorID, detail.CapabilityID),
		CreatedAt: now,
	})
	messages := buildCapabilityApprovalMessages(record.detail, detail, route, s.opts.ApprovalTimeout)
	for _, message := range messages {
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "approval_message_prepared",
			Message:   fmt.Sprintf("target=%s body=%s", message.Target, compactSnippet(message.Body)),
			CreatedAt: now,
		})
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

func (s *Service) currentDesensitizationConfig() *reasoning.DesensitizationConfig {
	if s == nil || s.desensitizationProvider == nil {
		cfg := reasoning.DefaultDesensitizationConfig()
		return &cfg
	}
	return s.desensitizationProvider.CurrentDesensitizationConfig()
}

func (s *Service) nextID(prefix string, counter *atomic.Uint64) string {
	id := counter.Add(1)
	return fmt.Sprintf("%s-%06d", prefix, id)
}

func (s *Service) enqueueSessionClosed(sessionID string, now time.Time) {
	status := "pending"
	blockedReason := ""
	if !s.opts.KnowledgeIngestEnabled {
		status = "blocked"
		blockedReason = "knowledge_ingest_disabled"
	}

	_, _ = s.publishEventLocked(contracts.EventPublishRequest{
		Topic:         "session.closed",
		AggregateID:   sessionID,
		Payload:       []byte(`{}`),
		Status:        status,
		BlockedReason: blockedReason,
		CreatedAt:     now,
		AvailableAt:   now,
	})
}

func pickHost(labels map[string]string) string {
	for _, key := range []string{"instance", "host", "node", "pod"} {
		if labels[key] != "" {
			return labels[key]
		}
	}
	return ""
}

func cloneSessionDetail(in contracts.SessionDetail) contracts.SessionDetail {
	out := in
	out.Alert = cloneInterfaceMap(in.Alert)
	out.ToolPlan = cloneToolPlanSteps(in.ToolPlan)
	out.Attachments = cloneMessageAttachments(in.Attachments)
	out.Executions = cloneExecutionDetails(in.Executions)
	out.Timeline = append([]contracts.TimelineEvent(nil), in.Timeline...)
	if in.Verification != nil {
		verification := *in.Verification
		verification.Details = cloneInterfaceMap(in.Verification.Details)
		verification.Runtime = contracts.CloneRuntimeMetadata(in.Verification.Runtime)
		out.Verification = &verification
	}
	if in.GoldenSummary != nil {
		summary := *in.GoldenSummary
		summary.Evidence = append([]string(nil), in.GoldenSummary.Evidence...)
		out.GoldenSummary = &summary
	}
	if len(in.Notifications) > 0 {
		out.Notifications = append([]contracts.NotificationDigest(nil), in.Notifications...)
	}
	return out
}

func cloneExecutionDetails(items []contracts.ExecutionDetail) []contracts.ExecutionDetail {
	if len(items) == 0 {
		return nil
	}
	out := make([]contracts.ExecutionDetail, len(items))
	copy(out, items)
	for i := range out {
		out[i].SessionID = items[i].SessionID
		out[i].Runtime = contracts.CloneRuntimeMetadata(items[i].Runtime)
		if items[i].GoldenSummary != nil {
			summary := *items[i].GoldenSummary
			out[i].GoldenSummary = &summary
		}
		out[i].CapabilityParams = cloneInterfaceMap(items[i].CapabilityParams)
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

func cloneOutboxEvent(in contracts.OutboxEvent) contracts.OutboxEvent {
	return in
}

func upsertExecutionDetail(items []contracts.ExecutionDetail, in contracts.ExecutionDetail) []contracts.ExecutionDetail {
	for i := range items {
		if items[i].ExecutionID == in.ExecutionID {
			items[i] = in
			return items
		}
	}
	return append(items, in)
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneInterfaceMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		switch typed := v.(type) {
		case map[string]string:
			out[k] = cloneStringMap(typed)
		case map[string]interface{}:
			out[k] = cloneInterfaceMap(typed)
		default:
			out[k] = typed
		}
	}
	return out
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

func (s *Service) resolveApprovalRoute(alert map[string]interface{}, requester string) approvalrouting.Route {
	if alertLabel(alert, "tars_chat") == "true" {
		return approvalrouting.Route{
			GroupKey:    "chat_request:direct",
			SourceLabel: "chat request(direct)",
			Targets:     []string{pickNotificationTarget(alert)},
		}
	}
	if s.approvalRouter == nil {
		return approvalrouting.Route{
			GroupKey:    "fallback:default",
			SourceLabel: "fallback(default)",
			Targets:     []string{pickNotificationTarget(alert)},
		}
	}
	return s.approvalRouter.Resolve(alert, requester, pickNotificationTarget(alert))
}

func (s *Service) resolveAuthorizationDecision(alert map[string]interface{}, command string) authorization.Decision {
	var decision authorization.Decision
	if s.authorizationPolicy == nil {
		decision = authorization.Decision{
			Action:    authorization.ActionRequireApproval,
			RuleID:    "mvp_default",
			MatchedBy: "default",
		}
	} else {
		decision = s.authorizationPolicy.EvaluateSSHCommand(authorization.SSHCommandInput{
			Command: command,
			Service: alertLabel(alert, "service"),
			Host:    stringFromAlert(alert, "host"),
			Channel: stringFromAlert(alert, "source"),
		})
	}
	// Apply agent role policy enforcement (take the more restrictive result).
	if s.opts.AgentRoleManager != nil {
		// For in-memory workflow, look up session's agent_role_id from the alert context.
		// The default role for sessions is "diagnosis".
		role := s.opts.AgentRoleManager.ResolveForSession("diagnosis")
		enforced := agentrole.EnforcePolicy(role, string(decision.Action), command)
		decision.Action = authorization.Action(enforced)
	}
	return decision
}

func (s *Service) resolveChatWithoutExecution(record *sessionRecord, now time.Time, source string, message string) {
	if record.detail.Status == "resolved" {
		return
	}
	switch source {
	case "telegram_chat", "web_chat", "ops_api":
		// chat-originated sessions: close on diagnosis completion
	default:
		if alertLabel(record.detail.Alert, "tars_generated") != "ops_setup" {
			return
		}
	}
	record.detail.Status = "resolved"
	record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
		Event:     "chat_answer_completed",
		Message:   message,
		CreatedAt: now,
	})
	s.enqueueSessionClosed(record.detail.SessionID, now)
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
	if diagnosis.ExecutionHint != "" {
		lines = append(lines, fmt.Sprintf("下一步: %s", diagnosis.ExecutionHint))
	}
	if len(diagnosis.Citations) > 0 {
		lines = append(lines, fmt.Sprintf("参考: %d 条知识", minInt(len(diagnosis.Citations), 3)))
	}
	if fingerprint != "" && source != "telegram_chat" {
		lines = append(lines, fmt.Sprintf("指纹: %s", fingerprint))
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

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPlannedExecutionCommand(steps []contracts.ToolPlanStep) string {
	for _, step := range steps {
		if strings.TrimSpace(step.Tool) != "execution.run_command" {
			continue
		}
		if command := firstNonEmpty(mapString(step.ResolvedInput, "command"), mapString(step.Input, "command")); command != "" {
			return command
		}
	}
	return ""
}

func mapString(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return value
	}
	return ""
}

func updateToolPlanStep(record *sessionRecord, stepID string, mutate func(step *contracts.ToolPlanStep)) {
	if record == nil || strings.TrimSpace(stepID) == "" || mutate == nil {
		return
	}
	for idx := range record.detail.ToolPlan {
		if strings.TrimSpace(record.detail.ToolPlan[idx].ID) != strings.TrimSpace(stepID) {
			continue
		}
		mutate(&record.detail.ToolPlan[idx])
		return
	}
}

func ensureMap(target *map[string]interface{}) map[string]interface{} {
	if target == nil {
		return nil
	}
	if *target == nil {
		*target = map[string]interface{}{}
	}
	return *target
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

func compactSnippet(value string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len(trimmed) <= 96 {
		return trimmed
	}
	return trimmed[:93] + "..."
}

func workflowExecutionRuntimeSummary(execution contracts.ExecutionDetail) string {
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

func riskLevelForAlert(alert map[string]interface{}) string {
	switch stringFromAlert(alert, "severity") {
	case "critical":
		return "critical"
	case "warning":
		return "warning"
	default:
		return "info"
	}
}

type executionRuntimeSelection struct {
	ConnectorID     string
	ConnectorType   string
	ConnectorVendor string
	Protocol        string
	ExecutionMode   string
	Runtime         *contracts.RuntimeMetadata
}

func (s *Service) selectExecutionRuntime(_ string) executionRuntimeSelection {
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
	entry, ok := connectors.SelectHealthyRuntimeManifest(s.connectors, "execution", "", map[string]struct{}{"jumpserver_api": {}})
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

func matchesSessionQuery(detail contracts.SessionDetail, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}

	candidates := []string{
		detail.SessionID,
		detail.Status,
		detail.DiagnosisSummary,
		stringFromAlert(detail.Alert, "host"),
		stringFromAlert(detail.Alert, "severity"),
		alertLabel(detail.Alert, "alertname"),
		alertLabel(detail.Alert, "instance"),
		alertLabel(detail.Alert, "service"),
		annotationString(detail.Alert, "summary"),
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), query) {
			return true
		}
	}
	return false
}

func sortSessionDetails(items []contracts.SessionDetail, sortBy string, sortOrder string) {
	desc := strings.ToLower(strings.TrimSpace(sortOrder)) != "asc"
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		switch strings.ToLower(strings.TrimSpace(sortBy)) {
		case "triage":
			cmp := contracts.CompareSessionTriage(left, right)
			if cmp == 0 {
				return false
			}
			if desc {
				return cmp > 0
			}
			return cmp < 0
		case "status":
			if desc {
				return left.Status > right.Status
			}
			return left.Status < right.Status
		case "session_id":
			if desc {
				return left.SessionID > right.SessionID
			}
			return left.SessionID < right.SessionID
		case "created_at":
			fallthrough
		case "updated_at":
			leftTime := sessionSortTime(left)
			rightTime := sessionSortTime(right)
			if desc {
				return leftTime.After(rightTime)
			}
			return leftTime.Before(rightTime)
		default:
			leftTime := sessionSortTime(left)
			rightTime := sessionSortTime(right)
			if desc {
				return leftTime.After(rightTime)
			}
			return leftTime.Before(rightTime)
		}
	})
}

func sessionSortTime(detail contracts.SessionDetail) time.Time {
	if len(detail.Timeline) > 0 {
		return detail.Timeline[len(detail.Timeline)-1].CreatedAt
	}
	return time.Time{}
}

func matchesExecutionQuery(detail contracts.ExecutionDetail, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}

	candidates := []string{
		detail.ExecutionID,
		detail.Status,
		detail.RiskLevel,
		detail.Command,
		detail.TargetHost,
		detail.RequestedBy,
		detail.ApprovalGroup,
		detail.OutputRef,
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), query) {
			return true
		}
	}
	return false
}

func sortExecutionDetails(items []contracts.ExecutionDetail, sortBy string, sortOrder string) {
	desc := strings.ToLower(strings.TrimSpace(sortOrder)) != "asc"
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		switch strings.ToLower(strings.TrimSpace(sortBy)) {
		case "triage":
			cmp := contracts.CompareExecutionTriage(left, right)
			if cmp == 0 {
				return false
			}
			if desc {
				return cmp > 0
			}
			return cmp < 0
		case "status":
			if desc {
				return left.Status > right.Status
			}
			return left.Status < right.Status
		case "target_host":
			if desc {
				return left.TargetHost > right.TargetHost
			}
			return left.TargetHost < right.TargetHost
		case "execution_id":
			if desc {
				return left.ExecutionID > right.ExecutionID
			}
			return left.ExecutionID < right.ExecutionID
		case "completed_at":
			if desc {
				return left.CompletedAt.After(right.CompletedAt)
			}
			return left.CompletedAt.Before(right.CompletedAt)
		case "created_at":
			fallthrough
		default:
			if desc {
				return left.CreatedAt.After(right.CreatedAt)
			}
			return left.CreatedAt.Before(right.CreatedAt)
		}
	})
}

func matchesOutboxQuery(detail contracts.OutboxEvent, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}

	candidates := []string{
		detail.ID,
		detail.Topic,
		detail.Status,
		detail.AggregateID,
		detail.LastError,
		detail.BlockedReason,
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), query) {
			return true
		}
	}
	return false
}

func sortOutboxEvents(items []contracts.OutboxEvent, sortBy string, sortOrder string) {
	desc := strings.ToLower(strings.TrimSpace(sortOrder)) != "asc"
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		switch strings.ToLower(strings.TrimSpace(sortBy)) {
		case "status":
			if desc {
				return left.Status > right.Status
			}
			return left.Status < right.Status
		case "topic":
			if desc {
				return left.Topic > right.Topic
			}
			return left.Topic < right.Topic
		case "created_at":
			fallthrough
		default:
			if desc {
				return left.CreatedAt.After(right.CreatedAt)
			}
			return left.CreatedAt.Before(right.CreatedAt)
		}
	})
}

func buildExecutionOutputChunks(preview string, createdAt time.Time) []contracts.ExecutionOutputChunk {
	if preview == "" {
		return nil
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	rawChunks := splitStringByBytes(preview, 16*1024)
	items := make([]contracts.ExecutionOutputChunk, 0, len(rawChunks))
	for index, chunk := range rawChunks {
		items = append(items, contracts.ExecutionOutputChunk{
			Seq:        index,
			StreamType: "combined",
			Content:    chunk,
			ByteSize:   len([]byte(chunk)),
			CreatedAt:  createdAt,
		})
	}
	return items
}

func splitStringByBytes(input string, chunkBytes int) []string {
	if chunkBytes <= 0 {
		chunkBytes = 16 * 1024
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
