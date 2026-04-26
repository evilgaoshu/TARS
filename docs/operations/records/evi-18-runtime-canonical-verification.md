# EVI-18 Runtime Canonical Verification

Date: 2026-04-26

## Scope

Verify and harden the shared-lab runtime identity for `192.168.3.100`.

## Expected Canonical Path

- Canonical runtime root: `/data/tars-setup-lab`
- Shared config path: `/data/tars-setup-lab/team-shared`
- Shared env path: `/data/tars-setup-lab/team-shared/shared-test.env`
- Managed service: `tars-shared-lab.service` (systemd)

## Runtime Identity

Pending final shared-host verification in this branch.

- PID: pending
- Binary: pending
- CWD / workdir: pending
- `TARS_DIR`: pending
- `shared-test.env`: pending
- Runtime git head: pending
- PR/head commit: pending
- systemd unit evidence: pending

## Commands And Results

Pending final execution:

```sh
TARS_REMOTE_HOST=192.168.3.100 \
TARS_REMOTE_USER=root \
bash scripts/deploy_team_shared.sh

TARS_SHARED_LAB_EXPECTED_GIT_HEAD="$(git rev-parse HEAD)" \
ssh root@192.168.3.100 \
  "TARS_SHARED_LAB_EXPECTED_GIT_HEAD=${TARS_SHARED_LAB_EXPECTED_GIT_HEAD} bash -s" \
  < scripts/check-shared-lab.sh
```

## Current Status

Not yet final. This record will be updated with the actual `192.168.3.100` command output before delivery, or with a blocker if shared-host access/deploy/verification cannot complete.
