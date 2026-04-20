import {normalizeShortcut, toAriaKeyshortcuts} from "../../viewerShortcutUtils";
import {mustGetReservedViewerShortcutDef} from "../../viewerReservedShortcuts";

export const FILE_CONTENT_PREVIEW_TOGGLE_SHORTCUT_ID = "file-content-preview-toggle";

const previewToggleShortcut = mustGetReservedViewerShortcutDef(FILE_CONTENT_PREVIEW_TOGGLE_SHORTCUT_ID);

export const FILE_CONTENT_PREVIEW_TOGGLE_SHORTCUT = normalizeShortcut(previewToggleShortcut.shortcut);
export const FILE_CONTENT_PREVIEW_TOGGLE_SHORTCUT_LABEL = previewToggleShortcut.shortcut;
export const FILE_CONTENT_PREVIEW_TOGGLE_SHORTCUT_ARIA = toAriaKeyshortcuts(previewToggleShortcut.shortcut);
