/**
 * Normalize an unknown caught value into a human-readable error message.
 *
 * @param err - The caught value (Error, string, or anything else).
 * @param fallback - Message to return when `err` does not contain useful text.
 * @returns A non-empty string suitable for UI display.
 */
export function toErrorMessage(err: unknown, fallback: string): string {
    if (err instanceof Error && err.message.trim() !== "") return err.message;
    if (typeof err === "string" && err.trim() !== "") return err;
    return fallback;
}
