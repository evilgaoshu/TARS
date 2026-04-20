import { api } from './client';
import type { MsgTemplate, MsgTemplateListResponse } from './types';

function normalizeMsgTemplate(template: MsgTemplate): MsgTemplate {
  const status = template.status || (template.enabled ? 'active' : 'disabled');
  return {
    ...template,
    status,
    enabled: status === 'active',
    variable_schema: template.variable_schema || {},
    usage_refs: template.usage_refs || [],
  };
}

function serializeMsgTemplate(template: Partial<MsgTemplate>): Partial<MsgTemplate> {
  const status = template.status || (template.enabled ? 'active' : 'disabled');
  return {
    ...template,
    status,
    enabled: status === 'active',
  };
}

export async function fetchMsgTemplates(params?: {
  q?: string;
  page?: number;
  limit?: number;
}): Promise<MsgTemplateListResponse> {
  const response = await api.get<MsgTemplateListResponse>('/msg-templates', { params });
  return {
    ...response.data,
    items: (response.data.items || []).map(normalizeMsgTemplate),
  };
}

export async function fetchMsgTemplate(id: string): Promise<MsgTemplate> {
  const response = await api.get<MsgTemplate>(`/msg-templates/${id}`);
  return normalizeMsgTemplate(response.data);
}

export async function createMsgTemplate(
  template: Omit<MsgTemplate, 'id' | 'updated_at'>,
  operatorReason: string,
): Promise<MsgTemplate> {
  const response = await api.post<MsgTemplate>('/msg-templates', {
    template: serializeMsgTemplate(template),
    operator_reason: operatorReason,
  });
  return normalizeMsgTemplate(response.data);
}

export async function updateMsgTemplate(
  id: string,
  template: Partial<MsgTemplate>,
  operatorReason: string,
): Promise<MsgTemplate> {
  const response = await api.put<MsgTemplate>(`/msg-templates/${id}`, {
    template: { ...serializeMsgTemplate(template), id },
    operator_reason: operatorReason,
  });
  return normalizeMsgTemplate(response.data);
}

export async function setMsgTemplateEnabled(
  id: string,
  enabled: boolean,
  operatorReason: string,
): Promise<MsgTemplate> {
  const response = await api.post<MsgTemplate>(
    `/msg-templates/${id}/${enabled ? 'enable' : 'disable'}`,
    { operator_reason: operatorReason },
  );
  return normalizeMsgTemplate(response.data);
}

export interface MsgTemplateRenderResult {
  template_id: string;
  subject: string;
  body: string;
}

/**
 * Server-side render a template with variable substitution.
 * Pass a vars map (e.g. { AlertName: "CPUHigh" }); missing keys stay as {{Placeholder}}.
 */
export async function renderMsgTemplate(
  id: string,
  vars?: Record<string, string>,
): Promise<MsgTemplateRenderResult> {
  const response = await api.post<MsgTemplateRenderResult>(`/msg-templates/${id}/render`, {
    vars: vars ?? {},
  });
  return response.data;
}

/**
 * Export a template as a downloadable JSON or YAML file.
 * Returns the raw blob and a suggested filename.
 */
export async function exportMsgTemplate(
  id: string,
  format: 'json' | 'yaml' = 'json',
): Promise<{ blob: Blob; filename: string }> {
  const response = await api.get(`/msg-templates/${id}/export`, {
    params: { format },
    responseType: 'blob',
  });
  const contentDisposition = (response.headers as Record<string, string>)['content-disposition'] ?? '';
  const match = /filename="([^"]+)"/.exec(contentDisposition);
  const filename = match ? match[1] : `template-${id}.${format}`;
  return { blob: response.data as Blob, filename };
}
