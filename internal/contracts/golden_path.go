package contracts

import (
	"fmt"
	"strings"
	"time"
)

type SessionGoldenSummary struct {
	Headline             string   `json:"headline,omitempty"`
	Conclusion           string   `json:"conclusion,omitempty"`
	Risk                 string   `json:"risk,omitempty"`
	NextAction           string   `json:"next_action,omitempty"`
	Evidence             []string `json:"evidence,omitempty"`
	NotificationHeadline string   `json:"notification_headline,omitempty"`
	ExecutionHeadline    string   `json:"execution_headline,omitempty"`
	VerificationHeadline string   `json:"verification_headline,omitempty"`
}

type NotificationDigest struct {
	Stage     string    `json:"stage,omitempty"`
	Target    string    `json:"target,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Preview   string    `json:"preview,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type ExecutionGoldenSummary struct {
	Headline       string `json:"headline,omitempty"`
	Risk           string `json:"risk,omitempty"`
	Approval       string `json:"approval,omitempty"`
	Result         string `json:"result,omitempty"`
	NextAction     string `json:"next_action,omitempty"`
	CommandPreview string `json:"command_preview,omitempty"`
}

func PopulateSessionGoldenPath(detail *SessionDetail) {
	if detail == nil {
		return
	}
	detail.Notifications = BuildNotificationDigests(detail.Timeline)
	for i := range detail.Executions {
		detail.Executions[i].SessionID = detail.SessionID
		PopulateExecutionGoldenPath(&detail.Executions[i], detail)
	}
	detail.GoldenSummary = buildSessionGoldenSummary(*detail)
}

func PopulateExecutionGoldenPath(detail *ExecutionDetail, session *SessionDetail) {
	if detail == nil {
		return
	}
	detail.GoldenSummary = buildExecutionGoldenSummary(*detail, session)
}

func BuildNotificationDigests(timeline []TimelineEvent) []NotificationDigest {
	items := make([]NotificationDigest, 0, 4)
	for _, event := range timeline {
		stage, reason := notificationStageAndReason(event.Event)
		if stage == "" {
			continue
		}
		target, preview := parseNotificationTimelineMessage(event.Message)
		items = append(items, NotificationDigest{
			Stage:     stage,
			Target:    target,
			Reason:    reason,
			Preview:   preview,
			CreatedAt: event.CreatedAt,
		})
	}
	return items
}

func buildSessionGoldenSummary(detail SessionDetail) *SessionGoldenSummary {
	headline := sessionHeadline(detail)
	conclusion := firstNonEmpty(
		extractSummarySection(detail.DiagnosisSummary, []string{"Conclusion", "Root Cause", "结论"}),
		extractSummarySection(detail.DiagnosisSummary, []string{"Summary", "摘要"}),
		firstMeaningfulLine(detail.DiagnosisSummary),
		summaryFromVerification(detail.Verification),
		"诊断进行中",
	)
	evidence := sessionEvidence(detail)
	risk := sessionRisk(detail)
	nextAction := sessionNextAction(detail)
	verificationHeadline := summaryFromVerification(detail.Verification)
	executionHeadline := latestExecutionHeadline(detail.Executions)
	notificationHeadline := notificationHeadline(detail.Notifications)
	return &SessionGoldenSummary{
		Headline:             headline,
		Conclusion:           conclusion,
		Risk:                 risk,
		NextAction:           nextAction,
		Evidence:             evidence,
		NotificationHeadline: notificationHeadline,
		ExecutionHeadline:    executionHeadline,
		VerificationHeadline: verificationHeadline,
	}
}

func buildExecutionGoldenSummary(detail ExecutionDetail, session *SessionDetail) *ExecutionGoldenSummary {
	headline := executionHeadline(detail, session)
	approval := executionApprovalSummary(detail)
	result := executionResultSummary(detail, session)
	nextAction := executionNextAction(detail, session)
	return &ExecutionGoldenSummary{
		Headline:       headline,
		Risk:           firstNonEmpty(strings.TrimSpace(detail.RiskLevel), "info"),
		Approval:       approval,
		Result:         result,
		NextAction:     nextAction,
		CommandPreview: executionCommandPreview(detail),
	}
}

func sessionHeadline(detail SessionDetail) string {
	if source := alertString(detail.Alert, "source"); source == "telegram_chat" {
		request := alertAnnotation(detail.Alert, "user_request")
		if request == "" {
			request = alertAnnotation(detail.Alert, "summary")
		}
		if request != "" {
			return compactText(request, 96)
		}
	}
	alertName := firstNonEmpty(alertLabel(detail.Alert, "alertname"), "unknown-alert")
	host := firstNonEmpty(alertString(detail.Alert, "host"), alertLabel(detail.Alert, "instance"), "unknown-host")
	return fmt.Sprintf("%s @ %s", alertName, host)
}

func sessionEvidence(detail SessionDetail) []string {
	items := make([]string, 0, 3)
	if text := extractSummarySection(detail.DiagnosisSummary, []string{"Evidence", "Observed", "证据", "观察"}); text != "" {
		items = append(items, compactText(text, 140))
	}
	for _, step := range detail.ToolPlan {
		if len(items) >= 3 {
			break
		}
		reason := compactText(firstNonEmpty(step.Reason, step.Tool), 110)
		status := firstNonEmpty(strings.TrimSpace(step.Status), "pending")
		items = append(items, fmt.Sprintf("%s (%s)", reason, status))
	}
	if detail.Verification != nil && len(items) < 3 {
		items = append(items, compactText(detail.Verification.Summary, 120))
	}
	if len(detail.Attachments) > 0 && len(items) < 3 {
		items = append(items, fmt.Sprintf("已生成 %d 个附件供回放与验收", len(detail.Attachments)))
	}
	if len(items) == 0 {
		items = append(items, "证据仍在收集中")
	}
	return items
}

func sessionRisk(detail SessionDetail) string {
	for i := len(detail.Executions) - 1; i >= 0; i-- {
		if risk := strings.TrimSpace(detail.Executions[i].RiskLevel); risk != "" {
			return risk
		}
	}
	return severityToRisk(alertString(detail.Alert, "severity"))
}

func sessionNextAction(detail SessionDetail) string {
	latestExecution := latestExecution(detail.Executions)
	switch detail.Status {
	case "pending_approval":
		if latestExecution != nil {
			return fmt.Sprintf("等待审批，目标命令：%s", executionCommandPreview(*latestExecution))
		}
		return "等待审批人确认是否执行"
	case "executing":
		if latestExecution != nil {
			return fmt.Sprintf("等待执行完成：%s", executionCommandPreview(*latestExecution))
		}
		return "等待执行完成并回传结果"
	case "verifying":
		return "等待校验结果，确认故障是否恢复"
	case "resolved":
		return "闭环完成，可直接回放证据与执行结果"
	case "failed":
		if latestExecution != nil {
			return fmt.Sprintf("查看失败执行并决定是否重试：%s", executionCommandPreview(*latestExecution))
		}
		return "查看失败原因并补充诊断证据"
	case "open":
		if latestExecution != nil && latestExecution.Status == "rejected" {
			return "审批被拒绝，建议补充上下文后重新分析"
		}
		return "继续观察诊断结论，必要时手动推进"
	default:
		return "等待诊断与证据收口"
	}
}

func latestExecutionHeadline(items []ExecutionDetail) string {
	latest := latestExecution(items)
	if latest == nil {
		return "当前未生成执行动作"
	}
	return fmt.Sprintf("最新动作 %s：%s", latest.Status, executionCommandPreview(*latest))
}

func notificationHeadline(items []NotificationDigest) string {
	if len(items) == 0 {
		return "当前未生成通知消息"
	}
	latest := items[len(items)-1]
	parts := []string{fmt.Sprintf("最近通知 %s", latest.Reason)}
	if latest.Target != "" {
		parts = append(parts, fmt.Sprintf("目标 %s", latest.Target))
	}
	return strings.Join(parts, "，")
}

func executionHeadline(detail ExecutionDetail, session *SessionDetail) string {
	prefix := "执行"
	if detail.RequestKind == "capability" {
		prefix = "能力调用"
	}
	target := firstNonEmpty(strings.TrimSpace(detail.TargetHost), executionTargetFromSession(session), "unknown-target")
	return fmt.Sprintf("%s %s @ %s", prefix, executionCommandPreview(detail), target)
}

func executionApprovalSummary(detail ExecutionDetail) string {
	switch detail.Status {
	case "pending":
		return fmt.Sprintf("待审批，审批组 %s", firstNonEmpty(strings.TrimSpace(detail.ApprovalGroup), "未指定"))
	case "approved", "executing", "completed", "failed", "timeout":
		if !detail.ApprovedAt.IsZero() {
			return fmt.Sprintf("已审批，时间 %s", detail.ApprovedAt.UTC().Format(time.RFC3339))
		}
		if strings.TrimSpace(detail.ApprovalGroup) != "" {
			return fmt.Sprintf("按审批组 %s 进入执行链路", detail.ApprovalGroup)
		}
	case "rejected":
		return "审批未通过"
	}
	return "未触发审批信息"
}

func executionResultSummary(detail ExecutionDetail, session *SessionDetail) string {
	switch detail.Status {
	case "completed":
		if session != nil && session.Verification != nil {
			return fmt.Sprintf("执行完成，校验状态 %s", session.Verification.Status)
		}
		return fmt.Sprintf("执行完成，退出码 %d，输出 %d bytes", detail.ExitCode, detail.OutputBytes)
	case "failed":
		return fmt.Sprintf("执行失败，退出码 %d", detail.ExitCode)
	case "timeout":
		return "执行超时，需人工确认目标侧状态"
	case "executing":
		return "执行中，等待结果回传"
	case "pending":
		return "尚未开始执行"
	case "rejected":
		return "审批拒绝，未执行"
	default:
		return fmt.Sprintf("当前状态 %s", firstNonEmpty(strings.TrimSpace(detail.Status), "unknown"))
	}
}

func executionNextAction(detail ExecutionDetail, session *SessionDetail) string {
	switch detail.Status {
	case "pending":
		return "在 Telegram / Inbox 中完成审批或拒绝"
	case "approved", "executing":
		return "等待执行完成并查看输出"
	case "completed":
		if session != nil {
			switch session.Status {
			case "verifying":
				return "等待校验步骤确认是否恢复"
			case "resolved":
				return "会话已闭环，可回放输出与校验结果"
			}
		}
		return "查看输出并确认是否需要继续校验"
	case "failed", "timeout":
		if strings.TrimSpace(detail.OutputRef) != "" {
			return fmt.Sprintf("查看输出引用并决定是否重试：%s", detail.OutputRef)
		}
		return "查看失败输出并决定是否重试"
	case "rejected":
		return "审批被拒绝，必要时回到会话补充证据"
	default:
		return "查看当前状态并按会话链路继续推进"
	}
}

func executionCommandPreview(detail ExecutionDetail) string {
	if detail.RequestKind == "capability" {
		return compactText(firstNonEmpty(detail.CapabilityID, detail.ConnectorID+" capability"), 96)
	}
	return compactText(firstNonEmpty(detail.Command, "未记录命令"), 96)
}

func latestExecution(items []ExecutionDetail) *ExecutionDetail {
	if len(items) == 0 {
		return nil
	}
	latest := items[len(items)-1]
	return &latest
}

func summaryFromVerification(verification *SessionVerification) string {
	if verification == nil {
		return ""
	}
	return compactText(strings.TrimSpace(verification.Summary), 140)
}

func executionTargetFromSession(session *SessionDetail) string {
	if session == nil {
		return ""
	}
	return firstNonEmpty(alertString(session.Alert, "host"), alertLabel(session.Alert, "instance"))
}

func extractSummarySection(body string, headers []string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for i, line := range lines {
		normalized := normalizeHeaderLine(line)
		for _, header := range headers {
			if normalized != normalizeHeaderText(header) {
				continue
			}
			collected := make([]string, 0, 2)
			for j := i + 1; j < len(lines); j++ {
				candidate := strings.TrimSpace(lines[j])
				if candidate == "" {
					if len(collected) > 0 {
						break
					}
					continue
				}
				if isHeaderLine(candidate) {
					break
				}
				collected = append(collected, strings.TrimLeft(candidate, "-*0123456789. "))
				if len(collected) >= 2 {
					break
				}
			}
			return compactText(strings.Join(collected, " "), 160)
		}
	}
	return ""
}

func firstMeaningfulLine(body string) string {
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isHeaderLine(trimmed) {
			continue
		}
		trimmed = strings.Trim(trimmed, "*#`>")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			return compactText(trimmed, 160)
		}
	}
	return ""
}

func parseNotificationTimelineMessage(message string) (string, string) {
	target := extractKV(message, "target=")
	body := extractKV(message, "body=")
	if body == "" {
		body = strings.TrimSpace(message)
	}
	return target, compactText(body, 160)
}

func extractKV(body string, prefix string) string {
	idx := strings.Index(body, prefix)
	if idx == -1 {
		return ""
	}
	value := body[idx+len(prefix):]
	for _, marker := range []string{" body=", " target="} {
		if marker == " "+prefix {
			continue
		}
		if next := strings.Index(value, marker); next >= 0 {
			value = value[:next]
		}
	}
	return strings.TrimSpace(value)
}

func notificationStageAndReason(eventType string) (string, string) {
	switch strings.TrimSpace(eventType) {
	case "diagnosis_message_prepared":
		return "diagnosis", "发送诊断结论"
	case "approval_message_prepared":
		return "approval", "请求人工审批"
	default:
		return "", ""
	}
}

func severityToRisk(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "fatal", "page":
		return "critical"
	case "warning", "warn":
		return "warning"
	default:
		return "info"
	}
}

func alertString(alert map[string]interface{}, key string) string {
	if value, ok := alert[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func alertLabel(alert map[string]interface{}, key string) string {
	labels, ok := alert["labels"].(map[string]interface{})
	if !ok {
		return ""
	}
	if value, ok := labels[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func alertAnnotation(alert map[string]interface{}, key string) string {
	annotations, ok := alert["annotations"].(map[string]interface{})
	if !ok {
		return ""
	}
	if value, ok := annotations[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func compactText(value string, limit int) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed
	}
	if limit <= 3 {
		return trimmed[:limit]
	}
	return trimmed[:limit-3] + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeHeaderLine(line string) string {
	trimmed := strings.TrimSpace(strings.TrimLeft(line, "#* "))
	trimmed = strings.TrimSuffix(trimmed, ":")
	return normalizeHeaderText(trimmed)
}

func normalizeHeaderText(line string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(line)), " "))
}

func isHeaderLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "###") || strings.HasPrefix(trimmed, "##") {
		return true
	}
	if strings.HasPrefix(trimmed, "**") && strings.HasSuffix(trimmed, "**") {
		return true
	}
	normalized := normalizeHeaderLine(trimmed)
	for _, candidate := range []string{"conclusion", "root cause", "summary", "evidence", "observed", "recommended action", "next steps", "结论", "证据", "观察", "建议", "下一步", "摘要"} {
		if normalized == normalizeHeaderText(candidate) {
			return true
		}
	}
	return false
}
