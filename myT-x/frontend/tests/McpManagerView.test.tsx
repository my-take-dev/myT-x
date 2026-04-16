import {act, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {MCPSnapshot} from "../src/types/mcp";

const mocked = vi.hoisted(() => {
    const closeView = vi.fn();
    const retryLoad = vi.fn();
    const dismissError = vi.fn();

    const state = {
        lspMcpList: [] as MCPSnapshot[],
        customMcpList: [] as MCPSnapshot[],
        orchMcpList: [] as MCPSnapshot[],
        strMcpList: [] as MCPSnapshot[],
        representativeMCP: null as MCPSnapshot | null,
        customRepresentativeMCP: null as MCPSnapshot | null,
        orchRepresentativeMCP: null as MCPSnapshot | null,
        strRepresentativeMCP: null as MCPSnapshot | null,
        isLoading: false,
        error: null as string | null,
        activeSession: "session-a" as string | null,
        activeSessionKey: "session-a:1",
        retryLoad,
        dismissError,
    };

    return {closeView, retryLoad, dismissError, state};
});

vi.mock("../src/i18n", () => ({
    useI18n: () => ({
        language: "en",
        t: (_key: string, fallback: string) => fallback,
    }),
}));

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: {
        closeView: () => void
    }) => unknown) => selector({closeView: mocked.closeView}),
}));

vi.mock("../src/components/viewer/views/mcp-manager/useMcpManager", () => ({
    useMcpManager: () => mocked.state,
}));

vi.mock("../src/components/viewer/views/shared/ViewerPanelShell", () => ({
    ViewerPanelShell: ({children, message}: { children?: ReactNode; message?: string }) => (
        <div>
            {message ? <div>{message}</div> : null}
            {children}
        </div>
    ),
}));

vi.mock("../src/components/viewer/views/mcp-manager/McpDetailPanel", () => ({
    McpDetailPanel: () => <div data-panel="lsp">lsp-panel</div>,
}));

vi.mock("../src/components/viewer/views/mcp-manager/OrchestratorDetailPanel", () => ({
    OrchestratorDetailPanel: () => <div data-panel="orchestrator">orchestrator-panel</div>,
}));

vi.mock("../src/components/viewer/views/mcp-manager/SingleTaskRunnerDetailPanel", () => ({
    SingleTaskRunnerDetailPanel: () => <div data-panel="single-task-runner">single-task-runner-panel</div>,
}));

vi.mock("../src/components/viewer/views/mcp-manager/CustomMcpDetailPanel", () => ({
    CustomMcpDetailPanel: () => <div data-panel="custom">custom-panel</div>,
}));

import {McpManagerView} from "../src/components/viewer/views/mcp-manager/McpManagerView";

function makeSnapshot(id: string, kind = ""): MCPSnapshot {
    return {
        id,
        kind,
        name: id,
        description: "",
        enabled: false,
        status: "stopped",
    };
}

function setMockState(next: Partial<typeof mocked.state>) {
    Object.assign(mocked.state, next);
}

describe("McpManagerView", () => {
    let container: HTMLDivElement;
    let root: Root;

    function renderView() {
        act(() => {
            root.render(<McpManagerView/>);
        });
    }

    function clickCategory(label: string) {
        const category = Array.from(container.querySelectorAll<HTMLElement>(".mcp-category-item")).find((node) =>
            (node.textContent ?? "").includes(label),
        );
        expect(category).toBeTruthy();
        act(() => {
            category?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
    }

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        mocked.closeView.mockReset();
        mocked.retryLoad.mockReset();
        mocked.dismissError.mockReset();
        setMockState({
            lspMcpList: [],
            customMcpList: [],
            orchMcpList: [],
            strMcpList: [],
            representativeMCP: null,
            customRepresentativeMCP: null,
            orchRepresentativeMCP: null,
            strRepresentativeMCP: null,
            isLoading: false,
            error: null,
            activeSession: "session-a",
            activeSessionKey: "session-a:1",
        });
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("prefers the orchestrator category on the first resolved render", () => {
        setMockState({
            lspMcpList: [makeSnapshot("lsp-gopls")],
            orchMcpList: [makeSnapshot("orch-agent-orchestrator", "orchestrator")],
            strMcpList: [makeSnapshot("single-task-runner", "single-task-runner")],
        });

        renderView();

        expect(container.textContent).toContain("orchestrator-panel");
    });

    it("prefers the orchestrator category when every MCP kind is available", () => {
        setMockState({
            lspMcpList: [makeSnapshot("lsp-gopls")],
            customMcpList: [makeSnapshot("memory")],
            orchMcpList: [makeSnapshot("orch-agent-orchestrator", "orchestrator")],
            strMcpList: [makeSnapshot("single-task-runner", "single-task-runner")],
        });

        renderView();

        expect(container.textContent).toContain("orchestrator-panel");
    });

    it("prefers the single-task-runner category over lsp when orchestrator is absent", () => {
        setMockState({
            lspMcpList: [makeSnapshot("lsp-gopls")],
            orchMcpList: [],
            strMcpList: [makeSnapshot("single-task-runner", "single-task-runner")],
        });

        renderView();

        expect(container.textContent).toContain("single-task-runner-panel");
    });

    it("falls back to the custom category when only custom MCPs are available", () => {
        setMockState({
            lspMcpList: [],
            customMcpList: [makeSnapshot("memory")],
            orchMcpList: [],
            strMcpList: [],
        });

        renderView();

        expect(container.textContent).toContain("custom-panel");
    });

    it("keeps the user-selected category across reload completion", () => {
        setMockState({
            lspMcpList: [makeSnapshot("lsp-gopls")],
            orchMcpList: [makeSnapshot("orch-agent-orchestrator", "orchestrator")],
            strMcpList: [],
        });

        renderView();
        clickCategory("LSP-MCP");
        expect(container.textContent).toContain("lsp-panel");

        setMockState({isLoading: true});
        renderView();
        setMockState({isLoading: false});
        renderView();

        expect(container.textContent).toContain("lsp-panel");
    });

    it("falls back to an available category when the current one disappears", () => {
        setMockState({
            lspMcpList: [],
            orchMcpList: [makeSnapshot("orch-agent-orchestrator", "orchestrator")],
            strMcpList: [],
        });

        renderView();
        expect(container.textContent).toContain("orchestrator-panel");

        setMockState({
            lspMcpList: [makeSnapshot("lsp-gopls")],
            orchMcpList: [],
        });
        renderView();

        expect(container.textContent).toContain("lsp-panel");
    });

    it("reselects the preferred category after a session switch resolves new snapshots", () => {
        setMockState({
            activeSession: "session-a",
            activeSessionKey: "session-a:1",
            lspMcpList: [makeSnapshot("lsp-gopls")],
            orchMcpList: [],
            strMcpList: [],
            isLoading: false,
        });
        renderView();
        expect(container.textContent).toContain("lsp-panel");

        setMockState({
            activeSession: "session-b",
            activeSessionKey: "session-b:2",
            lspMcpList: [makeSnapshot("lsp-gopls")],
            orchMcpList: [],
            strMcpList: [],
            isLoading: false,
        });
        renderView();
        expect(container.textContent).toContain("lsp-panel");

        setMockState({isLoading: true});
        renderView();

        setMockState({
            isLoading: false,
            lspMcpList: [makeSnapshot("lsp-gopls")],
            strMcpList: [makeSnapshot("single-task-runner", "single-task-runner")],
        });
        renderView();

        expect(container.textContent).toContain("single-task-runner-panel");
    });
});
