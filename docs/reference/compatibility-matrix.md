# Compatibility Matrix

This document defines the verified versions and support levels for third-party systems integrated with TARS.

## 1. Metrics Connectors

| System | Supported Versions | Status | Notes |
|--------|-------------------|--------|-------|
| VictoriaMetrics | v1.x | Verified | Recommended for high-performance metrics. |
| Prometheus | v2.x | Supported | Fully compatible with Prometheus API. |

## 2. Execution Connectors

| System | Supported Versions | Status | Notes |
|--------|-------------------|--------|-------|
| JumpServer | v3.x, v4.x | Verified | Supports Job execution API. |
| OpenSSH | Any modern version | Supported | Direct SSH execution via key or password. |

## 3. Messaging Channels

| System | Capabilities | Status | Notes |
|--------|--------------|--------|-------|
| Telegram | Full | Verified | Direct bot communication and inline buttons. |
| Web Chat | Limited | Experimental | Internal console messaging. |

## 4. Identity Providers

| System | Protocol | Status | Notes |
|--------|----------|--------|-------|
| Google | OIDC | Verified | |
| GitHub | OAuth2 | Supported | |
| Generic OIDC | OIDC | Supported | Any standard-compliant provider. |

## 5. Database Requirements

| System | Min Version | Status | Notes |
|--------|-------------|--------|-------|
| PostgreSQL | 15+ | Verified | Primary workflow store. |
| SQLite | 3.x | Verified | Used for local vector index (sqlite-vec). |
