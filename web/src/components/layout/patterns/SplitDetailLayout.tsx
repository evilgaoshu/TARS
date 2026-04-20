import React from 'react';
import type { ReactNode } from 'react';
import { clsx } from 'clsx';
import { DetailSkeleton } from '@/components/ui/layout-skeleton';

interface SplitDetailLayoutProps {
  sidebar: ReactNode;
  children: ReactNode;
  sidebarWidth?: string;
  loading?: boolean;
}

/**
 * Standard layout for Master-Detail views (Connectors, Skills, Docs)
 */
export const SplitDetailLayout: React.FC<SplitDetailLayoutProps> = ({
  sidebar,
  children,
  sidebarWidth = '340px',
  loading = false,
}) => {
  return (
    <div 
      className="grid grid-cols-[minmax(320px,0.95fr)_minmax(420px,1.35fr)] gap-4 items-start h-full items-stretch"
      style={{ gridTemplateColumns: `${sidebarWidth} 1fr` }}
    >
      {/* Sidebar (Master) */}
      <aside className="glass-card p-0 flex flex-col overflow-hidden h-full">
        <div className={clsx("flex-1 overflow-y-auto p-5", loading && "opacity-50 pointer-events-none")}>
          {sidebar}
        </div>
      </aside>

      {/* Main Content (Detail) */}
      <main className="flex flex-col overflow-y-auto h-full p-8 pt-0">
        {loading ? (
          <DetailSkeleton />
        ) : (
          <div className="animate-in fade-in duration-300 flex-1">
            {children}
          </div>
        )}
      </main>
    </div>
  );
};
