# Pilot Decision Gate Prefill For EVI-13

- Date: `2026-04-23`
- Evidence source: `docs/operations/records/evi-13-alert-e2e-evidence-20260423.md`
- Scope: fresh shared-lab evidence only

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
| 首个可行动判断时间 | blocker: no human-confirmed pilot timestamp | Session timelines show diagnosis completion timestamps, but this run did not capture a human-confirmed “first actionable judgement” moment. |
| 建议采纳率 | blocker: no approved execution sample | No real approval/execution sample was produced in this run, so adoption cannot be measured honestly. |
| 审批通过率 | blocker: no execution entered approval | No execution draft entered real approval in the EVI-13 runtime samples. |
| 一周主动复用率 | blocker: one-day lab refresh only | Requires a real pilot period, not a one-day lab refresh. |
| Knowledge 命中率 | blocker: no human usefulness labeling | Current evidence pack did not include a labeled human usefulness judgement for retrieved knowledge. |
| Outbox replay / dead-letter 率 | point-in-time snapshot: `failed_outbox=0`, `blocked_outbox=0` | Useful runtime evidence, but not yet a multi-day pilot rate. |
| 操作员额外负担 | concrete evidence: DEV intervention required | DEV had to fix shared baseline config and still hit live Telegram/provider credential blockers. |

## Evidence Summary

- Shared-lab health/readiness: pass
- Shared-lab smoke and live validation: pass
- Shared-lab config snapshot: captured
- Telegram missing-host guidance: pass after fixing live SSH allowlist baseline
- Explicit-host Telegram session creation: pass
- Full approval/execution/verifier closure: blocked

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
  - Point-in-time shared-lab hygiene showed `failed_outbox=0` and `blocked_outbox=0`.
  - The channel still degraded to Telegram `stub`, but that is a credential/config issue rather than evidence that outbox needs more platform surface.
  - This run does not justify deeper outbox investment yet.

## Go / No-Go Readout

### Go inputs that are now stronger

- Shared-lab config drift around `TARS_SSH_ALLOWED_HOSTS` is fixed in the repo and verified live.
- Shared runtime health, connector health, and core live-validation scripts are green.
- Telegram conversational missing-host behavior now matches the intended multi-host guidance path.

### Remaining no-go blockers

- Telegram bot token on `192.168.3.100` is still placeholder-backed, so approval/delivery is only `stub`.
- Reasoning provider baseline is not trustworthy enough to claim a real execution-planning closure.
- Required end-to-end evidence fields remain empty:
  - `execution_id`
  - approval outcome
  - verifier success
  - spool/output path evidence

## Recommended Next Capture

1. Replace the live Telegram bot token placeholder on `192.168.3.100` and verify `telegram.last_result` becomes a real success state.
2. Repair one reasoning path on `192.168.3.100`
   - restore `lmstudio-local` reachability, or
   - provide a valid assist credential and prove fresh success
3. Re-run one execution-seeking Telegram request and one smoke alert until the session reaches:
   - `pending_approval`
   - approved
   - executing
   - verifying
   - resolved
4. Record the resulting `execution_id`, verifier status, spool path, and Telegram approval evidence in a follow-up record.

## Decision Table

| 能力 | 结论 | 说明 |
| --- | --- | --- |
| Knowledge | Defer | No pilot-quality reuse evidence collected in this run. |
| Vector | Defer | No vector-specific value signal yet. |
| Outbox | Hold | Current reliability snapshot is acceptable, but not enough to justify deeper scope. |
