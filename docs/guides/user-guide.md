# TARS User Guide

> **Version**: v1.0
> **Target Version**: TARS MVP (Phase 1)
> **Last Updated**: 2026-03-23

---

## 1. Product Introduction

### 1.1 What is TARS

TARS (Troubleshooting & Automated Response System) is an AIOps intelligent operations assistant oriented towards alert analysis and controlled execution.

### 1.2 Core Values

| Value | Description |
|------|------|
| Reduce MTTR | Automatically collect metrics and knowledge context, shortening the time from alert to diagnostic recommendation |
| Maintain Control | Command execution must pass manual approval, with complete and traceable auditing |
| Enable Reuse | Accumulate closed-loop records into knowledge for subsequent reuse |

### 1.3 Core Workflow

```
Alert Inbound → Auto Context Collection → AI Diagnostic Recommendation → Manual Approval → Controlled Execution → Result Feedback → Knowledge Accumulation
```

### 1.4 Applicable Scenarios

- **High-frequency Alert Handling**: Common alerts such as CPU, memory, disk, service unavailability, etc.
- **Standardized Fault Handling**: Fault scenarios with existing Runbooks or SOPs.
- **On-call Scenarios**: Quickly obtain diagnostic recommendations and execution context.

---

## 2. User Roles and Permissions

### 2.1 Role Definitions

| Role | Responsibility | Permissions |
|------|------|------|
| On-call SRE/Operations Engineer | Receive alerts, view diagnostic recommendations, initiate approval requests | View sessions, initiate execution requests |
| Approver | Perform manual approval for execution requests | Approve/Reject/Modify execution requests |
| Platform Administrator | Deploy system, configure models and channels, view operational status | System configuration, monitoring view |
| Knowledge Manager | Manage documentation and knowledge accumulation | Knowledge base management |

### 2.2 Approval Permissions

| Risk Level | Definition | Approval Requirement |
|----------|------|----------|
| `info` | Read-only commands | Single-person approval |
| `warning` | Recoverable write operations | Single-person approval + Command confirmation |
| `critical` | High-impact or irreversible operations | Dual-person approval |

---

## 3. Alert Handling Workflow

### 3.1 Workflow Overview

1. **Alert Reception**: VMAlert sends alerts to TARS.
2. **Automated Analysis**: TARS automatically collects metrics context and generates AI diagnostic recommendations.
3. **Manual Confirmation**: On-call personnel receive Telegram messages and view recommendations.
4. **Approval Execution**: If commands need to be executed, initiate an approval request.
5. **Execution Results**: After command execution is completed, results are fed back to the session.

### 3.2 Session State Description

| State | Description |
|------|------|
| `open` | Session just created, waiting for analysis |
| `analyzing` | AI diagnostic analysis in progress |
| `pending_approval` | Waiting for manual approval of execution request |
| `executing` | Command execution in progress |
| `verifying` | Execution completed, verifying recovery status |
| `resolved` | Session resolved |
| `failed` | Execution failed or timed out |

---

## 4. Telegram Interaction Guide

### 4.1 First-time Use

1. **Add Bot**: Search for and add the TARS Bot in Telegram.
2. **Get Chat ID**: Provide the Chat ID to the administrator to configure in the alert routing.
3. **Confirm Reception**: Wait for the administrator to send a test message to confirm the configuration is correct.

### 4.2 Receiving Diagnostic Messages

When an alert is triggered, you will receive a message in Telegram containing the following information:

- **Alert Title**: Alert summary information.
- **Alert Details**: Metric values, thresholds, duration, etc.
- **AI Diagnostic Recommendation**: Automatically generated analysis and suggested commands.
- **Related Context**: Associated metric query results.

### 4.3 Approval Operations

When a command needs to be executed, TARS sends an approval request message, including:

- Target host information.
- Risk level (info/warning/critical).
- Suggested command to execute.
- Execution reason description.
- Rollback tips.
- Approval time limit.

**Optional Operations**:

| Operation | Description |
|------|------|
| ✅ Approve Execution | Agree to execute the suggested command |
| ❌ Reject Execution | Reject this execution, return to analysis state |
| ✏️ Approve after Modification | Modify command content then approve execution |
| 🔄 Transfer to Others | Transfer the approval request to another approver |
| ❓ Request Supplementary Info | Request additional context information |

### 4.4 Dialogue Mode

During the session, you can:

1. **Ask Follow-up Questions**: Send messages to ask for more context.
2. **Check Status**: Request to view the current session status.
3. **Manual Takeover**: Explicitly state manual handling; AI stops automatic recommendations.

### 4.5 Common Commands

```
/status      - View current session status
/history     - View session history
/help        - Display help information
```

---

## 5. Web Console User Guide

### 5.1 Accessing Web Console

1. Open browser and visit `https://tars.example.com` (actual address provided by the administrator).
2. If authentication is required, enter the credentials assigned by the administrator.

### 5.2 Interface Navigation

The Web Console contains the following main functional areas:

- **Overview Dashboard**: Displays system status and key metrics.
- **Session List**: View all alert sessions.
- **Execution Records**: View command execution history.
- **Knowledge Base**: View accumulated knowledge records.
- **Audit Logs**: View operation audit records.
- **Configuration Management**: Administrator performs system configuration.

### 5.3 Viewing Session List

1. Click "Sessions" in the left navigation bar.
2. View the session list, including:
   - Session ID
   - Alert source and title
   - Current status
   - Creation time
   - Last updated time

3. Use filters to screen:
   - Screen by status (open/resolved/failed, etc.)
   - Screen by time range
   - Screen by alert source

The first screen now prioritizes the operator summary rather than raw IDs only:

- `golden_summary.headline`
- `golden_summary.conclusion`
- `golden_summary.risk`
- `golden_summary.next_action`

### 5.4 Viewing Execution Details

Click on a session to enter the details page:

- **Alert Info**: Original alert content.
- **Diagnostic Recommendations**: AI-generated analysis results.
- **Execution Requests**: All execution request records.
- **Execution Output**: Complete output of command execution.
- **Timeline**: Session event timeline.

The Session detail page now highlights two operator-first blocks at the top:

- `Golden Path Snapshot`: headline, conclusion, risk, next action, notification/execution summary.
- `Notification Reasons`: why TARS sent a diagnosis message or approval request.

Execution detail pages now also show `Execution Golden Path`, summarizing current action, approval status, result, and next step.

### 5.5 Runtime Checks Testing

Administrators can use the Web Console for system testing:

1. **Setup Status**: View the health status of various system components.
2. **Smoke Test**: Send test alerts to verify the pipeline.
   - Complete first-run setup if needed, then enter the "Runtime Checks" page.
   - Click "Send Test Alert".
   - Check if the Telegram message is received.
   - Verify if diagnostic recommendations are generated normally.

---

## 6. Typical Usage Scenarios

### 6.1 Scenario 1: CPU Alert Handling

**Scenario Description**: Receive an alert that CPU usage exceeds the threshold.

**Handling Workflow**:

1. Receive Telegram alert message, including:
   - Alert host: `prod-web-01`
   - Current CPU: 95%
   - Duration: 5 minutes

2. View AI diagnostic recommendations:
   - Suggest querying TOP processes.
   - Suggest viewing system load trends.
   - Suggest checking for OOM events.

3. Confirm recommendations are reasonable, click "Request Execution".

4. Receive approval request, view:
   - Command: `top -bn1 | head -20`
   - Risk level: info (read-only)
   - Target host: `prod-web-01`

5. Approve execution, wait for results.

6. View execution output, confirm high CPU process.

7. Decide on the next step based on the output.

In Web Console, the recommended reading order is:

1. Open `Sessions` and read the `headline / conclusion / next action` summary first.
2. Open the session detail and confirm `Notification Reasons`.
3. If an execution was created, open the execution card or execution detail and read `Execution Golden Path` before scanning raw output.

### 6.2 Scenario 2: Service Unavailability Alert

**Scenario Description**: Receive a service unavailability alert.

**Handling Workflow**:

1. Receive alert, including service name and instance information.

2. View AI-suggested diagnostic steps:
   - Check service status.
   - View service logs.
   - Check port listening status.

3. Execute read-only diagnostic commands in order.

4. Based on diagnostic results, decide if a service restart is needed.

5. If restart is needed, initiate a `systemctl restart` execution request.

6. Wait for approver approval (warning level requires approval).

7. Verify service recovery after execution completion.

### 6.3 Scenario 3: Manual Takeover

**Scenario Description**: AI cannot give effective recommendations, manual handling is needed.

**Handling Workflow**:

1. Receive alert, but AI recommendation shows "Requires manual judgment".

2. Click "Manual Takeover" or send `/manual` command.

3. TARS stops automatic recommendations, waiting for manual operation.

4. Log in to the target host via SSH or other means.

5. Manually troubleshoot the problem.

6. Record the handling result in Telegram or Web Console.

---

## 7. Frequently Asked Questions (FAQ)

### 7.1 Alert Reception Issues

**Q: Why am I not receiving alert messages?**

A: Please check the following:
- Confirm Chat ID is correctly configured in TARS.
- Confirm Telegram Bot Token is correctly configured.
- Check if VMAlert is sending webhooks to TARS.
- View Setup Status in Web Console to confirm the Telegram component's health.

**Q: Is the received alert information incomplete?**

A: TARS standardizes alerts; some original fields may be mapped to labels or annotations. Viewing session details in the Web Console shows complete original data.

### 7.2 AI Diagnostic Issues

**Q: Why did AI not give diagnostic recommendations?**

A: Possible reasons:
- Model service is unavailable or timed out.
- Desensitized information is insufficient to generate recommendations.
- No matching knowledge documents for this alert type.
- TARS is currently in diagnosis_only mode.

**Q: What if the AI-suggested command is inappropriate?**

A: You can:
- Reject the current recommendation and re-analyze.
- Select "Approve after Modification" to adjust command content.
- Use manual takeover mode.

### 7.3 Approval Execution Issues

**Q: Who was the approval request sent to?**

A: Based on approval routing configuration, it defaults to routing by service owner or on-call group. The approval message will show the current approver source.

**Q: What if approval times out?**

A: Default approval timeout is 15 minutes. After timeout, the escalator will be notified. If extension is needed, new approval routes can be configured.

**Q: Why does my approval request require dual-person approval?**

A: Critical level commands require dual-person approval by default. This is a security policy; please contact the second approver.

### 7.4 Execution Failure Issues

**Q: Command execution failed, how to troubleshoot?**

A: View execution details in the Web Console:
- Check error messages.
- Confirm if SSH connection is normal.
- Verify if command syntax is correct.
- View target host logs.

**Q: Is the execution output truncated?**

A: TARS has a size limit for execution output (default 256KB); parts exceeding this will be truncated. If complete output is needed, please view it directly on the target host.

### 7.5 Security Related Issues

**Q: Will my data be sent to external models?**

A: TARS performs desensitization before sending to external models, including:
- IP addresses, hostnames, domain names.
- Passwords, Tokens, Secrets.

Sensitive information will be replaced with placeholders.

**Q: Which commands can be executed?**

A: Executable commands are controlled by authorization policies, including:
- Whitelisted commands can be executed directly.
- Blacklisted commands are rejected.
- Other commands require approval.

Please contact the administrator for specific configurations.

### 7.6 Other Issues

**Q: How to view historical alert records?**

A: Log in to the Web Console, enter the "Sessions" page, and filter historical records by time range.

**Q: How to export audit logs?**

A: Administrators can export audit logs via Ops API or Web Console.

**Q: Which alert sources does the system support?**

A: The MVP version supports VMAlert Webhook. Subsequent versions will support more alert sources.

---

## 8. Feedback and Support

### 8.1 Problem Feedback

If you encounter problems, please provide the following information:
- Session ID
- Time of occurrence
- Specific operation steps
- Screenshot of error messages

### 8.2 Feature Suggestions

Welcome to provide feature suggestions through the following channels:
- Contact the administrator via Telegram.
- Submit feedback through the Web Console.

## 9. Official Replay Entry

If the team needs a stable demo or acceptance path for the alert closed loop, administrators may run:

```bash
bash scripts/run_golden_path_replay.sh
```

This official replay uses fixed fixtures and is intended to demonstrate:

- alert ingestion
- session creation
- diagnosis summary
- approval callback
- execution and verification progression
- operator-facing golden summaries in the Web Console

---

*This document applies to the TARS MVP version; features in subsequent versions may be adjusted.*
