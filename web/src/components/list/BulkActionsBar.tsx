import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { cn } from '@/lib/utils';

type BulkAction = {
  key: string;
  label: string;
  disabled?: boolean;
  tone?: 'default' | 'danger';
  onClick: () => void;
};

type BulkActionsBarProps = {
  selectedCount: number;
  onClear: () => void;
  actions: BulkAction[];
  summaryText?: string;
  clearLabel?: string;
};

export const BulkActionsBar = ({ selectedCount, onClear, actions, summaryText, clearLabel = 'Clear' }: BulkActionsBarProps) => {
  if (selectedCount <= 0) {
    return null;
  }

  return (
    <Card className="mb-4 flex flex-col gap-4 p-4 md:flex-row md:items-center md:justify-between">
      <div className="text-sm font-semibold text-foreground">
        {summaryText || `${selectedCount} item${selectedCount > 1 ? 's' : ''} selected`}
      </div>
      <div className="flex flex-wrap items-center gap-3">
        {actions.map((action) => (
          <Button
            key={action.key}
            variant="outline"
            className={cn(action.tone === 'danger' && 'border-danger/40 text-danger hover:bg-danger/10 hover:text-danger')}
            disabled={action.disabled}
            onClick={action.onClick}
          >
            {action.label}
          </Button>
        ))}
        <Button variant="outline" onClick={onClear}>
          {clearLabel}
        </Button>
      </div>
    </Card>
  );
};
