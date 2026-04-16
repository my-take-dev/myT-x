import type {MouseEvent as ReactMouseEvent} from "react";
import {useI18n} from "../i18n";
import {useChatStore} from "../stores/chatStore";

interface PaneChatBarProps {
    readonly paneId: string;
    readonly preventTerminalFocusSteal: (event: ReactMouseEvent<HTMLElement>) => void;
}

export function PaneChatBar({paneId, preventTerminalFocusSteal}: PaneChatBarProps) {
    const {t} = useI18n();
    const requestOpen = useChatStore((s) => s.requestOpen);

    return (
        <div
            className="pane-chat-bar"
            role="button"
            tabIndex={0}
            onMouseDown={preventTerminalFocusSteal}
            onClick={(event) => {
                event.stopPropagation();
                requestOpen(paneId);
            }}
            onKeyDown={(event) => {
                if (event.target !== event.currentTarget) return;
                if (event.key !== "Enter" && event.key !== " ") return;
                event.preventDefault();
                event.stopPropagation();
                requestOpen(paneId);
            }}
            aria-label={t("paneChatBar.open", "Send message to this pane")}
        >
            <span className="pane-chat-bar-pane">{paneId}</span>
            <span className="pane-chat-bar-placeholder">
                {t("paneChatBar.placeholder", "Send message to pane...")}
            </span>
        </div>
    );
}
