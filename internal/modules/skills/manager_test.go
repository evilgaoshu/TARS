package skills

import (
	"os"
	"testing"
)

func TestRollback(t *testing.T) {
	t.Parallel()

	configPath := t.TempDir() + "/skills.yaml"
	if err := os.WriteFile(configPath, []byte(`skills:
  entries:
    - api_version: tars.skill/v1alpha1
      kind: skill_package
      metadata:
        id: sample-skill
        name: sample-skill
        display_name: Original Skill
        vendor: tars
      spec:
        planner:
          steps:
            - id: step_1
              tool: knowledge.search
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	manager, err := NewManager(configPath, "")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	// Initial Upsert
	_, _, err = manager.Upsert(UpsertOptions{
		Manifest: Manifest{
			APIVersion: "tars.skill/v1alpha1",
			Kind:       "skill_package",
			Metadata: Metadata{
				ID:          "sample-skill",
				DisplayName: "Original Skill",
				Name:        "sample-skill",
			},
		},
		Reason: "initial",
	})
	if err != nil {
		t.Fatalf("upsert initial: %v", err)
	}

	// Upsert a change
	_, _, err = manager.Upsert(UpsertOptions{
		Manifest: Manifest{
			APIVersion: "tars.skill/v1alpha1",
			Kind:       "skill_package",
			Metadata: Metadata{
				ID:          "sample-skill",
				DisplayName: "Updated Skill",
				Name:        "sample-skill",
			},
		},
		Reason: "update",
	})
	if err != nil {
		t.Fatalf("upsert update: %v", err)
	}

	rolledBack, _, err := manager.Rollback("sample-skill", RollbackOptions{OperatorReason: "rollback"})
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	t.Logf("Rolled back to: %s", rolledBack.Metadata.DisplayName)
	if rolledBack.Metadata.DisplayName != "Original Skill" {
		t.Fatalf("expected rollback to original display name, got %s", rolledBack.Metadata.DisplayName)
	}
}
