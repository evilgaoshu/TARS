# GitHub Actions Baseline Scope

> Baseline rule: GitHub-hosted runners are for validation only. Shared-host actions and deployment cutover stay out of scope until hardening is complete.

## Allowed Scope

| Layer | What runs here | Example |
| --- | --- | --- |
| L0 | format, lint, static checks | `go fmt`, shell lint, config validation |
| L1 | unit and integration tests that do not need shared infrastructure | `go test ./...` |
| L2 | repository smoke and MVP checks that stay inside the CI sandbox | `bash scripts/check_mvp.sh`, `bash scripts/ci/security-regression.sh` |
| Demo | static preview build with no runtime secrets | `bash scripts/ci/static-demo-build.sh` |

## Not Allowed Yet

- [ ] Any job that needs root SSH.
- [ ] Any job that depends on a persistent machine.
- [ ] Any job that copies secret-bearing configs to a remote host.
- [ ] Any deploy or live-validate step that touches shared infrastructure.
- [ ] Any test that only passes when a free public playground happens to be available.

## Minimum Workflow Rules

- [x] Pin every third-party action by commit SHA, not by floating tag.
- [x] Start with `contents: read` and add permissions only when a job needs them.
- [x] Keep secrets out of L0/L1/L2 unless a specific job absolutely needs one.
- [x] Use PR checks as the default gate for the baseline workflows.
- [x] Keep workflow names and job names aligned with the layer they run.
- [x] Fail closed if the workflow starts relying on a shared-machine assumption.
- [x] Keep demo preview builds static and repeatable; Supabase stays optional preview-only, never required for merge.

## Review Gate

Before enabling a new workflow, answer these questions:

- [x] Does it stay inside GitHub-hosted runners?
- [x] Does it avoid deployment or remote mutation?
- [x] Does it need secrets, and if so, are they scoped to the minimum possible boundary?
- [x] Would the workflow still be acceptable if the repo became public?

If the answer to any of these is no, the job does not belong in the baseline CI scope yet.
