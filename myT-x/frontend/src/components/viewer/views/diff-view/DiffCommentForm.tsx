import type {KeyboardEvent} from "react";
import {memo, useCallback, useEffect, useRef} from "react";
import {useDiffReviewStore} from "../../../../stores/diffReviewStore";
import {isImeTransitionalEvent} from "../../../../utils/ime";

export interface DiffCommentRangeOption {
    readonly value: string;
    readonly label: string;
}

interface DiffCommentFormProps {
    readonly draftKey: string;
    readonly onSave: (text: string) => void;
    readonly onCancel: () => void;
    readonly rangeOptions?: readonly DiffCommentRangeOption[];
    readonly selectedRangeValue?: string;
    readonly onRangeChange?: (value: string) => void;
}

export const DiffCommentForm = memo(function DiffCommentForm({
    draftKey,
    onSave,
    onCancel,
    rangeOptions,
    selectedRangeValue,
    onRangeChange,
}: DiffCommentFormProps) {
    const draftText = useDiffReviewStore((state) => state.drafts[draftKey] ?? "");
    const setDraft = useDiffReviewStore((state) => state.setDraft);
    const clearDraft = useDiffReviewStore((state) => state.clearDraft);
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const composingRef = useRef(false);
    const showRangeSelector = (rangeOptions?.length ?? 0) > 1 && selectedRangeValue != null && onRangeChange != null;

    useEffect(() => {
        textareaRef.current?.focus();
    }, [draftKey]);

    const handleChange = useCallback(
        (nextText: string) => {
            if (nextText === "") {
                clearDraft(draftKey);
                return;
            }
            setDraft(draftKey, nextText);
        },
        [clearDraft, draftKey, setDraft],
    );

    const handleKeyDown = useCallback(
        (e: KeyboardEvent<HTMLTextAreaElement>) => {
            if (isImeTransitionalEvent(e.nativeEvent)) return;
            if (composingRef.current) return;
            if (e.key === "Enter" && e.ctrlKey && !e.shiftKey && !e.altKey && !e.metaKey) {
                e.preventDefault();
                const trimmed = draftText.trim();
                if (trimmed) onSave(trimmed);
            }
            if (e.key === "Escape") {
                e.preventDefault();
                onCancel();
            }
        },
        [draftText, onSave, onCancel],
    );

    const handleSubmit = useCallback(() => {
        const trimmed = draftText.trim();
        if (trimmed) onSave(trimmed);
    }, [draftText, onSave]);

    return (
        <div className="diff-comment-form">
            {showRangeSelector && (
                <label className="diff-comment-range-field">
                    <span className="diff-comment-range-label">Lines</span>
                    <select
                        className="diff-comment-range-select"
                        value={selectedRangeValue}
                        onChange={(e) => onRangeChange(e.target.value)}
                    >
                        {rangeOptions!.map((option) => (
                            <option key={option.value} value={option.value}>
                                {option.label}
                            </option>
                        ))}
                    </select>
                </label>
            )}
            <textarea
                ref={textareaRef}
                className="diff-comment-textarea"
                value={draftText}
                onChange={(e) => handleChange(e.target.value)}
                onKeyDown={handleKeyDown}
                onCompositionStart={() => { composingRef.current = true; }}
                onCompositionEnd={() => { composingRef.current = false; }}
                placeholder="Review comment... (Ctrl+Enter to save, Escape to cancel)"
            />
            <div className="diff-comment-form-actions">
                <button type="button" className="diff-comment-btn" onClick={onCancel}>Cancel</button>
                <button type="button" className="diff-comment-btn diff-comment-btn--primary" onClick={handleSubmit}
                        disabled={!draftText.trim()}>
                    Add
                </button>
            </div>
        </div>
    );
});
