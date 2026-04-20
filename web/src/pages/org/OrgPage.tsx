import React, { useState, useEffect, useCallback, type ReactNode } from 'react'
import {
  Building2,
  Users2,
  LayoutGrid,
  ShieldAlert,
  Save,
  Edit2,
  Eye,
  EyeOff,
  Check,
  X,
  Plus,
  type LucideIcon,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { NativeSelect } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { useI18n } from '@/hooks/useI18n'
import { SectionTitle, SummaryGrid, StatCard } from '@/components/ui/page-hero'
import {
  SplitLayout,
  RegistrySidebar,
  RegistryPanel,
  RegistryCard,
  RegistryDetail,
} from '@/components/ui/registry-primitives'
import { DetailHeader } from '@/components/ui/detail-header'
import { EmptyDetailState } from '@/components/ui/empty-detail-state'

import {
  listOrganizations,
  createOrganization,
  updateOrganization,
  setOrganizationStatus,
  listTenants,
  createTenant,
  updateTenant,
  setTenantStatus,
  listWorkspaces,
  createWorkspace,
  updateWorkspace,
  setWorkspaceStatus,
  getOrgPolicy,
  setOrgPolicy,
  getTenantPolicy,
  setTenantPolicy,
  resolvePolicy,
} from '@/lib/api/org'

import type {
  OrgItem,
  TenantItem,
  WorkspaceItem,
  OrgPolicy,
  TenantPolicy,
  ResolvedPolicy,
} from '@/lib/api/types'

// ─── Local Components ──────────────────────────────────────────────────────

const SidebarBody = ({ children }: { children: ReactNode }) => (
  <CardContent className="flex flex-col gap-5 p-5">{children}</CardContent>
)

const DetailBody = ({ children }: { children: ReactNode }) => (
  <CardContent className="flex flex-col gap-5 p-6">{children}</CardContent>
)

const PolicyCard = ({ children }: { children: ReactNode }) => (
  <Card className="flex flex-col gap-5 p-6">{children}</Card>
)

// ─── Types & Constants ──────────────────────────────────────────────────────

type TabId = 'organizations' | 'tenants' | 'workspaces' | 'policy'

const TABS: { id: TabId; labelKey: string; icon: LucideIcon }[] = [
  { id: 'organizations', labelKey: 'org.tabs.organizations', icon: Building2 },
  { id: 'tenants', labelKey: 'org.tabs.tenants', icon: Users2 },
  { id: 'workspaces', labelKey: 'org.tabs.workspaces', icon: LayoutGrid },
  { id: 'policy', labelKey: 'org.tabs.policy', icon: ShieldAlert },
]

const FORM_GRID_CLASS = 'grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4'
const READ_ONLY_CLASS = 'bg-muted/50 border-dashed cursor-not-allowed opacity-80'
const SELECT_CLASS = 'h-10 w-full'
const PAGE_STACK_CLASS = 'flex flex-col gap-6'

const fmtDate = (s?: string) => (s ? new Date(s).toLocaleString() : '-')
const formatPolicyLabel = (value: string) => value.replace(/_/g, ' ')

const TabBar = ({ active, onChange }: { active: TabId; onChange: (t: TabId) => void }) => {
  const { t } = useI18n()
  return (
    <div role="tablist" aria-label="Organization sections" className="mb-6 flex border-b border-border/70">
      {TABS.map((tab) => {
        const Icon = tab.icon

        return (
          <button
            key={tab.id}
            id={`org-tab-${tab.id}`}
            type="button"
            role="tab"
            aria-selected={active === tab.id}
            aria-controls={`org-panel-${tab.id}`}
            onClick={() => onChange(tab.id)}
            className={cn(
              'mb-[-1px] inline-flex items-center gap-2 border-b-2 px-4 py-3 text-sm font-medium transition-colors',
              active === tab.id
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            <Icon className="size-4" />
            {t(tab.labelKey)}
          </button>
        )
      })}
    </div>
  )
}

const TabPanel = ({
  id,
  active,
  children,
}: {
  id: TabId
  active: boolean
  children: React.ReactNode
}) => (
  <div
    role="tabpanel"
    id={`org-panel-${id}`}
    aria-labelledby={`org-tab-${id}`}
    hidden={!active}
    className={cn('animate-in fade-in slide-in-from-top-1 duration-300', !active && 'hidden')}
  >
    {children}
  </div>
)

const LabeledField = ({
  label,
  children,
  required,
}: {
  label: string
  children: React.ReactNode
  required?: boolean
}) => (
  <div className="flex flex-col gap-2">
    <label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
      {label}
      {required && <span className="ml-1 text-destructive">*</span>}
    </label>
    {children}
  </div>
)

const ActiveBadge = ({ active, label }: { active: boolean; label: string }) => (
  <Badge variant={active ? 'success' : 'muted'} className="h-5 px-2 text-[10px] uppercase font-bold">
    {label}
  </Badge>
)

const TimestampGrid = ({ createdAt, updatedAt }: { createdAt?: string; updatedAt?: string }) => {
  const { t } = useI18n()
  return (
    <div className="grid gap-2 text-sm text-muted-foreground sm:grid-cols-2">
      <span>{t('common.created', 'Created')}: {fmtDate(createdAt)}</span>
      <span>{t('common.updated', 'Updated')}: {fmtDate(updatedAt)}</span>
    </div>
  )
}

const ToggleStatusButton = ({
  active,
  onClick,
}: {
  active: boolean
  onClick: (event: React.MouseEvent<HTMLButtonElement>) => void
}) => {
  const { t } = useI18n()
  return (
    <Button
      type="button"
      variant="outline"
      size="icon"
      className="size-7 rounded-lg"
      aria-label={active ? t('action.disable', 'Disable') : t('action.enable', 'Enable')}
      onClick={onClick}
    >
      {active ? <EyeOff /> : <Eye />}
    </Button>
  )
}

const BoolRow = ({
  label,
  value,
  editing,
  onChange,
}: {
  label: string
  value: boolean
  editing: boolean
  onChange: (v: boolean) => void
}) => {
  const { t } = useI18n()
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border/50 py-3 last:border-b-0">
      <span className="text-sm font-medium text-foreground">{label}</span>
      {editing ? (
        <Button
          variant={value ? 'amber' : 'outline'}
          size="sm"
          className="h-7 px-3 text-xs"
          onClick={() => onChange(!value)}
        >
          {value ? t('org.boolYes', 'YES') : t('org.boolNo', 'NO')}
        </Button>
      ) : value ? (
        <Check className="size-4 text-success" />
      ) : (
        <X className="size-4 text-muted-foreground" />
      )}
    </div>
  )
}

const ListRow = ({
  label,
  value,
  editing,
  onChange,
}: {
  label: string
  value: string[]
  editing: boolean
  onChange: (v: string[]) => void
}) => {
  const [inp, setInp] = useState('')

  const add = () => {
    const s = inp.trim()
    if (s && !value.includes(s)) {
      onChange([...value, s])
      setInp('')
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
        {label}
      </label>
      <div className="flex flex-wrap gap-1.5 min-h-[32px] items-center">
        {value.length ? (
          value.map((tag) => (
            <Badge
              key={tag}
              variant="secondary"
              className={cn('group rounded-full pl-2.5 pr-1 py-0.5', editing && 'pr-1.5')}
            >
              {tag}
              {editing && (
                <button
                  type="button"
                  onClick={() => onChange(value.filter((v) => v !== tag))}
                  className="ml-1 rounded-full p-0.5 hover:bg-black/10"
                >
                  <X className="size-3" />
                </button>
              )}
            </Badge>
          ))
        ) : (
          <span className="text-sm text-muted-foreground italic opacity-50">- none -</span>
        )}
      </div>
      {editing && (
        <div className="flex gap-2 mt-1">
          <Input
            value={inp}
            onChange={(e) => setInp(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                add()
              }
            }}
            placeholder="Add new item..."
            className="h-8 text-xs"
          />
          <Button size="sm" variant="outline" className="h-8 px-2" onClick={add}>
            <Plus className="size-3" />
          </Button>
        </div>
      )}
    </div>
  )
}

const OrgStatusMsg = ({ ok, error }: { ok?: string; error?: string }) => (
  <>
    {ok && (
      <div className="animate-in fade-in slide-in-from-left-2 rounded-xl bg-success/10 p-3 text-sm font-medium text-success border border-success/20">
        {ok}
      </div>
    )}
    {error && (
      <div className="animate-in fade-in slide-in-from-left-2 rounded-xl bg-destructive/10 p-3 text-sm font-medium text-destructive border border-destructive/20">
        {error}
      </div>
    )}
  </>
)

const OrgsTab: React.FC = () => {
  const { t } = useI18n()
  const [items, setItems] = useState<OrgItem[]>([])
  const [selected, setSelected] = useState<OrgItem | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [draft, setDraft] = useState<Partial<OrgItem>>({})
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState('')
  const [ok, setOk] = useState('')

  useEffect(() => {
    listOrganizations()
      .then((res) => setItems(res.items ?? []))
      .catch(() => setItems([]))
  }, [])

  const startCreate = () => {
    setIsCreating(true)
    setSelected(null)
    setDraft({})
    setErr('')
    setOk('')
  }

  const save = async () => {
    setSaving(true)
    setErr('')
    setOk('')

    try {
      if (isCreating) {
        if (!draft.id || !draft.name) {
          setErr(t('org.error.idNameRequired'))
          setSaving(false)
          return
        }

        const created = await createOrganization(draft)
        setItems((prev) => [...prev, created])
        setSelected(created)
        setIsCreating(false)
        setOk(t('org.success.created'))
      } else if (selected) {
        const updated = await updateOrganization(selected.id, draft)
        setItems((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
        setSelected(updated)
        setDraft({ ...updated })
        setOk(t('org.success.saved'))
      }
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : t('error.default'))
    }

    setSaving(false)
  }

  const toggleStatus = async (item: OrgItem) => {
    try {
      const updated = await setOrganizationStatus(item.id, item.status !== 'active')
      setItems((prev) => prev.map((org) => (org.id === updated.id ? updated : org)))
      if (selected?.id === updated.id) {
        setSelected(updated)
        setDraft({ ...updated })
      }
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : t('status.error'))
    }
  }

  const activeCount = items.filter((item) => item.status === 'active' || !item.status).length

  return (
    <div className={PAGE_STACK_CLASS}>
      <SummaryGrid>
        <StatCard title={t('org.stats.total')} value={String(items.length)} subtitle={t('org.stats.allOrgs')} />
        <StatCard title={t('org.stats.active')} value={String(activeCount)} subtitle={t('status.enabled')} />
        <StatCard title={t('org.stats.inactive')} value={String(items.length - activeCount)} subtitle={t('status.disabled')} />
      </SummaryGrid>

      <OrgStatusMsg ok={ok} error={err} />

      <SplitLayout
        sidebar={
          <RegistrySidebar>
            <SidebarBody>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 className="text-lg font-semibold tracking-tight text-foreground">{t('org.tabs.organizations')}</h2>
                <Button id="org-new-btn" variant="amber" type="button" onClick={startCreate}>
                  {t('org.sidebar.new')}
                </Button>
              </div>

              <RegistryPanel title={t('org.sidebar.directory')} emptyText={t('org.sidebar.empty')}>
                {items.map((item) => {
                  const isActive = item.status === 'active' || !item.status

                  return (
                    <div
                      key={item.id}
                      className="cursor-pointer rounded-2xl"
                      onClick={() => {
                        if (!isCreating) {
                          setSelected(item)
                          setDraft({ ...item })
                          setErr('')
                          setOk('')
                        }
                      }}
                    >
                      <RegistryCard
                        active={selected?.id === item.id}
                        title={item.name}
                        subtitle={item.id}
                        lines={[
                          item.domain ? `${t('org.sidebar.domain')}${item.domain}` : '',
                          `${t('org.sidebar.status')}${item.status || 'active'}`,
                        ].filter(Boolean)}
                        status={<ActiveBadge active={isActive} label={item.status || 'active'} />}
                        action={
                          <ToggleStatusButton
                            active={isActive}
                            onClick={(event) => {
                              event.stopPropagation()
                              void toggleStatus(item)
                            }}
                          />
                        }
                      />
                    </div>
                  )
                })}
              </RegistryPanel>
            </SidebarBody>
          </RegistrySidebar>
        }
        detail={
          selected || isCreating ? (
            <RegistryDetail>
              <DetailBody>
                <DetailHeader
                  title={isCreating ? t('org.detail.newOrg') : selected!.name}
                  subtitle={isCreating ? t('org.detail.newOrgDesc') : `ID: ${selected!.id}`}
                  status={
                    !isCreating && selected ? (
                      <ActiveBadge
                        active={!selected.status || selected.status === 'active'}
                        label={selected.status || 'active'}
                      />
                    ) : undefined
                  }
                  actions={
                    <>
                      <Button id="org-save-btn" variant="amber" onClick={save} disabled={saving}>
                        <Save />
                        {saving ? t('org.detail.saving') : t('org.detail.save')}
                      </Button>
                      {isCreating ? (
                        <Button variant="outline" onClick={() => setIsCreating(false)}>
                          {t('org.detail.cancel')}
                        </Button>
                      ) : null}
                    </>
                  }
                />

                <div className={FORM_GRID_CLASS}>
                  <LabeledField label="ID" required={isCreating}>
                    <Input
                      placeholder="acme"
                      value={draft.id ?? ''}
                      readOnly={!isCreating}
                      className={cn(!isCreating && READ_ONLY_CLASS)}
                      onChange={(e) => setDraft((prev) => ({ ...prev, id: e.target.value }))}
                    />
                  </LabeledField>

                  <LabeledField label={t('org.detail.name')} required>
                    <Input
                      placeholder="Acme Corp"
                      value={draft.name ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...prev, name: e.target.value }))}
                    />
                  </LabeledField>

                  <LabeledField label={t('org.detail.slug')}>
                    <Input
                      placeholder="acme"
                      value={draft.slug ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...prev, slug: e.target.value }))}
                    />
                  </LabeledField>

                  <LabeledField label={t('org.detail.domain')}>
                    <Input
                      placeholder="acme.com"
                      value={draft.domain ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...prev, domain: e.target.value }))}
                    />
                  </LabeledField>
                </div>

                <LabeledField label={t('org.detail.description')}>
                  <Textarea
                    rows={3}
                    placeholder="Brief description..."
                    value={draft.description ?? ''}
                    onChange={(e) => setDraft((prev) => ({ ...prev, description: e.target.value }))}
                  />
                </LabeledField>

                {!isCreating && selected ? (
                  <TimestampGrid createdAt={selected.created_at} updatedAt={selected.updated_at} />
                ) : null}
              </DetailBody>
            </RegistryDetail>
          ) : (
            <EmptyDetailState
              title={t('org.detail.emptyTitle')}
              description={t('org.detail.emptyDesc')}
            />
          )
        }
      />
    </div>
  )
}

const TenantsTab: React.FC = () => {
  const { t } = useI18n()
  const [items, setItems] = useState<TenantItem[]>([])
  const [orgs, setOrgs] = useState<OrgItem[]>([])
  const [filterOrg, setFilterOrg] = useState('')
  const [selected, setSelected] = useState<TenantItem | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [draft, setDraft] = useState<Partial<TenantItem>>({})
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState('')
  const [ok, setOk] = useState('')

  const load = useCallback(async () => {
    try {
      const [tenantsResponse, orgsResponse] = await Promise.all([
        listTenants(filterOrg || undefined),
        listOrganizations(),
      ])
      setItems(tenantsResponse.items ?? [])
      setOrgs(orgsResponse.items ?? [])
    } catch {
      setItems([])
    }
  }, [filterOrg])

  useEffect(() => {
    const doLoad = async () => {
      await load()
    }

    void doLoad()
  }, [load])

  const save = async () => {
    setSaving(true)
    setErr('')
    setOk('')

    try {
      if (isCreating) {
        if (!draft.id || !draft.name) {
          setErr(t('org.error.idNameRequired'))
          setSaving(false)
          return
        }

        const created = await createTenant(draft)
        setItems((prev) => [...prev, created])
        setSelected(created)
        setIsCreating(false)
        setOk(t('org.success.created'))
      } else if (selected) {
        const updated = await updateTenant(selected.id, draft)
        setItems((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
        setSelected(updated)
        setDraft({ ...updated })
        setOk(t('org.success.saved'))
      }
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : t('error.default'))
    }

    setSaving(false)
  }

  const toggleStatus = async (item: TenantItem) => {
    try {
      const updated = await setTenantStatus(item.id, item.status !== 'active')
      setItems((prev) => prev.map((tenant) => (tenant.id === updated.id ? updated : tenant)))
      if (selected?.id === updated.id) {
        setSelected(updated)
        setDraft({ ...updated })
      }
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : t('status.error'))
    }
  }

  const activeCount = items.filter((item) => item.status === 'active' || !item.status).length

  return (
    <div className={PAGE_STACK_CLASS}>
      <SummaryGrid>
        <StatCard title={t('org.stats.total')} value={String(items.length)} subtitle={filterOrg ? t('org.stats.filteredTenants') : t('org.stats.allTenants')} />
        <StatCard title={t('org.stats.active')} value={String(activeCount)} subtitle={t('status.enabled')} />
        <StatCard title={t('org.stats.organizations')} value={String(orgs.length)} subtitle={t('org.stats.available')} />
      </SummaryGrid>

      <OrgStatusMsg ok={ok} error={err} />

      <SplitLayout
        sidebar={
          <RegistrySidebar>
            <SidebarBody>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 className="text-lg font-semibold tracking-tight text-foreground">{t('org.tabs.tenants')}</h2>
                <Button
                  id="tenant-new-btn"
                  variant="amber"
                  type="button"
                  onClick={() => {
                    setIsCreating(true)
                    setSelected(null)
                    setDraft({ org_id: filterOrg || undefined })
                    setErr('')
                    setOk('')
                  }}
                >
                  {t('org.sidebar.new')}
                </Button>
              </div>

              <NativeSelect value={filterOrg} onChange={(e) => setFilterOrg(e.target.value)} className={SELECT_CLASS}>
                <option value="">{t('org.stats.allOrgs')}</option>
                {orgs.map((org) => (
                  <option key={org.id} value={org.id}>
                    {org.name}
                  </option>
                ))}
              </NativeSelect>

              <RegistryPanel title={t('org.sidebar.directory')} emptyText={t('org.sidebar.emptyTenants')}>
                {items.map((item) => {
                  const isActive = item.status === 'active' || !item.status

                  return (
                    <div
                      key={item.id}
                      className="cursor-pointer rounded-2xl"
                      onClick={() => {
                        if (!isCreating) {
                          setSelected(item)
                          setDraft({ ...item })
                          setErr('')
                          setOk('')
                        }
                      }}
                    >
                      <RegistryCard
                        active={selected?.id === item.id}
                        title={item.name}
                        subtitle={`${item.id}${item.org_id ? ` · ${item.org_id}` : ''}`}
                        lines={[
                          item.default_locale ? `${t('org.sidebar.locale')}${item.default_locale}` : '',
                          item.default_timezone ? `${t('org.sidebar.tz')}${item.default_timezone}` : '',
                        ].filter(Boolean)}
                        status={<ActiveBadge active={isActive} label={item.status || 'active'} />}
                        action={
                          <ToggleStatusButton
                            active={isActive}
                            onClick={(event) => {
                              event.stopPropagation()
                              void toggleStatus(item)
                            }}
                          />
                        }
                      />
                    </div>
                  )
                })}
              </RegistryPanel>
            </SidebarBody>
          </RegistrySidebar>
        }
        detail={
          selected || isCreating ? (
            <RegistryDetail>
              <DetailBody>
                <DetailHeader
                  title={isCreating ? t('org.detail.newTenant') : selected!.name}
                  subtitle={isCreating ? t('org.detail.newTenantDesc') : `ID: ${selected!.id}`}
                  status={
                    !isCreating && selected ? (
                      <ActiveBadge
                        active={!selected.status || selected.status === 'active'}
                        label={selected.status || 'active'}
                      />
                    ) : undefined
                  }
                  actions={
                    <>
                      <Button id="tenant-save-btn" variant="amber" onClick={save} disabled={saving}>
                        <Save />
                        {saving ? t('org.detail.saving') : t('org.detail.save')}
                      </Button>
                      {isCreating ? (
                        <Button variant="outline" onClick={() => setIsCreating(false)}>
                          {t('org.detail.cancel')}
                        </Button>
                      ) : null}
                    </>
                  }
                />

                <div className={FORM_GRID_CLASS}>
                  <LabeledField label="ID" required={isCreating}>
                    <Input
                      placeholder="acme-prod"
                      value={draft.id ?? ''}
                      readOnly={!isCreating}
                      className={cn(!isCreating && READ_ONLY_CLASS)}
                      onChange={(e) => setDraft((prev) => ({ ...prev, id: e.target.value }))}
                    />
                  </LabeledField>

                  <LabeledField label={t('org.detail.name', 'Name')} required>
                    <Input
                      placeholder="Acme Production"
                      value={draft.name ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...prev, name: e.target.value }))}
                    />
                  </LabeledField>

                  <LabeledField label={t('org.detail.slug', 'Slug')}>
                    <Input
                      placeholder="acme-prod"
                      value={draft.slug ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...prev, slug: e.target.value }))}
                    />
                  </LabeledField>

                  <LabeledField label={t('org.sidebar.org')}>
                    <NativeSelect
                      value={draft.org_id ?? ''}
                      disabled={!isCreating}
                      className={cn(SELECT_CLASS, !isCreating && READ_ONLY_CLASS)}
                      onChange={(e) => setDraft((prev) => ({ ...prev, org_id: e.target.value }))}
                    >
                       <option value="">{t('org.detail.selectOrg')}</option>
                      {orgs.map((org) => (
                        <option key={org.id} value={org.id}>
                          {org.name} ({org.id})
                        </option>
                      ))}
                    </NativeSelect>
                  </LabeledField>

                  <LabeledField label={t('org.sidebar.locale')}>
                    <Input
                      placeholder="zh-CN"
                      value={draft.default_locale ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...prev, default_locale: e.target.value }))}
                    />
                  </LabeledField>

                  <LabeledField label={t('org.sidebar.tz')}>
                    <Input
                      placeholder="Asia/Shanghai"
                      value={draft.default_timezone ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...prev, default_timezone: e.target.value }))}
                    />
                  </LabeledField>
                </div>

                <LabeledField label={t('org.detail.description')}>
                  <Textarea
                    rows={2}
                    placeholder="Brief description..."
                    value={draft.description ?? ''}
                    onChange={(e) => setDraft((prev) => ({ ...prev, description: e.target.value }))}
                  />
                </LabeledField>

                {!isCreating && selected ? (
                  <TimestampGrid createdAt={selected.created_at} updatedAt={selected.updated_at} />
                ) : null}
              </DetailBody>
            </RegistryDetail>
          ) : (
            <EmptyDetailState
              title={t('org.detail.emptyTitle')}
              description={t('org.detail.emptyDesc')}
            />
          )
        }
      />
    </div>
  )
}

const WorkspacesTab: React.FC = () => {
  const { t } = useI18n()
  const [items, setItems] = useState<WorkspaceItem[]>([])
  const [tenants, setTenants] = useState<TenantItem[]>([])
  const [filterTenant, setFilterTenant] = useState('')
  const [selected, setSelected] = useState<WorkspaceItem | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [draft, setDraft] = useState<Partial<WorkspaceItem>>({})
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState('')
  const [ok, setOk] = useState('')

  const load = useCallback(async () => {
    try {
      const [workspacesResponse, tenantsResponse] = await Promise.all([
        listWorkspaces(filterTenant || undefined),
        listTenants(),
      ])
      setItems(workspacesResponse.items ?? [])
      setTenants(tenantsResponse.items ?? [])
    } catch {
      setItems([])
    }
  }, [filterTenant])

  useEffect(() => {
    const doLoad = async () => {
      await load()
    }

    void doLoad()
  }, [load])

  const save = async () => {
    setSaving(true)
    setErr('')
    setOk('')

    try {
      if (isCreating) {
        if (!draft.id || !draft.name) {
          setErr(t('org.error.idNameRequired'))
          setSaving(false)
          return
        }

        const created = await createWorkspace(draft)
        setItems((prev) => [...prev, created])
        setSelected(created)
        setIsCreating(false)
        setOk(t('org.success.created'))
      } else if (selected) {
        const updated = await updateWorkspace(selected.id, draft)
        setItems((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
        setSelected(updated)
        setDraft({ ...updated })
        setOk(t('org.success.saved'))
      }
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : t('error.default'))
    }

    setSaving(false)
  }

  const toggleStatus = async (item: WorkspaceItem) => {
    try {
      const updated = await setWorkspaceStatus(item.id, item.status !== 'active')
      setItems((prev) => prev.map((workspace) => (workspace.id === updated.id ? updated : workspace)))
      if (selected?.id === updated.id) {
        setSelected(updated)
        setDraft({ ...updated })
      }
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : t('status.error'))
    }
  }

  const activeCount = items.filter((item) => item.status === 'active' || !item.status).length

  return (
    <div className={PAGE_STACK_CLASS}>
      <SummaryGrid>
        <StatCard
          title={t('org.stats.total')}
          value={String(items.length)}
          subtitle={filterTenant ? t('org.stats.filteredWorkspaces', 'filtered workspaces') : t('org.stats.allWorkspaces', 'all workspaces')}
        />
        <StatCard title={t('org.stats.active')} value={String(activeCount)} subtitle={t('status.enabled')} />
        <StatCard title={t('org.tabs.tenants')} value={String(tenants.length)} subtitle={t('org.stats.available')} />
      </SummaryGrid>

      <OrgStatusMsg ok={ok} error={err} />

      <SplitLayout
        sidebar={
          <RegistrySidebar>
            <SidebarBody>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 className="text-lg font-semibold tracking-tight text-foreground">{t('org.tabs.workspaces')}</h2>
                <Button
                  id="ws-new-btn"
                  variant="amber"
                  type="button"
                  onClick={() => {
                    setIsCreating(true)
                    setSelected(null)
                    setDraft({ tenant_id: filterTenant || undefined })
                    setErr('')
                    setOk('')
                  }}
                >
                  {t('org.sidebar.new')}
                </Button>
              </div>

              <NativeSelect
                value={filterTenant}
                onChange={(e) => setFilterTenant(e.target.value)}
                className={SELECT_CLASS}
              >
                <option value="">{t('org.stats.allTenants')}</option>
                {tenants.map((tenant) => (
                  <option key={tenant.id} value={tenant.id}>
                    {tenant.name}
                  </option>
                ))}
              </NativeSelect>

              <RegistryPanel title={t('org.sidebar.directory')} emptyText={t('org.sidebar.emptyWorkspaces')}>
                {items.map((item) => {
                  const isActive = item.status === 'active' || !item.status

                  return (
                    <div
                      key={item.id}
                      className="cursor-pointer rounded-2xl"
                      onClick={() => {
                        if (!isCreating) {
                          setSelected(item)
                          setDraft({ ...item })
                          setErr('')
                          setOk('')
                        }
                      }}
                    >
                      <RegistryCard
                        active={selected?.id === item.id}
                        title={item.name}
                        subtitle={`${item.id}${item.tenant_id ? ` · ${item.tenant_id}` : ''}`}
                        lines={[item.org_id ? `${t('org.sidebar.org')}${item.org_id}` : ''].filter(Boolean)}
                        status={<ActiveBadge active={isActive} label={item.status || 'active'} />}
                        action={
                          <ToggleStatusButton
                            active={isActive}
                            onClick={(event) => {
                              event.stopPropagation()
                              void toggleStatus(item)
                            }}
                          />
                        }
                      />
                    </div>
                  )
                })}
              </RegistryPanel>
            </SidebarBody>
          </RegistrySidebar>
        }
        detail={
          selected || isCreating ? (
            <RegistryDetail>
              <DetailBody>
                <DetailHeader
                  title={isCreating ? t('org.detail.newWorkspace') : selected!.name}
                  subtitle={isCreating ? t('org.detail.newWorkspaceDesc') : `ID: ${selected!.id}`}
                  status={
                    !isCreating && selected ? (
                      <ActiveBadge
                        active={!selected.status || selected.status === 'active'}
                        label={selected.status || 'active'}
                      />
                    ) : undefined
                  }
                  actions={
                    <>
                      <Button id="ws-save-btn" variant="amber" onClick={save} disabled={saving}>
                        <Save />
                        {saving ? t('org.detail.saving') : t('org.detail.save')}
                      </Button>
                      {isCreating ? (
                        <Button variant="outline" onClick={() => setIsCreating(false)}>
                          {t('org.detail.cancel')}
                        </Button>
                      ) : null}
                    </>
                  }
                />

                <div className={FORM_GRID_CLASS}>
                  <LabeledField label="ID" required={isCreating}>
                    <Input
                      placeholder="acme-prod-default"
                      value={draft.id ?? ''}
                      readOnly={!isCreating}
                      className={cn(!isCreating && READ_ONLY_CLASS)}
                      onChange={(e) => setDraft((prev) => ({ ...prev, id: e.target.value }))}
                    />
                  </LabeledField>

                  <LabeledField label={t('org.detail.name')} required>
                    <Input
                      placeholder="Production"
                      value={draft.name ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...prev, name: e.target.value }))}
                    />
                  </LabeledField>

                  <LabeledField label={t('org.detail.slug')}>
                    <Input
                      placeholder="production"
                      value={draft.slug ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...prev, slug: e.target.value }))}
                    />
                  </LabeledField>

                  <LabeledField label={t('org.tabs.tenants')}>
                    <NativeSelect
                      value={draft.tenant_id ?? ''}
                      disabled={!isCreating}
                      className={cn(SELECT_CLASS, !isCreating && READ_ONLY_CLASS)}
                      onChange={(e) => setDraft((prev) => ({ ...prev, tenant_id: e.target.value }))}
                    >
                       <option value="">{t('org.detail.selectTenant')}</option>
                      {tenants.map((tenant) => (
                        <option key={tenant.id} value={tenant.id}>
                          {tenant.name} ({tenant.id})
                        </option>
                      ))}
                    </NativeSelect>
                  </LabeledField>
                </div>

                <LabeledField label={t('org.detail.description')}>
                  <Textarea
                    rows={2}
                    placeholder="Brief description..."
                    value={draft.description ?? ''}
                    onChange={(e) => setDraft((prev) => ({ ...prev, description: e.target.value }))}
                  />
                </LabeledField>

                {!isCreating && selected ? (
                  <TimestampGrid createdAt={selected.created_at} updatedAt={selected.updated_at} />
                ) : null}
              </DetailBody>
            </RegistryDetail>
          ) : (
            <EmptyDetailState
              title={t('org.detail.emptyTitle')}
              description={t('org.detail.emptyDesc')}
            />
          )
        }
      />
    </div>
  )
}

const PolicyTab: React.FC = () => {
  const { t } = useI18n()
  const [orgs, setOrgs] = useState<OrgItem[]>([])
  const [tenants, setTenants] = useState<TenantItem[]>([])
  const [selOrg, setSelOrg] = useState('')
  const [selTenant, setSelTenant] = useState('')
  const [orgPolicy, setOrgPolicyState] = useState<OrgPolicy | null>(null)
  const [tenantPolicy, setTenantPolicyState] = useState<TenantPolicy | null>(null)
  const [resolved, setResolved] = useState<ResolvedPolicy | null>(null)
  const [editing, setEditing] = useState(false)
  const [editingTenant, setEditingTenant] = useState(false)
  const [draft, setDraft] = useState<OrgPolicy>({})
  const [tenantDraft, setTenantDraft] = useState<TenantPolicy | null>(null)
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState('')
  const [ok, setOk] = useState('')

  useEffect(() => {
    Promise.all([listOrganizations(), listTenants()])
      .then(([orgsResponse, tenantsResponse]) => {
        const nextOrgs = orgsResponse.items ?? []
        const nextTenants = tenantsResponse.items ?? []
        setOrgs(nextOrgs)
        setTenants(nextTenants)
        if (nextOrgs.length) setSelOrg(nextOrgs[0].id)
        if (nextTenants.length) setSelTenant(nextTenants[0].id)
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (!selOrg) return
    getOrgPolicy(selOrg)
      .then((policy) => {
        setOrgPolicyState(policy)
        setDraft({ ...policy })
      })
      .catch(() => {})
  }, [selOrg])

  useEffect(() => {
    if (!selTenant) return
    getTenantPolicy(selTenant)
      .then((policy) => {
        setTenantPolicyState(policy)
        setTenantDraft({ ...policy })
      })
      .catch(() => {})
  }, [selTenant])

  useEffect(() => {
    if (!selOrg || !selTenant) return
    resolvePolicy(selOrg, selTenant)
      .then((policy) => setResolved(policy))
      .catch(() => {})
  }, [selOrg, selTenant])

  const savePolicy = async () => {
    setSaving(true)
    setErr('')
    setOk('')

    try {
      const updated = await setOrgPolicy({ ...draft, org_id: selOrg })
      setOrgPolicyState(updated)
      setDraft({ ...updated })
      const nextResolved = await resolvePolicy(selOrg, selTenant)
      setResolved(nextResolved)
      setEditing(false)
      setOk(t('org.success.saved'))
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : t('error.default'))
    }

    setSaving(false)
  }

  const saveTenantPolicy = async () => {
    if (!tenantDraft) return

    setSaving(true)
    setErr('')
    setOk('')

    try {
      const updated = await setTenantPolicy(selTenant, {
        ...tenantDraft,
        tenant_id: selTenant,
        org_id: tenantDraft.org_id || selOrg,
      })
      setTenantPolicyState(updated)
      setTenantDraft({ ...updated })
      const nextResolved = await resolvePolicy(selOrg, selTenant)
      setResolved(nextResolved)
      setEditingTenant(false)
      setOk(t('org.success.saved'))
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : t('error.default'))
    }

    setSaving(false)
  }

  const activePolicy = editing ? draft : (orgPolicy ?? {})

  return (
    <div className={PAGE_STACK_CLASS}>
      <OrgStatusMsg ok={ok} error={err} />

      <div className="grid gap-6 xl:grid-cols-2 xl:items-start">
        <PolicyCard>
          <DetailHeader
            title={t('org.policy.effective')}
            subtitle={t('org.policy.resolvedDesc')}
            actions={
              editing ? (
                <>
                  <Button id="org-policy-save-btn" variant="amber" onClick={savePolicy} disabled={saving}>
                    <Save />
                    {saving ? t('org.detail.saving') : t('org.detail.save')}
                  </Button>
                  <Button
                    variant="outline"
                    onClick={() => {
                      setEditing(false)
                      setDraft({ ...(orgPolicy ?? {}) })
                    }}
                  >
                    {t('org.detail.cancel')}
                  </Button>
                </>
              ) : (
                  <Button id="org-policy-edit-btn" variant="outline" onClick={() => setEditing(true)}>
                    <Edit2 />
                    {t('org.policy.edit')}
                  </Button>
              )
            }
          />

          <LabeledField label={t('org.sidebar.org')}>
            <NativeSelect
              value={selOrg}
              className={SELECT_CLASS}
              onChange={(e) => {
                setSelOrg(e.target.value)
                setEditing(false)
              }}
            >
              {orgs.map((org) => (
                <option key={org.id} value={org.id}>
                  {org.name}
                </option>
              ))}
            </NativeSelect>
          </LabeledField>

          {orgPolicy ? (
            <div className="flex flex-col gap-4">
              <div className="rounded-2xl border border-border/70 px-4">
                <BoolRow
                  label={t('org.policy.requireMfa')}
                  value={!!activePolicy.require_mfa}
                  editing={editing}
                  onChange={(value) => setDraft((prev) => ({ ...prev, require_mfa: value }))}
                />
                <BoolRow
                  label={t('org.policy.requireApproval')}
                  value={!!activePolicy.require_approval_for_execution}
                  editing={editing}
                  onChange={(value) =>
                    setDraft((prev) => ({ ...prev, require_approval_for_execution: value }))
                  }
                />
                <BoolRow
                  label={t('org.policy.prohibitSelfApproval')}
                  value={!!activePolicy.prohibit_self_approval}
                  editing={editing}
                  onChange={(value) => setDraft((prev) => ({ ...prev, prohibit_self_approval: value }))}
                />
              </div>

              <div className="flex flex-col gap-4">
                <ListRow
                  label={t('org.policy.defaultJitRoles')}
                  value={activePolicy.default_jit_roles ?? []}
                  editing={editing}
                  onChange={(value) => setDraft((prev) => ({ ...prev, default_jit_roles: value }))}
                />
                <ListRow
                  label={t('org.policy.allowedAuthMethods')}
                  value={activePolicy.allowed_auth_methods ?? []}
                  editing={editing}
                  onChange={(value) => setDraft((prev) => ({ ...prev, allowed_auth_methods: value }))}
                />
                <ListRow
                  label={t('org.policy.skillAllowlist')}
                  value={activePolicy.skill_allowlist ?? []}
                  editing={editing}
                  onChange={(value) => setDraft((prev) => ({ ...prev, skill_allowlist: value }))}
                />
                <ListRow
                  label={t('org.policy.skillBlocklist')}
                  value={activePolicy.skill_blocklist ?? []}
                  editing={editing}
                  onChange={(value) => setDraft((prev) => ({ ...prev, skill_blocklist: value }))}
                />
              </div>

              {orgPolicy.updated_at && orgPolicy.updated_at !== '0001-01-01T00:00:00Z' ? (
                <div className="text-xs text-muted-foreground">{t('org.policy.updatedAt', 'Updated:')} {fmtDate(orgPolicy.updated_at)}</div>
              ) : null}
            </div>
          ) : null}
        </PolicyCard>

        <PolicyCard>
          <DetailHeader
            title={t('org.tabs.tenants') + ' ' + t('org.tabs.policy')}
            subtitle={t('org.policy.resolvedDesc')}
            actions={
              editingTenant ? (
                <>
                  <Button variant="amber" onClick={saveTenantPolicy} disabled={saving}>
                    <Save />
                    {saving ? t('org.detail.saving') : t('org.detail.save')}
                  </Button>
                  <Button
                    variant="outline"
                    onClick={() => {
                      setEditingTenant(false)
                      setTenantDraft(tenantPolicy ? { ...tenantPolicy } : null)
                    }}
                  >
                    {t('org.detail.cancel')}
                  </Button>
                </>
              ) : (
                <Button variant="outline" onClick={() => setEditingTenant(true)} disabled={!tenantDraft}>
                  <Edit2 />
                  {t('org.policy.edit')}
                </Button>
              )
            }
          />

          <LabeledField label={t('org.tabs.tenants')}>
            <NativeSelect value={selTenant} className={SELECT_CLASS} onChange={(e) => setSelTenant(e.target.value)}>
              {tenants.map((tenant) => (
                <option key={tenant.id} value={tenant.id}>
                  {tenant.name}
                </option>
              ))}
            </NativeSelect>
          </LabeledField>

          {tenantDraft ? (
            <div className="flex flex-col gap-5">
              <div className="flex flex-col gap-3">
                <div className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">
                   {t('org.policy.raw')}
                </div>
                <div className="rounded-2xl border border-border/70 px-4">
                  <BoolRow
                    label={t('org.policy.requireMfa')}
                    value={!!tenantDraft.require_mfa}
                    editing={editingTenant}
                    onChange={(value) =>
                      setTenantDraft((prev) => (prev ? { ...prev, require_mfa: value } : prev))
                    }
                  />
                  <BoolRow
                    label={t('org.policy.requireApproval')}
                    value={!!tenantDraft.require_approval_for_execution}
                    editing={editingTenant}
                    onChange={(value) =>
                      setTenantDraft((prev) =>
                        prev ? { ...prev, require_approval_for_execution: value } : prev
                      )
                    }
                  />
                  <BoolRow
                    label={t('org.policy.prohibitSelfApproval')}
                    value={!!tenantDraft.prohibit_self_approval}
                    editing={editingTenant}
                    onChange={(value) =>
                      setTenantDraft((prev) => (prev ? { ...prev, prohibit_self_approval: value } : prev))
                    }
                  />
                </div>

                <div className="flex flex-col gap-4">
                  <ListRow
                    label={t('org.policy.defaultJitRoles')}
                    value={tenantDraft.default_jit_roles ?? []}
                    editing={editingTenant}
                    onChange={(value) =>
                      setTenantDraft((prev) => (prev ? { ...prev, default_jit_roles: value } : prev))
                    }
                  />
                  <ListRow
                    label={t('org.policy.allowedAuthMethods')}
                    value={tenantDraft.allowed_auth_methods ?? []}
                    editing={editingTenant}
                    onChange={(value) =>
                      setTenantDraft((prev) => (prev ? { ...prev, allowed_auth_methods: value } : prev))
                    }
                  />
                  <ListRow
                    label={t('org.policy.skillAllowlist')}
                    value={tenantDraft.skill_allowlist ?? []}
                    editing={editingTenant}
                    onChange={(value) =>
                      setTenantDraft((prev) => (prev ? { ...prev, skill_allowlist: value } : prev))
                    }
                  />
                  <ListRow
                    label={t('org.policy.skillBlocklist')}
                    value={tenantDraft.skill_blocklist ?? []}
                    editing={editingTenant}
                    onChange={(value) =>
                      setTenantDraft((prev) => (prev ? { ...prev, skill_blocklist: value } : prev))
                    }
                  />
                </div>
              </div>

              {resolved ? (
                <div className="flex flex-col gap-3">
                    <div className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">
                     {t('org.policy.effective')}
                    </div>

                  <div className="rounded-2xl border border-border/70 px-4">
                    {([
                      [t('org.policy.requireMfa'), 'require_mfa'],
                      [t('org.policy.requireApproval'), 'require_approval_for_execution'],
                      [t('org.policy.prohibitSelfApproval'), 'prohibit_self_approval'],
                    ] as [string, keyof ResolvedPolicy][]).map(([label, field]) => (
                      <div
                        key={field}
                        className="flex items-center justify-between gap-4 border-b border-border/50 py-3 last:border-b-0"
                      >
                        <span className="text-sm text-muted-foreground">{label}</span>
                        {resolved[field] ? (
                          <Check className="size-4 text-success" />
                        ) : (
                          <X className="size-4 text-muted-foreground" />
                        )}
                      </div>
                    ))}
                  </div>

                  <div className="flex flex-col gap-4">
                    {(
                      ['default_jit_roles', 'skill_allowlist', 'skill_blocklist', 'allowed_auth_methods'] as Array<
                        keyof ResolvedPolicy
                      >
                    ).map((field) => {
                      const value = resolved[field] as string[] | undefined

                      return (
                        <div key={field} className="flex flex-col gap-2 pb-3 last:pb-0">
                    <div className="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">
                             {t(`org.policy.${field}`, formatPolicyLabel(String(field)))}
                          </div>
                          {value?.length ? (
                            <div className="flex flex-wrap gap-2">
                              {value.map((item) => (
                                <Badge
                                  key={item}
                                  variant="outline"
                                  className="rounded-full px-3 py-1 text-muted-foreground"
                                >
                                  {item}
                                </Badge>
                              ))}
                            </div>
                          ) : (
                            <span className="text-sm text-muted-foreground">-</span>
                          )}
                        </div>
                      )
                    })}
                  </div>
                </div>
              ) : null}

              {tenantPolicy?.updated_at ? (
                <div className="text-xs text-muted-foreground">
                  {t('org.policy.tenantUpdatedAt', 'Tenant policy updated:')} {fmtDate(tenantPolicy.updated_at)}
                </div>
              ) : null}
            </div>
          ) : (
            <div className="text-sm text-muted-foreground">{t('org.policy.selectScope', 'Select an org + tenant to see resolved policy.')}</div>
          )}
        </PolicyCard>
      </div>
    </div>
  )
}

export const OrgPage: React.FC = () => {
  const [tab, setTab] = useState<TabId>('organizations')
  const { t } = useI18n()

  return (
    <div className="animate-fade-in grid gap-4">
      <SectionTitle
        title={t('org.title')}
        subtitle={t('org.subtitle')}
      />

      <TabBar active={tab} onChange={setTab} />

      <TabPanel id="organizations" active={tab === 'organizations'}>
        <OrgsTab />
      </TabPanel>
      <TabPanel id="tenants" active={tab === 'tenants'}>
        <TenantsTab />
      </TabPanel>
      <TabPanel id="workspaces" active={tab === 'workspaces'}>
        <WorkspacesTab />
      </TabPanel>
      <TabPanel id="policy" active={tab === 'policy'}>
        <PolicyTab />
      </TabPanel>
    </div>
  )
}

export default OrgPage
