package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/contracts"
	"tars/internal/foundation/audit"
)

var errTelegramValidation = errors.New("telegram_validation")

type telegramCallbackAcker interface {
	AnswerCallbackQuery(ctx context.Context, callbackID string, text string) error
}

type telegramUpdate struct {
	UpdateID      int64                  `json:"update_id"`
	Message       map[string]interface{} `json:"message"`
	CallbackQuery *telegramCallbackQuery `json:"callback_query"`
}

type telegramCallbackQuery struct {
	ID      string                 `json:"id"`
	Data    string                 `json:"data"`
	From    map[string]interface{} `json:"from"`
	Message map[string]interface{} `json:"message"`
}

func telegramHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if secret := strings.TrimSpace(deps.Config.Telegram.WebhookSecret); secret != "" {
			if r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != secret {
				writeError(w, http.StatusUnauthorized, "invalid_signature", "telegram webhook secret verification failed")
				return
			}
		}

		var update telegramUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
			return
		}

		if err := processTelegramUpdate(r.Context(), deps, update); err != nil {
			switch {
			case errors.Is(err, errTelegramValidation):
				writeError(w, http.StatusBadRequest, "validation_failed", strings.TrimPrefix(err.Error(), errTelegramValidation.Error()+": "))
			case errors.Is(err, contracts.ErrNotFound):
				writeError(w, http.StatusNotFound, "not_found", "execution not found")
			case errors.Is(err, contracts.ErrInvalidState):
				writeError(w, http.StatusConflict, "invalid_state", "execution is no longer pending approval")
			default:
				writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			}
			return
		}

		writeJSON(w, http.StatusOK, dto.TelegramWebhookResponse{Accepted: true})
	}
}

func ProcessTelegramUpdatePayload(ctx context.Context, deps Dependencies, rawUpdate []byte) error {
	var update telegramUpdate
	if err := json.Unmarshal(rawUpdate, &update); err != nil {
		return fmt.Errorf("%w: invalid request body", errTelegramValidation)
	}
	return processTelegramUpdate(ctx, deps, update)
}

func processTelegramUpdate(ctx context.Context, deps Dependencies, update telegramUpdate) error {
	if update.CallbackQuery != nil {
		var ackText string
		ctx = context.WithValue(ctx, telegramCallbackAckKey{}, &ackText)
		defer acknowledgeTelegramCallback(ctx, deps, update)
	}
	if update.CallbackQuery == nil && update.Message == nil {
		return nil
	}
	if update.CallbackQuery == nil {
		return handleTelegramMessage(ctx, deps, update)
	}

	channelEvent, err := parseTelegramCallback(update)
	if err != nil {
		setTelegramCallbackAckText(ctx, "回调格式无效")
		return fmt.Errorf("%w: %s", errTelegramValidation, err.Error())
	}
	auditTelegramEvent(ctx, deps, "telegram_callback", fmt.Sprintf("telegram_update:%d", update.UpdateID), "receive", channelEvent.UserID, map[string]any{
		"update_id":    update.UpdateID,
		"chat_id":      channelEvent.ChatID,
		"channel":      channelEvent.Channel,
		"action":       channelEvent.Action,
		"execution_id": channelEvent.ExecutionID,
		"command":      channelEvent.Command,
	})

	dispatchResult, err := deps.Workflow.HandleChannelEvent(ctx, channelEvent)
	if err != nil {
		setTelegramCallbackAckText(ctx, telegramCallbackErrorText(err))
		return err
	}
	setTelegramCallbackAckText(ctx, telegramCallbackSuccessText(channelEvent.Action))

	if outcome, outcomeErr := processApprovalDispatch(ctx, deps, channelEvent, dispatchResult); outcomeErr != nil {
		setTelegramCallbackAckText(ctx, "处理失败，请稍后重试")
		return outcomeErr
	} else {
		switch {
		case outcome.ExecutionFailure != nil:
			setTelegramCallbackAckText(ctx, "执行失败，请查看结果消息")
			return outcome.ExecutionFailure
		case outcome.CapabilityFailure != nil:
			setTelegramCallbackAckText(ctx, "能力执行失败，请查看会话详情")
			return outcome.CapabilityFailure
		case outcome.ResultNotificationQueued:
			setTelegramCallbackAckText(ctx, "执行已处理，结果通知将重试发送")
		case outcome.CapabilityNotificationQueued:
			setTelegramCallbackAckText(ctx, "能力已处理，但结果通知发送失败")
		}
	}

	return nil
}

func handleTelegramMessage(ctx context.Context, deps Dependencies, update telegramUpdate) error {
	request, guidance := parseTelegramConversationRequest(update, deps.Config.SSH.AllowedHosts)
	if guidance != nil {
		if guidance.Target == "" {
			return nil
		}
		auditTelegramEvent(ctx, deps, "telegram_chat", fmt.Sprintf("telegram_update:%d", update.UpdateID), "guidance", "", map[string]any{
			"update_id":    update.UpdateID,
			"target":       guidance.Target,
			"body_preview": compactTelegramBody(guidance.Body, 240),
		})
		return deliverNotifications(ctx, deps.Channel, deps.Workflow, "", []contracts.ChannelMessage{*guidance})
	}
	if request == nil {
		return nil
	}

	event := contracts.AlertEvent{
		Source:         "telegram_chat",
		Severity:       request.Severity,
		Fingerprint:    fmt.Sprintf("telegram-chat:%d", update.UpdateID),
		IdempotencyKey: fmt.Sprintf("telegram_update:%d", update.UpdateID),
		RequestHash:    fmt.Sprintf("telegram_update:%d", update.UpdateID),
		Labels: map[string]string{
			"alertname":      request.AlertName,
			"instance":       request.Host,
			"host":           request.Host,
			"service":        request.Service,
			"severity":       request.Severity,
			"tars_chat":      "true",
			"tars_generated": "telegram_chat",
			"chat_id":        request.ChatID,
		},
		Annotations: map[string]string{
			"summary":      request.UserRequest,
			"user_request": request.UserRequest,
			"requested_by": request.UserID,
		},
	}

	result, err := deps.Workflow.HandleAlertEvent(ctx, event)
	if err != nil {
		return err
	}
	auditTelegramEvent(ctx, deps, "telegram_chat", result.SessionID, "receive", request.UserID, map[string]any{
		"update_id":    update.UpdateID,
		"chat_id":      request.ChatID,
		"host":         request.Host,
		"service":      request.Service,
		"user_request": request.UserRequest,
		"alert_name":   request.AlertName,
		"duplicated":   result.Duplicated,
	})

	ackBody := fmt.Sprintf(
		"[TARS 对话]\nsession: %s\n目标主机: %s\n请求: %s\n状态: %s\n正在分析并生成建议。",
		result.SessionID,
		request.Host,
		request.UserRequest,
		result.Status,
	)
	if result.Duplicated {
		ackBody = fmt.Sprintf("[TARS 对话]\n这条请求已经在处理中。\nsession: %s", result.SessionID)
	}

	messages := []contracts.ChannelMessage{
		{
			Channel: "telegram",
			Target:  request.ChatID,
			Body:    ackBody,
		},
	}
	if err := deliverNotifications(ctx, deps.Channel, deps.Workflow, result.SessionID, messages); err != nil {
		return err
	}
	for _, message := range messages {
		auditTelegramEvent(ctx, deps, "telegram_message", result.SessionID, "dispatch", "tars", map[string]any{
			"chat_id":      message.Target,
			"delivery":     "direct_or_queued",
			"message_type": "conversation_ack",
			"body_preview": compactTelegramBody(message.Body, 240),
		})
	}
	return nil
}

type telegramCallbackAckKey struct{}

func setTelegramCallbackAckText(ctx context.Context, text string) {
	if ctx == nil || strings.TrimSpace(text) == "" {
		return
	}
	if slot, ok := ctx.Value(telegramCallbackAckKey{}).(*string); ok && slot != nil {
		*slot = text
	}
}

func acknowledgeTelegramCallback(ctx context.Context, deps Dependencies, update telegramUpdate) {
	callback := update.CallbackQuery
	if callback == nil || strings.TrimSpace(callback.ID) == "" {
		return
	}

	acker, ok := deps.Channel.(telegramCallbackAcker)
	if !ok {
		return
	}

	text := "已处理"
	if slot, ok := ctx.Value(telegramCallbackAckKey{}).(*string); ok && slot != nil && strings.TrimSpace(*slot) != "" {
		text = strings.TrimSpace(*slot)
	}

	if err := acker.AnswerCallbackQuery(ctx, callback.ID, text); err != nil {
		logger := deps.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Error("telegram answerCallbackQuery failed", "callback_id", callback.ID, "error", err)
	}
}

func telegramCallbackSuccessText(action string) string {
	switch action {
	case "approve", "modify_approve":
		return "已批准，开始执行"
	case "reject":
		return "已拒绝"
	case "request_context":
		return "已请求补充信息"
	default:
		return "已处理"
	}
}

func telegramCallbackErrorText(err error) string {
	switch {
	case errors.Is(err, contracts.ErrNotFound):
		return "执行不存在"
	case errors.Is(err, contracts.ErrInvalidState):
		return "这个审批已经处理过了"
	case errors.Is(err, errTelegramValidation):
		return "回调格式无效"
	default:
		return "处理失败，请稍后重试"
	}
}

func parseTelegramCallback(update telegramUpdate) (contracts.ChannelEvent, error) {
	callback := update.CallbackQuery
	if callback == nil {
		return contracts.ChannelEvent{}, fmt.Errorf("callback_query is required")
	}

	parts := strings.Split(callback.Data, ":")
	if len(parts) < 2 {
		return contracts.ChannelEvent{}, fmt.Errorf("invalid callback data")
	}

	action := parts[0]
	executionID := parts[1]
	command := ""
	if len(parts) > 2 {
		command = strings.Join(parts[2:], ":")
	}

	return contracts.ChannelEvent{
		EventType:      "approval",
		Channel:        "telegram",
		UserID:         nestedString(callback.From, "username", "id"),
		ChatID:         nestedMapString(callback.Message, "chat", "id"),
		Action:         action,
		ExecutionID:    executionID,
		Command:        command,
		IdempotencyKey: fmt.Sprintf("telegram_update:%d", update.UpdateID),
	}, nil
}

type telegramConversationRequest struct {
	ChatID      string
	UserID      string
	UserRequest string
	Host        string
	Service     string
	Severity    string
	AlertName   string
}

func parseTelegramConversationRequest(update telegramUpdate, allowedHosts []string) (*telegramConversationRequest, *contracts.ChannelMessage) {
	if update.Message == nil {
		return nil, nil
	}

	chatID := nestedMapString(update.Message, "chat", "id")
	if strings.TrimSpace(chatID) == "" {
		return nil, nil
	}
	if isBotSender(update.Message) {
		return nil, nil
	}

	text := strings.TrimSpace(messageText(update.Message))
	if text == "" {
		return nil, nil
	}
	if text == "/start" || text == "/help" {
		return nil, &contracts.ChannelMessage{
			Channel: "telegram",
			Target:  chatID,
			Body:    telegramConversationHelp(allowedHosts),
		}
	}

	host := resolveTelegramHost(text, allowedHosts)
	if host == "" {
		return nil, &contracts.ChannelMessage{
			Channel: "telegram",
			Target:  chatID,
			Body:    "还不知道要查哪台主机。请直接发主机名/IP，或在消息里写 `host=192.168.3.106 看系统负载`。",
		}
	}

	service := resolveTelegramService(text)
	return &telegramConversationRequest{
		ChatID:      chatID,
		UserID:      fallbackString(nestedMapString(update.Message, "from", "username"), nestedMapString(update.Message, "from", "id")),
		UserRequest: text,
		Host:        host,
		Service:     service,
		Severity:    "info",
		AlertName:   telegramConversationAlertName(text, service),
	}, nil
}

func telegramConversationHelp(allowedHosts []string) string {
	hostHint := "在消息里带上目标主机"
	if len(allowedHosts) == 1 {
		hostHint = "当前默认主机是 " + allowedHosts[0]
	}
	return strings.Join([]string{
		"[TARS 对话帮助]",
		hostHint,
		"你可以直接发自然语言，例如：",
		"- 看系统负载",
		"- host=192.168.3.106 看系统负载",
		"- 看一下 exim4 状态",
		"- host=192.168.3.106 查看磁盘使用情况",
		"TARS 会先给出建议命令，再走审批执行。",
	}, "\n")
}

func resolveTelegramHost(text string, allowedHosts []string) string {
	trimmed := strings.TrimSpace(text)
	for _, host := range allowedHosts {
		if host != "" && strings.Contains(trimmed, host) {
			return host
		}
	}

	for _, token := range strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '，'
	}) {
		lower := strings.ToLower(token)
		if strings.HasPrefix(lower, "host=") || strings.HasPrefix(lower, "host:") {
			return strings.TrimSpace(token[5:])
		}
		if strings.HasPrefix(token, "主机=") || strings.HasPrefix(token, "主机:") || strings.HasPrefix(token, "主机：") {
			return strings.TrimSpace(token[len("主机="):])
		}
	}

	if len(allowedHosts) == 1 {
		return allowedHosts[0]
	}
	return ""
}

func resolveTelegramService(text string) string {
	trimmed := strings.TrimSpace(text)
	for _, token := range strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '，'
	}) {
		lower := strings.ToLower(token)
		if strings.HasPrefix(lower, "service=") || strings.HasPrefix(lower, "service:") {
			return strings.TrimSpace(token[8:])
		}
		if strings.HasPrefix(token, "服务=") || strings.HasPrefix(token, "服务:") || strings.HasPrefix(token, "服务：") {
			return strings.TrimSpace(token[len("服务="):])
		}
	}

	for _, candidate := range []string{"sshd", "nginx", "exim4", "docker", "postgresql", "postgres"} {
		if strings.Contains(strings.ToLower(trimmed), candidate) {
			return candidate
		}
	}
	return ""
}

func telegramConversationAlertName(text string, service string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "load") || strings.Contains(lower, "uptime") || strings.Contains(text, "负载"):
		return "TarsChatLoadRequest"
	case strings.Contains(lower, "memory") || strings.Contains(text, "内存"):
		return "TarsChatMemoryRequest"
	case strings.Contains(lower, "disk") || strings.Contains(text, "磁盘"):
		return "TarsChatDiskRequest"
	case service != "" || strings.Contains(lower, "status") || strings.Contains(text, "状态"):
		return "TarsChatServiceStatus"
	default:
		return "TarsChatRequest"
	}
}

func messageText(message map[string]interface{}) string {
	if value, ok := message["text"].(string); ok {
		return value
	}
	return ""
}

func isBotSender(message map[string]interface{}) bool {
	from, ok := message["from"].(map[string]interface{})
	if !ok {
		return false
	}
	if value, ok := from["is_bot"].(bool); ok {
		return value
	}
	return false
}

func deliverNotifications(ctx context.Context, channel contracts.ChannelService, workflow contracts.WorkflowService, sessionID string, messages []contracts.ChannelMessage) error {
	for _, message := range messages {
		if _, err := channel.SendMessage(ctx, message); err != nil {
			if strings.TrimSpace(sessionID) == "" {
				return err
			}
			if enqueueErr := workflow.EnqueueNotifications(ctx, sessionID, []contracts.ChannelMessage{message}); enqueueErr != nil {
				return enqueueErr
			}
		}
	}
	return nil
}

func formatExecutionResultMessage(sessionDetail contracts.SessionDetail, executionDetail contracts.ExecutionDetail, outputChunks []contracts.ExecutionOutputChunk) string {
	lines := []string{
		"[TARS] 执行结果",
		fmt.Sprintf("主机: %s", executionDetail.TargetHost),
		fmt.Sprintf("状态: %s", executionDetail.Status),
		fmt.Sprintf("执行链: %s", compactExecutionRuntimeLabel(executionDetail)),
	}
	if executionDetail.ExitCode != 0 {
		lines = append(lines, fmt.Sprintf("退出码: %d", executionDetail.ExitCode))
	}
	if sessionDetail.Verification != nil {
		lines = append(lines, fmt.Sprintf("校验: %s", sessionDetail.Verification.Status))
		if strings.TrimSpace(sessionDetail.Verification.Summary) != "" {
			lines = append(lines, fmt.Sprintf("校验说明: %s", sessionDetail.Verification.Summary))
		}
	}
	if executionDetail.OutputRef != "" {
		lines = append(lines, fmt.Sprintf("日志: %s", executionDetail.OutputRef))
	}
	if preview := compactExecutionOutput(outputChunks); preview != "" {
		label := "输出"
		if executionDetail.OutputTruncated {
			label = "输出（已截断）"
		}
		lines = append(lines, fmt.Sprintf("%s:\n%s", label, preview))
	}
	lines = append(lines, fmt.Sprintf("会话: %s", sessionDetail.SessionID))
	return strings.Join(lines, "\n")
}

func compactExecutionRuntimeLabel(executionDetail contracts.ExecutionDetail) string {
	parts := []string{}
	if strings.TrimSpace(executionDetail.ConnectorID) != "" {
		parts = append(parts, executionDetail.ConnectorID)
	}
	if strings.TrimSpace(executionDetail.ExecutionMode) != "" {
		parts = append(parts, executionDetail.ExecutionMode)
	} else if strings.TrimSpace(executionDetail.Protocol) != "" {
		parts = append(parts, executionDetail.Protocol)
	} else if runtime := strings.TrimSpace(runtimeMetadataValue(executionDetail.Runtime, "runtime")); runtime != "" {
		parts = append(parts, runtime)
	}
	if len(parts) == 0 {
		parts = append(parts, "ssh")
	}
	if runtimeMetadataBool(executionDetail.Runtime, "fallback_used") {
		parts = append(parts, fmt.Sprintf("fallback→%s", fallbackString(runtimeMetadataValue(executionDetail.Runtime, "fallback_target"), "ssh")))
	}
	return strings.Join(parts, " · ")
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

func runtimeMetadataBool(runtime *contracts.RuntimeMetadata, field string) bool {
	if runtime == nil {
		return false
	}
	switch field {
	case "fallback_used":
		return runtime.FallbackUsed
	default:
		return false
	}
}

func compactExecutionOutput(chunks []contracts.ExecutionOutputChunk) string {
	if len(chunks) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		builder.WriteString(chunk.Content)
		if builder.Len() >= 480 {
			break
		}
	}

	preview := strings.TrimSpace(builder.String())
	if preview == "" {
		return ""
	}
	if len(preview) > 480 {
		preview = preview[:480]
	}

	lines := strings.Split(preview, "\n")
	if len(lines) > 6 {
		lines = lines[:6]
	}
	return strings.Join(lines, "\n")
}

func auditTelegramEvent(ctx context.Context, deps Dependencies, resourceType string, resourceID string, action string, actor string, metadata map[string]any) {
	if deps.Audit == nil {
		return
	}
	deps.Audit.Log(ctx, audit.Entry{
		ResourceType: resourceType,
		ResourceID:   fallbackString(resourceID, "unknown"),
		Action:       action,
		Actor:        actor,
		Metadata:     metadata,
	})
}

func compactTelegramBody(value string, maxLen int) string {
	trimmed := strings.TrimSpace(value)
	if maxLen <= 0 || len(trimmed) <= maxLen {
		return trimmed
	}
	return trimmed[:maxLen] + "..."
}

func nestedString(values map[string]interface{}, preferredKey string, fallbackKey string) string {
	if value, ok := values[preferredKey].(string); ok && value != "" {
		return value
	}
	if value, ok := values[fallbackKey].(float64); ok {
		return fmt.Sprintf("%.0f", value)
	}
	if value, ok := values[fallbackKey].(string); ok {
		return value
	}
	return ""
}

func alertLabel(values map[string]interface{}, key string) string {
	switch nested := values["labels"].(type) {
	case map[string]interface{}:
		if value, ok := nested[key].(string); ok {
			return value
		}
	case map[string]string:
		return nested[key]
	}
	return ""
}

func nestedMapString(values map[string]interface{}, nestedKey string, leafKey string) string {
	nested, ok := values[nestedKey].(map[string]interface{})
	if !ok {
		return ""
	}
	if value, ok := nested[leafKey].(string); ok {
		return value
	}
	if value, ok := nested[leafKey].(float64); ok {
		return fmt.Sprintf("%.0f", value)
	}
	return ""
}
