export type LineEnding = "\r\n" | "\n";

export const LINE_ENDING_CRLF: LineEnding = "\r\n";
export const LINE_ENDING_LF: LineEnding = "\n";

/**
 * Split text into lines while preserving trailing empty lines.
 * Example: "" -> [""], "a\n" -> ["a", ""].
 */
export function splitLines(text: string): string[] {
    return text.split(/\r?\n/);
}

/**
 * Detect the line ending style from the first newline in text.
 * Returns LF as default when no newline is found.
 *
 * NOTE: CR-only files (classic Mac OS format, \r without \n) are not
 * distinguished and will default to LF. This is intentional — CR-only
 * line endings are a legacy format that modern tooling does not produce.
 */
export function detectLineEnding(text: string): LineEnding {
    const firstLf = text.indexOf("\n");
    // No newline (or starts with LF) keeps the default LF behavior.
    if (firstLf <= 0) {
        return LINE_ENDING_LF;
    }
    return text[firstLf - 1] === "\r" ? LINE_ENDING_CRLF : LINE_ENDING_LF;
}
