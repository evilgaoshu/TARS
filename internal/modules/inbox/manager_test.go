package inbox

import (
	"context"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newMemManager() *Manager {
	return NewManager(nil)
}

func fixedNow() time.Time {
	return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
}

func managerWithFixedClock() *Manager {
	m := NewManager(nil)
	m.now = fixedNow
	return m
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestCreate_GeneratesID(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	msg, err := m.Create(context.Background(), Message{Subject: "hello", Body: "world"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if msg.ID == "" {
		t.Fatal("expected generated ID")
	}
}

func TestCreate_DefaultsChannel(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	msg, err := m.Create(context.Background(), Message{Subject: "s", Body: "b"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if msg.Channel != "in_app_inbox" {
		t.Fatalf("expected channel=in_app_inbox, got %q", msg.Channel)
	}
}

func TestCreate_DefaultsTenantID(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	msg, err := m.Create(context.Background(), Message{Subject: "s", Body: "b"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if msg.TenantID != "default" {
		t.Fatalf("expected TenantID=default, got %q", msg.TenantID)
	}
}

func TestCreate_PreservesExplicitID(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	msg, err := m.Create(context.Background(), Message{ID: "explicit-id", Subject: "s", Body: "b"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if msg.ID != "explicit-id" {
		t.Fatalf("expected explicit-id, got %s", msg.ID)
	}
}

func TestCreate_SetsCreatedAt(t *testing.T) {
	t.Parallel()
	m := managerWithFixedClock()
	msg, err := m.Create(context.Background(), Message{Subject: "s", Body: "b"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !msg.CreatedAt.Equal(fixedNow()) {
		t.Fatalf("expected CreatedAt=%v, got %v", fixedNow(), msg.CreatedAt)
	}
}

func TestCreate_WithActions(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	msg, err := m.Create(context.Background(), Message{
		Subject: "approve?",
		Body:    "please approve",
		Actions: []Action{
			{Label: "Approve", Value: "approve"},
			{Label: "Reject", Value: "reject"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(msg.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(msg.Actions))
	}
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func TestGet_ReturnsCreatedMessage(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	created, _ := m.Create(context.Background(), Message{Subject: "s", Body: "b", RefType: "session", RefID: "sid-1"})
	got, err := m.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RefID != "sid-1" {
		t.Fatalf("expected RefID=sid-1, got %q", got.RefID)
	}
}

func TestGet_NotFound(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	_, err := m.Get(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList_ReturnsAllByDefault(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	for i := 0; i < 3; i++ {
		m.Create(context.Background(), Message{Subject: "msg", Body: "body"})
	}
	msgs, err := m.List(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
}

func TestList_FilterByTenantID(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	m.Create(context.Background(), Message{TenantID: "acme", Subject: "for acme", Body: "b"})
	m.Create(context.Background(), Message{TenantID: "other", Subject: "for other", Body: "b"})
	msgs, err := m.List(context.Background(), ListFilter{TenantID: "acme"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(msgs) != 1 || msgs[0].TenantID != "acme" {
		t.Fatalf("expected 1 acme message, got %d", len(msgs))
	}
}

func TestList_UnreadOnlyFilter(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	created, _ := m.Create(context.Background(), Message{Subject: "unread", Body: "b"})
	m.Create(context.Background(), Message{Subject: "read", Body: "b"})
	m.MarkRead(context.Background(), created.ID)

	unread, err := m.List(context.Background(), ListFilter{UnreadOnly: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// created was marked read, so only the second message should be returned
	if len(unread) != 1 {
		t.Fatalf("expected 1 unread message, got %d", len(unread))
	}
	if unread[0].Subject != "read" {
		t.Fatalf("unexpected message: %q", unread[0].Subject)
	}
}

func TestList_Pagination(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	for i := 0; i < 5; i++ {
		m.Create(context.Background(), Message{Subject: "msg", Body: "body"})
	}
	page1, _ := m.List(context.Background(), ListFilter{Limit: 2, Offset: 0})
	page2, _ := m.List(context.Background(), ListFilter{Limit: 2, Offset: 2})
	if len(page1) != 2 || len(page2) != 2 {
		t.Fatalf("expected 2 per page, got p1=%d p2=%d", len(page1), len(page2))
	}
	if page1[0].ID == page2[0].ID {
		t.Fatal("pages must not overlap")
	}
}

// ---------------------------------------------------------------------------
// MarkRead
// ---------------------------------------------------------------------------

func TestMarkRead_SetsIsReadAndReadAt(t *testing.T) {
	t.Parallel()
	m := managerWithFixedClock()
	msg, _ := m.Create(context.Background(), Message{Subject: "s", Body: "b"})
	if msg.IsRead {
		t.Fatal("expected IsRead=false after create")
	}
	read, err := m.MarkRead(context.Background(), msg.ID)
	if err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if !read.IsRead {
		t.Fatal("expected IsRead=true after MarkRead")
	}
	if read.ReadAt.IsZero() {
		t.Fatalf("expected ReadAt to be set, got zero")
	}
	if !read.ReadAt.Equal(fixedNow()) {
		t.Fatalf("expected ReadAt=%v (fixed clock), got %v", fixedNow(), read.ReadAt)
	}
}

func TestMarkRead_NotFound(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	_, err := m.MarkRead(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing message")
	}
}

// ---------------------------------------------------------------------------
// MarkAllRead
// ---------------------------------------------------------------------------

func TestMarkAllRead_MarksAllUnread(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	for i := 0; i < 4; i++ {
		m.Create(context.Background(), Message{TenantID: "acme", Subject: "s", Body: "b"})
	}
	n, err := m.MarkAllRead(context.Background(), "acme")
	if err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	if n != 4 {
		t.Fatalf("expected 4 marked, got %d", n)
	}
	unread, _ := m.List(context.Background(), ListFilter{TenantID: "acme", UnreadOnly: true})
	if len(unread) != 0 {
		t.Fatalf("expected 0 unread after MarkAllRead, got %d", len(unread))
	}
}

func TestMarkAllRead_OnlyAffectsSpecifiedTenant(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	m.Create(context.Background(), Message{TenantID: "acme", Subject: "s", Body: "b"})
	m.Create(context.Background(), Message{TenantID: "other", Subject: "s", Body: "b"})
	m.MarkAllRead(context.Background(), "acme")

	otherUnread, _ := m.List(context.Background(), ListFilter{TenantID: "other", UnreadOnly: true})
	if len(otherUnread) != 1 {
		t.Fatalf("expected other tenant's messages unaffected, got %d unread", len(otherUnread))
	}
}

// ---------------------------------------------------------------------------
// CountUnread
// ---------------------------------------------------------------------------

func TestCountUnread_ReturnsCorrectCount(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	m.Create(context.Background(), Message{TenantID: "acme", Subject: "s", Body: "b"})
	m.Create(context.Background(), Message{TenantID: "acme", Subject: "s", Body: "b"})
	read, _ := m.Create(context.Background(), Message{TenantID: "acme", Subject: "s", Body: "b"})
	m.MarkRead(context.Background(), read.ID)

	count, err := m.CountUnread(context.Background(), "acme")
	if err != nil {
		t.Fatalf("CountUnread: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 unread, got %d", count)
	}
}

func TestCountUnread_ZeroForEmptyTenant(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	count, err := m.CountUnread(context.Background(), "nobody")
	if err != nil {
		t.Fatalf("CountUnread: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}
