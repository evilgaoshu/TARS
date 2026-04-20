import type { ReactNode } from "react"
import { Search, X } from "lucide-react"
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/select"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface FilterBarProps {
  search?: {
    value: string
    onChange: (value: string) => void
    placeholder?: string
  }
  filters?: Array<{
    key: string
    label?: string
    value: string
    onChange: (value: string) => void
    options: Array<{ value: string; label: string }>
    className?: string
  }>
  actions?: ReactNode
  className?: string
}

export function FilterBar({ search, filters, actions, className }: FilterBarProps) {
  return (
    <div className={cn("flex flex-col gap-3 md:flex-row md:flex-wrap md:items-center", className)}>
      {search ? (
        <div className="relative w-full md:max-w-sm md:flex-1">
          <Search data-icon="inline-start" className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search.value}
            onChange={(event) => search.onChange(event.target.value)}
            placeholder={search.placeholder || "Search..."}
            aria-label={search.placeholder || "Search"}
            className="h-10 pr-10 pl-9"
          />
          {search.value ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="absolute right-1 top-1/2 size-8 -translate-y-1/2"
              onClick={() => search.onChange("")}
              aria-label="Clear search"
            >
              <X />
            </Button>
          ) : null}
        </div>
      ) : null}

      {filters?.map((filter) => (
        <NativeSelect
          key={filter.key}
          value={filter.value}
          onChange={(event) => filter.onChange(event.target.value)}
          aria-label={filter.label || filter.key}
          className={cn("h-10 w-full md:w-44", filter.className)}
        >
          {filter.options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </NativeSelect>
      ))}

      {actions ? <div className="flex flex-wrap items-center gap-2 md:ml-auto">{actions}</div> : null}
    </div>
  )
}
