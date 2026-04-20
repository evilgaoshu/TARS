import { AlertTriangle, CheckCircle2, Info, ShieldAlert } from 'lucide-react'
import { cn } from '@/lib/utils'

type NoticeTone = 'success' | 'error' | 'warning' | 'info'

const toneMap: Record<NoticeTone, { icon: typeof CheckCircle2; className: string }> = {
  success: { icon: CheckCircle2, className: 'border-success/30 bg-success/10 text-success' },
  error: { icon: ShieldAlert, className: 'border-danger/30 bg-danger/10 text-danger' },
  warning: { icon: AlertTriangle, className: 'border-warning/30 bg-warning/10 text-warning' },
  info: { icon: Info, className: 'border-info/30 bg-info/10 text-info' },
}

export function ActionResultNotice({ tone, message, className }: { tone: NoticeTone; message?: string; className?: string }) {
  if (!message) {
    return null
  }
  const Icon = toneMap[tone].icon
  return (
    <div className={cn('flex items-start gap-3 rounded-2xl border px-4 py-3 text-sm', toneMap[tone].className, className)}>
      <Icon className="mt-0.5 size-4 shrink-0" />
      <div className="leading-6">{message}</div>
    </div>
  )
}
