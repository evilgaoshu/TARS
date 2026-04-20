import type { ComponentType, ReactNode } from 'react'
import { Search } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  loading,
  className,
}: {
  icon?: ComponentType<{ size?: number; className?: string }>
  title: string
  description?: string
  action?: ReactNode
  loading?: boolean
  className?: string
}) {
  return (
    <div className={cn('flex flex-col items-center justify-center rounded-2xl border border-dashed border-border bg-white/[0.02] px-6 py-14 text-center', className)}>
      {loading ? (
        <div className="mb-3 flex items-center gap-2 text-primary/60">
          <div className="size-1.5 animate-bounce rounded-full bg-current" />
          <div className="size-1.5 animate-bounce rounded-full bg-current [animation-delay:-.3s]" />
          <div className="size-1.5 animate-bounce rounded-full bg-current [animation-delay:-.5s]" />
        </div>
      ) : Icon ? (
        <Icon size={36} className="mb-4 opacity-20" />
      ) : (
        <div className="mb-4 flex size-12 items-center justify-center rounded-full bg-white/5 opacity-40">
          <Search size={20} />
        </div>
      )}
      <h3 className="text-base font-semibold text-foreground">{title}</h3>
      {description ? <p className="mt-1.5 max-w-sm text-sm leading-6 text-muted-foreground">{description}</p> : null}
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  )
}

export function EmptyStateAction({ children, onClick }: { children: ReactNode; onClick?: () => void }) {
  return <Button variant="outline" onClick={onClick}>{children}</Button>
}
