// Package channel provides a composite ChannelService that routes messages
// to the correct backend based on the message.Channel field.
//
// Supported channel values:
//   - "telegram"      → Telegram Bot API
//   - "in_app_inbox"  → in-app inbox (站内信)
//   - ""              → defaults to "telegram" for backward compatibility
package channel

import (
	"context"
	"fmt"
	"strings"

	"tars/internal/contracts"
	"tars/internal/modules/channel/telegram"
	"tars/internal/modules/inbox"
)

// InboxService is the subset of inbox.Manager that channel routing needs.
type InboxService interface {
	Create(ctx context.Context, msg inbox.Message) (inbox.Message, error)
}

// CompositeService satisfies contracts.ChannelService and routes to the
// appropriate backend channel based on ChannelMessage.Channel.
type CompositeService struct {
	tg    *telegram.Service // original telegram-only service
	inbox InboxService
}

// NewCompositeService creates a CompositeService.
// Pass nil for inbox if inbox is not configured.
func NewCompositeService(tg *telegram.Service, inboxSvc InboxService) *CompositeService {
	return &CompositeService{
		tg:    tg,
		inbox: inboxSvc,
	}
}

// TelegramService returns the underlying telegram.Service for polling.
func (s *CompositeService) TelegramService() *telegram.Service {
	return s.tg
}

// SendMessage dispatches the message to the right channel.
func (s *CompositeService) SendMessage(ctx context.Context, msg contracts.ChannelMessage) (contracts.SendResult, error) {
	ch := strings.TrimSpace(msg.Channel)
	if ch == "" {
		ch = "telegram"
	}

	switch ch {
	case "in_app_inbox":
		return s.sendInbox(ctx, msg)
	default:
		// telegram (and any future direct channels)
		return s.tg.SendMessage(ctx, msg)
	}
}

func (s *CompositeService) sendInbox(ctx context.Context, msg contracts.ChannelMessage) (contracts.SendResult, error) {
	if s.inbox == nil {
		return contracts.SendResult{}, fmt.Errorf("in_app_inbox: inbox service not configured")
	}
	// Derive subject from the first line of the body, or a short truncation.
	subject := msg.Subject
	if subject == "" {
		subject = firstLine(msg.Body, 120)
	}

	created, err := s.inbox.Create(ctx, inbox.Message{
		Subject: subject,
		Body:    msg.Body,
		Channel: "in_app_inbox",
		RefType: msg.RefType,
		RefID:   msg.RefID,
		Source:  fallback(msg.Source, "system"),
		Actions: inboxActions(msg.Actions),
	})
	if err != nil {
		return contracts.SendResult{}, fmt.Errorf("in_app_inbox create: %w", err)
	}
	return contracts.SendResult{MessageID: created.ID}, nil
}

func inboxActions(actions []contracts.ChannelAction) []inbox.Action {
	if len(actions) == 0 {
		return nil
	}
	items := make([]inbox.Action, 0, len(actions))
	for _, action := range actions {
		items = append(items, inbox.Action{Label: action.Label, Value: action.Value})
	}
	return items
}

func firstLine(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return strings.TrimSpace(s)
}

func fallback(primary, def string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	return def
}
