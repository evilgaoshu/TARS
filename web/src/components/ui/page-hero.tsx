import type { ReactNode } from "react"
import { Card } from "@/components/ui/card"
import { cn } from "@/lib/utils"

export function SectionTitle({
  title,
  subtitle,
  className,
}: {
  title: string
  subtitle?: string
  className?: string
}) {
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <h1 className="text-2xl font-semibold tracking-tight text-foreground">{title}</h1>
      {subtitle ? (
        <p className="max-w-3xl text-sm leading-6 text-muted-foreground">{subtitle}</p>
      ) : null}
    </div>
  )
}

export function SummaryGrid({
  children,
  className,
}: {
  children: ReactNode
  className?: string
}) {
  return <div className={cn("grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4", className)}>{children}</div>
}

export function StatCard({
  icon,
  title,
  value,
  subtitle,
  colorClass = "text-primary",
  trend,
}: {
  icon?: ReactNode
  title: string
  value: string | number
  subtitle: string
  colorClass?: string
  trend?: { value: string; positive?: boolean }
}) {
  return (
    <Card className="glass-card-interactive flex min-h-36 flex-col gap-5 overflow-hidden p-5">
      <div className="flex items-start justify-between gap-4">
        <div className="flex flex-col gap-2">
          <span className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">
            {title}
          </span>
          <span className="text-3xl font-black tracking-tight text-foreground sm:text-4xl">{value}</span>
        </div>
        {icon ? (
          <div className={cn("flex size-10 items-center justify-center rounded-2xl border border-border bg-white/5", colorClass)}>
            {icon}
          </div>
        ) : null}
      </div>
      <div className="mt-auto flex items-center justify-between gap-4">
        <p className="text-xs font-medium leading-5 text-muted-foreground">{subtitle}</p>
        {trend ? (
          <span className={cn("text-xs font-bold", trend.positive ? "text-success" : "text-danger")}>
            {trend.value}
          </span>
        ) : null}
      </div>
    </Card>
  )
}
