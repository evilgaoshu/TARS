import { Badge, type BadgeProps } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import { getSharedTone } from "@/components/ui/shared-state"

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
  analyzing: "info",
  executing: "info",
  processing: "info",
  verifying: "info",
  reviewing: "info",
  warning: "warning",
  open: "warning",
  pending: "warning",
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
  disabled: "muted",
  offline: "muted",
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
      data-tone={getSharedTone(normalized)}
      className={cn("uppercase tracking-[0.14em] text-[0.68rem] font-bold", className)}
    >
      {displayLabel}
    </Badge>
  )
}
