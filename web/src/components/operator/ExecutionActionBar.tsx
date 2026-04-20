import { useMemo, useState } from 'react'
import { CheckCheck, ChevronDown, ChevronUp, Info, PencilLine, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { ActionResultNotice } from '@/components/operator/ActionResultNotice'
import { ConfirmActionDialog } from '@/components/operator/ConfirmActionDialog'
import { useNotify } from '@/hooks/ui/useNotify'
import { useI18n } from '@/hooks/useI18n'
import { approveExecution, modifyApproveExecution, rejectExecution, requestExecutionContext } from '@/lib/api/ops'
import type { ExecutionDetail } from '@/lib/api/types'

type PendingAction = 'approve' | 'reject' | 'request_context' | 'modify_approve' | null

export function ExecutionActionBar({
  execution,
  onUpdated,
  className,
}: {
  execution: ExecutionDetail
  onUpdated?: (execution: ExecutionDetail) => void
  className?: string
}) {
  const { t } = useI18n()
  const notify = useNotify()
  const [pendingAction, setPendingAction] = useState<PendingAction>(null)
  const [loading, setLoading] = useState(false)
  const [command, setCommand] = useState(execution.command || '')
  const [reason, setReason] = useState('')
  const [modifyExpanded, setModifyExpanded] = useState(false)
  const [notice, setNotice] = useState<{ tone: 'success' | 'error' | 'warning' | 'info'; message: string } | null>(null)

  const isPending = execution.status === 'pending'
  const risk = execution.golden_summary?.risk || execution.risk_level || 'info'
  const approvalSummary = execution.golden_summary?.approval || t('executions.action.approvalSummaryEmpty', 'No approval summary recorded yet.')
  const manualGateMessage = risk === 'critical'
    ? t('executions.action.manualGateCritical', 'High-risk request. This command will only run after an operator approves it.')
    : t('executions.action.manualGate', 'This request stays queued until an operator approves it.')
  const title = useMemo(() => {
    switch (pendingAction) {
      case 'approve':
        return t('executions.action.confirmApproveTitle', 'Approve execution')
      case 'reject':
        return t('executions.action.confirmRejectTitle', 'Reject execution')
      case 'request_context':
        return t('executions.action.confirmRequestContextTitle', 'Request more context')
      case 'modify_approve':
        return t('executions.action.confirmModifyTitle', 'Modify and approve')
      default:
        return ''
    }
  }, [pendingAction, t])

  const description = useMemo(() => {
    switch (pendingAction) {
      case 'approve':
        return t('executions.action.confirmApproveDesc', 'Approve this execution and allow the run pipeline to continue.')
      case 'reject':
        return t('executions.action.confirmRejectDesc', 'Reject this execution request and send it back for analysis.')
      case 'request_context':
        return t('executions.action.confirmRequestContextDesc', 'Pause here and request more context before any command runs.')
      case 'modify_approve':
        return t('executions.action.confirmModifyDesc', 'Replace the command, then approve the updated execution.')
      default:
        return ''
    }
  }, [pendingAction, t])

  if (!isPending) {
    return null
  }

  const runAction = async () => {
    if (!pendingAction) {
      return
    }
    setLoading(true)
    setNotice(null)
    try {
      let updated: ExecutionDetail
      switch (pendingAction) {
        case 'approve':
          updated = await approveExecution(execution.execution_id, reason ? { reason } : undefined)
          break
        case 'reject':
          updated = await rejectExecution(execution.execution_id, { reason })
          break
        case 'request_context':
          updated = await requestExecutionContext(execution.execution_id)
          break
        case 'modify_approve':
          updated = await modifyApproveExecution(execution.execution_id, { command, ...(reason ? { reason } : {}) })
          break
        default:
          return
      }
      onUpdated?.(updated)
      setNotice({ tone: 'success', message: t('executions.action.successStatus', { defaultValue: 'Execution is now {{status}}.', status: updated.status }) })
      notify.success(`Execution ${updated.execution_id} updated to ${updated.status}.`, 'Approval updated')
      setPendingAction(null)
      setReason('')
    } catch (error) {
      setNotice({ tone: 'error', message: t('executions.action.failedStatus', 'Approval action failed.') })
      notify.error(error, 'Approval action failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={className}>
      <div className="flex flex-col gap-3 rounded-2xl border border-warning/30 bg-warning/5 p-4">
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="text-sm font-semibold text-foreground">{t('executions.action.pendingTitle', 'Pending approval')}</div>
            <div className="text-sm text-muted-foreground">{t('executions.action.pendingDesc', 'Review the execution request and choose an action below.')}</div>
          </div>
          <Info className="size-4 text-warning" />
        </div>

        <div className="rounded-2xl border border-warning/40 bg-black/20 p-3 text-sm leading-6 text-foreground">
          {manualGateMessage}
        </div>

        <div className="grid gap-3 md:grid-cols-3">
          <SummaryField label={t('executions.action.summaryRisk', 'Risk')} value={risk} />
          <SummaryField label={t('executions.action.summaryStatus', 'Status')} value={execution.status} />
          <SummaryField label={t('executions.action.summaryApproval', 'Approval note')} value={approvalSummary} />
        </div>

        {/* Modify command — collapsed by default */}
        <div className="flex flex-col gap-2">
          <button
            type="button"
            className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors w-fit"
            onClick={() => setModifyExpanded((prev) => !prev)}
            aria-expanded={modifyExpanded}
          >
            {modifyExpanded ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
            {modifyExpanded ? t('executions.action.modifyHide', 'Hide command override') : t('executions.action.modifyLabel', 'Modify command before approving')}
          </button>
          {modifyExpanded && (
            <Input value={command} onChange={(event) => setCommand(event.target.value)} placeholder={t('executions.action.modifyPlaceholder', 'Replacement command...')} />
          )}
        </div>

        <div className="flex flex-wrap gap-2">
          <Button variant="amber" size="sm" onClick={() => setPendingAction('approve')}><CheckCheck size={14} />{t('executions.action.approve', 'Approve')}</Button>
          <Button variant="outline" size="sm" onClick={() => { setModifyExpanded(true); setPendingAction('modify_approve') }}><PencilLine size={14} />{t('executions.action.modifyApprove', 'Modify + approve')}</Button>
          <Button variant="outline" size="sm" onClick={() => setPendingAction('request_context')}><Info size={14} />{t('executions.action.requestContext', 'Request context')}</Button>
          <Button variant="destructive" size="sm" onClick={() => setPendingAction('reject')}><X size={14} />{t('executions.action.reject', 'Reject')}</Button>
        </div>
        <ActionResultNotice tone={notice?.tone || 'info'} message={notice?.message} />
      </div>

      <ConfirmActionDialog
        open={pendingAction !== null}
        onOpenChange={(open) => {
          if (!open) {
            setPendingAction(null)
            setReason('')
          }
        }}
        title={title}
        description={description}
        confirmLabel={pendingAction === 'reject' ? t('executions.action.confirmRejectLabel', 'Reject') : pendingAction === 'request_context' ? t('executions.action.confirmRequestContextLabel', 'Request context') : t('executions.action.confirmContinueLabel', 'Continue')}
        loading={loading}
        danger={pendingAction === 'reject'}
        onConfirm={() => void runAction()}
        extraContent={
          (pendingAction === 'approve' || pendingAction === 'reject' || pendingAction === 'modify_approve') ? (
            <div className="flex flex-col gap-1.5">
              <label className="text-sm font-medium text-foreground">
                {pendingAction === 'reject'
                  ? t('executions.action.rejectionReason', 'Rejection reason')
                  : pendingAction === 'modify_approve'
                    ? t('executions.action.modifyNoteOptional', 'Modification note (optional)')
                    : t('executions.action.noteOptional', 'Approval note (optional)')}
              </label>
              <Textarea
                rows={3}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder={pendingAction === 'reject'
                  ? t('executions.action.rejectionPlaceholder', 'Explain why this execution is being rejected...')
                  : pendingAction === 'modify_approve'
                    ? t('executions.action.modifyNotePlaceholder', 'Explain why the command was changed before approval...')
                    : t('executions.action.notePlaceholder', 'Record why this approval is safe to proceed...')}
              />
            </div>
          ) : pendingAction === 'request_context' ? (
            <div className="flex flex-col gap-1.5">
              <div className="text-sm font-medium text-foreground">{t('executions.action.contextRequestNote', 'Context request note')}</div>
              <div className="text-sm leading-6 text-muted-foreground">{t('executions.action.contextRequestHelp', 'This asks the linked session for more evidence and does not run the command.')}</div>
            </div>
          ) : undefined
        }
      />
    </div>
  )
}

function SummaryField({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-border/60 bg-black/20 p-3">
      <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className="mt-2 text-sm leading-6 text-foreground">{value}</div>
    </div>
  )
}
