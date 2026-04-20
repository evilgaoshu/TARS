import { api } from './client';
import type { InboxMessage, InboxListResponse } from './types';

export async function listInbox(params?: {
  page?: number;
  limit?: number;
  unread_only?: boolean;
}): Promise<InboxListResponse> {
  const response = await api.get<InboxListResponse>('/inbox', { params });
  return response.data;
}

export async function getInboxMessage(id: string): Promise<InboxMessage> {
  const response = await api.get<InboxMessage>(`/inbox/${id}`);
  return response.data;
}

export async function markInboxRead(id: string): Promise<InboxMessage> {
  const response = await api.post<InboxMessage>(`/inbox/${id}/read`);
  return response.data;
}

export async function markAllInboxRead(): Promise<void> {
  await api.post('/inbox/mark-all-read');
}
