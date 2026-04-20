import { Button } from '@/components/ui/button';
import { NativeSelect } from '@/components/ui/select';

type PaginationControlsProps = {
  page: number;
  limit: number;
  total: number;
  hasNext: boolean;
  onPageChange: (nextPage: number) => void;
  onLimitChange: (nextLimit: number) => void;
  limitOptions?: number[];
};

const defaultLimitOptions = [20, 50, 100];

export const PaginationControls = ({
  page,
  limit,
  total,
  hasNext,
  onPageChange,
  onLimitChange,
  limitOptions = defaultLimitOptions,
}: PaginationControlsProps) => {
  const totalPages = Math.max(1, Math.ceil(total / Math.max(limit, 1)));
  return (
    <div className="mt-6 flex flex-col gap-4 rounded-2xl border border-border bg-white/[0.02] p-4 md:flex-row md:items-center md:justify-between">
      <div className="text-sm text-muted-foreground">
        Page {page} / {totalPages} · {total} records
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <label className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>Limit</span>
          <NativeSelect
            value={limit}
            onChange={(event) => onLimitChange(Number(event.target.value))}
            className="h-9 w-24 bg-background"
            aria-label="Items per page"
          >
            {limitOptions.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </NativeSelect>
        </label>
        <Button variant="outline" onClick={() => onPageChange(page - 1)} disabled={page <= 1}>
          Previous
        </Button>
        <Button variant="outline" onClick={() => onPageChange(page + 1)} disabled={!hasNext}>
          Next
        </Button>
      </div>
    </div>
  );
};
