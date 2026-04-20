import { useEffect, useMemo, useState } from 'react'
import { BellRing, Plus, RadioTower, RefreshCw, ShieldCheck, Workflow } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { NativeSelect } from '@/components/ui/select'
import { EmptyState } from '@/components/ui/empty-state'
import { ActionResultNotice } from '@/components/operator/ActionResultNotice'
import { ConfirmActionDialog } from '@/components/operator/ConfirmActionDialog'
import { OperatorHero, OperatorSection, OperatorStats, OperatorStack } from '@/components/operator/OperatorPage'
import { getTrigger, listTriggers, setTriggerEnabled, upsertTrigger, updateTrigger } from '@/lib/api/triggers'
import { fetchChannels } from '@/lib/api/access'
import { fetchAutomations } from '@/lib/api/ops'
import type { AccessChannel, AutomationJob, TriggerDTO } from '@/lib/api/types'
import { DEFAULT_TRIGGER_EVENT_TYPE, TRIGGER_EVENT_OPTIONS, filterTriggerDeliveryChannels } from '@/lib/triggers/catalog'
import { useI18n } from '@/hooks/useI18n'

const defaultDraft = {
  display_name: '',
  description: '',
  event_type: DEFAULT_TRIGGER_EVENT_TYPE,
  channel_id: '',
  automation_job_id: '',
  governance: 'delivery_policy',
  template_id: '',
  filter_expr: '',
  target_audience: '',
  cooldown_sec: 0,
  operator_reason: 'Update trigger from web console',
}

export function TriggersPage() {
  const { t } = useI18n()
  const [items, setItems] = useState<TriggerDTO[]>([])
  const [selectedID, setSelectedID] = useState('')
  const [draft, setDraft] = useState(defaultDraft)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [toggleTarget, setToggleTarget] = useState<TriggerDTO | null>(null)
  const [channels, setChannels] = useState<AccessChannel[]>([])
  const [automations, setAutomations] = useState<AutomationJob[]>([])

  const governanceOptions = [
    { value: 'delivery_policy', label: t('triggers.governance.deliveryPolicy') },
    { value: 'advanced_review', label: t('triggers.governance.advancedReview') },
    { value: 'org_guardrail', label: t('triggers.governance.orgGuardrail') },
    { value: 'audience_routing', label: t('triggers.governance.audienceRouting') },
  ]

  const load = async (preferredID?: string) => {
    setLoading(true)
    setError('')
    try {
      const response = await listTriggers({ limit: 100 })
      const channelsResp = await fetchChannels({ limit: 100 })
      const automationResp = await fetchAutomations({ limit: 100, sort_by: 'id', sort_order: 'asc' })
      const supportedChannels = filterTriggerDeliveryChannels(channelsResp.items || [])
      const nextItems = response.items || []
      setItems(nextItems)
      setChannels(supportedChannels)
      setAutomations(automationResp.items || [])
      const recommendedChannelID = supportedChannels[0]?.id || ''
      const nextID = preferredID || selectedID || nextItems[0]?.id || ''
      if (nextID) {
        const detail = await getTrigger(nextID)
        setSelectedID(nextID)
        setDraft({
          display_name: detail.display_name,
          description: detail.description || '',
          event_type: detail.event_type,
          channel_id: detail.channel_id || detail.channel || recommendedChannelID,
          automation_job_id: detail.automation_job_id || '',
          governance: detail.governance || 'delivery_policy',
          template_id: detail.template_id || '',
          filter_expr: detail.filter_expr || '',
          target_audience: detail.target_audience || '',
          cooldown_sec: detail.cooldown_sec || 0,
          operator_reason: 'Update trigger from web console',
        })
      } else {
        setDraft((current) => ({
          ...current,
          channel_id: current.channel_id || recommendedChannelID,
        }))
      }
    } catch (loadError) {
      setError(String(loadError))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const stats = useMemo(() => ({
    enabled: items.filter((item) => item.enabled).length,
    inbox: items.filter((item) => (item.channel_id || item.channel) === 'inbox-primary').length,
    guardrails: items.filter((item) => (item.governance || '') !== '' && item.governance !== 'delivery_policy').length,
  }), [items])

  const handleSave = async () => {
    setSaving(true)
    setError('')
    setMessage('')
    try {
      const saved = selectedID
        ? await updateTrigger(selectedID, draft)
        : await upsertTrigger(draft)
      setMessage(selectedID ? t('triggers.updated') : t('triggers.created'))
      await load(saved.id)
    } catch (saveError) {
      setError(String(saveError))
    } finally {
      setSaving(false)
    }
  }

  const handleToggle = async () => {
    if (!toggleTarget) return
    setSaving(true)
    setError('')
    setMessage('')
    try {
      await setTriggerEnabled(toggleTarget.id, !toggleTarget.enabled, 'Toggle trigger from web console')
      setMessage(toggleTarget.enabled ? t('triggers.disabled') : t('triggers.enabled'))
      setToggleTarget(null)
      await load(toggleTarget.id)
    } catch (toggleError) {
      setError(String(toggleError))
    } finally {
      setSaving(false)
    }
  }

  const selectedAutomation = automations.find((item) => item.id === draft.automation_job_id)

  return (
    <div className="flex flex-col gap-6 pb-10">
      <OperatorHero
        eyebrow={t('triggers.eyebrow')}
        title={t('triggers.title')}
        description={t('triggers.subtitle')}
        chips={[
          { label: `${stats.enabled} enabled`, tone: stats.enabled > 0 ? 'success' : 'muted' },
          { label: `${stats.inbox} inbox`, tone: 'info' },
          { label: `${stats.guardrails} advanced`, tone: stats.guardrails > 0 ? 'info' : 'muted' },
        ]}
        primaryAction={<Button variant="amber" onClick={() => void load(selectedID)}><RefreshCw size={14} />{t('triggers.refresh')}</Button>}
        secondaryAction={<Button variant="outline" onClick={() => { setSelectedID(''); setDraft(defaultDraft) }}><Plus size={14} />{t('triggers.new')}</Button>}
      />

      <ActionResultNotice tone="error" message={error} />
      <ActionResultNotice tone="success" message={message} />

      <OperatorStats
        stats={[
          { title: t('triggers.stats.rules'), value: items.length, description: t('triggers.stats.rulesDesc'), icon: Workflow, tone: 'info' },
          { title: t('triggers.stats.enabled'), value: stats.enabled, description: t('triggers.stats.enabledDesc'), icon: BellRing, tone: stats.enabled > 0 ? 'success' : 'muted' },
          { title: t('triggers.stats.inbox'), value: stats.inbox, description: t('triggers.stats.inboxDesc'), icon: RadioTower, tone: 'info' },
          { title: t('triggers.stats.guardrails'), value: stats.guardrails, description: t('triggers.stats.guardrailsDesc'), icon: ShieldCheck, tone: stats.guardrails > 0 ? 'info' : 'muted' },
        ]}
      />

      <div className="grid gap-6 xl:grid-cols-[320px_1fr]">
        <OperatorSection title={t('triggers.registry.title')} description={t('triggers.registry.desc')}>
          {loading ? <div className="text-sm text-muted-foreground">{t('triggers.registry.loading')}</div> : items.length === 0 ? <EmptyState icon={Workflow} title={t('triggers.registry.empty.title')} description={t('triggers.registry.empty.desc')} /> : (
            <OperatorStack>
              {items.map((item) => (
                <button key={item.id} type="button" onClick={() => void load(item.id)} className={`rounded-2xl border p-4 text-left ${selectedID === item.id ? 'border-primary/40 bg-primary/5' : 'border-border bg-white/[0.03]'}`}>
                  <div className="flex items-start justify-between gap-3">
                      <div className="space-y-1">
                        <div className="text-sm font-semibold text-foreground">{item.display_name}</div>
                        <div className="mt-1 text-xs text-muted-foreground">{item.event_type}</div>
                        <div className="text-[11px] text-muted-foreground">{item.governance || 'delivery_policy'} • {item.channel_id || item.channel || 'unassigned'}</div>
                      </div>
                    <Badge variant={item.enabled ? 'success' : 'outline'}>{item.enabled ? 'enabled' : 'disabled'}</Badge>
                  </div>
                </button>
              ))}
            </OperatorStack>
          )}
        </OperatorSection>

        <OperatorSection title={selectedID ? t('triggers.detail.title') : t('triggers.detail.createTitle')} description={t('triggers.detail.desc')}>
          <div className="grid gap-4 md:grid-cols-2">
            <Field label={t('triggers.form.displayName')}><Input value={draft.display_name} onChange={(e) => setDraft((current) => ({ ...current, display_name: e.target.value }))} placeholder="Inbox incident updates" /></Field>
            <Field label={t('triggers.form.eventType')}><NativeSelect value={draft.event_type} onChange={(e) => setDraft((current) => ({ ...current, event_type: e.target.value }))}>{TRIGGER_EVENT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label} ({option.value})</option>)}</NativeSelect></Field>
            <Field label={t('triggers.form.governance')}><NativeSelect value={draft.governance} onChange={(e) => setDraft((current) => ({ ...current, governance: e.target.value }))}>{governanceOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</NativeSelect></Field>
            <Field label={t('triggers.form.channel')}><NativeSelect value={draft.channel_id} onChange={(e) => setDraft((current) => ({ ...current, channel_id: e.target.value }))}><option value="">{t('triggers.form.channelSelect')}</option>{channels.map((channel) => <option key={channel.id} value={channel.id}>{channel.name || channel.id} ({channel.kind || channel.type})</option>)}</NativeSelect></Field>
            <Field label={t('triggers.form.automationOwner')}><NativeSelect value={draft.automation_job_id} onChange={(e) => setDraft((current) => ({ ...current, automation_job_id: e.target.value }))}><option value="">{t('triggers.form.standalone')}</option>{automations.map((automation) => <option key={automation.id} value={automation.id}>{automation.display_name || automation.id}</option>)}</NativeSelect></Field>
            <Field label={t('triggers.form.templateId')}><Input value={draft.template_id} onChange={(e) => setDraft((current) => ({ ...current, template_id: e.target.value }))} placeholder="diagnosis-zh-cn" /></Field>
            <Field label={t('triggers.form.targetAudience')}><Input value={draft.target_audience} onChange={(e) => setDraft((current) => ({ ...current, target_audience: e.target.value }))} placeholder="ops.primary / org.admins" /></Field>
            <Field label={t('triggers.form.cooldown')}><Input type="number" value={String(draft.cooldown_sec)} onChange={(e) => setDraft((current) => ({ ...current, cooldown_sec: Number(e.target.value || 0) }))} /></Field>
            <Field label={t('triggers.form.operatorReason')}><Input value={draft.operator_reason} onChange={(e) => setDraft((current) => ({ ...current, operator_reason: e.target.value }))} /></Field>
          </div>
          <Field label={t('triggers.form.description')}><Input value={draft.description} onChange={(e) => setDraft((current) => ({ ...current, description: e.target.value }))} placeholder="Route incident updates into inbox." /></Field>
          <Field label={t('triggers.form.filterExpr')}><Input value={draft.filter_expr} onChange={(e) => setDraft((current) => ({ ...current, filter_expr: e.target.value }))} placeholder="severity in ['warning','critical']" /></Field>
          {selectedAutomation ? <div className="rounded-2xl border border-border bg-white/[0.03] p-4 text-sm text-muted-foreground">{t('triggers.form.automationOwnerLabel')} <span className="font-medium text-foreground">{selectedAutomation.display_name || selectedAutomation.id}</span></div> : null}
          {selectedID ? (
            <div className="grid gap-4 md:grid-cols-3 rounded-2xl border border-border bg-white/[0.03] p-4 text-xs text-muted-foreground">
              <div>
                <div className="uppercase tracking-widest mb-1">{t('triggers.lifecycle.title')}</div>
                <div className="text-foreground">{items.find((item) => item.id === selectedID)?.enabled ? 'active' : 'disabled'}</div>
              </div>
              <div>
                <div className="uppercase tracking-widest mb-1">{t('triggers.lifecycle.governance')}</div>
                <div className="text-foreground">{items.find((item) => item.id === selectedID)?.governance || 'delivery_policy'}</div>
              </div>
              <div>
                <div className="uppercase tracking-widest mb-1">{t('triggers.lifecycle.updated')}</div>
                <div className="text-foreground">{items.find((item) => item.id === selectedID)?.updated_at || 'n/a'}</div>
              </div>
              <div className="md:col-span-1">
                <div className="uppercase tracking-widest mb-1">{t('triggers.lifecycle.lastFired')}</div>
                <div className="text-foreground">{items.find((item) => item.id === selectedID)?.last_fired_at || t('triggers.lifecycle.never')}</div>
              </div>
            </div>
          ) : null}
          <div className="flex flex-wrap gap-3">
            <Button variant="amber" onClick={() => void handleSave()} disabled={saving}>{saving ? t('triggers.saving') : selectedID ? t('triggers.save') : t('triggers.create')}</Button>
            {selectedID ? <Button variant="outline" onClick={() => setToggleTarget(items.find((item) => item.id === selectedID) || null)}>{items.find((item) => item.id === selectedID)?.enabled ? t('triggers.disableRule') : t('triggers.enableRule')}</Button> : null}
          </div>
        </OperatorSection>
      </div>

      <ConfirmActionDialog
        open={Boolean(toggleTarget)}
        onOpenChange={(open) => { if (!open) setToggleTarget(null) }}
        title={toggleTarget?.enabled ? t('triggers.confirmDisable.title') : t('triggers.confirmEnable.title')}
        description={toggleTarget?.enabled ? t('triggers.confirmDisable.desc') : t('triggers.confirmEnable.desc')}
        confirmLabel={toggleTarget?.enabled ? t('triggers.disableRule') : t('triggers.enableRule')}
        loading={saving}
        onConfirm={() => void handleToggle()}
      />
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <div className="text-sm font-medium text-foreground">{label}</div>
      {children}
    </div>
  )
}
