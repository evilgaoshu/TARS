package httpapi

import (
	"context"
	"net/http"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/contracts"
	"tars/internal/foundation/audit"
	"tars/internal/modules/access"
	"tars/internal/modules/automations"
)

func requireOpsAccess(deps Dependencies, w http.ResponseWriter, r *http.Request) bool {
	if !deps.Config.OpsAPI.Enabled {
		writeError(w, http.StatusNotFound, "ops_api_disabled", "ops api is disabled")
		return false
	}
	principal, ok := authenticatedPrincipal(deps, r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid bearer token")
		return false
	}
	if deps.Access != nil {
		permission := routePermission(r)
		if permission != "" && !deps.Access.Evaluate(principal, permission) {
			writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return false
		}
	}
	return true
}

func requireAuthenticatedPrincipal(deps Dependencies, w http.ResponseWriter, r *http.Request, permission string) (access.Principal, bool) {
	principal, ok := authenticatedPrincipal(deps, r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return access.Principal{}, false
	}
	if permission != "" && deps.Access != nil && !deps.Access.Evaluate(principal, permission) {
		writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return access.Principal{}, false
	}
	return principal, true
}

func authenticatedPrincipal(deps Dependencies, r *http.Request) (access.Principal, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return access.Principal{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return access.Principal{}, false
	}
	if deps.Config.OpsAPI.Token != "" && token == strings.TrimSpace(deps.Config.OpsAPI.Token) {
		return breakGlassPrincipal(deps, token), true
	}
	if deps.Access != nil {
		return deps.Access.AuthenticateSession(token)
	}
	return access.Principal{}, false
}

func breakGlassPrincipal(deps Dependencies, token string) access.Principal {
	user := access.User{UserID: "ops-admin", Username: "ops-admin", DisplayName: "Ops Admin", Status: "active", Source: "ops-token", Roles: []string{"platform_admin"}}
	if deps.Access != nil {
		if existing, ok := deps.Access.GetUser("ops-admin"); ok {
			user = existing
		}
	}
	permission := map[string]struct{}{"*": {}}
	return access.Principal{Kind: "ops_token", Token: token, User: &user, RoleIDs: append([]string(nil), user.Roles...), Permission: permission, Source: "ops-token"}
}

func routePermission(r *http.Request) string {
	path := r.URL.Path
	method := r.Method
	switch {
	case strings.HasPrefix(path, "/api/v1/connectors"):
		if method == http.MethodGet {
			return "connectors.read"
		}
		return "connectors.write"
	case strings.HasPrefix(path, "/api/v1/skills"):
		if method == http.MethodGet {
			return "skills.read"
		}
		return "skills.write"
	case strings.HasPrefix(path, "/api/v1/extensions"):
		if method == http.MethodGet {
			return "skills.read"
		}
		return "skills.write"
	case strings.HasPrefix(path, "/api/v1/automations"):
		if method == http.MethodGet {
			return "platform.read"
		}
		return "configs.write"
	case strings.HasPrefix(path, "/api/v1/config/connectors"):
		if method == http.MethodGet {
			return "configs.read"
		}
		return "configs.write"
	case strings.HasPrefix(path, "/api/v1/ssh-credentials"):
		if method == http.MethodGet {
			return "ssh_credentials.read"
		}
		return "ssh_credentials.write"
	case strings.HasPrefix(path, "/api/v1/config/skills"):
		return "configs.write"
	case strings.HasPrefix(path, "/api/v1/config/authorization"):
		if method == http.MethodGet {
			return "configs.read"
		}
		return "configs.write"
	case strings.HasPrefix(path, "/api/v1/config/approval-routing"):
		if method == http.MethodGet {
			return "configs.read"
		}
		return "configs.write"
	case strings.HasPrefix(path, "/api/v1/config/providers"):
		if method == http.MethodGet {
			return "configs.read"
		}
		return "configs.write"
	case strings.HasPrefix(path, "/api/v1/config/reasoning-prompts"):
		if method == http.MethodGet {
			return "configs.read"
		}
		return "configs.write"
	case strings.HasPrefix(path, "/api/v1/config/desensitization"):
		if method == http.MethodGet {
			return "configs.read"
		}
		return "configs.write"
	case strings.HasPrefix(path, "/api/v1/logs"):
		return "audit.read"
	case strings.HasPrefix(path, "/api/v1/observability"):
		return "platform.read"
	case strings.HasPrefix(path, "/api/v1/setup"):
		if method == http.MethodGet {
			return "platform.read"
		}
		return "platform.write"
	case strings.HasPrefix(path, "/api/v1/sessions"):
		if method == http.MethodGet {
			return "sessions.read"
		}
		return "sessions.write"
	case strings.HasPrefix(path, "/api/v1/executions"):
		if method == http.MethodGet {
			return "executions.read"
		}
		return "executions.write"
	case strings.HasPrefix(path, "/api/v1/audit"):
		if method == http.MethodGet {
			return "audit.read"
		}
		return "audit.write"
	case strings.HasPrefix(path, "/api/v1/knowledge"):
		if method == http.MethodGet {
			return "knowledge.read"
		}
		return "knowledge.write"
	case strings.HasPrefix(path, "/api/v1/outbox"):
		if method == http.MethodGet {
			return "outbox.read"
		}
		return "outbox.write"
	case strings.HasPrefix(path, "/api/v1/summary"):
		return "platform.read"
	case strings.HasPrefix(path, "/api/v1/reindex"):
		return "knowledge.write"
	default:
		return "platform.read"
	}
}

func requireSSHCredentialUseAccess(deps Dependencies, w http.ResponseWriter, r *http.Request, protocol string) bool {
	if !strings.EqualFold(strings.TrimSpace(protocol), "ssh_native") {
		return true
	}
	principal, ok := authenticatedPrincipal(deps, r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return false
	}
	if deps.Access != nil && !deps.Access.Evaluate(principal, "ssh_credentials.use") {
		writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return false
	}
	return true
}

func auditOpsRead(ctx context.Context, deps Dependencies, resourceType string, resourceID string, action string, metadata map[string]any) {
	if deps.Audit == nil {
		return
	}

	deps.Audit.Log(ctx, audit.Entry{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Actor:        "ops_api",
		TraceID:      metadataTraceID(metadata),
		Metadata:     metadata,
	})
}

func auditOpsWrite(ctx context.Context, deps Dependencies, resourceType string, resourceID string, action string, metadata map[string]any) {
	if deps.Audit == nil {
		return
	}

	deps.Audit.Log(ctx, audit.Entry{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Actor:        "ops_api",
		TraceID:      metadataTraceID(metadata),
		Metadata:     metadata,
	})
}

func principalAuditMetadata(principal access.Principal) map[string]any {
	metadata := map[string]any{
		"actor_source": strings.TrimSpace(principal.Source),
		"break_glass":  strings.EqualFold(strings.TrimSpace(principal.Source), "ops-token"),
	}
	if principal.User != nil {
		metadata["actor_user_id"] = strings.TrimSpace(principal.User.UserID)
	}
	return metadata
}

func metadataTraceID(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	for _, key := range []string{"trace_id", "session_id", "execution_id"} {
		if value, ok := metadata[key].(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func sessionDTO(in contracts.SessionDetail) dto.SessionDetail {
	out := dto.SessionDetail{
		SessionID:        in.SessionID,
		AgentRoleID:      in.AgentRoleID,
		Status:           in.Status,
		IsSmoke:          isSmokeAlert(in.Alert),
		DiagnosisSummary: in.DiagnosisSummary,
		GoldenSummary:    sessionGoldenSummaryDTO(in.GoldenSummary),
		ToolPlan:         toolPlanDTOs(in.ToolPlan),
		Attachments:      attachmentDTOs(in.Attachments),
		Alert:            in.Alert,
		Notifications:    notificationDigestDTOs(in.Notifications),
		Executions:       make([]dto.ExecutionDetail, 0, len(in.Executions)),
		Timeline:         make([]dto.TimelineEvent, 0, len(in.Timeline)),
	}
	if in.Verification != nil {
		out.Verification = &dto.SessionVerification{
			Status:    in.Verification.Status,
			Summary:   in.Verification.Summary,
			Details:   in.Verification.Details,
			Runtime:   runtimeMetadataDTO(in.Verification.Runtime),
			CheckedAt: in.Verification.CheckedAt,
		}
	}
	for _, execution := range in.Executions {
		out.Executions = append(out.Executions, executionDTO(execution))
	}
	for _, timeline := range in.Timeline {
		out.Timeline = append(out.Timeline, dto.TimelineEvent{
			Event:     timeline.Event,
			Message:   timeline.Message,
			CreatedAt: timeline.CreatedAt,
		})
	}
	return out
}

func sessionTraceDTO(sessionID string, auditRecords []audit.Record, knowledgeTrace *contracts.SessionKnowledgeTrace) dto.SessionTraceResponse {
	out := dto.SessionTraceResponse{
		SessionID:    sessionID,
		AuditEntries: make([]dto.AuditRecord, 0, len(auditRecords)),
	}
	for _, record := range auditRecords {
		out.AuditEntries = append(out.AuditEntries, dto.AuditRecord{
			ID:           record.ID,
			ResourceType: record.ResourceType,
			ResourceID:   record.ResourceID,
			Action:       record.Action,
			Actor:        record.Actor,
			TraceID:      record.TraceID,
			Metadata:     record.Metadata,
			CreatedAt:    record.CreatedAt,
		})
	}
	if knowledgeTrace != nil && knowledgeTrace.Available {
		out.Knowledge = &dto.SessionKnowledgeTrace{
			DocumentID:     knowledgeTrace.DocumentID,
			Title:          knowledgeTrace.Title,
			Summary:        knowledgeTrace.Summary,
			ContentPreview: knowledgeTrace.ContentPreview,
			Conversation:   append([]string(nil), knowledgeTrace.Conversation...),
			Runtime:        runtimeMetadataDTO(knowledgeTrace.Runtime),
			UpdatedAt:      knowledgeTrace.UpdatedAt,
		}
	}
	return out
}

func auditRecordDTO(record audit.Record) dto.AuditRecord {
	return dto.AuditRecord{
		ID:           record.ID,
		ResourceType: record.ResourceType,
		ResourceID:   record.ResourceID,
		Action:       record.Action,
		Actor:        record.Actor,
		TraceID:      record.TraceID,
		Metadata:     record.Metadata,
		CreatedAt:    record.CreatedAt,
	}
}

func automationJobDTO(job automations.Job, state automations.JobState) dto.AutomationJob {
	out := dto.AutomationJob{
		ID:                  job.ID,
		DisplayName:         job.DisplayName,
		Description:         job.Description,
		AgentRoleID:         job.AgentRoleID,
		GovernancePolicy:    job.GovernancePolicy,
		Type:                job.Type,
		TargetRef:           job.TargetRef,
		Schedule:            job.Schedule,
		Enabled:             job.Enabled,
		Owner:               job.Owner,
		RuntimeMode:         job.RuntimeMode,
		TimeoutSeconds:      job.TimeoutSeconds,
		RetryMaxAttempts:    job.RetryMaxAttempts,
		RetryInitialBackoff: job.RetryInitialBackoff,
		Labels:              cloneConnectorStringMap(job.Labels),
		State: &dto.AutomationJobState{
			Status:              state.Status,
			LastRunAt:           state.LastRunAt,
			NextRunAt:           state.NextRunAt,
			LastOutcome:         state.LastOutcome,
			LastError:           state.LastError,
			ConsecutiveFailures: state.ConsecutiveFailures,
			UpdatedAt:           state.UpdatedAt,
		},
	}
	if job.Skill != nil {
		out.Skill = &dto.AutomationSkillTarget{SkillID: job.Skill.SkillID, Context: cloneInterfaceMap(job.Skill.Context)}
	}
	if job.ConnectorCapability != nil {
		out.ConnectorCapability = &dto.AutomationConnectorCapability{
			ConnectorID:  job.ConnectorCapability.ConnectorID,
			CapabilityID: job.ConnectorCapability.CapabilityID,
			Params:       cloneInterfaceMap(job.ConnectorCapability.Params),
		}
	}
	if len(state.Runs) > 0 {
		out.State.Runs = make([]dto.AutomationRun, 0, len(state.Runs))
		for _, item := range state.Runs {
			out.State.Runs = append(out.State.Runs, automationRunDTO(item))
		}
		last := automationRunDTO(state.Runs[0])
		out.LastRun = &last
	}
	return out
}

func automationRunDTO(run automations.Run) dto.AutomationRun {
	return dto.AutomationRun{
		RunID:        run.RunID,
		JobID:        run.JobID,
		Trigger:      run.Trigger,
		Status:       run.Status,
		StartedAt:    run.StartedAt,
		CompletedAt:  run.CompletedAt,
		AttemptCount: run.AttemptCount,
		Summary:      run.Summary,
		Error:        run.Error,
		Metadata:     cloneInterfaceMap(run.Metadata),
	}
}

func cloneInterfaceMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		out[trimmed] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func knowledgeRecordDTO(item contracts.KnowledgeRecordDetail) dto.KnowledgeRecord {
	return dto.KnowledgeRecord{
		DocumentID: item.DocumentID,
		SessionID:  item.SessionID,
		Title:      item.Title,
		Summary:    item.Summary,
		UpdatedAt:  item.UpdatedAt,
	}
}

func executionDTO(in contracts.ExecutionDetail) dto.ExecutionDetail {
	return dto.ExecutionDetail{
		ExecutionID:      in.ExecutionID,
		SessionID:        in.SessionID,
		AgentRoleID:      in.AgentRoleID,
		RequestKind:      in.RequestKind,
		Status:           in.Status,
		RiskLevel:        in.RiskLevel,
		GoldenSummary:    executionGoldenSummaryDTO(in.GoldenSummary),
		Command:          in.Command,
		TargetHost:       in.TargetHost,
		StepID:           in.StepID,
		CapabilityID:     in.CapabilityID,
		CapabilityParams: in.CapabilityParams,
		ConnectorID:      in.ConnectorID,
		ConnectorType:    in.ConnectorType,
		ConnectorVendor:  in.ConnectorVendor,
		Protocol:         in.Protocol,
		ExecutionMode:    in.ExecutionMode,
		RequestedBy:      in.RequestedBy,
		ApprovalGroup:    in.ApprovalGroup,
		Runtime:          runtimeMetadataDTO(in.Runtime),
		ExitCode:         in.ExitCode,
		OutputRef:        in.OutputRef,
		OutputBytes:      in.OutputBytes,
		OutputTruncated:  in.OutputTruncated,
		CreatedAt:        in.CreatedAt,
		ApprovedAt:       in.ApprovedAt,
		CompletedAt:      in.CompletedAt,
	}
}

func sessionGoldenSummaryDTO(in *contracts.SessionGoldenSummary) *dto.SessionGoldenSummary {
	if in == nil {
		return nil
	}
	return &dto.SessionGoldenSummary{
		Headline:             in.Headline,
		Conclusion:           in.Conclusion,
		Risk:                 in.Risk,
		NextAction:           in.NextAction,
		Evidence:             append([]string(nil), in.Evidence...),
		NotificationHeadline: in.NotificationHeadline,
		ExecutionHeadline:    in.ExecutionHeadline,
		VerificationHeadline: in.VerificationHeadline,
	}
}

func notificationDigestDTOs(items []contracts.NotificationDigest) []dto.NotificationDigest {
	if len(items) == 0 {
		return nil
	}
	out := make([]dto.NotificationDigest, 0, len(items))
	for _, item := range items {
		out = append(out, dto.NotificationDigest{
			Stage:     item.Stage,
			Target:    item.Target,
			Reason:    item.Reason,
			Preview:   item.Preview,
			CreatedAt: item.CreatedAt,
		})
	}
	return out
}

func executionGoldenSummaryDTO(in *contracts.ExecutionGoldenSummary) *dto.ExecutionGoldenSummary {
	if in == nil {
		return nil
	}
	return &dto.ExecutionGoldenSummary{
		Headline:       in.Headline,
		Risk:           in.Risk,
		Approval:       in.Approval,
		Result:         in.Result,
		NextAction:     in.NextAction,
		CommandPreview: in.CommandPreview,
	}
}

func runtimeMetadataDTO(in *contracts.RuntimeMetadata) *dto.RuntimeMetadata {
	if in == nil {
		return nil
	}
	return &dto.RuntimeMetadata{
		Runtime:         in.Runtime,
		Selection:       in.Selection,
		ConnectorID:     in.ConnectorID,
		ConnectorType:   in.ConnectorType,
		ConnectorVendor: in.ConnectorVendor,
		Protocol:        in.Protocol,
		ExecutionMode:   in.ExecutionMode,
		RuntimeState:    runtimeStateValue(in),
		FallbackEnabled: in.FallbackEnabled,
		FallbackUsed:    in.FallbackUsed,
		FallbackReason:  in.FallbackReason,
		FallbackTarget:  in.FallbackTarget,
	}
}

func runtimeStateValue(in *contracts.RuntimeMetadata) string {
	if in == nil {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(in.Protocol), "stub") || strings.Contains(strings.ToLower(strings.TrimSpace(in.Runtime)), "stub") {
		return "stub"
	}
	if strings.TrimSpace(in.FallbackReason) != "" || in.FallbackUsed {
		return "degraded"
	}
	if strings.TrimSpace(in.ConnectorID) != "" || strings.TrimSpace(in.Protocol) != "" {
		return "real"
	}
	return ""
}

func executionOutputDTO(executionID string, chunks []contracts.ExecutionOutputChunk) dto.ExecutionOutputResponse {
	out := dto.ExecutionOutputResponse{
		ExecutionID: executionID,
		Chunks:      make([]dto.ExecutionOutputChunk, 0, len(chunks)),
	}
	for _, chunk := range chunks {
		out.Chunks = append(out.Chunks, dto.ExecutionOutputChunk{
			Seq:        chunk.Seq,
			StreamType: chunk.StreamType,
			Content:    chunk.Content,
			ByteSize:   chunk.ByteSize,
			CreatedAt:  chunk.CreatedAt,
		})
	}
	return out
}

func toolPlanDTOs(items []contracts.ToolPlanStep) []dto.ToolPlanStep {
	if len(items) == 0 {
		return nil
	}
	out := make([]dto.ToolPlanStep, 0, len(items))
	for _, item := range items {
		out = append(out, dto.ToolPlanStep{
			ID:                item.ID,
			Tool:              item.Tool,
			ConnectorID:       item.ConnectorID,
			Reason:            item.Reason,
			Priority:          item.Priority,
			Status:            item.Status,
			Input:             item.Input,
			ResolvedInput:     item.ResolvedInput,
			Output:            item.Output,
			OnFailure:         item.OnFailure,
			OnPendingApproval: item.OnPendingApproval,
			OnDenied:          item.OnDenied,
			Runtime:           runtimeMetadataDTO(item.Runtime),
			StartedAt:         item.StartedAt,
			CompletedAt:       item.CompletedAt,
		})
	}
	return out
}

func attachmentDTOs(items []contracts.MessageAttachment) []dto.MessageAttachment {
	if len(items) == 0 {
		return nil
	}
	out := make([]dto.MessageAttachment, 0, len(items))
	for _, item := range items {
		out = append(out, dto.MessageAttachment{
			Type:        item.Type,
			Name:        item.Name,
			MimeType:    item.MimeType,
			URL:         item.URL,
			Content:     item.Content,
			PreviewText: item.PreviewText,
			Metadata:    item.Metadata,
		})
	}
	return out
}

func outboxDTO(in contracts.OutboxEvent) dto.OutboxEvent {
	return dto.OutboxEvent{
		ID:            in.ID,
		Topic:         in.Topic,
		Status:        in.Status,
		AggregateID:   in.AggregateID,
		RetryCount:    in.RetryCount,
		LastError:     in.LastError,
		BlockedReason: in.BlockedReason,
		CreatedAt:     in.CreatedAt,
	}
}

func isSmokeAlert(alert map[string]interface{}) bool {
	return alertLabelValue(alert, "tars_smoke") == "true"
}

func alertLabelValue(alert map[string]interface{}, key string) string {
	labels, ok := alert["labels"]
	if !ok {
		return ""
	}

	switch typed := labels.(type) {
	case map[string]string:
		return strings.TrimSpace(typed[key])
	case map[string]interface{}:
		if value, ok := typed[key].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func alertStringValue(alert map[string]interface{}, key string) string {
	if value, ok := alert[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}
