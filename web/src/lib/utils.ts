import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/**
 * @deprecated Use `@/components/ui/status-badge` instead.
 * This function returns legacy `.badge-*` CSS class names and remains only for older callers.
 */
export function getStatusBadgeStyle(status?: string): string {
  switch ((status || '').toLowerCase()) {
    case 'healthy':
    case 'success':
    case 'enabled':
    case 'active':
    case 'approved':
    case 'completed':
    case 'resolved':
    case 'delivered':
    case 'set':
      return 'badge-success';
    case 'info':
    case 'open':
    case 'pending':
    case 'processing':
    case 'executing':
    case 'verifying':
    case 'analyzing':
      return 'badge-info';
    case 'warning':
    case 'degraded':
    case 'pending_approval':
    case 'timeout':
    case 'blocked':
      return 'badge-warning';
    case 'critical':
    case 'error':
    case 'failed':
    case 'rejected':
    case 'missing':
    case 'disabled':
      return 'badge-danger';
    default:
      return 'badge-muted';
  }
}
