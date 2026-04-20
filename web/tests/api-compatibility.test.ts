import { beforeEach, describe, expect, it, vi } from "vitest";

const getMock = vi.fn();
const postMock = vi.fn();
const putMock = vi.fn();

vi.mock("../src/lib/api/client", () => ({
  api: {
    get: getMock,
    post: postMock,
    put: putMock,
  },
}));

describe("API compatibility normalization", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
    putMock.mockReset();
  });

  it("normalizes trigger channel_id from legacy channel", async () => {
    getMock.mockResolvedValueOnce({
      data: {
        id: "trg-1",
        tenant_id: "default",
        display_name: "Ops inbox rule",
        description: "",
        enabled: true,
        event_type: "incident.session.updated",
        channel: "inbox-primary",
        governance: "advanced_review",
        template_id: "diagnosis-zh-cn",
        cooldown_sec: 30,
        created_at: "2026-03-28T00:00:00Z",
        updated_at: "2026-03-28T00:00:00Z",
      },
    });

    const { getTrigger } = await import("../src/lib/api/triggers");
    const trigger = await getTrigger("trg-1");

    expect(trigger.channel_id).toBe("inbox-primary");
    expect(trigger.channel).toBe("inbox-primary");
    expect(trigger.governance).toBe("advanced_review");
  });

  it("normalizes agent role model_binding from structured API payloads", async () => {
    getMock.mockResolvedValueOnce({
      data: {
        role_id: "diagnosis",
        display_name: "Diagnosis",
        status: "enabled",
        is_builtin: false,
        profile: { system_prompt: "diagnose" },
        capability_binding: { mode: "unrestricted" },
        policy_binding: {
          max_risk_level: "warning",
          max_action: "require_approval",
        },
        model_binding: {
          primary: {
            provider_id: "openai-main",
            model: "gpt-4.1-mini",
          },
          fallback: {
            provider_id: "openai-backup",
            model: "gpt-4o-mini",
          },
          inherit_platform_default: false,
        },
        created_at: "2026-03-28T00:00:00Z",
        updated_at: "2026-03-28T00:00:00Z",
      },
    });

    const { fetchAgentRole } = await import("../src/lib/api/agent-roles");
    const role = await fetchAgentRole("diagnosis");

    expect(role.status).toBe("active");
    expect(role.model_binding).toEqual({
      primary: {
        provider_id: "openai-main",
        model: "gpt-4.1-mini",
      },
      fallback: {
        provider_id: "openai-backup",
        model: "gpt-4o-mini",
      },
      inherit_platform_default: false,
    });
  });

  it("serializes agent role model_binding without provider_preference mirrors", async () => {
    putMock.mockImplementationOnce(async (_url: string, body: unknown) => ({
      data: body,
    }));

    const { updateAgentRole } = await import("../src/lib/api/agent-roles");
    await updateAgentRole("diagnosis", {
      role_id: "diagnosis",
      model_binding: {
        primary: {
          provider_id: "openai-main",
          model: "gpt-4.1-mini",
        },
        fallback: {
          provider_id: "openai-backup",
          model: "gpt-4o-mini",
        },
        inherit_platform_default: false,
      },
    });

    const payload = putMock.mock.calls[0]?.[1] as Record<string, unknown>;
    expect(payload.model_binding).toEqual({
      primary: {
        provider_id: "openai-main",
        model: "gpt-4.1-mini",
      },
      fallback: {
        provider_id: "openai-backup",
        model: "gpt-4o-mini",
      },
      inherit_platform_default: false,
    });
    expect(payload).not.toHaveProperty("provider_preference");
  });

  it("preserves structured inherit_platform_default flags", async () => {
    getMock.mockResolvedValueOnce({
      data: {
        role_id: "diagnosis",
        display_name: "Diagnosis",
        status: "active",
        is_builtin: false,
        profile: { system_prompt: "diagnose" },
        capability_binding: { mode: "unrestricted" },
        policy_binding: {
          max_risk_level: "warning",
          max_action: "require_approval",
        },
        model_binding: {
          inherit_platform_default: true,
        },
        created_at: "2026-03-28T00:00:00Z",
        updated_at: "2026-03-28T00:00:00Z",
      },
    });

    const { fetchAgentRole } = await import("../src/lib/api/agent-roles");
    const role = await fetchAgentRole("diagnosis");

    expect(role.model_binding).toEqual({
      inherit_platform_default: true,
    });
  });

  it("serializes channel writes with kind/usages and legacy mirrors", async () => {
    postMock.mockImplementationOnce(async (_url: string, body: unknown) => ({
      data: (body as { channel: unknown }).channel,
    }));

    const { createChannel } = await import("../src/lib/api/access");
    const saved = await createChannel({
      id: "inbox-primary",
      name: "Primary Inbox",
      kind: "in_app_inbox",
      target: "default",
      enabled: true,
      usages: ["approval", "notifications"],
    });

    expect(postMock).toHaveBeenCalledTimes(1);
    const payload = postMock.mock.calls[0]?.[1] as {
      channel: Record<string, unknown>;
    };
    expect(payload.channel.kind).toBe("in_app_inbox");
    expect(payload.channel.type).toBe("in_app_inbox");
    expect(payload.channel.usages).toEqual(["approval", "notifications"]);
    expect(payload.channel.capabilities).toEqual(["approval", "notifications"]);
    expect(saved.kind).toBe("in_app_inbox");
    expect(saved.type).toBe("in_app_inbox");
  });

  it("preserves distinct channel usages and capabilities on write", async () => {
    postMock.mockImplementationOnce(async (_url: string, body: unknown) => ({
      data: (body as { channel: unknown }).channel,
    }));

    const { createChannel } = await import("../src/lib/api/access");
    await createChannel({
      id: "web-chat-primary",
      name: "Web Chat Primary",
      kind: "web_chat",
      target: "default",
      enabled: true,
      usages: ["conversation_entry"],
      capabilities: ["supports_session_reply"],
    });

    const payload = postMock.mock.calls[0]?.[1] as {
      channel: Record<string, unknown>;
    };
    expect(payload.channel.usages).toEqual(["conversation_entry"]);
    expect(payload.channel.capabilities).toEqual(["supports_session_reply"]);
  });

  it("preserves distinct channel usages and capabilities on read", async () => {
    getMock.mockResolvedValueOnce({
      data: {
        id: "web-chat-primary",
        name: "Web Chat Primary",
        kind: "web_chat",
        type: "web_chat",
        target: "default",
        enabled: true,
        usages: ["conversation_entry"],
        capabilities: ["supports_session_reply"],
      },
    });

    const { fetchChannel } = await import("../src/lib/api/access");
    const channel = await fetchChannel("web-chat-primary");

    expect(channel.usages).toEqual(["conversation_entry"]);
    expect(channel.capabilities).toEqual(["supports_session_reply"]);
  });

  it("serializes trigger governance alongside legacy channel compatibility", async () => {
    postMock.mockImplementationOnce(async (_url: string, body: unknown) => ({
      data: body,
    }));

    const { upsertTrigger } = await import("../src/lib/api/triggers");
    const saved = await upsertTrigger({
      display_name: "Governed Rule",
      event_type: "on_execution_completed",
      channel_id: "inbox-primary",
      governance: "advanced_review",
      filter_expr: "severity == 'critical'",
      operator_reason: "Create governance rule",
    });

    const payload = postMock.mock.calls[0]?.[1] as Record<string, unknown>;
    expect(payload.channel_id).toBe("inbox-primary");
    expect(payload.channel).toBe("inbox-primary");
    expect(payload.governance).toBe("advanced_review");
    expect((saved as { governance?: string }).governance).toBe(
      "advanced_review",
    );
  });

  it("preserves trigger automation ownership on read and write", async () => {
    getMock.mockResolvedValueOnce({
      data: {
        id: "trg-automation",
        tenant_id: "default",
        display_name: "Automation-owned trigger",
        event_type: "on_execution_completed",
        channel_id: "inbox-primary",
        automation_job_id: "daily-health",
        enabled: true,
      },
    });

    const { getTrigger, upsertTrigger } = await import("../src/lib/api/triggers");
    const detail = await getTrigger("trg-automation");
    expect(detail.automation_job_id).toBe("daily-health");

    postMock.mockImplementationOnce(async (_url: string, body: unknown) => ({
      data: body,
    }));

    await upsertTrigger({
      display_name: "Automation-owned trigger",
      event_type: "on_execution_completed",
      channel_id: "inbox-primary",
      automation_job_id: "daily-health",
      operator_reason: "Create automation-owned trigger",
    });

    const payload = postMock.mock.calls.at(-1)?.[1] as Record<string, unknown>;
    expect(payload.automation_job_id).toBe("daily-health");
    expect((payload.trigger as Record<string, unknown>).automation_job_id).toBe("daily-health");
  });

  it("serializes automation governance_policy on write", async () => {
    postMock.mockImplementationOnce(async (_url: string, body: unknown) => ({
      data: (body as { job: unknown }).job,
    }));

    const { createAutomation } = await import("../src/lib/api/ops");
    const saved = await createAutomation({
      job: {
        id: "daily-health",
        display_name: "Daily Health",
        type: "skill",
        target_ref: "health-check",
        schedule: "@every 15m",
        enabled: true,
        governance_policy: "approval_required",
        skill: { skill_id: "health-check", context: {} },
      },
    });

    const payload = postMock.mock.calls[0]?.[1] as {
      job: Record<string, unknown>;
    };
    expect(payload.job.governance_policy).toBe("approval_required");
    expect((saved as { governance_policy?: string }).governance_policy).toBe(
      "approval_required",
    );
  });
});
