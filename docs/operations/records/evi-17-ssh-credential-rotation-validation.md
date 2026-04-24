# EVI-17 SSH Credential Rotation Validation

- PR URL: pending
- Runtime validated commit SHA: `4cf0f4b4b92e6b9eaafd296ced55bdcccb57dd00`
- Evidence commit SHA or docs-only evidence commit SHA: pending
- Final PR head commit SHA: `4cf0f4b4b92e6b9eaafd296ced55bdcccb57dd00`
- Shared lab host: `192.168.3.100`
- Target SSH host: `192.168.3.9`
- Verification time (UTC): `2026-04-24T01:43:29Z`
- Verifier: DEV-opencode-gpt5.4

## Runtime Identity Or Config Path

- Live runtime root: `/root/tars-dev/team-shared` (PID `131949` cwd)
- Expected canonical shared-lab root per handbook: `/data/tars-setup-lab`
- Actual `shared-test.env` path used to resolve ops token: `/root/tars-dev/team-shared/shared-test.env`
- Actual `TARS_CONNECTORS_CONFIG_PATH`: `/root/tars-dev/team-shared/connectors.shared.yaml`
- Actual `TARS_SECRETS_CONFIG_PATH`: `/root/tars-dev/team-shared/secrets.shared.yaml`
- Actual `TARS_ACCESS_CONFIG_PATH`: `/root/tars-dev/team-shared/access.shared.yaml`

## Commands

```bash
make check-mvp

TARS_REMOTE_HOST=192.168.3.100 \
TARS_REMOTE_USER=root \
make deploy-sync

TARS_REMOTE_HOST=192.168.3.100 \
TARS_REMOTE_USER=root \
TARS_REMOTE_BASE_DIR=/root/tars-dev \
TARS_EVI17_AUTH_TYPE=password \
TARS_EVI17_PASSWORD='<redacted at runtime>' \
bash scripts/replay_ssh_credential_rotation_fixture.sh
```

If the live runtime is moved back to the documented canonical tree, the same replay can be run with:

```bash
TARS_REMOTE_BASE_DIR=/data/tars-setup-lab \
bash scripts/replay_ssh_credential_rotation_fixture.sh
```

## Result Summary

- Pre-rotation credential creation/update: PASS (`201` create, credential active, `host_scope=192.168.3.9`)
- `rotation_required` status set: PASS (`200`, execution immediately failed closed afterward)
- SSH execution hard block after rotation: PASS (`500`, body: `ssh credential is not active\nssh credential status is rotation_required`)
- Credential material update cleared status: PASS (`200`, credential returned to `active` and preserved `host_scope=192.168.3.9`)
- `last_rotated_at` refreshed: PASS (`2026-04-24T01:43:22.128979463Z` -> `2026-04-24T01:43:22.90878828Z`)
- Post-rotation SSH execution recovery: PASS (`200`, `output_preview=root`)

## Failure Logs Or Blockers

- First live replay before the fix showed `pre_execution=200` but `post_execution=500` with `ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain`. Root cause was the replay script rotating password fixtures to a random value that the target host did not know; the script now replays the supplied fixture material instead of inventing new remote state.
- Earlier local/shared failure before deploy was `open REPLACE_WITH_SSH_PRIVATE_KEY_PATH: no such file or directory`. Root cause was `RunWithCredential()` inheriting `TARS_SSH_PRIVATE_KEY_PATH` for password-only credentials; fixed in `internal/modules/action/ssh/executor.go` and covered by regression test.
- Shared runtime log path: `/root/tars-dev/team-shared/tars-dev.log`
- Replay summary path: `/tmp/evi17-summary.json`

## Fixture Lifecycle And Cleanup

- Fixture auth provider: temporary `local_token` provider created only for replay, then removed by auth-config restore unless explicitly preserved.
- Fixture SSH credential: `evi17-ssh-rotation` by default, deleted during cleanup unless explicitly preserved.
- Connector mutation: `ssh-main` host/username/credential_id patched for replay; resulting runtime config should be restored or re-synced after the run.
- Temporary root credential guidance: rotate or discard the root password/private key used for `192.168.3.9` after validation.
- Password replay note: the validation run reuses the provided fixture password to prove `rotation_required -> material update -> active` semantics without silently changing the actual remote host password. If operators want to validate a real remote password rotation, that must be coordinated as an explicit out-of-band host change and then replayed with the new material.

## Notes

- All secrets, passwords, and private-key material must remain redacted in this record.
- If the final head differs from the runtime validated commit only because of docs/evidence files, note that explicitly.
