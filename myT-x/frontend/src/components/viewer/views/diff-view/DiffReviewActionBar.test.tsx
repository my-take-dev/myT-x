import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {useMCPStore} from "../../../../stores/mcpStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {setLanguage} from "../../../../i18n";
import {DiffReviewActionBar} from "./DiffReviewActionBar";

const handleSendMock = vi.fn();
const handleAddToSingleTaskRunnerMock = vi.fn();
const listMCPServersMock = vi.fn();
const hookComments = [{
    id: "alpha-1",
    sessionKey: "session:1",
    filePath: "a.ts",
    startLineNum: 1,
    startLineType: "added" as const,
    endLineNum: 1,
    endLineType: "added" as const,
    lineContent: "const a = 1;",
    commentText: "alpha",
}];
let hookCommentCount = 1;
let hookSending = false;
let hookRegistering = false;

vi.mock("./useDiffReviewSend", () => ({
    useDiffReviewSend: () => ({
        commentCount: hookCommentCount,
        comments: hookComments,
        sending: hookSending,
        registering: hookRegistering,
        sendError: null,
        handleSend: handleSendMock,
        handleAddToSingleTaskRunner: handleAddToSingleTaskRunnerMock,
    }),
}));

vi.mock("../../../../api", () => ({
    api: {
        ListMCPServers: (sessionName: string) => listMCPServersMock(sessionName),
    },
}));

function buildPane(id: string, index: number, title = "") {
    return {id, index, title, active: false, width: 0, height: 0};
}

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((innerResolve, innerReject) => {
        resolve = innerResolve;
        reject = innerReject;
    });
    return {promise, resolve, reject};
}

function changeTextareaValue(textarea: HTMLTextAreaElement | null, value: string): void {
    if (textarea == null) return;
    const valueSetter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")?.set;
    valueSetter?.call(textarea, value);
    textarea.dispatchEvent(new Event("input", {bubbles: true}));
}

async function selectPane(container: HTMLElement, paneId: string): Promise<void> {
    const select = container.querySelector("select") as HTMLSelectElement | null;
    expect(select).not.toBeNull();
    await act(async () => {
        if (select) {
            select.value = paneId;
            select.dispatchEvent(new Event("change", {bubbles: true}));
        }
    });
}

async function openSendDialog(container: HTMLElement, paneId: string): Promise<void> {
    await selectPane(container, paneId);
    const sendButton = container.querySelector(".diff-review-send-btn") as HTMLButtonElement | null;
    expect(sendButton).not.toBeNull();
    await act(async () => {
        sendButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
    });
}

async function changeDialogMessage(container: HTMLElement, value: string): Promise<void> {
    await act(async () => {
        const message = container.querySelector(".diff-review-send-message") as HTMLTextAreaElement | null;
        changeTextareaValue(message, value);
    });
}

async function clickDialogSend(container: HTMLElement): Promise<void> {
    await act(async () => {
        container.querySelector(".diff-review-send-primary")
            ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        await Promise.resolve();
    });
}

async function clickDialogCancel(container: HTMLElement): Promise<void> {
    await act(async () => {
        (container.querySelector(".diff-review-send-footer .modal-btn") as HTMLButtonElement | null)?.click();
    });
}

describe("DiffReviewActionBar", () => {
    let container: HTMLDivElement;
    let root: Root;
    let confirmSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        handleSendMock.mockReset().mockResolvedValue(true);
        handleAddToSingleTaskRunnerMock.mockReset();
        listMCPServersMock.mockReset().mockResolvedValue([]);
        hookCommentCount = 1;
        hookSending = false;
        hookRegistering = false;
        useDiffReviewStore.setState(
            {
                ...useDiffReviewStore.getState(),
                comments: [
                    {
                        id: "alpha-1",
                        sessionKey: "session:1",
                        filePath: "a.ts",
                        startLineNum: 1,
                        startLineType: "added",
                        endLineNum: 1,
                        endLineType: "added",
                        lineContent: "const a = 1;",
                        commentText: "alpha",
                    },
                    {
                        id: "beta-1",
                        sessionKey: "session:2",
                        filePath: "b.ts",
                        startLineNum: 2,
                        startLineType: "context",
                        endLineNum: 2,
                        endLineType: "context",
                        lineContent: "return value;",
                        commentText: "beta",
                    },
                ],
                drafts: {},
                activeCommentLineKey: null,
            },
            true,
        );
        useMCPStore.setState({
            snapshots: {},
            sessionStates: {},
        });
        confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
        useTmuxStore.setState({
            config: null,
            sessions: [{
                id: 1,
                name: "alpha",
                created_at: "",
                is_idle: false,
                active_window_id: 1,
                windows: [{id: 1, name: "win-1", active_pane: 0, panes: [buildPane("%1", 0, "left"), buildPane("%2", 1, "right")]}],
            }],
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
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        confirmSpy.mockRestore();
        setLanguage("ja");
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("clears an invalid pane selection when the pane list changes", async () => {
        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        const select = container.querySelector("select");
        const sendButton = container.querySelector(".diff-review-send-btn");
        expect(select).not.toBeNull();
        expect(sendButton).not.toBeNull();

        await act(async () => {
            (select as HTMLSelectElement).value = "%1";
            select?.dispatchEvent(new Event("change", {bubbles: true}));
        });

        expect((select as HTMLSelectElement).value).toBe("%1");
        expect((sendButton as HTMLButtonElement).disabled).toBe(false);

        await act(async () => {
            useTmuxStore.setState((state) => ({
                ...state,
                sessions: [{
                    id: 1,
                    name: "alpha",
                    created_at: "",
                    is_idle: false,
                    active_window_id: 1,
                    windows: [{id: 1, name: "win-1", active_pane: 1, panes: [buildPane("%2", 1, "right")]}],
                }],
            }));
            await Promise.resolve();
        });

        expect((select as HTMLSelectElement).value).toBe("");
        expect((sendButton as HTMLButtonElement).disabled).toBe(true);
    });

    it("keeps the send dialog open when pane invalidation discard is cancelled", async () => {
        confirmSpy.mockReturnValue(false);

        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        const select = container.querySelector("select") as HTMLSelectElement | null;
        const sendButton = container.querySelector(".diff-review-send-btn") as HTMLButtonElement | null;
        await act(async () => {
            if (select) {
                select.value = "%1";
                select.dispatchEvent(new Event("change", {bubbles: true}));
            }
        });
        await act(async () => {
            sendButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await act(async () => {
            const message = container.querySelector(".diff-review-send-message") as HTMLTextAreaElement | null;
            changeTextareaValue(message, "Keep the draft");
        });

        await act(async () => {
            useTmuxStore.setState((state) => ({
                ...state,
                sessions: [{
                    id: 1,
                    name: "alpha",
                    created_at: "",
                    is_idle: false,
                    active_window_id: 1,
                    windows: [{id: 1, name: "win-1", active_pane: 1, panes: [buildPane("%2", 1, "right")]}],
                }],
            }));
            await Promise.resolve();
        });

        expect(confirmSpy).toHaveBeenCalledTimes(1);
        expect(container.querySelector(".diff-review-send-dialog")).not.toBeNull();
        expect((container.querySelector(".diff-review-send-primary") as HTMLButtonElement | null)?.disabled).toBe(true);
        expect((container.querySelector(".diff-review-send-message") as HTMLTextAreaElement | null)?.value).toBe(
            "Keep the draft",
        );
    });

    it("keeps comments when destructive clear is cancelled", async () => {
        confirmSpy.mockReturnValue(false);

        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        await act(async () => {
            container.querySelector(".diff-review-clear-btn")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(confirmSpy).toHaveBeenCalledTimes(1);
        expect(useDiffReviewStore.getState().comments).toHaveLength(2);
    });

    it("shows Single Task Runner registration only when its MCP is enabled", async () => {
        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        expect(container.querySelector(".diff-review-str-btn")).toBeNull();

        await act(async () => {
            useMCPStore.setState({
                snapshots: {
                    alpha: [{
                        id: "single-task-runner",
                        name: "Single Task Runner",
                        description: "",
                        enabled: false,
                        status: "stopped",
                        kind: "single-task-runner",
                    }],
                },
            });
            await Promise.resolve();
        });

        expect(container.querySelector(".diff-review-str-btn")).toBeNull();

        await act(async () => {
            useMCPStore.setState({
                snapshots: {
                    alpha: [{
                        id: "single-task-runner",
                        name: "Single Task Runner",
                        description: "",
                        enabled: true,
                        status: "error",
                        kind: "single-task-runner",
                    }],
                },
            });
            await Promise.resolve();
        });

        expect(container.querySelector(".diff-review-str-btn")).toBeNull();

        await act(async () => {
            useMCPStore.setState({
                snapshots: {
                    alpha: [{
                        id: "single-task-runner",
                        name: "Single Task Runner",
                        description: "",
                        enabled: true,
                        status: "stopped",
                        kind: "single-task-runner",
                    }],
                },
            });
            await Promise.resolve();
        });

        expect(container.querySelector(".diff-review-str-btn")).toBeNull();

        await act(async () => {
            useMCPStore.setState({
                snapshots: {
                    alpha: [{
                        id: "single-task-runner",
                        name: "Single Task Runner",
                        description: "",
                        enabled: true,
                        status: "running",
                        kind: "single-task-runner",
                    }],
                },
            });
            await Promise.resolve();
        });

        expect(container.querySelector(".diff-review-str-btn")).not.toBeNull();
    });

    it("loads MCP snapshots on first render so the Single Task Runner action can appear", async () => {
        listMCPServersMock.mockResolvedValueOnce([{
            id: "single-task-runner",
            name: "Single Task Runner",
            description: "",
            enabled: true,
            status: "running",
            kind: "single-task-runner",
        }]);

        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        await act(async () => {
            await Promise.resolve();
        });

        expect(listMCPServersMock).toHaveBeenCalledWith("alpha");
        expect(container.querySelector(".diff-review-str-btn")).not.toBeNull();
        expect(useMCPStore.getState().sessionStates.alpha?.loading).toBe(false);
    });

    it("clears MCP loading when a pending snapshot load resolves after the session changes", async () => {
        const request = deferred<never[]>();
        listMCPServersMock.mockReturnValueOnce(request.promise);

        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        expect(useMCPStore.getState().sessionStates.alpha?.loading).toBe(true);

        await act(async () => {
            useTmuxStore.setState((state) => ({
                ...state,
                activeSession: null,
            }));
            await Promise.resolve();
        });

        await act(async () => {
            request.resolve([]);
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(useMCPStore.getState().sessionStates.alpha?.loading).toBe(false);
    });

    it("registers review comments to Single Task Runner using the selected pane", async () => {
        useMCPStore.setState({
            snapshots: {
                alpha: [{
                    id: "single-task-runner",
                    name: "Single Task Runner",
                    description: "",
                    enabled: true,
                    status: "running",
                    kind: "single-task-runner",
                }],
            },
        });

        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        const strButton = container.querySelector(".diff-review-str-btn") as HTMLButtonElement | null;
        expect(strButton).not.toBeNull();

        await selectPane(container, "%2");

        await act(async () => {
            strButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(handleAddToSingleTaskRunnerMock).toHaveBeenCalledTimes(1);
        expect(handleAddToSingleTaskRunnerMock).toHaveBeenCalledWith("%2");
    });

    it("opens the send preparation dialog after pane selection", async () => {
        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        await openSendDialog(container, "%1");

        expect(container.querySelector(".diff-review-send-dialog")).not.toBeNull();
        expect(container.textContent).toContain("レビュー送信");
        expect(handleSendMock).not.toHaveBeenCalled();
    });

    it("sends the dialog draft to the selected pane from the top-right send button", async () => {
        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        await openSendDialog(container, "%2");

        const primarySend = container.querySelector(".diff-review-send-primary") as HTMLButtonElement | null;
        expect(primarySend).not.toBeNull();

        await act(async () => {
            primarySend?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(handleSendMock).toHaveBeenCalledTimes(1);
        expect(handleSendMock).toHaveBeenCalledWith("%2", expect.objectContaining({
            message: "",
            comments: [expect.objectContaining({id: "alpha-1", commentText: "alpha"})],
        }));
        expect(container.querySelector(".diff-review-send-dialog")).toBeNull();
    });

    it("does not ask to discard a dirty draft when send completion clears comments", async () => {
        const request = deferred<boolean>();
        handleSendMock.mockReturnValueOnce(request.promise);

        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        await openSendDialog(container, "%2");
        await changeDialogMessage(container, "Send this draft");

        await clickDialogSend(container);
        await act(async () => {
            hookCommentCount = 0;
            root.render(<DiffReviewActionBar/>);
            await Promise.resolve();
        });
        await act(async () => {
            request.resolve(true);
            await Promise.resolve();
        });

        expect(confirmSpy).not.toHaveBeenCalled();
        expect(handleSendMock).toHaveBeenCalledTimes(1);
        expect(container.querySelector(".diff-review-send-dialog")).toBeNull();
    });

    it("restores discard confirmation when send returns false", async () => {
        handleSendMock.mockResolvedValueOnce(false);

        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        await openSendDialog(container, "%2");
        await changeDialogMessage(container, "Keep this draft after a blocked send");

        await clickDialogSend(container);
        await clickDialogCancel(container);

        expect(handleSendMock).toHaveBeenCalledTimes(1);
        expect(confirmSpy).toHaveBeenCalledTimes(1);
        expect(container.querySelector(".diff-review-send-dialog")).toBeNull();
    });

    it("does not ask to discard a dirty message-only draft after send succeeds", async () => {
        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        await openSendDialog(container, "%1");
        await act(async () => {
            container.querySelector(".diff-review-send-delete")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await changeDialogMessage(container, "Message only");
        await clickDialogSend(container);

        expect(confirmSpy).not.toHaveBeenCalled();
        expect(handleSendMock).toHaveBeenCalledWith("%1", expect.objectContaining({
            message: "Message only",
            comments: [],
        }));
        expect(container.querySelector(".diff-review-send-dialog")).toBeNull();
    });

    it("keeps a dirty send dialog open without discard confirmation while sending", async () => {
        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        await openSendDialog(container, "%1");
        await changeDialogMessage(container, "Do not close while sending");
        await act(async () => {
            hookSending = true;
            useTmuxStore.setState((state) => ({
                ...state,
                sessions: [...state.sessions],
            }));
            root.render(<DiffReviewActionBar/>);
            await Promise.resolve();
        });

        expect((container.querySelector(".diff-review-send-footer .modal-btn") as HTMLButtonElement | null)?.disabled).toBe(
            true,
        );
        expect(confirmSpy).not.toHaveBeenCalled();
        expect(container.querySelector(".diff-review-send-dialog")).not.toBeNull();
    });

    it("defers pane invalidation close while send is pending and keeps a failed draft", async () => {
        confirmSpy.mockReturnValue(false);
        const request = deferred<boolean>();
        handleSendMock.mockReturnValueOnce(request.promise);

        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        await openSendDialog(container, "%1");
        await changeDialogMessage(container, "Keep this draft if the send fails");
        await clickDialogSend(container);

        await act(async () => {
            useTmuxStore.setState((state) => ({
                ...state,
                sessions: [{
                    id: 1,
                    name: "alpha",
                    created_at: "",
                    is_idle: false,
                    active_window_id: 1,
                    windows: [{id: 1, name: "win-1", active_pane: 1, panes: [buildPane("%2", 1, "right")]}],
                }],
            }));
            await Promise.resolve();
        });

        expect(confirmSpy).not.toHaveBeenCalled();
        expect(container.querySelector(".diff-review-send-dialog")).not.toBeNull();
        expect((container.querySelector(".diff-review-send-message") as HTMLTextAreaElement | null)?.value).toBe(
            "Keep this draft if the send fails",
        );

        await act(async () => {
            request.resolve(false);
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(confirmSpy).toHaveBeenCalledTimes(1);
        expect(container.querySelector(".diff-review-send-dialog")).not.toBeNull();
        expect((container.querySelector(".diff-review-send-message") as HTMLTextAreaElement | null)?.value).toBe(
            "Keep this draft if the send fails",
        );
    });

    it("disables destructive clear while sending", async () => {
        hookSending = true;

        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        const clearButton = container.querySelector(".diff-review-clear-btn") as HTMLButtonElement | null;
        expect(clearButton).not.toBeNull();
        expect(clearButton?.disabled).toBe(true);

        await act(async () => {
            clearButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(confirmSpy).not.toHaveBeenCalled();
        expect(useDiffReviewStore.getState().comments).toHaveLength(2);
    });

    it("renders English dialog labels through the i18n pattern", async () => {
        setLanguage("en");
        await act(async () => {
            root.render(<DiffReviewActionBar/>);
        });

        await openSendDialog(container, "%1");

        expect(container.textContent).toContain("Send Review");
        expect(container.textContent).toContain("Optional message");
        expect(container.textContent).toContain("Edit");
        expect(container.textContent).toContain("Delete");
    });
});
