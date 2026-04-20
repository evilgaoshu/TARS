import { cn } from '@/lib/utils'

export function ActiveBadge({ active, label }: { active: boolean; label: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-bold uppercase tracking-[0.14em]',
        active
          ? 'border-success/20 bg-success/10 text-success'
          : 'border-warning/20 bg-warning/10 text-warning'
      )}
    >
      {label}
    </span>
  )
}
