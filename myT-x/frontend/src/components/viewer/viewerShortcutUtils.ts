const MODIFIER_ORDER = ["ctrl", "shift", "alt", "meta"] as const;

type ShortcutModifier = (typeof MODIFIER_ORDER)[number];
const FUNCTION_KEY_PATTERN = /^f(?:[1-9]|1[0-9]|2[0-4])$/i;

const modifierAliases: Record<string, ShortcutModifier> = {
    ctrl: "ctrl",
    control: "ctrl",
    shift: "shift",
    alt: "alt",
    option: "alt",
    meta: "meta",
    cmd: "meta",
    command: "meta",
};

function normalizeShortcutToken(rawToken: string): string {
    return rawToken.trim().toLowerCase();
}

function normalizeKeyboardKey(rawKey: string): string {
    if (rawKey === " ") {
        return "space";
    }
    return rawKey.toLowerCase();
}

export function isFunctionKeyToken(token: string): boolean {
    return FUNCTION_KEY_PATTERN.test(token);
}

export function hasShortcutModifier(shortcut: string): boolean {
    const normalized = normalizeShortcut(shortcut);
    if (normalized === "") {
        return false;
    }
    const tokens = normalized.split("+");
    if (tokens.length === 1) {
        return isFunctionKeyToken(tokens[0] ?? "");
    }
    // tokens.length >= 2 guaranteed: length=0 returns "" from normalizeShortcut
    // (caught by normalized === "" above), length=1 handled above.
    return tokens.length >= 2;
}

export function normalizeShortcut(rawShortcut: string): string {
    const tokens = rawShortcut
        .split("+")
        .map(normalizeShortcutToken)
        .filter((token) => token !== "");

    if (tokens.length === 0) {
        return "";
    }

    const modifiers = new Set<ShortcutModifier>();
    let key = "";
    for (const token of tokens) {
        const modifier = modifierAliases[token];
        if (modifier) {
            modifiers.add(modifier);
            continue;
        }
        key = token;
    }
    if (key === "") {
        return "";
    }

    const orderedModifiers = MODIFIER_ORDER.filter((modifier) => modifiers.has(modifier));
    return [...orderedModifiers, key].join("+");
}

export function buildShortcutFromKeyboardEvent(event: KeyboardEvent): string {
    const key = normalizeKeyboardKey(event.key);
    if (key === "control" || key === "shift" || key === "alt" || key === "meta") {
        return "";
    }
    if (!event.ctrlKey && !event.shiftKey && !event.altKey && !event.metaKey && !isFunctionKeyToken(key)) {
        return "";
    }

    const parts: string[] = [];
    if (event.ctrlKey) {
        parts.push("ctrl");
    }
    if (event.shiftKey) {
        parts.push("shift");
    }
    if (event.altKey) {
        parts.push("alt");
    }
    if (event.metaKey) {
        parts.push("meta");
    }
    parts.push(key);
    return normalizeShortcut(parts.join("+"));
}

export function getEffectiveViewerShortcut(
    configuredShortcut: string | null | undefined,
    defaultShortcut: string | null | undefined,
): string | null {
    const configured = typeof configuredShortcut === "string" ? configuredShortcut.trim() : "";
    if (configured !== "") {
        const normalizedConfigured = normalizeShortcut(configured);
        if (normalizedConfigured !== "") {
            return normalizedConfigured;
        }
    }
    const fallback = typeof defaultShortcut === "string" ? defaultShortcut.trim() : "";
    if (fallback === "") {
        return null;
    }
    const normalizedFallback = normalizeShortcut(fallback);
    return normalizedFallback !== "" ? normalizedFallback : null;
}
