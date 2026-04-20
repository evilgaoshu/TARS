import { useCallback, useEffect, useMemo, useState } from 'react'
import { Bell, CheckCheck, ExternalLink, Inbox as InboxIcon, RefreshCw } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state'
import { OperatorHero, OperatorSection, OperatorStats, OperatorStack } from '@/components/operator/OperatorPage'
import { ExecutionActionBar } from '@/components/operator/ExecutionActionBar'
import { useI18n } from '@/hooks/useI18n'
import { listInbox, markAllInboxRead, markInboxRead } from '@/lib/api/inbox'
import { fetchExecution } from '@/lib/api/ops'
import type { ExecutionDetail, InboxMessage } from '@/lib/api/types'
import { cn } from '@/lib/utils'

export function InboxPage() {
  const { lang, t } = useI18n()
  const [messages, setMessages] = useState<InboxMessage[]>([])
  const [unreadCount, setUnreadCount] = useState(0)
  const [loading, setLoading] = useState(true)
  const [showUnreadOnly, setShowUnreadOnly] = useState(false)

  const loadInbox = useCallback(async (unreadOnly = showUnreadOnly) => {
    setLoading(true)
    try {
      const response = await listInbox({ limit: 50, unread_only: unreadOnly || undefined })
      setMessages(response.items ?? [])
      setUnreadCount(response.unread_count ?? 0)
    } finally {
      setLoading(false)
    }
  }, [showUnreadOnly])

  useEffect(() => {
    void loadInbox(showUnreadOnly)
  }, [loadInbox, showUnreadOnly])

  const filtered = useMemo(() => showUnreadOnly ? messages.filter((item) => !item.is_read) : messages, [messages, showUnreadOnly])

  const handleMarkRead = async (message: InboxMessage) => {
    if (message.is_read) {
      return
    }
    await markInboxRead(message.id)
    setMessages((current) => current.map((item) => item.id === message.id ? { ...item, is_read: true } : item))
    setUnreadCount((current) => Math.max(0, current - 1))
  }

  const handleMarkAllRead = async () => {
    await markAllInboxRead()
    setMessages((current) => current.map((item) => ({ ...item, is_read: true })))
    setUnreadCount(0)
  }

  return (
    <div className="flex flex-col gap-6 pb-10">
      <OperatorHero
        eyebrow={t('inbox.hero.eyebrow', 'First-party delivery surface')}
        title={t('inbox.hero.title')}
        description={t('inbox.hero.description')}
        chips={[
          { label: `${unreadCount} ${t('common.status.unread', 'unread')}`, tone: unreadCount > 0 ? 'warning' : 'success' },
          { label: t('inbox.hero.chipReady', 'in_app_inbox ready'), tone: 'info' },
        ]}
        primaryAction={<Button variant="amber" onClick={() => void loadInbox(showUnreadOnly)}><RefreshCw size={14} className={cn(loading && 'animate-spin')} />{t('inbox.action.refresh', 'Refresh inbox')}</Button>}
        secondaryAction={<Button variant="outline" onClick={() => void handleMarkAllRead()} disabled={unreadCount === 0}>{t('inbox.action.markAllRead', 'Mark all read')}</Button>}
      />

      <OperatorStats
        stats={[
          {
            title: t('inbox.stats.unread', 'Unread'),
            value: unreadCount,
            description: t('inbox.stats.unreadDesc', 'Messages that still need operator attention.'),
            icon: Bell,
            tone: unreadCount > 0 ? 'warning' : 'success',
          },
          {
            title: t('inbox.stats.loaded', 'Loaded items'),
            value: filtered.length,
            description: t('inbox.stats.loadedDesc', 'Toggle between all notifications and unread only.'),
            icon: InboxIcon,
            tone: 'info',
          },
        ]}
      />

      <OperatorSection
        title={t('inbox.section.title', 'Message stream')}
        description={t('inbox.section.description', 'Prioritize actionable notifications tied to the incident response loop.')}
        action={
          <div className="flex items-center gap-2">
            <Button variant={showUnreadOnly ? 'amber' : 'outline'} size="sm" onClick={() => setShowUnreadOnly((current) => !current)}>
              {showUnreadOnly ? t('inbox.action.showAll', 'Show all') : t('inbox.action.unreadOnly', 'Unread only')}
            </Button>
          </div>
        }
      >
        {filtered.length === 0 ? (
          <EmptyState
            icon={InboxIcon}
            loading={loading}
            title={showUnreadOnly ? t('inbox.empty.unreadOnly') : t('inbox.empty.title')}
            description={showUnreadOnly ? t('inbox.empty.unreadOnlyDesc') : t('inbox.empty.description')}
            action={showUnreadOnly ? <Button variant="outline" onClick={() => setShowUnreadOnly(false)}>{t('inbox.action.viewAll')}</Button> : undefined}
          />
        ) : (
          <OperatorStack>
            {filtered.map((message) => (
              <Card key={message.id} className={cn('transition-colors', !message.is_read && 'border-primary/30 bg-primary/5')}>
                <CardHeader className="pb-3">
                  <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <CardTitle className="text-base">{message.subject}</CardTitle>
                        <Badge variant={message.is_read ? 'outline' : 'warning'}>{message.is_read ? t('common.status.read', 'Read') : t('common.status.unread', 'Unread')}</Badge>
                      </div>
                      <CardDescription>
                        {message.channel} · {message.source} · {formatDateTime(message.created_at, lang)}
                      </CardDescription>
                    </div>
                    <div className="flex items-center gap-2">
                      {!message.is_read ? (
                        <Button variant="outline" size="sm" onClick={() => void handleMarkRead(message)}>
                          <CheckCheck size={14} />
                          {t('inbox.action.markRead', 'Mark read')}
                        </Button>
                      ) : null}
                      {message.ref_id ? (
                        <Button variant="ghost" size="sm" asChild>
                          <Link to={resolveInboxTarget(message)}>
                            <ExternalLink size={14} />
                            {t('inbox.action.openRecord', 'Open record')}
                          </Link>
                        </Button>
                      ) : null}
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <p className="text-sm leading-6 text-muted-foreground">{message.body}</p>
                  {message.ref_type === 'execution' && message.ref_id ? (
                    <InboxExecutionAction refID={message.ref_id} onUpdated={() => void loadInbox(showUnreadOnly)} />
                  ) : null}
                </CardContent>
              </Card>
            ))}
          </OperatorStack>
        )}
      </OperatorSection>
    </div>
  )
}

function InboxExecutionAction({ refID, onUpdated }: { refID: string; onUpdated: () => void }) {
  const [execution, setExecution] = useState<ExecutionDetail | null>(null)

  useEffect(() => {
    let mounted = true
    void fetchExecution(refID).then((detail) => {
      if (mounted) {
        setExecution(detail)
      }
    }).catch(() => undefined)
    return () => {
      mounted = false
    }
  }, [refID])

  if (!execution || execution.status !== 'pending') {
    return null
  }

  return <ExecutionActionBar execution={execution} onUpdated={() => onUpdated()} className="mt-4" />
}

function resolveInboxTarget(message: InboxMessage): string {
  switch (message.ref_type) {
    case 'session':
      return `/sessions/${message.ref_id}`
    case 'execution':
      return `/executions/${message.ref_id}`
    default:
      return '/inbox'
  }
}

function formatDateTime(value: string, lang: 'en-US' | 'zh-CN'): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString(lang)
}
