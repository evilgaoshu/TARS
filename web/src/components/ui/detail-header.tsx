import type { ReactNode } from 'react'

export function DetailHeader({
  title,
  subtitle,
  status,
  actions,
}: {
  title: string
  subtitle?: string
  status?: ReactNode
  actions?: ReactNode
}) {
  return (
    <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
      <div className="flex flex-col gap-1.5">
        <div className="flex flex-wrap items-center gap-2.5">
          <h2 className="text-xl font-semibold tracking-tight text-foreground">{title}</h2>
          {status}
        </div>
        {subtitle ? <p className="text-sm leading-6 text-muted-foreground">{subtitle}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  )
}
