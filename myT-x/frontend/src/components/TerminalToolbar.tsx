import {memo} from "react";
import type {MouseEvent as ReactMouseEvent} from "react";
import {useI18n} from "../i18n";

interface TerminalToolbarProps {
    readonly paneId: string;
    readonly titleDraft: string;
    readonly titleEditing: boolean;
    readonly renameBusy: boolean;
    readonly autoRunning: boolean;
    readonly onTitleEditStart: () => void;
    readonly onTitleChange: (value: string) => void;
    readonly onTitleCommit: () => void;
    readonly onTitleCancel: () => void;
    readonly onAutoClick: () => void;
    readonly onSplitVertical: () => void;
    readonly onSplitHorizontal: () => void;
    readonly onAddMember: () => void;
    readonly onClose: () => void;
    readonly preventTerminalFocusSteal: (event: ReactMouseEvent<HTMLElement>) => void;
}

export const TerminalToolbar = memo(function TerminalToolbar({
    paneId,
    titleDraft,
    titleEditing,
    renameBusy,
    autoRunning,
    onTitleEditStart,
    onTitleChange,
    onTitleCommit,
    onTitleCancel,
    onAutoClick,
    onSplitVertical,
    onSplitHorizontal,
    onAddMember,
    onClose,
    preventTerminalFocusSteal,
}: TerminalToolbarProps) {
    const {language, t} = useI18n();
    const isEn = language === "en";

    const handleToolbarMouseDown = (event: ReactMouseEvent<HTMLDivElement>): void => {
        const target = event.target;
        if (
            target instanceof HTMLInputElement
            || target instanceof HTMLTextAreaElement
            || (target instanceof HTMLElement && target.isContentEditable)
        ) {
            event.stopPropagation();
            return;
        }
        preventTerminalFocusSteal(event);
    };

    const autoButtonClass = autoRunning
        ? "terminal-toolbar-btn terminal-toolbar-btn-auto-active"
        : "terminal-toolbar-btn";

    return (
        <div
            className="terminal-toolbar"
            draggable={false}
            onMouseDown={handleToolbarMouseDown}
        >
            <div className="terminal-toolbar-pane">
                <span className="terminal-toolbar-id">{paneId}</span>
                <input
                    className="terminal-toolbar-title-input"
                    value={titleDraft}
                    placeholder={
                        isEn
                            ? "Pane name"
                            : t("terminalPane.titleInput.placeholder", "ペイン名")
                    }
                    disabled={renameBusy}
                    onFocus={onTitleEditStart}
                    onChange={(event) => onTitleChange(event.target.value)}
                    onBlur={onTitleCommit}
                    onKeyDown={(event) => {
                        if (event.key === "Enter") {
                            event.preventDefault();
                            (event.currentTarget as HTMLInputElement).blur();
                            return;
                        }
                        if (event.key === "Escape") {
                            event.preventDefault();
                            onTitleCancel();
                            (event.currentTarget as HTMLInputElement).blur();
                        }
                    }}
                />
            </div>
            <div className="terminal-toolbar-actions">
                <button
                    type="button"
                    className={autoButtonClass}
                    draggable={false}
                    title={
                        isEn
                            ? (autoRunning ? "Stop Auto Enter" : "Auto Enter")
                            : t(
                                autoRunning
                                    ? "terminalPane.action.autoStop.title"
                                    : "terminalPane.action.auto.title",
                                autoRunning ? "Auto Enter 停止" : "Auto Enter",
                            )
                    }
                    aria-label={
                        isEn
                            ? (autoRunning
                                ? `Stop auto enter on pane ${paneId}`
                                : `Auto enter on pane ${paneId}`)
                            : t(
                                autoRunning
                                    ? "terminalPane.action.autoStop.aria"
                                    : "terminalPane.action.auto.aria",
                                autoRunning
                                    ? `Stop auto enter on pane ${paneId}`
                                    : `Auto enter on pane ${paneId}`,
                            )
                    }
                    onMouseDown={preventTerminalFocusSteal}
                    onClick={(e) => {
                        e.stopPropagation();
                        onAutoClick();
                    }}
                >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor"
                         strokeWidth="1.5">
                        <path d="M11.5 7A4.5 4.5 0 1 1 7 2.5"/>
                        <polyline points="7,0.5 7,2.5 9,2.5"/>
                    </svg>
                </button>
                <button
                    type="button"
                    className="terminal-toolbar-btn"
                    draggable={false}
                    title={
                        isEn
                            ? "Split Left/Right (Prefix: %)"
                            : t("terminalPane.action.splitVertical.title", "左右分割 (Prefix: %)")
                    }
                    aria-label={
                        isEn
                            ? `Split pane ${paneId} left-right`
                            : t("terminalPane.action.splitVertical.aria", "Split pane {paneId} left-right", {paneId})
                    }
                    onMouseDown={preventTerminalFocusSteal}
                    onClick={(e) => {
                        e.stopPropagation();
                        onSplitVertical();
                    }}
                >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor"
                         strokeWidth="1.5">
                        <rect x="1" y="1" width="12" height="12" rx="1.5"/>
                        <line x1="7" y1="1" x2="7" y2="13"/>
                    </svg>
                </button>
                <button
                    type="button"
                    className="terminal-toolbar-btn"
                    draggable={false}
                    title={
                        isEn
                            ? "Split Top/Bottom (Prefix: quote)"
                            : t("terminalPane.action.splitHorizontal.title", "上下分割 (Prefix: &quot;)")
                    }
                    aria-label={
                        isEn
                            ? `Split pane ${paneId} top-bottom`
                            : t("terminalPane.action.splitHorizontal.aria", "Split pane {paneId} top-bottom", {paneId})
                    }
                    onMouseDown={preventTerminalFocusSteal}
                    onClick={(e) => {
                        e.stopPropagation();
                        onSplitHorizontal();
                    }}
                >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor"
                         strokeWidth="1.5">
                        <rect x="1" y="1" width="12" height="12" rx="1.5"/>
                        <line x1="1" y1="7" x2="13" y2="7"/>
                    </svg>
                </button>
                <button
                    type="button"
                    className="terminal-toolbar-btn"
                    draggable={false}
                    title={
                        isEn
                            ? "Add Member"
                            : t("terminalPane.action.addMember.title", "メンバー追加")
                    }
                    aria-label={
                        isEn
                            ? `Add member to pane ${paneId}`
                            : t("terminalPane.action.addMember.aria", "Add member to pane {paneId}", {paneId})
                    }
                    onMouseDown={preventTerminalFocusSteal}
                    onClick={(e) => {
                        e.stopPropagation();
                        onAddMember();
                    }}
                >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor"
                         strokeWidth="1.5">
                        <circle cx="5.5" cy="4.5" r="2.5"/>
                        <path d="M1 12c0-2.5 2-4 4.5-4s4.5 1.5 4.5 4"/>
                        <line x1="12" y1="5" x2="12" y2="9"/>
                        <line x1="10" y1="7" x2="14" y2="7"/>
                    </svg>
                </button>
                <button
                    type="button"
                    className="terminal-toolbar-btn terminal-toolbar-btn-danger terminal-toolbar-btn-close"
                    draggable={false}
                    title={
                        isEn
                            ? "Close Pane (Prefix: x)"
                            : t("terminalPane.action.close.title", "ペインを閉じる (Prefix: x)")
                    }
                    aria-label={
                        isEn
                            ? `Close pane ${paneId}`
                            : t("terminalPane.action.close.aria", "Close pane {paneId}", {paneId})
                    }
                    onMouseDown={preventTerminalFocusSteal}
                    onClick={(e) => {
                        e.stopPropagation();
                        onClose();
                    }}
                >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor"
                         strokeWidth="1.8">
                        <line x1="2" y1="2" x2="10" y2="10"/>
                        <line x1="10" y1="2" x2="2" y2="10"/>
                    </svg>
                </button>
            </div>
        </div>
    );
});
