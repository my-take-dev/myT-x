import type {ReactNode} from "react";
import {useEffect, useMemo, useRef, useState} from "react";
import {useI18n} from "../i18n";
import {useChatStore} from "../stores/chatStore";
import type {PaneSnapshot, ValidationRules} from "../types/tmux";
import {ChatDivider} from "./ChatDivider";
import {ChatInputBar} from "./ChatInputBar";
import {PromptPresetSelector} from "./PromptPresetSelector";
import {useChatInput} from "./useChatInput";
import {ANCHOR_ARROWS, ANCHOR_BUTTONS, useChatResize} from "./useChatResize";

interface ChatLayoutProps {
    readonly children: ReactNode;
    readonly activePaneId: string | null;
    readonly activePaneTitle: string;
    readonly panes: PaneSnapshot[];
    readonly chatOverlayPercentage: number;
    readonly validationRules?: ValidationRules | null;
}

export function ChatLayout({
                                children,
                                activePaneId,
                                activePaneTitle,
                                panes,
                                chatOverlayPercentage,
                                validationRules,
                            }: ChatLayoutProps) {
    const {t} = useI18n();
    const [expanded, setExpanded] = useState(false);
    const [autoClose, setAutoClose] = useState(true);
    const expandedRef = useRef(expanded);
    const paneIds = useMemo(() => panes.map((pane) => pane.id), [panes]);
    const input = useChatInput({activePaneId, paneIds, autoClose, expanded, setExpanded});
    const resize = useChatResize({chatOverlayPercentage, validationRules});
    const collapsedTargetPaneId = input.targetPaneId ?? activePaneId;
    const collapsedTargetPaneTitle = useMemo(() => {
        if (collapsedTargetPaneId === null) {
            return activePaneTitle;
        }
        return panes.find((pane) => pane.id === collapsedTargetPaneId)?.title ?? "";
    }, [activePaneTitle, collapsedTargetPaneId, panes]);

    // PaneChatBar からのリクエストを監視し、対象ペインを選択してパネルを展開する。
    const requestedPaneId = useChatStore((s) => s.requestedPaneId);
    const clearRequest = useChatStore((s) => s.clearRequest);
    const {setSelectedPaneId, textareaRef} = input;
    useEffect(() => {
        expandedRef.current = expanded;
    }, [expanded]);
    useEffect(() => {
        if (requestedPaneId === null) return;
        setSelectedPaneId(requestedPaneId);
        if (!expandedRef.current) {
            setExpanded(true);
        } else {
            // パネル展開済みの場合は expanded が変化しないため、
            // 既存の auto-focus useEffect が発火しない。明示的にフォーカスする。
            textareaRef.current?.focus();
        }
        clearRequest();
    }, [requestedPaneId, clearRequest, setSelectedPaneId, textareaRef]);

    // top/left: panel appears before main content in DOM order (CSS flex places it at the correct edge).
    // bottom/right: panel appears after main content.
    const panelBeforeContent = resize.anchor === "top" || resize.anchor === "left";
    const layoutClassName = resize.isHorizontal
        ? "chat-layout chat-layout--horizontal"
        : "chat-layout";
    const panelClassName = [
        "chat-docked-panel",
        `chat-docked-panel--${resize.anchor}`,
        resize.isHorizontal && "chat-docked-panel--horizontal",
    ].filter(Boolean).join(" ");

    const panel = expanded ? (
        <div className={panelClassName} style={resize.panelStyle}>
            <div className="chat-panel-header">
                <div className="chat-panel-pane-selector">
                    {panes.map((pane) => (
                        <button
                            key={pane.id}
                            type="button"
                            className={`chat-panel-pane-icon${pane.id === input.targetPaneId ? " selected" : ""}`}
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
                    className="chat-panel-close"
                    onClick={() => setExpanded(false)}
                    title={t("chat.close", "Close")}
                    aria-label={t("chat.close", "Close")}
                >
                    &times;
                </button>
            </div>

            {input.sendError && (
                <div className="chat-panel-error" onClick={() => input.setSendError(null)}>
                    {input.sendError}
                </div>
            )}

            <textarea
                ref={input.textareaRef}
                className="chat-panel-textarea"
                value={input.text}
                onChange={(event) => input.setText(event.target.value)}
                onKeyDown={input.handleExpandedKeyDown}
                onCompositionStart={input.handleCompositionStart}
                onCompositionEnd={input.handleCompositionEnd}
                placeholder={t("chat.placeholder", "Enter message... (Ctrl+Enter to send)")}
            />

            <div className="chat-panel-footer">
                <div className="chat-panel-footer-actions">
                    <button
                        type="button"
                        className={`chat-panel-half-btn${resize.isHalfSize ? " active" : ""}`}
                        onClick={resize.toggleHalfSize}
                        title={t("chat.halfSize", "Toggle half size")}
                        aria-label={t("chat.halfSize", "Toggle half size")}
                        aria-pressed={resize.isHalfSize}
                    >
                        &#189;
                    </button>
                    <div className="chat-panel-anchor-group">
                        {ANCHOR_BUTTONS.map((pos) => (
                            <button
                                key={pos}
                                type="button"
                                className={`chat-panel-anchor-btn${resize.anchor === pos ? " active" : ""}`}
                                onClick={() => resize.changeAnchor(pos)}
                                title={ANCHOR_ARROWS[pos]}
                                aria-label={t(`chat.anchor.${pos}`, `Dock ${pos}`)}
                                aria-pressed={resize.anchor === pos}
                            >
                                {ANCHOR_ARROWS[pos]}
                            </button>
                        ))}
                    </div>
                    <PromptPresetSelector
                        setText={input.setText}
                        onApplied={() => input.textareaRef.current?.focus()}
                    />
                </div>
                <div className="chat-panel-send-group">
                    <label className="chat-panel-auto-close">
                        <input
                            type="checkbox"
                            checked={autoClose}
                            onChange={(event) => setAutoClose(event.target.checked)}
                        />
                        {t("chat.autoClose", "Auto close")}
                    </label>
                    <button
                        type="button"
                        className="chat-panel-send"
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
    ) : null;

    const divider = expanded ? (
        <ChatDivider
            anchor={resize.anchor}
            onRatioChange={resize.setChatRatio}
            onReset={resize.resetChatRatio}
        />
    ) : null;

    return (
        <>
            <div className={layoutClassName}>
                {panelBeforeContent && panel}
                {panelBeforeContent && divider}
                <div className="chat-layout__content">{children}</div>
                {!panelBeforeContent && divider}
                {!panelBeforeContent && panel}
            </div>
            {!expanded && (
                <ChatInputBar
                    targetPaneId={collapsedTargetPaneId}
                    targetPaneTitle={collapsedTargetPaneTitle}
                    text={input.text}
                    sending={input.sending}
                    sendError={input.sendError}
                    onExpand={() => setExpanded(true)}
                    onSend={() => void input.handleSend()}
                    onClearError={() => input.setSendError(null)}
                />
            )}
        </>
    );
}
