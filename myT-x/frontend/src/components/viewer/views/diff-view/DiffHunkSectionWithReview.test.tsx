import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {ParsedDiffHunk} from "../../../../utils/diffParser";
import {buildDiffReviewDraftKey} from "./diffReviewKeys";
import {DiffHunkSectionWithReview} from "./DiffHunkSectionWithReview";

vi.mock("./DiffReviewLineRow", () => ({
    DiffReviewLineRow: ({
        lineKey,
        lineIndex,
        requestedSelectionEndIndex,
        onRangeSelectionStart,
        onRangeSelectionHover,
        consumePendingAddClickSuppression,
        isInDragSelection,
        isDragSelectionAnchor,
    }: {
        lineKey: string;
        lineIndex: number;
        requestedSelectionEndIndex?: number;
        onRangeSelectionStart?: (anchorIndex: number) => void;
        onRangeSelectionHover?: (lineIndex: number) => void;
        consumePendingAddClickSuppression?: (lineKey: string) => boolean;
        isInDragSelection?: boolean;
        isDragSelectionAnchor?: boolean;
    }) => (
        <div
            className={`mock-row${isInDragSelection ? " mock-row--selected" : ""}${isDragSelectionAnchor ? " mock-row--anchor" : ""}`}
            data-line-index={lineIndex}
            data-requested-selection-end-index={requestedSelectionEndIndex ?? ""}
        >
            <button
                type="button"
                className={`mock-add-${lineIndex}`}
                onClick={(event) => {
                    event.currentTarget.setAttribute(
                        "data-consume-result",
                        consumePendingAddClickSuppression?.(lineKey) ? "suppressed" : "opened",
                    );
                }}
            >
                Add
            </button>
            <button type="button" className={`mock-start-${lineIndex}`}
                    onMouseDown={() => onRangeSelectionStart?.(lineIndex)}>
                Start
            </button>
            <button type="button" className={`mock-hover-${lineIndex}`}
                    onMouseMove={() => onRangeSelectionHover?.(lineIndex)}>
                Hover
            </button>
        </div>
    ),
}));

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
}

describe("DiffHunkSectionWithReview", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        resetStore();
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
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("propagates the drag-selected range into the anchor row request", async () => {
        const hunk: ParsedDiffHunk = {
            header: "@@ -10,2 +12,3 @@",
            lines: [
                {
                    type: "context",
                    content: "shared line",
                    oldLineNum: 10,
                    newLineNum: 12,
                },
                {
                    type: "context",
                    content: "next line",
                    oldLineNum: 11,
                    newLineNum: 13,
                },
                {
                    type: "added",
                    content: "added line",
                    newLineNum: 14,
                },
            ],
            startOldLine: 10,
            startNewLine: 12,
        };

        await act(async () => {
            root.render(
                <DiffHunkSectionWithReview
                    filePath="src/app.ts"
                    hunk={hunk}
                />,
            );
        });

        await act(async () => {
            container.querySelector(".mock-start-0")
                ?.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, button: 0}));
        });

        await act(async () => {
            container.querySelector(".mock-hover-2")
                ?.dispatchEvent(new MouseEvent("mousemove", {bubbles: true}));
        });

        await act(async () => {
            window.dispatchEvent(new MouseEvent("mouseup", {bubbles: true}));
        });

        const rows = Array.from(container.querySelectorAll(".mock-row"));
        expect(rows[0]?.getAttribute("data-requested-selection-end-index")).toBe("2");
        expect(useDiffReviewStore.getState().activeCommentLineKey).toBe(
            buildDiffReviewDraftKey("session:1", "src/app.ts", "@@ -10,2 +12,3 @@:10:12:0"),
        );
    });

    it("opens the range editor from the first selected line when dragging upward", async () => {
        const hunk: ParsedDiffHunk = {
            header: "@@ -10,2 +12,3 @@",
            lines: [
                {
                    type: "context",
                    content: "shared line",
                    oldLineNum: 10,
                    newLineNum: 12,
                },
                {
                    type: "context",
                    content: "next line",
                    oldLineNum: 11,
                    newLineNum: 13,
                },
                {
                    type: "added",
                    content: "added line",
                    newLineNum: 14,
                },
            ],
            startOldLine: 10,
            startNewLine: 12,
        };

        await act(async () => {
            root.render(<DiffHunkSectionWithReview filePath="src/app.ts" hunk={hunk}/>);
        });

        await act(async () => {
            container.querySelector(".mock-start-2")
                ?.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, button: 0}));
        });

        await act(async () => {
            container.querySelector(".mock-hover-0")
                ?.dispatchEvent(new MouseEvent("mousemove", {bubbles: true}));
        });

        await act(async () => {
            window.dispatchEvent(new MouseEvent("mouseup", {bubbles: true}));
        });

        const rows = Array.from(container.querySelectorAll(".mock-row"));
        expect(rows[0]?.getAttribute("data-requested-selection-end-index")).toBe("2");
        expect(useDiffReviewStore.getState().activeCommentLineKey).toBe(
            buildDiffReviewDraftKey("session:1", "src/app.ts", "@@ -10,2 +12,3 @@:10:12:0"),
        );
    });

    it("does not suppress unrelated add clicks after a drag finishes outside the button", async () => {
        const hunk: ParsedDiffHunk = {
            header: "@@ -10,2 +12,3 @@",
            lines: [
                {
                    type: "context",
                    content: "shared line",
                    oldLineNum: 10,
                    newLineNum: 12,
                },
                {
                    type: "context",
                    content: "next line",
                    oldLineNum: 11,
                    newLineNum: 13,
                },
                {
                    type: "added",
                    content: "added line",
                    newLineNum: 14,
                },
            ],
            startOldLine: 10,
            startNewLine: 12,
        };

        await act(async () => {
            root.render(<DiffHunkSectionWithReview filePath="src/app.ts" hunk={hunk}/>);
        });

        await act(async () => {
            container.querySelector(".mock-start-0")
                ?.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, button: 0}));
            container.querySelector(".mock-hover-2")
                ?.dispatchEvent(new MouseEvent("mousemove", {bubbles: true}));
            window.dispatchEvent(new MouseEvent("mouseup", {bubbles: true}));
        });

        await act(async () => {
            await new Promise<void>((resolve) => {
                requestAnimationFrame(() => resolve());
            });
        });

        await act(async () => {
            container.querySelector(".mock-add-1")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const addButton = container.querySelector(".mock-add-1");
        expect(addButton?.getAttribute("data-consume-result")).toBe("opened");
    });
});
