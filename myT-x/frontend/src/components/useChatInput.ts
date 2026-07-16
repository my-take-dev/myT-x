import type {Dispatch, KeyboardEvent, SetStateAction} from "react";
import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../api";
import {type InputHistoryEntry, useInputHistoryStore} from "../stores/inputHistoryStore";
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

interface ChatHistoryInput {
    readonly seq: number;
    readonly input: string;
}

function recallableHistoryInputs(inputHistoryEntries: readonly InputHistoryEntry[]): ChatHistoryInput[] {
    const historyInputs: ChatHistoryInput[] = [];
    for (const entry of inputHistoryEntries) {
        if (entry.source !== "chat" || entry.input.trim().length === 0) {
            continue;
        }
        const previous = historyInputs[historyInputs.length - 1];
        if (previous?.input === entry.input) {
            historyInputs[historyInputs.length - 1] = {seq: entry.seq, input: entry.input};
            continue;
        }
        historyInputs.push({seq: entry.seq, input: entry.input});
    }
    return historyInputs;
}

function indexForSeq(historyInputs: readonly ChatHistoryInput[], seq: number): number {
    return historyInputs.findIndex((entry) => entry.seq === seq);
}

function insertionIndexForSeq(historyInputs: readonly ChatHistoryInput[], seq: number): number {
    const nextIndex = historyInputs.findIndex((entry) => entry.seq > seq);
    return nextIndex === -1 ? historyInputs.length : nextIndex;
}

function hasHistoryNavigationModifier(event: KeyboardEvent<HTMLTextAreaElement>): boolean {
    return event.altKey || event.ctrlKey || event.metaKey || event.shiftKey;
}

function selectionIsCollapsed(target: HTMLTextAreaElement): boolean {
    return target.selectionStart === target.selectionEnd;
}

function shouldUseArrowForHistory(event: KeyboardEvent<HTMLTextAreaElement>): boolean {
    if (hasHistoryNavigationModifier(event)) {
        return false;
    }
    const target = event.currentTarget;
    if (!selectionIsCollapsed(target)) {
        return false;
    }
    return target.selectionStart === target.value.length;
}

export function useChatInput({activePaneId, paneIds, autoClose, expanded, setExpanded}: UseChatInputParams) {
    const inputHistoryEntries = useInputHistoryStore((s) => s.entries);
    const [text, setTextState] = useState("");
    const [sending, setSending] = useState(false);
    const [sendError, setSendError] = useState<string | null>(null);
    const [selectedPaneId, setSelectedPaneId] = useState<string | null>(null);
    const [historySeq, setHistorySeq] = useState<number | null>(null);
    const hasDirectUserInputRef = useRef(false);
    const composingRef = useRef(false);
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const historyInputs = useMemo(
        () => recallableHistoryInputs(inputHistoryEntries),
        [inputHistoryEntries],
    );
    const hasText = text.length > 0;

    // External text writes exit history recall mode.
    const setText = useCallback<Dispatch<SetStateAction<string>>>((value) => {
        setHistorySeq(null);
        setTextState((previous) => {
            const next = typeof value === "function" ? value(previous) : value;
            if (next.length === 0) {
                hasDirectUserInputRef.current = false;
            }
            return next;
        });
    }, []);

    const setDirectText = useCallback((value: string) => {
        setHistorySeq(null);
        hasDirectUserInputRef.current = value.length > 0;
        setTextState(value);
    }, []);

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
        } catch (err: unknown) {
            console.warn("[chat] SendChatMessage failed", err);
            setSendError(toErrorMessage(err, "Failed to send message."));
            notifyAndLog("Send chat message", "error", err, "ChatInput");
        } finally {
            setSending(false);
        }
    }, [text, targetPaneId, sending, autoClose, setExpanded, setText]);

    const handleExpandedKeyDown = useCallback(
        (e: KeyboardEvent<HTMLTextAreaElement>) => {
            if (isImeTransitionalEvent(e.nativeEvent)) return;
            if (composingRef.current) return;

            switch (e.key) {
                case "Enter":
                    if (e.ctrlKey) {
                        e.preventDefault();
                        void handleSend();
                    }
                    return;
                case "ArrowUp": {
                    if (historyInputs.length === 0 || (historySeq === null && hasText && hasDirectUserInputRef.current)) return;
                    if (!shouldUseArrowForHistory(e)) return;
                    e.preventDefault();
                    const currentIndex = historySeq === null ? -1 : indexForSeq(historyInputs, historySeq);
                    const currentPosition = historySeq === null
                        ? historyInputs.length
                        : currentIndex === -1
                            ? insertionIndexForSeq(historyInputs, historySeq)
                            : currentIndex;
                    const nextIndex = historySeq === null
                        ? historyInputs.length - 1
                        : Math.max(0, currentPosition - 1);
                    const nextEntry = historyInputs[nextIndex];
                    // Keep the recall cursor while replacing only the visible text.
                    setHistorySeq(nextEntry.seq);
                    setTextState(nextEntry.input);
                    return;
                }
                case "ArrowDown": {
                    if (historySeq === null) return;
                    if (hasText && hasDirectUserInputRef.current) return;
                    if (!shouldUseArrowForHistory(e)) return;
                    e.preventDefault();
                    const currentIndex = indexForSeq(historyInputs, historySeq);
                    const nextIndex = currentIndex === -1
                        ? insertionIndexForSeq(historyInputs, historySeq)
                        : currentIndex + 1;
                    if (nextIndex >= historyInputs.length) {
                        setText("");
                        return;
                    }
                    const nextEntry = historyInputs[nextIndex];
                    // Keep the recall cursor while replacing only the visible text.
                    setHistorySeq(nextEntry.seq);
                    setTextState(nextEntry.input);
                    return;
                }
                case "Escape":
                    e.preventDefault();
                    setExpanded(false);
                    return;
                default:
                    return;
            }
        },
        [handleSend, hasText, historySeq, historyInputs, setExpanded, setText],
    );

    const handleCompositionStart = useCallback(() => {
        composingRef.current = true;
    }, []);

    const handleCompositionEnd = useCallback(() => {
        composingRef.current = false;
    }, []);

    return {
        text,
        setText,
        setDirectText,
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
    };
}
