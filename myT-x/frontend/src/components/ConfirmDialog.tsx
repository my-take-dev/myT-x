import {useCallback, useEffect, useId, useRef} from "react";

interface ConfirmAction {
    label: string;
    value: string;
    variant?: "primary" | "danger" | "ghost";
}

interface ConfirmDialogProps {
    open: boolean;
    title: string;
    message: string;
    actions: ConfirmAction[];
    onAction: (value: string) => void;
    onClose: () => void;
}

export function ConfirmDialog({open, title, message, actions, onAction, onClose}: ConfirmDialogProps) {
    // S-31 / S-32: WAI-ARIA dialog pattern — generate stable IDs for
    // aria-labelledby / aria-describedby associations.
    const instanceId = useId();
    const titleId = `${instanceId}-title`;
    const messageId = `${instanceId}-message`;

    // I-10: ダイアログ表示時に最初のアクションボタン（またはキャンセルボタン）へフォーカスを移動し、
    // Tabキーによるダイアログ外へのフォーカス漏れを防ぐ。
    const cancelBtnRef = useRef<HTMLButtonElement | null>(null);
    const panelRef = useRef<HTMLDivElement | null>(null);

    const handleKeyDown = useCallback(
        (e: KeyboardEvent) => {
            if (e.key === "Escape") {
                // S-25: Prevent Escape from propagating to parent handlers (e.g. prefix key mode).
                e.stopPropagation();
                onClose();
                return;
            }
            // I-10: Tab キーをダイアログ内でトラップする。
            // I-25: usePrefixKeyMode 等の上位キーハンドラへの伝播を防止する。
            if (e.key === "Tab" && panelRef.current) {
                e.stopPropagation();
                const focusable = panelRef.current.querySelectorAll<HTMLElement>(
                    'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
                );
                if (focusable.length === 0) return;
                const first = focusable[0];
                const last = focusable[focusable.length - 1];
                if (e.shiftKey) {
                    if (document.activeElement === first) {
                        e.preventDefault();
                        last.focus();
                    }
                } else {
                    if (document.activeElement === last) {
                        e.preventDefault();
                        first.focus();
                    }
                }
            }
        },
        [onClose],
    );

    useEffect(() => {
        if (!open) return;
        document.addEventListener("keydown", handleKeyDown);
        return () => document.removeEventListener("keydown", handleKeyDown);
    }, [open, handleKeyDown]);

    // ダイアログが開いた際にキャンセルボタンへ自動フォーカスする。
    useEffect(() => {
        if (open) {
            cancelBtnRef.current?.focus();
        }
    }, [open]);

    if (!open) return null;

    return (
        // S-31 / S-32: WAI-ARIA dialog pattern
        // role="dialog" + aria-modal="true" signals to assistive technologies that
        // this is a modal dialog and content behind it is inert.
        // aria-labelledby and aria-describedby link the dialog to its title and message.
        //
        // SUG-14: The HTML `inert` attribute on the background content was considered
        // but is unnecessary for this application. This is a Wails-based Windows desktop
        // app where the dialog overlay covers the entire viewport, the focus trap (Tab key
        // handler above) prevents focus escape, and there are no other interactive surfaces
        // behind the dialog that could receive pointer events.
        <div className="modal-overlay" onClick={onClose}>
            <div
                ref={panelRef}
                className="modal-panel"
                role="dialog"
                aria-modal="true"
                aria-labelledby={titleId}
                aria-describedby={messageId}
                onClick={(e) => e.stopPropagation()}
            >
                <div className="modal-header">
                    <h2 id={titleId}>{title}</h2>
                </div>
                <div className="modal-body">
                    <p id={messageId} style={{margin: 0, fontSize: "0.88rem"}}>{message}</p>
                </div>
                <div className="modal-footer">
                    <button ref={cancelBtnRef} type="button" className="modal-btn" onClick={onClose}>
                        キャンセル
                    </button>
                    {actions.map((action) => (
                        <button
                            key={action.value}
                            type="button"
                            className={`modal-btn ${action.variant || ""}`}
                            onClick={() => onAction(action.value)}
                        >
                            {action.label}
                        </button>
                    ))}
                </div>
            </div>
        </div>
    );
}
