import { Card } from "@/components/ui/card"

export function EmptyDetailState({ title, description }: { title: string; description: string }) {
  return (
    <Card className="flex min-h-[320px] flex-col items-center justify-center gap-3 p-8 text-center">
      <h2 className="text-xl font-semibold tracking-tight text-foreground">{title}</h2>
      <p className="max-w-lg text-sm leading-6 text-muted-foreground">{description}</p>
    </Card>
  )
}
