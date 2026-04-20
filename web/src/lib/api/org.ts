import { api } from './client';
import type {
  OrgContextResponse,
  OrgItem,
  OrgListResponse,
  OrgPolicy,
  ResolvedPolicy,
  TenantItem,
  TenantListResponse,
  TenantPolicy,
  WorkspaceItem,
  WorkspaceListResponse,
} from './types';

// ---- Organizations --------------------------------------------------------

export async function listOrganizations(): Promise<OrgListResponse> {
  const r = await api.get<OrgListResponse>('/organizations');
  return r.data;
}

export async function getOrganization(id: string): Promise<OrgItem> {
  const r = await api.get<OrgItem>(`/organizations/${id}`);
  return r.data;
}

export async function createOrganization(body: Partial<OrgItem>): Promise<OrgItem> {
  const r = await api.post<OrgItem>('/organizations', body);
  return r.data;
}

export async function updateOrganization(id: string, body: Partial<OrgItem>): Promise<OrgItem> {
  const r = await api.put<OrgItem>(`/organizations/${id}`, body);
  return r.data;
}

export async function setOrganizationStatus(id: string, enable: boolean): Promise<OrgItem> {
  const r = await api.post<OrgItem>(`/organizations/${id}/${enable ? 'enable' : 'disable'}`, {});
  return r.data;
}

// ---- Tenants ---------------------------------------------------------------

export async function listTenants(orgId?: string): Promise<TenantListResponse> {
  const params = orgId ? `?org_id=${orgId}` : '';
  const r = await api.get<TenantListResponse>(`/tenants${params}`);
  return r.data;
}

export async function getTenant(id: string): Promise<TenantItem> {
  const r = await api.get<TenantItem>(`/tenants/${id}`);
  return r.data;
}

export async function createTenant(body: Partial<TenantItem>): Promise<TenantItem> {
  const r = await api.post<TenantItem>('/tenants', body);
  return r.data;
}

export async function updateTenant(id: string, body: Partial<TenantItem>): Promise<TenantItem> {
  const r = await api.put<TenantItem>(`/tenants/${id}`, body);
  return r.data;
}

export async function setTenantStatus(id: string, enable: boolean): Promise<TenantItem> {
  const r = await api.post<TenantItem>(`/tenants/${id}/${enable ? 'enable' : 'disable'}`, {});
  return r.data;
}

// ---- Workspaces ------------------------------------------------------------

export async function listWorkspaces(tenantId?: string): Promise<WorkspaceListResponse> {
  const params = tenantId ? `?tenant_id=${tenantId}` : '';
  const r = await api.get<WorkspaceListResponse>(`/workspaces${params}`);
  return r.data;
}

export async function getWorkspace(id: string): Promise<WorkspaceItem> {
  const r = await api.get<WorkspaceItem>(`/workspaces/${id}`);
  return r.data;
}

export async function createWorkspace(body: Partial<WorkspaceItem>): Promise<WorkspaceItem> {
  const r = await api.post<WorkspaceItem>('/workspaces', body);
  return r.data;
}

export async function updateWorkspace(id: string, body: Partial<WorkspaceItem>): Promise<WorkspaceItem> {
  const r = await api.put<WorkspaceItem>(`/workspaces/${id}`, body);
  return r.data;
}

export async function setWorkspaceStatus(id: string, enable: boolean): Promise<WorkspaceItem> {
  const r = await api.post<WorkspaceItem>(`/workspaces/${id}/${enable ? 'enable' : 'disable'}`, {});
  return r.data;
}

// ---- Org Context -----------------------------------------------------------

export async function getOrgContext(): Promise<OrgContextResponse> {
  const r = await api.get<OrgContextResponse>('/org/context');
  return r.data;
}

// ---- Org Policy (ORG-N5) ---------------------------------------------------

export async function getOrgPolicy(orgId: string): Promise<OrgPolicy> {
  const r = await api.get<OrgPolicy>(`/org/policy?org_id=${orgId}`);
  return r.data;
}

export async function setOrgPolicy(policy: OrgPolicy): Promise<OrgPolicy> {
  const r = await api.put<OrgPolicy>('/org/policy', policy);
  return r.data;
}

export async function getTenantPolicy(tenantId: string): Promise<TenantPolicy> {
  const r = await api.get<TenantPolicy>(`/tenants/${tenantId}/policy`);
  return r.data;
}

export async function setTenantPolicy(tenantId: string, policy: TenantPolicy): Promise<TenantPolicy> {
  const r = await api.put<TenantPolicy>(`/tenants/${tenantId}/policy`, policy);
  return r.data;
}

export async function resolvePolicy(orgId: string, tenantId: string): Promise<ResolvedPolicy> {
  const r = await api.get<ResolvedPolicy>(`/org/policy/resolve?org_id=${orgId}&tenant_id=${tenantId}`);
  return r.data;
}
