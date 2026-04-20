import React from 'react';
import type { ReactNode } from 'react';
import { Search, RefreshCcw, X } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { TableSkeleton } from '@/components/ui/layout-skeleton';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card } from '@/components/ui/card';

interface RegistryLayoutProps {
  title: string;
  description?: string;
  icon?: React.ReactElement<LucideIcon>;
  
  // Toolbar Props
  searchQuery?: string;
  onSearchChange?: (val: string) => void;
  searchPlaceholder?: string;
  toolbarActions?: ReactNode;
  
  // State Props
  loading?: boolean;
  limit?: number;
  onRefresh?: () => void;
  error?: string | null;
  
  // Selection Props
  bulkActions?: ReactNode;
  
  // Children
  children: ReactNode;
  
  // Footer
  footer?: ReactNode;
}

/**
 * Standard layout for list/registry pages (Audit, Executions, Connectors, etc.)
 */
export const RegistryLayout: React.FC<RegistryLayoutProps> = ({
  title,
  description,
  icon,
  searchQuery,
  onSearchChange,
  searchPlaceholder = 'Search...',
  toolbarActions,
  loading,
  limit = 20,
  onRefresh,
  error,
  bulkActions,
  children,
  footer,
}) => {
  return (
    <div className="animate-fade-in flex flex-col gap-6 min-h-full p-8">
      {/* 1. Header Section */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold m-0 flex items-center gap-3">
            {icon && <span className="text-primary">{icon}</span>}
            {title}
          </h1>
          {description && <p className="mt-1 text-text-muted max-w-2xl">{description}</p>}
        </div>
        <div className="flex items-center gap-2">
          {onRefresh && (
            <Button 
              variant="outline"
              className="flex items-center gap-2" 
              onClick={onRefresh} 
              disabled={loading}
            >
              <RefreshCcw size={16} className={loading ? 'animate-spin' : ''} />
              Refresh
            </Button>
          )}
          {toolbarActions}
        </div>
      </div>

      {/* 2. Error Message */}
      {error && (
        <Card className="border-danger/20 text-danger p-4 flex items-center gap-3">
          <X className="shrink-0" size={20} /> {error}
        </Card>
      )}

      {/* 3. Toolbar Section */}
      {(onSearchChange || bulkActions) && (
        <div className="flex gap-4 my-5 flex-wrap items-center" style={{ margin: 0 }}>
          {onSearchChange && (
            <div className="relative flex-1 max-w-md">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-text-muted" size={18} />
              <Input
                className="pl-10"
                placeholder={searchPlaceholder}
                value={searchQuery}
                onChange={(e) => onSearchChange(e.target.value)}
              />
              {searchQuery && (
                <button 
                  onClick={() => onSearchChange('')}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary"
                >
                  <X size={14} />
                </button>
              )}
            </div>
          )}
          
          <div className="flex-1" />

          {bulkActions && (
            <div className="flex items-center gap-2">
              {bulkActions}
            </div>
          )}
        </div>
      )}

      {/* 4. Content Section */}
      <div className="flex-1">
        {loading ? (
          <div className="animate-in fade-in duration-500">
            <TableSkeleton rows={limit > 20 ? 10 : 6} cols={5} />
          </div>
        ) : (
          <div className="animate-in fade-in duration-300">
            {children}
          </div>
        )}
      </div>

      {/* 5. Footer Section */}
      {footer && (
        <div className="mt-auto pt-4 border-t border-white/5">
          {footer}
        </div>
      )}
    </div>
  );
};
