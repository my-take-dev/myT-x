import {normalizeShortcut} from "./viewerShortcutUtils";

export interface ReservedViewerShortcutDef {
    readonly id: string;
    readonly labelJa: string;
    readonly labelEn: string;
    readonly shortcut: string;
}

export const RESERVED_VIEWER_SHORTCUTS = [
    {
        id: "command-palette",
        labelJa: "コマンドパレット",
        labelEn: "Command Palette",
        shortcut: "Ctrl+P",
    },
    {
        id: "file-content-preview-toggle",
        labelJa: "ファイルビューのプレビュー切替",
        labelEn: "File View preview toggle",
        shortcut: "Ctrl+Shift+V",
    },
] as const satisfies readonly ReservedViewerShortcutDef[];

const reservedViewerShortcutsByID = new Map<string, ReservedViewerShortcutDef>(
    RESERVED_VIEWER_SHORTCUTS.map((shortcut) => [shortcut.id, shortcut]),
);

const reservedViewerShortcutsByNormalized = new Map<string, ReservedViewerShortcutDef>(
    RESERVED_VIEWER_SHORTCUTS.map((shortcut) => [normalizeShortcut(shortcut.shortcut), shortcut]),
);

export function mustGetReservedViewerShortcutDef(id: string): ReservedViewerShortcutDef {
    const shortcut = reservedViewerShortcutsByID.get(id);
    if (!shortcut) {
        throw new Error(`[viewer-shortcut] missing reserved shortcut definition: ${id}`);
    }
    return shortcut;
}

export function getReservedViewerShortcutDef(shortcut: string): ReservedViewerShortcutDef | undefined {
    const normalizedShortcut = normalizeShortcut(shortcut);
    if (normalizedShortcut === "") {
        return undefined;
    }
    return reservedViewerShortcutsByNormalized.get(normalizedShortcut);
}

export function getReservedViewerShortcutLabel(
    shortcut: ReservedViewerShortcutDef,
    language: "ja" | "en",
): string {
    return language === "en" ? shortcut.labelEn : shortcut.labelJa;
}
