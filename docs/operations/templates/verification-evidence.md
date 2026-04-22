# Shared Lab Verification Evidence Template

- PR URL:
- Head commit SHA:
- Shared lab host: `192.168.3.100`
- Canonical runtime root: `/data/tars-setup-lab`
- Verification time (UTC):
- Verifier:

## Runtime Identity Check Output

Paste the full `bash scripts/check-shared-lab.sh` output here.

```text
hostname:
timestamp_utc:
check.listener_8081:
check.binary_path:
check.workdir_path:
check.config_paths:
check.shared_env_file:
check.auth_login_local_token:
check.setup_status_endpoint:
check.session_url:
overall:
```

## Auth And Session Endpoint Status

- `POST /api/v1/auth/login` (`local_token`):
- `GET /api/v1/setup/status`:
- Session URL checked:
- Session URL status:

## Browser Evidence

- Desktop 1440px screenshot path or PR attachment:
- Mobile 390px screenshot path or PR attachment:

## Notes / Blockers

-
