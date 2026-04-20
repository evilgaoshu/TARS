import { api } from './client';
import type {
  AccessChannel,
  AccessConfigResponse,
  AccessGroup,
  AccessPerson,
  AccessRole,
  AccessUser,
  AuthChallengeResponse,
  AuthLoginResponse,
  AuthProviderInfo,
  AuthProviderListResponse,
  ChannelListResponse,
  GroupListResponse,
  MeResponse,
  PersonListResponse,
  ProviderCheckResponse,
  ProviderBindingsResponse,
  ProviderBindingsUpdateRequest,
  ProviderEntry,
  ProviderListModelsResponse,
  ProviderRegistryEntry,
  ProviderRegistryListResponse,
  RoleBindingRequest,
  RoleBindingsResponse,
  RoleListResponse,
  SessionInventoryResponse,
  UserListResponse,
} from './types';

function normalizeChannel(channel: AccessChannel): AccessChannel {
  const kind = channel.kind || channel.type;
  const usages = channel.usages?.length ? channel.usages : channel.capabilities;
  return {
    ...channel,
    kind,
    type: channel.type || kind,
    usages: usages || [],
    capabilities: channel.capabilities?.length ? channel.capabilities : usages || [],
  };
}

function serializeChannel(channel: AccessChannel): AccessChannel {
  const normalized = normalizeChannel(channel);
  return {
    ...normalized,
    kind: normalized.kind,
    type: normalized.type || normalized.kind,
    usages: normalized.usages,
    capabilities: normalized.capabilities,
  };
}

function normalizeProviderRegistryEntry(entry: ProviderRegistryEntry): ProviderRegistryEntry {
  return { ...entry };
}

export async function fetchAuthProviders(): Promise<AuthProviderListResponse> {
  const response = await api.get<AuthProviderListResponse>('/auth/providers');
  return response.data;
}

export async function fetchAuthProvider(providerID: string): Promise<AuthProviderInfo> {
  const response = await api.get<AuthProviderInfo>(`/auth/providers/${providerID}`);
  return response.data;
}

export async function createAuthProvider(provider: AuthProviderInfo): Promise<AuthProviderInfo> {
  const response = await api.post<AuthProviderInfo>('/auth/providers', { provider });
  return response.data;
}

export async function updateAuthProvider(providerID: string, provider: AuthProviderInfo): Promise<AuthProviderInfo> {
  const response = await api.put<AuthProviderInfo>(`/auth/providers/${providerID}`, { provider });
  return response.data;
}

export async function setAuthProviderEnabled(providerID: string, enabled: boolean): Promise<AuthProviderInfo> {
  const response = await api.post<AuthProviderInfo>(`/auth/providers/${providerID}/${enabled ? 'enable' : 'disable'}`, {});
  return response.data;
}

export async function loginWithToken(token: string, providerID = 'local_token'): Promise<AuthLoginResponse> {
  const response = await api.post<AuthLoginResponse>('/auth/login', {
    provider_id: providerID,
    token,
  });
  return response.data;
}

export async function loginWithPassword(providerID: string, username: string, password: string): Promise<AuthLoginResponse> {
  const response = await api.post<AuthLoginResponse>('/auth/login', {
    provider_id: providerID,
    username,
    password,
  });
  return response.data;
}

export async function startAuthChallenge(pendingToken: string): Promise<AuthChallengeResponse> {
  const response = await api.post<AuthChallengeResponse>('/auth/challenge', {
    pending_token: pendingToken,
  });
  return response.data;
}

export async function verifyAuthChallenge(pendingToken: string, challengeID: string, code: string): Promise<AuthLoginResponse> {
  const response = await api.post<AuthLoginResponse>('/auth/verify', {
    pending_token: pendingToken,
    challenge_id: challengeID,
    code,
  });
  return response.data;
}

export async function verifyAuthMFA(pendingToken: string, code: string): Promise<AuthLoginResponse> {
  const response = await api.post<AuthLoginResponse>('/auth/mfa/verify', {
    pending_token: pendingToken,
    code,
  });
  return response.data;
}

export async function fetchMe(token?: string): Promise<MeResponse> {
  const response = await api.get<MeResponse>('/me', {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return response.data;
}

export async function logoutSession(): Promise<void> {
  await api.post('/auth/logout');
}

export async function fetchAuthSessions(): Promise<SessionInventoryResponse> {
  const response = await api.get<SessionInventoryResponse>('/auth/sessions');
  return response.data;
}

export async function fetchUsers(params?: {
  q?: string;
  page?: number;
  limit?: number;
}): Promise<UserListResponse> {
  const response = await api.get<UserListResponse>('/users', { params });
  return response.data;
}

export async function createUser(user: AccessUser): Promise<AccessUser> {
  const response = await api.post<AccessUser>('/users', { user });
  return response.data;
}

export async function fetchUser(userID: string): Promise<AccessUser> {
  const response = await api.get<AccessUser>(`/users/${userID}`);
  return response.data;
}

export async function updateUser(userID: string, user: AccessUser): Promise<AccessUser> {
  const response = await api.put<AccessUser>(`/users/${userID}`, { user });
  return response.data;
}

export async function setUserEnabled(userID: string, enabled: boolean): Promise<AccessUser> {
  const response = await api.post<AccessUser>(`/users/${userID}/${enabled ? 'enable' : 'disable'}`, {});
  return response.data;
}

export async function fetchGroups(params?: {
  q?: string;
  page?: number;
  limit?: number;
}): Promise<GroupListResponse> {
  const response = await api.get<GroupListResponse>('/groups', { params });
  return response.data;
}

export async function createGroup(group: AccessGroup): Promise<AccessGroup> {
  const response = await api.post<AccessGroup>('/groups', { group });
  return response.data;
}

export async function fetchGroup(groupID: string): Promise<AccessGroup> {
  const response = await api.get<AccessGroup>(`/groups/${groupID}`);
  return response.data;
}

export async function updateGroup(groupID: string, group: AccessGroup): Promise<AccessGroup> {
  const response = await api.put<AccessGroup>(`/groups/${groupID}`, { group });
  return response.data;
}

export async function setGroupEnabled(groupID: string, enabled: boolean): Promise<AccessGroup> {
  const response = await api.post<AccessGroup>(`/groups/${groupID}/${enabled ? 'enable' : 'disable'}`, {});
  return response.data;
}

export async function fetchRoles(): Promise<RoleListResponse> {
  const response = await api.get<RoleListResponse>('/roles');
  return response.data;
}

export async function createRole(role: AccessRole): Promise<AccessRole> {
  const response = await api.post<AccessRole>('/roles', { role });
  return response.data;
}

export async function fetchRole(roleID: string): Promise<AccessRole> {
  const response = await api.get<AccessRole>(`/roles/${roleID}`);
  return response.data;
}

export async function updateRole(roleID: string, role: AccessRole): Promise<AccessRole> {
  const response = await api.put<AccessRole>(`/roles/${roleID}`, { role });
  return response.data;
}

export async function fetchRoleBindings(roleID: string): Promise<RoleBindingsResponse> {
  const response = await api.get<RoleBindingsResponse>(`/roles/${roleID}/bindings`);
  return response.data;
}

export async function bindRole(roleID: string, payload: RoleBindingRequest): Promise<AccessRole> {
  const response = await api.post<AccessRole>(`/roles/${roleID}/bindings`, payload);
  return response.data;
}

export async function fetchPeople(params?: {
  q?: string;
  page?: number;
  limit?: number;
}): Promise<PersonListResponse> {
  const response = await api.get<PersonListResponse>('/people', { params });
  return response.data;
}

export async function fetchPerson(personID: string): Promise<AccessPerson> {
  const response = await api.get<AccessPerson>(`/people/${personID}`);
  return response.data;
}

export async function createPerson(person: AccessPerson): Promise<AccessPerson> {
  const response = await api.post<AccessPerson>('/people', { person });
  return response.data;
}

export async function updatePerson(personID: string, person: AccessPerson): Promise<AccessPerson> {
  const response = await api.put<AccessPerson>(`/people/${personID}`, { person });
  return response.data;
}

export async function setPersonEnabled(personID: string, enabled: boolean): Promise<AccessPerson> {
  const response = await api.post<AccessPerson>(`/people/${personID}/${enabled ? 'enable' : 'disable'}`, {});
  return response.data;
}

export async function fetchChannels(params?: {
  q?: string;
  page?: number;
  limit?: number;
}): Promise<ChannelListResponse> {
  const response = await api.get<ChannelListResponse>('/channels', { params });
  return {
    ...response.data,
    items: (response.data.items || []).map(normalizeChannel),
  };
}

export async function fetchChannel(channelID: string): Promise<AccessChannel> {
  const response = await api.get<AccessChannel>(`/channels/${channelID}`);
  return normalizeChannel(response.data);
}

export async function createChannel(channel: AccessChannel): Promise<AccessChannel> {
  const response = await api.post<AccessChannel>('/channels', { channel: serializeChannel(channel) });
  return normalizeChannel(response.data);
}

export async function updateChannel(channelID: string, channel: AccessChannel): Promise<AccessChannel> {
  const response = await api.put<AccessChannel>(`/channels/${channelID}`, { channel: serializeChannel(channel) });
  return normalizeChannel(response.data);
}

export async function setChannelEnabled(channelID: string, enabled: boolean): Promise<AccessChannel> {
  const response = await api.post<AccessChannel>(`/channels/${channelID}/${enabled ? 'enable' : 'disable'}`, {});
  return normalizeChannel(response.data);
}

export async function fetchProviders(params?: {
  q?: string;
  page?: number;
  limit?: number;
}): Promise<ProviderRegistryListResponse> {
  const response = await api.get<ProviderRegistryListResponse>('/providers', { params });
  return {
    ...response.data,
    items: (response.data.items || []).map(normalizeProviderRegistryEntry),
  };
}

export async function fetchProvider(providerID: string): Promise<ProviderRegistryEntry> {
  const response = await api.get<ProviderRegistryEntry>(`/providers/${providerID}`);
  return normalizeProviderRegistryEntry(response.data);
}

export async function createProvider(provider: ProviderRegistryEntry): Promise<ProviderRegistryEntry> {
  const response = await api.post<ProviderRegistryEntry>('/providers', { provider });
  return normalizeProviderRegistryEntry(response.data);
}

export async function updateProvider(providerID: string, provider: ProviderRegistryEntry): Promise<ProviderRegistryEntry> {
  const response = await api.put<ProviderRegistryEntry>(`/providers/${providerID}`, { provider });
  return normalizeProviderRegistryEntry(response.data);
}

export async function setProviderEnabled(providerID: string, enabled: boolean): Promise<ProviderRegistryEntry> {
  const response = await api.post<ProviderRegistryEntry>(`/providers/${providerID}/${enabled ? 'enable' : 'disable'}`, {});
  return normalizeProviderRegistryEntry(response.data);
}

export async function fetchProviderBindings(): Promise<ProviderBindingsResponse> {
  const response = await api.get<ProviderBindingsResponse>('/providers/bindings');
  return response.data;
}

export async function updateProviderBindings(payload: ProviderBindingsUpdateRequest): Promise<ProviderBindingsResponse> {
  const response = await api.put<ProviderBindingsResponse>('/providers/bindings', payload);
  return response.data;
}

export async function listProviderModels(providerID?: string, provider?: ProviderRegistryEntry): Promise<ProviderListModelsResponse> {
  const response = await api.post<ProviderListModelsResponse>('/config/providers/models', {
    provider_id: providerID,
    provider,
  });
  return response.data;
}

export async function checkProviderAvailability(providerID?: string, model?: string, provider?: ProviderRegistryEntry): Promise<ProviderCheckResponse> {
  const response = await api.post<ProviderCheckResponse>('/config/providers/check', {
    provider_id: providerID,
    model,
    provider,
  });
  return response.data;
}

export function providerRegistryEntryToConfigEntry(entry: ProviderRegistryEntry): ProviderEntry {
  return {
    id: entry.id || '',
    vendor: entry.vendor,
    protocol: entry.protocol,
    base_url: entry.base_url,
    api_key: entry.api_key,
    api_key_ref: entry.api_key_ref,
    api_key_set: entry.api_key_set,
    enabled: entry.enabled,
    templates: entry.templates,
  };
}

export async function fetchAccessConfig(): Promise<AccessConfigResponse> {
  const response = await api.get<AccessConfigResponse>('/config/auth');
  return response.data;
}
