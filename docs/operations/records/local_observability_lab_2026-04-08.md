# Local Observability Lab Runbook (2026-04-08)

## Scope

This runbook records the local AMD64 lab observability stack for TARS development testing.

- Central host: `root@192.168.3.100`
- Central stack path: `/data/tars-observability`
- Secondary host: `root@192.168.3.9`
- Goal: collect metrics and logs from `192.168.3.100` and `192.168.3.9` into VictoriaMetrics and VictoriaLogs.

## Central stack on `192.168.3.100`

The central Docker Compose stack lives under `/data/tars-observability`.

| Service | Container | Purpose | Port |
| --- | --- | --- | --- |
| VictoriaMetrics | `tars-victoriametrics` | Metrics storage/query | `8428` |
| VictoriaLogs | `tars-victorialogs` | Logs storage/query | `9428` |
| vmagent | `tars-vmagent` | Scrapes `192.168.3.100:9100` and `192.168.3.9:9100` | internal |
| node-exporter | `tars-node-exporter` | `192.168.3.100` host metrics | `9100` |
| promtail | `tars-promtail` | `192.168.3.100` host/docker logs to VictoriaLogs | internal |

Key files:

- `/data/tars-observability/docker-compose.yml`
- `/data/tars-observability/vmagent.yml`
- `/data/tars-observability/promtail.yml`

## Secondary host `192.168.3.9`

`192.168.3.9` already had `node-exporter` running and reachable on `:9100`. Promtail is managed separately at `/opt/promtail` and forwards logs to the central VictoriaLogs endpoint:

- Compose path: `/opt/promtail/docker-compose.yml`
- Config path: `/opt/promtail/promtail-config.yml`
- Backup before host-file scrape change: `/opt/promtail/promtail-config.yml.bak-host-files-20260408`
- VictoriaLogs client URL: `http://192.168.3.100:9428/insert/loki/api/v1/push`

The `grafana/promtail:latest` image on this host does not include systemd journal support, so journald scraping is not the current verified path. The working fallback is a file scrape job:

```yaml
- job_name: host_files
  static_configs:
    - targets:
        - localhost
      labels:
        job: host_file
        host: 192.168.3.9
        __path__: /var/log/*.log
```

The verified sample file is `/var/log/tars-observability-test.log`.

## Verification

VictoriaMetrics queries on `192.168.3.100`:

```bash
curl -fsS "http://127.0.0.1:8428/api/v1/query" \
  --data-urlencode 'query=up{job="node_3_100"}'

curl -fsS "http://127.0.0.1:8428/api/v1/query" \
  --data-urlencode 'query=up{job="node_3_9"}'
```

Both returned `1` on 2026-04-08.

VictoriaLogs queries on `192.168.3.100`:

```bash
curl -fsS "http://127.0.0.1:9428/select/logsql/query" \
  --data-urlencode 'query=host:* | stats by (host, job) count()' \
  --data-urlencode 'limit=20'

curl -fsS "http://127.0.0.1:9428/select/logsql/query" \
  --data-urlencode 'query=tars-observability-host-file-test' \
  --data-urlencode 'limit=2'
```

The stats query returned logs for:

- `192.168.3.100`: `auth`, `cloud-init`, `cloud-init-output`, `syslog`, `kern`, `bootstrap`, `alternatives`, `dpkg`, `docker`
- `192.168.3.9`: `host_file`, `docker`

The marker query returned:

```text
tars-observability-host-file-test host=192.168.3.9 ts=2026-04-08T09:42:31+08:00
```

## Current caveats

- `192.168.3.9` journald scraping is not enabled because the current promtail image reports that journal support is not compiled in.
- If full journald coverage becomes necessary, switch `192.168.3.9` to a collector with systemd journal support, for example Vector or Fluent Bit.
- GitHub Actions must not SSH into this lab stack. This remains a local controlled test environment.
