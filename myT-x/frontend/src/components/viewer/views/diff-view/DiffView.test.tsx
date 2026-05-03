import {act, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {devpanel} from "../../../../../wailsjs/go/models";
import type {DiffReviewComment} from "../../../../stores/diffReviewStore";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {useViewerStore} from "../../viewerStore";
import {DiffView} from "./DiffView";

const useDiffViewMock = vi.fn();

vi.mock("./useDiffView", () => ({
    useDiffView: () => useDiffViewMock(),
}));

vi.mock("../shared/ViewerPanelShell", () => ({
    ViewerPanelShell: ({
        children,
        message,
        onRefresh,
        refreshTitle,
    }: {
        children?: ReactNode;
        message?: string;
        onRefresh?: () => void;
        refreshTitle?: string;
    }) => (
        <div>
            {onRefresh && (
                <button type="button" aria-label={refreshTitle} onClick={onRefresh}>
                    shell refresh
                </button>
            )}
            {message && <div>{message}</div>}
            {children}
        </div>
    ),
}));

vi.mock("./CommitPanel", () => ({
    CommitPanel: () => <div data-testid="commit-panel"/>,
}));

vi.mock("./DiffContentViewer", () => ({
    DiffContentViewer: () => <div data-testid="diff-content"/>,
}));

vi.mock("./DiffFileSidebar", () => ({
    DiffFileSidebar: () => <div data-testid="diff-sidebar"/>,
}));

vi.mock("./DiffReviewActionBar", () => ({
    DiffReviewActionBar: () => <div data-testid="diff-review-action-bar"/>,
}));

vi.mock("./DiffViewModeToggle", () => ({
    DiffViewModeToggle: () => <div data-testid="diff-view-mode-toggle"/>,
}));

vi.mock("./StagingFlatView", () => ({
    StagingFlatView: () => <div data-testid="staging-flat-view"/>,
}));

function resetDiffReviewStore(): void {
    useDiffReviewStore.setState(
        {
            ...useDiffReviewStore.getState(),
            comments: [],
            drafts: {},
            activeCommentLineKey: null,
        },
        true,
    );
}

function buildComment(overrides: Partial<Omit<DiffReviewComment, "id">> = {}): Omit<DiffReviewComment, "id"> {
    return {
        sessionKey: "session:1",
        filePath: "a.ts",
        startLineNum: 1,
        startLineType: "added",
        endLineNum: 1,
        endLineType: "added",
        lineContent: "const a = 1;",
        commentText: "first comment",
        ...overrides,
    };
}

function buildDiff(diff: string): devpanel.WorkingDiffResult {
    return new devpanel.WorkingDiffResult({
        files: [{path: "a.ts", old_path: "", status: "modified", additions: 1, deletions: 0, diff}],
        total_added: 1,
        total_deleted: 0,
        truncated: false,
    });
}

function buildUseDiffViewState(
    diffResult: devpanel.WorkingDiffResult,
    overrides: { readonly loadDiff?: () => void; readonly error?: string | null; readonly activeSession?: string | null } = {},
) {
    return {
        flatNodes: [],
        selectedPath: "a.ts",
        selectedFile: diffResult.files?.[0] ?? null,
        diffResult,
        isLoading: false,
        error: null,
        toggleDir: vi.fn(),
        selectFile: vi.fn(),
        loadDiff: vi.fn(),
        activeSession: "alpha",
        sidebarMode: "tree" as const,
        setSidebarMode: vi.fn(),
        stagingItems: [],
        stagedCount: 0,
        unstagedCount: 0,
        branchInfo: null,
        toggleStagingGroup: vi.fn(),
        operationInFlight: false,
        stageFile: vi.fn(),
        unstageFile: vi.fn(),
        discardFile: vi.fn(),
        stageAll: vi.fn(),
        unstageAll: vi.fn(),
        commit: vi.fn(),
        commitAndPush: vi.fn(),
        push: vi.fn(),
        pull: vi.fn(),
        fetch: vi.fn(),
        commitMessage: "",
        setCommitMessage: vi.fn(),
        ...overrides,
    };
}

describe("DiffView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        resetDiffReviewStore();
        useDiffViewMock.mockReset();
        useTmuxStore.setState({
            config: null,
            sessions: [{id: 1, name: "alpha", created_at: "", is_idle: false, active_window_id: 1, windows: []}],
            sessionOrder: ["alpha"],
            activeSession: "alpha",
            activeWindowId: "1",
            zoomPaneId: null,
            pendingPrefixKillPaneId: null,
            prefixMode: false,
            syncInputMode: false,
            fontSize: 13,
            imeResetSignal: 0,
        });
        useViewerStore.setState({
            activeViewId: null,
            viewContext: null,
            dockRatio: useViewerStore.getState().dockRatio,
        });
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("keeps session comments and shows a warning when the diff generation changes", async () => {
        const firstDiff = buildDiff("@@ -1 +1 @@\n-old\n+new");
        const nextDiff = buildDiff("@@ -1 +1 @@\n-old\n+newer");
        useDiffReviewStore.getState().addComment(buildComment());
        useDiffViewMock.mockReturnValue(buildUseDiffViewState(firstDiff));

        await act(async () => {
            root.render(<DiffView/>);
        });

        expect(container.textContent).not.toContain("Diff changed after review comments were added.");
        expect(useDiffReviewStore.getState().comments).toHaveLength(1);

        useDiffViewMock.mockReturnValue(buildUseDiffViewState(nextDiff));
        await act(async () => {
            root.render(<DiffView/>);
        });

        expect(useDiffReviewStore.getState().comments).toHaveLength(1);
        expect(container.textContent).toContain(
            "Diff changed after review comments were added. Verify line references before sending.",
        );
    });

    it("preserves session drafts when a non-empty diff generation changes", async () => {
        const firstDiff = buildDiff("@@ -1 +1 @@\n-old\n+new");
        const nextDiff = buildDiff("@@ -1 +1 @@\n-old\n+newer");
        useDiffReviewStore.getState().setDraft("session:1::a.ts\u001fhunk:1:1:0", "draft text");
        useDiffViewMock.mockReturnValue(buildUseDiffViewState(firstDiff));

        await act(async () => {
            root.render(<DiffView/>);
        });

        useDiffViewMock.mockReturnValue(buildUseDiffViewState(nextDiff));
        await act(async () => {
            root.render(<DiffView/>);
        });

        expect(useDiffReviewStore.getState().drafts["session:1::a.ts\u001fhunk:1:1:0"]).toBe("draft text");
        expect(container.textContent).toContain(
            "Diff changed while review comments were being prepared. Draft inputs were preserved.",
        );
    });

    it("refreshes the diff when the refresh button is clicked", async () => {
        const loadDiff = vi.fn();
        useDiffViewMock.mockReturnValue(buildUseDiffViewState(buildDiff("@@ -1 +1 @@\n-old\n+new"), {loadDiff}));

        await act(async () => {
            root.render(<DiffView/>);
        });

        const refreshButton = container.querySelector<HTMLButtonElement>("button[aria-label='Diff を更新']");
        expect(refreshButton).not.toBeNull();

        await act(async () => {
            refreshButton?.click();
        });

        expect(loadDiff).toHaveBeenCalledTimes(1);
    });

    it("refreshes the diff from the error shell refresh button", async () => {
        const loadDiff = vi.fn();
        useDiffViewMock.mockReturnValue(buildUseDiffViewState(buildDiff("@@ -1 +1 @@\n-old\n+new"), {
            error: "load failed",
            loadDiff,
        }));

        await act(async () => {
            root.render(<DiffView/>);
        });

        const refreshButton = container.querySelector<HTMLButtonElement>("button[aria-label='Diff を更新']");
        expect(refreshButton).not.toBeNull();

        await act(async () => {
            refreshButton?.click();
        });

        expect(loadDiff).toHaveBeenCalledTimes(1);
    });
});
