import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {DiffReviewComment} from "../../../../stores/diffReviewStore";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {useNotificationStore} from "../../../../stores/notificationStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {useViewerStore} from "../../viewerStore";
import {setLanguage} from "../../../../i18n";
import {useDiffReviewSend} from "./useDiffReviewSend";

const sendDiffReviewMock = vi.fn<(paneID: string, markdown: string) => Promise<void>>();
const addSingleTaskRunnerItemMock = vi.fn<(
    sessionKey: string,
    title: string,
    message: string,
    targetPaneID: string,
    clearBefore: boolean,
    clearCommand: string,
) => Promise<void>>();
const logFrontendEventSafeMock = vi.fn();
const ALPHA_SESSION_KEY = "session:1";
const ALPHA_API_SESSION_KEY = "alpha:1";
const BETA_SESSION_KEY = "session:2";

vi.mock("../../../../api", () => ({
    api: {
        AddSingleTaskRunnerItem: (
            sessionKey: string,
            title: string,
            message: string,
            targetPaneID: string,
            clearBefore: boolean,
            clearCommand: string,
        ) => addSingleTaskRunnerItemMock(sessionKey, title, message, targetPaneID, clearBefore, clearCommand),
        SendDiffReview: (paneID: string, markdown: string) => sendDiffReviewMock(paneID, markdown),
    },
}));

vi.mock("../../../../utils/logFrontendEventSafe", () => ({
    logFrontendEventSafe: (...args: unknown[]) => logFrontendEventSafeMock(...args),
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

function resetViewerStore(): void {
    useViewerStore.setState({
        ...useViewerStore.getState(),
        activeViewId: null,
        viewContext: null,
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
        resetViewerStore();
        latestHook = null;
        sendDiffReviewMock.mockReset();
        addSingleTaskRunnerItemMock.mockReset();
        logFrontendEventSafeMock.mockReset();
        setLanguage("ja");
        consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        consoleWarnSpy.mockRestore();
        setLanguage("ja");
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
        expect(useNotificationStore.getState().notifications.at(-1)?.message).toBe(
            "1件のレビューコメントを送信しました。",
        );
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
        expect(useNotificationStore.getState().notifications.at(-1)?.message).toBe("send failed");
        expect(logFrontendEventSafeMock).toHaveBeenCalledWith("error", "send failed", "DiffReview");
        expect(consoleWarnSpy).toHaveBeenCalledTimes(1);
    });

    it("prepends an optional message and sends edited draft comment text", async () => {
        sendDiffReviewMock.mockResolvedValueOnce(undefined);
        useDiffReviewStore.getState().addComment(createComment({commentText: "stored text"}));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        const original = useDiffReviewStore.getState().comments[0];
        expect(original).not.toBeUndefined();

        await act(async () => {
            await latestHook?.handleSend("%1", {
                message: "Please review the concurrency edge cases.",
                comments: [{...original!, commentText: "edited draft text"}],
            });
        });

        const sentMarkdown = sendDiffReviewMock.mock.calls[0]?.[1] ?? "";
        expect(sentMarkdown.startsWith(
            "# Overall Comment\n\nPlease review the concurrency edge cases.\n\n---\n\n# Code Review Comments",
        )).toBe(true);
        expect(sentMarkdown).toContain("> edited draft text");
        expect(sentMarkdown).not.toContain("> stored text");
        expect(useDiffReviewStore.getState().comments).toHaveLength(0);
    });

    it("leaves comments excluded from the send draft pending after success", async () => {
        sendDiffReviewMock.mockResolvedValueOnce(undefined);
        const store = useDiffReviewStore.getState();
        store.addComment(createComment({commentText: "included"}));
        store.addComment(createComment({
            filePath: "b.ts",
            startLineNum: 2,
            startLineType: "context",
            endLineNum: 2,
            endLineType: "context",
            lineContent: "return value;",
            commentText: "excluded",
        }));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        const [included] = useDiffReviewStore.getState().comments;
        expect(included).not.toBeUndefined();

        await act(async () => {
            await latestHook?.handleSend("%1", {comments: [included!]});
        });

        const remaining = useDiffReviewStore.getState().comments;
        expect(sendDiffReviewMock.mock.calls[0]?.[1]).toContain("> included");
        expect(sendDiffReviewMock.mock.calls[0]?.[1]).not.toContain("> excluded");
        expect(remaining).toHaveLength(1);
        expect(remaining[0]?.commentText).toBe("excluded");
    });

    it("allows message-only sends without removing pending comments", async () => {
        sendDiffReviewMock.mockResolvedValueOnce(undefined);
        useDiffReviewStore.getState().addComment(createComment({commentText: "pending"}));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        await act(async () => {
            await latestHook?.handleSend("%1", {message: "Message only.", comments: []});
        });

        expect(sendDiffReviewMock).toHaveBeenCalledWith("%1", "# Overall Comment\n\nMessage only.");
        expect(useDiffReviewStore.getState().comments).toHaveLength(1);
    });

    it("does not send a completely empty draft payload", async () => {
        useDiffReviewStore.getState().addComment(createComment({commentText: "pending"}));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        let result: boolean | undefined;
        await act(async () => {
            result = await latestHook?.handleSend("%1", {message: " ", comments: []});
        });

        expect(result).toBe(false);
        expect(sendDiffReviewMock).not.toHaveBeenCalled();
        expect(useDiffReviewStore.getState().comments).toHaveLength(1);
    });

    it("logs stale send failures without mutating the current session error state", async () => {
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

        expect(logFrontendEventSafeMock).toHaveBeenCalledWith(
            "error",
            "send failed after switch",
            "DiffReview",
        );
        expect(latestHook?.sendError).toBeNull();
        expect(useNotificationStore.getState().notifications.at(-1)?.message).toBe("send failed after switch");
        expect(useDiffReviewStore.getState().comments).toHaveLength(1);
    });

    it("registers each review comment as a Single Task Runner item", async () => {
        addSingleTaskRunnerItemMock.mockResolvedValue(undefined);
        const store = useDiffReviewStore.getState();
        store.addComment(createComment({commentText: "first"}));
        store.addComment(createComment({
            filePath: "b.ts",
            startLineNum: 2,
            startLineType: "context",
            endLineNum: 2,
            endLineType: "context",
            lineContent: "return value;",
            commentText: "second",
        }));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        await act(async () => {
            await latestHook?.handleAddToSingleTaskRunner("%2");
        });

        expect(addSingleTaskRunnerItemMock).toHaveBeenCalledTimes(2);
        expect(addSingleTaskRunnerItemMock.mock.calls[0]?.[0]).toBe(ALPHA_API_SESSION_KEY);
        expect(addSingleTaskRunnerItemMock.mock.calls[0]?.[1]).toBe("Review: a.ts L+1");
        expect(addSingleTaskRunnerItemMock.mock.calls[0]?.[2]).toContain("first");
        expect(addSingleTaskRunnerItemMock.mock.calls[0]?.[2]).not.toContain("second");
        expect(addSingleTaskRunnerItemMock.mock.calls[0]?.[3]).toBe("%2");
        expect(addSingleTaskRunnerItemMock.mock.calls[0]?.[4]).toBe(false);
        expect(addSingleTaskRunnerItemMock.mock.calls[0]?.[5]).toBe("");
        expect(addSingleTaskRunnerItemMock.mock.calls[1]?.[1]).toBe("Review: b.ts L2");
        expect(addSingleTaskRunnerItemMock.mock.calls[1]?.[2]).toContain("second");
        expect(useDiffReviewStore.getState().comments).toHaveLength(0);
        expect(useViewerStore.getState().activeViewId).toBe("single-task-runner");
    });

    it("keeps unregistered comments when Single Task Runner registration partially fails", async () => {
        addSingleTaskRunnerItemMock
            .mockResolvedValueOnce(undefined)
            .mockRejectedValueOnce(new Error("runner unavailable"));
        const store = useDiffReviewStore.getState();
        store.addComment(createComment({commentText: "registered"}));
        store.addComment(createComment({
            filePath: "b.ts",
            startLineNum: 2,
            startLineType: "context",
            endLineNum: 2,
            endLineType: "context",
            lineContent: "return value;",
            commentText: "still pending",
        }));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        await act(async () => {
            await latestHook?.handleAddToSingleTaskRunner("%1");
        });

        const remaining = useDiffReviewStore.getState().comments;
        expect(addSingleTaskRunnerItemMock).toHaveBeenCalledTimes(2);
        expect(remaining).toHaveLength(1);
        expect(remaining[0]?.commentText).toBe("still pending");
        expect(useViewerStore.getState().activeViewId).toBeNull();
        expect(latestHook?.sendError).toBe("runner unavailable");
        expect(useNotificationStore.getState().notifications.map((notification) => notification.message)).toEqual([
            "1件のレビュータスクを登録した後、登録に失敗しました。残りのコメントはdiff reviewに残りました。",
            "runner unavailable",
        ]);
        expect(logFrontendEventSafeMock).toHaveBeenCalledWith(
            "error",
            "runner unavailable",
            "DiffReview",
        );
    });

    it("keeps all comments when Single Task Runner registration fails before progress", async () => {
        addSingleTaskRunnerItemMock.mockRejectedValueOnce(new Error("runner unavailable"));
        useDiffReviewStore.getState().addComment(createComment({commentText: "still pending"}));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        await act(async () => {
            await latestHook?.handleAddToSingleTaskRunner("%1");
        });

        const remaining = useDiffReviewStore.getState().comments;
        expect(addSingleTaskRunnerItemMock).toHaveBeenCalledTimes(1);
        expect(remaining).toHaveLength(1);
        expect(remaining[0]?.commentText).toBe("still pending");
        expect(useViewerStore.getState().activeViewId).toBeNull();
        expect(latestHook?.sendError).toBe("runner unavailable");
        expect(useNotificationStore.getState().notifications.at(-1)?.message).toBe("runner unavailable");
        expect(logFrontendEventSafeMock).toHaveBeenCalledWith(
            "error",
            "runner unavailable",
            "DiffReview",
        );
    });

    it("skips stale Single Task Runner registration notifications after the active session changes", async () => {
        const request = deferred<void>();
        addSingleTaskRunnerItemMock.mockReturnValueOnce(request.promise);
        useDiffReviewStore.getState().addComment(createComment({commentText: "alpha only"}));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        await act(async () => {
            void latestHook?.handleAddToSingleTaskRunner("%1");
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
        expect(remaining).toHaveLength(2);
        expect(remaining.map((comment) => comment.commentText)).toEqual(["alpha only", "beta survives"]);
        expect(useViewerStore.getState().activeViewId).toBeNull();
        expect(useNotificationStore.getState().notifications).toHaveLength(0);
    });

    it("does not remove registered comments when registration fails after a session switch", async () => {
        const firstRequest = deferred<void>();
        const secondRequest = deferred<void>();
        addSingleTaskRunnerItemMock
            .mockReturnValueOnce(firstRequest.promise)
            .mockReturnValueOnce(secondRequest.promise);
        const store = useDiffReviewStore.getState();
        store.addComment(createComment({commentText: "registered before switch"}));
        store.addComment(createComment({
            filePath: "b.ts",
            startLineNum: 2,
            startLineType: "context",
            endLineNum: 2,
            endLineType: "context",
            lineContent: "return value;",
            commentText: "failed after switch",
        }));

        await act(async () => {
            root.render(<HookHarness/>);
        });

        let registerPromise: Promise<void> | undefined;
        await act(async () => {
            registerPromise = latestHook?.handleAddToSingleTaskRunner("%1");
            await Promise.resolve();
        });

        await act(async () => {
            firstRequest.resolve();
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
            secondRequest.reject(new Error("runner unavailable"));
            await registerPromise;
        });

        const remaining = useDiffReviewStore.getState().comments;
        expect(remaining).toHaveLength(2);
        expect(remaining.map((comment) => comment.commentText)).toEqual(["registered before switch", "failed after switch"]);
        expect(latestHook?.sendError).toBeNull();
        expect(useNotificationStore.getState().notifications.at(-1)?.message).toBe("runner unavailable");
        expect(logFrontendEventSafeMock).toHaveBeenCalledWith(
            "error",
            "runner unavailable",
            "DiffReview",
        );
    });
});
