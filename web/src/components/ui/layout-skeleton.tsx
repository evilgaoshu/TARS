import { Skeleton } from "./skeleton"
import { Card } from "./card"

export const TableSkeleton = ({ rows = 5, cols = 4 }: { rows?: number; cols?: number }) => (
  <div className="w-full rounded-2xl border border-border bg-white/[0.02] p-4">
    <div className="flex items-center gap-4 border-b border-border px-1 py-2">
      {Array.from({ length: cols }).map((_, i) => (
        <Skeleton key={i} className="h-4 flex-1" />
      ))}
    </div>
    {Array.from({ length: rows }).map((_, i) => (
      <div key={i} className="flex items-center gap-4 border-b border-border px-1 py-4 last:border-0">
        {Array.from({ length: cols }).map((_, j) => (
          <Skeleton key={j} className={`h-4 ${j === 0 ? 'flex-[1.5]' : 'flex-1'}`} />
        ))}
      </div>
    ))}
  </div>
)

export const CardSkeleton = () => (
  <Card className="flex flex-col gap-4 p-6">
    <div className="flex items-center gap-4">
      <Skeleton className="h-12 w-12 rounded-full" />
      <div className="flex flex-col gap-2">
        <Skeleton className="h-4 w-[200px]" />
        <Skeleton className="h-4 w-[150px]" />
      </div>
    </div>
    <Skeleton className="h-4 w-full" />
    <Skeleton className="h-4 w-3/4" />
  </Card>
)

export const DetailSkeleton = () => (
  <div className="animate-fade-in flex flex-col gap-8">
    <div className="flex flex-col justify-between gap-6 border-b border-border pb-6 md:flex-row md:items-center">
      <div className="flex flex-col gap-2">
        <Skeleton className="h-8 w-[300px]" />
        <Skeleton className="h-4 w-[200px]" />
      </div>
      <div className="flex gap-2">
        <Skeleton className="h-10 w-[120px]" />
        <Skeleton className="h-10 w-[100px]" />
      </div>
    </div>
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
      <div className="flex flex-col gap-6 lg:col-span-2">
        <Skeleton className="h-[200px] w-full rounded-2xl" />
        <Skeleton className="h-[400px] w-full rounded-2xl" />
      </div>
      <div className="flex flex-col gap-6">
        <Skeleton className="h-[300px] w-full rounded-2xl" />
        <Skeleton className="h-[200px] w-full rounded-2xl" />
      </div>
    </div>
  </div>
)
