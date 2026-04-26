import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {ParsedDiffLine} from "../../../../utils/diffParser";
import {buildDiffReviewDraftKey} from "./diffReviewKeys";
import {DiffReviewLineRow} from "./DiffReviewLineRow";

vi.mock("./DiffCommentForm", () => ({
    DiffCommentForm: ({
        onSave,
        onCancel,
        rangeOptions = [],
        selectedRangeValue = "0",
        onRangeChange,
    }: {
        onSave: (text: string) => void;
        onCancel: () => void;
        rangeOptions?: readonly {value: string; label: string}[];
        selectedRangeValue?: string;
        onRangeChange?: (value: string) => void;
    }) => (
        <>
            <select
                className="mock-range-select"
                value={selectedRangeValue}
                onChange={(e) => onRangeChange?.(e.target.value)}
            >
                {rangeOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                        {option.label}
                    </option>
                ))}
            </select>
            <button type="button" className="mock-save-btn" onClick={() => onSave("context comment")}>
                Save
            </button>
            <button type="button" className="mock-cancel-btn" onClick={onCancel}>
                Cancel
            </button>
        </>
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

describe("DiffReviewLineRow", () => {
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

    it("stores multi-line comments against the selected range", async () => {
        const hunkLines: ParsedDiffLine[] = [
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
        ];
        const line = hunkLines[0]!;

        await act(async () => {
            root.render(
                <DiffReviewLineRow
                    filePath="src/app.ts"
                    lineKey="hunk:10:12:0"
                    line={line}
                    hunkLines={hunkLines}
                    lineIndex={0}
                />,
            );
        });

        const addButton = container.querySelector(".diff-review-add-btn");
        expect(addButton).not.toBeNull();

        await act(async () => {
            addButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const saveButton = container.querySelector(".mock-save-btn");
        expect(saveButton).not.toBeNull();

        const rangeSelect = container.querySelector(".mock-range-select");
        expect(rangeSelect).not.toBeNull();

        await act(async () => {
            (rangeSelect as HTMLSelectElement).value = "1";
            rangeSelect?.dispatchEvent(new Event("change", {bubbles: true}));
        });

        await act(async () => {
            saveButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const [comment] = useDiffReviewStore.getState().comments;
        expect(comment).toBeDefined();
        expect(comment?.sessionKey).toBe("session:1");
        expect(comment?.startLineNum).toBe(12);
        expect(comment?.startLineType).toBe("context");
        expect(comment?.endLineNum).toBe(13);
        expect(comment?.endLineType).toBe("context");
        expect(comment?.lineContent).toBe("shared line\nnext line");
        expect(useDiffReviewStore.getState().activeCommentLineKey).toBeNull();
    });

    it("uses the requested range as the default selection when the form opens", async () => {
        const hunkLines: ParsedDiffLine[] = [
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
        ];

        await act(async () => {
            root.render(
                <DiffReviewLineRow
                    filePath="src/app.ts"
                    lineKey="hunk:10:12:0"
                    line={hunkLines[0]!}
                    hunkLines={hunkLines}
                    lineIndex={0}
                    requestedSelectionEndIndex={2}
                    requestedSelectionToken={1}
                />,
            );
        });

        await act(async () => {
            useDiffReviewStore.getState().setActiveCommentLineKey(
                buildDiffReviewDraftKey("session:1", "src/app.ts", "hunk:10:12:0"),
            );
            await Promise.resolve();
        });

        const rangeSelect = container.querySelector(".mock-range-select") as HTMLSelectElement | null;
        expect(rangeSelect).not.toBeNull();
        expect(rangeSelect?.value).toBe("2");

        await act(async () => {
            container.querySelector(".mock-save-btn")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const [comment] = useDiffReviewStore.getState().comments;
        expect(comment?.startLineNum).toBe(12);
        expect(comment?.endLineNum).toBe(14);
        expect(comment?.lineContent).toBe("shared line\nnext line\nadded line");
    });

    it("closes the form without deleting the draft on cancel", async () => {
        const hunkLines: ParsedDiffLine[] = [{
            type: "context",
            content: "shared line",
            oldLineNum: 10,
            newLineNum: 12,
        }];

        await act(async () => {
            root.render(
                <DiffReviewLineRow
                    filePath="src/app.ts"
                    lineKey="hunk:10:12:0"
                    line={hunkLines[0]!}
                    hunkLines={hunkLines}
                    lineIndex={0}
                />,
            );
        });

        await act(async () => {
            container.querySelector(".diff-review-add-btn")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            useDiffReviewStore.getState().setDraft(
                buildDiffReviewDraftKey("session:1", "src/app.ts", "hunk:10:12:0"),
                "draft text",
            );
            await Promise.resolve();
        });

        await act(async () => {
            container.querySelector(".mock-cancel-btn")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const state = useDiffReviewStore.getState();
        expect(state.activeCommentLineKey).toBeNull();
        expect(state.drafts[buildDiffReviewDraftKey("session:1", "src/app.ts", "hunk:10:12:0")]).toBe("draft text");
    });

    it("disables review creation when there is no active session key", async () => {
        useTmuxStore.setState((state) => ({
            ...state,
            activeSession: null,
        }));
        const hunkLines: ParsedDiffLine[] = [{
            type: "context",
            content: "shared line",
            oldLineNum: 10,
            newLineNum: 12,
        }];

        await act(async () => {
            root.render(
                <DiffReviewLineRow
                    filePath="src/app.ts"
                    lineKey="hunk:10:12:0"
                    line={hunkLines[0]!}
                    hunkLines={hunkLines}
                    lineIndex={0}
                />,
            );
        });

        const addButton = container.querySelector(".diff-review-add-btn") as HTMLButtonElement | null;
        expect(addButton).not.toBeNull();
        expect(addButton?.disabled).toBe(true);

        await act(async () => {
            addButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(useDiffReviewStore.getState().activeCommentLineKey).toBeNull();
    });
});
