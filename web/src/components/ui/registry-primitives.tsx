import type { ReactNode } from 'react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { cn } from '@/lib/utils'

export function SplitLayout({
  sidebar,
  detail,
  className,
}: {
  sidebar: ReactNode
  detail: ReactNode
  className?: string
}) {
  return (
    <div className={cn('grid grid-cols-1 gap-6 xl:grid-cols-[340px_minmax(0,1fr)]', className)}>
      <div>{sidebar}</div>
      <div>{detail}</div>
    </div>
  )
}

export function RegistrySidebar({ children, className }: { children: ReactNode; className?: string }) {
  return <Card className={cn('p-0', className)}>{children}</Card>
}

export function RegistryDetail({ children, className }: { children: ReactNode; className?: string }) {
  return <Card className={cn('relative overflow-hidden p-0', className)}>{children}</Card>
}

export function RegistryPanel({
  title,
  emptyText,
  children,
  className,
}: {
  title: string
  emptyText: string
  children: ReactNode
  className?: string
}) {
  const hasChildren = children && !(Array.isArray(children) && children.length === 0)
  return (
    <div className={cn('flex flex-col gap-3', className)}>
      <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{title}</div>
      {hasChildren ? children : <div className="rounded-2xl border border-dashed border-border px-4 py-10 text-center text-sm text-muted-foreground">{emptyText}</div>}
    </div>
  )
}

export function RegistryCard({
  title,
  subtitle,
  lines,
  status,
  action,
  active,
}: {
  title: string
  subtitle: string
  lines: string[]
  status?: ReactNode
  action?: ReactNode
  active?: boolean
}) {
  return (
    <div className={cn(
      'rounded-2xl border p-4 text-left transition-all',
      active ? 'border-primary/40 bg-primary/5' : 'border-border bg-white/[0.03] hover:bg-white/[0.05]'
    )}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-foreground">{title}</div>
          <div className="mt-1 truncate font-mono text-xs text-muted-foreground">{subtitle}</div>
        </div>
        <div className="flex shrink-0 items-center gap-2">{status}{action}</div>
      </div>
      {lines.length > 0 ? (
        <div className="mt-3 grid gap-1 text-xs text-muted-foreground">
          {lines.map((line) => <div key={line}>{line}</div>)}
        </div>
      ) : null}
    </div>
  )
}

export function RegistryCardHeader({
  title,
  description,
  action,
}: {
  title: string
  description?: string
  action?: ReactNode
}) {
  return (
    <CardHeader className="border-b border-border bg-white/[0.02]">
      <div className="flex items-center justify-between gap-3">
        <div>
          <CardTitle>{title}</CardTitle>
          {description ? <CardDescription className="mt-1">{description}</CardDescription> : null}
        </div>
        {action}
      </div>
    </CardHeader>
  )
}

export function RegistryCardBody({ children, className }: { children: ReactNode; className?: string }) {
  return <CardContent className={cn('flex flex-col gap-4 p-5', className)}>{children}</CardContent>
}
