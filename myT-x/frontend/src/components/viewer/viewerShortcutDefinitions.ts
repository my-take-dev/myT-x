export interface ViewerShortcutDef {
    viewId: string;
    label: string;
    defaultShortcut: string;
}

// Single source of truth for viewer shortcut defaults and labels.
// Keep this list in sync with viewer registrations under views/*/index.ts.
export const VIEWER_SHORTCUTS: readonly ViewerShortcutDef[] = [
    {viewId: "file-tree", label: "File Tree", defaultShortcut: "Ctrl+Shift+E"},
    {viewId: "git-graph", label: "Git Graph", defaultShortcut: "Ctrl+Shift+G"},
    {viewId: "error-log", label: "Error Log", defaultShortcut: "Ctrl+Shift+L"},
    {viewId: "diff", label: "Diff", defaultShortcut: "Ctrl+Shift+D"},
    {viewId: "input-history", label: "Input History", defaultShortcut: "Ctrl+Shift+H"},
    {viewId: "mcp-manager", label: "MCP Manager", defaultShortcut: "Ctrl+Shift+M"},
];

const viewerShortcutByID = new Map<string, ViewerShortcutDef>(
    VIEWER_SHORTCUTS.map((definition) => [definition.viewId, definition]),
);

export function getViewerShortcutDef(viewID: string): ViewerShortcutDef | undefined {
    return viewerShortcutByID.get(viewID);
}

export function mustGetViewerShortcutDef(viewID: string): ViewerShortcutDef {
    const definition = getViewerShortcutDef(viewID);
    if (!definition) {
        throw new Error(`[viewer-shortcut] missing definition for view: ${viewID}`);
    }
    return definition;
}
