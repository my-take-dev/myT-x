import {useI18n} from "../i18n";

interface ChatInputBarProps {
    readonly targetPaneId: string | null;
    readonly targetPaneTitle: string;
    readonly text: string;
    readonly sending: boolean;
    readonly sendError: string | null;
    readonly onExpand: () => void;
    readonly onSend: () => void;
    readonly onClearError: () => void;
}

export function ChatInputBar({
                                 targetPaneId,
                                 targetPaneTitle,
                                 text,
                                 sending,
                                 sendError,
                                 onExpand,
                                 onSend,
                                 onClearError,
                             }: ChatInputBarProps) {
    const {t} = useI18n();
    return (
        <div
            className="chat-input-bar"
            role="button"
            tabIndex={0}
            onClick={onExpand}
            onKeyDown={(event) => {
                if (event.target !== event.currentTarget) {
                    return;
                }
                if (event.key !== "Enter" && event.key !== " ") {
                    return;
                }
                event.preventDefault();
                onExpand();
            }}
            aria-label={t("chat.open", "Open chat input")}
        >
            <span className="chat-input-bar-pane">
                {targetPaneId || "%0"} {targetPaneTitle || ""}
            </span>
            {sendError && (
                <div className="chat-input-bar-error" onClick={(e) => {
                    e.stopPropagation();
                    onClearError();
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
                    onSend();
                }}
                disabled={sending || !text.trim()}
            >
                {sending ? t("chat.sending", "Sending...") : t("chat.send", "Send")}
            </button>
        </div>
    );
}
