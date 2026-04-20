import { useState, useRef, useEffect, useCallback } from 'react'
import { Send, MessageCircle, Loader2, ArrowRight, AlertCircle, CheckCircle2, Clock, RadioTower } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { OperatorHero, OperatorSection } from '@/components/operator/OperatorPage'
import { useI18n } from '@/hooks/useI18n'
import { sendChatMessage, listChatSessions } from '@/lib/api/chat'
import { fetchSession } from '@/lib/api/ops'
import { ExecutionActionBar } from '@/components/operator/ExecutionActionBar'
import type { ChatSessionSummary, ChatMessageResponse } from '@/lib/api/chat'
import type { ExecutionDetail } from '@/lib/api/types'
import { cn } from '@/lib/utils'

interface LocalMessage {
  id: string
  role: 'user' | 'ack'
  text: string
  sessionId?: string
  duplicated?: boolean
  ts: number
}

function statusBadge(status: string, t: (key: string) => string) {
  switch (status) {
    case 'resolved':
      return <Badge variant="success" className="rounded-full text-[0.62rem] font-black uppercase tracking-[0.14em]"><CheckCircle2 size={10} />{t('common.status.resolved')}</Badge>
    case 'failed':
      return <Badge variant="danger" className="rounded-full text-[0.62rem] font-black uppercase tracking-[0.14em]"><AlertCircle size={10} />{t('common.status.failed')}</Badge>
    default:
      return <Badge variant="warning" className="rounded-full text-[0.62rem] font-black uppercase tracking-[0.14em]"><Clock size={10} />{t('common.status.pending')}</Badge>
  }
}

export function ChatPage() {
  const { t } = useI18n()

  const [messages, setMessages] = useState<LocalMessage[]>([])
  const [input, setInput] = useState('')
  const [host, setHost] = useState('')
  const [service, setService] = useState('')
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState<string | null>(null)
  const [sessions, setSessions] = useState<ChatSessionSummary[]>([])
  const [pendingExecution, setPendingExecution] = useState<ExecutionDetail | null>(null)
  const [sessionsLoading, setSessionsLoading] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  useEffect(() => {
    scrollToBottom()
  }, [messages, scrollToBottom])

  const fetchSessions = useCallback(async () => {
    setSessionsLoading(true)
    try {
      const res = await listChatSessions()
      setSessions(res.items)
    } catch {
      // non-fatal
    } finally {
      setSessionsLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchSessions()
  }, [fetchSessions])

  const handleSend = useCallback(async () => {
    const text = input.trim()
    if (!text || sending) return

    const userMsg: LocalMessage = {
      id: `user-${Date.now()}`,
      role: 'user',
      text,
      ts: Date.now(),
    }
    setMessages(prev => [...prev, userMsg])
    setInput('')
    setSending(true)
    setSendError(null)

    let result: ChatMessageResponse
    try {
      result = await sendChatMessage({ message: text, host: host || undefined, service: service || undefined })
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      setSendError(msg)
      setSending(false)
      return
    }

    const ackMsg: LocalMessage = {
      id: `ack-${Date.now()}`,
      role: 'ack',
      text: result.ack_message,
      sessionId: result.session_id,
      duplicated: result.duplicated,
      ts: Date.now(),
    }
    setMessages(prev => [...prev, ackMsg])
    if (result.session_id) {
      void fetchSession(result.session_id).then((session) => {
        const nextPending = (session.executions || []).find((item) => item.status === 'pending') || null
        setPendingExecution(nextPending)
      }).catch(() => undefined)
    }
    setSending(false)
    fetchSessions()
  }, [input, host, service, sending, fetchSessions])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div className="flex flex-col gap-6 pb-10">
      <OperatorHero
        eyebrow={t('chat.hero.eyebrow')}
        title={t('chat.hero.title')}
        description={t('chat.hero.description')}
        chips={[
          { label: t('chat.hero.chipFirstParty'), tone: 'info' },
          { label: t('chat.hero.chipLive'), tone: 'success' },
        ]}
        secondaryAction={
          <Button variant="outline" asChild>
            <Link to="/channels"><RadioTower size={14} />{t('nav.channels')}</Link>
          </Button>
        }
      />

      <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
        {/* ── Left: chat panel ── */}
        <Card className="flex flex-col overflow-hidden">
          <CardHeader className="border-b border-border pb-4">
            <CardTitle className="flex items-center gap-2 text-base">
              <MessageCircle size={16} className="text-primary" />
              {t('chat.action.sendRequest')}
            </CardTitle>
            <CardDescription>
              {t('chat.action.sendRequestDesc')}
            </CardDescription>
          </CardHeader>

          <CardContent className="flex flex-1 flex-col gap-4 p-4">
            {/* optional context fields */}
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label className="text-xs text-muted-foreground">{t('chat.action.addHost')}</Label>
                <Input
                  placeholder="e.g. prod-web-01"
                  value={host}
                  onChange={e => setHost(e.target.value)}
                  className="h-8 text-sm"
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs text-muted-foreground">{t('chat.action.addService')}</Label>
                <Input
                  placeholder="e.g. nginx"
                  value={service}
                  onChange={e => setService(e.target.value)}
                  className="h-8 text-sm"
                />
              </div>
            </div>

            {/* message thread */}
            <div className="flex-1 overflow-y-auto space-y-3 min-h-[260px] max-h-[420px] py-1 pr-1">
              {messages.length === 0 && (
                <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
                  {t('chat.empty.title')}
                </div>
              )}
              {messages.map(msg => (
                <div
                  key={msg.id}
                  className={cn(
                    'flex flex-col gap-1',
                    msg.role === 'user' ? 'items-end' : 'items-start',
                  )}
                >
                  <div
                    className={cn(
                      'max-w-[85%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed',
                      msg.role === 'user'
                        ? 'bg-primary text-primary-foreground'
                        : msg.duplicated
                          ? 'border border-warning/20 bg-warning/10 text-warning'
                          : 'border border-border bg-card text-foreground',
                    )}
                  >
                    {msg.text}
                  </div>
                  {msg.sessionId && (
                    <Link
                      to={`/sessions/${msg.sessionId}`}
                      className="flex items-center gap-1 text-[0.7rem] text-muted-foreground hover:text-primary transition-colors"
                    >
                      <ArrowRight size={10} />
                      {t('chat.action.viewSession')} {msg.sessionId.slice(0, 8)}…
                    </Link>
                  )}
                </div>
              ))}
              {sending && (
                <div className="flex items-start">
                  <div className="rounded-2xl border border-border bg-card px-4 py-2.5 text-sm text-muted-foreground flex items-center gap-2">
                    <Loader2 size={12} className="animate-spin" />
                    {t('common.processing')}
                  </div>
                </div>
              )}
              <div ref={messagesEndRef} />
            </div>

            {/* error notice */}
            {sendError && (
              <div className="rounded-lg border border-danger/20 bg-danger/10 px-3 py-2 text-xs text-danger flex items-center gap-2">
                <AlertCircle size={12} />
                {sendError}
              </div>
            )}

            {pendingExecution ? (
              <ExecutionActionBar execution={pendingExecution} onUpdated={(next) => setPendingExecution(next.status === 'pending' ? next : null)} />
            ) : null}

            {/* input row */}
            <div className="flex gap-2 items-end">
              <Textarea
                placeholder={t('chat.action.inputPlaceholder')}
                value={input}
                onChange={e => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                rows={3}
                className="resize-none text-sm flex-1"
                disabled={sending}
              />
              <Button
                variant="amber"
                size="icon"
                onClick={handleSend}
                disabled={sending || !input.trim()}
                className="shrink-0 h-[72px] w-10"
              >
                {sending ? <Loader2 size={14} className="animate-spin" /> : <Send size={14} />}
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* ── Right: recent sessions ── */}
        <div className="flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <span className="text-sm font-semibold text-foreground">
              {t('chat.recent.title')}
            </span>
            <Button variant="ghost" size="sm" onClick={fetchSessions} disabled={sessionsLoading}>
              {sessionsLoading ? <Loader2 size={12} className="animate-spin" /> : t('common.refresh')}
            </Button>
          </div>

          {sessions.length === 0 && !sessionsLoading && (
            <div className="flex flex-col items-center justify-center rounded-xl border border-border bg-card p-8 text-center gap-2">
              <Clock size={24} className="text-muted-foreground/20" />
              <div className="space-y-1">
                <p className="text-sm font-medium text-foreground">{t('chat.recent.none')}</p>
                <p className="text-xs text-muted-foreground">{t('chat.recent.noneDesc')}</p>
              </div>
            </div>
          )}

          <div className="space-y-2">
            {sessions.map(s => (
              <Link key={s.session_id} to={`/sessions/${s.session_id}`} className="block">
                <Card className="glass-card-interactive p-0 transition-shadow hover:shadow-md">
                  <CardContent className="p-3 space-y-1.5">
                    <div className="flex items-center justify-between gap-2">
                      {statusBadge(s.status, t)}
                      <span className="text-[0.65rem] text-muted-foreground font-mono">{s.session_id.slice(0, 8)}…</span>
                    </div>
                    <p className="text-xs text-foreground leading-relaxed line-clamp-2">
                      {s.user_request || t('common.noSummary')}
                    </p>
                    {(s.host || s.service) && (
                      <div className="flex gap-1 flex-wrap">
                        {s.host && <Badge variant="outline" className="text-[0.6rem] px-1.5 py-0 rounded-full">{s.host}</Badge>}
                        {s.service && <Badge variant="outline" className="text-[0.6rem] px-1.5 py-0 rounded-full">{s.service}</Badge>}
                      </div>
                    )}
                  </CardContent>
                </Card>
              </Link>
            ))}
          </div>
        </div>
      </div>

      <OperatorSection
        title={t('chat.workflow.title')}
        description={t('chat.workflow.description')}
      >
        <div className="grid gap-4 lg:grid-cols-3">
          {[
            {
              title: t('chat.workflow.step1Title'),
              description: t('chat.workflow.step1Desc'),
            },
            {
              title: t('chat.workflow.step2Title'),
              description: t('chat.workflow.step2Desc'),
            },
            {
              title: t('chat.workflow.step3Title'),
              description: t('chat.workflow.step3Desc'),
            },
          ].map(item => (
            <Card key={item.title}>
              <CardHeader>
                <CardTitle className="text-sm">{item.title}</CardTitle>
                <CardDescription className="text-xs">{item.description}</CardDescription>
              </CardHeader>
            </Card>
          ))}
        </div>
      </OperatorSection>
    </div>
  )
}
