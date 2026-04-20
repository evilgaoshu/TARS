// @vitest-environment jsdom

import { act } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

const fetchAgentRolesMock = vi.fn();
const createAgentRoleMock = vi.fn();
const updateAgentRoleMock = vi.fn();
const deleteAgentRoleMock = vi.fn();
const setAgentRoleEnabledMock = vi.fn();

const notify = {
  success: vi.fn(),
  error: vi.fn(),
  warn: vi.fn(),
};

const i18n = {
  t: (key: string, fallbackOrParams?: string | Record<string, unknown>) => {
    if (typeof fallbackOrParams === "string") {
      return fallbackOrParams;
    }
    switch (key) {
      case "common.no":
        return "no";
      case "identity.agentRoles.detail.modelBindingProviderBadge":
        return `Provider: ${fallbackOrParams?.provider ?? "auto"}`;
      case "identity.agentRoles.detail.modelBindingModelBadge":
        return `Model: ${fallbackOrParams?.model ?? "n/a"}`;
      case "identity.agentRoles.detail.fallbackProviderBadge":
        return `Fallback Provider: ${fallbackOrParams?.provider ?? "none"}`;
      case "identity.agentRoles.detail.fallbackModelBadge":
        return `Fallback Model: ${fallbackOrParams?.model ?? "none"}`;
      case "identity.agentRoles.detail.inheritPlatformDefaultBadge":
        return `Inherit Platform Default: ${fallbackOrParams?.value ?? "no"}`;
      default:
        return key;
    }
  },
  lang: "en-US",
  setLang: vi.fn(),
};

vi.mock("../src/lib/api/agent-roles", () => ({
  fetchAgentRoles: fetchAgentRolesMock,
  createAgentRole: createAgentRoleMock,
  updateAgentRole: updateAgentRoleMock,
  deleteAgentRole: deleteAgentRoleMock,
  setAgentRoleEnabled: setAgentRoleEnabledMock,
}));

vi.mock("../src/hooks/ui/useNotify", () => ({
  useNotify: () => notify,
}));

vi.mock("../src/hooks/useI18n", () => ({
  useI18n: () => i18n,
}));

async function flush() {
  await act(async () => {
    await Promise.resolve();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

async function flushAll() {
  await flush();
  await flush();
  await flush();
}

function setInputValue(input: HTMLInputElement, value: string) {
  const descriptor = Object.getOwnPropertyDescriptor(
    HTMLInputElement.prototype,
    "value",
  );
  descriptor?.set?.call(input, value);
  input.dispatchEvent(new Event("input", { bubbles: true }));
  input.dispatchEvent(new Event("change", { bubbles: true }));
}

async function clickByText(container: HTMLElement, text: string) {
  for (let attempt = 0; attempt < 5; attempt += 1) {
    const button = Array.from(container.querySelectorAll("button")).find(
      (item) => item.textContent?.includes(text),
    );
    if (button) {
      button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      return;
    }
    await flushAll();
  }
  expect(
    Array.from(container.querySelectorAll("button")).map((item) => item.textContent),
  ).toContain(text);
}

describe("AgentRolesPage model binding editor", () => {
  beforeEach(() => {
    (
      globalThis as typeof globalThis & {
        IS_REACT_ACT_ENVIRONMENT?: boolean;
      }
    ).IS_REACT_ACT_ENVIRONMENT = true;

    fetchAgentRolesMock.mockReset();
    createAgentRoleMock.mockReset();
    updateAgentRoleMock.mockReset();
    deleteAgentRoleMock.mockReset();
    setAgentRoleEnabledMock.mockReset();

    fetchAgentRolesMock.mockResolvedValue({
      items: [
        {
          role_id: "diagnosis",
          display_name: "Diagnosis",
          description: "Built-in diagnosis role",
          status: "active",
          is_builtin: true,
          profile: { system_prompt: "diagnose", persona_tags: ["sre"] },
          capability_binding: {
            mode: "unrestricted",
            allowed_skills: [],
            allowed_skill_tags: [],
          },
          policy_binding: {
            max_risk_level: "warning",
            max_action: "require_approval",
            hard_deny: [],
            require_approval_for: [],
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
          created_at: "2026-03-29T00:00:00Z",
          updated_at: "2026-03-29T00:00:00Z",
        },
      ],
      page: 1,
      limit: 100,
      total: 1,
      has_next: false,
    });
    updateAgentRoleMock.mockImplementation(
      async (_roleID: string, payload: unknown) => payload,
    );
    createAgentRoleMock.mockImplementation(async (payload: unknown) => ({
      ...(payload as Record<string, unknown>),
      role_id: "custom-role",
      status: "active",
      is_builtin: false,
      created_at: "2026-03-29T00:00:00Z",
      updated_at: "2026-03-29T00:00:00Z",
    }));
  });

  it("edits structured primary/fallback bindings without legacy mirrors", async () => {
    const { AgentRolesPage } = await import(
      "../src/pages/identity/AgentRolesPage"
    );

    const queryClient = new QueryClient();
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <AgentRolesPage />
        </QueryClientProvider>,
      );
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "Diagnosis");
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "identity.agentRoles.detail.edit");
    });
    await flushAll();

    const inputs = Array.from(container.querySelectorAll("input")) as HTMLInputElement[];
    const primaryProvider = inputs.find((input) => input.value === "openai-main");
    const primaryModel = inputs.find((input) => input.value === "gpt-4.1-mini");
    const fallbackProvider = inputs.find(
      (input) => input.value === "openai-backup",
    );
    const fallbackModel = inputs.find((input) => input.value === "gpt-4o-mini");
    const inheritCheckbox = inputs.find((input) => input.type === "checkbox");

    expect(primaryProvider).toBeTruthy();
    expect(primaryModel).toBeTruthy();
    expect(fallbackProvider).toBeTruthy();
    expect(fallbackModel).toBeTruthy();
    expect(inheritCheckbox).toBeTruthy();

    await act(async () => {
      setInputValue(primaryModel!, "gpt-4.1");
      setInputValue(fallbackProvider!, "openai-dr");
      if (inheritCheckbox) {
        inheritCheckbox.dispatchEvent(
          new MouseEvent("click", { bubbles: true }),
        );
      }
    });
    await flush();

    await act(async () => {
      await clickByText(container, "identity.agentRoles.detail.save");
    });
    await flushAll();

    expect(updateAgentRoleMock).toHaveBeenCalledTimes(1);
    const payload = updateAgentRoleMock.mock.calls[0]?.[1] as {
      model_binding?: Record<string, unknown>;
      provider_preference?: unknown;
    };
    expect(payload.model_binding).toEqual({
      primary: {
        provider_id: "openai-main",
        model: "gpt-4.1",
      },
      fallback: {
        provider_id: "openai-dr",
        model: "gpt-4o-mini",
      },
      inherit_platform_default: true,
    });
    expect(payload).not.toHaveProperty("provider_preference");

    root.unmount();
    container.remove();
  });

  it("shows structured model binding badges in detail view", async () => {
    const { AgentRolesPage } = await import(
      "../src/pages/identity/AgentRolesPage"
    );

    const queryClient = new QueryClient();
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <AgentRolesPage />
        </QueryClientProvider>,
      );
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "Diagnosis");
    });
    await flushAll();

    expect(container.textContent).toContain("Provider: openai-main");
    expect(container.textContent).toContain("Model: gpt-4.1-mini");
    expect(container.textContent).toContain("Fallback Provider: openai-backup");
    expect(container.textContent).toContain("Fallback Model: gpt-4o-mini");
    expect(container.textContent).toContain("Inherit Platform Default: no");

    root.unmount();
    container.remove();
  });
});
