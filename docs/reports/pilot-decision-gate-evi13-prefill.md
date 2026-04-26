# Pilot Decision Gate Prefill For EVI-13

- Date: `2026-04-23`
- Evidence source: `docs/operations/records/evi-13-alert-e2e-evidence-20260423.md`
- Scope: owner-approved PR #8 closeout scope on `192.168.3.100`

## Pilot Input Snapshot

| Field | Value |
| --- | --- |
| 试点团队 | shared-lab internal validation |
| 值班角色 | DEV-opencode-gpt5.4 acting as operator/verifier |
| 试点周期 | `2026-04-23` one-day evidence refresh |
| 目标告警类型 | `Evi13Manual` smoke alert, Telegram conversational diagnostics |
| 主要渠道 | Telegram webhook ingress, Ops API, shared web runtime |
| 核心成功标准 | fresh `192.168.3.100` evidence for diagnosis/approval/execute feasibility |
| 样本量 | 1 alert-driven smoke sample + 2 Telegram webhook samples |

## Required Evidence Fields

| 证据项 | 当前填写 | 说明 |
| --- | --- | --- |
| 首个可行动判断时间 | concrete evidence: session `e24d6484-a27c-4b3b-a73b-c165e7f76807` reached `execution_draft_ready` and `approval_message_prepared` | This prefill records the actionable approval point qualitatively. Exact timestamp remains available in the session trace API/log rather than pasted here. |
| 建议采纳率 | limited sample: 1 owner-approved conversational sample reached accepted approval | This is not a pilot-rate metric yet; it only shows the single closeout sample advanced through approval acceptance. |
| 审批通过率 | `1/1` in the owner-approved closeout sample | Session `e24d6484-a27c-4b3b-a73b-c165e7f76807` includes `approval_accepted` for execution `249ba0b4-1c1f-4bde-9fb5-341b07aecbc0`. |
| 一周主动复用率 | blocker: one-day lab refresh only | Requires a real pilot period, not a one-day lab refresh. |
| Knowledge 命中率 | blocker: no human usefulness labeling | Current evidence pack did not include a labeled human usefulness judgement for retrieved knowledge. |
| Outbox replay / dead-letter 率 | point-in-time snapshot only | This closeout focuses on approval-path evidence. No failed/blocked outbox issue was used as a blocker in the refreshed shared-lab sample. |
| 操作员额外负担 | concrete evidence: owner/DEV intervention still needed for execution runtime | Approval-path evidence is now reproducible, but full execution/verifier still depends on the degraded `jumpserver-main` runtime. |

## Evidence Summary

- Shared-lab health/readiness: pass
- Shared-lab provider/channel baseline: pass for approval-path evidence
- Shared-lab config snapshot: captured
- Telegram missing-host guidance: pass after fixing live SSH allowlist baseline
- Explicit-host Telegram session creation: pass
- Explicit-host approval path: pass through `approval_accepted`
- Full execution/verifier closure: intentionally skipped for PR #8 closeout

## Component Prefill

### Knowledge

- Current conclusion: `Defer`
- Why:
  - Current EVI-13 samples did not prove operator-visible value from knowledge in the pilot path.
  - The run was dominated by channel/provider baseline problems, not by knowledge-driven decision support.
  - No human usefulness labels or repeated reuse data were collected.

### Vector

- Current conclusion: `Defer`
- Why:
  - Knowledge value itself is not yet proven in this evidence run.
  - There is no EVI-13 evidence showing vector recall produced a measurable delta over lexical/default evidence collection.

### Outbox

- Current conclusion: `Hold`
- Why:
  - This closeout is no longer dominated by Telegram/provider placeholder drift.
  - Outbox did not surface as the limiting factor in the refreshed approval-path sample.
  - The current evidence still does not justify deeper outbox investment.

## Go / No-Go Readout

### Go inputs that are now stronger

- Shared-lab config drift around `TARS_SSH_ALLOWED_HOSTS` is fixed in the repo and verified live.
- Shared runtime health and core live-validation scripts are green.
- Telegram conversational missing-host behavior now matches the intended multi-host guidance path.
- Explicit-host conversational requests now reach `execution_draft_ready` and `approval_accepted` on `192.168.3.100`.
- The evidence pack now includes concrete `session_id`, `execution_id`, and approval-path trace evidence.

### Remaining full-go blockers

- `jumpserver-main` API health is now configured, but command job submission is still denied by JumpServer with `403 permission_denied` / `命令执行已禁用`.
- EVI-19 captured a controlled `ssh-main` fallback full-go sample on `192.168.3.100 -> 192.168.3.9`: session `811a30f4-8e3e-4ccc-8989-8af485a25c38`, execution `0f46f347-29c5-49b4-b01d-23d41ebb1253`, verifier `success`, final state `resolved`.
- Remaining primary-path blocker: JumpServer command execution must be enabled before claiming JumpServer primary full-go success.

### PR #8 closeout disposition

- Owner-approved closeout scope: acceptable
- Why:
  - `execution_draft_ready` and `approval_accepted` are now stable on `192.168.3.100`
  - SSH / JumpServer execution was explicitly skipped by owner instruction for this closeout
  - the evidence pack records the skipped runtime blocker rather than silently omitting it

## Recommended Next Capture

1. Keep PR #8 evidence tied to the owner-approved EVI-13 scope exception.
2. Treat EVI-19 as the first full-go fallback closure: `ssh-main` produced successful execution, verifier success, and final `resolved`.
3. For JumpServer primary-path go/no-go, enable JumpServer command execution for the supplied AK/SK and target asset, then rerun the same approval/execution/verifier capture without fallback.

## Decision Table

| 能力 | 结论 | 说明 |
| --- | --- | --- |
| Knowledge | Defer | No pilot-quality reuse evidence collected in this run. |
| Vector | Defer | No vector-specific value signal yet. |
| Outbox | Hold | Current reliability snapshot is acceptable, but not enough to justify deeper scope. |
