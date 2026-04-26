import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {DiffCommentForm} from "./DiffCommentForm";

const isImeTransitionalEventMock = vi.fn<(event: Event) => boolean>(() => false);

vi.mock("../../../../utils/ime", () => ({
    isImeTransitionalEvent: (event: Event) => isImeTransitionalEventMock(event),
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

describe("DiffCommentForm", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        resetStore();
        isImeTransitionalEventMock.mockReset();
        isImeTransitionalEventMock.mockReturnValue(false);
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("hydrates the stored draft and saves it with Ctrl+Enter", async () => {
        const onSave = vi.fn();
        const onCancel = vi.fn();
        useDiffReviewStore.getState().setDraft("line-1", "existing draft");

        await act(async () => {
            root.render(<DiffCommentForm draftKey="line-1" onSave={onSave} onCancel={onCancel}/>);
        });

        const textarea = container.querySelector("textarea");
        expect(textarea).not.toBeNull();
        expect((textarea as HTMLTextAreaElement).value).toBe("existing draft");

        await act(async () => {
            useDiffReviewStore.getState().setDraft("line-1", "updated draft");
            await Promise.resolve();
        });

        expect((textarea as HTMLTextAreaElement).value).toBe("updated draft");

        await act(async () => {
            textarea?.dispatchEvent(new KeyboardEvent("keydown", {key: "Enter", ctrlKey: true, bubbles: true}));
        });

        expect(onSave).toHaveBeenCalledWith("updated draft");
        expect(onCancel).not.toHaveBeenCalled();
        expect(useDiffReviewStore.getState().drafts["line-1"]).toBe("updated draft");
    });

    it("does not save while IME transitional handling is active", async () => {
        const onSave = vi.fn();

        await act(async () => {
            root.render(<DiffCommentForm draftKey="line-2" onSave={onSave} onCancel={vi.fn()}/>);
        });

        const textarea = container.querySelector("textarea");
        expect(textarea).not.toBeNull();

        await act(async () => {
            useDiffReviewStore.getState().setDraft("line-2", "ime text");
            await Promise.resolve();
        });

        isImeTransitionalEventMock.mockReturnValueOnce(true);
        await act(async () => {
            textarea?.dispatchEvent(new KeyboardEvent("keydown", {key: "Enter", ctrlKey: true, bubbles: true}));
        });

        expect(onSave).not.toHaveBeenCalled();
    });

    it("does not save on modified Enter chords beyond Ctrl+Enter", async () => {
        const onSave = vi.fn();

        await act(async () => {
            root.render(<DiffCommentForm draftKey="line-2b" onSave={onSave} onCancel={vi.fn()}/>);
        });

        const textarea = container.querySelector("textarea");
        expect(textarea).not.toBeNull();

        await act(async () => {
            useDiffReviewStore.getState().setDraft("line-2b", "keep editing");
            await Promise.resolve();
        });

        await act(async () => {
            textarea?.dispatchEvent(new KeyboardEvent("keydown", {
                key: "Enter",
                ctrlKey: true,
                shiftKey: true,
                bubbles: true,
            }));
        });

        expect(onSave).not.toHaveBeenCalled();
    });

    it("cancels the form on Escape", async () => {
        const onCancel = vi.fn();

        await act(async () => {
            root.render(<DiffCommentForm draftKey="line-3" onSave={vi.fn()} onCancel={onCancel}/>);
        });

        const textarea = container.querySelector("textarea");
        expect(textarea).not.toBeNull();

        await act(async () => {
            textarea?.dispatchEvent(new KeyboardEvent("keydown", {key: "Escape", bubbles: true}));
        });

        expect(onCancel).toHaveBeenCalledTimes(1);
    });

    it("allows selecting a multi-line range", async () => {
        const onRangeChange = vi.fn();

        await act(async () => {
            root.render(
                <DiffCommentForm
                    draftKey="line-4"
                    onSave={vi.fn()}
                    onCancel={vi.fn()}
                    rangeOptions={[
                        {value: "0", label: "L12"},
                        {value: "2", label: "L12 to L14"},
                    ]}
                    selectedRangeValue="0"
                    onRangeChange={onRangeChange}
                />,
            );
        });

        const select = container.querySelector(".diff-comment-range-select");
        expect(select).not.toBeNull();

        await act(async () => {
            (select as HTMLSelectElement).value = "2";
            select?.dispatchEvent(new Event("change", {bubbles: true}));
        });

        expect(onRangeChange).toHaveBeenCalledWith("2");
    });
});
