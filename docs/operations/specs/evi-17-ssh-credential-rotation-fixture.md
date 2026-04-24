# EVI-17 SSH Credential Rotation Fixture

## Goal

Provide a repeatable shared-lab fixture and runbook for replaying the SSH credential rotation chain on `192.168.3.100` against target host `192.168.3.9` without writing root secrets or private keys into the repository, logs, or evidence records.

## Scope

- Use the existing shared-lab deployment and canonical credential injection flow.
- Create or update a temporary SSH credential fixture through `/api/v1/ssh-credentials`.
- Mark that fixture `rotation_required`.
- Confirm SSH execution is hard-blocked while the credential is in `rotation_required`.
- Update credential material and confirm:
  - status returns to `active`
  - `last_rotated_at` refreshes
  - SSH execution resumes
- Record runtime identity, commands, results, and cleanup steps under `docs/operations/records`.

## Non-Goals

- No Vault, KMS, BYOK, or external custody changes.
- No GitHub required-check changes.
- No change to the EVI-15 product contract for break-glass, governance semantics, or UI behavior.

## Runtime Constraints

- The shared lab verification entrypoint remains `192.168.3.100`.
- The SSH target host for this fixture is `192.168.3.9`.
- The live runtime root must be recorded exactly as observed. At the time of this spec update, the running process may still be serving from `/root/tars-dev`; evidence must report the actual runtime identity instead of assuming `/data/tars-setup-lab`.
- Root passwords, private keys, passphrases, and raw bearer secrets must stay out of repo files, logs, comments, screenshots, and evidence markdown.

## Fixture Design

### Inputs

The replay script accepts fixture material via environment variables only:

- `TARS_EVI17_AUTH_TYPE=password|private_key`
- `TARS_EVI17_PASSWORD` for password fixtures
- `TARS_EVI17_PRIVATE_KEY_PATH` and optional `TARS_EVI17_PASSPHRASE` for private-key fixtures
- `TARS_REMOTE_HOST`, `TARS_REMOTE_USER`, `TARS_REMOTE_BASE_DIR`, and optional `TARS_OPS_API_TOKEN`

The script resolves the shared ops token through `scripts/lib/shared_ops_token.sh` so it can follow the same `team-shared/shared-test.env` flow used by the rest of the shared-lab tooling.

For password fixtures, the replay updates the credential with the provided password material and preserves `host_scope` so the validation proves the product state transition without implicitly changing the actual remote host password on `192.168.3.9`.

### Temporary Auth Path

`ssh_native` intentionally rejects the break-glass ops token. To replay the credential lifecycle through a non-breakglass principal, the script creates a temporary `local_token` auth provider, logs in through that provider, and uses the returned session token for the SSH execution and health endpoints.

The provider is fixture-only and must be removed or the auth config restored at the end of the run unless the operator explicitly sets `TARS_EVI17_PRESERVE_FIXTURE=1` for debugging.

### Connector Mutation

The script patches the live `ssh-main` connector config so that:

- `host = 192.168.3.9`
- `username = root`
- `credential_id = evi17-ssh-rotation` (or override)

This keeps the replay inside the existing SSH connector semantics instead of adding a second fixture connector.

## Replay Sequence

1. Capture live auth config and connector config snapshots.
2. Create the temporary `local_token` auth provider.
3. Log in and obtain a non-breakglass session token.
4. Patch `ssh-main` to point at the fixture target and credential ID.
5. Create or update the SSH credential fixture.
6. Run a pre-rotation execution against `192.168.3.9`.
7. Mark the credential `rotation_required`.
8. Re-run execution and confirm it fails closed.
9. Update credential material.
   For password fixtures this means replaying the supplied password material and preserving `host_scope`; rotating the real host password is intentionally out of scope for this automation.
10. Re-read credential metadata and confirm `status=active` plus refreshed `last_rotated_at`.
11. Re-run execution and confirm recovery.
12. Clean up the SSH credential fixture and restore auth config unless fixture preservation is explicitly requested.

## Expected Failure Signal

The replay uses the live execution endpoint as the hard-block proof.

- Successful pre-rotation execution is expected to return `200`.
- After `rotation_required`, execution is expected to fail non-200 because the SSH credential cannot be resolved for `ssh_native`.
- After material update, execution is expected to return `200` again.

The exact error envelope may come back as an `internal_error` wrapper in the live connector execution endpoint. Evidence should record the returned status code and body without rewriting it.

## Cleanup And Lifecycle

### Default cleanup

- Delete the temporary SSH credential fixture.
- Restore the original auth config snapshot.
- Leave no persistent plaintext credential material in repo or docs.

### Re-run model

The script is designed to be idempotent enough for repeated runs:

- It can update an existing fixture credential instead of requiring a pristine lab.
- It restores auth config from the captured snapshot.
- It can preserve the fixture only when explicitly requested for debugging.

### Post-validation credential hygiene

Any temporary root password or private key used to reach `192.168.3.9` should be rotated or discarded after verification. The evidence record must state that recommendation even if the actual secret lifecycle is handled outside the repo.

## Verification Requirements

For each run, the evidence record must include:

- PR URL or explicit local-only status
- runtime validated commit SHA / head SHA
- actual runtime identity (`/proc/<pid>/environ`-derived paths or equivalent)
- exact commands run
- status/result summary for pre-rotation, blocked execution, and post-rotation recovery
- cleanup path
- blocker details if the lab is unreachable or credentials are unavailable

## Script Entry Point

- `scripts/replay_ssh_credential_rotation_fixture.sh`
- `make replay-ssh-rotation-fixture`

This entry point is best-effort automation for EVI-17 and future shared-lab SSH credential rotation replays.
