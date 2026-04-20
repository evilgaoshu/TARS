// @vitest-environment jsdom

import { act } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";

const probeConnectorManifestMock = vi.fn();

const i18n = {
  lang: "zh-CN",
  setLang: vi.fn(),
  t: (key: string, fallback?: string) => {
    const table: Record<string, string> = {
      "connectors.hero.title": "Connectors",
      "connectors.hero.description": "管理外部系统接入",
      "connectors.stats.healthDesc": "稳定接入比例",
      "connectors.stats.totalDesc": "",
      "connectors.stats.discovered": "",
      "connectors.stats.discoveredDesc": "",
      "connectors.stats.capabilities": "",
      "connectors.stats.capabilitiesDesc": "",
      "connectors.stats.health": "",
      "connectors.status.credNone": "无需凭据",
      "connectors.action.creationTitle": "新建连接器",
      "connectors.action.add": "新建连接器",
      "connectors.action.creationDesc": "",
      "connectors.action.commonTypes": "",
      "connectors.action.commonTypesDesc": "",
      "connectors.action.secrets": "",
      "connectors.status.testPassed": "连接测试通过",
      "connectors.status.testFailed": "连接测试失败",
      "connectors.list.header.name": "连接器",
      "connectors.list.header.status": "启用状态",
      "connectors.list.header.type": "类型 / 接入方式",
      "connectors.list.header.health": "健康状态",
      "connectors.list.header.credentials": "凭据状态",
      "connectors.list.header.lastCheck": "最近检查",
      "connectors.list.noReport": "",
      "connectors.filter.allKinds": "",
      "connectors.filter.standard": "",
      "connectors.filter.allStates": "",
      "connectors.filter.enabled": "",
      "connectors.filter.disabled": "",
      "connectors.filter.searchPlaceholder": "",
      "connectors.stats.total": "",
      "connectors.stats.discoveredCount": "",
      "connectors.stats.noKinds": "",
      "connectors.editor.searchPlaceholder": "搜索 connector 类型，例如 SSH / VictoriaLogs",
      "connectors.editor.testAction": "测试连接",
      "connectors.editor.createAction": "创建连接器",
      "connectors.editor.testSuccess": "连接测试成功",
      "connectors.editor.testFailed": "连接测试失败",
      "connectors.editor.testIdle": "点击测试连接以验证配置",
      "connectors.editor.type": "类型",
      "connectors.editor.protocol": "协议",
      "connectors.editor.vendor": "厂商",
      "connectors.editor.searchTitle": "搜索",
      "connectors.editor.searchDesc": "搜索连接器类型",
      "connectors.editor.identityTitle": "身份",
      "connectors.editor.identityDesc": "连接器身份信息",
      "connectors.editor.idLabel": "连接器 ID",
      "connectors.editor.idHint": "",
      "connectors.editor.nameLabel": "名称",
      "connectors.editor.nameHint": "",
      "connectors.editor.descLabel": "描述",
      "connectors.editor.descHint": "",
      "connectors.editor.descPlaceholder": "",
      "connectors.editor.connectivityTitle": "连接",
      "connectors.editor.connectivityDesc": "连接配置",
      "connectors.editor.afterSaveLabel": "保存后",
      "connectors.editor.enableAfterSave": "启用",
      "connectors.editor.enableAfterSaveDesc": "",
      "connectors.editor.opsHint": "",
      "connectors.editor.opsSuffix": "",
      "connectors.editor.cancel": "取消",
      "connectors.editor.createHint": "",
      "connectors.editor.noMatch": "",
      "connectors.editor.chooseTypeHint": "选择连接器类型",
      "connectors.editor.noFields": "无字段",
      "connectors.editor.noFieldsDesc": "",
      "connectors.editor.secretHint": "敏感字段",
      "common.na": "—",
      "common.saving": "保存中...",
    };
    return table[key] || fallback || key;
  },
};

vi.mock("../src/lib/api/ops", () => ({
  fetchConnectors: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, limit: 10, has_next: false }),
  fetchPlatformDiscovery: vi.fn().mockResolvedValue({ registered_connectors_count: 0, registered_connector_kinds: [], tool_plan_capabilities: [] }),
  createConnector: vi.fn().mockResolvedValue({ metadata: { id: "test-connector" } }),
  probeConnectorManifest: probeConnectorManifestMock,
  getApiErrorMessage: (error: unknown, fallback: string) => error instanceof Error ? error.message : fallback,
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
    children,
  }: {
    open: boolean;
    title: string;
    children: React.ReactNode;
  }) =>
    open ? (
      <div data-testid="guided-form-dialog">
        <h2>{title}</h2>
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

describe("Connector Create Probe Validation", () => {
  beforeEach(() => {
    (
      globalThis as typeof globalThis & {
        IS_REACT_ACT_ENVIRONMENT?: boolean;
      }
    ).IS_REACT_ACT_ENVIRONMENT = true;

    probeConnectorManifestMock.mockReset();
    probeConnectorManifestMock.mockResolvedValue({
      connector_id: "test-connector",
      display_name: "Test Connector",
      enabled: true,
      health: {
        status: "healthy",
        summary: "Connection successful",
      },
      compatibility: { compatible: true },
      runtime: { protocol: "victoriametrics_http" },
    });
  });

  // 1. 空 Base URL 点击测试连接应失败并显示错误
  it("should show validation error when testing connection with empty base_url", async () => {
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

    // 打开创建对话框
    await act(async () => {
      await clickByText(container, "新建连接器");
    });
    await flushAll();

    // 选择 VictoriaMetrics 模板
    await act(async () => {
      await clickByText(container, "VictoriaMetrics");
    });
    await flushAll();

    // 填写 ID，但不填写 base_url
    const idInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "victoriametrics-main",
    ) as HTMLInputElement | undefined;
    expect(idInput).toBeTruthy();
    await act(async () => {
      setInputValue(idInput!, "test-connector");
    });
    await flushAll();

    // 点击测试连接
    await act(async () => {
      await clickByText(container, "测试连接");
    });
    await flushAll();

    // 验证：probeConnectorManifest 不应该被调用
    expect(probeConnectorManifestMock).not.toHaveBeenCalled();

    // 验证：应该显示验证错误
    const content = container.textContent || "";
    expect(content).toContain("Base URL");
    expect(content).toContain("required");

    root.unmount();
    container.remove();
  });

  // 2. 非 http(s) URL 应失败
  it("should show validation error when testing connection with non-http(s) URL", async () => {
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

    // 打开创建对话框
    await act(async () => {
      await clickByText(container, "新建连接器");
    });
    await flushAll();

    // 选择 VictoriaMetrics 模板
    await act(async () => {
      await clickByText(container, "VictoriaMetrics");
    });
    await flushAll();

    // 填写 ID
    const idInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "victoriametrics-main",
    ) as HTMLInputElement | undefined;
    await act(async () => {
      setInputValue(idInput!, "test-connector");
    });
    await flushAll();

    // 填写 base_url（使用 ftp 协议 - 非 http(s)）
    const baseUrlInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "http://127.0.0.1:8428",
    ) as HTMLInputElement | undefined;
    expect(baseUrlInput).toBeTruthy();
    await act(async () => {
      setInputValue(baseUrlInput!, "ftp://example.com");
    });
    await flushAll();

    // 点击测试连接
    await act(async () => {
      await clickByText(container, "测试连接");
    });
    await flushAll();

    // 验证：probeConnectorManifest 不应该被调用
    expect(probeConnectorManifestMock).not.toHaveBeenCalled();

    // 验证：应该显示 http(s) 验证错误
    const content = container.textContent || "";
    expect(content).toContain("http://");
    expect(content).toContain("https://");

    root.unmount();
    container.remove();
  });

  // 3. 非法 URL 格式应失败
  it("should show validation error when testing connection with invalid URL format", async () => {
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

    // 打开创建对话框
    await act(async () => {
      await clickByText(container, "新建连接器");
    });
    await flushAll();

    // 选择 VictoriaMetrics 模板
    await act(async () => {
      await clickByText(container, "VictoriaMetrics");
    });
    await flushAll();

    // 填写 ID
    const idInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "victoriametrics-main",
    ) as HTMLInputElement | undefined;
    await act(async () => {
      setInputValue(idInput!, "test-connector");
    });
    await flushAll();

    // 填写 base_url（使用非法格式）
    const baseUrlInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "http://127.0.0.1:8428",
    ) as HTMLInputElement | undefined;
    await act(async () => {
      setInputValue(baseUrlInput!, "http://[invalid url format");
    });
    await flushAll();

    // 点击测试连接
    await act(async () => {
      await clickByText(container, "测试连接");
    });
    await flushAll();

    // 验证：probeConnectorManifest 不应该被调用
    expect(probeConnectorManifestMock).not.toHaveBeenCalled();

    // 验证：应该显示 URL 格式错误
    const content = container.textContent || "";
    expect(content).toContain("Invalid");
    expect(content).toContain("URL");

    root.unmount();
    container.remove();
  });

  // 4. 这些场景下不应调用 probeConnectorManifest（合并验证）
  it("should never call probeConnectorManifest when validation fails", async () => {
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

    // 打开创建对话框
    await act(async () => {
      await clickByText(container, "新建连接器");
    });
    await flushAll();

    // 选择 VictoriaMetrics 模板
    await act(async () => {
      await clickByText(container, "VictoriaMetrics");
    });
    await flushAll();

    // 填写 ID
    const idInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "victoriametrics-main",
    ) as HTMLInputElement | undefined;
    await act(async () => {
      setInputValue(idInput!, "test-connector");
    });
    await flushAll();

    const baseUrlInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "http://127.0.0.1:8428",
    ) as HTMLInputElement | undefined;

    // 测试场景 1: 空 URL
    probeConnectorManifestMock.mockClear();
    await act(async () => {
      setInputValue(baseUrlInput!, "");
    });
    await flushAll();
    await act(async () => {
      await clickByText(container, "测试连接");
    });
    await flushAll();
    expect(probeConnectorManifestMock).not.toHaveBeenCalled();

    // 测试场景 2: 仅空格
    probeConnectorManifestMock.mockClear();
    await act(async () => {
      setInputValue(baseUrlInput!, "   ");
    });
    await flushAll();
    await act(async () => {
      await clickByText(container, "测试连接");
    });
    await flushAll();
    expect(probeConnectorManifestMock).not.toHaveBeenCalled();

    // 测试场景 3: file:// 协议
    probeConnectorManifestMock.mockClear();
    await act(async () => {
      setInputValue(baseUrlInput!, "file:///etc/passwd");
    });
    await flushAll();
    await act(async () => {
      await clickByText(container, "测试连接");
    });
    await flushAll();
    expect(probeConnectorManifestMock).not.toHaveBeenCalled();

    // 测试场景 4: 非法格式
    probeConnectorManifestMock.mockClear();
    await act(async () => {
      setInputValue(baseUrlInput!, "not a url at all");
    });
    await flushAll();
    await act(async () => {
      await clickByText(container, "测试连接");
    });
    await flushAll();
    expect(probeConnectorManifestMock).not.toHaveBeenCalled();

    root.unmount();
    container.remove();
  });

  // 5. probe 成功/失败状态展示与字段修改后重置
  it("should display probe success status and reset on field change", async () => {
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

    // 打开创建对话框
    await act(async () => {
      await clickByText(container, "新建连接器");
    });
    await flushAll();

    // 选择 VictoriaMetrics 模板
    await act(async () => {
      await clickByText(container, "VictoriaMetrics");
    });
    await flushAll();

    // 填写 ID
    const idInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "victoriametrics-main",
    ) as HTMLInputElement | undefined;
    await act(async () => {
      setInputValue(idInput!, "test-connector");
    });
    await flushAll();

    // 填写 base_url（有效）
    const baseUrlInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "http://127.0.0.1:8428",
    ) as HTMLInputElement | undefined;
    await act(async () => {
      setInputValue(baseUrlInput!, "http://localhost:8428");
    });
    await flushAll();

    // 点击测试连接
    await act(async () => {
      await clickByText(container, "测试连接");
    });
    await flushAll();

    // 验证：probeConnectorManifest 应该被调用
    expect(probeConnectorManifestMock).toHaveBeenCalledTimes(1);

    // 验证：应该显示成功状态
    const content = container.textContent || "";
    expect(content.includes("success") || content.includes("通过")).toBe(true);

    // 修改字段后，状态应该重置
    await act(async () => {
      setInputValue(baseUrlInput!, "http://localhost:9090");
    });
    await flushAll();

    // 验证：状态已重置为 idle
    const contentAfter = container.textContent || "";
    expect(contentAfter.includes("idle") || contentAfter.includes("验证配置")).toBe(true);

    root.unmount();
    container.remove();
  });

  // 6. probe 失败状态展示
  it("should display probe error status when probe fails", async () => {
    // 捕获未处理的 Promise rejection 错误，避免测试警告
    const originalErrorHandler = window.onerror;
    const originalUnhandledRejection = window.onunhandledrejection;
    window.onerror = () => {};
    window.onunhandledrejection = () => {};

    probeConnectorManifestMock.mockRejectedValue(new Error("Connection refused"));

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

    // 打开创建对话框
    await act(async () => {
      await clickByText(container, "新建连接器");
    });
    await flushAll();

    // 选择 VictoriaMetrics 模板
    await act(async () => {
      await clickByText(container, "VictoriaMetrics");
    });
    await flushAll();

    // 填写 ID
    const idInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "victoriametrics-main",
    ) as HTMLInputElement | undefined;
    await act(async () => {
      setInputValue(idInput!, "test-connector");
    });
    await flushAll();

    // 填写 base_url（有效）
    const baseUrlInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "http://127.0.0.1:8428",
    ) as HTMLInputElement | undefined;
    await act(async () => {
      setInputValue(baseUrlInput!, "http://localhost:8428");
    });
    await flushAll();

    // 点击测试连接
    await act(async () => {
      await clickByText(container, "测试连接");
    });
    await flushAll();

    // 验证：probeConnectorManifest 应该被调用
    expect(probeConnectorManifestMock).toHaveBeenCalledTimes(1);

    // 验证：应该显示失败状态
    const content = container.textContent || "";
    expect(content.includes("error") || content.includes("失败")).toBe(true);
    expect(content).toContain("Connection refused");

    root.unmount();
    container.remove();

    // 恢复错误处理
    window.onerror = originalErrorHandler;
    window.onunhandledrejection = originalUnhandledRejection;
  });

  it("should treat non-healthy probe responses as failed and show the summary", async () => {
    probeConnectorManifestMock.mockResolvedValue({
      connector_id: "test-connector",
      display_name: "Test Connector",
      enabled: true,
      health: {
        status: "unhealthy",
        summary: "victorialogs health probe failed: connection refused or timeout",
      },
      compatibility: { compatible: true },
      runtime: { protocol: "victorialogs_http" },
    });

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
      await clickByText(container, "VictoriaLogs");
    });
    await flushAll();

    const idInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "victorialogs-main",
    ) as HTMLInputElement | undefined;
    await act(async () => {
      setInputValue(idInput!, "test-connector");
    });
    await flushAll();

    const baseUrlInput = Array.from(container.querySelectorAll("input")).find(
      (input) => (input as HTMLInputElement).placeholder === "http://127.0.0.1:9428",
    ) as HTMLInputElement | undefined;
    await act(async () => {
      setInputValue(baseUrlInput!, "http://localhost:9428");
    });
    await flushAll();

    await act(async () => {
      await clickByText(container, "测试连接");
    });
    await flushAll();

    expect(probeConnectorManifestMock).toHaveBeenCalledTimes(1);

    const content = container.textContent || "";
    expect(content.includes("error") || content.includes("失败")).toBe(true);
    expect(content).toContain("victorialogs health probe failed: connection refused or timeout");

    root.unmount();
    container.remove();
  });
});
