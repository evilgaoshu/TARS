import type { ConnectorLifecycle } from '@/lib/api/types';

export type ConnectorProbeResult = {
  status: 'idle' | 'success' | 'error';
  summary?: string;
};

export function connectorProbeResultFromLifecycle(
  lifecycle: Pick<ConnectorLifecycle, 'health'>,
  successFallback: string,
  failureFallback: string,
): ConnectorProbeResult {
  const healthy = String(lifecycle.health?.status || '').trim().toLowerCase() === 'healthy';
  const summary = lifecycle.health?.summary || (healthy ? successFallback : failureFallback);
  return {
    status: healthy ? 'success' : 'error',
    summary,
  };
}
