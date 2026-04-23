package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"tars/internal/contracts"
	"tars/internal/modules/access"
)

type executionActionRequest struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

type approvalActionOutcome struct {
	ExecutionFailure             error
	CapabilityFailure            error
	ResultNotificationQueued     bool
	CapabilityNotificationQueued bool
}

func (o approvalActionOutcome) HasWarnings() bool {
	return o.ExecutionFailure != nil || o.CapabilityFailure != nil || o.ResultNotificationQueued || o.CapabilityNotificationQueued
}

func executionActionHandler(deps Dependencies, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		principal, ok := requireAuthenticatedPrincipal(deps, w, r, "executions.write")
		if !ok {
			return
		}

		executionID, _ := nestedResourcePath(r.URL.Path, "/api/v1/executions/")
		if strings.TrimSpace(executionID) == "" {
			writeError(w, http.StatusNotFound, "not_found", "execution not found")
			return
		}

		var req executionActionRequest
		if action == "modify_approve" || action == "reject" {
			if err := decodeJSONBody(r, &req); err != nil && action == "modify_approve" {
				writeValidationError(w, err.Error())
				return
			}
			if action == "modify_approve" && strings.TrimSpace(req.Command) == "" {
				writeValidationError(w, "command is required")
				return
			}
		}

		actorID := approvalActorID(principal)
		event := contracts.ChannelEvent{
			EventType:      "approval",
			Channel:        "in_app_inbox",
			UserID:         actorID,
			ChatID:         actorID,
			Action:         action,
			ExecutionID:    executionID,
			Command:        strings.TrimSpace(req.Command),
			Reason:         strings.TrimSpace(req.Reason),
			IdempotencyKey: approvalHTTPIdempotencyKey(r, executionID, action, actorID),
		}

		dispatchResult, err := deps.Workflow.HandleChannelEvent(r.Context(), event)
		if err != nil {
			writeApprovalActionError(w, err)
			return
		}

		if _, err := processApprovalDispatch(r.Context(), deps, event, dispatchResult); err != nil {
			writeApprovalActionError(w, err)
			return
		}

		auditMetadata := map[string]any{
			"execution_id": executionID,
			"action":       action,
		}
		for key, value := range principalAuditMetadata(principal) {
			auditMetadata[key] = value
		}
		auditOpsWrite(r.Context(), deps, "execution", executionID, "approval_endpoint_invoked", auditMetadata)

		executionDetail, err := deps.Workflow.GetExecution(r.Context(), executionID)
		if err != nil {
			if errors.Is(err, contracts.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "execution not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, executionDTO(executionDetail))
	}
}

func processApprovalDispatch(ctx context.Context, deps Dependencies, channelEvent contracts.ChannelEvent, dispatchResult contracts.WorkflowDispatchResult) (approvalActionOutcome, error) {
	outcome := approvalActionOutcome{}
	if err := deliverNotifications(ctx, deps.Channel, deps.Workflow, dispatchResult.SessionID, dispatchResult.Notifications); err != nil {
		return outcome, err
	}

	for _, executionReq := range dispatchResult.Executions {
		result, execErr := deps.Action.ExecuteApproved(ctx, executionReq)
		if execErr != nil {
			result = contracts.ExecutionResult{
				ExecutionID:   executionReq.ExecutionID,
				SessionID:     executionReq.SessionID,
				Status:        "failed",
				ConnectorID:   executionReq.ConnectorID,
				Protocol:      executionReq.Protocol,
				ExecutionMode: executionReq.ExecutionMode,
				Runtime: &contracts.RuntimeMetadata{
					Runtime:         fallbackString(executionReq.Protocol, "ssh"),
					Selection:       "auto_selector",
					ConnectorID:     executionReq.ConnectorID,
					ConnectorVendor: executionReq.ConnectorVendor,
					Protocol:        executionReq.Protocol,
					ExecutionMode:   executionReq.ExecutionMode,
					FallbackEnabled: true,
					FallbackTarget:  "ssh",
				},
				ExitCode: 1,
			}
			outcome.ExecutionFailure = execErr
		}

		mutation, mutationErr := deps.Workflow.HandleExecutionResult(ctx, result)
		if mutationErr != nil {
			return outcome, mutationErr
		}
		if mutation.Status == "verifying" {
			sessionDetail, sessionErr := deps.Workflow.GetSession(ctx, mutation.SessionID)
			if sessionErr != nil {
				return outcome, sessionErr
			}

			verificationResult, verifyErr := deps.Action.VerifyExecution(ctx, contracts.VerificationRequest{
				SessionID:     executionReq.SessionID,
				ExecutionID:   executionReq.ExecutionID,
				TargetHost:    executionReq.TargetHost,
				Service:       alertLabel(sessionDetail.Alert, "service"),
				ConnectorID:   executionReq.ConnectorID,
				Protocol:      executionReq.Protocol,
				ExecutionMode: executionReq.ExecutionMode,
			})
			if verifyErr != nil {
				return outcome, verifyErr
			}
			mutation, mutationErr = deps.Workflow.HandleVerificationResult(ctx, verificationResult)
			if mutationErr != nil {
				return outcome, mutationErr
			}
		}

		queued, err := deliverExecutionResultNotification(ctx, deps, channelEvent, mutation.SessionID, executionReq.ExecutionID)
		if err != nil {
			return outcome, err
		}
		if queued {
			outcome.ResultNotificationQueued = true
		}
	}

	for _, capabilityReq := range dispatchResult.Capabilities {
		capResult, capErr := deps.Action.InvokeApprovedCapability(ctx, capabilityReq)
		capMutationInput := contracts.CapabilityExecutionResult{
			ApprovalID:   capabilityReq.ApprovalID,
			SessionID:    capabilityReq.SessionID,
			StepID:       capabilityReq.StepID,
			ConnectorID:  capabilityReq.ConnectorID,
			CapabilityID: capabilityReq.CapabilityID,
		}
		if capErr != nil {
			capMutationInput.Status = "failed"
			capMutationInput.Runtime = contracts.CloneRuntimeMetadata(capabilityReq.Runtime)
			capMutationInput.Error = capErr.Error()
			outcome.CapabilityFailure = capErr
		} else {
			capMutationInput.Status = capResult.Status
			capMutationInput.Output = capResult.Output
			capMutationInput.Artifacts = capResult.Artifacts
			capMutationInput.Metadata = capResult.Metadata
			capMutationInput.Runtime = capResult.Runtime
			capMutationInput.Error = capResult.Error
		}

		mutation, mutationErr := deps.Workflow.HandleCapabilityResult(ctx, capMutationInput)
		if mutationErr != nil {
			return outcome, mutationErr
		}

		message := contracts.ChannelMessage{
			Channel:     channelEvent.Channel,
			Target:      channelEvent.ChatID,
			Body:        fmt.Sprintf("[%s] capability=%s status=%s connector=%s", capabilityReq.ApprovalID, capabilityReq.CapabilityID, capMutationInput.Status, capabilityReq.ConnectorID),
			RefType:     "execution",
			RefID:       capabilityReq.ApprovalID,
			Source:      "workflow_capability_result",
			Attachments: capMutationInput.Artifacts,
		}
		if _, sendErr := deps.Channel.SendMessage(ctx, message); sendErr != nil {
			if enqueueErr := deps.Workflow.EnqueueNotifications(ctx, mutation.SessionID, []contracts.ChannelMessage{message}); enqueueErr != nil {
				return outcome, enqueueErr
			}
			outcome.CapabilityNotificationQueued = true
		}
	}

	return outcome, nil
}

func deliverExecutionResultNotification(ctx context.Context, deps Dependencies, channelEvent contracts.ChannelEvent, sessionID string, executionID string) (bool, error) {
	sessionDetail, sessionErr := deps.Workflow.GetSession(ctx, sessionID)
	if sessionErr != nil {
		return false, sessionErr
	}
	executionDetail, detailErr := deps.Workflow.GetExecution(ctx, executionID)
	if detailErr != nil {
		return false, detailErr
	}
	executionOutput, outputErr := deps.Workflow.GetExecutionOutput(ctx, executionID)
	if outputErr != nil && !errors.Is(outputErr, contracts.ErrNotFound) {
		return false, outputErr
	}

	message := contracts.ChannelMessage{
		Channel: channelEvent.Channel,
		Target:  channelEvent.ChatID,
		Body:    formatExecutionResultMessage(sessionDetail, executionDetail, executionOutput),
		RefType: "execution",
		RefID:   executionID,
		Source:  "workflow_execution_result",
	}
	if _, sendErr := deps.Channel.SendMessage(ctx, message); sendErr != nil {
		if enqueueErr := deps.Workflow.EnqueueNotifications(ctx, sessionID, []contracts.ChannelMessage{message}); enqueueErr != nil {
			return false, enqueueErr
		}
		return true, nil
	}
	return false, nil
}

func approvalActorID(principal access.Principal) string {
	if principal.User != nil {
		if trimmed := strings.TrimSpace(principal.User.UserID); trimmed != "" {
			return trimmed
		}
		if trimmed := strings.TrimSpace(principal.User.Username); trimmed != "" {
			return trimmed
		}
	}
	if trimmed := strings.TrimSpace(principal.Token); trimmed != "" {
		return trimmed
	}
	return "web_operator"
}

func approvalHTTPIdempotencyKey(r *http.Request, executionID string, action string, actorID string) string {
	if r == nil {
		return ""
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		return ""
	}
	return fmt.Sprintf("web_action:%s:%s:%s:%s", actorID, executionID, action, key)
}

func writeApprovalActionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, contracts.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "execution not found")
	case errors.Is(err, contracts.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", "execution is no longer pending approval")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

func decodeJSONBody(r *http.Request, dest interface{}) error {
	if r == nil || r.Body == nil {
		return fmt.Errorf("invalid request body")
	}
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		return fmt.Errorf("invalid request body")
	}
	return nil
}
