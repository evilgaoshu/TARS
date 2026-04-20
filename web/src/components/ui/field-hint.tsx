import type { ReactNode } from "react"

export function FieldHint({ children }: { children: ReactNode }) {
  return <div className="text-xs leading-5 text-muted-foreground">{children}</div>
}
