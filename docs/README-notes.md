# README Notes

This page holds deeper links that do not need to live on the repository front page.

## Operational Follow-ups

- Current priority workstreams: [operations/current_high_priority_workstreams.md](./operations/current_high_priority_workstreams.md)
- Operations index: [operations/README.md](./operations/README.md)
- Deployment guide: [guides/deployment-guide.md](./guides/deployment-guide.md)
- Team development and test environment: [operations/team_dev_test_environment.md](./operations/team_dev_test_environment.md)

## Validation Commands

The root README keeps the first-run path short. If you need the broader validation set, start with:

```sh
make web-install
make secret-scan
make pre-check
make security-regression
make check-mvp
cd web && npm run test:unit
make static-demo-build
```

## Local Container Path

For a minimal local container run, the repository includes `deploy/docker/docker-compose.yml`.

```sh
export TARS_OPS_API_TOKEN=change-me
docker compose -f deploy/docker/docker-compose.yml up --build
```

## Configuration and Secrets

Use the committed `.example` files as templates. Do not commit real tokens, private keys, database passwords, `.env` files, build artifacts, or local runtime state.

Relevant template locations:

- `configs/*.example.yaml`
- `deploy/pilot/*.example`
- `deploy/team-shared/shared-test.env.example`
- `deploy/team-shared/secrets.shared.yaml.example`
