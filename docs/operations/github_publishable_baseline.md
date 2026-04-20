# GitHub Publishable Baseline Checklist

> Use this as the last local review before the first GitHub-tracked baseline. It tells us what should be visible in GitHub, what must stay local, and which commands to run before the first commit.

## Safe To Track Now

- Repository source and behavior: `api/`, `cmd/`, `configs/`, `internal/`, `migrations/`, `project/`, `scripts/`, `specs/`, `web/`
- Documentation and repo metadata: `docs/`, `.github/`, `README.md`, `CHANGELOG.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `Makefile`, `go.mod`, `go.sum`
- Deployment and template-safe assets: checked-in Dockerfiles, workflow definitions, example configs, `deploy/team-shared/shared-test.env` with placeholder values, and non-secret docs under `deploy/`

## Must Stay Local Or Ignored

- Agent and browser state: `.codex-tmp/`, `.claude/`, `.gemini/`, `.playwright-cli/`, `.superpowers/`, `.alma/`
- Local screenshots and review artifacts: `screenshots/`
- Local shared-test secrets: `deploy/team-shared/*.local.env`, `deploy/team-shared/*.private.env`
- Build and cache output: `/bin/`, `/build/`, `/dist/`, `/tmp/`, `web/node_modules/`, `web/dist/`, `web/playwright-report/`, `web/test-results/`
- Runtime noise and local data: `*.log`, `.env`, `.env.*`, `*.db`, `*.sqlite`, `*.sqlite3`

## Human Review Commands

Run these locally before the first commit:

```bash
git status --short
git ls-files --others --exclude-standard
git check-ignore -v \
  .codex-tmp/batch1-review/dashboard-initial.png \
  .gemini/settings.json \
  .playwright-cli/page-2026-03-29T14-11-14-103Z.png \
  .superpowers/brainstorm/86471-1774617819/state/server.pid \
  screenshots/dashboard.png \
  deploy/team-shared/shared-test.local.env
git diff -- .gitignore docs/operations/github_publishable_baseline.md docs/operations/github_migration_prep_runbook.md
```

Machine gate note:

- `make secret-scan` now scans the publishable non-test tree, including `docs/`, `scripts/`, `project/`, `specs/`, `web/`, and root repo metadata.
- To keep the scan high-signal, it intentionally excludes test fixtures plus historical archives under `docs/reports/` and `docs/operations/records/`; those still require human review before the first push.

## Push Gate

- Do not push until the separate secret worker has acknowledged scan and rotation status.
- If any file under `deploy/team-shared` still contains live secrets, keep it in a `.local.env` / `.private.env` file or convert it to a checked-in placeholder template before the first GitHub baseline.
