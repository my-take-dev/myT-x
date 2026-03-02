import {ClipboardSetText} from "../../wailsjs/runtime/runtime";

/** Shared state type for clipboard copy feedback UI ("idle" -> "copied" -> "idle", or "failed" -> "idle"). */
export type CopyNoticeState = "idle" | "copied" | "failed";

/**
 * Write text to the system clipboard.
 * Uses Wails ClipboardSetText as the primary method, with navigator.clipboard.writeText
 * as a fallback for environments where Wails is unavailable (e.g. dev server in browser).
 */
export async function writeClipboardText(text: string): Promise<void> {
    try {
        await ClipboardSetText(text);
        return;
    } catch (wailsErr: unknown) {
        console.warn("[clipboard] Wails ClipboardSetText failed, trying browser API", wailsErr);
        const hasBrowserClipboard =
            typeof navigator !== "undefined"
            && typeof navigator.clipboard?.writeText === "function";
        if (hasBrowserClipboard) {
            try {
                await navigator.clipboard.writeText(text);
                return;
            } catch (browserErr: unknown) {
                throw new Error(
                    `Clipboard write failed: Wails: ${wailsErr instanceof Error ? wailsErr.message : String(wailsErr)}; Browser: ${browserErr instanceof Error ? browserErr.message : String(browserErr)}`,
                    {cause: browserErr}
                );
            }
        }
        throw new Error(`Clipboard write failed (Wails): ${String(wailsErr)}`);
    }
}
