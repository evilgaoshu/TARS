import type { ComponentType, ReactNode } from 'react'
import { AlertTriangle, Ban, ChevronDown, DatabaseZap, Info, Search } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

type SharedTone = 'active' | 'warning' | 'success' | 'danger' | 'muted'
type StateTone = 'empty' | 'error' | 'loading' | 'degraded' | 'disabled'

const stateToneMap: Record<string, SharedTone> = {
  open: 'warning',
  pending: 'warning',
  degraded: 'warning',
  warning: 'warning',
  pending_approval: 'warning',
  timeout: 'warning',
  blocked: 'warning',
  draft: 'warning',
  analyzing: 'active',
  executing: 'active',
  processing: 'active',
  verifying: 'active',
  reviewing: 'active',
  info: 'active',
  resolved: 'success',
  completed: 'success',
  healthy: 'success',
  approved: 'success',
  enabled: 'success',
  delivered: 'success',
  active: 'success',
  online: 'success',
  valid: 'success',
  failed: 'danger',
  rejected: 'danger',
  critical: 'danger',
  error: 'danger',
  missing: 'danger',
  disabled: 'muted',
  muted: 'muted',
  offline: 'muted',
  unknown: 'muted',
}

const toneVariantMap: Record<SharedTone, 'info' | 'warning' | 'success' | 'danger' | 'muted'> = {
  active: 'info',
  warning: 'warning',
  success: 'success',
  danger: 'danger',
  muted: 'muted',
}

const statePanelToneMap: Record<StateTone, { tone: SharedTone; icon: ComponentType<{ className?: string }> }> = {
  empty: { tone: 'muted', icon: Search },
  error: { tone: 'danger', icon: AlertTriangle },
  loading: { tone: 'active', icon: DatabaseZap },
  degraded: { tone: 'warning', icon: Info },
  disabled: { tone: 'muted', icon: Ban },
}

// eslint-disable-next-line react-refresh/only-export-components
export function getSharedTone(value: string | null | undefined): SharedTone {
  return stateToneMap[String(value || 'unknown').toLowerCase()] ?? 'muted'
}

export function RiskBadge({ risk, label, className }: { risk: string; label?: string; className?: string }) {
  const normalized = String(risk || 'muted').toLowerCase()
  const tone: SharedTone = normalized === 'critical' || normalized === 'high'
    ? 'danger'
    : normalized === 'warning' || normalized === 'medium'
      ? 'warning'
      : normalized === 'healthy' || normalized === 'low'
        ? 'success'
        : 'muted'

  return (
    <Badge
      variant={toneVariantMap[tone]}
      data-tone={tone}
      className={cn('rounded-md border px-2 py-0.5 text-[0.68rem] font-bold uppercase tracking-[0.14em]', className)}
    >
      {label ?? `${normalized} risk`}
    </Badge>
  )
}

export function StatePanel({
  title,
  description,
  tone,
  icon,
  action,
  className,
}: {
  title: string
  description: string
  tone: StateTone
  icon?: ReactNode
  action?: ReactNode
  className?: string
}) {
  const config = statePanelToneMap[tone]
  const Icon = config.icon

  return (
    <section
      data-tone={tone}
      className={cn(
        'flex flex-col gap-3 rounded-2xl border px-4 py-4',
        config.tone === 'danger' && 'border-danger/30 bg-danger/8 text-danger',
        config.tone === 'warning' && 'border-warning/30 bg-warning/8 text-warning',
        config.tone === 'active' && 'border-info/30 bg-info/8 text-info',
        config.tone === 'success' && 'border-success/30 bg-success/8 text-success',
        config.tone === 'muted' && 'border-border bg-card text-muted-foreground',
        className,
      )}
    >
      <div className="flex items-start gap-3">
        <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-xl border border-current/15 bg-background/50">
          {icon ?? <Icon className="size-4" />}
        </div>
        <div className="min-w-0 flex-1">
          <div className={cn('text-sm font-semibold', config.tone === 'muted' ? 'text-foreground' : 'text-current')}>{title}</div>
          <p className={cn('mt-1 text-sm leading-6', config.tone === 'muted' ? 'text-muted-foreground' : 'text-current/85')}>{description}</p>
        </div>
      </div>
      {action ? <div>{action}</div> : null}
    </section>
  )
}

export function RawPayloadFold({
  title,
  summary,
  defaultOpen = false,
  children,
  className,
}: {
  title: string
  summary: string
  defaultOpen?: boolean
  children: ReactNode
  className?: string
}) {
  return (
    <details
      open={defaultOpen}
      data-slot="raw-payload-fold"
      className={cn('group min-w-0 rounded-2xl border border-border bg-card/80', className)}
    >
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-4 py-3">
        <div className="min-w-0">
          <div className="break-words text-sm font-semibold text-foreground">{title}</div>
          <div className="mt-1 break-words text-xs text-muted-foreground">{summary}</div>
        </div>
        <ChevronDown className="shrink-0 text-muted-foreground transition-transform group-open:rotate-180" size={16} />
      </summary>
      <div className="min-w-0 px-4 pb-4">
        <pre className="max-w-full overflow-x-auto rounded-2xl border border-border bg-black/20 p-3 text-xs leading-6 text-muted-foreground">{children}</pre>
      </div>
    </details>
  )
}

export function SharedStateAction({ children, onClick }: { children: ReactNode; onClick?: () => void }) {
  return <Button variant="outline" onClick={onClick}>{children}</Button>
}
