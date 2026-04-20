import { useState, useEffect, useMemo } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { Command } from 'cmdk'
import {
  ClipboardCheck,
  Command as CommandIcon,
  FileSearch,
  FileText,
  Globe,
  MessageCircle,
  Monitor,
  Moon,
  Search as SearchIcon,
  Sparkles,
  Sun,
} from 'lucide-react'
import { useI18n } from '../../hooks/useI18n'
import { useTheme } from '../../hooks/useTheme'
import { COMMAND_HUB_OPEN_EVENT, COMMAND_HUB_TOGGLE_EVENT } from './commandHubEvents'
import { useNavigationRoutes } from './navigation'

// Import all docs for searching
const allDocs = import.meta.glob('../../../../docs/**/*.md', { query: '?raw', import: 'default', eager: true })

// Command Palette styles are defined in index.css under [cmdk-*] selectors

export const GlobalSearch = () => {
  const navigate = useNavigate()
  const { lang, setLang, t } = useI18n()
  const { theme, setTheme } = useTheme()
  const navRoutes = useNavigationRoutes()

  const [open, setOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [oramaDb, setOramaDb] = useState<any>(null)
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [docResults, setDocResults] = useState<any[]>([])

  // Toggle the menu when ⌘K is pressed or close on ESC
  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        setOpen((current) => !current)
      }
      if (e.key === 'Escape' && open) {
        setOpen(false)
      }
    }
    document.addEventListener('keydown', down)
    return () => document.removeEventListener('keydown', down)
  }, [open])

  useEffect(() => {
    const handleOpen = () => setOpen(true)
    const handleToggle = () => setOpen((current) => !current)
    window.addEventListener(COMMAND_HUB_OPEN_EVENT, handleOpen)
    window.addEventListener(COMMAND_HUB_TOGGLE_EVENT, handleToggle)
    return () => {
      window.removeEventListener(COMMAND_HUB_OPEN_EVENT, handleOpen)
      window.removeEventListener(COMMAND_HUB_TOGGLE_EVENT, handleToggle)
    }
  }, [])

  // Initialize Search Engine
  useEffect(() => {
    const initSearch = async () => {
        const { create, insert } = await import('@orama/orama')
        const db = await create({
          schema: { id: 'string', title: 'string', category: 'string', content: 'string' }
        })

      const coreDocs = [
        { id: 'user-guide', path: '../../../../docs/guides/user-guide', title: 'User Guide' },
        { id: 'admin-guide', path: '../../../../docs/guides/admin-guide', title: 'Admin Guide' },
        { id: 'troubleshooting', path: '../../../../docs/guides/troubleshooting', title: 'Troubleshooting' },
        { id: 'deployment-guide', path: '../../../../docs/guides/deployment-guide', title: 'Deployment Guide' },
        ]

        for (const d of coreDocs) {
          const suffix = lang === 'zh-CN' ? '.zh-CN.md' : '.md'
          const content = (allDocs[`${d.path}${suffix}`] as string) || ''
          await insert(db, { id: d.id, title: d.title, category: 'Documentation', content: content.slice(0, 5000) })
        }
        setOramaDb(db)
      }
    if (open && !oramaDb) void initSearch()
  }, [open, oramaDb, lang])

  // Execute Doc Search
  useEffect(() => {
    if (!oramaDb || !searchQuery.trim()) {
      setDocResults([])
      return
    }
    const doSearch = async () => {
      const { search: oramaSearch } = await import('@orama/orama')
      try {
        const results = await oramaSearch(oramaDb, {
          term: searchQuery,
          properties: ['title', 'content'],
        })
        setDocResults(results.hits.slice(0, 3))
      } catch {
        setDocResults([])
      }
    }
    const timer = setTimeout(() => void doSearch(), 150)
    return () => clearTimeout(timer)
  }, [searchQuery, oramaDb])

  const navigationItems = useMemo(() => navRoutes.map((item) => {
    const Icon = item.icon
    return {
      ...item,
      title: t(item.labelKey as never),
      group: t('command.group.navigation'),
      icon: <Icon size={18} />,
    }
  }), [navRoutes, t])

  const actionItems = useMemo(() => {
    const group = t('command.group.actions')
    return [
      {
        id: 'review-incidents',
        title: lang === 'zh-CN' ? '查看待处理诊断' : 'Review active incidents',
        keywords: 'incident sessions diagnosis triage 告警 会话 诊断',
        group,
        icon: <Sparkles size={18} />,
        action: () => navigate('/sessions'),
      },
      {
        id: 'review-approvals',
        title: lang === 'zh-CN' ? '查看审批与执行' : 'Review approvals and runs',
        keywords: 'executions approvals runs 审批 执行',
        group,
        icon: <ClipboardCheck size={18} />,
        action: () => navigate('/executions'),
      },
      {
        id: 'open-inbox',
        title: t('header.openInbox'),
        keywords: 'inbox notifications 站内信 通知',
        group,
        icon: <FileText size={18} />,
        action: () => navigate('/inbox'),
      },
      {
        id: 'open-chat',
        title: t('header.openChat'),
        keywords: 'chat web chat 对话',
        group,
        icon: <MessageCircle size={18} />,
        action: () => navigate('/chat'),
      },
    ]
  }, [lang, navigate, t])

  const runCommand = (action: () => void) => {
    action()
    setOpen(false)
    setSearchQuery('')
  }

  /**
   * Robust multi-language matching logic
   */
  const customFilter = (value: string, search: string) => {
    const s = search.toLowerCase().trim()
    if (!s) return 1
    
    // For doc results, we trust Orama and always show them
    if (value.startsWith('doc-')) return 1
    
    // For navigation items, perform simple inclusion check on the token
    if (value.includes(s)) return 1

    return 0
  }

  return (
    <>
      <div className="relative hidden lg:flex justify-center w-[400px]">
        <button 
          onClick={() => setOpen(true)}
          className="w-full flex items-center justify-between gap-2 px-4 py-2 rounded-xl transition-all duration-300 bg-white/5 border border-white/5 hover:bg-white/10 group"
        >
          <div className="flex items-center gap-3">
            <SearchIcon size={16} className="text-text-muted group-hover:text-primary transition-colors" />
            <span className="text-sm text-text-muted group-hover:text-text-secondary transition-colors">
              {t('command.searchPlaceholder')}
            </span>
          </div>
          <div className="flex items-center gap-1 px-1.5 py-0.5 rounded bg-white/5 border border-white/10 text-[0.6rem] font-bold text-text-muted">
            <CommandIcon size={10} /> K
          </div>
        </button>

        {open && createPortal(
          <div className="fixed inset-0 z-[10000] flex items-start justify-center pt-[15vh] p-4 animate-in fade-in duration-200">
            <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setOpen(false)} />
            
            <div 
              className="relative w-full max-w-2xl glass-panel shadow-2xl overflow-hidden animate-in slide-in-from-top-4 duration-300 border-primary/20"
              onKeyDown={(e) => { if (e.key === 'Escape') { e.stopPropagation(); setOpen(false) } }}
            >
              <Command label="Global Command Palette" filter={customFilter}>
                <Command.Input 
                  autoFocus 
                  value={searchQuery}
                  onValueChange={setSearchQuery}
                  placeholder={t('command.searchPlaceholder')}
                />
                
                <Command.List>
                  <Command.Empty>
                    {t('command.empty')}
                  </Command.Empty>

                  <Command.Group heading={t('command.group.actions')}>
                    {actionItems.map(item => (
                      <Command.Item
                        key={item.id}
                        value={`${item.id} ${item.title} ${item.keywords}`.toLowerCase()}
                        onSelect={() => runCommand(item.action)}
                      >
                        <div cmdk-item-icon>{item.icon}</div>
                        <div className="flex-1 font-bold">{item.title}</div>
                        <div className="text-[0.6rem] px-1.5 py-0.5 rounded bg-white/5 text-text-muted font-black tracking-widest uppercase">
                          {item.group}
                        </div>
                      </Command.Item>
                    ))}
                  </Command.Group>

                  <Command.Group heading={t('command.group.navigation')}>
                    {navigationItems.map(item => (
                      <Command.Item 
                        key={item.id} 
                        value={`${item.id} ${item.title} ${item.keywords}`.toLowerCase()}
                        onSelect={() => runCommand(() => navigate(item.path))}
                      >
                        <div cmdk-item-icon>{item.icon}</div>
                        <div className="flex-1 font-bold">{item.title}</div>
                        <div className="text-[0.6rem] px-1.5 py-0.5 rounded bg-white/5 text-text-muted font-black tracking-widest uppercase">
                          {item.group}
                        </div>
                      </Command.Item>
                    ))}
                  </Command.Group>

                  {docResults.length > 0 && (
                    <Command.Group heading={lang === 'zh-CN' ? '文档匹配' : 'Documentation Hits'}>
                      {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                      {docResults.map((hit: any) => (
                        <Command.Item 
                          key={hit.id} 
                          value={`doc-${hit.id}`.toLowerCase()}
                          onSelect={() => runCommand(() => navigate(`/docs/${hit.document.id}`))}
                        >
                          <div cmdk-item-icon><FileSearchIcon /></div>
                          <div className="flex flex-col gap-0.5">
                            <div className="font-bold">{hit.document.title}</div>
                            <div className="text-[0.7rem] text-text-muted line-clamp-1 italic">{hit.document.content.slice(0, 100)}...</div>
                          </div>
                        </Command.Item>
                      ))}
                    </Command.Group>
                  )}

                  <Command.Group heading={t('command.group.preferences')}>
                    <Command.Item value="theme-light 切换浅色模式" onSelect={() => runCommand(() => setTheme('light'))}>
                      <div cmdk-item-icon><Sun size={18} /></div>
                      <div className="flex-1">{t('header.theme.light')}</div>
                      {theme === 'light' && <div className="w-1.5 h-1.5 rounded-full bg-primary" />}
                    </Command.Item>
                    <Command.Item value="theme-dark 切换深色模式" onSelect={() => runCommand(() => setTheme('dark'))}>
                      <div cmdk-item-icon><Moon size={18} /></div>
                      <div className="flex-1">{t('header.theme.dark')}</div>
                      {theme === 'dark' && <div className="w-1.5 h-1.5 rounded-full bg-primary" />}
                    </Command.Item>
                    <Command.Item value="theme-system 跟随系统模式" onSelect={() => runCommand(() => setTheme('system'))}>
                      <div cmdk-item-icon><Monitor size={18} /></div>
                      <div className="flex-1">{t('header.theme.system')}</div>
                      {theme === 'system' && <div className="w-1.5 h-1.5 rounded-full bg-primary" />}
                    </Command.Item>
                    <Command.Item value="lang-en 切换英文" onSelect={() => runCommand(() => setLang('en-US'))}>
                      <div cmdk-item-icon><Globe size={18} /></div>
                      <div className="flex-1">English (US)</div>
                      {lang === 'en-US' && <div className="w-1.5 h-1.5 rounded-full bg-primary" />}
                    </Command.Item>
                    <Command.Item value="lang-zh 切换中文" onSelect={() => runCommand(() => setLang('zh-CN'))}>
                      <div cmdk-item-icon><Globe size={18} /></div>
                      <div className="flex-1">简体中文</div>
                      {lang === 'zh-CN' && <div className="w-1.5 h-1.5 rounded-full bg-primary" />}
                    </Command.Item>
                  </Command.Group>
                </Command.List>
                
                <div className="p-3 bg-white/[0.02] border-t border-white/5 flex items-center justify-between text-[0.65rem] text-text-muted">
                  <div className="flex gap-4">
                    <span className="flex items-center gap-1"><kbd className="bg-white/10 px-1 rounded">↑↓</kbd> {t('command.footerNavigate')}</span>
                    <span className="flex items-center gap-1"><kbd className="bg-white/10 px-1 rounded">Enter</kbd> {t('command.footerSelect')}</span>
                  </div>
                  <div className="flex items-center gap-1"><kbd className="bg-white/10 px-1 rounded">ESC</kbd> {t('command.footerClose')}</div>
                </div>
              </Command>
            </div>
          </div>,
          document.body
        )}
      </div>
    </>
  )
}

const FileSearchIcon = () => (
  <div className="relative">
    <FileText size={18} />
    <FileSearch size={8} className="absolute -bottom-0.5 -right-0.5 bg-primary text-bg-base rounded-full p-[1px]" />
  </div>
);
