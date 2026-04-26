import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {DiffReviewActionBar} from "./DiffReviewActionBar";

const handleSendMock = vi.fn();

vi.mock("./useDiffReviewSend", () => ({
    useDiffReviewSend: () => ({
        commentCount: 1,
        sending: false,
        sendError: null,
        handleSend: handleSendMock,
    }),
}));

function buildPane(id: string, index: number, title = "") {
    return {id, index, title, active: false, width: 0, height: 0};
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
        handleSendMock.mockReset();
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
});
