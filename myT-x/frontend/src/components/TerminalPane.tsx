import {memo, useCallback, type MouseEvent as ReactMouseEvent, useEffect, useRef, useState} from "react";
import type {FitAddon} from "@xterm/addon-fit";
import type {SearchAddon} from "@xterm/addon-search";
import type {Terminal} from "@xterm/xterm";
import {SearchBar} from "./SearchBar";
import {ConfirmDialog} from "./ConfirmDialog";
import {AutoEnterPopover} from "./AutoEnterPopover";
import {AutoStartPopover} from "./AutoStartPopover";
import {PaneChatBar} from "./PaneChatBar";
import {TerminalToolbar} from "./TerminalToolbar";
import {api} from "../api";
import {useTmuxStore} from "../stores/tmuxStore";
import {useAutoEnterStore, startAutoEnter, stopAutoEnter} from "../stores/autoEnterStore";
import {useCanvasStore} from "../stores/canvasStore";
import {useViewerStore} from "./viewer/viewerStore";
import {notifyAndLog} from "../utils/notifyUtils";
import {useTerminalSetup} from "../hooks/useTerminalSetup";
import {useTerminalEvents} from "../hooks/useTerminalEvents";
import {useTerminalResize} from "../hooks/useTerminalResize";
import {useTerminalFontSize} from "../hooks/useTerminalFontSize";
import {useI18n} from "../i18n";
import type {AppConfigAutoStartCommand} from "../types/tmux";

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

const EMPTY_AUTO_START_ENTRIES: AppConfigAutoStartCommand[] = [];

function TerminalPaneComponent(props: TerminalPaneProps) {
    const {language, t} = useI18n();
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
    const mountedRef = useRef(true);
    const searchAddonRef = useRef<SearchAddon | null>(null);
    const fitAddonRef = useRef<FitAddon | null>(null);
    const skipTitleCommitRef = useRef(false);
    const autoStartCommandInFlightRef = useRef(false);
    const [titleDraft, setTitleDraft] = useState("");
    const [titleEditing, setTitleEditing] = useState(false);
    const [renameBusy, setRenameBusy] = useState(false);
    const [pendingPaneCloseConfirm, setPendingPaneCloseConfirm] = useState(false);
    const [searchOpen, setSearchOpen] = useState(false);
    const [scrollAtBottom, setScrollAtBottom] = useState(true);
    const [autoPopoverOpen, setAutoPopoverOpen] = useState(false);
    const [autoStartPopoverOpen, setAutoStartPopoverOpen] = useState(false);
    const [autoStartCommandInFlight, setAutoStartCommandInFlight] = useState(false);

    const paneTitle = (props.paneTitle || "").trim();
    const activeSession = useTmuxStore((s) => s.activeSession);
    const autoStartEntries = useTmuxStore((s) => s.config?.auto_start ?? EMPTY_AUTO_START_ENTRIES);
    const hasAutoStartEntries = autoStartEntries.some((entry) => entry.command.trim());
    const canvasMode = useCanvasStore((s) => s.mode);
    const rootPaneId = useCanvasStore((s) => s.rootPaneId);
    const setRootPaneId = useCanvasStore((s) => s.setRootPaneId);
    const isRootPane = rootPaneId === props.paneId;
    const isRootToggleVisible = canvasMode === "canvas";

    // Auto Enter state from store (re-renders only when this pane's state changes).
    const autoRunning = useAutoEnterStore(
        (s) => s.activeEntries[props.paneId] !== undefined,
    );
    // Close popover when auto-enter starts running from another source.
    useEffect(() => {
        if (autoRunning) {
            setAutoPopoverOpen(false);
        }
    }, [autoRunning]);

    // syncInputMode をrefで追跡（term.onDataクロージャから参照するため）
    const syncInputMode = useTmuxStore((s) => s.syncInputMode);
    useEffect(() => {
        syncInputModeRef.current = syncInputMode;
    }, [syncInputMode]);

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
        };
    }, []);

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
            await props.onRenamePane(props.paneId, next);
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
    // Each async-scheduling hook owns its own cleanup guard. useTerminalSetup
    // keeps a local disposed flag, while event/data/key handlers share
    // TerminalEventShared.disposed. React calls cleanup in reverse hook-call
    // order, so useTerminalFontSize and useTerminalResize cleanups run before
    // useTerminalSetup disposes the terminal instance.
    // -----------------------------------------------------------------------

    // --- Terminal セットアップ（インスタンス生成・addon・WebGL・リプレイ） ---
    useTerminalSetup({
        paneId: props.paneId,
        focusOnOpen: props.active,
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

    const handleAutoClick = useCallback(() => {
        if (autoRunning) {
            void stopAutoEnter(props.paneId).catch((err: unknown) => {
                console.warn("[DEBUG-auto-enter] stop failed", err);
                notifyAndLog("Auto Enter stop", "warn", err, "TerminalPane");
            });
        } else {
            setAutoPopoverOpen(true);
        }
    }, [autoRunning, props.paneId]);

    const handleAutoStart = useCallback((intervalSeconds: number) => {
        setAutoPopoverOpen(false);
        void startAutoEnter(props.paneId, intervalSeconds).catch((err: unknown) => {
            console.warn("[DEBUG-auto-enter] start failed", err);
            notifyAndLog("Auto Enter start", "warn", err, "TerminalPane");
        });
        terminalRef.current?.focus();
    }, [props.paneId]);

    const handleAutoPopoverClose = useCallback(() => {
        setAutoPopoverOpen(false);
        terminalRef.current?.focus();
    }, []);

    const handleAutoStartPopoverClose = useCallback(() => {
        setAutoStartPopoverOpen(false);
        terminalRef.current?.focus();
    }, []);

    const handleAutoStartCommand = useCallback((entry: typeof autoStartEntries[number]) => {
        if (autoStartCommandInFlightRef.current) {
            return;
        }
        autoStartCommandInFlightRef.current = true;
        setAutoStartCommandInFlight(true);
        const capturedActiveSession = activeSession;
        void api.StartAutoStartCommand(props.paneId, entry)
            .then(() => {
                if (!mountedRef.current || useTmuxStore.getState().activeSession !== capturedActiveSession) {
                    return;
                }
                setAutoStartPopoverOpen(false);
                terminalRef.current?.focus();
            })
            .catch((err: unknown) => {
                console.warn("[DEBUG-auto-start] start failed", err);
                notifyAndLog("AutoStart", "warn", err, "TerminalPane");
                if (!mountedRef.current || useTmuxStore.getState().activeSession !== capturedActiveSession) {
                    return;
                }
                terminalRef.current?.focus();
            })
            .finally(() => {
                autoStartCommandInFlightRef.current = false;
                if (mountedRef.current) {
                    setAutoStartCommandInFlight(false);
                }
            });
    }, [activeSession, props.paneId]);

    const handleAddMember = useCallback(() => {
        useViewerStore.getState().openViewWithContext("orchestrator-teams", {
            kind: "orchestrator-teams-add-term-member",
            addTermMemberPaneId: props.paneId,
        });
    }, [props.paneId]);

    const handleRootToggle = useCallback(() => {
        setRootPaneId(isRootPane ? null : props.paneId);
    }, [isRootPane, props.paneId, setRootPaneId]);

    const focusPaneAndTerminal = useCallback(() => {
        props.onFocus(props.paneId);
        terminalRef.current?.focus();
    }, [props.onFocus, props.paneId]);
    const focusTerminalOnly = useCallback(() => {
        terminalRef.current?.focus();
    }, []);

    return (
        <div
            className={`terminal-pane ${props.active ? "active" : ""}`}
            data-terminal-pane-id={props.paneId}
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
                    void Promise.resolve(props.onSwapPane(sourcePaneId, props.paneId)).catch((err: unknown) => {
                        console.warn("[pane] swap failed", err);
                        notifyAndLog("Swap panes", "warn", err, "TerminalPane");
                    });
                }
            }}
            onMouseDown={focusTerminalOnly}
            onClick={focusPaneAndTerminal}
        >
            <TerminalToolbar
                paneId={props.paneId}
                titleDraft={titleDraft}
                renameBusy={renameBusy}
                autoRunning={autoRunning}
                isRootToggleVisible={isRootToggleVisible}
                isRootPane={isRootPane}
                onTitleEditStart={() => setTitleEditing(true)}
                onTitleChange={setTitleDraft}
                onTitleCommit={() => {
                    if (skipTitleCommitRef.current) {
                        skipTitleCommitRef.current = false;
                        return;
                    }
                    // I-22: fire-and-forget async needs .catch() per defensive-coding #95
                    void commitPaneTitle().catch((err: unknown) => {
                        console.warn("[DEBUG-pane] commitPaneTitle failed", err);
                        notifyAndLog("Rename pane", "warn", err, "TerminalPane");
                    });
                }}
                onTitleCancel={() => {
                    skipTitleCommitRef.current = true;
                    setTitleDraft(paneTitle);
                    setTitleEditing(false);
                }}
                onAutoClick={handleAutoClick}
                onAutoStartClick={() => setAutoStartPopoverOpen(true)}
                autoStartDisabled={!hasAutoStartEntries || autoStartCommandInFlight}
                onRootToggle={handleRootToggle}
                onSplitVertical={() => {
                    focusPaneAndTerminal();
                    props.onSplitVertical(props.paneId);
                }}
                onSplitHorizontal={() => {
                    focusPaneAndTerminal();
                    props.onSplitHorizontal(props.paneId);
                }}
                onAddMember={handleAddMember}
                onClose={() => setPendingPaneCloseConfirm(true)}
                preventTerminalFocusSteal={preventTerminalFocusSteal}
            />
            <div className="terminal-pane-body">
                <SearchBar
                    open={searchOpen}
                    onClose={() => {
                        setSearchOpen(false);
                        terminalRef.current?.focus();
                    }}
                    searchAddon={searchAddonRef.current}
                />
                {autoPopoverOpen && !autoRunning && (
                    <AutoEnterPopover
                        onStart={handleAutoStart}
                        onClose={handleAutoPopoverClose}
                        preventTerminalFocusSteal={preventTerminalFocusSteal}
                    />
                )}
                {autoStartPopoverOpen && hasAutoStartEntries && (
                    <AutoStartPopover
                        entries={autoStartEntries}
                        onStart={handleAutoStartCommand}
                        onClose={handleAutoStartPopoverClose}
                        startDisabled={autoStartCommandInFlight}
                        preventTerminalFocusSteal={preventTerminalFocusSteal}
                    />
                )}
                <div ref={containerRef} className="terminal-mount"/>
                {!scrollAtBottom && (
                    <button
                        type="button"
                        className="scroll-to-bottom-btn"
                        title={
                            language === "en"
                                ? "Scroll to bottom"
                                : t("terminalPane.action.scrollToBottom.title", "最下部にスクロール")
                        }
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
            <PaneChatBar
                paneId={props.paneId}
                preventTerminalFocusSteal={preventTerminalFocusSteal}
            />
            <ConfirmDialog
                open={pendingPaneCloseConfirm}
                title={
                    language === "en"
                        ? "Close pane"
                        : t("terminalPane.confirm.closePane.title", "Close pane")
                }
                message={
                    language === "en"
                        ? `Close pane ${props.paneId}?`
                        : t("terminalPane.confirm.closePane.message", "Close pane {paneId}?", {paneId: props.paneId})
                }
                actions={[
                    {
                        label: language === "en" ? "Close" : t("common.action.close", "Close"),
                        value: "close",
                        variant: "danger",
                    },
                ]}
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
 *
 * autoRunning は props ではなく Zustand store からの内部状態のため
 * ここでの比較は不要。store 値の変化は React が直接検知する。
 */
function areTerminalPanePropsEqual(prev: TerminalPaneProps, next: TerminalPaneProps): boolean {
    return (
        prev.paneId === next.paneId
        && prev.active === next.active
        && prev.paneTitle === next.paneTitle
    );
}

export const TerminalPane = memo(TerminalPaneComponent, areTerminalPanePropsEqual);
