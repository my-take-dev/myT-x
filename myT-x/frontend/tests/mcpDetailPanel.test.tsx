import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {McpDetailPanel} from "../src/components/viewer/views/mcp-manager/McpDetailPanel";
import {OrchestratorDetailPanel} from "../src/components/viewer/views/mcp-manager/OrchestratorDetailPanel";
import type {MCPSnapshot} from "../src/types/mcp";

const mocked = vi.hoisted(() => ({
    writeClipboardText: vi.fn<(text: string) => Promise<void>>(),
    notifyClipboardFailure: vi.fn(),
}));

vi.mock("../src/utils/clipboardUtils", () => ({
    writeClipboardText: (text: string) => mocked.writeClipboardText(text),
}));

vi.mock("../src/utils/notifyUtils", () => ({
    notifyClipboardFailure: () => mocked.notifyClipboardFailure(),
}));

function flushMicrotasks(): Promise<void> {
    return Promise.resolve();
}

// ---------------------------------------------------------------------------
// Shared test environment helpers (SUG-3)
// ---------------------------------------------------------------------------

function setupTestEnv(): {container: HTMLDivElement; root: Root} {
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    mocked.writeClipboardText.mockReset();
    mocked.notifyClipboardFailure.mockReset();
    mocked.writeClipboardText.mockResolvedValue(undefined);
    vi.useFakeTimers();
    (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
    return {container, root};
}

function teardownTestEnv(container: HTMLDivElement, root: Root) {
    act(() => {
        root.unmount();
    });
    container.remove();
    vi.useRealTimers();
    vi.restoreAllMocks();
    (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
}

// ---------------------------------------------------------------------------
// Sample data factories
// ---------------------------------------------------------------------------

function sampleMCP(): MCPSnapshot {
    return {
        id: "lsp-gopls",
        name: "Go LSP",
        description: "",
        enabled: false,
        status: "stopped",
        bridge_command: `C:\\Program Files\\myT-x\\myT-x.exe`,
    };
}

function sampleOrchMCP(): MCPSnapshot {
    return {
        id: "orch-agent-orchestrator",
        name: "Agent Orchestrator",
        description: "",
        enabled: false,
        status: "stopped",
        bridge_command: `C:\\Program Files\\myT-x\\myT-x.exe`,
    };
}

// ---------------------------------------------------------------------------
// McpDetailPanel
// ---------------------------------------------------------------------------

describe("McpDetailPanel", () => {
    let container: HTMLDivElement;
    let root: Root;
    const totalLspCount = 3;

    function renderPanel(mcp: MCPSnapshot = sampleMCP()) {
        root.render(
            <McpDetailPanel
                representativeMCP={mcp}
                activeSession="session-a"

                totalLspCount={totalLspCount}
            />,
        );
    }

    beforeEach(() => {
        ({container, root} = setupTestEnv());
    });

    afterEach(() => teardownTestEnv(container, root));

    it("renders shared LSP-MCP guidance and client setup examples", () => {
        act(() => {
            renderPanel();
        });

        const text = container.textContent ?? "";
        expect(text).toContain("LSP-MCP");
        expect(text).toContain("mytx-lsp-mcp");
        expect(text).toContain(`--mcp ${"$LSP_NAME"}`);
        expect(text).toContain(String(totalLspCount));
        expect(text).toContain("Claude Code");
        expect(text).toContain("Codex CLI");
        expect(text).toContain("Gemini CLI");
        expect(text).toContain("Copilot CLI");
    });

    it("uses the shared clipboard helper for copy feedback and resets the label", async () => {
        act(() => {
            renderPanel();
        });

        const copyButton = container.querySelector<HTMLButtonElement>(".mcp-copy-btn");
        expect(copyButton).not.toBeNull();

        await act(async () => {
            copyButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await flushMicrotasks();
        });

        expect(mocked.writeClipboardText).toHaveBeenCalledTimes(1);
        expect(mocked.writeClipboardText.mock.calls[0]?.[0]).toContain("\"mytx-lsp-mcp\"");
        expect(mocked.writeClipboardText.mock.calls[0]?.[0]).toContain("$LSP_NAME");
        expect(copyButton?.textContent).toBe("コピー済");
        expect(mocked.notifyClipboardFailure).not.toHaveBeenCalled();

        act(() => {
            vi.advanceTimersByTime(2000);
        });
        expect(copyButton?.textContent).toBe("コピー");
    });

    it("notifies the user when clipboard copy fails", async () => {
        mocked.writeClipboardText.mockRejectedValueOnce(new Error("clipboard denied"));
        const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined);

        act(() => {
            renderPanel();
        });

        const copyButton = container.querySelector<HTMLButtonElement>(".mcp-copy-btn");
        expect(copyButton).not.toBeNull();

        await act(async () => {
            copyButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await flushMicrotasks();
        });

        expect(mocked.notifyClipboardFailure).toHaveBeenCalledTimes(1);
        expect(consoleWarnSpy).toHaveBeenCalled();
        expect(copyButton?.textContent).toBe("コピー");
    });

    it("escapes control characters in the Codex TOML snippet", async () => {
        const mcp = sampleMCP();
        mcp.bridge_command = "C:\\tmp\\myT-x\ttool\r\n\u0000";

        act(() => {
            renderPanel(mcp);
        });

        const copyButtons = container.querySelectorAll<HTMLButtonElement>(".mcp-copy-btn");
        expect(copyButtons.length).toBeGreaterThan(1);

        await act(async () => {
            copyButtons[1]?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await flushMicrotasks();
        });

        const copiedText = mocked.writeClipboardText.mock.calls[0]?.[0] ?? "";
        expect(copiedText).toContain(`command = "C:\\\\tmp\\\\myT-x\\ttool\\r\\n\\u0000"`);
        expect(copiedText).toContain(`args = ["mcp", "stdio", "--mcp", "$LSP_NAME"]`);
    });

    it("escapes quotes in the bridge command preview", () => {
        const mcp = sampleMCP();
        mcp.bridge_command = `C:\\Tools\\my "quoted".exe`;

        act(() => {
            renderPanel(mcp);
        });

        const previews = Array.from(container.querySelectorAll<HTMLElement>(".mcp-detail-usage-pre code"));
        const commandPreview = previews[0]?.textContent ?? "";
        expect(commandPreview).toContain(`\\"quoted\\"`);
        expect(commandPreview).toContain(`"$LSP_NAME"`);
    });

    it("shows a reload hint when bridge command metadata is unavailable", () => {
        const mcp = sampleMCP();
        delete mcp.bridge_command;

        act(() => {
            renderPanel(mcp);
        });

        const text = container.textContent ?? "";
        expect(text).toContain("ブリッジコマンドのメタデータがありません");
        expect(text).toContain("ビューを更新してブリッジコマンドのメタデータを読み込んでください");
        expect(container.querySelectorAll(".mcp-copy-btn")).toHaveLength(0);
    });

    it("renders a placeholder instead of invalid examples when active session is missing", () => {
        act(() => {
            root.render(
                <McpDetailPanel
                    representativeMCP={sampleMCP()}
                    activeSession={null}

                    totalLspCount={3}
                />,
            );
        });

        const text = container.textContent ?? "";
        expect(text).toContain("アクティブなセッションを選択");
        expect(container.querySelectorAll(".mcp-copy-btn")).toHaveLength(0);
    });

    it("renders a placeholder when totalLspCount is zero", () => {
        act(() => {
            root.render(
                <McpDetailPanel
                    representativeMCP={sampleMCP()}
                    activeSession="session-a"
                    totalLspCount={0}
                />,
            );
        });

        const text = container.textContent ?? "";
        expect(text).toContain("このセッションで利用可能なLSP-MCPプロファイルはありません。");
        expect(container.querySelectorAll(".mcp-copy-btn")).toHaveLength(0);
    });

    it("does not leave a timer behind when copy resolves after unmount", async () => {
        let resolveClipboard: (() => void) | null = null;
        mocked.writeClipboardText.mockImplementationOnce(() => new Promise<void>((resolve) => {
            resolveClipboard = resolve;
        }));

        act(() => {
            renderPanel();
        });

        const copyButton = container.querySelector<HTMLButtonElement>(".mcp-copy-btn");
        expect(copyButton).not.toBeNull();

        await act(async () => {
            copyButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await flushMicrotasks();
        });

        act(() => {
            root.render(<></>);
        });

        await act(async () => {
            resolveClipboard?.();
            await flushMicrotasks();
        });

        expect(vi.getTimerCount()).toBe(0);
    });
});

// ---------------------------------------------------------------------------
// OrchestratorDetailPanel
// ---------------------------------------------------------------------------

/** Find the --session section inside the rendered panel. */
function findSessionSection(container: HTMLElement): Element | undefined {
    const sections = container.querySelectorAll(".mcp-connection-section");
    return Array.from(sections).find((s) =>
        s.querySelector("h4")?.textContent === "--session",
    );
}

describe("OrchestratorDetailPanel", () => {
    let container: HTMLDivElement;
    let root: Root;

    function renderOrchPanel(
        activeSession: string | null = "my-session",
        mcp: MCPSnapshot = sampleOrchMCP(),
    ) {
        root.render(
            <OrchestratorDetailPanel
                representativeMCP={mcp}
                activeSession={activeSession}
            />,
        );
    }

    beforeEach(() => {
        ({container, root} = setupTestEnv());
    });

    afterEach(() => teardownTestEnv(container, root));

    // --- Display & content ------------------------------------------------

    it("displays --session with the active session name and a copy button", async () => {
        act(() => renderOrchPanel());

        const text = container.textContent ?? "";
        expect(text).toContain("--session");
        expect(text).toContain(`"--session", "my-session"`);

        const sessionSection = findSessionSection(container);
        expect(sessionSection).not.toBeUndefined();

        const copyBtn = sessionSection?.querySelector<HTMLButtonElement>(".mcp-copy-btn");
        expect(copyBtn).not.toBeNull();

        await act(async () => {
            copyBtn?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await flushMicrotasks();
        });

        expect(mocked.writeClipboardText).toHaveBeenCalledWith(`"--session", "my-session"`);
        expect(copyBtn?.textContent).toBe("コピー済");
    });

    it("resets copied label after 2000ms", async () => {
        act(() => renderOrchPanel());

        const copyBtn = findSessionSection(container)?.querySelector<HTMLButtonElement>(".mcp-copy-btn");
        expect(copyBtn).not.toBeNull();

        await act(async () => {
            copyBtn?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await flushMicrotasks();
        });

        expect(copyBtn?.textContent).toBe("コピー済");

        act(() => {
            vi.advanceTimersByTime(2000);
        });

        expect(copyBtn?.textContent).toBe("コピー");
    });

    // --- Clipboard failure ------------------------------------------------

    it("notifies the user when clipboard copy fails", async () => {
        mocked.writeClipboardText.mockRejectedValueOnce(new Error("clipboard denied"));
        const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined);

        act(() => renderOrchPanel());

        const copyBtn = findSessionSection(container)?.querySelector<HTMLButtonElement>(".mcp-copy-btn");
        expect(copyBtn).not.toBeNull();

        await act(async () => {
            copyBtn?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await flushMicrotasks();
        });

        expect(mocked.notifyClipboardFailure).toHaveBeenCalledTimes(1);
        expect(consoleWarnSpy).toHaveBeenCalled();
        expect(copyBtn?.textContent).toBe("コピー");
    });

    // --- Unmount safety ---------------------------------------------------

    it("does not leave a timer behind when copy resolves after unmount", async () => {
        let resolveClipboard: (() => void) | null = null;
        mocked.writeClipboardText.mockImplementationOnce(() => new Promise<void>((resolve) => {
            resolveClipboard = resolve;
        }));

        act(() => renderOrchPanel());

        const copyBtn = findSessionSection(container)?.querySelector<HTMLButtonElement>(".mcp-copy-btn");
        expect(copyBtn).not.toBeNull();

        await act(async () => {
            copyBtn?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await flushMicrotasks();
        });

        act(() => {
            root.render(<></>);
        });

        await act(async () => {
            resolveClipboard?.();
            await flushMicrotasks();
        });

        expect(vi.getTimerCount()).toBe(0);
    });

    // --- Empty / missing session ------------------------------------------

    it("does not display --session section when session is null", () => {
        act(() => renderOrchPanel(null));

        const text = container.textContent ?? "";
        expect(text).not.toContain("--session");
        expect(text).toContain("アクティブなセッションを選択");
    });

    it("does not display --session section when session is empty string", () => {
        act(() => renderOrchPanel(""));

        const text = container.textContent ?? "";
        expect(text).not.toContain("--session");
        expect(text).toContain("アクティブなセッションを選択");
    });

    it("does not display --session section when session is whitespace only", () => {
        act(() => renderOrchPanel("   "));

        const text = container.textContent ?? "";
        expect(text).not.toContain("--session");
        expect(text).toContain("アクティブなセッションを選択");
    });

    // --- Bridge command unavailable ---------------------------------------

    it("shows session section even when bridge_command is unavailable", () => {
        const mcp = sampleOrchMCP();
        delete mcp.bridge_command;

        act(() => renderOrchPanel("my-session", mcp));

        const text = container.textContent ?? "";
        expect(text).toContain("--session");
        expect(text).toContain(`"--session", "my-session"`);
        expect(text).toContain("ビューを更新してブリッジコマンドのメタデータを読み込んでください");
    });
});
