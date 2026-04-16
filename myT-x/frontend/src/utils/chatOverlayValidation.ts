import type {ValidationRules} from "../types/tmux";

export interface ChatOverlayValidationRules {
    readonly minPercentage: number;
    readonly maxPercentage: number;
    readonly defaultPercentage: number;
}

export const fallbackChatOverlayValidationRules: ChatOverlayValidationRules = {
    minPercentage: 15,
    maxPercentage: 70,
    defaultPercentage: 40,
};

function normalizeRuleValue(value: number | undefined, fallback: number): number {
    if (typeof value !== "number" || !Number.isFinite(value)) {
        return fallback;
    }
    return Math.max(1, Math.trunc(value));
}

export function getChatOverlayValidationRules(rules?: ValidationRules | null): ChatOverlayValidationRules {
    const minPercentage = normalizeRuleValue(
        rules?.min_chat_overlay_percentage,
        fallbackChatOverlayValidationRules.minPercentage,
    );
    const maxPercentage = Math.max(
        minPercentage,
        normalizeRuleValue(rules?.max_chat_overlay_percentage, fallbackChatOverlayValidationRules.maxPercentage),
    );
    const defaultPercentage = Math.min(
        maxPercentage,
        Math.max(
            minPercentage,
            normalizeRuleValue(
                rules?.default_chat_overlay_percentage,
                fallbackChatOverlayValidationRules.defaultPercentage,
            ),
        ),
    );
    return {
        minPercentage,
        maxPercentage,
        defaultPercentage,
    };
}

export function clampChatOverlayPercentage(value: number, rules?: ValidationRules | null): number {
    const {minPercentage, maxPercentage, defaultPercentage} = getChatOverlayValidationRules(rules);
    if (!Number.isFinite(value)) {
        return defaultPercentage;
    }
    return Math.max(minPercentage, Math.min(maxPercentage, Math.trunc(value)));
}
