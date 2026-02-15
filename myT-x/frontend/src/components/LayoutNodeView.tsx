import { useEffect, useRef, useState } from "react";
import type { LayoutNode, PaneSnapshot } from "../types/tmux";
import { TerminalPane } from "./TerminalPane";

export interface LayoutNodeActions {
  onFocusPane: (paneId: string) => void;
  onSplitVertical: (paneId: string) => void;
  onSplitHorizontal: (paneId: string) => void;
  onToggleZoom: (paneId: string) => void;
  onKillPane: (paneId: string) => void;
  onRenamePane: (paneId: string, title: string) => void;
  onSwapPane: (sourcePaneId: string, targetPaneId: string) => void;
  onDetachSession: () => void;
}

interface LayoutNodeViewProps extends LayoutNodeActions {
  node: LayoutNode;
  paneMap: Map<string, PaneSnapshot>;
  activePaneId: string | null;
  nodePath?: string;
}

function renderPaneTerminal(
  pane: PaneSnapshot,
  active: boolean,
  actions: LayoutNodeActions,
) {
  return (
    <TerminalPane
      paneId={pane.id}
      paneTitle={pane.title}
      active={active}
      onFocus={actions.onFocusPane}
      onSplitVertical={actions.onSplitVertical}
      onSplitHorizontal={actions.onSplitHorizontal}
      onToggleZoom={actions.onToggleZoom}
      onKillPane={actions.onKillPane}
      onRenamePane={actions.onRenamePane}
      onSwapPane={actions.onSwapPane}
      onDetach={actions.onDetachSession}
    />
  );
}

export function LayoutNodeView(props: LayoutNodeViewProps) {
  const node = props.node;
  if (node.type === "leaf") {
    if (typeof node.pane_id !== "number" || !Number.isFinite(node.pane_id)) {
      return <div className="session-empty">Pane id is missing.</div>;
    }
    const paneId = `%${node.pane_id}`;
    const pane = props.paneMap.get(paneId);
    if (!pane) {
      return <div className="session-empty">ペイン {paneId} が見つかりません。</div>;
    }
    return renderPaneTerminal(
      pane,
      props.activePaneId === pane.id || pane.active,
      props,
    );
  }

  return <SplitLayoutNodeView {...props} />;
}

function childNodeKey(node: LayoutNode | undefined, fallbackPath: string): string {
  if (!node) {
    return fallbackPath;
  }
  if (node.type === "leaf") {
    return `leaf:${node.pane_id}:${fallbackPath}`;
  }
  return `split:${fallbackPath}`;
}

function SplitLayoutNodeView(props: LayoutNodeViewProps) {
  const node = props.node;

  // [C-1 fix] Hooks must be called before any conditional return (React Rules of Hooks).
  const [ratio, setRatio] = useState(node.ratio && node.ratio > 0 ? node.ratio : 0.5);
  const dragCleanupRef = useRef<(() => void) | null>(null);
  const nodePath = props.nodePath ?? "root";

  useEffect(() => {
    if (node.ratio && node.ratio > 0) {
      setRatio(node.ratio);
      return;
    }
    setRatio(0.5);
  }, [node.ratio]);

  // [C-2 fix] Cleanup drag event listeners on unmount to prevent listener leak.
  useEffect(
    () => () => {
      if (dragCleanupRef.current) {
        dragCleanupRef.current();
        dragCleanupRef.current = null;
      }
    },
    [],
  );

  if (node.type !== "split") {
    return null;
  }

  const direction = node.direction === "vertical" ? "column" : "row";

  const startDrag = (start: MouseEvent, mode: "horizontal" | "vertical") => {
    const parent = (start.target as HTMLElement)?.parentElement;
    if (!parent) {
      return;
    }
    const rect = parent.getBoundingClientRect();
    const startRatio = ratio;

    const onMove = (event: MouseEvent) => {
      const delta = mode === "horizontal" ? event.clientX - start.clientX : event.clientY - start.clientY;
      const size = mode === "horizontal" ? rect.width : rect.height;
      if (size <= 0) {
        return;
      }
      const next = Math.max(0.1, Math.min(0.9, startRatio + delta / size));
      setRatio(next);
    };
    const cleanup = () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
    const onUp = () => {
      cleanup();
      if (dragCleanupRef.current === cleanup) {
        dragCleanupRef.current = null;
      }
    };
    if (dragCleanupRef.current) {
      dragCleanupRef.current();
    }
    dragCleanupRef.current = cleanup;
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  };

  const childMode = direction === "row" ? "horizontal" : "vertical";

  return (
    <div className="layout-split" style={{ flexDirection: direction }}>
      <div style={{ flex: ratio, minWidth: 0, minHeight: 0 }}>
        {node.children?.[0] ? (
          <LayoutNodeView
            key={childNodeKey(node.children[0], `${nodePath}.0`)}
            node={node.children[0]}
            paneMap={props.paneMap}
            activePaneId={props.activePaneId}
            onFocusPane={props.onFocusPane}
            onSplitVertical={props.onSplitVertical}
            onSplitHorizontal={props.onSplitHorizontal}
            onToggleZoom={props.onToggleZoom}
            onKillPane={props.onKillPane}
            onRenamePane={props.onRenamePane}
            onSwapPane={props.onSwapPane}
            onDetachSession={props.onDetachSession}
            nodePath={`${nodePath}.0`}
          />
        ) : null}
      </div>
      <div
        className={`pane-divider ${direction === "row" ? "vertical" : "horizontal"}`}
        onMouseDown={(event) => {
          event.preventDefault();
          startDrag(event.nativeEvent, childMode);
        }}
        onDoubleClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
          setRatio(0.5);
        }}
      />
      <div style={{ flex: 1 - ratio, minWidth: 0, minHeight: 0 }}>
        {node.children?.[1] ? (
          <LayoutNodeView
            key={childNodeKey(node.children[1], `${nodePath}.1`)}
            node={node.children[1]}
            paneMap={props.paneMap}
            activePaneId={props.activePaneId}
            onFocusPane={props.onFocusPane}
            onSplitVertical={props.onSplitVertical}
            onSplitHorizontal={props.onSplitHorizontal}
            onToggleZoom={props.onToggleZoom}
            onKillPane={props.onKillPane}
            onRenamePane={props.onRenamePane}
            onSwapPane={props.onSwapPane}
            onDetachSession={props.onDetachSession}
            nodePath={`${nodePath}.1`}
          />
        ) : null}
      </div>
    </div>
  );
}
