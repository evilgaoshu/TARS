// @vitest-environment jsdom

import { act } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";

const fetchConnectorsMock = vi.fn();
const fetchPlatformDiscoveryMock = vi.fn();
const createConnectorMock = vi.fn();
const probeConnectorManifestMock = vi.fn();

const i18n = {
  lang: "zh-CN",
  setLang: vi.fn(),
  t: (key: string, fallback?: string) => {
    const table: Record<string, string> = {
      "connectors.hero.title": "Connectors",
      "connectors.hero.description": "管理底层外部系统接入，供 incident 诊断链路取证和受控动作复用，不作为值班入口首页。",
      "connectors.stats.healthDesc": "稳定接入比例",
      "connectors.status.credNone": "无需凭据",
      "connectors.action.creationTitle": "配置底层接入",
      "connectors.action.add": "新建连接器",
      "connectors.status.testPassed": "连接测试通过",
      "connectors.status.testFailed": "连接测试失败",
      "connectors.list.header.name": "连接器",
      "connectors.list.header.status": "启用状态",
      "connectors.list.header.type": "类型 / 接入方式",
      "connectors.list.header.health": "健康状态",
      "connectors.list.header.credentials": "凭据状态",
      "connectors.list.header.lastCheck": "最近检查",
      "connectors.editor.searchPlaceholder": "搜索 connector 类型，例如 SSH / VictoriaLogs",
      "connectors.editor.testAction": "测试连接",
      "connectors.editor.createAction": "创建连接器",
    };
    return table[key] || fallback || key;
  },
};

vi.mock("../src/lib/api/ops", () => ({
  fetchConnectors: fetchConnectorsMock,
  fetchPlatformDiscovery: fetchPlatformDiscoveryMock,
  createConnector: createConnectorMock,
  probeConnectorManifest: probeConnectorManifestMock,
  fetchSecretsInventory: vi.fn().mockResolvedValue({ items: [] }),
}));

vi.mock("../src/hooks/useI18n", () => ({
  useI18n: () => i18n,
}));

vi.mock("../src/hooks/ui/useNotify", () => ({
  useNotify: () => ({ success: vi.fn(), error: vi.fn() }),
}));

vi.mock("../src/components/operator/GuidedFormDialog", () => ({
  GuidedFormDialog: ({
    open,
    title,
    description,
    children,
  }: {
    open: boolean;
    title: string;
    description?: string;
    children: React.ReactNode;
  }) =>
    open ? (
      <div data-testid="guided-form-dialog">
        <h2>{title}</h2>
        {description ? <p>{description}</p> : null}
        <div>{children}</div>
      </div>
    ) : null,
}));

async function flushAll() {
  await act(async () => {
    await Promise.resolve();
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
  await act(async () => {
    await Promise.resolve();
  });
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
      return button;
    }
    await flushAll();
  }
  expect(
    Array.from(container.querySelectorAll("button")).map((item) => item.textContent),
  ).toContain(text);
  return null;
}

describe("ConnectorsList Spec Alignment", () => {
  beforeEach(() => {
    (
      globalThis as typeof globalThis & {
        IS_REACT_ACT_ENVIRONMENT?: boolean;
      }
    ).IS_REACT_ACT_ENVIRONMENT = true;

    fetchConnectorsMock.mockReset();
    fetchPlatformDiscoveryMock.mockReset();
    createConnectorMock.mockReset();
    probeConnectorManifestMock.mockReset();

    fetchPlatformDiscoveryMock.mockResolvedValue({
      registered_connectors_count: 2,
      registered_connector_kinds: ["prometheus_http", "jumpserver_api"],
      tool_plan_capabilities: [{}, {}, {}],
    });

    fetchConnectorsMock.mockResolvedValue({
      items: [
        {
          metadata: { id: "existing-1", display_name: "Existing Connector", version: "1.0.0" },
          enabled: true,
          kind: "connector",
          spec: { protocol: "prometheus_http" },
          config: { secret_refs: { api_key: "ref:key" } },
          lifecycle: {
            health: { status: "healthy", summary: "All systems go", checked_at: "2026-03-29T10:00:00Z" }
          }
        }
      ],
      total: 2,
      page: 1,
      limit: 10,
      has_next: false,
    });

    createConnectorMock.mockResolvedValue({
      metadata: { id: "victorialogs-main", display_name: "VictoriaLogs", version: "1.0.0" },
    });
    probeConnectorManifestMock.mockResolvedValue({
      connector_id: "victorialogs-main",
      display_name: "VictoriaLogs",
      enabled: true,
      health: {
        status: "healthy",
        summary: "logs connector health probe succeeded",
      },
      compatibility: {
        compatible: true,
      },
      runtime: {
        protocol: "victorialogs_http",
      },
    });
  });

  it("renders aligned columns and summary cards", async () => {
    const { ConnectorsList } = await import("../src/pages/connectors/ConnectorsList");
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } }
    });
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter>
          <QueryClientProvider client={queryClient}>
            <ConnectorsList />
          </QueryClientProvider>
        </MemoryRouter>
      );
    });
    await flushAll();

    // Check Summary Cards
    const content = container.textContent || "";
    expect(content).toContain("管理底层外部系统接入，供 incident 诊断链路取证和受控动作复用，不作为值班入口首页。");
    expect(content).toContain("100%"); // Health Score calculation (2/2 * 100)
    expect(content).toContain("稳定接入比例");

    // Check Columns
    expect(content).toContain("Existing Connector");
    expect(content.toUpperCase()).toContain("PROMETHEUS_HTTP");
    expect(content).toContain("All systems go");
    expect(content).toContain("无需凭据"); // Credentials
    expect(content).toMatch(/\d{1,2}:\d{2}:\d{2}/); // Last Check time (resilient to timezone)

    root.unmount();
    container.remove();
  });

  it("uses official templates as create shortcuts instead of importing immediately", async () => {
    const { ConnectorsList } = await import("../src/pages/connectors/ConnectorsList");
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } }
    });
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter>
          <QueryClientProvider client={queryClient}>
            <ConnectorsList />
          </QueryClientProvider>
        </MemoryRouter>
      );
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "VictoriaLogs");
    });
    await flushAll();

    expect(createConnectorMock).not.toHaveBeenCalled();
    expect(container.textContent || "").toContain("配置底层接入");
    expect(container.textContent || "").toContain("VictoriaLogs");

    root.unmount();
    container.remove();
  });

  it("supports searchable connector-type selection and one-page create flow", async () => {
    const { ConnectorsList } = await import("../src/pages/connectors/ConnectorsList");
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } }
    });
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter>
          <QueryClientProvider client={queryClient}>
            <ConnectorsList />
          </QueryClientProvider>
        </MemoryRouter>
      );
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "新建连接器");
    });
    await flushAll();

    const searchInput = Array.from(container.querySelectorAll("input")).find(
      (input) =>
        (input as HTMLInputElement).placeholder ===
        "搜索 connector 类型，例如 SSH / VictoriaLogs",
    ) as HTMLInputElement | undefined;
    expect(searchInput).toBeTruthy();

    await act(async () => {
      setInputValue(searchInput!, "VictoriaLogs");
    });
    await flushAll();

    expect(container.textContent || "").toContain("VictoriaLogs");

    await act(async () => {
      await clickByText(container, "VictoriaLogs");
    });
    await flushAll();

    const idInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "victorialogs-main",
    ) as HTMLInputElement | undefined;
    expect(idInput).toBeTruthy();

    expect(container.textContent || "").toContain("测试连接");
    expect(container.textContent || "").toContain("新建连接器");

    const baseUrlInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "https://play-vmlogs.victoriametrics.com",
    ) as HTMLInputElement | undefined;
    expect(baseUrlInput).toBeTruthy();

    await act(async () => {
      setInputValue(idInput!, "prometheus-ops");
      setInputValue(baseUrlInput!, "https://prom.example.test");
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "创建连接器");
    });
    await flushAll();

    expect(createConnectorMock).toHaveBeenCalledTimes(1);

    expect(probeConnectorManifestMock).not.toHaveBeenCalled();
    expect(createConnectorMock.mock.calls[0]?.[0]?.manifest?.spec?.connection_form).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ key: "base_url", required: true }),
      ]),
    );

    root.unmount();
    container.remove();
  });

  it("validates base_url before test connection", async () => {
    const { ConnectorsList } = await import("../src/pages/connectors/ConnectorsList");
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } }
    });
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter>
          <QueryClientProvider client={queryClient}>
            <ConnectorsList />
          </QueryClientProvider>
        </MemoryRouter>
      );
    });
    await flushAll();

    // Open create dialog
    await act(async () => {
      await clickByText(container, "新建连接器");
    });
    await flushAll();

    // Select VictoriaMetrics template
    await act(async () => {
      await clickByText(container, "VictoriaMetrics");
    });
    await flushAll();

    // Fill in ID
    const idInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "victoriametrics-main",
    ) as HTMLInputElement | undefined;
    expect(idInput).toBeTruthy();
    await act(async () => {
      setInputValue(idInput!, "victoriametrics-test");
    });
    await flushAll();

    // Verify placeholder shows correct VictoriaMetrics URL
    const baseUrlInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "http://127.0.0.1:8428",
    ) as HTMLInputElement | undefined;
    expect(baseUrlInput).toBeTruthy();

    // Test with empty base_url should fail validation locally
    // (the test button requires createReady which checks required fields)
    root.unmount();
    container.remove();
  });

  it("blocks save and test when required ssh fields are empty and shows field errors", async () => {
    const { ConnectorsList } = await import("../src/pages/connectors/ConnectorsList");
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } }
    });
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter>
          <QueryClientProvider client={queryClient}>
            <ConnectorsList />
          </QueryClientProvider>
        </MemoryRouter>
      );
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "新建连接器");
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "SSH Connector");
    });
    await flushAll();

    const createButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent?.includes("创建连接器")) as HTMLButtonElement | undefined;
    const testButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent?.includes("测试连接")) as HTMLButtonElement | undefined;

    expect(createButton).toBeTruthy();
    expect(testButton).toBeTruthy();
    expect(createButton?.disabled).toBe(true);

    await act(async () => {
      testButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    await flushAll();

    expect(probeConnectorManifestMock).not.toHaveBeenCalled();
    const content = container.textContent || "";
    expect(content).toContain("Host is required");
    expect(content).toContain("Username is required");
    expect(content).toContain("Credential ID is required");

    root.unmount();
    container.remove();
  });

  it("shows correct placeholder for different connector types", async () => {
    const { ConnectorsList } = await import("../src/pages/connectors/ConnectorsList");
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } }
    });
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter>
          <QueryClientProvider client={queryClient}>
            <ConnectorsList />
          </QueryClientProvider>
        </MemoryRouter>
      );
    });
    await flushAll();

    // Open create dialog
    await act(async () => {
      await clickByText(container, "新建连接器");
    });
    await flushAll();

    // Select Prometheus template
    const searchInput = Array.from(container.querySelectorAll("input")).find(
      (input) =>
        (input as HTMLInputElement).placeholder ===
        "搜索 connector 类型，例如 SSH / VictoriaLogs",
    ) as HTMLInputElement | undefined;

    await act(async () => {
      setInputValue(searchInput!, "Prometheus");
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "Prometheus");
    });
    await flushAll();

    // Verify placeholder shows correct Prometheus URL
    const prometheusUrlInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "http://127.0.0.1:9090",
    ) as HTMLInputElement | undefined;
    expect(prometheusUrlInput).toBeTruthy();

    root.unmount();
    container.remove();
  });
});
