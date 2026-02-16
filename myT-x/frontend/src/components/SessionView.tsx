import { useCallback, useMemo } from "react";
import { api } from "../api";
import { useTmuxStore } from "../stores/tmuxStore";
import type { PaneSnapshot, SessionSnapshot } from "../types/tmux";
import { LayoutPresetSelector } from "./LayoutPresetSelector";
import { LayoutRenderer } from "./LayoutRenderer";

interface SessionViewProps {
  session: SessionSnapshot | null;
}

export function SessionView(props: SessionViewProps) {
  const zoomPaneId = useTmuxStore((s) => s.zoomPaneId);
  const setZoomPaneId = useTmuxStore((s) => s.setZoomPaneId);
  const syncInputMode = useTmuxStore((s) => s.syncInputMode);
  const toggleSyncInputMode = useTmuxStore((s) => s.toggleSyncInputMode);

  const paneList = useMemo(() => {
    if (!props.session) {
      return [] as PaneSnapshot[];
    }
    const panes: PaneSnapshot[] = [];
    for (const window of props.session.windows) {
      panes.push(...window.panes);
    }
    return panes;
  }, [props.session]);

  const activePaneId = useMemo(() => {
    const active = paneList.find((pane) => pane.active);
    return active?.id ?? paneList[0]?.id ?? null;
  }, [paneList]);
  const sessionName = props.session?.name ?? "";

  const onFocusPane = useCallback((paneId: string) => {
    void api.FocusPane(paneId).catch((err) => {
      console.warn("[session-view] FocusPane failed", err);
    });
  }, []);

  const onSplitVertical = useCallback((paneId: string) => {
    void api.SplitPane(paneId, true).catch((err) => {
      console.warn("[session-view] SplitPane(vertical) failed", err);
    });
  }, []);

  const onSplitHorizontal = useCallback((paneId: string) => {
    void api.SplitPane(paneId, false).catch((err) => {
      console.warn("[session-view] SplitPane(horizontal) failed", err);
    });
  }, []);

  const onToggleZoom = useCallback((paneId: string) => {
    const current = useTmuxStore.getState().zoomPaneId;
    setZoomPaneId(current === paneId ? null : paneId);
  }, [setZoomPaneId]);

  const onKillPane = useCallback((paneId: string) => {
    void api.KillPane(paneId).catch((err) => {
      console.warn("[session-view] KillPane failed", err);
    });
  }, []);

  const onRenamePane = useCallback((paneId: string, title: string) => {
    void api.RenamePane(paneId, title).catch((err) => {
      console.warn("[session-view] RenamePane failed", err);
    });
  }, []);

  const onSwapPane = useCallback((sourcePaneId: string, targetPaneId: string) => {
    void api.SwapPanes(sourcePaneId, targetPaneId).catch((err) => {
      console.warn("[session-view] SwapPanes failed", err);
    });
  }, []);

  const onDetachSession = useCallback(() => {
    if (sessionName === "") {
      return;
    }
    void api.DetachSession(sessionName).catch((err) => {
      console.warn("[session-view] DetachSession failed", err);
    });
  }, [sessionName]);

  if (!props.session) {
    return <div className="session-empty">セッションを作成してください。</div>;
  }
  if (props.session.windows.length === 0) {
    return <div className="session-empty">セッションにウィンドウがありません。</div>;
  }

  const window = props.session.windows[0];

  return (
    <div className="session-view">
      <div className="session-view-header">
        <LayoutPresetSelector
          sessionName={props.session.name}
          paneCount={paneList.length}
        />
        {paneList.length >= 2 && (
          <button
            type="button"
            className={`terminal-toolbar-btn sync-toggle-btn ${syncInputMode ? "sync-active" : ""}`}
            title="同期入力モード (Prefix: s)"
            aria-label="Toggle sync input mode"
            onClick={toggleSyncInputMode}
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.4">
              <path d="M2 5h4l-2-3M12 9H8l2 3" />
              <path d="M2 5c0 3.3 2.7 6 6 6M12 9c0-3.3-2.7-6-6-6" />
            </svg>
            <span className="sync-toggle-label">Sync</span>
          </button>
        )}
      </div>
      <div className="session-view-body">
        <LayoutRenderer
          layout={window.layout ?? null}
          panes={window.panes}
          activePaneId={activePaneId}
          zoomPaneId={zoomPaneId}
          onFocusPane={onFocusPane}
          onSplitVertical={onSplitVertical}
          onSplitHorizontal={onSplitHorizontal}
          onToggleZoom={onToggleZoom}
          onKillPane={onKillPane}
          onRenamePane={onRenamePane}
          onSwapPane={onSwapPane}
          onDetachSession={onDetachSession}
        />
      </div>
    </div>
  );
}
