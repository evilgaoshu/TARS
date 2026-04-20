package extensions

import (
	"os"
	"testing"

	"tars/internal/modules/skills"
)

func TestGenerateValidateAndImportCandidate(t *testing.T) {
	t.Parallel()

	configPath := t.TempDir() + "/skills.yaml"
	statePath := t.TempDir() + "/extensions.state.yaml"
	if err := os.WriteFile(configPath, []byte("skills:\n  entries: []\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	skillManager, err := skills.NewManager(configPath, "")
	if err != nil {
		t.Fatalf("new skill manager: %v", err)
	}
	manager, err := NewManager(statePath, skillManager)
	if err != nil {
		t.Fatalf("new extension manager: %v", err)
	}

	candidate, err := manager.Generate(GenerateOptions{
		OperatorReason: "generate bundle candidate",
		Bundle: Bundle{
			Metadata: BundleMetadata{DisplayName: "Disk Space Extension", Source: "official-generator", Summary: "Respond to disk pressure incidents"},
			Skill: skills.Manifest{
				Metadata: skills.Metadata{ID: "disk-space-extension", Name: "disk-space-extension", DisplayName: "Disk Space Extension", Version: "1.0.0", Vendor: "tars", Source: "official-generator"},
				Spec: skills.Spec{
					Type:    "incident_skill",
					Planner: skills.Planner{Summary: "Investigate disk pressure.", Steps: []skills.PlannerStep{{ID: "step_1", Tool: "knowledge.search", Required: true, Reason: "Load guidance first."}}},
				},
			},
			Docs:  []DocsAsset{{Slug: "disk-space-extension", Title: "Disk Space Extension", Summary: "Runbook summary", Content: "# Disk Space\nInvestigate inode and volume pressure."}},
			Tests: []TestSpec{{ID: "smoke", Name: "Smoke validation", Kind: "smoke", Command: "go test ./..."}},
		},
	})
	if err != nil {
		t.Fatalf("generate candidate: %v", err)
	}
	if candidate.ID == "" {
		t.Fatal("expected candidate id")
	}
	if !candidate.Validation.Valid {
		t.Fatalf("expected candidate valid, got %+v", candidate.Validation)
	}
	if candidate.Status != StatusValidated {
		t.Fatalf("expected status %s, got %s", StatusValidated, candidate.Status)
	}

	validated, err := manager.ValidateCandidate(candidate.ID)
	if err != nil {
		t.Fatalf("validate candidate: %v", err)
	}
	if !validated.Validation.Valid {
		t.Fatalf("expected validation to remain valid, got %+v", validated.Validation)
	}

	reviewed, err := manager.ReviewCandidate(candidate.ID, ReviewOptions{State: ReviewApproved, OperatorReason: "approved after review"})
	if err != nil {
		t.Fatalf("review candidate: %v", err)
	}
	if reviewed.ReviewState != ReviewApproved {
		t.Fatalf("expected approved review state, got %s", reviewed.ReviewState)
	}

	result, err := manager.ImportCandidate(candidate.ID, "import generated bundle")
	if err != nil {
		t.Fatalf("import candidate: %v", err)
	}
	if result.Manifest.Metadata.ID != "disk-space-extension" {
		t.Fatalf("expected imported skill id, got %s", result.Manifest.Metadata.ID)
	}
	if result.Candidate.Status != StatusImported {
		t.Fatalf("expected imported status, got %s", result.Candidate.Status)
	}
	stored, ok := skillManager.Get("disk-space-extension")
	if !ok {
		t.Fatal("expected imported skill in registry")
	}
	if stored.Metadata.Version != "1.0.0" {
		t.Fatalf("expected stored version 1.0.0, got %s", stored.Metadata.Version)
	}

	reloaded, err := NewManager(statePath, skillManager)
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}
	persisted, ok := reloaded.Get(candidate.ID)
	if !ok {
		t.Fatal("expected candidate persisted to state file")
	}
	if persisted.Status != StatusImported {
		t.Fatalf("expected persisted imported status, got %s", persisted.Status)
	}
}

func TestGenerateCandidateRejectsWrongKind(t *testing.T) {
	t.Parallel()

	manager, err := NewManager("", nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	_, err = manager.Generate(GenerateOptions{Bundle: Bundle{Kind: "channel_bundle"}})
	if err == nil {
		t.Fatal("expected invalid kind error")
	}
}

func TestImportRequiresApprovedReview(t *testing.T) {
	t.Parallel()

	skillPath := t.TempDir() + "/skills.yaml"
	if err := os.WriteFile(skillPath, []byte("skills:\n  entries: []\n"), 0o600); err != nil {
		t.Fatalf("write skill config: %v", err)
	}
	skillManager, err := skills.NewManager(skillPath, "")
	if err != nil {
		t.Fatalf("new skill manager: %v", err)
	}
	manager, err := NewManager("", skillManager)
	if err != nil {
		t.Fatalf("new extension manager: %v", err)
	}
	candidate, err := manager.Generate(GenerateOptions{Bundle: Bundle{Skill: skills.Manifest{Metadata: skills.Metadata{ID: "review-gated-skill", Name: "review-gated-skill", DisplayName: "Review Gated Skill", Version: "1.0.0", Vendor: "tars"}, Spec: skills.Spec{Type: "incident_skill", Planner: skills.Planner{Steps: []skills.PlannerStep{{ID: "step_1", Tool: "knowledge.search"}}}}}}})
	if err != nil {
		t.Fatalf("generate candidate: %v", err)
	}
	_, err = manager.ImportCandidate(candidate.ID, "try import without review")
	if err == nil {
		t.Fatal("expected import to require approved review state")
	}
}
