import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {DiffReviewComment} from "../../../../stores/diffReviewStore";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {useNotificationStore} from "../../../../stores/notificationStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {useDiffReviewSend} from "./useDiffReviewSend";

const sendDiffReviewMock = vi.fn<(paneID: string, markdown: string) => Promise<void>>();
const notifyAndLogMock = vi.fn();
const ALPHA_SESSION_KEY = "session:1";
const BETA_SESSION_KEY = "session:2";

vi.mock("../../../../api", () => ({
    api: {
        SendDiffReview: (paneID: string, markdown: string) => sendDiffReviewMock(paneID, markdown),
    },
}));

vi.mock("../../../../utils/notifyUtils", () => ({
    notifyAndLog: (...args: unknown[]) => notifyAndLogMock(...args),
}));

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((innerResolve, innerReject) => {
        resolve = innerResolve;
        reject = innerReject;
    });
    return {promise, resolve, reject};
}

function resetStore(): void {
    useDiffReviewStore.setState(
        {
            ...useDiffReviewStore.getState(),
            comments: [],
            drafts: {},
            activeCommentLineKey: null,
        },
        true,
    );
    useNotificationStore.setState({notifications: []});
}

function resetTmuxStore(): void {
    useTmuxStore.setState({
        config: null,
        sessions: [
            {id: 1, name: "alpha", created_at: "", is_idle: false, active_window_id: 1, windows: []},
            {id: 2, name: "beta", created_at: "", is_idle: false, active_window_id: 1, windows: []},
        ],
        sessionOrder: ["alpha", "beta"],
        activeSession: "alpha",
        activeWindowId: "1",
        zoomPaneId: null,
        pendingPrefixKillPaneId: null,
        prefixMode: false,
        syncInputMode: false,
        fontSize: 13,
        imeResetSignal: 0,
    });
}

function createComment(overrides: Partial<Omit<DiffReviewComment, "id">> = {}): Omit<DiffReviewComment, "id"> {
    return {
        sessionKey: ALPHA_SESSION_KEY,
        filePath: "a.ts",
        startLineNum: 1,
        startLineType: "added",
        endLineNum: 1,
        endLineType: "added",
        lineContent: "const a = 1;",
        commentText: "first",
        ...overrides,
    };
}

let latestHook: ReturnType<typeof useDiffReviewSend> | null = null;

function HookHarness() {
    latestHook = useDiffReviewSend();
    return null;
}

describe("useDiffReviewSend", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        resetStore();
        resetTmuxStore();
        latestHook = null;
        sendDiffReviewMock.mockReset();
        notifyAndLogMock.mockReset();
        consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        consoleWarnSpy.mockRestore();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("removes only comments captured at send start", async () => {
        const request = deferred<void>();
        sendDiffReviewMock.mockReturnValueOnce(request.promise);
        const store = useDiffReviewStore.getState();
        store.addComment(createComment());
        store.addComment(createComment({
            sessionKey: BETA_SESSION_KEY,
            filePath: "z.ts",
            startLineNum: 9,
            startLineType: "context",
            endLineNum: 9,
            endLineType: "context",
            lineContent: "beta only",
            commentText: "hidden",
        }));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        await act(async () => {
            void latestHook?.handleSend("%1");
            await Promise.resolve();
        });

        await act(async () => {
            useDiffReviewStore.getState().addComment(createComment({
                filePath: "b.ts",
                startLineNum: 2,
                startLineType: "context",
                endLineNum: 2,
                endLineType: "context",
                lineContent: "return value;",
                commentText: "second",
            }));
        });

        await act(async () => {
            request.resolve();
            await Promise.resolve();
        });

        const remaining = useDiffReviewStore.getState().comments;
        expect(sendDiffReviewMock).toHaveBeenCalledTimes(1);
        expect(sendDiffReviewMock.mock.calls[0]?.[1]).toContain("`a.ts` (L+1)");
        expect(sendDiffReviewMock.mock.calls[0]?.[1]).not.toContain("beta only");
        expect(remaining).toHaveLength(2);
        expect(remaining.map((comment) => comment.commentText)).toEqual(["hidden", "second"]);
        expect(useNotificationStore.getState().notifications.at(-1)?.message).toBe("Sent 1 review comment.");
    });

    it("removes sent comments from their original session even after the active session changes", async () => {
        const request = deferred<void>();
        sendDiffReviewMock.mockReturnValueOnce(request.promise);
        const store = useDiffReviewStore.getState();
        store.addComment(createComment({commentText: "alpha only"}));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        await act(async () => {
            void latestHook?.handleSend("%1");
            await Promise.resolve();
        });

        await act(async () => {
            useTmuxStore.setState((state) => ({
                ...state,
                activeSession: "beta",
            }));
            useDiffReviewStore.getState().addComment(createComment({
                sessionKey: BETA_SESSION_KEY,
                filePath: "beta.ts",
                commentText: "beta survives",
            }));
            await Promise.resolve();
        });

        await act(async () => {
            request.resolve();
            await Promise.resolve();
        });

        const remaining = useDiffReviewStore.getState().comments;
        expect(remaining).toHaveLength(1);
        expect(remaining[0]?.sessionKey).toBe(BETA_SESSION_KEY);
        expect(remaining[0]?.commentText).toBe("beta survives");
    });

    it("surfaces send failures without clearing comments", async () => {
        sendDiffReviewMock.mockRejectedValueOnce(new Error("send failed"));
        useDiffReviewStore.getState().addComment(createComment({
            startLineNum: 3,
            startLineType: "removed",
            endLineNum: 4,
            endLineType: "removed",
            lineContent: "legacy()\ncleanup()",
            commentText: "remove this",
        }));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        await act(async () => {
            await latestHook?.handleSend("%2");
        });

        expect(latestHook?.sendError).toBe("send failed");
        expect(useDiffReviewStore.getState().comments).toHaveLength(1);
        expect(notifyAndLogMock).toHaveBeenCalledTimes(1);
        expect(consoleWarnSpy).toHaveBeenCalledTimes(1);
    });

    it("skips stale notifications when the send fails after the active session changes", async () => {
        const request = deferred<void>();
        sendDiffReviewMock.mockReturnValueOnce(request.promise);
        useDiffReviewStore.getState().addComment(createComment({commentText: "alpha only"}));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        await act(async () => {
            void latestHook?.handleSend("%1");
            await Promise.resolve();
        });

        await act(async () => {
            useTmuxStore.setState((state) => ({
                ...state,
                activeSession: "beta",
            }));
            await Promise.resolve();
        });

        await act(async () => {
            request.reject(new Error("send failed after switch"));
            await Promise.resolve();
        });

        expect(notifyAndLogMock).not.toHaveBeenCalled();
        expect(latestHook?.sendError).toBeNull();
        expect(useDiffReviewStore.getState().comments).toHaveLength(1);
    });
});
