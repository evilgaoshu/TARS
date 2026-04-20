import { createContext, useContext } from 'react';
import type { Capability, CapabilityState } from './featureGates';

export interface FeatureGateContextType {
  capabilities: Record<Capability, CapabilityState>;
  loading: boolean;
  refresh: () => Promise<void>;
}

export const FeatureGateContext = createContext<FeatureGateContextType | undefined>(undefined);

export const useCapabilities = () => {
  const context = useContext(FeatureGateContext);
  if (context === undefined) {
    throw new Error('useCapabilities must be used within a FeatureGateProvider');
  }
  return context;
};
