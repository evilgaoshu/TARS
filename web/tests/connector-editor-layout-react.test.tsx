// @vitest-environment jsdom

import { act } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createRoot } from "react-dom/client";
import { MemoryRouter } from "react-router-dom";

vi.mock("../src/hooks/useI18n", () => ({
  useI18n: () => ({
    lang: "zh-CN",
    setLang: vi.fn(),
    t: (_key: string, fallback?: string) => fallback || _key,
  }),
}));

async function flushAll() {
  await act(async () => {
    await Promise.resolve();
    await new Promise((resolve) => setTimeout(resolve, 10));
  });
}

describe("Connector editor layout regression guards", () => {
  beforeEach(() => {
    (
      globalThis as typeof globalThis & {
        IS_REACT_ACT_ENVIRONMENT?: boolean;
      }
    ).IS_REACT_ACT_ENVIRONMENT = true;
  });

  it("keeps guided dialogs scrollable with a dedicated body region", async () => {
    const { GuidedFormDialog } = await import("../src/components/operator/GuidedFormDialog");
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <GuidedFormDialog
          open
          onOpenChange={() => {}}
          title="New Connector"
          description="Create a connector"
          wide
          onConfirm={() => {}}
        >
          <div style={{ minHeight: 1600 }}>Tall editor content</div>
        </GuidedFormDialog>,
      );
    });
    await flushAll();

    const dialog = document.querySelector('[role="dialog"]');
    const body = document.querySelector('[data-slot="guided-form-body"]');

    expect(dialog).not.toBeNull();
    expect(dialog?.className).toContain("max-h-[92vh]");
    expect(dialog?.className).toContain("overflow-hidden");
    expect(body).not.toBeNull();
    expect(body?.className).toContain("overflow-y-auto");
    expect(body?.className).toContain("min-h-0");

    root.unmount();
    container.remove();
  });

  it("shows compact connector type options and only expands the active template details", async () => {
    const { ConnectorManifestEditor } = await import("../src/components/operator/ConnectorManifestEditor");
    const { connectorSamples, createEmptyConnectorManifest } = await import("../src/lib/connector-samples");
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);

    let manifest = createEmptyConnectorManifest();
    let selectedTemplateID: string | null = null;
    const renderEditor = () => {
      root.render(
        <MemoryRouter>
          <ConnectorManifestEditor
            manifest={manifest}
            isEdit={false}
            onChange={(next) => {
              manifest = next;
              renderEditor();
            }}
            createTemplates={connectorSamples}
            selectedCreateTemplate={selectedTemplateID}
            onSelectCreateTemplate={(templateID) => {
              selectedTemplateID = templateID;
              renderEditor();
            }}
          />
        </MemoryRouter>,
      );
    };

    await act(async () => {
      renderEditor();
    });
    await flushAll();

    expect(container.textContent || "").not.toContain("Prometheus metrics provider for alert context and trend queries.");
    expect(container.textContent || "").not.toContain("VictoriaMetrics metrics provider for alert context and PromQL-compatible queries.");

    const victoriaMetricsButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("VictoriaMetrics"),
    );

    expect(victoriaMetricsButton).toBeTruthy();

    await act(async () => {
      victoriaMetricsButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    await flushAll();

    expect(container.textContent || "").toContain("VictoriaMetrics metrics provider for alert context and PromQL-compatible queries.");
    expect(container.textContent || "").not.toContain("Prometheus metrics provider for alert context and trend queries.");

    root.unmount();
    container.remove();
  });
});
