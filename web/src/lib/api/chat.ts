import { api } from './client';

export interface ChatMessageRequest {
  message: string;
  host?: string;
  service?: string;
  severity?: string;
}

export interface ChatMessageResponse {
  session_id: string;
  status: string;
  duplicated: boolean;
  ack_message: string;
}

export interface ChatSessionSummary {
  session_id: string;
  status: string;
  user_request: string;
  host: string;
  service: string;
}

export interface ChatSessionsResponse {
  items: ChatSessionSummary[];
  total: number;
}

export async function sendChatMessage(req: ChatMessageRequest): Promise<ChatMessageResponse> {
  const response = await api.post<ChatMessageResponse>('/chat/messages', req);
  return response.data;
}

export async function listChatSessions(): Promise<ChatSessionsResponse> {
  const response = await api.get<ChatSessionsResponse>('/chat/sessions');
  return response.data;
}
