package knowledge

import "testing"

func TestDocumentFields(t *testing.T) {
	d := Document{
		ID:         "doc-001",
		SourceType: "confluence",
		SourceRef:  "https://wiki.example.com/page/123",
		Title:      "Runbook: Restart API Service",
	}

	if d.ID != "doc-001" {
		t.Errorf("expected ID doc-001, got %q", d.ID)
	}
	if d.SourceType != "confluence" {
		t.Errorf("expected SourceType confluence, got %q", d.SourceType)
	}
	if d.SourceRef != "https://wiki.example.com/page/123" {
		t.Errorf("expected SourceRef set, got %q", d.SourceRef)
	}
	if d.Title != "Runbook: Restart API Service" {
		t.Errorf("expected Title set, got %q", d.Title)
	}
}

func TestDocumentZeroValue(t *testing.T) {
	var d Document
	if d.ID != "" {
		t.Errorf("expected empty ID, got %q", d.ID)
	}
	if d.SourceType != "" {
		t.Errorf("expected empty SourceType, got %q", d.SourceType)
	}
}

func TestRecordFields(t *testing.T) {
	r := Record{
		ID:         "rec-001",
		SessionID:  "sess-abc",
		DocumentID: "doc-001",
		Status:     "linked",
	}

	if r.ID != "rec-001" {
		t.Errorf("expected ID rec-001, got %q", r.ID)
	}
	if r.SessionID != "sess-abc" {
		t.Errorf("expected SessionID sess-abc, got %q", r.SessionID)
	}
	if r.DocumentID != "doc-001" {
		t.Errorf("expected DocumentID doc-001, got %q", r.DocumentID)
	}
	if r.Status != "linked" {
		t.Errorf("expected Status linked, got %q", r.Status)
	}
}

func TestRecordZeroValue(t *testing.T) {
	var r Record
	if r.ID != "" {
		t.Errorf("expected empty ID, got %q", r.ID)
	}
	if r.Status != "" {
		t.Errorf("expected empty Status, got %q", r.Status)
	}
}

func TestDocumentSourceTypes(t *testing.T) {
	sourceTypes := []string{"confluence", "github", "notion", "manual"}
	for _, st := range sourceTypes {
		d := Document{SourceType: st}
		if d.SourceType != st {
			t.Errorf("source type round-trip failed: expected %q, got %q", st, d.SourceType)
		}
	}
}

func TestRecordDocumentLinkage(t *testing.T) {
	doc := Document{ID: "doc-xyz"}
	rec := Record{DocumentID: doc.ID}
	if rec.DocumentID != doc.ID {
		t.Errorf("record should reference document ID %q, got %q", doc.ID, rec.DocumentID)
	}
}
