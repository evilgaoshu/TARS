// Package org — context.go
// ORG-N4: Org context propagation.
//
// Clients that operate in a multi-tenant environment can signal their
// org/tenant/workspace by setting HTTP request headers:
//
//	X-Tars-Org-ID        — organization id
//	X-Tars-Tenant-ID     — tenant id
//	X-Tars-Workspace-ID  — workspace id
//
// The middleware extracts these headers (or falls back to query params)
// and injects an OrgContext into the request context. Handlers that need
// tenant-aware filtering call RequestOrgContext(r) instead of reading
// headers directly.
package org

import (
	"context"
	"net/http"
	"strings"
)

// ContextKey is the type-safe key used to store OrgContext in request contexts.
type contextKey struct{}

// OrgContext carries the resolved org/tenant/workspace for a single request.
type OrgContext struct {
	OrgID       string
	TenantID    string
	WorkspaceID string
}

// IsEmpty returns true when no org context was set.
func (c OrgContext) IsEmpty() bool {
	return c.OrgID == "" && c.TenantID == "" && c.WorkspaceID == ""
}

// WithOrgContext stores an OrgContext on a context.
func WithOrgContext(ctx context.Context, orgCtx OrgContext) context.Context {
	return context.WithValue(ctx, contextKey{}, orgCtx)
}

// RequestOrgContext extracts the OrgContext from a request context.
// Returns zero value if no context was set.
func RequestOrgContext(r *http.Request) OrgContext {
	if v, ok := r.Context().Value(contextKey{}).(OrgContext); ok {
		return v
	}
	return OrgContext{}
}

// OrgContextMiddleware is an HTTP middleware (compatible with net/http.Handler)
// that extracts org identity from request headers or query parameters and
// injects it into the request context.
//
// Header precedence (first wins):
//  1. X-Tars-Org-ID / X-Tars-Tenant-ID / X-Tars-Workspace-ID
//  2. Query params ?org_id=... / ?tenant_id=... / ?workspace_id=...
//  3. Default context from the Manager (DefaultOrg / DefaultTenant / DefaultWorkspace)
func OrgContextMiddleware(m *Manager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgCtx := extractOrgContext(r)
		if orgCtx.IsEmpty() && m != nil {
			// Fall back to platform defaults so single-tenant code always
			// has a valid context without requiring clients to set headers.
			orgCtx = OrgContext{
				OrgID:       m.DefaultOrg().ID,
				TenantID:    m.DefaultTenant().ID,
				WorkspaceID: m.DefaultWorkspace().ID,
			}
		}
		next.ServeHTTP(w, r.WithContext(WithOrgContext(r.Context(), orgCtx)))
	})
}

func extractOrgContext(r *http.Request) OrgContext {
	c := OrgContext{
		OrgID:       strings.TrimSpace(r.Header.Get("X-Tars-Org-ID")),
		TenantID:    strings.TrimSpace(r.Header.Get("X-Tars-Tenant-ID")),
		WorkspaceID: strings.TrimSpace(r.Header.Get("X-Tars-Workspace-ID")),
	}
	q := r.URL.Query()
	if c.OrgID == "" {
		c.OrgID = strings.TrimSpace(q.Get("org_id"))
	}
	if c.TenantID == "" {
		c.TenantID = strings.TrimSpace(q.Get("tenant_id"))
	}
	if c.WorkspaceID == "" {
		c.WorkspaceID = strings.TrimSpace(q.Get("workspace_id"))
	}
	return c
}
