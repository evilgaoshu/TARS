import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"
import { X, CheckCircle2, AlertTriangle, Info, AlertCircle } from "lucide-react"

const alertVariants = cva(
  "relative w-full rounded-xl border px-4 py-3.5 text-sm flex items-start gap-3",
  {
    variants: {
      variant: {
        default: "bg-white/5 border-white/10 text-[var(--text-primary)]",
        info: "bg-[rgba(109,198,214,0.08)] border-[rgba(109,198,214,0.25)] text-[var(--info)]",
        success: "bg-[rgba(126,212,173,0.08)] border-[rgba(126,212,173,0.25)] text-[var(--success)]",
        warning: "bg-[rgba(242,184,75,0.08)] border-[rgba(242,184,75,0.25)] text-[var(--warning)]",
        destructive: "bg-[rgba(255,127,107,0.08)] border-[rgba(255,127,107,0.25)] text-[var(--danger)]",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

const alertIcons = {
  default: Info,
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  destructive: AlertCircle,
}

interface AlertProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof alertVariants> {
  onClose?: () => void
}

const Alert = React.forwardRef<HTMLDivElement, AlertProps>(
  ({ className, variant = "default", onClose, children, ...props }, ref) => {
    const Icon = alertIcons[variant ?? "default"]
    return (
      <div
        ref={ref}
        role="alert"
        className={cn(alertVariants({ variant }), className)}
        {...props}
      >
        <Icon size={16} className="mt-0.5 shrink-0" />
        <div className="flex-1 min-w-0">{children}</div>
        {onClose && (
          <button
            onClick={onClose}
            className="shrink-0 opacity-60 hover:opacity-100 transition-opacity"
          >
            <X size={14} />
          </button>
        )}
      </div>
    )
  }
)
Alert.displayName = "Alert"

const AlertTitle = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h5
    ref={ref}
    className={cn("mb-1 font-bold leading-none tracking-tight", className)}
    {...props}
  />
))
AlertTitle.displayName = "AlertTitle"

const AlertDescription = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn("text-sm opacity-90 leading-relaxed", className)}
    {...props}
  />
))
AlertDescription.displayName = "AlertDescription"

export { Alert, AlertTitle, AlertDescription }
