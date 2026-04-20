import { describe, expect, it, vi } from "vitest";
import { readFileSync } from "node:fs";

const providersPage = readFileSync(
  "/Users/yue/TARS/web/src/pages/providers/ProvidersPage.tsx",
  "utf8",
);
const agentRolesPage = readFileSync(
  "/Users/yue/TARS/web/src/pages/identity/AgentRolesPage.tsx",
  "utf8",
);
const automationsPage = readFileSync(
  "/Users/yue/TARS/web/src/pages/automations/AutomationsPage.tsx",
  "utf8",
);
const extensionsPage = readFileSync(
  "/Users/yue/TARS/web/src/pages/extensions/ExtensionsPage.tsx",
  "utf8",
);
const opsActionView = readFileSync(
  "/Users/yue/TARS/web/src/pages/ops/OpsActionView.tsx",
  "utf8",
);
const channelsPage = readFileSync(
  "/Users/yue/TARS/web/src/pages/channels/ChannelsPage.tsx",
  "utf8",
);
const connectorManifestEditor = readFileSync(
  "/Users/yue/TARS/web/src/components/operator/ConnectorManifestEditor.tsx",
  "utf8",
);
const featureGateProvider = readFileSync(
  "/Users/yue/TARS/web/src/lib/FeatureGateProvider.tsx",
  "utf8",
);
const skillsList = readFileSync(
  "/Users/yue/TARS/web/src/pages/skills/SkillsList.tsx",
  "utf8",
);
const sessionList = readFileSync(
  "/Users/yue/TARS/web/src/pages/sessions/SessionList.tsx",
  "utf8",
);
const outboxConsole = readFileSync(
  "/Users/yue/TARS/web/src/pages/outbox/OutboxConsole.tsx",
  "utf8",
);
const executionActionBar = readFileSync(
  "/Users/yue/TARS/web/src/components/operator/ExecutionActionBar.tsx",
  "utf8",
);
const docsView = readFileSync(
  "/Users/yue/TARS/web/src/pages/docs/DocsView.tsx",
  "utf8",
);
const sessionsExecutionsSmoke = readFileSync(
  "/Users/yue/TARS/web/tests/sessions-executions.smoke.spec.ts",
  "utf8",
);
const webGitignore = readFileSync(
  "/Users/yue/TARS/web/.gitignore",
  "utf8",
);
const webPackage = JSON.parse(
  readFileSync("/Users/yue/TARS/web/package.json", "utf8"),
) as {
  dependencies?: Record<string, string>;
  devDependencies?: Record<string, string>;
};

describe("trigger event catalog", () => {
  it("uses a real backend trigger event as the default draft event", async () => {
    const { DEFAULT_TRIGGER_EVENT_TYPE, TRIGGER_EVENT_OPTIONS } =
      await import("../src/lib/triggers/catalog");

    expect(TRIGGER_EVENT_OPTIONS.map((option) => option.value)).toContain(
      DEFAULT_TRIGGER_EVENT_TYPE,
    );
    expect(DEFAULT_TRIGGER_EVENT_TYPE).toBe("on_execution_completed");
  });

  it("filters trigger delivery channels down to supported direct kinds", async () => {
    const { filterTriggerDeliveryChannels } =
      await import("../src/lib/triggers/catalog");

    expect(
      filterTriggerDeliveryChannels([
        { id: "inbox-primary", kind: "in_app_inbox" },
        { id: "telegram-main", kind: "telegram" },
        { id: "slack-primary", kind: "slack" },
        { id: "webchat-primary", kind: "web_chat" },
      ]),
    ).toEqual([
      { id: "inbox-primary", kind: "in_app_inbox" },
      { id: "telegram-main", kind: "telegram" },
    ]);
  });
});

describe("provider discovery copy", () => {
  it("passes interpolation params to translation helpers", async () => {
    const t = vi.fn((key: string, params?: unknown) =>
      JSON.stringify({ key, params }),
    );
    const { providerDiscoveryMessages } =
      await import("../src/lib/ui/provider-discovery");

    providerDiscoveryMessages.discovered(t, 3);
    providerDiscoveryMessages.unreachable(t, "gpt-4o");

    expect(t).toHaveBeenNthCalledWith(1, "prov.discovery.discovered", {
      count: 3,
    });
    expect(t).toHaveBeenNthCalledWith(2, "prov.discovery.unreachable", {
      model: "gpt-4o",
    });
  });
});

describe("governance surfaces", () => {
  it("keeps provider defaults read-only on the provider registry page", () => {
    expect(providersPage).not.toContain("saveBinding(");
    expect(providersPage).not.toContain("Legacy Primary / Assist Bindings");
    expect(providersPage).toContain("Read-only on this page");
  });

  it("exposes full agent role governance fields in the role editor", () => {
    expect(agentRolesPage).toContain("allowed_connector_capabilities");
    expect(agentRolesPage).toContain("denied_connector_capabilities");
    expect(agentRolesPage).toContain("allowed_skill_tags");
    expect(agentRolesPage).toContain("require_approval_for");
    expect(agentRolesPage).toContain("inherit_platform_default");
    expect(agentRolesPage).toContain("primary?.provider_id");
    expect(agentRolesPage).toContain("fallback?.model");
  });

  it("renders automation governance policy controls in the automation editor", () => {
    expect(automationsPage).toContain("governance_policy");
    expect(automationsPage).toContain("t('auto.form.governanceApproval')");
    expect(automationsPage).toContain("t('auto.form.governanceDisabled')");
  });
});

describe("ExtensionsPage UX convergence", () => {
  it("uses TagInput for tags — no raw CSV input", () => {
    expect(extensionsPage).not.toContain('Tags (CSV)');
    expect(extensionsPage).toContain('TagInput');
  });

  it("form state tags is string[] not string", () => {
    // The type annotation should declare tags as string[]
    expect(extensionsPage).toContain("tags: string[];");
  });

  it("subtitle is in English", () => {
    expect(extensionsPage).not.toContain("生成受治理");
    expect(extensionsPage).toContain("Generate governed skill bundle candidates");
  });

  it("stat card subtitles are in English", () => {
    expect(extensionsPage).not.toContain("全部候选");
    expect(extensionsPage).not.toContain("已通过验证");
    expect(extensionsPage).not.toContain("待审批");
    expect(extensionsPage).not.toContain("已导入技能注册表");
    expect(extensionsPage).toContain("Total candidates");
    expect(extensionsPage).toContain("Passed validation");
  });

  it("empty state and composer subtitle are in English", () => {
    expect(extensionsPage).not.toContain("选择一个候选");
    expect(extensionsPage).not.toContain("填写字段后点击");
    expect(extensionsPage).toContain("Select a candidate");
    expect(extensionsPage).toContain("Fill in the fields below");
  });

  it("Planner Steps hint is in English", () => {
    expect(extensionsPage).not.toContain("保持为有效 JSON");
    expect(extensionsPage).toContain("Must be valid JSON");
  });

  it("has no duplicate Read-only First rows — only one occurrence", () => {
    const matches = extensionsPage.match(/Read-only First/g);
    expect(matches?.length).toBe(1); // deduplicated — only one InfoRow
  });

  it("does not use splitCsv for tags in buildBundle", () => {
    // splitCsv should not be called with form.tags
    expect(extensionsPage).not.toContain("splitCsv(form.tags)");
  });
});

describe("OpsActionView UX convergence", () => {
  it("uses ConfirmActionDialog instead of window.confirm for reindex", () => {
    expect(opsActionView).not.toContain("window.confirm");
    expect(opsActionView).toContain("ConfirmActionDialog");
    expect(opsActionView).toContain("reindexConfirmOpen");
  });
});

describe("ChannelsPage UX convergence", () => {
  it("no longer shows raw CSV label for linked_users", () => {
    expect(channelsPage).not.toContain("Linked Users (CSV)");
  });

  it("uses friendly labels for usages and capabilities", () => {
    expect(channelsPage).toContain("usageFriendlyLabel");
    expect(channelsPage).toContain("capabilityFriendlyLabel");
  });

  it("groups channels into first-party and external sections", () => {
    expect(channelsPage).toContain("FIRST_PARTY_KINDS");
    expect(channelsPage).toContain("First-party");
    expect(channelsPage).toContain("External");
  });
});

describe("Connector first-class protocols", () => {
  it("exposes SSH Native and VictoriaLogs protocols in the connector editor", () => {
    expect(connectorManifestEditor).toContain('value="ssh_native"');
    expect(connectorManifestEditor).toContain('value="victorialogs_http"');
  });
});

describe("Feature gate defaults", () => {
  it("keeps OIDC and local password login fail-closed until runtime config proves they are available", () => {
    expect(featureGateProvider).toContain("'identity.oidc': { enabled: false, status: 'requires_config' }");
    expect(featureGateProvider).toContain("'identity.local_password': { enabled: false, status: 'requires_config' }");
  });
});

describe("SkillsList UX convergence", () => {
  it("includes deprecated and archived in status filter options", () => {
    expect(skillsList).toContain("deprecated");
    expect(skillsList).toContain("archived");
  });

  it("shows execution_policy badge on skill cards", () => {
    expect(skillsList).toContain("execution_policy");
    expect(skillsList).toContain("Policy:");
  });
});

describe("SessionList UX convergence", () => {
  it("has a copy-to-clipboard button for session IDs", () => {
    expect(sessionList).toContain("session.session_id");
    // Copy icon imported from lucide-react
    expect(sessionList).toContain("Copy");
  });
});

describe("OutboxConsole UX convergence", () => {
  it("shows full event ID (no truncation)", () => {
    expect(outboxConsole).not.toContain(".split('-')[1]");
    expect(outboxConsole).toContain("evt.id");
  });

  it("uses ConfirmActionDialog instead of window.confirm and window.alert", () => {
    expect(outboxConsole).not.toContain("window.confirm");
    expect(outboxConsole).not.toContain("window.alert");
    expect(outboxConsole).toContain("ConfirmActionDialog");
  });
});

describe("ExecutionActionBar UX convergence", () => {
  it("does not reference Telegram-specific copy", () => {
    expect(executionActionBar).not.toContain("Telegram already supports");
  });

  it("uses a reason textarea inside the confirm dialog", () => {
    expect(executionActionBar).toContain("reason");
    expect(executionActionBar).toContain("extraContent");
  });

  it("collapses command modification behind an expand toggle", () => {
    expect(executionActionBar).toContain("modifyExpanded");
    expect(executionActionBar).toContain("executions.action.modifyLabel");
  });
});

describe("Browser smoke guardrails", () => {
  it("collects real console and page errors on the sessions/executions smoke path", () => {
    expect(sessionsExecutionsSmoke).toContain("page.on('console'");
    expect(sessionsExecutionsSmoke).toContain("page.on('pageerror'");
    expect(sessionsExecutionsSmoke).not.toContain("const errs: string[] = []");
  });
});

describe("Publishable source guardrails", () => {
  it("does not ignore the logs page source directory", () => {
    const activeIgnoreLines = webGitignore
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter((line) => line && !line.startsWith("#"));

    expect(activeIgnoreLines).not.toContain("logs");
    expect(activeIgnoreLines).toContain("/logs/");
  });
});

describe("Docs dependency health", () => {
  it("does not depend on swagger-ui-react while running React 19", () => {
    const deps = {
      ...(webPackage.dependencies || {}),
      ...(webPackage.devDependencies || {}),
    };

    expect(deps).not.toHaveProperty("swagger-ui-react");
    expect(deps).not.toHaveProperty("@types/swagger-ui-react");
    expect(docsView).not.toContain("swagger-ui-react");
  });
});
