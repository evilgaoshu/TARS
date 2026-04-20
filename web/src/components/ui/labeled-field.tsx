import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

export function LabeledField({
  label,
  required,
  hint,
  children,
  className,
}: {
  label: string
  required?: boolean
  hint?: string
  children: ReactNode
  className?: string
}) {
  return (
    <label className={cn("flex flex-col gap-2", className)}>
      <span className="flex items-center gap-1 text-sm font-semibold text-foreground">
        {label}
        {required ? <span className="text-danger">*</span> : null}
      </span>
      {children}
      {hint ? <span className="text-xs leading-5 text-muted-foreground">{hint}</span> : null}
    </label>
  )
}
