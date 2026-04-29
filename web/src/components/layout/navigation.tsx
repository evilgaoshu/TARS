import {
  Activity,
  BookOpenText,
  Bot,
  BrainCircuit,
  Building2,
  Clock3,
  FileText,
  Inbox,
  LayoutDashboard,
  LayoutTemplate,
  MessageCircle,
  PackagePlus,
  PlugZap,
  RadioTower,
  ScrollText,
  Settings,
  ShieldCheck,
  TerminalSquare,
  UserCog,
  Workflow,
  Waypoints,
  Zap,
  type LucideIcon,
} from 'lucide-react'

import { useAuth } from '../../hooks/useAuth'
import { canAccessOps } from '../../lib/auth/permissions'
import { useCapabilities } from '../../lib/FeatureGateContext'
import { isEnabled, type Capability } from '../../lib/featureGates'
import type { AuthUserSummary } from '../../lib/api/types'

export interface NavRouteDefinition {
  id: string
  path: string
  labelKey: string
  icon: LucideIcon
  keywords: string
  capability?: Capability
  canAccess?: (user: AuthUserSummary | null | undefined) => boolean
}

export interface NavGroupDefinition {
  id: string
  labelKey: string
  routes: NavRouteDefinition[]
}

export function useNavigationGroups(): NavGroupDefinition[] {
  const { capabilities } = useCapabilities()
  const { user } = useAuth()

  const allGroups: NavGroupDefinition[] = [
    {
      id: 'runtime',
      labelKey: 'nav.group.runtime',
      routes: [
        { id: 'sessions', path: '/sessions', labelKey: 'nav.sessions', icon: Zap, keywords: 'sessions diagnosis incidents triage 会话 诊断 告警' },
        { id: 'executions', path: '/executions', labelKey: 'nav.executions', icon: TerminalSquare, keywords: 'executions approvals runs approval execution 执行 审批' },
        { id: 'dashboard', path: '/runtime', labelKey: 'nav.dashboard', icon: LayoutDashboard, keywords: 'dashboard command center incidents runtime posture overview 总览 运行中心' },
        { id: 'runtime-checks', path: '/runtime-checks', labelKey: 'nav.runtimeChecks', icon: Activity, keywords: 'runtime checks smoke validation health checks 自检 体检 运行检查' },
      ],
    },
    {
      id: 'delivery',
      labelKey: 'nav.group.aiDelivery',
      routes: [
        { id: 'inbox', path: '/inbox', labelKey: 'nav.inbox', icon: Inbox, keywords: 'inbox notifications messages 通知 站内信', capability: 'channels.inbox' },
        { id: 'chat', path: '/chat', labelKey: 'nav.chat', icon: MessageCircle, keywords: 'chat web chat assistant 对话 会话' },
        { id: 'providers', path: '/providers', labelKey: 'nav.providers', icon: BrainCircuit, keywords: 'providers ai llm models reasoning provider 模型 提供方' },
        { id: 'channels', path: '/channels', labelKey: 'nav.channels', icon: RadioTower, keywords: 'channels delivery telegram in app inbox 渠道 触达' },
        { id: 'notification-templates', path: '/notification-templates', labelKey: 'nav.notificationTemplates', icon: LayoutTemplate, keywords: 'notification templates notifications delivery copy 模板 通知' },
      ],
    },
    {
      id: 'platform',
      labelKey: 'nav.group.platform',
      routes: [
        { id: 'connectors', path: '/connectors', labelKey: 'nav.connectors', icon: PlugZap, keywords: 'connectors integrations jumpserver connector 连接器 集成' },
        { id: 'automations', path: '/automations', labelKey: 'nav.automations', icon: Clock3, keywords: 'automations triggers workflows 自动化 触发器', capability: 'automations.workflows' },
        { id: 'skills', path: '/skills', labelKey: 'nav.skills', icon: Bot, keywords: 'skills manifests playbooks tools 技能 剧本' },
        { id: 'extensions', path: '/extensions', labelKey: 'nav.extensions', icon: PackagePlus, keywords: 'extensions bundles marketplace 扩展 插件', capability: 'extensions.bundles' },
        { id: 'knowledge', path: '/knowledge', labelKey: 'nav.knowledge', icon: BookOpenText, keywords: 'knowledge docs rag context 知识库 文档', capability: 'knowledge.vector' },
      ],
    },
    {
      id: 'governance',
      labelKey: 'nav.group.governance',
      routes: [
        { id: 'observability', path: '/ops/observability', labelKey: 'nav.observability', icon: Waypoints, keywords: 'observability metrics traces 可观测性 指标', capability: 'observability.internal' },
        { id: 'audit', path: '/audit', labelKey: 'nav.audit', icon: ScrollText, keywords: 'audit trail governance 审计 治理' },
        { id: 'logs', path: '/logs', labelKey: 'nav.logs', icon: FileText, keywords: 'logs runtime logs 日志' },
        { id: 'triggers', path: '/triggers', labelKey: 'nav.triggers', icon: Workflow, keywords: 'governance triggers advanced delivery automation 治理 触发器 高级' },
        { id: 'outbox', path: '/outbox', labelKey: 'nav.outbox', icon: Inbox, keywords: 'outbox delivery retries 发件箱 重试' },
        { id: 'ops', path: '/ops', labelKey: 'nav.ops', icon: Settings, keywords: 'ops settings configs 运维 配置', canAccess: canAccessOps },
      ],
    },
    {
      id: 'identity',
      labelKey: 'nav.group.identity',
      routes: [
        { id: 'identity', path: '/identity', labelKey: 'nav.identity', icon: ShieldCheck, keywords: 'identity auth users groups roles 身份 认证 权限' },
        { id: 'agent-roles', path: '/identity/agent-roles', labelKey: 'nav.agentRoles', icon: UserCog, keywords: 'agent roles persona capability policy 智能体角色 能力 策略' },
        { id: 'org', path: '/org', labelKey: 'nav.org', icon: Building2, keywords: 'org tenant tenants organization 租户 组织' },
      ],
    },
    {
      id: 'docs',
      labelKey: 'nav.group.overview',
      routes: [
        { id: 'docs', path: '/docs', labelKey: 'nav.docs', icon: BookOpenText, keywords: 'docs guides playbooks help 文档 指南 手册' },
      ],
    },
  ]

  return allGroups.map(group => ({
    ...group,
    routes: group.routes.filter(route => (!route.capability || isEnabled(capabilities, route.capability)) && (!route.canAccess || route.canAccess(user)))
  })).filter(group => group.routes.length > 0)
}

export function useNavigationRoutes(): NavRouteDefinition[] {
  return useNavigationGroups().flatMap((group) => group.routes)
}

export function useFindRouteFunction() {
  const routes = useNavigationRoutes().sort((left, right) => right.path.length - left.path.length)
  return (pathname: string) => 
    routes.find((route) => route.path === '/' ? pathname === '/' : pathname === route.path || pathname.startsWith(`${route.path}/`))
}

export function useFindNavigationRoute(pathname: string): NavRouteDefinition | undefined {
  const findRoute = useFindRouteFunction()
  return findRoute(pathname)
}
