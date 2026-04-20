export type QuickSearchTriggerMode = "palette" | "dropdown";

export const QUICK_SEARCH_DIALOG_ID = "quick-search-dialog";
export const QUICK_SEARCH_SHORTCUT_DISPLAY = "Ctrl+P";

export function isQuickSearchShortcut(
    event: Pick<KeyboardEvent, "altKey" | "ctrlKey" | "key" | "metaKey" | "shiftKey">,
): boolean {
    return event.ctrlKey
        && !event.shiftKey
        && !event.altKey
        && !event.metaKey
        && (event.key === "p" || event.key === "P");
}
