import { Badge, type BadgeProps } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

const statusVariantMap: Record<string, BadgeProps["variant"]> = {
  healthy: "success",
  success: "success",
  enabled: "success",
  active: "success",
  approved: "success",
  completed: "success",
  resolved: "success",
  delivered: "success",
  set: "success",
  online: "success",
  valid: "success",
  info: "info",
  open: "info",
  pending: "info",
  processing: "info",
  executing: "info",
  verifying: "info",
  analyzing: "info",
  reviewing: "info",
  warning: "warning",
  degraded: "warning",
  pending_approval: "warning",
  timeout: "warning",
  blocked: "warning",
  draft: "warning",
  critical: "danger",
  error: "danger",
  failed: "danger",
  rejected: "danger",
  missing: "danger",
  disabled: "danger",
  offline: "danger",
  invalid: "danger",
}

export function StatusBadge({
  status,
  label,
  className,
}: {
  status: string
  label?: string
  className?: string
}) {
  const normalized = (status || "unknown").toLowerCase()
  const variant = statusVariantMap[normalized] ?? "muted"
  const displayLabel = label || status.replace(/_/g, " ")

  return (
    <Badge
      variant={variant}
      className={cn("uppercase tracking-[0.14em] text-[0.68rem] font-bold", className)}
    >
      {displayLabel}
    </Badge>
  )
}
