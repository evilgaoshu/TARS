import React, { useState, useEffect, useCallback } from 'react';
import { NavLink, Outlet, useLocation, Link } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import {
  TerminalSquare, 
  BookOpenText, 
  Layers3, 
  Inbox,
  Bell,
  Sun,
  Moon,
  Monitor,
  ChevronDown,
  FileText,
  Cpu,
  ExternalLink,
  Tag,
  LifeBuoy,
  MessageCircle,
  LogOut,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  Sparkles,
  ShieldCheck,
} from 'lucide-react';
import { useAuth } from '../../hooks/useAuth';
import { useTheme } from '../../hooks/useTheme';
import { useI18n } from '../../hooks/useI18n';
import { GlobalSearch } from './GlobalSearch';
import { Breadcrumbs } from './Breadcrumbs';
import { Button } from '../ui/button';
import { Badge } from '../ui/badge';
import { Separator } from '../ui/separator';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '../ui/sheet';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../ui/dropdown-menu';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '../ui/tooltip';
import { cn } from '../../lib/utils';
import { listInbox, markInboxRead, markAllInboxRead } from '../../lib/api/inbox';
import type { InboxMessage } from '../../lib/api/types';
import { openCommandHub } from './commandHubEvents';
import { useNavigationGroups, type NavGroupDefinition } from './navigation';
import { RiskBadge } from '../ui/shared-state';

/**
 * TARS Enterprise Application Layout
 * Features: Collapsible Sidebar, Mobile Sheet, Dynamic Header, Global Command Palette, i18n, Dark Mode
 */
export const AppLayout: React.FC = () => {
  return (
    <TooltipProvider delayDuration={300}>
      <AppLayoutContent />
    </TooltipProvider>
  );
};

const AppLayoutContent: React.FC = () => {
  const { user, logout } = useAuth();
  const { theme, setTheme } = useTheme();
  const { lang, setLang, t } = useI18n();
  const location = useLocation();
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [mobileOpen, setMobileOpen] = useState(false);
  const navGroups = useNavigationGroups();

  // Close mobile sidebar on navigation — use key reset approach
  const mobileSheetKey = location.pathname;

  const sidebarContent = (
      <SidebarNav
        sidebarOpen={sidebarOpen}
        user={user}
        logout={logout}
        t={t}
        navGroups={navGroups}
      />
  );

  return (
    <div className="flex min-h-screen bg-[var(--bg-base)] text-foreground">
      {/* ─── Desktop Sidebar ─────────────────────────────────── */}
      <aside
        className={cn(
          "hidden lg:flex flex-col shrink-0 border-r border-border bg-[color:var(--bg-surface-solid)]/96 backdrop-blur z-20",
          "transition-[width] duration-300 ease-in-out",
          sidebarOpen ? "w-[var(--sidebar-width)]" : "w-[var(--sidebar-rail-width)]"
        )}
      >
        {/* Logo */}
        <div className="flex h-[var(--header-height)] items-center border-b border-border px-5 shrink-0">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-xl border border-primary/20 bg-primary/12 text-primary shadow-[0_0_0_1px_rgba(50,199,189,0.08)]">
            <Layers3 size={18} className="shrink-0" />
          </div>
          {sidebarOpen && (
            <div className="ml-3 min-w-0">
              <div className="text-sm font-semibold uppercase tracking-[0.22em] text-foreground">{t('header.logo')}</div>
              <div className="text-[0.68rem] font-medium uppercase tracking-[0.16em] text-muted-foreground">
                On-call Evidence Desk
              </div>
            </div>
          )}
        </div>
        {sidebarContent}
      </aside>

      {/* ─── Mobile Sheet Sidebar ─────────────────────────────── */}
      <Sheet key={mobileSheetKey} open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent side="left" className="w-[280px] border-r border-border bg-[color:var(--bg-surface-solid)] p-0">
          <SheetHeader className="flex h-[var(--header-height)] flex-row items-center gap-3 border-b border-border px-5">
            <div className="flex size-10 items-center justify-center rounded-xl border border-primary/20 bg-primary/12 text-primary">
              <Layers3 size={18} />
            </div>
            <div className="min-w-0 text-left">
              <SheetTitle className="text-sm font-semibold uppercase tracking-[0.22em]">{t('header.logo')}</SheetTitle>
              <div className="text-[0.68rem] uppercase tracking-[0.16em] text-muted-foreground">On-call Evidence Desk</div>
            </div>
          </SheetHeader>
          <SidebarNav
            sidebarOpen={true}
            user={user}
            logout={logout}
            t={t}
            navGroups={navGroups}
          />
        </SheetContent>
      </Sheet>

      {/* ─── Main View ─────────────────────────────────────── */}
      <main className="flex-1 flex flex-col min-w-0 h-screen overflow-hidden">
        <header className="flex h-[var(--header-height)] items-center border-b border-border bg-[color:var(--bg-surface)] px-4 backdrop-blur-xl z-[100] shrink-0 lg:px-8">
          <div className="flex items-center gap-3 flex-1 min-w-0">
            {/* Mobile menu button */}
            <Button
              variant="ghost"
              size="icon"
              className="size-8 lg:hidden shrink-0"
              onClick={() => setMobileOpen(true)}
            >
              <Menu size={16} />
            </Button>
            {/* Desktop sidebar toggle */}
            <Button
              variant="ghost"
              size="icon"
              className="size-8 hidden lg:inline-flex shrink-0"
              onClick={() => setSidebarOpen((v) => !v)}
              title={t('header.toggleSidebar')}
            >
              {sidebarOpen ? <PanelLeftClose size={16} /> : <PanelLeftOpen size={16} />}
            </Button>
            <span className="hidden rounded-md border border-border bg-card px-2.5 py-1 font-mono text-xs text-muted-foreground sm:inline-block">
              {user?.authSource || 'unknown'}
            </span>
            {user?.breakGlass && (
              <RiskBadge risk="critical" label={t('header.breakGlass')} className="py-0.5" />
            )}
          </div>
          
          <GlobalSearch />
          
          <div className="flex-1 min-w-[120px] lg:min-w-[200px] flex gap-2 items-center justify-end">
            <Button
              variant="amber"
              size="sm"
              className="hidden md:inline-flex h-8 gap-2"
              onClick={openCommandHub}
            >
              <Sparkles size={14} />
              {t('header.actionHub')}
            </Button>

            <Button variant="outline" size="sm" className="hidden lg:inline-flex h-8 gap-1.5" asChild>
              <Link to="/chat">
                <MessageCircle size={14} />
                {t('header.openChat')}
              </Link>
            </Button>

            {/* Theme Toggle */}
            <div className="hidden rounded-lg border border-border bg-card p-0.5 sm:flex">
              <Button
                variant="ghost"
                size="icon"
                className={cn("size-7 rounded-md", theme === 'light' && 'bg-primary/12 text-primary')}
                title={t('header.theme.light')}
                onClick={() => setTheme('light')}
              >
                <Sun size={13} />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className={cn("size-7 rounded-md", theme === 'system' && 'bg-primary/12 text-primary')}
                title={t('header.theme.system')}
                onClick={() => setTheme('system')}
              >
                <Monitor size={13} />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className={cn("size-7 rounded-md", theme === 'dark' && 'bg-primary/12 text-primary')}
                title={t('header.theme.dark')}
                onClick={() => setTheme('dark')}
              >
                <Moon size={13} />
              </Button>
            </div>

            {/* Language Toggle */}
            <div className="hidden rounded-lg border border-border bg-card p-0.5 sm:flex">
              <Button
                variant="ghost"
                size="sm"
                className={cn("h-7 px-2 rounded-md text-xs font-bold", lang === 'en-US' && 'bg-primary/12 text-primary')}
                onClick={() => setLang('en-US')}
              >
                EN
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className={cn("h-7 px-2 rounded-md text-xs font-bold", lang === 'zh-CN' && 'bg-primary/12 text-primary')}
                onClick={() => setLang('zh-CN')}
              >
                中
              </Button>
            </div>

            <InboxDropdown />

            {/* Docs Center Dropdown */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-8 gap-1.5 hidden md:inline-flex"
                >
                  <BookOpenText size={14} />
                  <span className="hidden lg:inline">{t('header.docs')}</span>
                  <ChevronDown size={12} />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="max-h-[80vh] w-[300px] overflow-y-auto">
                <DropdownMenuLabel className="text-[0.65rem] font-black uppercase tracking-widest opacity-70">
                  {t('nav.group.overview')}
                </DropdownMenuLabel>
                <DropdownMenuItem asChild>
                    <Link to="/docs/user-guide" className="flex items-center gap-2.5">
                    <FileText size={14} className="text-info" />
                    {t('docs.userGuide')}
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                    <Link to="/docs/web-console-guide" className="flex items-center gap-2.5">
                    <Monitor size={14} className="text-primary" />
                    {t('docs.consoleGuide')}
                  </Link>
                </DropdownMenuItem>

                <DropdownMenuSeparator />
                <DropdownMenuLabel className="text-[0.65rem] font-black uppercase tracking-widest opacity-70">
                  {t('nav.group.system')}
                </DropdownMenuLabel>
                <DropdownMenuItem asChild>
                    <Link to="/docs/admin-guide" className="flex items-center gap-2.5">
                    <ShieldCheck size={14} className="text-warning" />
                    {t('docs.adminGuide')}
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                    <Link to="/docs/deployment-requirements" className="flex items-center gap-2.5">
                    <Cpu size={14} className="text-info" />
                    {t('docs.requirements')}
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                    <Link to="/docs/deployment-guide" className="flex items-center gap-2.5">
                    <ExternalLink size={14} className="text-success" />
                    {t('docs.deploymentGuide')}
                  </Link>
                </DropdownMenuItem>

                <DropdownMenuSeparator />
                <DropdownMenuLabel className="text-[0.65rem] font-black uppercase tracking-widest opacity-70">
                  {t('nav.group.governance')}
                </DropdownMenuLabel>
                <DropdownMenuItem asChild>
                    <Link to="/docs/compatibility-matrix" className="flex items-center gap-2.5">
                    <Tag size={14} className="text-success" />
                    {t('docs.compatibility')}
                  </Link>
                </DropdownMenuItem>

                <DropdownMenuSeparator />
                <DropdownMenuLabel className="text-[0.65rem] font-black uppercase tracking-widest opacity-70">
                  {lang === 'zh-CN' ? '开发与支持' : 'Development'}
                </DropdownMenuLabel>
                <DropdownMenuItem asChild>
                    <Link to="/docs/api-reference" className="flex items-center gap-2.5">
                    <TerminalSquare size={14} className="text-primary" />
                    {t('docs.apiReference')}
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                    <Link to="/docs/troubleshooting" className="flex items-center gap-2.5">
                    <LifeBuoy size={14} className="text-danger" />
                    {t('docs.troubleshooting')}
                  </Link>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            <Separator orientation="vertical" className="h-5 opacity-30 mx-1 hidden md:block" />
          </div>
        </header>

        <div className="flex-1 overflow-y-auto relative">
          <AnimatePresence mode="wait">
            <motion.div
              key={location.pathname}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              transition={{ duration: 0.18 }}
              className="flex min-h-full flex-col p-4 lg:p-8"
            >
              <Breadcrumbs />
              <Outlet />
            </motion.div>
          </AnimatePresence>
        </div>
      </main>

      <ChatFab />
    </div>
  );
};

// ─── Sidebar Nav (shared between desktop & mobile sheet) ──────────────────────

interface SidebarNavProps {
  sidebarOpen: boolean;
  user: ReturnType<typeof useAuth>['user'];
  logout: () => void;
  t: ReturnType<typeof useI18n>['t'];
  navGroups: NavGroupDefinition[];
}

const SidebarNav: React.FC<SidebarNavProps> = ({ sidebarOpen, user, logout, t, navGroups }) => (
  <>
    <nav className={cn(
      "flex flex-1 flex-col gap-1 overflow-y-auto",
      sidebarOpen ? "px-3 py-6" : "px-1 py-3 items-center"
    )}>
      {navGroups.map((group, index) => (
        <React.Fragment key={group.id}>
          {index > 0 ? <Separator className="my-2 opacity-40" /> : null}
          <NavGroup label={t(group.labelKey as never)} sidebarOpen={sidebarOpen} defaultOpen={true}>
            {group.routes.map((route) => {
              const Icon = route.icon;
              return (
                <NavItem
                  key={route.id}
                  to={route.path}
                  label={t(route.labelKey as never)}
                  icon={<Icon size={16} />}
                  sidebarOpen={sidebarOpen}
                />
              );
            })}
          </NavGroup>
        </React.Fragment>
      ))}
    </nav>

    {/* User Profile */}
    <div className="group mx-3 my-4 flex items-center gap-3 rounded-2xl border border-border bg-card/75 p-3 transition-colors hover:bg-card">
      <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/14 text-[0.85rem] font-black text-primary shadow-sm transition-transform group-hover:scale-105">
        {user?.username?.charAt(0).toUpperCase() || 'U'}
      </div>
      {sidebarOpen && (
        <div className="flex flex-col min-w-0">
          <span className="text-sm font-bold text-foreground truncate leading-tight">
            {user?.username || t('header.operator')}
          </span>
          <span className="text-[0.65rem] text-muted-foreground uppercase font-black tracking-widest opacity-60">
            {t('header.operatorRole')}
          </span>
        </div>
      )}
      {sidebarOpen && (
        <Button
          variant="ghost"
          size="icon"
          className="ml-auto size-8 shrink-0 opacity-40 transition-colors hover:bg-danger/10 hover:text-danger hover:opacity-100"
          onClick={logout}
          title={t('header.logout')}
        >
          <LogOut size={14} />
        </Button>
      )}
    </div>
  </>
);

// ─── Sub-components ──────────────────────────────────────────────────────────

const NavGroup: React.FC<{
  label: string;
  children: React.ReactNode;
  defaultOpen?: boolean;
  sidebarOpen?: boolean;
}> = ({ label, children, defaultOpen = false, sidebarOpen = true }) => {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  const location = useLocation();
  const isActive = React.Children.toArray(children).some(child => {
    if (React.isValidElement<{ to?: string }>(child) && child.props.to) {
      return location.pathname === child.props.to || location.pathname.startsWith(child.props.to + '/');
    }
    return false;
  });

  if (!sidebarOpen) {
    return <div className="flex flex-col gap-1.5 py-2">{children}</div>;
  }

  return (
    <div className="mt-2 px-3">
      <button
        className={cn(
          "group flex w-full items-center justify-between rounded-lg px-3 py-2 text-xs font-black uppercase tracking-[0.15em] text-muted-foreground/70 transition-colors",
          isActive && "text-primary opacity-100"
        )}
        onClick={() => setIsOpen(!isOpen)}
      >
        <span className="truncate">{label}</span>
        <ChevronDown
          size={12}
          className={cn(
            "shrink-0 transition-transform duration-300 opacity-40 group-hover:opacity-100",
            isOpen && "rotate-180"
          )}
        />
      </button>
      {isOpen && (
        <div className="mt-1 animate-in slide-in-from-left-1 duration-200 space-y-0.5">
          {children}
        </div>
      )}
    </div>
  );
};

const NavItem: React.FC<{
  to: string;
  label: string;
  icon: React.ReactNode;
  sidebarOpen?: boolean;
}> = ({ to, label, icon, sidebarOpen = true }) => {
  const location = useLocation();
  const isActive = location.pathname === to || location.pathname.startsWith(to + '/');

  if (!sidebarOpen) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <NavLink
            to={to}
            className={cn(
              "flex items-center justify-center size-10 rounded-xl transition-all duration-200 relative group",
              isActive ? "bg-primary/12 text-primary" : "text-muted-foreground hover:bg-card hover:text-foreground"
            )}
          >
            {icon}
            {isActive && <motion.div layoutId="sidebar-active-pill-compact" className="absolute left-0 w-1 h-5 bg-[var(--primary)] rounded-r-full" />}
          </NavLink>
        </TooltipTrigger>
        <TooltipContent side="right" className="font-bold">{label}</TooltipContent>
      </Tooltip>
    );
  }

  return (
    <NavLink
      to={to}
      className={cn(
        "flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium transition-all duration-200 relative group",
        isActive ? "bg-primary/12 text-primary shadow-inner" : "text-muted-foreground hover:bg-card hover:text-foreground"
      )}
    >
      <span className={cn("shrink-0 transition-transform duration-200 group-hover:scale-110", isActive && "text-primary")}>
        {icon}
      </span>
      <span className="flex-1 truncate">{label}</span>
      {isActive && (
        <motion.div 
          layoutId="sidebar-active-indicator" 
          className="absolute left-0 h-5 w-1 rounded-r-full bg-primary shadow-[0_0_12px_var(--primary-glow)]" 
        />
      )}
    </NavLink>
  );
};

const InboxDropdown: React.FC = () => {
  const { t, lang } = useI18n();
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<InboxMessage[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(false);

  const fetchInbox = useCallback(async () => {
    try {
      setLoading(true);
      const res = await listInbox({ limit: 20 });
      setMessages(res.items ?? []);
      setUnreadCount(res.unread_count ?? 0);
    } catch {
      // silently ignore - inbox is non-critical
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchInbox();
    const interval = setInterval(fetchInbox, 30_000);
    return () => clearInterval(interval);
  }, [fetchInbox]);

  const handleMarkRead = async (id: string) => {
    try {
      await markInboxRead(id);
      setMessages((prev) => prev.map((m) => m.id === id ? { ...m, is_read: true } : m));
      setUnreadCount((c) => Math.max(0, c - 1));
    } catch { /* ignore */ }
  };

  const handleMarkAllRead = async () => {
    try {
      await markAllInboxRead();
      setMessages((prev) => prev.map((m) => ({ ...m, is_read: true })));
      setUnreadCount(0);
    } catch { /* ignore */ }
  };

  const formatTime = (iso: string) => {
    try {
      const d = new Date(iso);
      const now = new Date();
      const diff = now.getTime() - d.getTime();
      if (diff < 60_000) return lang === 'zh-CN' ? '刚刚' : 'just now';
      if (diff < 3_600_000) return lang === 'zh-CN' ? `${Math.floor(diff / 60_000)}分钟前` : `${Math.floor(diff / 60_000)}m ago`;
      if (diff < 86_400_000) return lang === 'zh-CN' ? `${Math.floor(diff / 3_600_000)}小时前` : `${Math.floor(diff / 3_600_000)}h ago`;
      return d.toLocaleDateString(lang, { month: 'short', day: 'numeric' });
    } catch {
      return '';
    }
  };

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="icon"
          className="size-8 relative"
          title={t('inbox.title')}
        >
          <Bell size={14} />
          {unreadCount > 0 && (
          <span className="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-danger px-0.5 text-[10px] font-bold leading-none text-white">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="w-[340px] p-0 overflow-hidden"
        sideOffset={4}
      >
        <div className="flex items-center justify-between border-b border-border bg-card px-4 py-3">
          <div className="flex items-center gap-2">
            <h3 className="m-0 text-sm font-bold">
              {t('inbox.title')}
            </h3>
            {unreadCount > 0 && (
              <Badge variant="danger" className="text-[10px] px-1.5 py-0.5">
                {unreadCount}
              </Badge>
            )}
          </div>
          {unreadCount > 0 && (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 text-xs"
              onClick={(e) => { e.preventDefault(); void handleMarkAllRead(); }}
            >
              {t('inbox.markAllRead')}
            </Button>
          )}
        </div>
        <div className="max-h-[360px] overflow-y-auto">
          {loading && messages.length === 0 ? (
            <div className="p-8 text-center">
              <p className="text-sm text-muted-foreground">{lang === 'zh-CN' ? '加载中...' : 'Loading...'}</p>
            </div>
          ) : messages.length === 0 ? (
            <div className="p-8 text-center">
              <Inbox size={28} className="mx-auto mb-3 opacity-20" />
              <p className="text-sm text-muted-foreground">{t('inbox.empty')}</p>
              <p className="text-xs opacity-50 mt-1">{t('inbox.placeholder')}</p>
            </div>
          ) : (
            messages.map((msg) => (
              <div
                key={msg.id}
                onClick={() => !msg.is_read && void handleMarkRead(msg.id)}
                className={cn(
                  "px-4 py-3 border-b border-[var(--border-color)] transition-colors",
                  !msg.is_read && "bg-[rgba(242,184,75,0.04)] cursor-pointer hover:bg-[rgba(242,184,75,0.06)]",
                  msg.is_read && "cursor-default"
                )}
              >
                <div className="flex items-start gap-2">
                  {!msg.is_read && (
                    <span className="size-1.5 rounded-full bg-[var(--danger)] shrink-0 mt-2" />
                  )}
                  <div className="flex-1 min-w-0">
                    <div className={cn("text-sm truncate", msg.is_read ? 'font-normal' : 'font-semibold')}>
                      {msg.subject}
                    </div>
                    <div className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                      {msg.body}
                    </div>
                    <div className="mt-1 text-[10px] text-muted-foreground opacity-60">
                      {formatTime(msg.created_at)}
                    </div>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
        <div className="border-t border-border bg-card p-2">
          <Button variant="ghost" size="sm" className="w-full justify-center" asChild>
            <Link to="/inbox">{t('header.openInbox')}</Link>
          </Button>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};

const ChatFab: React.FC = () => {
  const { t } = useI18n();
  return (
    <div className="fixed bottom-8 right-8 z-[100]">
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="primary" className="size-14 rounded-full border border-primary/20 p-0 shadow-[0_8px_32px_rgba(50,199,189,0.24)]" asChild>
            <Link to="/chat" aria-label={t('header.openChat')}>
              <MessageCircle size={22} />
            </Link>
          </Button>
        </TooltipTrigger>
        <TooltipContent side="left">{t('header.openChat')}</TooltipContent>
      </Tooltip>
    </div>
  );
};
