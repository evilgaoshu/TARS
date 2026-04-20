import type { AuthUserSummary } from '../api/types';

export type OpsTabID =
  | 'auth'
  | 'approval'
  | 'secrets'
  | 'providers'
  | 'connectors'
  | 'prompts'
  | 'desense'
  | 'advanced';

const CONFIG_OPS_READ_PERMISSIONS = ['configs.read', 'configs.write'];
const SSH_CREDENTIAL_READ_PERMISSIONS = ['ssh_credentials.read', 'ssh_credentials.write'];

export const OPS_ROUTE_PERMISSIONS = [
  ...CONFIG_OPS_READ_PERMISSIONS,
  ...SSH_CREDENTIAL_READ_PERMISSIONS,
  'knowledge.write',
];

export function hasPermission(user: AuthUserSummary | null | undefined, permission: string): boolean {
  if (!permission) return true;
  const permissions = user?.permissions || [];
  if (permissions.includes('*') || permissions.includes(permission)) {
    return true;
  }
  const normalized = permission.replace(/\./g, '/');
  return permissions.some((entry) => {
    if (!entry) return false;
    if (entry === '*') return true;
    const candidate = entry.replace(/\./g, '/');
    if (candidate === normalized) return true;
    if (candidate.endsWith('/*')) {
      return normalized === candidate.slice(0, -2) || normalized.startsWith(candidate.slice(0, -1));
    }
    return false;
  });
}

export function hasAnyPermission(user: AuthUserSummary | null | undefined, permissions: string[]): boolean {
  return permissions.some((permission) => hasPermission(user, permission));
}

export function canReadConfigOps(user: AuthUserSummary | null | undefined): boolean {
  return hasAnyPermission(user, CONFIG_OPS_READ_PERMISSIONS);
}

export function canWriteConfigOps(user: AuthUserSummary | null | undefined): boolean {
  return hasPermission(user, 'configs.write');
}

export function canReadSSHCredentials(user: AuthUserSummary | null | undefined): boolean {
  return hasAnyPermission(user, SSH_CREDENTIAL_READ_PERMISSIONS);
}

export function canWriteSSHCredentials(user: AuthUserSummary | null | undefined): boolean {
  return hasPermission(user, 'ssh_credentials.write');
}

export function canAccessOps(user: AuthUserSummary | null | undefined): boolean {
  return hasAnyPermission(user, OPS_ROUTE_PERMISSIONS);
}

export function canAccessOpsTab(user: AuthUserSummary | null | undefined, tabID: OpsTabID): boolean {
  switch (tabID) {
    case 'secrets':
      return canReadConfigOps(user) || canReadSSHCredentials(user);
    case 'advanced':
      return hasPermission(user, 'knowledge.write');
    default:
      return canReadConfigOps(user);
  }
}

export function canAccessIdentity(user: AuthUserSummary | null | undefined): boolean {
  return hasAnyPermission(user, [
    'auth.read',
    'users.read',
    'groups.read',
    'roles.read',
    'people.read',
  ]);
}
