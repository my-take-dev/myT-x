import {type KeyboardEvent, useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../api";
import {useI18n} from "../i18n";
import type {PaneSnapshot} from "../types/tmux";
import {isImeTransitionalEvent} from "../utils/ime";

type AnchorPosition = "bottom" | "top" | "left" | "right";

const ANCHOR_BUTTONS: AnchorPosition[] = ["left", "top", "bottom", "right"];
const ANCHOR_ARROWS: Record<AnchorPosition, string> = {
    bottom: "\u2193",
    right: "\u2192",
    top: "\u2191",
    left: "\u2190",
};

interface ChatInputBarProps {
    activePaneId: string | null;
    activePaneTitle: string;
    panes: PaneSnapshot[];
    chatOverlayPercentage: number;
}

const MIN_OVERLAY_HEIGHT_PX = 120;
const MIN_OVERLAY_WIDTH_PX = 200;

export function ChatInputBar({
                                 activePaneId,
                                 activePaneTitle,
                                 panes,
                                 chatOverlayPercentage,
                             }: ChatInputBarProps) {
    const {t} = useI18n();
    const [text, setText] = useState("");
    const [expanded, setExpanded] = useState(false);
    const [halfHeight, setHalfHeight] = useState(false);
    const [autoClose, setAutoClose] = useState(true);
    const [anchor, setAnchor] = useState<AnchorPosition>("bottom");
    const [sending, setSending] = useState(false);
    const [sendError, setSendError] = useState<string | null>(null);
    const [heightPx, setHeightPx] = useState<number | null>(null);
    const [fullHeightPx, setFullHeightPx] = useState<number | null>(null);
    const [widthPx, setWidthPx] = useState<number | null>(null);
    const [fullWidthPx, setFullWidthPx] = useState<number | null>(null);
    const [selectedPaneId, setSelectedPaneId] = useState<string | null>(activePaneId);
    const composingRef = useRef(false);
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const overlayRef = useRef<HTMLDivElement>(null);
    const dragCleanupRef = useRef<(() => void) | null>(null);
    const anchorRef = useRef<AnchorPosition>("bottom");

    // Initialize selectedPaneId once when activePaneId becomes available.
    const initializedRef = useRef(false);
    useEffect(() => {
        if (!initializedRef.current && activePaneId) {
            setSelectedPaneId(activePaneId);
            initializedRef.current = true;
        }
    }, [activePaneId]);

    const isHorizontal = anchor === "left" || anchor === "right";

    // Record initial dimension in px when overlay first renders or anchor mode changes.
    useEffect(() => {
        if (!expanded || !overlayRef.current) return;
        const rect = overlayRef.current.getBoundingClientRect();
        if (isHorizontal) {
            if (fullWidthPx == null && rect.width > 0) setFullWidthPx(rect.width);
        } else {
            if (fullHeightPx == null && rect.height > 0) setFullHeightPx(rect.height);
        }
    }, [expanded, isHorizontal, fullHeightPx, fullWidthPx]);

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
            if (autoClose) setExpanded(false);
        } catch (err) {
            console.warn("[chat] SendChatMessage failed", err);
            setSendError(String(err));
        } finally {
            setSending(false);
        }
    }, [text, targetPaneId, sending, autoClose]);

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

        const currentAnchor = anchorRef.current;
        const isHoriz = currentAnchor === "left" || currentAnchor === "right";
        const containerRect = mainContent.getBoundingClientRect();
        const maxSize = isHoriz
            ? containerRect.width * 0.95
            : containerRect.height * 0.95;
        const minSize = isHoriz ? MIN_OVERLAY_WIDTH_PX : MIN_OVERLAY_HEIGHT_PX;

        const onMove = (event: MouseEvent) => {
            const rect = mainContent.getBoundingClientRect();
            if (isHoriz) {
                const newWidth = currentAnchor === "left"
                    ? event.clientX - rect.left
                    : rect.right - event.clientX;
                setWidthPx(Math.max(minSize, Math.min(maxSize, newWidth)));
            } else {
                const newHeight = currentAnchor === "top"
                    ? event.clientY - rect.top
                    : rect.bottom - event.clientY;
                setHeightPx(Math.max(minSize, Math.min(maxSize, newHeight)));
            }
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
            if (overlayRef.current) {
                const overlayRect = overlayRef.current.getBoundingClientRect();
                if (isHoriz) {
                    if (overlayRect.width > 0) {
                        setFullWidthPx(overlayRect.width);
                        setHalfHeight(false);
                    }
                } else {
                    if (overlayRect.height > 0) {
                        setFullHeightPx(overlayRect.height);
                        setHalfHeight(false);
                    }
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

    // Toggle half size.
    const toggleHalf = useCallback(() => {
        if (isHorizontal) {
            if (fullWidthPx == null) return;
            if (halfHeight) {
                setWidthPx(fullWidthPx);
            } else {
                setWidthPx(Math.max(MIN_OVERLAY_WIDTH_PX, Math.round(fullWidthPx / 2)));
            }
        } else {
            if (fullHeightPx == null) return;
            if (halfHeight) {
                setHeightPx(fullHeightPx);
            } else {
                setHeightPx(Math.max(MIN_OVERLAY_HEIGHT_PX, Math.round(fullHeightPx / 2)));
            }
        }
        setHalfHeight((prev) => !prev);
    }, [isHorizontal, fullWidthPx, fullHeightPx, halfHeight]);

    // Keep anchor ref in sync for use inside resize handler.
    anchorRef.current = anchor;

    // Set anchor position directly.
    const changeAnchor = useCallback((pos: AnchorPosition) => {
        setAnchor(pos);
        setHalfHeight(false);
    }, []);

    // Overlay class name based on anchor position.
    const overlayClassName = useMemo(() => {
        if (anchor === "bottom") return "chat-overlay";
        return `chat-overlay chat-overlay--anchor-${anchor}`;
    }, [anchor]);

    // Style: px-based if set, otherwise %-based from config.
    const overlayStyle = useMemo(() => {
        if (isHorizontal) {
            const w = widthPx != null ? `${widthPx}px` : `${chatOverlayPercentage}%`;
            return {width: w};
        }
        const h = heightPx != null ? `${heightPx}px` : `${chatOverlayPercentage}%`;
        return {height: h};
    }, [isHorizontal, widthPx, heightPx, chatOverlayPercentage]);

    if (expanded) {
        return (
            <div
                ref={overlayRef}
                className={overlayClassName}
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
                                title={pane.title || pane.id}
                            >
                                {pane.id}
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
                        <div className="chat-overlay-anchor-group">
                            {ANCHOR_BUTTONS.map((pos) => (
                                <button
                                    key={pos}
                                    type="button"
                                    className={`chat-overlay-anchor-btn${anchor === pos ? " active" : ""}`}
                                    onClick={() => changeAnchor(pos)}
                                    title={ANCHOR_ARROWS[pos]}
                                >
                                    {ANCHOR_ARROWS[pos]}
                                </button>
                            ))}
                        </div>
                    </div>
                    <div className="chat-overlay-send-group">
                        <label className="chat-overlay-auto-close">
                            <input
                                type="checkbox"
                                checked={autoClose}
                                onChange={(e) => setAutoClose(e.target.checked)}
                            />
                            {t("chat.autoClose", "Auto close")}
                        </label>
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
            </div>
        );
    }

    return (
        <div className="chat-input-bar" onClick={handleBarClick}>
            <span className="chat-input-bar-pane">
                {activePaneId || "%0"} {activePaneTitle || ""}
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
