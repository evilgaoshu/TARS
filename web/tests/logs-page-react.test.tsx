// @vitest-environment jsdom

import { act } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'react-dom/client';

const fetchLogsMock = vi.fn();

vi.mock('../src/lib/api/ops', () => {
  return {
    fetchLogs: fetchLogsMock,
    getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
  };
});

async function flush() {
  await act(async () => {
    await Promise.resolve();
  });
}

describe('LogsPage', () => {
  beforeEach(() => {
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    fetchLogsMock.mockReset();
    fetchLogsMock.mockResolvedValue({
      items: [
        {
          id: 'log-1',
          timestamp: '2026-03-29T08:00:00Z',
          level: 'error',
          component: 'scheduler',
          message: 'automation run failed',
          trace_id: 'trace-1',
        },
      ],
      page: 1,
      limit: 20,
      total: 1,
      has_next: false,
    });
  });

  it('renders a real logs console surface', async () => {
    const { LogsPage } = await import('../src/pages/logs/LogsPage');
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(<LogsPage />);
    });
    await flush();
    await flush();

    expect(fetchLogsMock).toHaveBeenCalled();
    expect(container.textContent).toContain('Runtime Logs');
    expect(container.textContent).toContain('scheduler');
    expect(container.textContent).toContain('automation run failed');

    root.unmount();
    container.remove();
  });
});
