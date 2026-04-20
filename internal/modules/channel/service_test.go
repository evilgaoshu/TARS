package channel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tars/internal/contracts"
	"tars/internal/modules/inbox"
)

// ---------------------------------------------------------------------------
// stub inbox
// ---------------------------------------------------------------------------

type stubInbox struct {
	created []inbox.Message
	err     error
}

func (s *stubInbox) Create(_ context.Context, msg inbox.Message) (inbox.Message, error) {
	if s.err != nil {
		return inbox.Message{}, s.err
	}
	if msg.ID == "" {
		msg.ID = "inbox_stub_" + msg.Subject
	}
	s.created = append(s.created, msg)
	return msg, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newService(inboxSvc InboxService) *CompositeService {
	return NewCompositeService(nil, inboxSvc)
}

// ---------------------------------------------------------------------------
// sendInbox routing
// ---------------------------------------------------------------------------

func TestSendMessage_InAppInbox_RouteToInbox(t *testing.T) {
	t.Parallel()
	stub := &stubInbox{}
	svc := newService(stub)

	res, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "in_app_inbox",
		Subject: "Alert fired",
		Body:    "CPU is high",
		RefType: "session",
		RefID:   "sess-1",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if res.MessageID == "" {
		t.Fatal("expected non-empty MessageID")
	}
	if len(stub.created) != 1 {
		t.Fatalf("expected 1 inbox message created, got %d", len(stub.created))
	}
	if stub.created[0].RefID != "sess-1" {
		t.Fatalf("expected RefID=sess-1, got %q", stub.created[0].RefID)
	}
}

func TestSendMessage_InAppInbox_SubjectDerivedFromBody(t *testing.T) {
	t.Parallel()
	stub := &stubInbox{}
	svc := newService(stub)

	_, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "in_app_inbox",
		Body:    "First line\nSecond line",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if stub.created[0].Subject != "First line" {
		t.Fatalf("expected subject=First line, got %q", stub.created[0].Subject)
	}
}

func TestSendMessage_InAppInbox_ExplicitSubjectPreserved(t *testing.T) {
	t.Parallel()
	stub := &stubInbox{}
	svc := newService(stub)

	_, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "in_app_inbox",
		Subject: "Explicit Subject",
		Body:    "Body text",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if stub.created[0].Subject != "Explicit Subject" {
		t.Fatalf("expected preserved subject, got %q", stub.created[0].Subject)
	}
}

func TestSendMessage_InAppInbox_SourceDefaultsToSystem(t *testing.T) {
	t.Parallel()
	stub := &stubInbox{}
	svc := newService(stub)

	_, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "in_app_inbox",
		Body:    "test",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if stub.created[0].Source != "system" {
		t.Fatalf("expected Source=system, got %q", stub.created[0].Source)
	}
}

func TestSendMessage_InAppInbox_SourcePreserved(t *testing.T) {
	t.Parallel()
	stub := &stubInbox{}
	svc := newService(stub)

	_, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "in_app_inbox",
		Body:    "test",
		Source:  "automation",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if stub.created[0].Source != "automation" {
		t.Fatalf("expected Source=automation, got %q", stub.created[0].Source)
	}
}

func TestSendMessage_InAppInbox_ActionsForwarded(t *testing.T) {
	t.Parallel()
	stub := &stubInbox{}
	svc := newService(stub)

	_, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "in_app_inbox",
		Body:    "approve?",
		Actions: []contracts.ChannelAction{
			{Label: "Approve", Value: "approve"},
		},
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if len(stub.created[0].Actions) != 1 || stub.created[0].Actions[0].Value != "approve" {
		t.Fatalf("expected action forwarded, got %+v", stub.created[0].Actions)
	}
}

func TestSendMessage_InAppInbox_NilInbox_ReturnsError(t *testing.T) {
	t.Parallel()
	svc := NewCompositeService(nil, nil)
	_, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "in_app_inbox",
		Body:    "test",
	})
	if err == nil {
		t.Fatal("expected error when inbox is nil")
	}
	if !strings.Contains(err.Error(), "inbox service not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendMessage_InAppInbox_PropagatesCreateError(t *testing.T) {
	t.Parallel()
	stub := &stubInbox{err: errors.New("db down")}
	svc := newService(stub)

	_, err := svc.SendMessage(context.Background(), contracts.ChannelMessage{
		Channel: "in_app_inbox",
		Body:    "test",
	})
	if err == nil {
		t.Fatal("expected error from inbox Create failure")
	}
	if !strings.Contains(err.Error(), "db down") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// ---------------------------------------------------------------------------
// helper functions
// ---------------------------------------------------------------------------

func TestFirstLine_MultiLine(t *testing.T) {
	t.Parallel()
	got := firstLine("hello\nworld", 200)
	if got != "hello" {
		t.Fatalf("expected hello, got %q", got)
	}
}

func TestFirstLine_Truncation(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 200)
	got := firstLine(long, 120)
	if len(got) != 120 {
		t.Fatalf("expected len=120, got %d", len(got))
	}
}

func TestFallback_ReturnsPrimary(t *testing.T) {
	t.Parallel()
	if fallback("primary", "default") != "primary" {
		t.Fatal("expected primary")
	}
}

func TestFallback_ReturnsDefault(t *testing.T) {
	t.Parallel()
	if fallback("", "default") != "default" {
		t.Fatal("expected default")
	}
	if fallback("   ", "default") != "default" {
		t.Fatal("expected default for whitespace")
	}
}
