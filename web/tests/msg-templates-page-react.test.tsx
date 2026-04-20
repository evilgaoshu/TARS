// @vitest-environment jsdom

import React from 'react';
import { act } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'react-dom/client';

const createMsgTemplateMock = vi.fn();
const exportMsgTemplateMock = vi.fn();
const fetchMsgTemplatesMock = vi.fn();
const renderMsgTemplateMock = vi.fn();
const setMsgTemplateEnabledMock = vi.fn();
const updateMsgTemplateMock = vi.fn();

vi.mock('../src/lib/api/msgtpl', () => ({
  createMsgTemplate: createMsgTemplateMock,
  exportMsgTemplate: exportMsgTemplateMock,
  fetchMsgTemplates: fetchMsgTemplatesMock,
  renderMsgTemplate: renderMsgTemplateMock,
  setMsgTemplateEnabled: setMsgTemplateEnabledMock,
  updateMsgTemplate: updateMsgTemplateMock,
}));

vi.mock('../src/hooks/useI18n', () => ({
  useI18n: () => ({
    t: (key: string) => {
      const messages: Record<string, string> = {
        'msgtpl.title': 'Notification Templates',
        'msgtpl.subtitle': 'Template control plane',
        'msgtpl.loading': 'Loading message templates...',
        'msgtpl.sidebar.title': 'Template Registry',
        'msgtpl.sidebar.desc': 'Template registry',
        'msgtpl.sidebar.new': 'New Template',
        'msgtpl.sidebar.empty': 'No templates of this type.',
        'msgtpl.detail.newTitle': 'New Template',
        'msgtpl.detail.newSubtitle': 'Create template',
        'msgtpl.detail.editSubtitle': 'Edit template',
        'msgtpl.detail.disable': 'Disable',
        'msgtpl.detail.enable': 'Enable',
        'msgtpl.detail.preview': 'Preview',
        'msgtpl.detail.edit': 'Edit',
        'msgtpl.detail.exportJson': 'Export JSON',
        'msgtpl.detail.exportYaml': 'Export YAML',
        'msgtpl.detail.templateId': 'Template ID',
        'msgtpl.detail.idHint': 'id hint',
        'msgtpl.detail.displayName': 'Display Name',
        'msgtpl.detail.type': 'Type',
        'msgtpl.detail.locale': 'Locale',
        'msgtpl.detail.subject': 'Subject',
        'msgtpl.detail.subjectHint': 'subject hint',
        'msgtpl.detail.body': 'Body',
        'msgtpl.detail.bodyHint': 'body hint',
        'msgtpl.detail.variables': 'Available Variables',
        'msgtpl.detail.saving': 'Saving...',
        'msgtpl.detail.create': 'Create Template',
        'msgtpl.detail.saveChanges': 'Save Changes',
        'msgtpl.detail.reset': 'Reset',
        'msgtpl.empty.title': 'Select a Template',
        'msgtpl.empty.desc': 'Select one',
        'msgtpl.stat.templates': 'Templates',
        'msgtpl.stat.enabled': 'Enabled',
        'msgtpl.stat.types': 'Types',
        'msgtpl.stat.storage': 'Storage',
        'msgtpl.stat.totalDesc': 'total templates',
        'msgtpl.stat.enabledDesc': 'currently active',
        'msgtpl.stat.typesDesc': 'types',
        'msgtpl.stat.storageDesc': 'api',
        'msgtpl.enabled': 'Template enabled.',
        'msgtpl.disabled': 'Template disabled.',
        'msgtpl.saved': 'Template saved.',
        'msgtpl.created': 'Template created.',
        'msgtpl.opFailed': 'Operation failed.',
        'msgtpl.dialog.title': 'New Template',
        'msgtpl.dialog.desc': 'Create a new message template.',
        'msgtpl.dialog.creating': 'Creating...',
        'msgtpl.dialog.required': 'Required',
        'msgtpl.dialog.bodyHint': 'body hint',
      };
      return messages[key] ?? key;
    },
  }),
}));

vi.mock('../src/hooks/ui/useNotify', () => ({
  useNotify: () => ({
    success: vi.fn(),
    error: vi.fn(),
  }),
}));

vi.mock('../src/components/operator/GuidedFormDialog', () => ({
  GuidedFormDialog: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

async function flush() {
  await act(async () => {
    await Promise.resolve();
  });
}

describe('MsgTemplatesPage', () => {
  beforeEach(() => {
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    fetchMsgTemplatesMock.mockReset();
    updateMsgTemplateMock.mockReset();
    setMsgTemplateEnabledMock.mockReset();
    createMsgTemplateMock.mockReset();
    exportMsgTemplateMock.mockReset();
    renderMsgTemplateMock.mockReset();

    fetchMsgTemplatesMock.mockResolvedValue({
      items: [
        {
          id: 'diagnosis-zh-CN',
          type: 'diagnosis',
          locale: 'zh-CN',
          name: 'Diagnosis Chinese',
          status: 'draft',
          enabled: false,
          variable_schema: {
            AlertName: 'string',
            Summary: 'markdown',
          },
          usage_refs: ['trigger:trg-1', 'channel:inbox-primary'],
          content: {
            subject: '[TARS] Diagnosis',
            body: 'Alert {{AlertName}}',
          },
          updated_at: '2026-03-29T08:00:00Z',
        },
      ],
      page: 1,
      limit: 20,
      total: 1,
      has_next: false,
    });

    updateMsgTemplateMock.mockImplementation(async (_id: string, payload: unknown) => payload);
  });

  it('shows lifecycle, variable schema, usage refs, and preserves lifecycle on save', async () => {
    const { MsgTemplatesPage } = await import('../src/pages/msg-templates/MsgTemplatesPage');

    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(<MsgTemplatesPage />);
    });
    await flush();
    await flush();

    expect(container.textContent).toContain('draft');
    expect(container.textContent).toContain('AlertName');
    expect(container.textContent).toContain('trigger:trg-1');

    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Save Changes'));
    expect(saveButton).toBeTruthy();

    await act(async () => {
      saveButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flush();

    expect(updateMsgTemplateMock).toHaveBeenCalledTimes(1);
    const payload = updateMsgTemplateMock.mock.calls[0]?.[1] as { status?: string; variable_schema?: Record<string, string>; usage_refs?: string[] };
    expect(payload.status).toBe('draft');
    expect(payload.variable_schema?.AlertName).toBe('string');
    expect(payload.usage_refs).toContain('trigger:trg-1');

    root.unmount();
    container.remove();
  });
});
