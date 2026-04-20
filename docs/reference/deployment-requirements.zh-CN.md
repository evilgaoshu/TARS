# 部署要求

本文档概述了部署 TARS 平台的硬件和软件先决条件。

## 1. 硬件要求

| 组件 | 最低配置 | 推荐配置 |
|-----------|---------|-------------|
| CPU | 2 核 (x86_64) | 4 核及以上 |
| 内存 | 4 GB | 8 GB 及以上 |
| 磁盘 | 20 GB (推荐 SSD) | 100 GB 及以上 |

## 2. 操作系统

- **Linux**: Ubuntu 22.04+、RHEL 8+ 或同等系统。
- **Docker**: v24.0+ (容器化部署必需)。
- **Docker Compose**: v2.20+ (堆栈管理必需)。

## 3. 网络要求

- **入站端口**:
  - `8081`: TARS 统一 HTTP 入口 (Web 控制台 / Ops API / 健康检查 / Webhooks)。
- **出站访问**:
  - 访问已配置的 LLM Provider API。
  - 访问 Telegram Bot API (如果使用 Telegram)。
  - 与 VictoriaMetrics/Prometheus 的连通性。
  - 对目标主机的 SSH 或 API 访问 (通过 JumpServer)。

## 4. 依赖项

- **PostgreSQL**: 版本 15 或 16。
- **兼容 OpenAI 的 API**: 本地 (LM Studio、Ollama) 或云端 (OpenRouter、Gemini)。
- **Telegram Bot Token**: 在没有 Slack 的通信中必需。

## 5. 安全建议

- 部署在反向代理 (Nginx/Envoy) 之后并启用 TLS。
- 使用具有最小权限的专用数据库用户。
- 尽量把 `8081` 仅暴露在内部网络；如需对外开放，请通过反向代理只暴露必要的 webhook / Web 路径。
