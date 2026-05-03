import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {DiffReviewComment} from "../../../../stores/diffReviewStore";
import {setLanguage} from "../../../../i18n";
import {DiffReviewSendDialog} from "./DiffReviewSendDialog";
import type {DiffReviewSendPayload} from "./useDiffReviewSend";

function createComment(overrides: Partial<DiffReviewComment> = {}): DiffReviewComment {
    return {
        id: "comment-1",
        sessionKey: "session:1",
        filePath: "src/app.ts",
        startLineNum: 7,
        startLineType: "added",
        endLineNum: 7,
        endLineType: "added",
        lineContent: "const value = read();",
        commentText: "Check this value",
        ...overrides,
    };
}

function changeTextareaValue(textarea: HTMLTextAreaElement | null, value: string): void {
    if (textarea == null) return;
    const valueSetter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")?.set;
    valueSetter?.call(textarea, value);
    textarea.dispatchEvent(new Event("input", {bubbles: true}));
}

describe("DiffReviewSendDialog", () => {
    let container: HTMLDivElement;
    let root: Root;
    let confirmSpy: ReturnType<typeof vi.spyOn>;
    const onClose = vi.fn();
    const onSend = vi.fn<(targetPaneId: string, payload: DiffReviewSendPayload) => Promise<boolean>>();

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        onClose.mockReset();
        onSend.mockReset().mockResolvedValue(true);
        confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
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

    it("disables send when deleted comments leave no message content", async () => {
        await act(async () => {
            root.render(
                <DiffReviewSendDialog
                    open={true}
                    comments={[createComment()]}
                    targetPaneId="%1"
                    sending={false}
                    sendError={null}
                    onClose={onClose}
                    onSend={onSend}
                />,
            );
        });

        await act(async () => {
            container.querySelector(".diff-review-send-delete")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const primary = container.querySelector(".diff-review-send-primary") as HTMLButtonElement | null;
        expect(primary).not.toBeNull();
        expect(primary?.disabled).toBe(true);

        await act(async () => {
            const message = container.querySelector(".diff-review-send-message") as HTMLTextAreaElement | null;
            changeTextareaValue(message, "Message only");
        });

        expect(primary?.disabled).toBe(false);
    });

    it("treats an empty edited comment as invalid until restored or deleted", async () => {
        await act(async () => {
            root.render(
                <DiffReviewSendDialog
                    open={true}
                    comments={[createComment()]}
                    targetPaneId="%1"
                    sending={false}
                    sendError={null}
                    onClose={onClose}
                    onSend={onSend}
                />,
            );
        });

        await act(async () => {
            container.querySelector(".diff-comment-btn")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await act(async () => {
            const editor = container.querySelector(".diff-review-send-comment-editor") as HTMLTextAreaElement | null;
            changeTextareaValue(editor, "");
        });

        const primary = container.querySelector(".diff-review-send-primary") as HTMLButtonElement | null;
        const done = container.querySelector(".diff-comment-btn") as HTMLButtonElement | null;
        expect(primary?.disabled).toBe(true);
        expect(done?.disabled).toBe(false);
        expect(container.textContent).toContain("コメント本文を入力するか、この行を削除してください。");
    });

    it("keeps the dialog draft open when sending fails", async () => {
        onSend.mockResolvedValueOnce(false);
        await act(async () => {
            root.render(
                <DiffReviewSendDialog
                    open={true}
                    comments={[createComment()]}
                    targetPaneId="%1"
                    sending={false}
                    sendError="Failed to send"
                    onClose={onClose}
                    onSend={onSend}
                />,
            );
        });

        await act(async () => {
            const message = container.querySelector(".diff-review-send-message") as HTMLTextAreaElement | null;
            changeTextareaValue(message, "Keep this message");
        });
        await act(async () => {
            container.querySelector(".diff-comment-btn")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await act(async () => {
            const editor = container.querySelector(".diff-review-send-comment-editor") as HTMLTextAreaElement | null;
            changeTextareaValue(editor, "Edited but not sent");
        });
        await act(async () => {
            container.querySelector(".diff-review-send-primary")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(onClose).not.toHaveBeenCalled();
        expect(container.querySelector(".diff-review-send-dialog")).not.toBeNull();
        expect((container.querySelector(".diff-review-send-message") as HTMLTextAreaElement | null)?.value).toBe(
            "Keep this message",
        );
        expect((container.querySelector(".diff-review-send-comment-editor") as HTMLTextAreaElement | null)?.value).toBe(
            "Edited but not sent",
        );
    });

    it("asks before closing a dirty draft and keeps it when discard is cancelled", async () => {
        confirmSpy.mockReturnValue(false);
        await act(async () => {
            root.render(
                <DiffReviewSendDialog
                    open={true}
                    comments={[createComment()]}
                    targetPaneId="%1"
                    sending={false}
                    sendError={null}
                    onClose={onClose}
                    onSend={onSend}
                />,
            );
        });

        await act(async () => {
            const message = container.querySelector(".diff-review-send-message") as HTMLTextAreaElement | null;
            changeTextareaValue(message, "Do not lose this");
        });
        await act(async () => {
            container.querySelector(".modal-footer .modal-btn")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(confirmSpy).toHaveBeenCalledTimes(1);
        expect(onClose).not.toHaveBeenCalled();
        expect(container.querySelector(".diff-review-send-dialog")).not.toBeNull();
        expect((container.querySelector(".diff-review-send-message") as HTMLTextAreaElement | null)?.value).toBe(
            "Do not lose this",
        );
    });

    it("discards draft edits after close and recreates the opening snapshot", async () => {
        const comment = createComment({commentText: "Original text"});
        await act(async () => {
            root.render(
                <DiffReviewSendDialog
                    open={true}
                    comments={[comment]}
                    targetPaneId="%1"
                    sending={false}
                    sendError={null}
                    onClose={onClose}
                    onSend={onSend}
                />,
            );
        });

        await act(async () => {
            container.querySelector(".diff-comment-btn")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await act(async () => {
            const editor = container.querySelector(".diff-review-send-comment-editor") as HTMLTextAreaElement | null;
            changeTextareaValue(editor, "Unsaved edit");
        });

        await act(async () => {
            root.render(
                <DiffReviewSendDialog
                    open={false}
                    comments={[comment]}
                    targetPaneId="%1"
                    sending={false}
                    sendError={null}
                    onClose={onClose}
                    onSend={onSend}
                />,
            );
        });
        await act(async () => {
            root.render(
                <DiffReviewSendDialog
                    open={true}
                    comments={[comment]}
                    targetPaneId="%1"
                    sending={false}
                    sendError={null}
                    onClose={onClose}
                    onSend={onSend}
                />,
            );
        });

        expect(container.querySelector(".diff-review-send-comment-text")?.textContent).toBe("Original text");
    });

    it("does not render while the target pane is empty", async () => {
        await act(async () => {
            root.render(
                <DiffReviewSendDialog
                    open={true}
                    comments={[createComment()]}
                    targetPaneId=""
                    sending={false}
                    sendError={null}
                    onClose={onClose}
                    onSend={onSend}
                />,
            );
        });

        expect(container.querySelector(".diff-review-send-dialog")).toBeNull();
    });
});
