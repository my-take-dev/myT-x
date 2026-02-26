import {memo, type MouseEvent as ReactMouseEvent, useEffect, useRef, useState} from "react";
import type {FitAddon} from "@xterm/addon-fit";
import type {SearchAddon} from "@xterm/addon-search";
import type {Terminal} from "@xterm/xterm";
import {SearchBar} from "./SearchBar";
import {ConfirmDialog} from "./ConfirmDialog";
import {useTmuxStore} from "../stores/tmuxStore";
import {useTerminalSetup} from "../hooks/useTerminalSetup";
import {useTerminalEvents} from "../hooks/useTerminalEvents";
import {useTerminalResize} from "../hooks/useTerminalResize";
import {useTerminalFontSize} from "../hooks/useTerminalFontSize";

interface TerminalPaneProps {
    paneId: string;
    paneTitle?: string;
    active: boolean;
    onFocus: (paneId: string) => void;
    onSplitVertical: (paneId: string) => void;
    onSplitHorizontal: (paneId: string) => void;
    onToggleZoom: (paneId: string) => void;
    onKillPane: (paneId: string) => void;
    onRenamePane: (paneId: string, title: string) => void | Promise<void>;
    onSwapPane: (sourcePaneId: string, targetPaneId: string) => void | Promise<void>;
    onDetach: () => void;
}

function TerminalPaneComponent(props: TerminalPaneProps) {
    const syncInputModeRef = useRef(false);
    const isComposingRef = useRef(false);
    const fontSize = useTmuxStore((s) => s.fontSize);
    // fontSizeRef tracks the latest committed font size synchronously so that
    // the wheel handler — which runs inside a long-lived closure capturing only
    // the initial fontSize value — always uses an up-to-date base when
    // pendingFontSize is null.
    const fontSizeRef = useRef(fontSize);
    const containerRef = useRef<HTMLDivElement | null>(null);
    const terminalRef = useRef<Terminal | null>(null);
    const searchAddonRef = useRef<SearchAddon | null>(null);
    const fitAddonRef = useRef<FitAddon | null>(null);
    const skipTitleCommitRef = useRef(false);
    const [titleDraft, setTitleDraft] = useState("");
    const [titleEditing, setTitleEditing] = useState(false);
    const [renameBusy, setRenameBusy] = useState(false);
    const [pendingPaneCloseConfirm, setPendingPaneCloseConfirm] = useState(false);
    const [searchOpen, setSearchOpen] = useState(false);
    const [scrollAtBottom, setScrollAtBottom] = useState(true);

    const paneTitle = (props.paneTitle || "").trim();

    // syncInputMode をrefで追跡（term.onDataクロージャから参照するため）
    const syncInputMode = useTmuxStore((s) => s.syncInputMode);
    useEffect(() => {
        syncInputModeRef.current = syncInputMode;
    }, [syncInputMode]);

    useEffect(() => {
        if (titleEditing) {
            return;
        }
        setTitleDraft(paneTitle);
    }, [paneTitle, titleEditing]);

    const commitPaneTitle = async (): Promise<void> => {
        if (renameBusy) {
            return;
        }
        const current = paneTitle;
        const next = titleDraft.trim();
        setTitleEditing(false);
        if (next === current) {
            return;
        }
        setRenameBusy(true);
        try {
            await Promise.resolve(props.onRenamePane(props.paneId, next));
        } catch {
            setTitleDraft(current);
        } finally {
            setRenameBusy(false);
        }
    };

    // -----------------------------------------------------------------------
    // INVARIANT: Hook call order matters.
    //
    // 1. useTerminalSetup MUST be called first — it creates the Terminal
    //    instance and writes it to terminalRef. All subsequent hooks read
    //    terminalRef.current and will no-op if it is null.
    //
    // 2. useTerminalEvents MUST be called before useTerminalResize because
    //    useTerminalResize reads isComposingRef which is updated by the
    //    composition event listeners registered in useTerminalEvents.
    //
    // 3. useTerminalResize and useTerminalFontSize are independent of each
    //    other but both depend on (1) and (2).
    //
    // Each hook uses a local `disposed` flag set in its cleanup function.
    // React calls cleanup in reverse hook-call order, so useTerminalFontSize
    // and useTerminalResize cleanups run before useTerminalSetup's cleanup
    // calls term.dispose(). The `disposed` flag guards against writes to an
    // already-disposed terminal in async callbacks (RAF, setTimeout, Promise)
    // that may fire between cleanup and disposal.
    // -----------------------------------------------------------------------

    // --- Terminal セットアップ（インスタンス生成・addon・WebGL・リプレイ） ---
    useTerminalSetup({
        paneId: props.paneId,
        containerRef,
        terminalRef,
        searchAddonRef,
        fitAddonRef,
    });

    // --- イベント登録（pane:data・onData・IME・右クリック・キーハンドラ・スクロール） ---
    useTerminalEvents({
        paneId: props.paneId,
        terminalRef,
        syncInputModeRef,
        setSearchOpen,
        setScrollAtBottom,
        isComposingRef,
    });

    // --- リサイズ（ResizeObserver による DOM サイズ変動検知） ---
    useTerminalResize({
        paneId: props.paneId,
        containerRef,
        terminalRef,
        fitAddonRef,
        isComposingRef,
    });

    // --- フォントサイズ変更（Ctrl+ホイール・ストア値反映・ResizePane 通知） ---
    useTerminalFontSize({
        paneId: props.paneId,
        containerRef,
        terminalRef,
        fitAddonRef,
        fontSizeRef,
    });

    const preventTerminalFocusSteal = (event: ReactMouseEvent<HTMLElement>): void => {
        event.preventDefault();
        event.stopPropagation();
    };

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

    return (
        <div
            className={`terminal-pane ${props.active ? "active" : ""}`}
            draggable
            onDragStart={(event) => {
                event.dataTransfer.setData("text/plain", props.paneId);
            }}
            onDragOver={(event) => {
                event.preventDefault();
            }}
            onDrop={(event) => {
                event.preventDefault();
                // ファイルドロップは Wails OnFileDrop (useFileDrop hook) で処理
                if (event.dataTransfer.files.length > 0) return;
                const sourcePaneId = event.dataTransfer.getData("text/plain");
                if (sourcePaneId && sourcePaneId !== props.paneId) {
                    void Promise.resolve(props.onSwapPane(sourcePaneId, props.paneId)).catch((err) => {
                        console.warn("[pane] swap failed", err);
                    });
                }
            }}
            onClick={() => props.onFocus(props.paneId)}
            onMouseDown={() => terminalRef.current?.focus()}
        >
            <div
                className="terminal-toolbar"
                draggable={false}
                onMouseDown={handleToolbarMouseDown}
            >
                <div className="terminal-toolbar-pane">
                    <span className="terminal-toolbar-id">{props.paneId}</span>
                    <input
                        className="terminal-toolbar-title-input"
                        value={titleDraft}
                        placeholder="ペイン名"
                        disabled={renameBusy}
                        onFocus={() => setTitleEditing(true)}
                        onChange={(event) => setTitleDraft(event.target.value)}
                        onBlur={() => {
                            if (skipTitleCommitRef.current) {
                                skipTitleCommitRef.current = false;
                                return;
                            }
                            // I-22: fire-and-forget async needs .catch() per defensive-coding #95
                            void commitPaneTitle().catch((err) => {
                                console.warn("[DEBUG-pane] commitPaneTitle failed", err);
                            });
                        }}
                        onKeyDown={(event) => {
                            if (event.key === "Enter") {
                                event.preventDefault();
                                (event.currentTarget as HTMLInputElement).blur();
                                return;
                            }
                            if (event.key === "Escape") {
                                event.preventDefault();
                                skipTitleCommitRef.current = true;
                                setTitleDraft(paneTitle);
                                setTitleEditing(false);
                                (event.currentTarget as HTMLInputElement).blur();
                            }
                        }}
                    />
                </div>
                <div className="terminal-toolbar-actions">
                    <button
                        type="button"
                        className="terminal-toolbar-btn"
                        draggable={false}
                        title="左右分割 (Prefix: %)"
                        aria-label={`Split pane ${props.paneId} left-right`}
                        onMouseDown={preventTerminalFocusSteal}
                        onClick={(event) => {
                            event.stopPropagation();
                            props.onFocus(props.paneId);
                            props.onSplitVertical(props.paneId);
                            terminalRef.current?.focus();
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
                        title="上下分割 (Prefix: &quot;)"
                        aria-label={`Split pane ${props.paneId} top-bottom`}
                        onMouseDown={preventTerminalFocusSteal}
                        onClick={(event) => {
                            event.stopPropagation();
                            props.onFocus(props.paneId);
                            props.onSplitHorizontal(props.paneId);
                            terminalRef.current?.focus();
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
                        className="terminal-toolbar-btn terminal-toolbar-btn-danger terminal-toolbar-btn-close"
                        draggable={false}
                        title="ペインを閉じる (Prefix: x)"
                        aria-label={`Close pane ${props.paneId}`}
                        onMouseDown={preventTerminalFocusSteal}
                        onClick={(event) => {
                            event.stopPropagation();
                            setPendingPaneCloseConfirm(true);
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
            <div className="terminal-pane-body">
                <SearchBar
                    open={searchOpen}
                    onClose={() => {
                        setSearchOpen(false);
                        terminalRef.current?.focus();
                    }}
                    searchAddon={searchAddonRef.current}
                />
                <div ref={containerRef} className="terminal-mount"/>
                {!scrollAtBottom && (
                    <button
                        type="button"
                        className="scroll-to-bottom-btn"
                        title="最下部にスクロール"
                        onMouseDown={preventTerminalFocusSteal}
                        onClick={() => {
                            terminalRef.current?.scrollToBottom();
                            terminalRef.current?.focus();
                        }}
                    >
                        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor"
                             strokeWidth="1.8">
                            <polyline points="2,4 6,8 10,4"/>
                        </svg>
                    </button>
                )}
            </div>
            <ConfirmDialog
                open={pendingPaneCloseConfirm}
                title="Close pane"
                message={`Close pane ${props.paneId}?`}
                actions={[{label: "Close", value: "close", variant: "danger"}]}
                onAction={(value) => {
                    setPendingPaneCloseConfirm(false);
                    if (value !== "close") {
                        return;
                    }
                    props.onKillPane(props.paneId);
                }}
                onClose={() => setPendingPaneCloseConfirm(false)}
            />
        </div>
    );
}

/**
 * カスタム比較関数: paneId / active / paneTitle のみを比較対象とする。
 *
 * 前提: onFocus / onSplitVertical / onSplitHorizontal / onToggleZoom /
 *       onKillPane / onRenamePane / onSwapPane / onDetach は、
 *       親コンポーネントが useCallback で安定参照を維持していること。
 * これらの関数 props を比較から除外しているため、親が useCallback を
 * 使わない場合は不要な再レンダリングが抑制されなくなる。
 */
function areTerminalPanePropsEqual(prev: TerminalPaneProps, next: TerminalPaneProps): boolean {
    return (
        prev.paneId === next.paneId
        && prev.active === next.active
        && prev.paneTitle === next.paneTitle
    );
}

export const TerminalPane = memo(TerminalPaneComponent, areTerminalPanePropsEqual);
