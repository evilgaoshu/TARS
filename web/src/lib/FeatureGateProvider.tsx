import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { fetchSetupStatus } from './api/ops';
import type { Capability, CapabilityState } from './featureGates';
import { FeatureGateContext } from './FeatureGateContext';
import { useAuth } from '../hooks/useAuth';

const defaultState: CapabilityState = { enabled: false, status: 'disabled' };

const initialCapabilities: Record<Capability, CapabilityState> = {
  'identity.dex': defaultState,
  'identity.oidc': { enabled: false, status: 'requires_config' },
  'identity.ldap': { enabled: false, status: 'coming_soon' },
  'identity.totp': { enabled: false, status: 'requires_config' },
  'identity.mfa': { enabled: false, status: 'requires_config' },
  'identity.local_password': { enabled: false, status: 'requires_config' },
  'identity.local_token': { enabled: true, status: 'available' },
  'channels.telegram': defaultState,
  'channels.inbox': { enabled: true, status: 'available' },
  'channels.email': defaultState,
  'channels.slack': defaultState,
  'channels.webhook': defaultState,
  'connectors.ssh': { enabled: true, status: 'available' },
  'connectors.victoriametrics': { enabled: true, status: 'available' },
  'connectors.victorialogs': { enabled: true, status: 'available' },
  'providers.gemini': defaultState,
  'providers.dashscope': defaultState,
  'skills.marketplace': { enabled: false, status: 'coming_soon' },
  'automations.workflows': { enabled: true, status: 'available' },
  'extensions.bundles': { enabled: true, status: 'available' },
  'knowledge.vector': defaultState,
  'observability.internal': { enabled: true, status: 'available' },
  'observability.victorialogs': { enabled: true, status: 'available' },
  'observability.victoriatraces': { enabled: false, status: 'coming_soon' },
  'ops.destructive': { enabled: false, status: 'requires_config' },
};

export const FeatureGateProvider = ({ children }: { children: ReactNode }) => {
  const { isAuthenticated } = useAuth();
  const [capabilities, setCapabilities] = useState<Record<Capability, CapabilityState>>(initialCapabilities);
  const [loading, setLoading] = useState(true);

  const refresh = async () => {
    if (!isAuthenticated) {
      setCapabilities(initialCapabilities);
      setLoading(false);
      return;
    }
    try {
      const status = await fetchSetupStatus();
      const next: Record<Capability, CapabilityState> = { ...initialCapabilities };

      // Identity
      const authProviderID = status.initialization.auth_provider_id?.toLowerCase() ?? '';
      const authConfigured = status.initialization.auth_configured;
      const oidcConfigured = authConfigured && (authProviderID.includes('oidc') || authProviderID.includes('oauth'));
      const localPasswordConfigured = authConfigured && authProviderID.includes('local_password');
      next['identity.dex'] = {
        enabled: authConfigured && authProviderID.includes('dex'),
        status: authConfigured && authProviderID.includes('dex') ? 'available' : 'requires_config',
      };
      next['identity.oidc'] = {
        enabled: oidcConfigured,
        status: oidcConfigured ? 'available' : 'requires_config',
      };
      next['identity.local_password'] = {
        enabled: localPasswordConfigured,
        status: localPasswordConfigured ? 'available' : 'requires_config',
      };
      next['identity.totp'] = {
        enabled: authConfigured && status.initialization.mode === 'runtime',
        status: authConfigured && status.initialization.mode === 'runtime' ? 'available' : 'requires_config',
      };
      
      // Channels
      const availableChannels = status.connectors.kinds || [];
      next['channels.telegram'] = {
        enabled: status.telegram.configured,
        status: status.telegram.configured ? 'available' : 'requires_config',
        message: status.telegram.configured ? undefined : 'Requires Telegram token',
      };
      next['channels.slack'] = {
        enabled: availableChannels.includes('slack'),
        status: availableChannels.includes('slack') ? 'available' : 'coming_soon',
      };

      // Connectors
      const kinds = status.connectors.kinds || [];
      next['connectors.ssh'] = {
        enabled: true,
        status: kinds.includes('ssh') ? 'available' : 'requires_config',
      };
      next['connectors.victoriametrics'] = {
        enabled: true,
        status: kinds.includes('victoriametrics') ? 'available' : 'requires_config',
      };
      next['connectors.victorialogs'] = {
        enabled: true,
        status: kinds.includes('victorialogs') || kinds.includes('logs') ? 'available' : 'requires_config',
      };

      // Providers
      next['providers.gemini'] = {
        enabled: status.model.configured && status.model.vendor === 'google',
        status: status.model.configured ? 'available' : 'requires_config',
      };

      // Knowledge
      next['knowledge.vector'] = {
        enabled: status.features.knowledge_ingest_enabled,
        status: status.features.knowledge_ingest_enabled ? 'available' : 'requires_config',
      };

      setCapabilities(next);
    } catch (error) {
      console.warn('Failed to fetch capability status, falling back to safe defaults', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void refresh();
  }, [isAuthenticated]);

  return (
    <FeatureGateContext.Provider value={{ capabilities, loading, refresh }}>
      {children}
    </FeatureGateContext.Provider>
  );
};
