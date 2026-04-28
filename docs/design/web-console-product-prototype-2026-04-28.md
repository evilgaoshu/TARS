# TARS Web Console Product Prototype

Date: 2026-04-28  
Direction: On-call Evidence Desk  
Artifact: `docs/design/prototypes/web-console/index.html`

## Goal

This prototype establishes a high-fidelity static baseline for the TARS Web Console. It frames TARS as an on-call incident copilot for SRE and operations teams, with the primary path:

alert -> evidence confirmation -> AI diagnosis -> human approval -> controlled execution -> observation -> audit and knowledge capture.

The first viewport prioritizes conclusion, evidence, risk, and next action. Sessions and Executions are the main story. Runtime is a command-center support surface. Ops is a repair console.

## Design Tokens

### Color

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| `--bg` | `#f6f3ec` | `#111827` | warm off-white / deep slate background |
| `--panel` | `#fffdf7` | `#17202d` | primary surface |
| `--panel-strong` | `#eef3f4` | `#1f2a38` | raised surface |
| `--border` | `#d8dedc` | `#334155` | structure and dividers |
| `--text` | `#172026` | `#e5edf0` | primary text |
| `--muted` | `#66737a` | `#9aa7b2` | secondary text |
| `--primary` | `#0f9f9a` | `#32c7bd` | actionable / primary action |
| `--evidence` | `#2563eb` | `#68a4ff` | evidence links and trace context |
| `--warning` | `#b7791f` | `#f2b84b` | pending risk / warning |
| `--critical` | `#c2413d` | `#ff7d75` | dangerous / critical |
| `--healthy` | `#2f855a` | `#63c886` | healthy / resolved |
| `--slate` | `#64748b` | `#94a3b8` | muted system information |

### Typography

- UI and body: IBM Plex Sans from CDN.
- IDs, hosts, commands, traces, timestamps: IBM Plex Mono from CDN.
- No serif fonts.

### Spacing, Radius, Shadow

- Base spacing unit: 4px.
- Dense workbench pages use 8px, 12px, 16px rhythm.
- Registry pages use 12px, 16px, 24px rhythm.
- Card radius: 8px.
- Button radius: 6px.
- Badge radius: 4px.
- Shadows are minimal; most layering comes from background and border contrast.

### Status And Risk Mapping

| State | Color |
| --- | --- |
| `open`, `pending`, `degraded`, `warning` | amber |
| `analyzing`, `executing` | teal |
| `resolved`, `completed`, `healthy` | green |
| `failed`, `rejected`, `critical` | red |
| `muted`, `disabled` | slate |

## Component Prototype Coverage

The HTML prototype includes unified examples of:

- App Shell: sidebar, top toolbar, route content.
- Navigation: grouped by Runtime, Delivery, Platform, Governance, Identity, Docs.
- Global Search: overlay with session, execution, connector, audit examples.
- Page Hero: title, breadcrumbs, status, primary and secondary CTA.
- Queue Card: Sessions and Executions cards.
- Evidence Card: metrics, logs, traces, change, execution evidence.
- Risk Badge and Status Chip: shared state and risk language.
- Action Bar: page and card footer actions.
- Approval Dialog: approve, reject, request context pattern with reason input.
- Reason / note input: audit-oriented text entry.
- Filter Bar: triage sorting, status, service, search.
- Registry Table/Card: shared object list and detail language.
- Detail Drawer: right-side details for registry objects.
- Raw Payload Fold: folded JSON, trace, command, console output.
- Empty State, Error State, Loading State, Degraded State, Disabled State.

## Page Coverage

### Full Fidelity Critical Path

- `/login`: normal login default, provider loading state, setup ready/required/bootstrap error states, break-glass folded.
- `/setup`: five-step first-run wizard: admin, auth, provider, channel, complete.
- `/runtime-checks`: Auth, Providers, Connectors, Delivery, Evidence Path, Approval Gate cards with fix links.
- `/sessions`: incident queue with conclusion, risk, next action, evidence, state, service, host, updated time, triage sort.
- `/sessions/:id`: diagnosis detail with current diagnosis, recommended next step, risk, evidence summary, timeline, folded raw context.
- `/executions/:id`: action, reason, risk, approval, result, observation suggestions, approval dialog and folded console output.

### Runtime And Delivery

- `/runtime`: operator command center, focused on whether human intervention is needed.
- `/executions`: approval/run queue, triage sort, no confusion between approved and pending approval.
- `/chat`: first-party operations conversation, not generic chat.
- `/inbox`: first-party delivery/approval inbox; API failure is an error state, not an empty inbox.
- `/providers`, `/channels`, `/notification-templates`: registry pages using shared object patterns.

### Platform Registry Pattern

Connectors and Skills are full-fidelity representative pages. Automations, Extensions, and Knowledge are pattern-delta pages.

- `/connectors`, `/connectors/:id`: SSH, VictoriaMetrics, VictoriaLogs as first-class objects with probe results and usage relationships.
- `/skills`, `/skills/:id`: lifecycle states and execution policy.
- `/automations`: trigger is automation-owned, not a standalone daily object.
- `/extensions`: intake, validate, review, import.
- `/knowledge`: inventory, export, session linkage.

### Governance / Signals / Ops

- `/ops/observability`: metrics, logs, traces, event profiling current capability and gaps.
- `/audit`: searchable and exportable audit chain.
- `/logs`: visible runtime log search page.
- `/outbox`: delivery rescue queue with copyable event id and folded error detail.
- `/ops`: repair console; dangerous actions are low-frequency, explicit-confirmation paths.

### Identity / Org / Docs

- `/identity`: IAM overview with auth posture and deep links.
- `/identity/providers`, `/identity/users`, `/identity/groups`, `/identity/roles`, `/identity/agent-roles`, `/identity/people`: registry and posture pages.
- `/org`: tenant, workspace, policy, quota/usage summary.
- `/docs`: documentation home.

### Compatibility Redirects

- `/dashboard -> /runtime`
- `/auth -> /identity/providers`
- `/users -> /identity/users`
- `/groups -> /identity/groups`
- `/roles -> /identity/roles`
- `/people -> /identity/people`

## Clickable Flow

The prototype uses hash navigation in `index.html`:

`#/login -> #/setup -> #/runtime-checks -> #/sessions -> #/sessions/:id -> #/executions/:id -> #/audit -> #/ops/observability -> #/resolved-summary`

Primary buttons on the critical path link to the next page. Sidebar navigation covers the full route inventory.

## Responsive Spec

### 1440px Desktop

- Fixed left navigation.
- Top toolbar with global search, theme switch, environment and on-call identity.
- Main content uses a 12-column grid.
- Detail pages use two columns where useful: main diagnosis/execution content plus evidence/timeline side rail.

### 390px mobile

The HTML includes a mobile breakpoint at `@media (max-width: 640px)` and labels the mobile target as 390px mobile. It covers the first and second priority pages plus Observability and Ops:

- `/login`
- `/setup`
- `/runtime-checks`
- `/runtime`
- `/sessions`
- `/sessions/:id`
- `/executions`
- `/executions/:id`
- `/chat`
- `/inbox`
- `/providers`
- `/channels`
- `/notification-templates`
- `/ops/observability`
- `/ops`

Mobile behavior:

- Sidebar becomes a compact horizontal drawer-like navigation strip.
- Multi-column detail pages become single flow.
- Primary actions move into a sticky bottom action bar.
- JSON, command, trace, and long IDs use wrapping with no page-level horizontal overflow.

## Data Model Notes

Mock data follows field names from `web/src/lib/api/types.ts`:

- `SessionDetail`, `SessionGoldenSummary`, `SessionListResponse`
- `ExecutionDetail`, `ExecutionGoldenSummary`, `ExecutionListResponse`
- `AuditTraceEntry`, `LogRecord`, `OutboxEvent`
- `OpsSummaryResponse`, `ObservabilityResponse`
- `RuntimeSetupStatus`, `SetupFeatures`
- `KnowledgeRecord`, `ToolPlanStep`

Unsupported prototype-only fields are labeled `[future data]`.

## Quality Gates And Guardrails

- Login does not make break-glass the default CTA.
- Provider loading state is explicit to cover the LoginView local_token race.
- Sessions and Executions include a console/API error state to cover smoke fatal error detection.
- Raw JSON, console output, transport detail, and full payload are folded by default.
- Secrets are never displayed in plaintext; secret fields show write-only or missing/set states.
- Approvals are never bypassed. High-risk execution requires approve/reject/request-context with reason.
- Inbox API failure is rendered as Error State, not empty messages.
- Channels use structured channel selection, not CSV input.
- Setup does not discuss Agent Role model binding.
- Automations own triggers as internal structure rather than making triggers a daily top-level object.

## Frontend Refactor Implementation Issues

These are the recommended implementation issues for follow-up engineering work.

1. **Shell, tokens, and shared states**  
   Implement App Shell, navigation grouping, IBM Plex font loading, light/dark theme variables, status chips, risk badges, empty/error/loading/degraded/disabled states.

2. **Runtime critical path workbenches**  
   Rework `/runtime`, `/sessions`, `/sessions/:id`, `/executions`, `/executions/:id` around conclusion, evidence, risk, next action, approval reason, and folded raw payloads.

3. **Auth, setup, and runtime checks**  
   Rework `/login`, `/setup`, `/runtime-checks`; fix LoginView local_token provider-loading race; keep break-glass folded and first-run setup focused.

4. **Registry pages and object detail pattern**  
   Apply shared registry table/card/detail drawer pattern to Providers, Channels, Notification Templates, Connectors, Skills, Automations, Extensions, Knowledge, Identity, Org.

5. **Governance, signals, Ops, and test hardening**  
   Rework Observability, Audit, Logs, Outbox, Ops; add smoke checks that register browser console and pageerror listeners; record React 19 / swagger-ui-react peer dependency baseline or isolate Swagger UI; fix web/.gitignore logs rule so `web/src/pages/logs/LogsPage.tsx` cannot be missed in a clean checkout.

## Verification

Run:

```bash
bash docs/design/prototypes/web-console/verify-prototype.sh
```

The script checks the design doc, HTML entry, full route coverage, theme toggle, component markers, responsive breakpoint, future data labels, compatibility redirect notes, and implementation issue notes.
