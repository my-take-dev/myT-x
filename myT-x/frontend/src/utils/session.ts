import type {PaneSnapshot, SessionSnapshot, WindowSnapshot} from "../types/tmux";

export function resolveActiveWindow(session: SessionSnapshot | null | undefined): WindowSnapshot | null {
    if (!session || session.windows.length === 0) {
        return null;
    }
    return session.windows.find((win) => win.id === session.active_window_id)
        ?? session.windows.find((win) => win.panes.some((pane) => pane.active))
        ?? session.windows[0]
        ?? null;
}

export function resolveActivePane(activeWindow: WindowSnapshot | null | undefined): PaneSnapshot | null {
    if (!activeWindow || activeWindow.panes.length === 0) {
        return null;
    }
    return activeWindow.panes.find((pane) => pane.active) ?? activeWindow.panes[0] ?? null;
}

export function resolveActivePaneID(session: SessionSnapshot | null | undefined): string | null {
    const activeWindow = resolveActiveWindow(session);
    const activePane = resolveActivePane(activeWindow);
    return activePane?.id ?? null;
}
