# Web Console Frontend Audit Report

> Purpose: record the current Web Console frontend audit against spec/design, based on actual rendered UI, implemented page logic, and targeted browser checks.
> Date: 2026-03-29
> Scope: `http://localhost:8081/`
> Reviewer note: this document follows `00-frontend-module-ui-ux-convergence-template.md` and captures a visual/interaction-focused audit pass based on actual rendered UI.

---

## Findings

### 1. Setup

#### Setup: provider step still mixes provider registry with model binding
- Module: Setup
- Severity: P1
- Current behavior: the setup flow is now a 4-step first-run wizard, but Step 3 still presents `Initial Model Binding` together with provider setup semantics.
- Spec / design expectation: provider configuration and agent role model binding are separate object boundaries; model binding should be narrated under Agent Role, not Provider.
- Gap: the wizard still teaches the wrong ownership model even though the broader product direction has moved closer to the spec.
- Suggested direction: rename and restructure Step 3 so it clearly configures provider access only, then move model-binding semantics to the agent-role step or a dedicated follow-up.
- Files: `web/src/pages/setup/SetupSmokeView.tsx`, `specs/20-component-providers-and-agent-role-binding.md`, `specs/20-component-agent-roles.md`, `specs/40-web-console-setup-workbench.md`

#### Setup: channel usages are still entered as CSV-like text
- Module: Setup
- Severity: P2
- Current behavior: Step 4 already exposes `Kind` and `Usages`, but `Usages` is still a text field with CSV-style input expectations.
- Spec / design expectation: channel usage is a core semantic field and should be expressed via structured selection, not raw text composition.
- Gap: users must know internal tokens and formatting instead of selecting supported usage modes directly.
- Suggested direction: replace free-text usage entry with enum-backed multi-select or chips.
- Files: `web/src/pages/setup/SetupSmokeView.tsx`, `specs/20-component-channels-and-web-chat.md`

#### Setup: runtime checks still carry Telegram-first wording
- Module: Setup
- Severity: P2
- Current behavior: parts of the runtime checks area still read like Telegram-first operational guidance.
- Spec / design expectation: setup copy should reflect first-party channel and web-console mental models, with Telegram treated as one integration mode rather than the default story.
- Gap: the product narrative has shifted, but residual copy still anchors users to an older integration-first setup model.
- Suggested direction: rewrite runtime-check copy around generic channel readiness and first-party message delivery.
- Files: `web/src/pages/setup/SetupSmokeView.tsx`, `specs/40-web-console-setup-workbench.md`, `specs/20-component-channels-and-web-chat.md`

### 2. Login / Auth Providers

#### Login: default entry path is still break-glass token login
- Module: Login / Auth Providers
- Severity: P1
- Current behavior: `/login` defaults to `Local Token / Break Glass` on first paint.
- Spec / design expectation: the normal login path should foreground standard authentication, while break-glass remains exceptional and clearly secondary.
- Gap: the initial state communicates an emergency access path as the default authentication model.
- Suggested direction: switch the default provider selection to the normal auth path and visually demote break-glass access.
- Files: `web/src/pages/ops/LoginView.tsx`, `specs/20-component-identity-access.md`, `project/tars_prd.md`

#### Login: redirect notice i18n key is broken
- Module: Login / Auth Providers
- Severity: P1
- Current behavior: `login.redirectNotice` resolves incorrectly in the login experience.
- Spec / design expectation: redirect-state messaging should render readable, localized guidance.
- Gap: a key-level wiring issue leaks into visible product copy.
- Suggested direction: fix the translation key mapping and verify the redirect flow text in both supported locales.
- Files: `web/src/pages/ops/LoginView.tsx`

#### Auth providers console is missing `local_password`
- Module: Login / Auth Providers
- Severity: P1
- Current behavior: `AuthProvidersPage` does not surface `local_password` as a manageable provider type.
- Spec / design expectation: auth provider management should expose the supported identity provider types in the control plane.
- Gap: the operator-facing model is incomplete relative to supported authentication paths.
- Suggested direction: add `local_password` to the provider type model and UI treatment.
- Files: `web/src/pages/identity/AuthProvidersPage.tsx`, `web/src/lib/api/access.ts`, `web/src/lib/api/types.ts`

#### Identity navigation does not provide direct entry to key submodules
- Module: Login / Auth Providers
- Severity: P2
- Current behavior: Identity remains a shallow navigation bucket rather than exposing direct submodule entry points.
- Spec / design expectation: identity governance areas should be directly navigable because they represent distinct operational surfaces.
- Gap: discoverability and wayfinding are weaker than the spec’s information architecture intends.
- Suggested direction: add direct nav targets for major identity subpages.
- Files: `web/src/components/layout/navigation.tsx`, `web/src/pages/identity/IdentityOverview.tsx`, `specs/10-platform-object-boundaries-and-ia.md`

### 3. Dashboard / Ops

#### Dashboard: execution queue panel behaves like a recent list, not a true queue
- Module: Dashboard / Ops
- Severity: P2
- Current behavior: the queue area is effectively a recent executions list.
- Spec / design expectation: queue-oriented surfaces should communicate pending workload and actionable runtime pressure.
- Gap: the panel label implies queue semantics that the implementation does not actually represent.
- Suggested direction: either compute a real pending queue view or relabel the surface to match what is rendered.
- Files: `web/src/pages/dashboard/DashboardView.tsx`, `specs/40-web-console-runtime-dashboard.md`

#### Dashboard: key metrics depend on the current list window
- Module: Dashboard / Ops
- Severity: P2
- Current behavior: important counts appear derived from the currently fetched window rather than stable aggregate metrics.
- Spec / design expectation: dashboard metrics should represent trustworthy operational summaries.
- Gap: the current presentation can drift with pagination or list slice size.
- Suggested direction: separate summary metrics from local table windows and source them from dedicated aggregates.
- Files: `web/src/pages/dashboard/DashboardView.tsx`, `web/src/lib/api/ops.ts`

#### Ops page reads like a configuration surface instead of a repair console
- Module: Dashboard / Ops
- Severity: P1
- Current behavior: `/ops` currently feels closer to a configuration area than an operator repair console.
- Spec / design expectation: Ops should emphasize runtime repair, intervention, and incident handling.
- Gap: the page framing and action hierarchy do not align with the spec’s governance-vs-ops split.
- Suggested direction: reorganize the page around repair actions, degraded-state triage, and operational commands.
- Files: `web/src/pages/ops/OpsActionView.tsx`, `specs/40-web-console-ops-console.md`, `specs/40-web-console-governance-vs-ops.md`

#### Logs route exists without a page implementation in the current workspace
- Module: Dashboard / Ops
- Severity: P1
- Current behavior: `/logs` is routed, but `LogsPage` implementation is missing from the workspace.
- Spec / design expectation: routed operational pages should have a concrete implementation.
- Gap: this is a real product gap rather than a styling issue.
- Suggested direction: either implement the page or remove/guard the route until it exists.
- Files: `web/src/App.tsx`, `web/src/pages/logs/`

### 4. Sessions / Executions / Inbox / Outbox

#### Sessions: `session_id` visibility is still weak for debugging workflows
- Module: Sessions / Executions / Inbox / Outbox
- Severity: P3
- Current behavior: sessions are broadly aligned, but `session_id` is not strongly foregrounded.
- Spec / design expectation: debug-critical identifiers should be easy to see and copy.
- Gap: operators lose a quick handle that the spec expects to be prominent.
- Suggested direction: add stronger ID presentation and copy affordances in detail and list contexts.
- Files: `web/src/pages/sessions/SessionList.tsx`, `web/src/pages/sessions/SessionDetail.tsx`, `specs/40-web-console-sessions-workbench.md`

#### Executions: approval actions lack reason / note input
- Module: Sessions / Executions / Inbox / Outbox
- Severity: P1
- Current behavior: approval interaction exists, but it does not request structured rationale from the operator.
- Spec / design expectation: governance-sensitive execution decisions should capture operator intent or explanation where appropriate.
- Gap: decision logging and operator accountability are under-expressed in the UI.
- Suggested direction: add reason / note capture to approval and rejection flows.
- Files: `web/src/pages/executions/ExecutionDetail.tsx`, `web/src/components/operator/ExecutionActionBar.tsx`, `specs/40-web-console-executions-workbench.md`

#### Executions: command modification input is always visible
- Module: Sessions / Executions / Inbox / Outbox
- Severity: P2
- Current behavior: the modify-command input remains persistently visible.
- Spec / design expectation: intervention surfaces should keep action boundaries explicit, especially for risky operations.
- Gap: the UI blurs the boundary between passive inspection and active operator intervention.
- Suggested direction: collapse the modification form behind an explicit action trigger.
- Files: `web/src/components/operator/ExecutionActionBar.tsx`

#### Inbox: failed loads can degrade into a misleading empty state
- Module: Sessions / Executions / Inbox / Outbox
- Severity: P2
- Current behavior: on certain failures, Inbox can appear empty rather than clearly errored.
- Spec / design expectation: operator consoles should distinguish “no items” from “failed to load items.”
- Gap: the current empty-state fallback can hide system problems.
- Suggested direction: separate error state rendering from empty-state rendering.
- Files: `web/src/pages/inbox/InboxPage.tsx`, `specs/40-web-console-inbox-workbench.md`

#### Outbox: event id presentation is weak for troubleshooting
- Module: Sessions / Executions / Inbox / Outbox
- Severity: P2
- Current behavior: event IDs are shown in a way that is not especially useful for operational debugging.
- Spec / design expectation: delivery and async-event consoles should prioritize traceable identifiers.
- Gap: operators do not get efficient scan/copy behavior for the main troubleshooting key.
- Suggested direction: improve event-id display, truncation strategy, and copy affordance.
- Files: `web/src/pages/outbox/OutboxConsole.tsx`, `specs/20-component-outbox.md`

#### Outbox: error details need an expanded detail surface
- Module: Sessions / Executions / Inbox / Outbox
- Severity: P2
- Current behavior: error detail presentation lacks an expandable or drawer-style deep-inspection affordance.
- Spec / design expectation: delivery failures should support drill-down inspection.
- Gap: operators cannot easily inspect the full failure context from the main workflow.
- Suggested direction: add expandable detail rows or a side panel for error payloads and retries.
- Files: `web/src/pages/outbox/OutboxConsole.tsx`, `specs/20-component-outbox.md`

### 5. Providers

#### Providers: main page path is broadly aligned
- Module: Providers
- Severity: P3
- Current behavior: the primary providers registry path is broadly aligned with current object modeling.
- Spec / design expectation: provider registry should stay focused on provider configuration rather than role binding.
- Gap: no significant page-level gap found here; the larger issue sits in Setup narration.
- Suggested direction: no major immediate change required on the main providers page.
- Files: `web/src/pages/providers/ProvidersPage.tsx`

### 6. Connectors

#### Connectors: scope / permission boundary is under-expressed in detail view
- Module: Connectors
- Severity: P3
- Current behavior: the connectors structure is generally solid, but detail pages do not make scope and permission boundaries especially visible.
- Spec / design expectation: external integration surfaces should communicate access scope clearly.
- Gap: governance context is present conceptually but not surfaced strongly enough in the UI.
- Suggested direction: expose connector scope and permission boundary as first-class detail fields.
- Files: `web/src/pages/connectors/ConnectorsList.tsx`, `web/src/pages/connectors/ConnectorDetail.tsx`, `specs/20-component-connectors.md`

### 7. Agent Roles / Identity

#### Agent Roles: main page is largely aligned
- Module: Agent Roles / Identity
- Severity: P3
- Current behavior: the Agent Roles page already communicates `model_binding` reasonably well.
- Spec / design expectation: model binding belongs with Agent Roles, not Providers.
- Gap: no major page-level gap found on the role page itself.
- Suggested direction: keep this page as the anchor when fixing Setup narration.
- Files: `web/src/pages/identity/AgentRolesPage.tsx`, `specs/20-component-agent-roles.md`

#### Users: security posture is not visible enough
- Module: Agent Roles / Identity
- Severity: P2
- Current behavior: Users does not surface MFA, challenge state, or password-login security posture clearly.
- Spec / design expectation: identity management should expose meaningful account security signals.
- Gap: operator awareness around authentication hardening is limited.
- Suggested direction: surface MFA/challenge/password-login indicators in list and detail contexts.
- Files: `web/src/pages/identity/UsersPage.tsx`, `specs/20-component-identity-access.md`

#### People: object narrative has shifted, but IA and input model are still inconsistent
- Module: Agent Roles / Identity
- Severity: P1
- Current behavior: People copy now suggests org-owned semantics, but the page still lives under Identity and still allows raw CSV-style `channel_ids` input.
- Spec / design expectation: object ownership and interaction model should agree with the underlying boundary.
- Gap: the page narrative, navigation location, and form semantics are still partially contradictory.
- Suggested direction: resolve ownership placement in IA and replace raw channel-id entry with structured selection.
- Files: `web/src/pages/identity/PeoplePage.tsx`, `web/src/components/layout/navigation.tsx`, `specs/10-platform-object-boundaries-and-ia.md`, `specs/20-component-identity-access.md`, `specs/20-component-org.md`

#### Org: page still exposes legacy naming and lacks quota / usage visibility
- Module: Agent Roles / Identity
- Severity: P2
- Current behavior: Org still uses fields like `default_locale` and `default_timezone`, and it does not expose quota / usage visibility well.
- Spec / design expectation: org-facing settings should align with current naming and governance signals.
- Gap: the page looks partly anchored to an older config vocabulary.
- Suggested direction: update naming to match the current object model and surface org quota / usage summaries.
- Files: `web/src/pages/org/OrgPage.tsx`, `specs/20-component-org.md`

### 8. Automations / Triggers

#### Automations: trigger is still not expressed as subordinate automation structure
- Module: Automations / Triggers
- Severity: P1
- Current behavior: Automations already expose key fields like `agent_role_id` and `governance_policy`, but the UI still treats Trigger as too separate.
- Spec / design expectation: triggering logic should be understood within the automation story, not as a disconnected top-level governance island.
- Gap: the information architecture still fragments one conceptual workflow across separate top-level surfaces.
- Suggested direction: reframe triggers as automation-owned or automation-scoped structures in the frontend IA.
- Files: `web/src/pages/automations/AutomationsPage.tsx`, `web/src/pages/triggers/TriggersPage.tsx`, `specs/20-component-automations.md`, `specs/30-strategy-automations-and-triggers.md`

#### Triggers: standalone governance page lacks automation ownership context
- Module: Automations / Triggers
- Severity: P1
- Current behavior: the Trigger page still reads as an independent governance page.
- Spec / design expectation: trigger configuration should preserve clear automation ownership context.
- Gap: users can manage trigger-like objects without enough awareness of the automation they belong to.
- Suggested direction: add parent automation context to list/detail flows, or fold trigger management into automation workflows.
- Files: `web/src/pages/triggers/TriggersPage.tsx`, `specs/30-strategy-automations-and-triggers.md`

#### Triggers: `template_id` is still free text
- Module: Automations / Triggers
- Severity: P2
- Current behavior: trigger configuration accepts `template_id` as raw text.
- Spec / design expectation: referenced platform objects should be selected structurally when possible.
- Gap: users must know internal identifiers instead of selecting from available templates.
- Suggested direction: replace free-text template input with object selection.
- Files: `web/src/pages/triggers/TriggersPage.tsx`, `web/src/lib/api/triggers.ts`

### 9. Channels

#### Channels: `usages` and `capabilities` are still raw CSV inputs
- Module: Channels
- Severity: P1
- Current behavior: the page has adopted `kind` and `usages`, but `usages` and `capabilities` are still entered as CSV-like text.
- Spec / design expectation: these are core semantic fields and should be edited through structured controls.
- Gap: the frontend exposes implementation-shaped text input instead of product-shaped interaction.
- Suggested direction: convert both fields to structured multi-select controls with explicit supported values.
- Files: `web/src/pages/channels/ChannelsPage.tsx`, `specs/20-component-channels-and-web-chat.md`

#### Channels: hero narrative improved, but the list is not actually grouped by channel role
- Module: Channels
- Severity: P2
- Current behavior: the hero copy distinguishes first-party and external channels, but the underlying list view does not reflect that grouping.
- Spec / design expectation: the page structure should reinforce the conceptual grouping it introduces.
- Gap: the page says one thing and the collection presentation says another.
- Suggested direction: group or segment channels by role / kind in the main registry view.
- Files: `web/src/pages/channels/ChannelsPage.tsx`, `specs/20-component-channels-and-web-chat.md`

#### Channels: delivery strategy semantics are not surfaced
- Module: Channels
- Severity: P2
- Current behavior: delivery behaviors such as `reply_current_session` are not clearly expressed in the UI.
- Spec / design expectation: delivery strategy is part of how channels behave and should be understandable to operators.
- Gap: users can configure channels without seeing the routing / reply behavior clearly.
- Suggested direction: surface delivery strategy fields in list/detail/form presentation.
- Files: `web/src/pages/channels/ChannelsPage.tsx`, `specs/20-component-channels-and-web-chat.md`

### 10. Message Templates / Notification Templates

#### Templates: UI still tells the story of legacy `msg template` objects
- Module: Msg Templates / Notification Templates
- Severity: P1
- Current behavior: the frontend still frames templates around the older `msg template` object model.
- Spec / design expectation: the product should communicate notification-template semantics, usage context, and lifecycle more clearly.
- Gap: the narrative and visible object boundary have not caught up to the spec.
- Suggested direction: rename and restructure the page language around notification templates and usage context.
- Files: `web/src/pages/msg-templates/MsgTemplatesPage.tsx`, `web/src/lib/api/msgtpl.ts`, `specs/20-component-notification-templates.md`

#### Templates: lifecycle is still `enabled/status` rather than a clearer state model
- Module: Msg Templates / Notification Templates
- Severity: P1
- Current behavior: lifecycle treatment mixes `enabled` and `status`.
- Spec / design expectation: templates should expose a coherent lifecycle model rather than overlapping state concepts.
- Gap: the state story remains harder to understand and govern.
- Suggested direction: normalize template lifecycle into one explicit state model in the UI.
- Files: `web/src/pages/msg-templates/MsgTemplatesPage.tsx`, `specs/20-component-notification-templates.md`

#### Templates: no real `variable_schema` editing support
- Module: Msg Templates / Notification Templates
- Severity: P1
- Current behavior: the page only offers static hints rather than actual `variable_schema` editing affordances.
- Spec / design expectation: variable schema should be authorable and inspectable as a first-class part of template configuration.
- Gap: template authoring is materially incomplete.
- Suggested direction: add structured schema editing and preview support.
- Files: `web/src/pages/msg-templates/MsgTemplatesPage.tsx`, `specs/20-component-notification-templates.md`

#### Templates: `usage_refs` are not surfaced
- Module: Msg Templates / Notification Templates
- Severity: P2
- Current behavior: usage references are not represented in the current UI.
- Spec / design expectation: operators should understand where templates are intended to be used.
- Gap: usage discoverability is limited.
- Suggested direction: expose usage references in list/detail views.
- Files: `web/src/pages/msg-templates/MsgTemplatesPage.tsx`, `specs/20-component-notification-templates.md`

### 11. Skills / Extensions

#### Skills: status model is incomplete
- Module: Skills / Extensions
- Severity: P2
- Current behavior: Skills do not expose states such as `deprecated` or `archived`.
- Spec / design expectation: governed reusable assets should have a fuller lifecycle model.
- Gap: the UI cannot express important governance distinctions.
- Suggested direction: extend status presentation to cover the missing lifecycle states.
- Files: `web/src/pages/skills/SkillsList.tsx`, `web/src/pages/skills/SkillDetail.tsx`, `specs/20-component-skills.md`

#### Skills: governance execution policy is not visible in the list
- Module: Skills / Extensions
- Severity: P2
- Current behavior: the skills list does not show `governance.execution_policy`.
- Spec / design expectation: governance-sensitive execution constraints should be visible when scanning skills.
- Gap: operators must drill in or infer a critical governance property.
- Suggested direction: expose execution policy in list summaries or badges.
- Files: `web/src/pages/skills/SkillsList.tsx`, `specs/20-component-skills.md`

#### Extensions: promotion policy is not surfaced
- Module: Skills / Extensions
- Severity: P3
- Current behavior: the Extensions page does not show `promotion_policy`.
- Spec / design expectation: promotion behavior should be inspectable in extension management.
- Gap: one governance detail is absent from the visible model.
- Suggested direction: add promotion-policy visibility where extension state is summarized.
- Files: `web/src/pages/extensions/ExtensionsPage.tsx`, `specs/20-component-extensions.md`

### 12. Docs / Search / Global Navigation

#### Global search indexes too little content and truncates too aggressively
- Module: Docs / Search / Global Navigation
- Severity: P1
- Current behavior: global search only indexes 4 documents, truncates content, and only shows the first 3 results.
- Spec / design expectation: search should act as a real discovery surface for docs and console knowledge.
- Gap: the current implementation is closer to a toy lookup than a usable navigation aid.
- Suggested direction: expand indexing scope, improve result depth, and reduce arbitrary truncation.
- Files: `web/src/components/layout/GlobalSearch.tsx`, `web/src/pages/docs/DocsView.tsx`, `specs/40-web-console.md`

#### Docs dropdown and DocsView do not match
- Module: Docs / Search / Global Navigation
- Severity: P2
- Current behavior: the docs dropdown and the DocsView directory are inconsistent.
- Spec / design expectation: documentation navigation surfaces should agree on the available information architecture.
- Gap: users can see different structures depending on entry point.
- Suggested direction: align docs navigation sources and shared metadata.
- Files: `web/src/components/layout/navigation.tsx`, `web/src/pages/docs/DocsView.tsx`

#### `/docs` lacks a proper landing page
- Module: Docs / Search / Global Navigation
- Severity: P2
- Current behavior: `/docs` jumps directly to the user guide instead of presenting a docs landing page.
- Spec / design expectation: docs should have an entry page that explains major sections and entry paths.
- Gap: the documentation surface lacks orientation.
- Suggested direction: add a true docs landing page with section-level entry points.
- Files: `web/src/pages/docs/DocsView.tsx`, `specs/40-web-console.md`

#### Mobile global shell hides key controls
- Module: Docs / Search / Global Navigation
- Severity: P1
- Current behavior: Theme, Language, Docs, and Search are not visible in mobile layouts.
- Spec / design expectation: essential shell utilities should remain accessible on mobile.
- Gap: important navigation and personalization tools disappear on smaller screens.
- Suggested direction: redesign the mobile shell to preserve access to these functions.
- Files: `web/src/components/layout/AppLayout.tsx`, `web/src/components/layout/navigation.tsx`, `specs/40-web-console.md`, `specs/40-ux-design-system.md`

#### Breadcrumbs / route metadata can degrade into raw slugs
- Module: Docs / Search / Global Navigation
- Severity: P2
- Current behavior: breadcrumb and route metadata fallbacks can expose raw slug text.
- Spec / design expectation: navigation chrome should preserve product-quality naming.
- Gap: implementation fallback behavior leaks internal route shape into visible UI.
- Suggested direction: tighten route metadata coverage and breadcrumb label mapping.
- Files: `web/src/components/layout/Breadcrumbs.tsx`, `web/src/App.tsx`

#### Chat route still speaks in terminal-chat language
- Module: Docs / Search / Global Navigation
- Severity: P2
- Current behavior: `/chat` still uses `Terminal Chat / 终端对话` framing.
- Spec / design expectation: the current story should align with Web Chat and first-party channel semantics.
- Gap: residual wording conflicts with the platform’s newer channel narrative.
- Suggested direction: rename page copy and related labels to the current web-chat model.
- Files: `web/src/pages/chat/ChatPage.tsx`, `specs/20-component-channels-and-web-chat.md`, `specs/40-web-console-chat-workbench.md`

## Modules With No Significant Gap Found

- Providers: no significant page-level gap found beyond Setup narration spillover.
- Agent Roles: page-level structure is broadly aligned and already expresses `model_binding` better than Setup.
- Sessions: broadly aligned, with mostly debug-surface refinements remaining.

## Overall Summary

- The frontend is not uniformly underbuilt. Core runtime paths such as Sessions, Executions, parts of Agent Roles, and Connectors have already moved meaningfully toward the spec.
- The main remaining problems are concentrated in three areas:
  1. object-boundary storytelling is still not fully closed
  2. core semantic fields exist, but interaction layers still expose raw strings / CSV input
  3. the global shell and governance work surfaces are not yet fully productized
- This audit should be treated as stronger than any older gap-style inventory, especially where the current implementation has already moved on from earlier Setup and channel semantics.

## Top 5 Priorities

1. Fix login defaults and complete `local_password` management support.
2. Close the Setup narrative gap so provider configuration and model binding are no longer mixed.
3. Replace CSV-style editing in Channels and Setup for `usages` / `capabilities` with structured controls.
4. Rework Automations / Triggers frontend IA so triggers are shown with proper automation ownership context.
5. Upgrade Docs / Search / global shell, especially mobile access and search coverage.

## Candidate Refactor Themes

- Identity and access surface cleanup:
  login defaults, auth provider coverage, user security visibility, identity navigation depth.
- Structured semantics input cleanup:
  `usages`, `capabilities`, `channel_ids`, `template_id`, and similar object references.
- Governance IA cleanup:
  Automations vs Triggers, People ownership placement, Ops vs Governance positioning.
- Global shell and discoverability cleanup:
  docs landing, global search, breadcrumb metadata, mobile shell affordances.

## Display-Only vs Structural Work

### Mostly copy / display-layer fixes
- setup runtime-check wording
- login redirect notice i18n fix
- chat page terminology
- breadcrumb label polish
- event-id display polish

### Interaction or information-architecture changes
- login default provider behavior
- Setup provider vs model-binding restructuring
- structured controls for `usages` / `capabilities` / `channel_ids` / `template_id`
- Automations / Triggers IA changes
- mobile shell access model
- docs landing and search expansion
- approval note capture for Executions

## Related Files Reviewed During Audit

- `specs/00-frontend-module-ui-ux-convergence-template.md`
- `specs/00-nav-page-to-spec-map.md`
- `specs/20-component-*.md`
- `project/tars_technical_design.md`
- `project/tars_prd.md`
- `web/src/App.tsx`
- `web/src/components/layout/navigation.tsx`
- `web/src/components/layout/AppLayout.tsx`
- `web/src/components/layout/GlobalSearch.tsx`
- `web/src/components/layout/Breadcrumbs.tsx`
- `web/src/pages/setup/SetupSmokeView.tsx`
- `web/src/pages/ops/LoginView.tsx`
- `web/src/pages/docs/DocsView.tsx`
- `web/src/pages/dashboard/DashboardView.tsx`
- `web/src/pages/ops/OpsActionView.tsx`
- `web/src/pages/ops/ObservabilityPage.tsx`
- `web/src/pages/audit/AuditList.tsx`
- `web/src/pages/outbox/OutboxConsole.tsx`
- `web/src/pages/inbox/InboxPage.tsx`
- `web/src/pages/chat/ChatPage.tsx`
- `web/src/pages/sessions/SessionList.tsx`
- `web/src/pages/sessions/SessionDetail.tsx`
- `web/src/pages/executions/ExecutionList.tsx`
- `web/src/pages/executions/ExecutionDetail.tsx`
- `web/src/components/operator/ExecutionActionBar.tsx`
- `web/src/lib/operator.ts`
- `web/src/pages/providers/ProvidersPage.tsx`
- `web/src/pages/channels/ChannelsPage.tsx`
- `web/src/pages/triggers/TriggersPage.tsx`
- `web/src/pages/automations/AutomationsPage.tsx`
- `web/src/pages/connectors/ConnectorsList.tsx`
- `web/src/pages/connectors/ConnectorDetail.tsx`
- `web/src/pages/skills/SkillsList.tsx`
- `web/src/pages/skills/SkillDetail.tsx`
- `web/src/pages/extensions/ExtensionsPage.tsx`
- `web/src/pages/msg-templates/MsgTemplatesPage.tsx`
- `web/src/pages/identity/IdentityOverview.tsx`
- `web/src/pages/identity/AuthProvidersPage.tsx`
- `web/src/pages/identity/UsersPage.tsx`
- `web/src/pages/identity/GroupsPage.tsx`
- `web/src/pages/identity/RolesPage.tsx`
- `web/src/pages/identity/AgentRolesPage.tsx`
- `web/src/pages/identity/PeoplePage.tsx`
- `web/src/pages/org/OrgPage.tsx`
