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
  const actions = useMemo<LayoutNodeActions>(
    () => ({
      onFocusPane: props.onFocusPane,
      onSplitVertical: props.onSplitVertical,
      onSplitHorizontal: props.onSplitHorizontal,
      onToggleZoom: props.onToggleZoom,
      onKillPane: props.onKillPane,
      onRenamePane: props.onRenamePane,
      onSwapPane: props.onSwapPane,
      onDetachSession: props.onDetachSession,
    }),
    [
      props.onDetachSession,
      props.onFocusPane,
      props.onKillPane,
      props.onRenamePane,
      props.onSplitHorizontal,
      props.onSplitVertical,
      props.onSwapPane,
      props.onToggleZoom,
    ],
  );

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
          onFocus={actions.onFocusPane}
          onSplitVertical={actions.onSplitVertical}
          onSplitHorizontal={actions.onSplitHorizontal}
          onToggleZoom={actions.onToggleZoom}
          onKillPane={actions.onKillPane}
          onRenamePane={actions.onRenamePane}
          onSwapPane={actions.onSwapPane}
          onDetach={actions.onDetachSession}
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
      actions={actions}
    />
  );
}
