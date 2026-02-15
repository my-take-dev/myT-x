import { useMemo } from "react";
import type { LayoutNode, PaneSnapshot } from "../types/tmux";
import { LayoutNodeView, type LayoutNodeActions } from "./LayoutNodeView";
import { TerminalPane } from "./TerminalPane";

interface LayoutRendererProps extends LayoutNodeActions {
  layout: LayoutNode | null;
  panes: PaneSnapshot[];
  activePaneId: string | null;
  zoomPaneId: string | null;
}

export function LayoutRenderer(props: LayoutRendererProps) {
  const paneMap = useMemo(() => {
    const map = new Map<string, PaneSnapshot>();
    for (const pane of props.panes) {
      map.set(pane.id, pane);
    }
    return map;
  }, [props.panes]);

  if (props.zoomPaneId) {
    const pane = paneMap.get(props.zoomPaneId);
    if (!pane) {
      return <div className="session-empty">ズーム対象のペインがありません。</div>;
    }
    return (
      <div className="zoom-root">
        <TerminalPane
          paneId={pane.id}
          paneTitle={pane.title}
          active={true}
          onFocus={props.onFocusPane}
          onSplitVertical={props.onSplitVertical}
          onSplitHorizontal={props.onSplitHorizontal}
          onToggleZoom={props.onToggleZoom}
          onKillPane={props.onKillPane}
          onRenamePane={props.onRenamePane}
          onSwapPane={props.onSwapPane}
          onDetach={props.onDetachSession}
        />
      </div>
    );
  }

  if (!props.layout) {
    return <div className="session-empty">レイアウトがありません。</div>;
  }

  return (
    <LayoutNodeView
      node={props.layout}
      paneMap={paneMap}
      activePaneId={props.activePaneId}
      onFocusPane={props.onFocusPane}
      onSplitVertical={props.onSplitVertical}
      onSplitHorizontal={props.onSplitHorizontal}
      onToggleZoom={props.onToggleZoom}
      onKillPane={props.onKillPane}
      onRenamePane={props.onRenamePane}
      onSwapPane={props.onSwapPane}
      onDetachSession={props.onDetachSession}
    />
  );
}
