export type Capability =
  | 'identity.dex'
  | 'identity.oidc'
  | 'identity.ldap'
  | 'identity.totp'
  | 'identity.mfa'
  | 'identity.local_password'
  | 'identity.local_token'
  | 'channels.telegram'
  | 'channels.inbox'
  | 'channels.email'
  | 'channels.slack'
  | 'channels.webhook'
  | 'connectors.ssh'
  | 'connectors.victoriametrics'
  | 'connectors.victorialogs'
  | 'providers.gemini'
  | 'providers.dashscope'
  | 'skills.marketplace'
  | 'automations.workflows'
  | 'extensions.bundles'
  | 'knowledge.vector'
  | 'observability.internal'
  | 'observability.victorialogs'
  | 'observability.victoriatraces'
  | 'ops.destructive';

export interface CapabilityState {
  enabled: boolean;
  status?: 'available' | 'coming_soon' | 'requires_config' | 'disabled';
  message?: string;
}

export function isEnabled(capabilities: Record<Capability, CapabilityState>, cap: Capability): boolean {
  return capabilities[cap]?.enabled ?? false;
}

export function getStatus(capabilities: Record<Capability, CapabilityState>, cap: Capability): CapabilityState['status'] {
  return capabilities[cap]?.status ?? 'disabled';
}
