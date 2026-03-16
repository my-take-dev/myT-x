import {type KeyboardEvent, useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../api";
import {useI18n} from "../i18n";
import type {PaneSnapshot} from "../types/tmux";
import {isImeTransitionalEvent} from "../utils/ime";

interface ChatInputBarProps {
    activePaneId: string | null;
    activePaneIndex: number;
    activePaneTitle: string;
    panes: PaneSnapshot[];
    chatOverlayPercentage: number;
}

const MIN_OVERLAY_HEIGHT_PX = 120;

export function ChatInputBar({
                                 activePaneId,
                                 activePaneIndex,
                                 activePaneTitle,
                                 panes,
                                 chatOverlayPercentage,
                             }: ChatInputBarProps) {
    const {t} = useI18n();
    const [text, setText] = useState("");
    const [expanded, setExpanded] = useState(false);
    const [halfHeight, setHalfHeight] = useState(false);
    const [anchorTop, setAnchorTop] = useState(false);
    const [sending, setSending] = useState(false);
    const [sendError, setSendError] = useState<string | null>(null);
    const [heightPx, setHeightPx] = useState<number | null>(null);
    const [fullHeightPx, setFullHeightPx] = useState<number | null>(null);
    const [selectedPaneId, setSelectedPaneId] = useState<string | null>(activePaneId);
    const composingRef = useRef(false);
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const overlayRef = useRef<HTMLDivElement>(null);
    const dragCleanupRef = useRef<(() => void) | null>(null);
    const anchorTopRef = useRef(false);

    // Initialize selectedPaneId once when activePaneId becomes available.
    const initializedRef = useRef(false);
    useEffect(() => {
        if (!initializedRef.current && activePaneId) {
            setSelectedPaneId(activePaneId);
            initializedRef.current = true;
        }
    }, [activePaneId]);

    // Record initial height in px when overlay first renders.
    useEffect(() => {
        if (expanded && overlayRef.current && fullHeightPx == null) {
            const h = overlayRef.current.getBoundingClientRect().height;
            setFullHeightPx(h);
        }
    }, [expanded, fullHeightPx]);

    // Auto-focus textarea when expanding.
    useEffect(() => {
        if (expanded && textareaRef.current) {
            textareaRef.current.focus();
        }
    }, [expanded]);

    // Cleanup drag listeners on unmount.
    useEffect(() => {
        return () => {
            dragCleanupRef.current?.();
        };
    }, []);

    const targetPaneId = selectedPaneId ?? activePaneId;

    // Auto-clear send error after a few seconds.
    useEffect(() => {
        if (sendError == null) return;
        const id = setTimeout(() => setSendError(null), 5000);
        return () => clearTimeout(id);
    }, [sendError]);

    const handleSend = useCallback(async () => {
        const trimmed = text.trim();
        if (!trimmed || !targetPaneId || sending) {
            return;
        }
        setSending(true);
        setSendError(null);
        try {
            await api.SendChatMessage(targetPaneId, trimmed);
            setText("");
            setExpanded(false);
        } catch (err) {
            console.warn("[chat] SendChatMessage failed", err);
            setSendError(String(err));
        } finally {
            setSending(false);
        }
    }, [text, targetPaneId, sending]);

    const handleExpandedKeyDown = useCallback(
        (e: KeyboardEvent<HTMLTextAreaElement>) => {
            if (isImeTransitionalEvent(e.nativeEvent)) return;
            if (composingRef.current) return;
            if (e.key === "Enter" && e.ctrlKey) {
                e.preventDefault();
                void handleSend();
            }
            if (e.key === "Escape") {
                e.preventDefault();
                setExpanded(false);
            }
        },
        [handleSend],
    );

    const handleCompositionStart = useCallback(() => {
        composingRef.current = true;
    }, []);

    const handleCompositionEnd = useCallback(() => {
        composingRef.current = false;
    }, []);

    const handleBarClick = useCallback(
        (e: React.MouseEvent<HTMLDivElement>) => {
            // Avoid expanding when clicking the send button.
            const target = e.target as HTMLElement;
            if (target.tagName === "BUTTON") {
                return;
            }
            setExpanded(true);
        },
        [],
    );

    // Drag-to-resize handler (DockedDivider pattern).
    const startResize = useCallback((handle: HTMLDivElement) => {
        const mainContent = handle.closest(".main-content");
        if (!(mainContent instanceof HTMLElement)) return;

        const initialUserSelect = document.body.style.userSelect;
        document.body.style.userSelect = "none";

        const maxHeight = mainContent.getBoundingClientRect().height * 0.95;

        const onMove = (event: MouseEvent) => {
            const rect = mainContent.getBoundingClientRect();
            const newHeight = anchorTopRef.current
                ? event.clientY - rect.top
                : rect.bottom - event.clientY;
            const clamped = Math.max(MIN_OVERLAY_HEIGHT_PX, Math.min(maxHeight, newHeight));
            setHeightPx(clamped);
        };

        const cleanup = () => {
            window.removeEventListener("mousemove", onMove);
            window.removeEventListener("mouseup", onUp);
            window.removeEventListener("blur", onUp);
            document.body.style.userSelect = initialUserSelect;
        };

        const onUp = () => {
            cleanup();
            if (dragCleanupRef.current === cleanup) {
                dragCleanupRef.current = null;
            }
            // Update fullHeightPx to current height after drag.
            // Read the latest heightPx from overlayRef instead of using setState updater side-effects.
            if (overlayRef.current) {
                const currentHeight = overlayRef.current.getBoundingClientRect().height;
                if (currentHeight > 0) {
                    setFullHeightPx(currentHeight);
                    setHalfHeight(false);
                }
            }
        };

        if (dragCleanupRef.current) {
            dragCleanupRef.current();
        }
        dragCleanupRef.current = cleanup;
        window.addEventListener("mousemove", onMove);
        window.addEventListener("mouseup", onUp);
        window.addEventListener("blur", onUp, {once: true});
    }, []);

    // Toggle half height.
    const toggleHalf = useCallback(() => {
        if (fullHeightPx == null) return;
        if (halfHeight) {
            setHeightPx(fullHeightPx);
        } else {
            setHeightPx(Math.max(MIN_OVERLAY_HEIGHT_PX, Math.round(fullHeightPx / 2)));
        }
        setHalfHeight((prev) => !prev);
    }, [fullHeightPx, halfHeight]);

    // Keep anchorTop ref in sync for use inside resize handler.
    anchorTopRef.current = anchorTop;

    // Toggle anchor position (top / bottom).
    const toggleAnchor = useCallback(() => {
        setAnchorTop((prev) => !prev);
    }, []);

    // Style: px-based if set, otherwise %-based from config.
    const overlayStyle = useMemo(() => {
        const h = heightPx != null ? `${heightPx}px` : `${chatOverlayPercentage}%`;
        if (anchorTop) {
            return {height: h, top: 0, bottom: "auto"} as const;
        }
        return {height: h};
    }, [heightPx, chatOverlayPercentage, anchorTop]);

    if (expanded) {
        return (
            <div
                ref={overlayRef}
                className={`chat-overlay${anchorTop ? " chat-overlay--anchor-top" : ""}`}
                style={overlayStyle}
            >
                <div
                    className="chat-overlay-resize-handle"
                    onMouseDown={(e) => {
                        e.preventDefault();
                        startResize(e.currentTarget);
                    }}
                />
                <div className="chat-overlay-header">
                    <div className="chat-overlay-pane-selector">
                        {panes.map((pane) => (
                            <button
                                key={pane.id}
                                type="button"
                                className={`chat-overlay-pane-icon${pane.id === targetPaneId ? " selected" : ""}`}
                                onClick={() => setSelectedPaneId(pane.id)}
                                title={pane.title || `%${pane.index}`}
                            >
                                %{pane.index}
                                {pane.title ? ` ${pane.title}` : ""}
                            </button>
                        ))}
                    </div>
                    <button
                        type="button"
                        className="chat-overlay-close"
                        onClick={() => setExpanded(false)}
                        title={t("chat.close", "Close")}
                    >
                        &times;
                    </button>
                </div>

                {sendError && (
                    <div className="chat-overlay-error" onClick={() => setSendError(null)}>
                        {sendError}
                    </div>
                )}

                <textarea
                    ref={textareaRef}
                    className="chat-overlay-textarea"
                    value={text}
                    onChange={(e) => setText(e.target.value)}
                    onKeyDown={handleExpandedKeyDown}
                    onCompositionStart={handleCompositionStart}
                    onCompositionEnd={handleCompositionEnd}
                    placeholder={t("chat.placeholder", "Enter message... (Ctrl+Enter to send)")}
                />

                <div className="chat-overlay-footer">
                    <div className="chat-overlay-footer-actions">
                        <button
                            type="button"
                            className={`chat-overlay-half-btn${halfHeight ? " active" : ""}`}
                            onClick={toggleHalf}
                            title={t("chat.halfHeight", "Toggle half height")}
                        >
                            &#189;
                        </button>
                        <button
                            type="button"
                            className={`chat-overlay-anchor-btn${anchorTop ? " active" : ""}`}
                            onClick={toggleAnchor}
                            title={t("chat.anchorTop", "Toggle top anchor")}
                        >
                            {anchorTop ? "\u2193" : "\u2191"}
                        </button>
                    </div>
                    <button
                        type="button"
                        className="chat-overlay-send"
                        onClick={() => void handleSend()}
                        disabled={sending || !text.trim()}
                    >
                        {sending
                            ? t("chat.sending", "Sending...")
                            : t("chat.send", "Send")}
                    </button>
                </div>
            </div>
        );
    }

    return (
        <div className="chat-input-bar" onClick={handleBarClick}>
            <span className="chat-input-bar-pane">
                %{activePaneIndex} {activePaneTitle || ""}
            </span>
            {sendError && (
                <div className="chat-input-bar-error" onClick={(e) => {
                    e.stopPropagation();
                    setSendError(null);
                }}>
                    {sendError}
                </div>
            )}
            <input
                className="chat-input-bar-input"
                type="text"
                value={text}
                readOnly
                placeholder={t("chat.collapsedPlaceholder", "Send message to pane...")}
            />
            <button
                type="button"
                className="chat-input-bar-send"
                onClick={(e) => {
                    e.stopPropagation();
                    void handleSend();
                }}
                disabled={sending || !text.trim()}
            >
                {t("chat.send", "Send")}
            </button>
        </div>
    );
}
