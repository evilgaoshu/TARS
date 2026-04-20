import { api } from './client';
import type { TriggerDTO, TriggerListResponse, TriggerUpsertRequest } from './types';

function normalizeTrigger(trigger: TriggerDTO): TriggerDTO {
  const channelID = trigger.channel_id || trigger.channel;
  return {
    ...trigger,
    channel_id: channelID,
    channel: trigger.channel || channelID || '',
    automation_job_id: trigger.automation_job_id || '',
    governance: trigger.governance || '',
  };
}

function serializeTrigger(req: Partial<TriggerUpsertRequest>): Partial<TriggerUpsertRequest> {
  const channelID = req.channel_id || req.channel;
  const trigger: Partial<TriggerDTO> = {
    ...(req.id ? { id: req.id } : {}),
    ...(req.tenant_id ? { tenant_id: req.tenant_id } : {}),
    display_name: req.display_name || '',
    description: req.description || '',
    enabled: req.enabled ?? true,
    event_type: req.event_type || '',
    channel_id: channelID,
    channel: req.channel || channelID || '',
    automation_job_id: req.automation_job_id || '',
    governance: req.governance || '',
    filter_expr: req.filter_expr || '',
    target_audience: req.target_audience || '',
    template_id: req.template_id || '',
    cooldown_sec: req.cooldown_sec || 0,
  };
  return {
    ...req,
    trigger,
    channel_id: channelID,
    channel: req.channel || channelID,
    automation_job_id: req.automation_job_id || '',
    governance: req.governance || '',
  };
}

export async function listTriggers(params?: {
  page?: number;
  limit?: number;
}): Promise<TriggerListResponse> {
  const response = await api.get<TriggerListResponse>('/triggers', { params });
  return {
    ...response.data,
    items: (response.data.items || []).map(normalizeTrigger),
  };
}

export async function getTrigger(id: string): Promise<TriggerDTO> {
  const response = await api.get<TriggerDTO>(`/triggers/${id}`);
  return normalizeTrigger(response.data);
}

export async function upsertTrigger(
  req: TriggerUpsertRequest,
): Promise<TriggerDTO> {
  const response = await api.post<TriggerDTO>('/triggers', serializeTrigger(req));
  return normalizeTrigger(response.data);
}

export async function updateTrigger(
  id: string,
  req: Partial<TriggerUpsertRequest>,
): Promise<TriggerDTO> {
  const response = await api.put<TriggerDTO>(`/triggers/${id}`, serializeTrigger(req));
  return normalizeTrigger(response.data);
}

export async function setTriggerEnabled(
  id: string,
  enabled: boolean,
  operatorReason?: string,
): Promise<TriggerDTO> {
  const response = await api.post<TriggerDTO>(
    `/triggers/${id}/${enabled ? 'enable' : 'disable'}`,
    { operator_reason: operatorReason },
  );
  return normalizeTrigger(response.data);
}
