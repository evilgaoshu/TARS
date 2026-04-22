# TARS 共享开发测试机运行手册（192.168.3.100）

> 适用环境：当前团队共享开发测试机 `192.168.3.100`  
> 最近校验时间：2026-04-08  
> 目的：给新加入开发测试的同学或 agent 一份“按这台机器当前真实状态操作”的短手册。

## 1. 当前环境事实

- 主机：`root@192.168.3.100`
- 架构：`amd64`
- TARS 根目录：`/data/tars-setup-lab`
- 统一入口：`http://192.168.3.100:8081`
- 当前 `/setup` 语义：
  - 未初始化时：first-run wizard
  - 初始化完成后：运行态不再继续停留在 `/setup`，而是转到 `/runtime-checks`
- 当前共享 lab 已支持重置回首跑状态；最近一次 reset 后，系统会从 `/setup` 开始。
- 共享观测栈：
  - `VictoriaMetrics`：`http://127.0.0.1:8428`
  - `VictoriaLogs`：`http://127.0.0.1:9428`
- 当前共享 connector 基线要求 `victoriametrics-main / victorialogs-main` 默认指向这两个本机地址，而不是外部 demo 地址。

目录布局：

- 二进制：`/data/tars-setup-lab/bin/tars-linux-amd64-dev`
- Web dist：`/data/tars-setup-lab/web-dist`
- 共享配置：`/data/tars-setup-lab/team-shared`
- 执行输出：`/data/tars-setup-lab/execution-output`
- 本地数据目录：`/data/tars-setup-lab/data`
- Postgres 容器数据：`/data/tars-setup-lab/postgres`
- reset 备份：`/data/tars-setup-lab/reset-backups`

## 2. 当前配置落点

当前这台机器的 `shared-test.env` 关键项如下：

- `TARS_SERVER_LISTEN=0.0.0.0:8081`
- `TARS_WEB_DIST_DIR=/data/tars-setup-lab/web-dist`
- `TARS_POSTGRES_DSN=postgres://tars:tars@127.0.0.1:5432/tars?sslmode=disable`
- `TARS_PROVIDERS_CONFIG_PATH=/data/tars-setup-lab/team-shared/providers.shared.yaml`
- `TARS_CONNECTORS_CONFIG_PATH=/data/tars-setup-lab/team-shared/connectors.shared.yaml`
- `TARS_SKILLS_CONFIG_PATH=/data/tars-setup-lab/team-shared/skills.shared.yaml`
- `TARS_AUTOMATIONS_CONFIG_PATH=/data/tars-setup-lab/team-shared/automations.shared.yaml`
- `TARS_SECRETS_CONFIG_PATH=/data/tars-setup-lab/team-shared/secrets.shared.yaml`
- `TARS_ACCESS_CONFIG_PATH=/data/tars-setup-lab/team-shared/access.shared.yaml`

说明：

- 当前共享 lab 要求 `Postgres` 可用，不建议在这台机器上跑 runtime config 的内存 fallback。
- 真实凭据不应写回仓库；仓库里提交的是 `deploy/team-shared/shared-test.env.example` 模板，部署脚本会在同步时将其物化为 `shared-test.env` 并与远端 canonical 值合并。

## 3. 当前推荐部署命令

从本地仓库根目录执行：

```sh
export TARS_REMOTE_HOST=192.168.3.100
export TARS_REMOTE_USER=root
export TARS_REMOTE_BASE_DIR=/data/tars-setup-lab
# 可选：仅当你要临时覆盖共享 token 时才显式设置
# export TARS_OPS_API_TOKEN=<local-secret>

bash scripts/deploy_team_shared.sh
```

这会做：

- 自动构建 `linux/amd64` 二进制
- 构建 `web/dist`
- 同步 `deploy/team-shared/*.yaml`、marketplace 和 fixtures 脚本到 `/data/tars-setup-lab/team-shared`
- 同步 `shared-test.env` 时，会先从仓库里的 `shared-test.env.example` 生成模板，再保留远端真实 secret / placeholder 覆盖项，同时继续把仓库里的 host/path 等模板化字段同步到共享机
- 同步二进制到 `/data/tars-setup-lab/bin/tars-linux-amd64-dev`
- 同步前端到 `/data/tars-setup-lab/web-dist`
- 重启 TARS
- 运行 readiness / remote smoke / live validate / golden scenarios

当前 `live-validate` 已包含显式的共享观测联调验证：

- `victoriametrics-main / victorialogs-main` 的 health + 真实 query
- 临时 connector 的 `create / update / probe / health / query`
- 验证结束后自动清理临时 connector，避免长期污染共享环境

补充约定：

- `192.168.3.100` 的共享 `Ops API token` 以远端 `/data/tars-setup-lab/team-shared/shared-test.env` 为唯一事实来源。
- 真实 token 不回写仓库；仓库里的 `deploy/team-shared/shared-test.env.example` 只保留 placeholder。
- 当前 `deploy_team_shared.sh / scripts/ci/smoke-remote.sh / scripts/ci/live-validate.sh / scripts/ci/web-smoke.sh` 会先归一化本地 token；如果本地值是空值或 placeholder，再通过 SSH 从这份远端 `shared-test.env` 自动解析 canonical token。
- `deploy_team_shared.sh` 会继续同步 `shared-test.env`，但对仍是 placeholder / 空值的条目会保留远端真实值；这样共享机上的 canonical token 与其它机密配置不会被仓库模板冲回占位值，同时非 secret 的 env 演进仍能随仓库推进。
- 如需临时覆盖共享 token，再在当前 shell 显式 `export TARS_OPS_API_TOKEN=...`。

如果只想同步并重启，不跑远端验证：

```sh
export TARS_REMOTE_HOST=192.168.3.100
export TARS_REMOTE_USER=root
export TARS_REMOTE_BASE_DIR=/data/tars-setup-lab
# 可选：仅当你要临时覆盖共享 token 时才显式设置
# export TARS_OPS_API_TOKEN=<local-secret>
export TARS_DEPLOY_SKIP_VALIDATE=1

bash scripts/deploy_team_shared.sh
```

如果只想手工启动远端二进制：

```sh
ssh root@192.168.3.100 "
  cd /data/tars-setup-lab &&
  set -a &&
  . /data/tars-setup-lab/team-shared/shared-test.env &&
  set +a &&
  nohup /data/tars-setup-lab/bin/tars-linux-amd64-dev \
    > /data/tars-setup-lab/team-shared/tars-dev.log 2>&1 < /dev/null &
"
```

重要约束：

- 不要手工执行裸命令 `nohup ./bin/tars-linux-amd64-dev`。
- 必须先 `source /data/tars-setup-lab/team-shared/shared-test.env`，再启动二进制。
- 如果跳过 `shared-test.env`，进程会退回默认 `./web/dist`、默认配置路径和默认 secret/runtime config 行为，常见表象是：
  - `GET /` 返回 `503`
  - `{"error":{"code":"web_ui_unavailable","message":"web ui index is not available"}}`
  - `/setup`、`/login`、前端静态资源全部不可用

推荐把“能启动”与“启动正确”区分开来。共享 lab 上只有“带 env 启动”才算正确。

## 4. 常用检查命令

健康检查：

```sh
ssh root@192.168.3.100 'curl -fsS http://127.0.0.1:8081/healthz'
ssh root@192.168.3.100 'curl -fsS http://127.0.0.1:8081/api/v1/bootstrap/status'
```

进程与日志：

```sh
ssh root@192.168.3.100 'pgrep -af tars-linux-amd64-dev'
ssh root@192.168.3.100 'tail -n 100 /data/tars-setup-lab/team-shared/tars-dev.log'
```

确认进程真的吃到了共享环境变量：

```sh
ssh root@192.168.3.100 '
  pid=$(ps -ef | awk "/\\/data\\/tars-setup-lab\\/bin\\/tars-linux-amd64-dev/{print \$2; exit}")
  tr "\0" "\n" < /proc/$pid/environ | grep "^TARS_WEB_DIST_DIR="
  tr "\0" "\n" < /proc/$pid/environ | grep "^TARS_POSTGRES_DSN="
'
```

预期至少能看到：

- `TARS_WEB_DIST_DIR=/data/tars-setup-lab/web-dist`
- `TARS_POSTGRES_DSN=...`

如果这里看不到变量，说明服务是“裸起”的，应该立刻按第 3 节重启。

查看当前配置文件：

```sh
ssh root@192.168.3.100 'sed -n "1,120p" /data/tars-setup-lab/team-shared/access.shared.yaml'
ssh root@192.168.3.100 'sed -n "1,120p" /data/tars-setup-lab/team-shared/providers.shared.yaml'
ssh root@192.168.3.100 'sed -n "1,160p" /data/tars-setup-lab/team-shared/connectors.shared.yaml'
```

## 5. 首跑重置（Setup Reset）

当前这台 lab 可以被重置回真正的 first-run wizard。重置时需要同时清空：

- `setup_state`
- `runtime_config_documents` 中的 `access/providers/connectors`
- `team-shared` 下对应 YAML
- connector lifecycle state 文件

如果只改 `setup_state`，系统会从已有 `access/providers/channels` 推断“已经初始化”，看起来像 reset 没生效。

重置后验收标准：

- `GET /api/v1/bootstrap/status` 返回：
  - `initialized=false`
  - `mode=wizard`
  - `next_step=admin`
- 访问 `/` 会跳到 `/setup`
- 访问 `/login` 也会跳到 `/setup`
- `connectors.shared.yaml` 中 `entries: []`

## 6. 另一位 agent 加入开发测试时的建议协作方式

建议分工：

- Agent A：本地开发与仓库改动
- Agent B：共享机部署与验收

最低协作约定：

- 部署前先说清楚是否会覆盖 `192.168.3.100`
- 如果要 reset setup 或 connectors，先在 `reset-backups/` 下做备份
- 不要把共享机上的真实状态反向提交到仓库
- 共享机只作为受控开发测试，不作为 GitHub Actions 的 required runtime

建议交接信息：

- 远端根目录：`/data/tars-setup-lab`
- 当前入口：`http://192.168.3.100:8081`
- 当前观测栈：`/data/tars-observability`
- TARS 主日志：`/data/tars-setup-lab/team-shared/tars-dev.log`
- 当前 reset 备份目录：`/data/tars-setup-lab/reset-backups`

## 7. 相关文档

- [团队开发与测试环境手册](/Users/yue/TARS/docs/operations/team_dev_test_environment.md)
- [团队共享开发测试包说明](/Users/yue/TARS/deploy/team-shared/README.md)
- [部署说明](/Users/yue/TARS/deploy/README.md)
- [本地观测栈记录](/Users/yue/TARS/docs/operations/records/local_observability_lab_2026-04-08.md)
