import {useState} from "react";
import {useI18n} from "../i18n";
import type {PaneSnapshot} from "../types/tmux";
import {useChatInput} from "./useChatInput";
import {ANCHOR_ARROWS, ANCHOR_BUTTONS, useChatResize} from "./useChatResize";

interface ChatInputBarProps {
    activePaneId: string | null;
    activePaneTitle: string;
    panes: PaneSnapshot[];
    chatOverlayPercentage: number;
}

export function ChatInputBar({
                                 activePaneId,
                                 activePaneTitle,
                                 panes,
                                 chatOverlayPercentage,
                             }: ChatInputBarProps) {
    const {t} = useI18n();
    const [expanded, setExpanded] = useState(false);
    const [autoClose, setAutoClose] = useState(true);

    const input = useChatInput({activePaneId, autoClose, expanded, setExpanded});
    const resize = useChatResize({expanded, chatOverlayPercentage});

    if (expanded) {
        return (
            <div
                ref={resize.overlayRef}
                className={resize.overlayClassName}
                style={resize.overlayStyle}
            >
                <div
                    className="chat-overlay-resize-handle"
                    onMouseDown={(e) => {
                        e.preventDefault();
                        resize.startResize(e.currentTarget);
                    }}
                />
                <div className="chat-overlay-header">
                    <div className="chat-overlay-pane-selector">
                        {panes.map((pane) => (
                            <button
                                key={pane.id}
                                type="button"
                                className={`chat-overlay-pane-icon${pane.id === input.targetPaneId ? " selected" : ""}`}
                                onClick={() => input.setSelectedPaneId(pane.id)}
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

                {input.sendError && (
                    <div className="chat-overlay-error" onClick={() => input.setSendError(null)}>
                        {input.sendError}
                    </div>
                )}

                <textarea
                    ref={input.textareaRef}
                    className="chat-overlay-textarea"
                    value={input.text}
                    onChange={(e) => input.setText(e.target.value)}
                    onKeyDown={input.handleExpandedKeyDown}
                    onCompositionStart={input.handleCompositionStart}
                    onCompositionEnd={input.handleCompositionEnd}
                    placeholder={t("chat.placeholder", "Enter message... (Ctrl+Enter to send)")}
                />

                <div className="chat-overlay-footer">
                    <div className="chat-overlay-footer-actions">
                        <button
                            type="button"
                            className={`chat-overlay-half-btn${resize.isHalfSize ? " active" : ""}`}
                            onClick={resize.toggleHalfSize}
                            title={t("chat.halfSize", "Toggle half size")}
                        >
                            &#189;
                        </button>
                        <div className="chat-overlay-anchor-group">
                            {ANCHOR_BUTTONS.map((pos) => (
                                <button
                                    key={pos}
                                    type="button"
                                    className={`chat-overlay-anchor-btn${resize.anchor === pos ? " active" : ""}`}
                                    onClick={() => resize.changeAnchor(pos)}
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
                            onClick={() => void input.handleSend()}
                            disabled={input.sending || !input.text.trim()}
                        >
                            {input.sending
                                ? t("chat.sending", "Sending...")
                                : t("chat.send", "Send")}
                        </button>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="chat-input-bar" onClick={input.handleBarClick}>
            <span className="chat-input-bar-pane">
                {activePaneId || "%0"} {activePaneTitle || ""}
            </span>
            {input.sendError && (
                <div className="chat-input-bar-error" onClick={(e) => {
                    e.stopPropagation();
                    input.setSendError(null);
                }}>
                    {input.sendError}
                </div>
            )}
            <input
                className="chat-input-bar-input"
                type="text"
                value={input.text}
                readOnly
                placeholder={t("chat.collapsedPlaceholder", "Send message to pane...")}
            />
            <button
                type="button"
                className="chat-input-bar-send"
                onClick={(e) => {
                    e.stopPropagation();
                    void input.handleSend();
                }}
                disabled={input.sending || !input.text.trim()}
            >
                {t("chat.send", "Send")}
            </button>
        </div>
    );
}
