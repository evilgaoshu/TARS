import type { ReactNode } from 'react'
import { Loader2 } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogClose,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface GuidedFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  /** The form content (fields, etc.) */
  children: ReactNode
  /** Called when the primary action button is clicked — NOT on form submit. Wire submit inside children. */
  onConfirm?: () => void
  confirmLabel?: string
  cancelLabel?: string
  /** Shows a loading spinner on the confirm button and disables both buttons */
  loading?: boolean
  /** Extra class on DialogContent */
  className?: string
  /** Extra class on the scrollable body wrapper */
  bodyClassName?: string
  /** Wide variant — useful for complex forms */
  wide?: boolean
}

/**
 * GuidedFormDialog — Operator workbench modal for create / edit flows.
 *
 * Usage:
 *   <GuidedFormDialog open={open} onOpenChange={setOpen} title="New Channel" onConfirm={form.handleSubmit(onSave)} loading={saving}>
 *     <Form ...><form>...</form></Form>
 *   </GuidedFormDialog>
 *
 * If you need native form submission, omit `onConfirm` and put a submit button inside `children`.
 */
export function GuidedFormDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  onConfirm,
  confirmLabel = 'Save',
  cancelLabel = 'Cancel',
  loading = false,
  className,
  bodyClassName,
  wide = false,
}: GuidedFormDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={cn('max-h-[92vh] !flex !flex-col overflow-hidden p-0', wide ? 'max-w-5xl' : 'max-w-lg', className)}>
        <DialogHeader className="border-b border-white/10 px-6 pb-4 pt-6">
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>

        <div
          data-slot="guided-form-body"
          className={cn('min-h-0 flex-1 overflow-y-auto px-6 py-5', bodyClassName)}
        >
          {children}
        </div>

        {onConfirm ? (
          <DialogFooter className="border-t border-white/10 px-6 py-4">
            <DialogClose asChild>
              <Button variant="ghost" size="sm" disabled={loading}>{cancelLabel}</Button>
            </DialogClose>
            <Button variant="amber" size="sm" onClick={onConfirm} disabled={loading}>
              {loading ? <Loader2 size={12} className="animate-spin" /> : null}
              {confirmLabel}
            </Button>
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
