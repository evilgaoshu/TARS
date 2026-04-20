package approvalrouting

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolverPrefersServiceOwnerThenFallsBackToOncall(t *testing.T) {
	t.Parallel()

	router := New(Config{
		ProhibitSelfApproval: true,
		ServiceOwners: map[string][]string{
			"api": {"owner-room"},
		},
		OncallGroups: map[string][]string{
			"default": {"oncall-room"},
		},
		CommandAllowlist: map[string][]string{
			"api": {"systemctl restart api"},
		},
	})

	serviceRoute := router.Resolve(map[string]interface{}{
		"labels": map[string]string{"service": "api"},
	}, "tars", "ops-room")
	if serviceRoute.GroupKey != "service_owner:api" {
		t.Fatalf("unexpected service owner route: %+v", serviceRoute)
	}
	if !reflect.DeepEqual(serviceRoute.Targets, []string{"owner-room"}) {
		t.Fatalf("unexpected service owner targets: %+v", serviceRoute.Targets)
	}

	fallbackRoute := router.Resolve(map[string]interface{}{
		"labels": map[string]string{"service": "worker"},
	}, "tars", "ops-room")
	if fallbackRoute.GroupKey != "oncall:default" {
		t.Fatalf("unexpected fallback route: %+v", fallbackRoute)
	}
	if !reflect.DeepEqual(fallbackRoute.Targets, []string{"oncall-room"}) {
		t.Fatalf("unexpected fallback targets: %+v", fallbackRoute.Targets)
	}
	if !reflect.DeepEqual(router.AllowedCommandPrefixes("api"), []string{"systemctl restart api"}) {
		t.Fatalf("unexpected command allowlist: %+v", router.AllowedCommandPrefixes("api"))
	}
}

func TestLoadParsesApprovalFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "approvals.yaml")
	if err := os.WriteFile(path, []byte(`
approval:
  prohibit_self_approval: true
  routing:
    service_owner:
      api:
        - owner-room
    oncall_group:
      default:
        - oncall-room
  execution:
    command_allowlist:
      api:
        - "systemctl restart api"
`), 0o600); err != nil {
		t.Fatalf("write approvals file: %v", err)
	}

	router, err := Load(path)
	if err != nil {
		t.Fatalf("load approvals: %v", err)
	}

	route := router.Resolve(map[string]interface{}{
		"labels": map[string]string{"service": "api"},
	}, "tars", "ops-room")
	if route.SourceLabel != "service owner(api)" {
		t.Fatalf("unexpected source label: %+v", route)
	}
	if !reflect.DeepEqual(router.AllowedCommandPrefixes("api"), []string{"systemctl restart api"}) {
		t.Fatalf("unexpected command allowlist: %+v", router.AllowedCommandPrefixes("api"))
	}
}
