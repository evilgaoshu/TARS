import type { ReactNode } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { cn } from "@/lib/utils"

export function PanelCard({
  title,
  subtitle,
  icon,
  headerAction,
  className,
  children,
}: {
  title: string
  subtitle?: string
  icon?: ReactNode
  headerAction?: ReactNode
  className?: string
  children: ReactNode
}) {
  return (
    <Card className={cn("overflow-hidden p-0", className)}>
      <CardHeader className="flex flex-row items-start justify-between gap-4 border-b border-border bg-white/[0.02] px-5 py-4">
        <div className="flex min-w-0 flex-col gap-1.5">
          <CardTitle className="flex items-center gap-2 text-base font-semibold text-foreground">
            {icon}
            {title}
          </CardTitle>
          {subtitle ? <CardDescription className="text-xs leading-5">{subtitle}</CardDescription> : null}
        </div>
        {headerAction ? <div className="shrink-0">{headerAction}</div> : null}
      </CardHeader>
      <CardContent className="flex flex-col gap-4 p-5">{children}</CardContent>
    </Card>
  )
}

export function GlassPanel({ children, className }: { children: ReactNode; className?: string }) {
  return <Card className={cn("p-6", className)}>{children}</Card>
}
