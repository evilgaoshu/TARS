# Free Environment Usage Boundaries

> Free environments are for demo and smoke validation only. They are not the shared integration environment and they are not a substitute for hardening.

## Allowed Uses

| Environment | Allowed use | Good fit for |
| --- | --- | --- |
| GitHub Actions | CI validation and repository checks | L0, L1, L2 |
| Cloudflare Pages | frontend and demo previews | public preview URLs, UI smoke |
| Supabase Free | disposable demo data or narrow experiments | short-lived Postgres-backed demos |
| VictoriaMetrics Play | manual exploration and demo validation | screenshots, walkthroughs, query demos |

## Not Allowed On The Free Path

- [ ] Full TARS backend hosting.
- [ ] Shared integration workflows.
- [ ] Long-lived credentials or root SSH access.
- [ ] Automated tests that depend on public playground stability.
- [ ] Any path that needs persistent state to stay correct.
- [ ] Any environment that silently masks secret leakage or host coupling.

## Guardrails

- [ ] Treat preview data as disposable.
- [ ] Treat demo URLs as public unless access control is explicitly added.
- [ ] Prefer `noindex` or equivalent protections for preview surfaces when supported.
- [ ] Do not store production-like secrets in free environments.
- [ ] If a free service outage breaks the flow, that flow does not qualify as a baseline dependency.

## Boundary Test

Ask this before allowing a flow onto a free environment:

- [ ] Can the flow be recreated from scratch without hidden state?
- [ ] Can we replace the service with a local mock and still validate the same behavior?
- [ ] Would we be comfortable exposing the same surface to a broader audience?

If any answer is no, keep the flow on controlled infrastructure for now.
