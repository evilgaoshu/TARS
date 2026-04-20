# Frontend Priority Backlog

> Source inputs: `specs/00-frontend-ux-spec-audit-2026-03-29.md`, `specs/00-frontend-module-ui-ux-convergence-template.md`
> Date: 2026-03-29

---

## By Priority

### P1

| Topic | Scope | Suggested files | Change type |
|------|------|------|------|
| Login default path | Make normal auth the default login path, demote break-glass token login | `web/src/pages/ops/LoginView.tsx`, `web/src/locales/en-US.json`, `web/src/locales/zh-CN.json` | Interaction |
| Auth provider coverage | Add `local_password` to the auth-provider control surface | `web/src/pages/identity/AuthProvidersPage.tsx`, `web/src/lib/api/types.ts` | Interaction |
| Setup provider narration | Stop teaching model binding as a provider-owned concept in first-run setup | `web/src/pages/setup/SetupSmokeView.tsx` | Copy + Interaction |
| Structured channel semantics | Replace CSV editing for `usages` / `capabilities` with structured controls | `web/src/pages/channels/ChannelsPage.tsx`, `web/src/pages/setup/SetupSmokeView.tsx` | Interaction |
| Automations / Triggers IA | Reframe Trigger management as automation-scoped rather than a disconnected top-level governance area | `web/src/pages/automations/AutomationsPage.tsx`, `web/src/pages/triggers/TriggersPage.tsx`, `web/src/components/layout/navigation.tsx` | IA |
| Ops console framing | Rework `/ops` around repair and degraded-state intervention rather than config-first framing | `web/src/pages/ops/OpsActionView.tsx` | IA |
| Logs page gap | Implement `LogsPage` or remove/guard the route until it exists | `web/src/App.tsx`, `web/src/pages/logs/*` | IA |
| Template lifecycle and schema | Move away from legacy msg-template story; add coherent lifecycle and `variable_schema` editing | `web/src/pages/msg-templates/MsgTemplatesPage.tsx`, `web/src/lib/api/msgtpl.ts` | Interaction |

### P2

| Topic | Scope | Suggested files | Change type |
|------|------|------|------|
| Dashboard summaries | Separate queue semantics and real summary aggregates from current-window lists | `web/src/pages/dashboard/DashboardView.tsx`, `web/src/lib/api/ops.ts` | Interaction |
| Inbox error state | Distinguish load failure from empty state | `web/src/pages/inbox/InboxPage.tsx` | Interaction |
| Outbox troubleshooting | Improve event id visibility and add expandable failure detail | `web/src/pages/outbox/OutboxConsole.tsx` | Interaction |
| Identity navigation depth | Add direct entries for major identity submodules | `web/src/components/layout/navigation.tsx` | IA |
| Users security posture | Surface MFA / challenge / password-login security signals | `web/src/pages/identity/UsersPage.tsx` | Display |
| People ownership cleanup | Align IA placement and replace raw `channel_ids` input | `web/src/pages/identity/PeoplePage.tsx`, `web/src/components/layout/navigation.tsx` | IA + Interaction |
| Org vocabulary and quotas | Update old naming and surface quota / usage visibility | `web/src/pages/org/OrgPage.tsx` | Display + Interaction |
| Docs / Search shell | Expand search index, align docs nav, add docs landing, preserve mobile shell actions | `web/src/components/layout/GlobalSearch.tsx`, `web/src/pages/docs/DocsView.tsx`, `web/src/components/layout/AppLayout.tsx`, `web/src/components/layout/navigation.tsx` | IA |
| Channels grouping and delivery semantics | Group first-party vs external and expose delivery strategy | `web/src/pages/channels/ChannelsPage.tsx` | IA + Display |
| Skills lifecycle | Add missing lifecycle states and list execution policy | `web/src/pages/skills/SkillsList.tsx`, `web/src/pages/skills/SkillDetail.tsx` | Display |

### P3

| Topic | Scope | Suggested files | Change type |
|------|------|------|------|
| Session debug affordances | Foreground and copy `session_id` | `web/src/pages/sessions/SessionList.tsx`, `web/src/pages/sessions/SessionDetail.tsx` | Display |
| Connector scope visibility | Surface scope / permission boundaries in connector detail | `web/src/pages/connectors/ConnectorDetail.tsx` | Display |
| Extension promotion policy | Expose `promotion_policy` in extension summaries | `web/src/pages/extensions/ExtensionsPage.tsx` | Display |

---

## By Refactor Theme

### 1. Identity And Access Cleanup

| Item | Priority | Suggested files | Change type |
|------|------|------|------|
| Login default path | P1 | `web/src/pages/ops/LoginView.tsx` | Interaction |
| `local_password` provider management | P1 | `web/src/pages/identity/AuthProvidersPage.tsx` | Interaction |
| Identity navigation depth | P2 | `web/src/components/layout/navigation.tsx` | IA |
| Users security indicators | P2 | `web/src/pages/identity/UsersPage.tsx` | Display |

### 2. Structured Semantic Input Cleanup

| Item | Priority | Suggested files | Change type |
|------|------|------|------|
| Setup `usages` structure | P1 | `web/src/pages/setup/SetupSmokeView.tsx` | Interaction |
| Channels `usages` / `capabilities` structure | P1 | `web/src/pages/channels/ChannelsPage.tsx` | Interaction |
| People `channel_ids` structure | P2 | `web/src/pages/identity/PeoplePage.tsx` | Interaction |
| Trigger `template_id` selection | P2 | `web/src/pages/triggers/TriggersPage.tsx` | Interaction |

### 3. Governance IA Cleanup

| Item | Priority | Suggested files | Change type |
|------|------|------|------|
| Setup provider vs agent-role story | P1 | `web/src/pages/setup/SetupSmokeView.tsx` | Copy + IA |
| Automations / Triggers ownership | P1 | `web/src/pages/automations/AutomationsPage.tsx`, `web/src/pages/triggers/TriggersPage.tsx` | IA |
| Ops repair-console framing | P1 | `web/src/pages/ops/OpsActionView.tsx` | IA |
| People ownership placement | P2 | `web/src/pages/identity/PeoplePage.tsx`, `web/src/components/layout/navigation.tsx` | IA |

### 4. Global Shell And Discoverability

| Item | Priority | Suggested files | Change type |
|------|------|------|------|
| Search index breadth and result depth | P2 | `web/src/components/layout/GlobalSearch.tsx` | Interaction |
| Docs landing and nav consistency | P2 | `web/src/pages/docs/DocsView.tsx`, `web/src/components/layout/navigation.tsx` | IA |
| Mobile shell access to Theme / Language / Docs / Search | P2 | `web/src/components/layout/AppLayout.tsx` | IA |
| Breadcrumb label cleanup | P2 | `web/src/components/layout/Breadcrumbs.tsx` | Display |

---

## Display-Only Vs Structural Split

### Mostly display / copy

- Setup runtime-check wording cleanup
- Login redirect notice i18n fix
- User security badges
- Outbox event-id display polish
- Breadcrumb and page-title label cleanup
- Connector scope visibility

### Interaction / IA / workflow changes

- Login default provider behavior
- `local_password` provider management
- Setup provider vs agent-role reframing
- Structured controls for `usages`, `capabilities`, `channel_ids`, `template_id`
- Automations / Triggers IA rewrite
- Docs / Search / mobile shell expansion
- Ops repair-console restructuring
- Logs page implementation or route removal
