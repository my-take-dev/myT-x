import {getEffectiveViewerShortcut, normalizeShortcut} from "./viewerShortcutUtils";

export interface ViewerShortcutDef {
    viewId: string;
    label: string;
    defaultShortcut: string;
}

// Single source of truth for viewer shortcut defaults and labels.
// Keep this list in sync with viewer registrations under views/*/index.ts.
export const VIEWER_SHORTCUTS: readonly ViewerShortcutDef[] = [
    {viewId: "file-view", label: "File View", defaultShortcut: "Ctrl+Shift+E"},
    {viewId: "git-graph", label: "Git Graph", defaultShortcut: "Ctrl+Shift+G"},
    {viewId: "error-log", label: "Error Log", defaultShortcut: "Ctrl+Shift+L"},
    {viewId: "diff", label: "Diff", defaultShortcut: "Ctrl+Shift+D"},
    {viewId: "input-history", label: "Input History", defaultShortcut: "Ctrl+Shift+H"},
    {viewId: "mcp-manager", label: "MCP Manager", defaultShortcut: "Ctrl+Shift+M"},
    {viewId: "pane-scheduler", label: "Schedule", defaultShortcut: "Ctrl+Shift+K"},
    {viewId: "prompt-presets", label: "Prompt Presets", defaultShortcut: "Ctrl+Shift+P"},
    {viewId: "single-task-runner", label: "Single Task Runner", defaultShortcut: "Ctrl+Shift+J"},
    {viewId: "task-scheduler", label: "Task Scheduler", defaultShortcut: "Ctrl+Shift+Q"},
    {viewId: "editor", label: "Editor", defaultShortcut: "Ctrl+Shift+O"},
    {viewId: "orchestrator-teams", label: "Teams", defaultShortcut: "Ctrl+Shift+T"},
    {viewId: "usage-dashboard", label: "Usage Dashboard", defaultShortcut: "Ctrl+Shift+U"},
];

const viewerShortcutByID = new Map<string, ViewerShortcutDef>(
    VIEWER_SHORTCUTS.map((definition) => [definition.viewId, definition]),
);

const VIEWER_SHORTCUT_ALIASES: Readonly<Record<string, readonly string[]>> = {
    "file-view": ["file-tree"],
};

export function getViewerShortcutValue(
    shortcuts: Readonly<Record<string, string>> | null | undefined,
    viewID: string,
): string | undefined {
    if (!shortcuts) {
        return undefined;
    }

    const directValue = shortcuts[viewID];
    if (directValue?.trim()) {
        return directValue;
    }

    for (const legacyViewID of VIEWER_SHORTCUT_ALIASES[viewID] ?? []) {
        const legacyValue = shortcuts[legacyViewID];
        if (legacyValue?.trim()) {
            return legacyValue;
        }
    }

    return directValue;
}

export function normalizeViewerShortcutConfig(
    shortcuts: Readonly<Record<string, string>> | null | undefined,
): Record<string, string> {
    if (!shortcuts) {
        return {};
    }

    const normalizedShortcuts: Record<string, string> = {...shortcuts};
    for (const {viewId} of VIEWER_SHORTCUTS) {
        const aliases = VIEWER_SHORTCUT_ALIASES[viewId] ?? [];
        const directValue = normalizedShortcuts[viewId];
        if (directValue?.trim()) {
            for (const alias of aliases) {
                delete normalizedShortcuts[alias];
            }
            continue;
        }

        for (const alias of aliases) {
            const aliasValue = normalizedShortcuts[alias];
            if (aliasValue?.trim()) {
                normalizedShortcuts[viewId] = aliasValue;
                break;
            }
        }

        for (const alias of aliases) {
            delete normalizedShortcuts[alias];
        }
    }

    return normalizedShortcuts;
}

export function getViewerShortcutDef(viewID: string): ViewerShortcutDef | undefined {
    return viewerShortcutByID.get(viewID);
}

export function findViewerViewForShortcut(
    shortcuts: Readonly<Record<string, string>> | null | undefined,
    rawShortcut: string,
): string | null {
    const normalizedShortcut = normalizeShortcut(rawShortcut);
    if (normalizedShortcut === "") {
        return null;
    }

    for (const {viewId, defaultShortcut} of VIEWER_SHORTCUTS) {
        const effectiveShortcut = getEffectiveViewerShortcut(
            getViewerShortcutValue(shortcuts, viewId),
            defaultShortcut,
        );
        if (effectiveShortcut === normalizedShortcut) {
            return viewId;
        }
    }

    return null;
}

export function mustGetViewerShortcutDef(viewID: string): ViewerShortcutDef {
    const definition = getViewerShortcutDef(viewID);
    if (!definition) {
        throw new Error(`[viewer-shortcut] missing definition for view: ${viewID}`);
    }
    return definition;
}
