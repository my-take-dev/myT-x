import type {KeyboardEvent, MouseEvent} from "react";
import {useCallback, useEffect, useRef, useState} from "react";
import {api} from "../api";
import {toErrorMessage} from "../utils/errorUtils";
import {isImeTransitionalEvent} from "../utils/ime";
import {notifyAndLog} from "../utils/notifyUtils";

interface UseChatInputParams {
    readonly activePaneId: string | null;
    readonly paneIds: readonly string[];
    readonly autoClose: boolean;
    readonly expanded: boolean;
    readonly setExpanded: (expanded: boolean) => void;
}

export function useChatInput({activePaneId, paneIds, autoClose, expanded, setExpanded}: UseChatInputParams) {
    const [text, setText] = useState("");
    const [sending, setSending] = useState(false);
    const [sendError, setSendError] = useState<string | null>(null);
    const [selectedPaneId, setSelectedPaneId] = useState<string | null>(null);
    const composingRef = useRef(false);
    const textareaRef = useRef<HTMLTextAreaElement>(null);

    useEffect(() => {
        if (selectedPaneId !== null && paneIds.length > 0 && !paneIds.includes(selectedPaneId)) {
            setSelectedPaneId(null);
        }
    }, [paneIds, selectedPaneId]);

    const selectedPaneIsAvailable = selectedPaneId !== null && (paneIds.length === 0 || paneIds.includes(selectedPaneId));
    const targetPaneId = selectedPaneIsAvailable ? selectedPaneId : activePaneId;

    // Auto-clear send error after a few seconds.
    useEffect(() => {
        if (sendError == null) return;
        const id = setTimeout(() => setSendError(null), 5000);
        return () => clearTimeout(id);
    }, [sendError]);

    // Auto-focus textarea when expanding.
    useEffect(() => {
        if (expanded && textareaRef.current) {
            textareaRef.current.focus();
        }
    }, [expanded]);

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
            setSendError(toErrorMessage(err, "Failed to send message."));
            notifyAndLog("Send chat message", "error", err, "ChatInput");
        } finally {
            setSending(false);
        }
    }, [text, targetPaneId, sending, autoClose, setExpanded]);

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
        [handleSend, setExpanded],
    );

    const handleCompositionStart = useCallback(() => {
        composingRef.current = true;
    }, []);

    const handleCompositionEnd = useCallback(() => {
        composingRef.current = false;
    }, []);

    const handleBarClick = useCallback(
        (e: MouseEvent<HTMLDivElement>) => {
            // Avoid expanding when clicking the send button.
            const target = e.target as HTMLElement;
            if (target.tagName === "BUTTON") {
                return;
            }
            setExpanded(true);
        },
        [setExpanded],
    );

    return {
        text,
        setText,
        sending,
        sendError,
        setSendError,
        selectedPaneId,
        setSelectedPaneId,
        targetPaneId,
        textareaRef,
        handleSend,
        handleExpandedKeyDown,
        handleCompositionStart,
        handleCompositionEnd,
        handleBarClick,
    };
}
