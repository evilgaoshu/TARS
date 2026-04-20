package msgtpl

import (
	"testing"
)

func TestNewManager_DefaultTemplates(t *testing.T) {
	m := NewManager(nil)
	items := m.List()
	if len(items) != 6 {
		t.Fatalf("expected 6 default templates, got %d", len(items))
	}
}

func TestManager_Get(t *testing.T) {
	m := NewManager(nil)
	tpl, ok := m.Get("diagnosis-zh-CN")
	if !ok {
		t.Fatal("expected to find diagnosis-zh-CN")
	}
	if tpl.Kind != "diagnosis" {
		t.Errorf("expected kind=diagnosis, got %q", tpl.Kind)
	}
	if tpl.Locale != "zh-CN" {
		t.Errorf("expected locale=zh-CN, got %q", tpl.Locale)
	}
	if !tpl.Enabled {
		t.Error("expected default template to be enabled")
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	m := NewManager(nil)
	_, ok := m.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestManager_Upsert_Create(t *testing.T) {
	m := NewManager(nil)
	tpl := MsgTemplate{
		Kind:    "diagnosis",
		Locale:  "en-US",
		Name:    "My Custom Diagnosis",
		Enabled: true,
		Content: TemplateContent{
			Subject: "Alert: {{AlertName}}",
			Body:    "Summary: {{Summary}}",
		},
	}
	saved, err := m.Upsert(tpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.ID != "diagnosis-en-US" {
		t.Errorf("expected id=diagnosis-en-US, got %q", saved.ID)
	}
	if saved.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestManager_Upsert_Update(t *testing.T) {
	m := NewManager(nil)
	tpl, _ := m.Get("approval-zh-CN")
	tpl.Name = "Updated Approval ZH"
	saved, err := m.Upsert(tpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if saved.Name != "Updated Approval ZH" {
		t.Errorf("expected updated name, got %q", saved.Name)
	}
	// Verify it's actually persisted
	got, _ := m.Get("approval-zh-CN")
	if got.Name != "Updated Approval ZH" {
		t.Error("update was not persisted")
	}
}

func TestManager_Upsert_InvalidKind(t *testing.T) {
	m := NewManager(nil)
	_, err := m.Upsert(MsgTemplate{
		Kind:    "unknown_type",
		Locale:  "zh-CN",
		Name:    "Bad",
		Content: TemplateContent{Subject: "x", Body: "y"},
	})
	if err == nil {
		t.Fatal("expected validation error for invalid kind")
	}
}

func TestManager_Upsert_InvalidLocale(t *testing.T) {
	m := NewManager(nil)
	_, err := m.Upsert(MsgTemplate{
		Kind:    "diagnosis",
		Locale:  "fr-FR",
		Name:    "Bad",
		Content: TemplateContent{Subject: "x", Body: "y"},
	})
	if err == nil {
		t.Fatal("expected validation error for invalid locale")
	}
}

func TestManager_Upsert_MissingName(t *testing.T) {
	m := NewManager(nil)
	_, err := m.Upsert(MsgTemplate{
		Kind:    "diagnosis",
		Locale:  "zh-CN",
		Content: TemplateContent{Subject: "x", Body: "y"},
	})
	if err == nil {
		t.Fatal("expected validation error for missing name")
	}
}

func TestManager_SetEnabled_Enable(t *testing.T) {
	m := NewManager(nil)
	updated, err := m.SetEnabled("diagnosis-zh-CN", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Enabled {
		t.Error("expected template to be disabled")
	}
	// Re-enable
	updated, err = m.SetEnabled("diagnosis-zh-CN", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated.Enabled {
		t.Error("expected template to be enabled")
	}
}

func TestManager_SetEnabled_NotFound(t *testing.T) {
	m := NewManager(nil)
	_, err := m.SetEnabled("nonexistent", true)
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestManager_List_Sorted(t *testing.T) {
	m := NewManager(nil)
	items := m.List()
	for i := 1; i < len(items); i++ {
		if items[i].ID < items[i-1].ID {
			t.Errorf("list not sorted at index %d: %q < %q", i, items[i].ID, items[i-1].ID)
		}
	}
}
