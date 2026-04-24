# EVI-15 SSH Credential Governance Tail

## Scope

- `rotation_required` is a hard block for SSH execution and connector health probes that would read credential material.
- Clearing `rotation_required` requires a credential material update that refreshes `LastRotatedAt`; `enable` cannot bypass rotation.
- `ops-token` / break-glass may use explicit approval endpoints, but cannot resolve SSH password/private-key material or bypass approval endpoints.
- Secrets Inventory and Ops surfaces expose custody configuration state, `key_id`, and SSH credential summary states without echoing raw `secret_ref` or plaintext secrets.

## Runtime Rules

- Fail closed on missing custody backend, missing/invalid secret material, stored `key_id` mismatch, `rotation_required`, `disabled`, and host-scope mismatch.
- `ExpiresAt` is enforced lazily at resolve time: expired credentials are auto-marked `rotation_required` before use.
- Best effort: when `TARS_SECRET_CUSTODY_KEY_ID` changes, operators should bulk mark SSH credentials `rotation_required` before re-uploading material.

## Break-Glass Boundary

- `ops-token` can call explicit `/executions/{id}/approve|reject|modify-approve|request-context` endpoints for emergency handling.
- Every break-glass approval action must write audit metadata including source, actor, action, and target execution.
- `ops-token` cannot call SSH material resolution paths, including direct `ssh_native` execute/health endpoints that would require custody reads.

## Inventory / UI

- Secrets Inventory shows custody configured/not configured, current `key_id`, and per-item statuses: `active`, `disabled`, `rotation_required`, `missing`, `invalid_secret_ref`, `custody_not_configured`.
- SSH credential list remains metadata-only: never echo password, private key, passphrase, or raw `secret_ref`.
- Rotation-required UI copy must direct operators to replace credential material rather than re-enable the credential.

## Docs / Future Boundary

- Admin docs must describe `TARS_SECRET_CUSTODY_KEY` and `TARS_SECRET_CUSTODY_KEY_ID` injection, rotation workflow, and break-glass approval audit expectations.
- Future Vault/KMS/BYOK integrations can reuse the custody status model, rotation policy, and audit boundary; only the secret backend implementation and key metadata source should change.
