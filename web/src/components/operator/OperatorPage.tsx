import type { ReactNode } from 'react'
import { ArrowRight, type LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'

type StatTone = 'info' | 'success' | 'warning' | 'danger' | 'muted'

function toneClass(tone: StatTone) {
  switch (tone) {
    case 'success':
      return 'text-success border-success/20 bg-success/10'
    case 'warning':
      return 'text-warning border-warning/20 bg-warning/10'
    case 'danger':
      return 'text-danger border-danger/20 bg-danger/10'
    case 'muted':
      return 'text-muted-foreground border-border bg-white/[0.03]'
    case 'info':
    default:
      return 'text-info border-info/20 bg-info/10'
  }
}

export function OperatorHero({
  eyebrow,
  title,
  description,
  chips,
  primaryAction,
  secondaryAction,
  className,
}: {
  eyebrow?: string
  title: string
  description: string
  chips?: Array<{ label: string; tone?: StatTone }>
  primaryAction?: ReactNode
  secondaryAction?: ReactNode
  className?: string
}) {
  return (
    <section className={cn("relative overflow-hidden rounded-[28px] border border-border bg-[linear-gradient(135deg,rgba(242,184,75,0.12),rgba(14,165,233,0.08),rgba(255,255,255,0.02))] p-6 shadow-[0_24px_80px_-30px_rgba(0,0,0,0.8)] sm:p-8", className)}>
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(242,184,75,0.18),transparent_32%),radial-gradient(circle_at_bottom_left,rgba(14,165,233,0.14),transparent_28%)]" />
      <div className="relative flex flex-col gap-6 xl:flex-row xl:items-end xl:justify-between">
        <div className="max-w-4xl space-y-4">
          {eyebrow ? <div className="text-[0.72rem] font-black uppercase tracking-[0.22em] text-primary">{eyebrow}</div> : null}
          <div className="space-y-3">
            <h1 className="text-3xl font-black tracking-tight text-foreground sm:text-4xl">{title}</h1>
            <p className="max-w-3xl text-sm leading-7 text-muted-foreground sm:text-base">{description}</p>
          </div>
          {chips?.length ? (
            <div className="flex flex-wrap gap-2">
              {chips.map((chip) => (
                <Badge key={chip.label} variant={chip.tone || 'outline'} className="rounded-full px-3 py-1 text-[0.68rem] font-black uppercase tracking-[0.16em]">
                  {chip.label}
                </Badge>
              ))}
            </div>
          ) : null}
        </div>
        {(primaryAction || secondaryAction) ? (
          <div className="flex flex-wrap items-center gap-3">{secondaryAction}{primaryAction}</div>
        ) : null}
      </div>
    </section>
  )
}

export function OperatorStats({ stats }: { stats: Array<{ title: string; value: string | number; description: string; icon?: LucideIcon; tone?: StatTone }> }) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 2xl:grid-cols-4">
      {stats.map((stat) => {
        const Icon = stat.icon
        return (
          <Card key={stat.title} className="glass-card-interactive overflow-hidden p-0">
            <CardContent className="flex min-h-40 flex-col gap-5 p-5">
              <div className="flex items-start justify-between gap-4">
                <div className="flex flex-col gap-2">
                  <span className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{stat.title}</span>
                  <span className="text-3xl font-black tracking-tight text-foreground">{stat.value}</span>
                </div>
                {Icon ? (
                  <div className={cn('flex size-11 items-center justify-center rounded-2xl border', toneClass(stat.tone || 'info'))}>
                    <Icon />
                  </div>
                ) : null}
              </div>
              <p className="mt-auto text-sm leading-6 text-muted-foreground">{stat.description}</p>
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}

export function OperatorSection({
  title,
  description,
  icon: Icon,
  action,
  children,
  className,
}: {
  title: string
  description?: string
  icon?: LucideIcon
  action?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <Card className={cn('overflow-hidden p-0', className)}>
      <CardHeader className="border-b border-border bg-white/[0.02]">
        <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
          <div className="flex items-start gap-3">
            {Icon ? <div className="mt-0.5 flex size-10 items-center justify-center rounded-2xl border border-border bg-white/[0.04] text-primary"><Icon /></div> : null}
            <div className="space-y-1.5">
              <CardTitle className="text-xl font-semibold text-foreground">{title}</CardTitle>
              {description ? <CardDescription className="max-w-3xl text-sm leading-6">{description}</CardDescription> : null}
            </div>
          </div>
          {action ? <div className="flex flex-wrap gap-2">{action}</div> : null}
        </div>
      </CardHeader>
      <CardContent className="p-5 sm:p-6">{children}</CardContent>
    </Card>
  )
}

export function OperatorCardGrid({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('grid grid-cols-1 gap-4 xl:grid-cols-2', className)}>{children}</div>
}

export function OperatorStack({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('flex flex-col gap-4', className)}>{children}</div>
}

export function OperatorKicker({ label, value, tone = 'info' }: { label: string; value: string; tone?: StatTone }) {
  return (
    <div className={cn('inline-flex items-center gap-2 rounded-full border px-3 py-1', toneClass(tone))}>
      <span className="text-[0.62rem] font-black uppercase tracking-[0.18em] opacity-70">{label}</span>
      <span className="text-xs font-semibold uppercase tracking-[0.12em]">{value}</span>
    </div>
  )
}

export function OperatorActionLink({ label, onClick }: { label: string; onClick?: () => void }) {
  return (
    <Button variant="ghost" size="sm" className="h-8 gap-1.5 text-xs font-bold uppercase tracking-[0.14em] text-primary" onClick={onClick}>
      {label}
      <ArrowRight />
    </Button>
  )
}
