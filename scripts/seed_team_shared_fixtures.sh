#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="${1:-/root/tars-team-shared/fixtures}"
OBS_LOG="${BASE_DIR}/observability-main.log"
OBS_HTTP_DIR="${BASE_DIR}/observability-http/api/v1"
DELIVERY_REPO="${BASE_DIR}/delivery-main-repo"
MARKER_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
HOST_FILE_LOG="${TARS_HOST_FILE_LOG_PATH:-/var/log/tars-observability-test.log}"

mkdir -p "${BASE_DIR}"
mkdir -p "${OBS_HTTP_DIR}"

cat > "${OBS_LOG}" <<'EOF'
{"time":"2026-03-20T13:18:02Z","level":"INFO","service":"api","component":"gateway","message":"api release 2026.03.20-1 deployed to canary","release":"2026.03.20-1","host":"192.168.3.106"}
{"time":"2026-03-20T13:21:14Z","level":"WARN","service":"api","component":"gateway","message":"api p95 latency elevated after release 2026.03.20-1","release":"2026.03.20-1","host":"192.168.3.106","latency_ms":821}
{"time":"2026-03-20T13:24:31Z","level":"ERROR","service":"api","component":"orders","message":"api request failed with database timeout","release":"2026.03.20-1","trace_id":"trc-001","host":"192.168.3.106","endpoint":"/v1/orders","error":"database timeout"}
{"time":"2026-03-20T13:24:48Z","level":"ERROR","service":"api","component":"orders","message":"api request failed with upstream reset","release":"2026.03.20-1","trace_id":"trc-002","host":"192.168.3.106","endpoint":"/v1/orders","error":"upstream reset by peer"}
{"time":"2026-03-20T13:25:10Z","level":"ERROR","service":"api","component":"payments","message":"api request failed with redis timeout","release":"2026.03.20-1","trace_id":"trc-003","host":"192.168.3.106","endpoint":"/v1/payments","error":"redis timeout"}
{"time":"2026-03-20T13:27:02Z","level":"INFO","service":"api","component":"deploy","message":"rollback of api release 2026.03.20-1 started","release":"2026.03.20-1","host":"192.168.3.106"}
{"time":"2026-03-20T13:29:44Z","level":"INFO","service":"api","component":"deploy","message":"api release 2026.03.20-1 rollback completed","release":"2026.03.20-1","host":"192.168.3.106"}
EOF
printf '{"time":"%s","level":"INFO","service":"api","component":"observability","message":"tars-observability-host-file-test host=192.168.3.9 ts=%s","host":"192.168.3.9","job":"host_file"}\n' "${MARKER_TS}" "${MARKER_TS}" >> "${OBS_LOG}"
printf 'tars-observability-host-file-test host=192.168.3.9 ts=%s\n' "${MARKER_TS}" >> "${HOST_FILE_LOG}"

cat > "${OBS_HTTP_DIR}/alerts" <<'EOF'
{
  "status": "success",
  "data": {
    "alerts": [
      {
        "state": "firing",
        "labels": {
          "alertname": "DiskSpaceLow",
          "service": "api",
          "severity": "warning",
          "instance": "192.168.3.9"
        },
        "annotations": {
          "summary": "disk usage high on api host"
        },
        "value": 91
      },
      {
        "state": "firing",
        "labels": {
          "alertname": "ApiLatencyHigh",
          "service": "api",
          "severity": "warning",
          "instance": "192.168.3.9"
        },
        "annotations": {
          "summary": "api p95 latency elevated after release 2026.03.20-1"
        },
        "value": 821
      }
    ]
  }
}
EOF

cat > "${OBS_HTTP_DIR}/rules" <<'EOF'
{
  "status": "success",
  "data": {
    "groups": [
      {
        "name": "shared-team-rules",
        "file": "/etc/vmalert/rules/api.yaml",
        "rules": [
          {
            "name": "DiskSpaceLow",
            "state": "firing",
            "query": "node_filesystem_avail_bytes / node_filesystem_size_bytes < 0.1",
            "duration": 300
          },
          {
            "name": "ApiLatencyHigh",
            "state": "firing",
            "query": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 0.8",
            "duration": 300
          }
        ]
      }
    ]
  }
}
EOF

rm -rf "${DELIVERY_REPO}"
mkdir -p "${DELIVERY_REPO}"
git -C "${DELIVERY_REPO}" init -q
git -C "${DELIVERY_REPO}" config user.name "TARS Shared Fixtures"
git -C "${DELIVERY_REPO}" config user.email "tars-fixtures@example.test"
git -C "${DELIVERY_REPO}" checkout -q -B main

cat > "${DELIVERY_REPO}/README.md" <<'EOF'
# Delivery Fixture Repo

This repository is used by the shared TARS test environment to provide
deterministic delivery facts for `delivery.query`.
EOF

git -C "${DELIVERY_REPO}" add README.md
GIT_AUTHOR_DATE="2026-03-20T13:05:00Z" \
GIT_COMMITTER_DATE="2026-03-20T13:05:00Z" \
  git -C "${DELIVERY_REPO}" commit -q -m "api: baseline stable release"

mkdir -p "${DELIVERY_REPO}/deploy"
cat > "${DELIVERY_REPO}/deploy/release-notes.md" <<'EOF'
- api release 2026.03.20-1
- change: enable new orders retry policy
- scope: api
EOF
git -C "${DELIVERY_REPO}" add deploy/release-notes.md
GIT_AUTHOR_DATE="2026-03-20T13:18:00Z" \
GIT_COMMITTER_DATE="2026-03-20T13:18:00Z" \
  git -C "${DELIVERY_REPO}" commit -q -m "api: deploy release 2026.03.20-1"

cat > "${DELIVERY_REPO}/deploy/hotfix.txt" <<'EOF'
api rollback notes for release 2026.03.20-1
EOF
git -C "${DELIVERY_REPO}" add deploy/hotfix.txt
GIT_AUTHOR_DATE="2026-03-20T13:29:00Z" \
GIT_COMMITTER_DATE="2026-03-20T13:29:00Z" \
  git -C "${DELIVERY_REPO}" commit -q -m "api: rollback release 2026.03.20-1"

printf 'seeded fixtures under %s\n' "${BASE_DIR}"
