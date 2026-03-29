import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {SessionSnapshot} from "../types/tmux";
import {setLanguage} from "../i18n";
import {useCanvasStore} from "../stores/canvasStore";
import {useTmuxStore} from "../stores/tmuxStore";
import {SessionView} from "./SessionView";

const createPaneInSessionMock = vi.fn<(sessionName: string) => Promise<string>>();

vi.mock("../api", () => ({
    api: {
        CreatePaneInSession: (sessionName: string) => createPaneInSessionMock(sessionName),
        QuickStartSession: vi.fn(),
        FocusPane: vi.fn(),
        SplitPane: vi.fn(),
        KillPane: vi.fn(),
        RenamePane: vi.fn(),
        SwapPanes: vi.fn(),
        DetachSession: vi.fn(),
    },
}));

vi.mock("./LayoutPresetSelector", () => ({
    LayoutPresetSelector: () => null,
}));

vi.mock("./LayoutRenderer", () => ({
    LayoutRenderer: () => null,
}));

vi.mock("./canvas/CanvasModeToggle", () => ({
    CanvasModeToggle: () => null,
}));

vi.mock("./canvas/CanvasView", () => ({
    CanvasView: () => null,
}));

vi.mock("@xyflow/react", () => ({
    ReactFlowProvider: ({children}: {children: unknown}) => children,
}));

function emptySession(name = "demo"): SessionSnapshot {
    return {
        id: 1,
        name,
        created_at: "2026-03-29T00:00:00Z",
        is_idle: false,
        active_window_id: -1,
        windows: [],
    };
}

describe("SessionView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        createPaneInSessionMock.mockReset();
        setLanguage("ja");
        useTmuxStore.setState({
            sessions: [],
            sessionOrder: [],
            activeSession: null,
            activeWindowId: null,
            zoomPaneId: null,
            pendingPrefixKillPaneId: null,
            prefixMode: false,
            syncInputMode: false,
            fontSize: 13,
            imeResetSignal: 0,
        });
        useCanvasStore.setState({
            mode: "simple",
            activeSessionName: null,
            nodePositions: {},
            nodeSizes: {},
            taskEdgeMap: {},
            agentMap: {},
            processStatusMap: {},
            sessionDataMap: {},
        });
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("renders empty-session recovery UI", () => {
        act(() => {
            root.render(<SessionView session={emptySession()} />);
        });

        expect(container.textContent).toContain("全てのペインが閉じられました。");
        expect(container.textContent).toContain("+ 新しいペイン");
    });

    it("creates a pane for an empty session", async () => {
        createPaneInSessionMock.mockResolvedValue("%42");

        await act(async () => {
            root.render(<SessionView session={emptySession("alpha")} />);
        });

        const button = container.querySelector("button");
        if (!(button instanceof HTMLButtonElement)) {
            throw new Error("expected action button");
        }

        await act(async () => {
            button.click();
        });

        expect(createPaneInSessionMock).toHaveBeenCalledTimes(1);
        expect(createPaneInSessionMock).toHaveBeenCalledWith("alpha");
        expect(container.textContent).not.toContain("作成中...");
    });
});
