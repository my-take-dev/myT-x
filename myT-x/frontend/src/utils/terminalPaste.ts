export interface TerminalPasteTarget {
    paste(text: string): void;
}

/**
 * Strips a single trailing newline from pasted text.
 *
 * Design decision: When the clipboard contains only a newline character
 * (e.g. "\n", "\r\n"), the normalized result is an empty string and
 * pasteTextSafely returns false. This is intentional — a bare newline
 * paste is ambiguous (accidental copy of an empty line) and the user
 * can always press Enter explicitly to send a carriage return.
 */
export function normalizeTerminalPasteText(text: string): string {
    return text.replace(/\r\n$|\n$|\r$/, "");
}

export function pasteTextSafely(target: TerminalPasteTarget, text: string): boolean {
    const normalized = normalizeTerminalPasteText(text);
    if (normalized.length === 0) {
        return false;
    }
    try {
        target.paste(normalized);
    } catch (err) {
        console.warn("[terminal] paste failed", err);
        return false;
    }
    return true;
}
