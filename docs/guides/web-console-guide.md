# Web Console User Guide

TARS Web Console is the central management interface for platform administrators and operators. It provides visibility into incident response sessions, execution logs, knowledge base management, and platform configuration.

## 1. Dashboard

The Dashboard provides a high-level overview of the platform health:
- **Metrics**: Real-time status of connected metrics providers.
- **Executions**: Summary of recent automation and manual actions.
- **Sessions**: Active and recent incident recovery sessions.

## 2. Session Management

The Sessions page lists all incident recovery workflows triggered by alerts or manual smoke tests.
- **Session Detail**: View the full trace of an incident, including AI diagnosis, tool plans, and execution results.
- **Audit Trail**: Every action taken by the platform or operators is logged for compliance.

## 3. Connector Center

Manage external integrations such as:
- **Metrics Connectors**: Prometheus, VictoriaMetrics.
- **Execution Connectors**: JumpServer, SSH.
- **Observation Connectors**: Log systems, APM.

## 4. Skill Registry

Skills are the "playbooks" that TARS uses to solve specific problems.
- **Drafting**: Create new skills using the built-in editor.
- **Version Control**: Promote drafts to production or rollback to previous versions.

## 5. Identity & Access

Manage who can access the platform and what they can do:
- **Auth Providers**: Configure OIDC, OAuth2, or local token access.
- **Users & Groups**: Manage local account and group memberships.
- **Roles & Permissions**: Define fine-grained RBAC policies.

## 6. Documentation Center

Access this guide and other technical references directly from the top-right menu.
- **Search**: Use `Cmd+K` to search across all built-in documents.
- **Offline Support**: The documentation is available even when the platform is disconnected from the internet.
