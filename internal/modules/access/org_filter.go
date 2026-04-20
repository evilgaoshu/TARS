// Package access — org_filter.go
// ORG-N2: Tenant-aware query filters on access objects.
//
// Adds OrgFilter struct and filtered variants of the existing List* methods.
// The unfiltered methods continue to work for backward compat; handlers that
// have an org context should use the filtered variants instead.
package access

import "strings"

// OrgFilter carries optional org/tenant/workspace scoping for list queries.
// Empty string fields are treated as "match all" (wildcard).
type OrgFilter struct {
	OrgID       string
	TenantID    string
	WorkspaceID string
}

// IsEmpty returns true when no filter is set (wildcard — returns all objects).
func (f OrgFilter) IsEmpty() bool {
	return f.OrgID == "" && f.TenantID == "" && f.WorkspaceID == ""
}

func matchesOrgFilter(orgID, tenantID, workspaceID string, f OrgFilter) bool {
	if f.OrgID != "" && orgID != "" && !strings.EqualFold(orgID, f.OrgID) {
		return false
	}
	if f.TenantID != "" && tenantID != "" && !strings.EqualFold(tenantID, f.TenantID) {
		return false
	}
	if f.WorkspaceID != "" && workspaceID != "" && !strings.EqualFold(workspaceID, f.WorkspaceID) {
		return false
	}
	return true
}

// ListUsersFiltered returns users matching the OrgFilter.
// Objects without org affiliation (empty OrgID) are always included so
// that pre-existing records remain visible in single-tenant mode.
func (m *Manager) ListUsersFiltered(f OrgFilter) []User {
	if m == nil {
		return nil
	}
	if f.IsEmpty() {
		return m.ListUsers()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []User
	for _, u := range m.config.Users {
		if matchesOrgFilter(u.OrgID, u.TenantID, u.WorkspaceID, f) {
			out = append(out, u)
		}
	}
	return cloneUsers(out)
}

// ListGroupsFiltered returns groups matching the OrgFilter.
func (m *Manager) ListGroupsFiltered(f OrgFilter) []Group {
	if m == nil {
		return nil
	}
	if f.IsEmpty() {
		return m.ListGroups()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Group
	for _, g := range m.config.Groups {
		if matchesOrgFilter(g.OrgID, g.TenantID, g.WorkspaceID, f) {
			out = append(out, g)
		}
	}
	return cloneGroups(out)
}

// ListPeopleFiltered returns people matching the OrgFilter.
func (m *Manager) ListPeopleFiltered(f OrgFilter) []Person {
	if m == nil {
		return nil
	}
	if f.IsEmpty() {
		return m.ListPeople()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Person
	for _, p := range m.config.People {
		if matchesOrgFilter(p.OrgID, p.TenantID, p.WorkspaceID, f) {
			out = append(out, p)
		}
	}
	return clonePeople(out)
}

// ListChannelsFiltered returns channels matching the OrgFilter.
func (m *Manager) ListChannelsFiltered(f OrgFilter) []Channel {
	if m == nil {
		return nil
	}
	if f.IsEmpty() {
		return m.ListChannels()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Channel
	for _, c := range m.config.Channels {
		if matchesOrgFilter(c.OrgID, c.TenantID, c.WorkspaceID, f) {
			out = append(out, c)
		}
	}
	return cloneChannels(out)
}
