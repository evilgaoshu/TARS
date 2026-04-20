import { useState, type ReactNode } from "react"
import { ChevronDown, ChevronUp } from "lucide-react"
import { Button } from "@/components/ui/button"

export function CollapsibleList({
  items,
  limit = 3,
  visibleCount,
  emptyText,
}: {
  items: ReactNode[]
  limit?: number
  visibleCount?: number
  emptyText?: string
}) {
  const [expanded, setExpanded] = useState(false)
  const resolvedLimit = visibleCount ?? limit

  if (!items.length) {
    return emptyText ? <p className="text-sm italic text-muted-foreground">{emptyText}</p> : null
  }

  if (items.length <= resolvedLimit) {
    return <div className="flex flex-col gap-3">{items}</div>
  }

  const visible = expanded ? items : items.slice(0, resolvedLimit)

  return (
    <div className="flex flex-col gap-3">
      {visible}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="mx-auto h-8 rounded-full px-4 text-[0.68rem] font-black uppercase tracking-[0.16em] text-primary"
        onClick={() => setExpanded((value) => !value)}
      >
        {expanded ? <ChevronUp data-icon="inline-start" /> : <ChevronDown data-icon="inline-start" />}
        {expanded ? "Show less" : `Show ${items.length - resolvedLimit} more`}
      </Button>
    </div>
  )
}
