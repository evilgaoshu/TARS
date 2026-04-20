package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tars/internal/contracts"
	foundationmetrics "tars/internal/foundation/metrics"
)

type Config struct {
	BotToken       string
	BaseURL        string
	PollingEnabled bool
	PollTimeout    time.Duration
	PollInterval   time.Duration
	Client         *http.Client
	Metrics        *foundationmetrics.Registry
}

type Service struct {
	logger               *slog.Logger
	client               *http.Client
	cfg                  Config
	metrics              *foundationmetrics.Registry
	pollingErrorSampling pollingErrorSampler
}

type UpdateProcessor func(ctx context.Context, rawUpdate []byte) error

const telegramPollingErrorSampleWindow = 30 * time.Second

type pollingErrorSampler struct {
	window time.Duration
	active *pollingErrorSample
}

type pollingErrorSample struct {
	key               string
	firstAt           time.Time
	lastAt            time.Time
	lastError         string
	lastBackoff       time.Duration
	lastConsecutive   int
	suppressedCount   int
	suppressionReason string
}

func NewService(logger *slog.Logger, cfg Config) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.telegram.org"
	}
	if cfg.PollTimeout <= 0 {
		cfg.PollTimeout = 30 * time.Second
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}

	return &Service{
		logger:               logger,
		client:               withDefaultClient(cfg.Client, cfg.PollTimeout),
		cfg:                  cfg,
		metrics:              cfg.Metrics,
		pollingErrorSampling: pollingErrorSampler{window: telegramPollingErrorSampleWindow},
	}
}

func (s *Service) SendMessage(ctx context.Context, msg contracts.ChannelMessage) (contracts.SendResult, error) {
	kind := "notification"
	if len(msg.Actions) > 0 {
		kind = "interactive"
	}
	if !hasConfiguredBotToken(s.cfg.BotToken) {
		stubReason := telegramStubReason(s.cfg.BotToken)
		s.logger.Info("telegram send stub", "reason", stubReason, "target", msg.Target, "body", truncateBody(msg.Body, 400), "actions", len(msg.Actions), "attachments", len(msg.Attachments))
		if s.metrics != nil {
			s.metrics.IncChannelMessage("telegram", kind, "stub")
			s.metrics.RecordComponentResult("telegram", "stub", stubReason)
		}
		return contracts.SendResult{MessageID: "stub-message-id"}, nil
	}

	attachmentMessageID := ""
	for _, attachment := range msg.Attachments {
		messageID, err := s.sendAttachment(ctx, msg.Target, attachment)
		if err != nil {
			if s.metrics != nil {
				s.metrics.IncChannelMessage("telegram", kind, "error")
				s.metrics.RecordComponentResult("telegram", "error", err.Error())
			}
			return contracts.SendResult{}, err
		}
		if attachmentMessageID == "" {
			attachmentMessageID = messageID
		}
	}

	payloadBody := map[string]interface{}{
		"chat_id":                  msg.Target,
		"text":                     msg.Body,
		"disable_web_page_preview": true,
	}
	if len(msg.Actions) > 0 {
		payloadBody["reply_markup"] = telegramInlineKeyboard(msg.Actions)
	}

	payload, err := json.Marshal(payloadBody)
	if err != nil {
		return contracts.SendResult{}, err
	}

	url := strings.TrimRight(s.cfg.BaseURL, "/") + "/bot" + s.cfg.BotToken + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return contracts.SendResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncChannelMessage("telegram", kind, "error")
			s.metrics.RecordComponentResult("telegram", "error", err.Error())
		}
		return contracts.SendResult{}, err
	}
	defer resp.Body.Close()

	var body telegramSendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		if s.metrics != nil {
			s.metrics.IncChannelMessage("telegram", kind, "error")
			s.metrics.RecordComponentResult("telegram", "error", err.Error())
		}
		return contracts.SendResult{}, err
	}
	if resp.StatusCode >= 400 || !body.OK {
		if s.metrics != nil {
			s.metrics.IncChannelMessage("telegram", kind, "error")
			s.metrics.RecordComponentResult("telegram", "error", body.Description)
		}
		return contracts.SendResult{}, fmt.Errorf("telegram send failed: status=%d description=%s", resp.StatusCode, body.Description)
	}
	if s.metrics != nil {
		s.metrics.IncChannelMessage("telegram", kind, "success")
		s.metrics.RecordComponentResult("telegram", "success", "message sent")
	}

	messageID := strconv.FormatInt(body.Result.MessageID, 10)
	if attachmentMessageID != "" {
		messageID = attachmentMessageID
	}
	return contracts.SendResult{MessageID: messageID}, nil
}

func (s *Service) sendAttachment(ctx context.Context, target string, attachment contracts.MessageAttachment) (string, error) {
	attachmentType := strings.ToLower(strings.TrimSpace(attachment.Type))
	fileURL := strings.TrimSpace(attachment.URL)
	if fileURL == "" && strings.TrimSpace(attachment.Content) == "" {
		return "", nil
	}
	if fileURL == "" {
		content, err := attachmentContentBytes(attachment)
		if err != nil {
			return "", err
		}
		return s.sendUploadedAttachment(ctx, target, attachment, content)
	}

	method := "sendDocument"
	payload := map[string]interface{}{
		"chat_id":  target,
		"document": fileURL,
	}
	if strings.TrimSpace(attachment.Name) != "" {
		payload["caption"] = attachment.Name
	}
	if attachmentType == "image" {
		method = "sendPhoto"
		payload = map[string]interface{}{
			"chat_id": target,
			"photo":   fileURL,
		}
		if strings.TrimSpace(attachment.Name) != "" {
			payload["caption"] = attachment.Name
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.cfg.BaseURL, "/")+"/bot"+s.cfg.BotToken+"/"+method, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result telegramSendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 || !result.OK {
		return "", fmt.Errorf("telegram %s failed: status=%d description=%s", method, resp.StatusCode, result.Description)
	}
	return strconv.FormatInt(result.Result.MessageID, 10), nil
}

func (s *Service) sendUploadedAttachment(ctx context.Context, target string, attachment contracts.MessageAttachment, content []byte) (string, error) {
	attachmentType := strings.ToLower(strings.TrimSpace(attachment.Type))
	method := "sendDocument"
	fieldName := "document"
	if attachmentType == "image" {
		method = "sendPhoto"
		fieldName = "photo"
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("chat_id", target); err != nil {
		return "", err
	}
	if strings.TrimSpace(attachment.Name) != "" {
		if err := writer.WriteField("caption", attachment.Name); err != nil {
			return "", err
		}
	}
	filename := strings.TrimSpace(attachment.Name)
	if filename == "" {
		if attachmentType == "image" {
			filename = "attachment.png"
		} else {
			filename = "attachment.bin"
		}
	}
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(content); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.cfg.BaseURL, "/")+"/bot"+s.cfg.BotToken+"/"+method, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result telegramSendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 || !result.OK {
		return "", fmt.Errorf("telegram %s upload failed: status=%d description=%s", method, resp.StatusCode, result.Description)
	}
	return strconv.FormatInt(result.Result.MessageID, 10), nil
}

func attachmentContentBytes(attachment contracts.MessageAttachment) ([]byte, error) {
	content := strings.TrimSpace(attachment.Content)
	if content == "" {
		return nil, nil
	}
	if strings.EqualFold(metadataString(attachment.Metadata, "encoding"), "base64") {
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, err
		}
		return decoded, nil
	}
	return []byte(content), nil
}

func metadataString(metadata map[string]interface{}, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func (s *Service) AnswerCallbackQuery(ctx context.Context, callbackID string, text string) error {
	if strings.TrimSpace(callbackID) == "" {
		return nil
	}
	if !hasConfiguredBotToken(s.cfg.BotToken) {
		stubReason := telegramStubReason(s.cfg.BotToken)
		s.logger.Info("telegram answerCallbackQuery stub", "reason", stubReason, "callback_id", callbackID, "text", text)
		if s.metrics != nil {
			s.metrics.IncChannelCallback("stub")
			s.metrics.RecordComponentResult("telegram", "stub", stubReason)
		}
		return nil
	}

	payload, err := json.Marshal(map[string]interface{}{
		"callback_query_id": callbackID,
		"text":              text,
	})
	if err != nil {
		return err
	}

	url := strings.TrimRight(s.cfg.BaseURL, "/") + "/bot" + s.cfg.BotToken + "/answerCallbackQuery"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncChannelCallback("error")
			s.metrics.RecordComponentResult("telegram", "error", err.Error())
		}
		return err
	}
	defer resp.Body.Close()

	var body telegramSimpleResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		if s.metrics != nil {
			s.metrics.IncChannelCallback("error")
			s.metrics.RecordComponentResult("telegram", "error", err.Error())
		}
		return err
	}
	if resp.StatusCode >= 400 || !body.OK {
		if s.metrics != nil {
			s.metrics.IncChannelCallback("error")
			s.metrics.RecordComponentResult("telegram", "error", body.Description)
		}
		return fmt.Errorf("telegram answerCallbackQuery failed: status=%d description=%s", resp.StatusCode, body.Description)
	}
	if s.metrics != nil {
		s.metrics.IncChannelCallback("success")
		s.metrics.RecordComponentResult("telegram", "success", "callback acknowledged")
	}
	return nil
}

func (s *Service) StartPolling(ctx context.Context, processor UpdateProcessor) {
	if !s.cfg.PollingEnabled {
		return
	}
	if processor == nil {
		s.logger.Warn("telegram polling disabled: processor is nil")
		if s.metrics != nil {
			s.metrics.RecordComponentResult("telegram", "error", "polling processor is nil")
		}
		return
	}
	if strings.TrimSpace(s.cfg.BotToken) == "" {
		s.logger.Warn("telegram polling disabled: bot token is empty")
		if s.metrics != nil {
			s.metrics.RecordComponentResult("telegram", "stub", "bot token not configured")
		}
		return
	}
	if isPlaceholderToken(s.cfg.BotToken) {
		s.logger.Warn("telegram polling disabled: bot token looks like a placeholder",
			"token_prefix", s.cfg.BotToken[:min(len(s.cfg.BotToken), 12)]+"...")
		if s.metrics != nil {
			s.metrics.RecordComponentResult("telegram", "stub", "bot token is a placeholder")
		}
		return
	}

	s.logger.Info("telegram polling worker started")
	if err := s.deleteWebhook(ctx); err != nil {
		s.logger.Error("telegram deleteWebhook failed", "error", err)
		if s.metrics != nil {
			s.metrics.RecordComponentResult("telegram", "error", err.Error())
		}
	}

	var offset int64
	var consecutiveErrors int
	for {
		select {
		case <-ctx.Done():
			s.flushPollingErrorSampling("shutdown")
			s.logger.Info("telegram polling worker stopped")
			return
		default:
		}

		updates, err := s.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				s.flushPollingErrorSampling("shutdown")
				s.logger.Info("telegram polling worker stopped")
				return
			}
			consecutiveErrors++
			backoff := computeBackoff(s.cfg.PollInterval, consecutiveErrors)
			s.logGetUpdatesFailure(err, consecutiveErrors, backoff)
			if s.metrics != nil {
				s.metrics.RecordComponentResult("telegram", "error", err.Error())
			}
			if !sleepContext(ctx, backoff) {
				s.flushPollingErrorSampling("shutdown")
				s.logger.Info("telegram polling worker stopped")
				return
			}
			continue
		}
		if consecutiveErrors > 0 {
			s.flushPollingErrorSampling("recovered")
		}
		consecutiveErrors = 0
		if s.metrics != nil {
			s.metrics.RecordComponentResult("telegram", "success", "polling active")
		}

		if len(updates) == 0 {
			continue
		}

		for _, item := range updates {
			if item.UpdateID >= offset {
				offset = item.UpdateID + 1
			}
			if err := processor(ctx, item.Raw); err != nil {
				s.logger.Error("telegram update processing failed", "update_id", item.UpdateID, "error", err)
			}
		}
	}
}

func (s *Service) logGetUpdatesFailure(err error, consecutiveErrors int, backoff time.Duration) {
	if err == nil {
		return
	}
	now := time.Now().UTC()
	errText := strings.TrimSpace(err.Error())
	key := "getUpdates:"
	if errText != "" {
		key += errText
	}
	emitFirst, summary := s.pollingErrorSampling.record(now, key, errText, consecutiveErrors, backoff)
	if summary != nil {
		s.emitGetUpdatesFailureSummary(*summary)
	}
	if emitFirst {
		s.logger.Error("telegram getUpdates failed", "error", err, "consecutive_errors", consecutiveErrors, "backoff", backoff)
	}
}

func (s *Service) flushPollingErrorSampling(reason string) {
	if summary := s.pollingErrorSampling.flush(reason); summary != nil {
		s.emitGetUpdatesFailureSummary(*summary)
	}
}

func (s *Service) emitGetUpdatesFailureSummary(sample pollingErrorSample) {
	if sample.suppressedCount <= 0 {
		return
	}
	attrs := []any{
		"error", sample.lastError,
		"sampled", true,
		"suppressed_count", sample.suppressedCount,
		"window", s.pollingErrorSampling.window,
		"first_seen_at", sample.firstAt,
		"last_seen_at", sample.lastAt,
	}
	if sample.lastConsecutive > 0 {
		attrs = append(attrs, "consecutive_errors", sample.lastConsecutive)
	}
	if sample.lastBackoff > 0 {
		attrs = append(attrs, "backoff", sample.lastBackoff)
	}
	if strings.TrimSpace(sample.suppressionReason) != "" {
		attrs = append(attrs, "flush_reason", sample.suppressionReason)
	}
	s.logger.Error("telegram getUpdates failed", attrs...)
}

func (s *pollingErrorSampler) record(now time.Time, key string, errText string, consecutiveErrors int, backoff time.Duration) (bool, *pollingErrorSample) {
	if s.window <= 0 {
		s.window = telegramPollingErrorSampleWindow
	}
	if s.active == nil {
		s.active = &pollingErrorSample{
			key:             key,
			firstAt:         now,
			lastAt:          now,
			lastError:       errText,
			lastBackoff:     backoff,
			lastConsecutive: consecutiveErrors,
		}
		return true, nil
	}
	if s.active.key != key || now.Sub(s.active.firstAt) >= s.window {
		reason := "key_changed"
		if s.active.key == key {
			reason = "window_elapsed"
		}
		summary := s.flush(reason)
		s.active = &pollingErrorSample{
			key:             key,
			firstAt:         now,
			lastAt:          now,
			lastError:       errText,
			lastBackoff:     backoff,
			lastConsecutive: consecutiveErrors,
		}
		return true, summary
	}
	s.active.lastAt = now
	s.active.lastError = errText
	s.active.lastBackoff = backoff
	s.active.lastConsecutive = consecutiveErrors
	s.active.suppressedCount++
	return false, nil
}

func (s *pollingErrorSampler) flush(reason string) *pollingErrorSample {
	if s.active == nil {
		return nil
	}
	active := s.active
	s.active = nil
	if active.suppressedCount <= 0 {
		return nil
	}
	clone := *active
	clone.suppressionReason = reason
	return &clone
}

type telegramSendMessageResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      struct {
		MessageID int64 `json:"message_id"`
	} `json:"result"`
}

type telegramSimpleResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

type telegramGetUpdatesResponse struct {
	OK          bool              `json:"ok"`
	Description string            `json:"description"`
	Result      []json.RawMessage `json:"result"`
}

func telegramInlineKeyboard(actions []contracts.ChannelAction) map[string]interface{} {
	row := make([]map[string]string, 0, len(actions))
	for _, action := range actions {
		if strings.TrimSpace(action.Label) == "" || strings.TrimSpace(action.Value) == "" {
			continue
		}
		row = append(row, map[string]string{
			"text":          action.Label,
			"callback_data": action.Value,
		})
	}
	if len(row) == 0 {
		return nil
	}
	return map[string]interface{}{
		"inline_keyboard": [][]map[string]string{row},
	}
}

type polledUpdate struct {
	UpdateID int64
	Raw      []byte
}

func (s *Service) deleteWebhook(ctx context.Context) error {
	url := strings.TrimRight(s.cfg.BaseURL, "/") + "/bot" + s.cfg.BotToken + "/deleteWebhook"
	payload, err := json.Marshal(map[string]bool{
		"drop_pending_updates": false,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var body struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	if resp.StatusCode >= 400 || !body.OK {
		return fmt.Errorf("telegram deleteWebhook failed: status=%d description=%s", resp.StatusCode, body.Description)
	}
	return nil
}

func (s *Service) getUpdates(ctx context.Context, offset int64) ([]polledUpdate, error) {
	url := strings.TrimRight(s.cfg.BaseURL, "/") + "/bot" + s.cfg.BotToken + "/getUpdates"
	payload, err := json.Marshal(map[string]interface{}{
		"offset":          offset,
		"timeout":         int(s.cfg.PollTimeout / time.Second),
		"allowed_updates": []string{"callback_query", "message"},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body telegramGetUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 || !body.OK {
		return nil, fmt.Errorf("telegram getUpdates failed: status=%d description=%s", resp.StatusCode, body.Description)
	}

	items := make([]polledUpdate, 0, len(body.Result))
	for _, raw := range body.Result {
		var meta struct {
			UpdateID int64 `json:"update_id"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, err
		}
		items = append(items, polledUpdate{
			UpdateID: meta.UpdateID,
			Raw:      append([]byte(nil), raw...),
		})
	}
	return items, nil
}

func withDefaultClient(client *http.Client, pollTimeout time.Duration) *http.Client {
	if client != nil {
		return client
	}
	timeout := 10 * time.Second
	if pollTimeout > 0 && pollTimeout+5*time.Second > timeout {
		timeout = pollTimeout + 5*time.Second
	}
	return &http.Client{Timeout: timeout}
}

func truncateBody(body string, maxLen int) string {
	body = strings.TrimSpace(body)
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "..."
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func hasConfiguredBotToken(token string) bool {
	token = strings.TrimSpace(token)
	return token != "" && !isPlaceholderToken(token)
}

func telegramStubReason(token string) string {
	if strings.TrimSpace(token) == "" {
		return "bot token not configured"
	}
	return "bot token is a placeholder"
}

// isPlaceholderToken returns true if the token looks like an unconfigured
// placeholder rather than a real Telegram bot token.
func isPlaceholderToken(token string) bool {
	t := strings.ToLower(strings.TrimSpace(token))
	placeholders := []string{
		"replace_with",
		"your_token",
		"your-token",
		"changeme",
		"xxx",
		"placeholder",
		"todo",
		"insert_",
		"<token>",
		"<bot_token>",
	}
	for _, p := range placeholders {
		if strings.Contains(t, p) {
			return true
		}
	}
	return false
}

// computeBackoff returns an exponential backoff duration capped at 60 seconds.
// base is the normal poll interval; attempt is the consecutive error count (1-based).
func computeBackoff(base time.Duration, attempt int) time.Duration {
	if attempt <= 1 {
		return base
	}
	backoff := base
	for i := 1; i < attempt && i < 7; i++ {
		backoff *= 2
	}
	const maxBackoff = 60 * time.Second
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return backoff
}
