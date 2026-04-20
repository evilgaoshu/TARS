import { api } from "./client";
import type { AgentRole, AgentRoleListResponse } from "./types";

function normalizeAgentRole(role: AgentRole): AgentRole {
  const status = role.status === "enabled" ? "active" : role.status;
  return {
    ...role,
    status,
  };
}

function serializeAgentRole(role: Partial<AgentRole>): Partial<AgentRole> {
  const status = role.status === "enabled" ? "active" : role.status;
  return {
    ...role,
    status,
  };
}

export async function fetchAgentRoles(params?: {
  q?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}): Promise<AgentRoleListResponse> {
  const response = await api.get<AgentRoleListResponse>("/agent-roles", {
    params,
  });
  return {
    ...response.data,
    items: (response.data.items || []).map(normalizeAgentRole),
  };
}

export async function fetchAgentRole(roleID: string): Promise<AgentRole> {
  const response = await api.get<AgentRole>(`/agent-roles/${roleID}`);
  return normalizeAgentRole(response.data);
}

export async function createAgentRole(
  role: Partial<AgentRole>,
): Promise<AgentRole> {
  const response = await api.post<AgentRole>(
    "/agent-roles",
    serializeAgentRole(role),
  );
  return normalizeAgentRole(response.data);
}

export async function updateAgentRole(
  roleID: string,
  role: Partial<AgentRole>,
): Promise<AgentRole> {
  const response = await api.put<AgentRole>(
    `/agent-roles/${roleID}`,
    serializeAgentRole(role),
  );
  return normalizeAgentRole(response.data);
}

export async function deleteAgentRole(roleID: string): Promise<void> {
  await api.delete(`/agent-roles/${roleID}`);
}

export async function setAgentRoleEnabled(
  roleID: string,
  enabled: boolean,
): Promise<AgentRole> {
  const response = await api.post<AgentRole>(
    `/agent-roles/${roleID}/${enabled ? "enable" : "disable"}`,
    {},
  );
  return normalizeAgentRole(response.data);
}
