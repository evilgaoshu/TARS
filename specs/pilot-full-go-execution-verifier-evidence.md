# EVI-19 Pilot Full-Go Execution / Verifier Evidence Spec

## Goal

Collect a full shared-lab evidence path on `192.168.3.100` that closes the EVI-13 gap from approval-path evidence to `execution -> verifier -> resolved`.

## Runtime Choice

Primary runtime is `jumpserver-main`:

- `deploy/team-shared/connectors.shared.yaml` sets `jumpserver-main.config.values.base_url` to `http://192.168.3.100`.
- JumpServer credentials are supplied only through `connector/jumpserver-main/access_key` and `connector/jumpserver-main/secret_key` in the shared secrets file.
- The target asset is `192.168.3.9 dev`.

Controlled fallback is `ssh-main` when JumpServer cannot execute commands because of JumpServer permissions, API version behavior, network, or credential failures:

- `ssh-main` points to `192.168.3.9`, user `root`, and SSH Credential Custody ID `evi19-ssh-root-3-9`.
- `TARS_SSH_ALLOWED_HOSTS` includes `192.168.3.100,127.0.0.1,192.168.3.9`.
- Any fallback run must record the JumpServer failure reason before collecting ssh_native full-go evidence.

## Required Path

Use either a runtime-checks/test-alert path or a Telegram/web-chat path, but the evidence must include these ordered states:

1. Session created on `192.168.3.100` with target host `192.168.3.9`.
2. AI diagnosis produces an execution hint.
3. Authorization decides either `require_approval` or an equivalent approval-gated action.
4. Approval is accepted through the product approval endpoint or Telegram callback.
5. Execution runs on the selected runtime.
6. Verifier runs against the execution result and reports success.
7. Final session state is `resolved`.

## Evidence Record Requirements

Create `docs/operations/records/evi-19-full-go-evidence-YYYYMMDD.md` with:

- PR/head commit and deployed runtime identity.
- Runtime identity: connector ID, protocol, execution mode, and fallback reason when applicable.
- Session ID and execution ID.
- Approval result and approval mechanism.
- Execution output summary, exit code, spool/output reference, and log location.
- Verification result, verifier command or basis, and final session state.
- Outbox/notification state.
- Commands used for deployment, replay, and evidence extraction.
- GitHub CI/checks status and shared-lab validation status as separate sections.

## Replay Commands

The canonical replay remains:

```sh
export TARS_REMOTE_HOST=192.168.3.100
export TARS_REMOTE_USER=root
export TARS_REMOTE_BASE_DIR=/data/tars-setup-lab
export TARS_REMOTE_SHARED_DIR=/data/tars-setup-lab/team-shared
export TARS_OPS_BASE_URL=http://192.168.3.100:8081
TOKEN="$(source scripts/lib/shared_ops_token.sh && shared_ops_token_resolve)"
export TARS_OPS_API_TOKEN="$TOKEN"

TARS_GOLDEN_AUTO_APPROVE=1 \
TARS_GOLDEN_ALERT_FIXTURE=/tmp/evi19-full-go-alert.json \
TARS_GOLDEN_POLL_SECONDS=240 \
bash scripts/run_golden_path_replay.sh
```

For ssh_native fallback, first record a failed `jumpserver-main` execution attempt and the resulting connector health detail, then run the same replay after `ssh-main` is the selected healthy execution runtime.

## Acceptance

EVI-19 is not complete unless the PR contains this spec, the evidence record, updated MVP/checklist docs, and a shared-lab record proving successful execution, verifier success, and final `resolved`, or an explicit accepted blocker with exact failed command and log evidence.
