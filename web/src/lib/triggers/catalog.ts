export type TriggerEventOption = {
  value: string
  label: string
}

export const TRIGGER_EVENT_OPTIONS: TriggerEventOption[] = [
  { value: 'on_execution_completed', label: 'Execution completed' },
  { value: 'on_execution_failed', label: 'Execution failed' },
  { value: 'on_approval_requested', label: 'Approval requested' },
  { value: 'on_skill_completed', label: 'Skill completed' },
  { value: 'on_skill_failed', label: 'Skill failed' },
  { value: 'on_session_closed', label: 'Session closed' },
]

export const DEFAULT_TRIGGER_EVENT_TYPE = 'on_execution_completed'

export const TRIGGER_DELIVERY_CHANNEL_KINDS = ['in_app_inbox', 'telegram'] as const

export function isTriggerDeliveryChannelKind(kind?: string | null): boolean {
  return TRIGGER_DELIVERY_CHANNEL_KINDS.includes((kind || '').trim() as (typeof TRIGGER_DELIVERY_CHANNEL_KINDS)[number])
}

export function filterTriggerDeliveryChannels<T extends { kind?: string; type?: string }>(channels: T[]): T[] {
  return channels.filter((channel) => isTriggerDeliveryChannelKind(channel.kind || channel.type))
}
