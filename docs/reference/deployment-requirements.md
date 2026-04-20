# Deployment Requirements

This document outlines the hardware and software prerequisites for deploying the TARS platform.

## 1. Hardware Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 2 Cores (x86_64) | 4 Cores+ |
| Memory | 4 GB | 8 GB+ |
| Disk | 20 GB (SSD preferred) | 100 GB+ |

## 2. Operating System

- **Linux**: Ubuntu 22.04+, RHEL 8+, or equivalent.
- **Docker**: v24.0+ (Required for containerized deployment).
- **Docker Compose**: v2.20+ (Required for stack management).

## 3. Network Requirements

- **Inbound Ports**:
  - `8081`: Unified TARS HTTP entrypoint (Web Console / Ops API / Health / Webhooks).
- **Outbound Access**:
  - Access to configured LLM Provider API.
  - Access to Telegram Bot API (if using Telegram).
  - Connectivity to VictoriaMetrics/Prometheus.
  - SSH or API access to target hosts (via JumpServer).

## 4. Dependencies

- **PostgreSQL**: Version 15 or 16.
- **OpenAI-Compatible API**: Local (LM Studio, Ollama) or Cloud (OpenRouter, Gemini).
- **Telegram Bot Token**: Required for Slack-less communication.

## 5. Security Recommendations

- Deploy behind a reverse proxy (Nginx/Envoy) with TLS.
- Use a dedicated database user with minimal permissions.
- Keep port `8081` on internal networks only, or expose only the required webhook / Web paths through a reverse proxy.
