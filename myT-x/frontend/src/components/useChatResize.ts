import {useCallback, useEffect, useMemo, useState} from "react";
import type {ValidationRules} from "../types/tmux";
import {
    clampChatOverlayPercentage,
    fallbackChatOverlayValidationRules,
    getChatOverlayValidationRules,
} from "../utils/chatOverlayValidation";

export type AnchorPosition = "bottom" | "top" | "left" | "right";

export const ANCHOR_BUTTONS: AnchorPosition[] = ["left", "top", "bottom", "right"];
export const ANCHOR_ARROWS: Record<AnchorPosition, string> = {
    bottom: "\u2193",
    left: "\u2190",
    right: "\u2192",
    top: "\u2191",
};

export const CHAT_RATIO_MIN = fallbackChatOverlayValidationRules.minPercentage / 100;
export const CHAT_RATIO_DEFAULT = fallbackChatOverlayValidationRules.defaultPercentage / 100;
export const CHAT_RATIO_MAX = fallbackChatOverlayValidationRules.maxPercentage / 100;

interface UseChatResizeParams {
    readonly chatOverlayPercentage: number;
    readonly validationRules?: ValidationRules | null;
}

function normalizeChatRatio(chatOverlayPercentage: number, validationRules?: ValidationRules | null): number {
    const rules = getChatOverlayValidationRules(validationRules);
    if (!Number.isFinite(chatOverlayPercentage) || chatOverlayPercentage <= 0) {
        return rules.defaultPercentage / 100;
    }
    return clampChatOverlayPercentage(chatOverlayPercentage, validationRules) / 100;
}

export function clampChatRatio(ratio: number, validationRules?: ValidationRules | null): number {
    const rules = getChatOverlayValidationRules(validationRules);
    const minRatio = rules.minPercentage / 100;
    const maxRatio = rules.maxPercentage / 100;
    const defaultRatio = rules.defaultPercentage / 100;
    if (!Number.isFinite(ratio)) {
        return defaultRatio;
    }
    return Math.max(minRatio, Math.min(maxRatio, ratio));
}

export function useChatResize({chatOverlayPercentage, validationRules}: UseChatResizeParams) {
    const defaultRatio = useMemo(
        () => normalizeChatRatio(chatOverlayPercentage, validationRules),
        [chatOverlayPercentage, validationRules],
    );
    const [anchor, setAnchor] = useState<AnchorPosition>("bottom");
    const [chatRatio, setChatRatioState] = useState(defaultRatio);
    const [fullChatRatio, setFullChatRatio] = useState(defaultRatio);
    const [isHalfSize, setIsHalfSize] = useState(false);

    useEffect(() => {
        setChatRatioState(defaultRatio);
        setFullChatRatio(defaultRatio);
        setIsHalfSize(false);
    }, [defaultRatio]);

    const isHorizontal = anchor === "left" || anchor === "right";

    const setChatRatio = useCallback((ratio: number) => {
        const clamped = clampChatRatio(ratio, validationRules);
        setChatRatioState(clamped);
        setFullChatRatio(clamped);
        setIsHalfSize(false);
    }, [validationRules]);

    const resetChatRatio = useCallback(() => {
        setChatRatioState(defaultRatio);
        setFullChatRatio(defaultRatio);
        setIsHalfSize(false);
    }, [defaultRatio]);

    const toggleHalfSize = useCallback(() => {
        if (isHalfSize) {
            setChatRatioState(fullChatRatio);
            setIsHalfSize(false);
            return;
        }

        setFullChatRatio(chatRatio);
        setChatRatioState(clampChatRatio(chatRatio / 2, validationRules));
        setIsHalfSize(true);
    }, [chatRatio, fullChatRatio, isHalfSize, validationRules]);

    const changeAnchor = useCallback((nextAnchor: AnchorPosition) => {
        setAnchor(nextAnchor);
        if (isHalfSize) {
            setChatRatioState(fullChatRatio);
        }
        setIsHalfSize(false);
    }, [fullChatRatio, isHalfSize]);

    const panelStyle = useMemo(
        () => (isHorizontal ? {width: `${chatRatio * 100}%`} : {height: `${chatRatio * 100}%`}),
        [chatRatio, isHorizontal],
    );

    return {
        anchor,
        changeAnchor,
        chatRatio,
        isHalfSize,
        isHorizontal,
        panelStyle,
        resetChatRatio,
        setChatRatio,
        toggleHalfSize,
    };
}
