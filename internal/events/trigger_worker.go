// Package events provides the TriggerWorker which fires platform triggers
// when named events occur in the system.
//
// TriggerWorker exposes FireEvent() which is called synchronously by the
// dispatcher (or other workers) when a meaningful event occurs.
//
// Supported trigger event types:
//   - on_approval_requested  → fires when a capability/execution approval is created
//   - on_execution_completed → fires when an execution finishes
//   - on_execution_failed    → fires when an execution fails
//   - on_skill_completed     → fires when a skill run completes
//   - on_skill_failed        → fires when a skill run fails
//   - on_session_closed      → fires when a session is resolved/closed
package events

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"tars/internal/contracts"
	"tars/internal/modules/access"
	"tars/internal/modules/msgtpl"
	"tars/internal/modules/trigger"
)

// TriggerWorker fires platform triggers when named platform events occur.
type TriggerWorker struct {
	logger    *slog.Logger
	channel   contracts.ChannelService
	triggers  *trigger.Manager
	templates *msgtpl.Manager
	access    triggerChannelResolver
}

type triggerChannelResolver interface {
	GetChannel(id string) (access.Channel, bool)
}

// NewTriggerWorker creates a TriggerWorker.
func NewTriggerWorker(
	logger *slog.Logger,
	channel contracts.ChannelService,
	triggers *trigger.Manager,
	templates *msgtpl.Manager,
	accessResolver triggerChannelResolver,
) *TriggerWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &TriggerWorker{
		logger:    logger,
		channel:   channel,
		triggers:  triggers,
		templates: templates,
		access:    accessResolver,
	}
}

// FireEvent fires all enabled triggers that match the given event type.
// It is safe to call from multiple goroutines.
func (w *TriggerWorker) FireEvent(ctx context.Context, evt trigger.FireEvent) {
	if w.triggers == nil || w.channel == nil {
		return
	}
	tenantID := evt.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	matched, err := w.triggers.MatchEnabled(ctx, trigger.MatchRequest{
		TenantID:       tenantID,
		EventType:      evt.EventType,
		TargetAudience: strings.TrimSpace(evt.TemplateData["target_audience"]),
		EventData:      buildTriggerEventData(evt),
	})
	if err != nil {
		w.logger.Error("trigger worker: match enabled failed",
			"event_type", evt.EventType,
			"error", err,
		)
		return
	}

	now := time.Now().UTC()
	for _, t := range matched {
		if trigger.IsOnCooldown(t, now) {
			w.logger.Debug("trigger on cooldown, skipping",
				"trigger_id", t.ID,
				"event_type", evt.EventType,
			)
			continue
		}
		subject := evt.Subject
		if subject == "" {
			subject = humanEventType(evt.EventType)
		}
		body := evt.Body

		// If the trigger has a TemplateID, render subject/body via the template.
		if t.TemplateID != "" && w.templates != nil {
			if tpl, ok := w.templates.Get(t.TemplateID); ok && tpl.Enabled {
				rendered, renderedBody := tpl.Render(evt.TemplateData)
				if rendered != "" {
					subject = rendered
				}
				if renderedBody != "" {
					body = renderedBody
				}
			} else if !ok {
				w.logger.Warn("trigger template not found, using default message",
					"trigger_id", t.ID,
					"template_id", t.TemplateID,
				)
			}
		}

		resolvedChannel, resolvedTarget := resolvedTriggerDelivery(w.access, t)
		if strings.TrimSpace(resolvedChannel) == "" {
			w.logger.Error("trigger worker: unsupported or unresolved channel delivery",
				"trigger_id", t.ID,
				"event_type", evt.EventType,
				"channel_id", t.ChannelID,
			)
			continue
		}
		msg := contracts.ChannelMessage{
			Channel: resolvedChannel,
			Target:  resolvedTarget,
			Subject: subject,
			Body:    body,
			RefType: evt.RefType,
			RefID:   evt.RefID,
			Source:  triggerSource(evt.Source, "trigger:"+t.ID),
		}
		if _, sendErr := w.channel.SendMessage(ctx, msg); sendErr != nil {
			w.logger.Error("trigger worker: send message failed",
				"trigger_id", t.ID,
				"event_type", evt.EventType,
				"error", sendErr,
			)
			continue
		}
		_ = w.triggers.RecordFired(ctx, t.ID)
		w.logger.Info("trigger fired",
			"trigger_id", t.ID,
			"event_type", evt.EventType,
			"channel_id", t.ChannelID,
		)
	}
}

func resolvedTriggerChannel(t trigger.Trigger) string {
	if trigger.IsDirectDeliveryChannelKind(t.ChannelID) {
		return strings.TrimSpace(t.ChannelID)
	}
	return ""
}

func resolvedTriggerDelivery(resolver triggerChannelResolver, t trigger.Trigger) (string, string) {
	channel := resolvedTriggerChannel(t)
	target := ""
	channelID := strings.TrimSpace(t.ChannelID)
	if resolver != nil && channelID != "" {
		if item, ok := resolver.GetChannel(channelID); ok {
			if trigger.IsDirectDeliveryChannelKind(item.Kind) {
				channel = strings.TrimSpace(item.Kind)
			}
			target = strings.TrimSpace(item.Target)
		}
	}
	return channel, target
}

// FireApprovalRequested is a convenience method to fire on_approval_requested.
func (w *TriggerWorker) FireApprovalRequested(ctx context.Context, exec contracts.ExecutionDetail) {
	w.FireEvent(ctx, trigger.FireEvent{
		EventType:    trigger.EventOnApprovalRequested,
		TenantID:     "default",
		RefType:      "execution",
		RefID:        exec.ExecutionID,
		Subject:      "审批请求",
		Body:         formatApprovalMessage(exec),
		Source:       "dispatcher",
		TemplateData: buildExecutionTemplateData(exec),
	})
}

// FireExecutionCompleted fires on_execution_completed or on_execution_failed.
func (w *TriggerWorker) FireExecutionCompleted(ctx context.Context, exec contracts.ExecutionDetail) {
	eventType := trigger.EventOnExecutionCompleted
	subject := "命令执行完成"
	if exec.Status == "failed" || exec.Status == "timeout" || exec.Status == "rejected" {
		eventType = trigger.EventOnExecutionFailed
		subject = "命令执行失败"
	}
	w.FireEvent(ctx, trigger.FireEvent{
		EventType:    eventType,
		TenantID:     "default",
		RefType:      "execution",
		RefID:        exec.ExecutionID,
		Subject:      subject,
		Body:         formatExecutionCompletedMessage(exec),
		Source:       "dispatcher",
		TemplateData: buildExecutionTemplateData(exec),
	})
}

// FireSessionClosed fires on_session_closed.
func (w *TriggerWorker) FireSessionClosed(ctx context.Context, sessionID string, summary string) {
	body := summary
	if body == "" {
		body = fmt.Sprintf("会话 %s 已关闭。", sessionID)
	}
	w.FireEvent(ctx, trigger.FireEvent{
		EventType: trigger.EventOnSessionClosed,
		TenantID:  "default",
		RefType:   "session",
		RefID:     sessionID,
		Subject:   "会话已关闭",
		Body:      body,
		Source:    "dispatcher",
		TemplateData: map[string]string{
			"session_id": sessionID,
		},
	})
}

// FireSkillEvent fires on_skill_completed or on_skill_failed based on the execution status.
func (w *TriggerWorker) FireSkillEvent(ctx context.Context, exec contracts.ExecutionDetail) {
	eventType := trigger.EventOnSkillCompleted
	subject := "Skill 执行完成"
	statusLabel := "完成"
	if exec.Status == "failed" || exec.Status == "timeout" {
		eventType = trigger.EventOnSkillFailed
		subject = "Skill 执行失败"
		statusLabel = "失败"
	}
	w.FireEvent(ctx, trigger.FireEvent{
		EventType: eventType,
		TenantID:  "default",
		RefType:   "session",
		RefID:     exec.ExecutionID,
		Subject:   subject,
		Body:      formatSkillEventMessage(exec),
		Source:    "dispatcher",
		TemplateData: mergeTemplateData(buildExecutionTemplateData(exec), map[string]string{
			"ExecutionID":     exec.ExecutionID,
			"TargetHost":      exec.TargetHost,
			"ExitCode":        fmt.Sprintf("%d", exec.ExitCode),
			"ExecutionStatus": statusLabel,
			"OutputPreview":   "",
			"TruncationFlag":  "",
			"ActionTip":       "",
			"SessionID":       exec.SessionID,
		}),
	})
}

// --- helpers ---

func formatSkillEventMessage(exec contracts.ExecutionDetail) string {
	host := exec.TargetHost
	if host == "" {
		host = "(unknown)"
	}
	return fmt.Sprintf(
		"[TARS] Skill%s\n会话ID: %s\n主机: %s\n状态: %s",
		statusLabel(exec.Status), exec.ExecutionID, host, exec.Status,
	)
}

func formatApprovalMessage(exec contracts.ExecutionDetail) string {
	host := exec.TargetHost
	if host == "" {
		host = "(unknown)"
	}
	cmd := exec.Command
	if len(cmd) > 120 {
		cmd = cmd[:120] + "..."
	}
	risk := exec.RiskLevel
	if risk == "" {
		risk = "unknown"
	}
	return fmt.Sprintf(
		"[TARS] 审批请求\n执行ID: %s\n主机: %s\n命令: %s\n风险等级: %s\n请进入控制台审批。",
		exec.ExecutionID, host, cmd, risk,
	)
}

func formatExecutionCompletedMessage(exec contracts.ExecutionDetail) string {
	host := exec.TargetHost
	if host == "" {
		host = "(unknown)"
	}
	return fmt.Sprintf(
		"[TARS] 执行%s\n执行ID: %s\n主机: %s\n状态: %s\n退出码: %d",
		statusLabel(exec.Status), exec.ExecutionID, host, exec.Status, exec.ExitCode,
	)
}

func statusLabel(status string) string {
	switch status {
	case "completed":
		return "完成"
	case "failed":
		return "失败"
	case "timeout":
		return "超时"
	case "rejected":
		return "已拒绝"
	default:
		return ""
	}
}

func humanEventType(eventType string) string {
	switch eventType {
	case trigger.EventOnSkillCompleted:
		return "Skill 执行完成"
	case trigger.EventOnSkillFailed:
		return "Skill 执行失败"
	case trigger.EventOnExecutionCompleted:
		return "命令执行完成"
	case trigger.EventOnExecutionFailed:
		return "命令执行失败"
	case trigger.EventOnApprovalRequested:
		return "审批请求"
	case trigger.EventOnSessionClosed:
		return "会话已关闭"
	default:
		return eventType
	}
}

func triggerSource(primary, def string) string {
	if primary != "" {
		return primary
	}
	return def
}

func buildTriggerEventData(evt trigger.FireEvent) map[string]string {
	data := map[string]string{
		"event_type": evt.EventType,
		"tenant_id":  evt.TenantID,
		"ref_type":   evt.RefType,
		"ref_id":     evt.RefID,
		"subject":    evt.Subject,
		"body":       evt.Body,
		"source":     evt.Source,
	}
	for key, value := range evt.TemplateData {
		data[key] = value
	}
	return data
}

func buildExecutionTemplateData(exec contracts.ExecutionDetail) map[string]string {
	return map[string]string{
		"execution_id":     exec.ExecutionID,
		"session_id":       exec.SessionID,
		"agent_role_id":    exec.AgentRoleID,
		"request_kind":     exec.RequestKind,
		"status":           exec.Status,
		"risk_level":       exec.RiskLevel,
		"command":          exec.Command,
		"target_host":      exec.TargetHost,
		"step_id":          exec.StepID,
		"capability_id":    exec.CapabilityID,
		"connector_id":     exec.ConnectorID,
		"connector_type":   exec.ConnectorType,
		"connector_vendor": exec.ConnectorVendor,
		"protocol":         exec.Protocol,
		"execution_mode":   exec.ExecutionMode,
		"requested_by":     exec.RequestedBy,
		"approval_group":   exec.ApprovalGroup,
		"exit_code":        fmt.Sprintf("%d", exec.ExitCode),
	}
}

func mergeTemplateData(parts ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, part := range parts {
		for key, value := range part {
			out[key] = value
		}
	}
	return out
}
