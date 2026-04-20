# GitHub Migration And Free Test Environment Research

Date: 2026-04-02

## Executive Summary

TARS should move toward a GitHub-first development model, but not by pushing the current tree as-is.

The immediate blockers are local and concrete:

- the repo does not yet have tracked history
- `deploy/team-shared` historically carried real secrets and addresses, so any surviving copy or history must still be treated as rotation-required
- current shared-environment scripts assume a persistent machine, root SSH, and long-lived tokens

The best free-first operating model for the next phase is:

- GitHub for source control and CI
- GitHub Actions for L0/L1/L2 validation
- Cloudflare Pages for frontend/demo previews only
- Supabase free only for lightweight demo or disposable test data, not as the main shared integration environment
- VictoriaMetrics public play environments for demos and manual query validation, not as automated test dependencies

For the near term, the full TARS backend and shared integration flow should stay on a controlled machine until secrets, host coupling, and deployment assumptions are cleaned up.

Planning assumption update: the primary local dev/test host is `192.168.3.100` on AMD64, and GitHub should serve as the demo environment through GitHub-hosted CI plus a static demo surface unless a separate backend host is later chosen.

## Current-State Inventory

Already GitHub-ready:

- Basic CI exists in [mvp-checks.yml](/Users/yue/TARS/.github/workflows/mvp-checks.yml).
- A layered CI direction already exists in [ci-layered.yml](/Users/yue/TARS/.github/workflows/ci-layered.yml).
- Local strict checks are already scriptable via [pre-check.sh](/Users/yue/TARS/scripts/ci/pre-check.sh), [full-check.sh](/Users/yue/TARS/scripts/ci/full-check.sh), [security-regression.sh](/Users/yue/TARS/scripts/ci/security-regression.sh), and [check_mvp.sh](/Users/yue/TARS/scripts/check_mvp.sh).
- Secret indirection patterns already exist in [providers.shared.yaml](/Users/yue/TARS/deploy/team-shared/providers.shared.yaml) and [connectors.shared.yaml](/Users/yue/TARS/deploy/team-shared/connectors.shared.yaml).

Not GitHub-first yet:

- The repo has no tracked baseline yet. Local inspection showed `git rev-parse --verify HEAD` failing and the tree effectively behaving like an uncommitted workspace.
- Shared-environment flows depend on a persistent machine and hardcoded environment defaults in [smoke-remote.sh](/Users/yue/TARS/scripts/ci/smoke-remote.sh), [live-validate.sh](/Users/yue/TARS/scripts/ci/live-validate.sh), [web-smoke.sh](/Users/yue/TARS/scripts/ci/web-smoke.sh), and [deploy_team_shared.sh](/Users/yue/TARS/scripts/deploy_team_shared.sh).
- The current `team-shared` package should now be treated as template-safe in publishable docs, but the historical notes in [README.md](/Users/yue/TARS/deploy/team-shared/README.md) still matter because any older copies of real credentials or addresses must be rotated before reuse.

## Security Findings

Critical:

- Historically, real secrets were checked into [shared-test.env](/Users/yue/TARS/deploy/team-shared/shared-test.env), [secrets.shared.yaml](/Users/yue/TARS/deploy/team-shared/secrets.shared.yaml), [access.shared.yaml](/Users/yue/TARS/deploy/team-shared/access.shared.yaml), and [dex.config.yaml](/Users/yue/TARS/deploy/team-shared/dex.config.yaml); any surviving copies should still be treated as rotation-required.
- The `team-shared` docs already say these values must be rotated before wider sharing.

High:

- Shared deployment now requires explicit `TARS_REMOTE_USER` and keeps host-key checking enabled in the checked-in `team-shared` template, but any old local copy that still used `root` SSH or disabled host-key checking should be replaced.
- [deploy_team_shared.sh](/Users/yue/TARS/scripts/deploy_team_shared.sh) now requires explicit `TARS_OPS_API_TOKEN`; keep that token injected from the operator environment or a secret manager, never from the checked-in tree.

Medium:

- Example config still shows inline provider credentials in [providers.example.yaml](/Users/yue/TARS/deploy/pilot/providers.example.yaml), which conflicts with the repo's own secret-ref direction.
- Current GitHub workflows are pinned by SHA in the baseline; keep that rule for future workflow additions.

## Free-First Option Assessment

| Option | Fit | Use Now | Notes |
| --- | --- | --- | --- |
| GitHub Actions | High for CI | Yes | Good fit for repo checks and layered validation. Public repos get free/unlimited GitHub-hosted runners; private repos get quota-based usage. |
| Cloudflare Pages | High for frontend previews | Yes | Strong fit for public preview URLs and demo web builds. Preview deployments are public by default but can be protected with Access. |
| Cloudflare Workers | Low for full TARS backend | Not for core backend | Useful only for small edge helpers or preview glue. Inference: not a fit for TARS's long-running Go backend and worker model. |
| Vercel Hobby | Medium for personal/public demos | Maybe later | Useful for quick frontend previews, but a poor fit if the repo is private inside a GitHub org. |
| Supabase Free | Medium for lightweight demos | Yes, narrowly | Good for disposable/demo Postgres-backed experiments. Not enough for the long-lived shared integration environment. |
| VictoriaMetrics Play | Demo-only | Yes, for demos only | Good for manual exploration and UI validation. Not suitable as an automated test dependency. |

## Official-Source Notes

GitHub:

- GitHub-hosted runners are ephemeral per job: [GitHub Docs](https://docs.github.com/actions/using-github-hosted-runners/about-github-hosted-runners)
- Billing and free usage depend on public/private repo status: [GitHub Docs](https://docs.github.com/en/billing/managing-billing-for-github-actions/about-billing-for-github-actions)
- Environments and environment secrets are the right deployment boundary, but GitHub Free limits them for private/internal repos: [GitHub Docs](https://docs.github.com/actions/reference/workflows-and-actions/deployments-and-environments) and [GitHub Docs](https://docs.github.com/en/actions/configuring-and-managing-workflows/creating-and-storing-encrypted-secrets)

Cloudflare:

- Pages preview deployments are public by default and support `noindex`: [Cloudflare Docs](https://developers.cloudflare.com/pages/configuration/preview-deployments/)
- Pages/Workers limits: [Cloudflare Pages Limits](https://developers.cloudflare.com/pages/platform/limits/) and [Cloudflare Workers Limits](https://developers.cloudflare.com/workers/platform/limits/)

Vercel:

- Hobby limits and Git-deployment constraints: [Vercel Hobby](https://vercel.com/docs/accounts/plans/hobby), [Vercel Limits](https://vercel.com/docs/limits/overview), [Vercel Git Deployments](https://vercel.com/docs/deployments/git)

Supabase:

- Free-plan quotas and security guidance: [Billing](https://supabase.com/docs/guides/platform/billing-on-supabase), [Database Size](https://supabase.com/docs/guides/database/database-size), [RLS Hardening](https://supabase.com/docs/guides/database/hardening-data-api), [Secure Data](https://supabase.com/docs/guides/database/secure-data)

VictoriaMetrics:

- Public playgrounds are documented as playground/demo environments: [VictoriaMetrics Playground](https://docs.victoriametrics.com/playgrounds/victoriametrics/), [VictoriaLogs Playground](https://docs.victoriametrics.com/playgrounds/victorialogs/), [VictoriaTraces Docs](https://docs.victoriametrics.com/victoriatraces/)

## Recommended Target Operating Model

### Source Control And PR Model

- Move active development to GitHub only after creating a clean tracked baseline.
- Use branch + PR flow for all feature work.
- Keep `main` always releasable.

### CI Layering

Run on GitHub-hosted runners:

- L0: formatting, lint, static checks
- L1: `go test ./...`
- L2: `bash scripts/check_mvp.sh`

Keep manual or shared-machine only for now:

- L3: shared integration validation that depends on live connectors, shared host access, or protected credentials

### Environments And Secrets

- Move runtime secrets out of the repo.
- Store CI secrets in GitHub secrets or GitHub environments where supported by plan/repo visibility.
- For anything more sensitive than CI API tokens, prefer an external secret manager or machine-local secret injection.
- Keep the GitHub demo surface secret-free; if a value must exist only for local validation, keep it on the local host, not in GitHub Actions.

### What Stays On Shared Infra For Now

- Full backend integration testing
- Shared-host smoke validation
- Live connector validation against privileged providers
- Anything that requires persistent SSH or internal network access

### What Can Move Early

- Source control and PR workflow
- Unit/integration CI that does not require privileged shared-host access
- Frontend preview builds
- Demo-only observability validation against public playgrounds

## Recommended Next Step

Do these in order:

1. Create the initial tracked Git history.
2. Sanitize and template `deploy/team-shared` before any push to GitHub.
3. Rotate Telegram, Ops, OIDC, model, and provider credentials currently present in the tree.
4. Pin GitHub Actions by SHA and add explicit workflow permissions.
5. Keep GitHub-hosted CI scoped to L0/L1/L2 first.
6. Treat the current shared-machine flow as protected/manual until host coupling is removed.
7. Use Cloudflare Pages for frontend/demo previews if preview URLs are needed before full platform migration.

## What Should Not Be Migrated Yet

- `deploy/team-shared` as currently written
- any workflow that assumes `root` SSH and disabled host key checking
- full shared-environment deployment to free preview platforms
- automated tests that depend on VictoriaMetrics public playgrounds being stable

## Decision

Recommended now:

- GitHub for source control and CI
- GitHub Actions for core validation
- Cloudflare Pages for preview/demo UI
- Supabase free for narrow demo-only experiments

Recommended later:

- Cloudflare Workers or other edge runtime helpers
- more formal preview environments for non-frontend surfaces

Rejected for current phase:

- using Vercel Hobby as the main home for the TARS backend
- treating VictoriaMetrics playgrounds as test infrastructure
- migrating the current shared machine workflow to GitHub without first removing checked-in secrets
