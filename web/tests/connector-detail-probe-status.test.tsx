// @vitest-environment jsdom

import { act } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";

const fetchConnectorMock = vi.fn();
const checkConnectorHealthMock = vi.fn();
const probeConnectorManifestMock = vi.fn();
const notifySuccessMock = vi.fn();
const notifyErrorMock = vi.fn();

vi.mock("../src/lib/api/ops", () => ({
  fetchConnector: fetchConnectorMock,
  checkConnectorHealth: checkConnectorHealthMock,
  probeConnectorManifest: probeConnectorManifestMock,
  updateConnector: vi.fn(),
  executeConnectorCommand: vi.fn(),
  exportConnector: vi.fn(),
  invokeConnectorCapability: vi.fn(),
  queryConnectorMetrics: vi.fn(),
  applyConnectorTemplate: vi.fn(),
  setConnectorEnabled: vi.fn(),
  getApiErrorMessage: (error: unknown, fallback: string) => error instanceof Error ? error.message : fallback,
}));

vi.mock("../src/hooks/useI18n", () => ({
  useI18n: () => ({
    t: (key: string, fallback?: string) => {
      const table: Record<string, string> = {
        "common.edit": "编辑",
        "connectors.action.probeHealth": "重新探测",
        "connectors.status.probeHealthSuccess": "健康探测成功",
        "connectors.status.probeHealthFailed": "健康探测失败",
        "connectors.editor.editAction": "编辑连接器",
        "connectors.editor.identityTitleEdit": "编辑连接器身份",
        "connectors.editor.testSuccess": "连接测试成功",
        "connectors.editor.testFailed": "连接测试失败",
        "common.saveChanges": "保存变更",
      };
      return table[key] || fallback || key;
    },
  }),
}));

vi.mock("../src/hooks/ui/useNotify", () => ({
  useNotify: () => ({
    success: notifySuccessMock,
    error: notifyErrorMock,
  }),
}));

vi.mock("../src/components/layout/patterns/SplitDetailLayout", () => ({
  SplitDetailLayout: ({ sidebar, children }: { sidebar: React.ReactNode; children: React.ReactNode }) => (
    <div>
      <aside>{sidebar}</aside>
      <main>{children}</main>
    </div>
  ),
}));

vi.mock("../src/components/operator/GuidedFormDialog", () => ({
  GuidedFormDialog: ({
    open,
    title,
    children,
  }: {
    open: boolean;
    title: string;
    children: React.ReactNode;
  }) => open ? (
    <div data-testid="guided-form-dialog">
      <h2>{title}</h2>
      {children}
    </div>
  ) : null,
}));

vi.mock("../src/components/operator/ConnectorManifestEditor", () => ({
  ConnectorManifestEditor: ({
    onTest,
    testStatus,
    testing,
  }: {
    onTest?: () => void;
    testStatus?: { status: string; summary?: string };
    testing?: boolean;
  }) => (
    <div data-testid="connector-editor">
      <button type="button" onClick={() => onTest?.()}>
        编辑器测试连接
      </button>
      <div>{testing ? "testing" : "not-testing"}</div>
      <div>{testStatus?.status || "no-status"}</div>
      <div>{testStatus?.summary || "no-summary"}</div>
    </div>
  ),
}));

function buildConnector() {
  return {
    metadata: {
      id: "victorialogs-main",
      name: "victorialogs",
      display_name: "VictoriaLogs",
      vendor: "victoriametrics",
      version: "1.0.0",
      tenant_id: "",
      org_id: "",
    },
    marketplace: {
      category: "observability",
      tags: ["logs"],
      source: "custom",
    },
    compatibility: {
      tars_major_versions: ["1"],
    },
    enabled: true,
    kind: "connector",
    spec: {
      type: "logs",
      protocol: "victorialogs_http",
      capabilities: [],
      connection_form: [],
      import_export: {
        exportable: true,
        importable: true,
        formats: ["yaml"],
      },
    },
    config: {
      values: {
        base_url: "http://localhost:9428",
      },
      secret_refs: {},
    },
    lifecycle: {
      runtime: {
        state: "real",
        mode: "managed",
      },
      compatibility: {
        compatible: true,
        reasons: [],
      },
      health: {
        status: "unknown",
        summary: "runtime health check required",
      },
      health_history: [],
      revisions: [],
    },
  };
}

async function flushAll() {
  await act(async () => {
    await Promise.resolve();
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
  await act(async () => {
    await Promise.resolve();
  });
}

async function clickByText(container: HTMLElement, text: string) {
  for (let attempt = 0; attempt < 5; attempt += 1) {
    const button = Array.from(container.querySelectorAll("button")).find(
      (item) => item.textContent?.includes(text),
    );
    if (button) {
      button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      return button;
    }
    await flushAll();
  }
  expect(
    Array.from(container.querySelectorAll("button")).map((item) => item.textContent),
  ).toContain(text);
  return null;
}

describe("ConnectorDetail probe status", () => {
  beforeEach(() => {
    (
      globalThis as typeof globalThis & {
        IS_REACT_ACT_ENVIRONMENT?: boolean;
      }
    ).IS_REACT_ACT_ENVIRONMENT = true;

    fetchConnectorMock.mockReset();
    checkConnectorHealthMock.mockReset();
    probeConnectorManifestMock.mockReset();
    notifySuccessMock.mockReset();
    notifyErrorMock.mockReset();

    fetchConnectorMock.mockResolvedValue(buildConnector());
  });

  it("passes unhealthy health-check results into the editor as a failed test status", async () => {
    probeConnectorManifestMock.mockResolvedValue({
      connector_id: "victorialogs-main",
      health: {
        status: "unhealthy",
        summary: "victorialogs health probe failed: connection refused or timeout",
      },
      compatibility: { compatible: true },
      runtime: { protocol: "victorialogs_http" },
    });

    const { ConnectorDetail } = await import("../src/pages/connectors/ConnectorDetail");
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter initialEntries={["/connectors/victorialogs-main"]}>
          <QueryClientProvider client={queryClient}>
            <Routes>
              <Route path="/connectors/:id" element={<ConnectorDetail />} />
            </Routes>
          </QueryClientProvider>
        </MemoryRouter>,
      );
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "编辑");
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "编辑器测试连接");
    });
    await flushAll();

    expect(probeConnectorManifestMock).toHaveBeenCalledTimes(1);
    const content = container.textContent || "";
    expect(content).toContain("error");
    expect(content).toContain("victorialogs health probe failed: connection refused or timeout");

    root.unmount();
    container.remove();
  });
});
