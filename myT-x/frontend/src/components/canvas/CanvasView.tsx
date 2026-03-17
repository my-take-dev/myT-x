import {useCallback, useEffect, useMemo, useState} from "react";
import {
    applyEdgeChanges,
    applyNodeChanges,
    Background,
    BackgroundVariant,
    Controls,
    MiniMap,
    ReactFlow,
    useReactFlow,
    type Edge,
    type EdgeChange,
    type Node,
    type NodeChange,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import type {PaneSnapshot} from "../../types/tmux";
import {useCanvasStore} from "../../stores/canvasStore";
import {useCanvasTaskSync} from "../../hooks/useCanvasTaskSync";
import {computeTreeLayout} from "../../utils/canvasLayout";
import {aggregateTaskEdges} from "../../utils/aggregateTaskEdges";
import {determineEdgeDirection} from "../../utils/edgeRouting";
import {CanvasTerminalNode} from "./CanvasTerminalNode";
import {TaskEdge} from "./TaskEdge";
import type {TaskEdgeData} from "./TaskEdge";
import {TaskTimelinePanel} from "./TaskTimelinePanel";
import {useI18n} from "../../i18n";
import "../../styles/canvas.css";

const nodeTypes = {terminal: CanvasTerminalNode} as const;
const edgeTypes = {task: TaskEdge} as const;

interface CanvasViewProps {
    panes: PaneSnapshot[];
    activePaneId: string | null;
    sessionName: string;
    onFocusPane: (paneId: string) => void;
    onSplitVertical: (paneId: string) => void;
    onSplitHorizontal: (paneId: string) => void;
    onToggleZoom: (paneId: string) => void;
    onKillPane: (paneId: string) => void;
    onRenamePane: (paneId: string, title: string) => void | Promise<void>;
    onSwapPane: (sourcePaneId: string, targetPaneId: string) => void | Promise<void>;
    onDetachSession: () => void;
}

export function CanvasView(props: CanvasViewProps) {
    const {language, t} = useI18n();
    const {fitView} = useReactFlow();
    const setNodePosition = useCanvasStore((s) => s.setNodePosition);
    const taskEdgeMap = useCanvasStore((s) => s.taskEdgeMap);
    const [timelinePanelOpen, setTimelinePanelOpen] = useState(false);

    // 3秒ポーリング
    useCanvasTaskSync(props.sessionName);

    // ペインからReact Flowノードを生成
    // nodePositions は命令的に読み取り（getState）、依存配列に含めない。
    // ドラッグ終了時の setNodePosition → nodePositions 変更 → nodes 再計算 → setRfNodes
    // → ReactFlow再計測 → 無限ループを防止する。
    const nodes: Node[] = useMemo(() => {
        const currentPositions = useCanvasStore.getState().nodePositions;
        const currentSizes = useCanvasStore.getState().nodeSizes;
        return props.panes.map((pane, index) => {
            const pos = currentPositions[pane.id] ?? {
                x: 100 + index * 40,
                y: 100 + index * 40,
            };
            const size = currentSizes[pane.id] ?? {width: 450, height: 350};
            return {
                id: `pane-${pane.id}`,
                type: "terminal" as const,
                position: pos,
                dragHandle: ".terminal-toolbar",
                data: {
                    paneId: pane.id,
                    paneTitle: pane.title ?? "",
                    active: pane.id === props.activePaneId,
                    onFocus: props.onFocusPane,
                    onSplitVertical: props.onSplitVertical,
                    onSplitHorizontal: props.onSplitHorizontal,
                    onToggleZoom: props.onToggleZoom,
                    onKillPane: props.onKillPane,
                    onRenamePane: props.onRenamePane,
                    onSwapPane: props.onSwapPane,
                    onDetach: props.onDetachSession,
                },
                style: {width: size.width, height: size.height},
            };
        });
    }, [props.panes, props.activePaneId,
        props.onFocusPane, props.onSplitVertical, props.onSplitHorizontal,
        props.onToggleZoom, props.onKillPane, props.onRenamePane,
        props.onSwapPane, props.onDetachSession]);

    // タスクをペアごとに集約し、1ペア=1エッジで React Flow エッジを生成
    // Y座標に基づいて source/target を決定（ツリーの上→下フロー）
    const edges: Edge[] = useMemo(() => {
        const paneIdSet = new Set(props.panes.map((p) => p.id));
        const aggregated = aggregateTaskEdges(Object.values(taskEdgeMap), paneIdSet);
        const currentPositions = useCanvasStore.getState().nodePositions;

        return aggregated.map((agg) => {
            const {sourcePane, targetPane} = determineEdgeDirection(
                agg.paneA, agg.paneB, currentPositions,
            );
            // 方向反転時に表示名を合わせる
            const flipped = sourcePane !== agg.paneA;

            return {
                id: `pair-${agg.paneA}-${agg.paneB}`,
                source: `pane-${sourcePane}`,
                target: `pane-${targetPane}`,
                sourceHandle: "output",
                targetHandle: "input",
                type: "task" as const,
                zIndex: 1001,
                data: {
                    aggregateStatus: agg.aggregateStatus,
                    totalCount: agg.totalCount,
                    pendingCount: agg.pendingCount,
                    completedCount: agg.completedCount,
                    failedCount: agg.failedCount,
                    abandonedCount: agg.abandonedCount,
                    forwardTasks: flipped ? agg.reverseTasks : agg.forwardTasks,
                    reverseTasks: flipped ? agg.forwardTasks : agg.reverseTasks,
                    sourceName: flipped ? agg.targetName : agg.sourceName,
                    targetName: flipped ? agg.sourceName : agg.targetName,
                } satisfies TaskEdgeData,
            };
        });
    }, [taskEdgeMap, props.panes]);

    const [rfNodes, setRfNodes] = useState<Node[]>(nodes);
    const [rfEdges, setRfEdges] = useState<Edge[]>(edges);

    // ペイン追加/削除/アクティブ変更時にRFノードを同期。
    // 既存ノードのReactFlow内部メタデータ（measured等）を保持しつつ data/style のみ更新。
    useEffect(() => {
        setRfNodes((prev) => {
            const prevMap = new Map(prev.map((n) => [n.id, n]));
            return nodes.map((node) => {
                const existing = prevMap.get(node.id);
                if (existing) {
                    // data + style を更新。style は nodeSizes から算出済み。
                    return {...existing, data: node.data, style: node.style};
                }
                return node;
            });
        });
    }, [nodes]);

    // エッジ同期: 内部可変状態なし → 直接置換
    useEffect(() => {
        setRfEdges(edges);
    }, [edges]);

    const handleNodesChange = useCallback(
        (changes: NodeChange[]) => {
            setRfNodes((nds) => applyNodeChanges(changes, nds));
            // ドラッグ完了時に位置を保存
            for (const change of changes) {
                if (change.type === "position" && change.dragging === false && change.position) {
                    const paneId = change.id.replace("pane-", "");
                    setNodePosition(paneId, change.position);
                }
            }
        },
        [setNodePosition],
    );

    const handleEdgesChange = useCallback(
        (changes: EdgeChange[]) => {
            setRfEdges((eds) => applyEdgeChanges(changes, eds));
        },
        [],
    );

    const handleAutoLayout = useCallback(() => {
        const paneIds = props.panes.map((p) => p.id);
        const tasks = Object.values(taskEdgeMap);
        const currentSizes = useCanvasStore.getState().nodeSizes;
        const positions = computeTreeLayout(paneIds, tasks, currentSizes);
        for (const [paneId, pos] of Object.entries(positions)) {
            setNodePosition(paneId, pos);
        }
        // ノード位置を即座に反映
        setRfNodes((prev) =>
            prev.map((node) => {
                const paneId = node.id.replace("pane-", "");
                const newPos = positions[paneId];
                if (newPos) {
                    return {...node, position: newPos};
                }
                return node;
            }),
        );
        // レイアウト完了後、ビューポートをアニメーション付きでフィット
        requestAnimationFrame(() => {
            fitView({padding: 0.15, duration: 400});
        });
    }, [props.panes, taskEdgeMap, setNodePosition, setRfNodes, fitView]);

    const getNodeColor = useCallback((node: Node) => {
        const data = node.data as { active?: boolean } | undefined;
        if (data?.active) return "var(--accent, #d4a843)";
        return "var(--accent-secondary, #58a6ff)";
    }, []);

    return (
        <div className="canvas-view">
            <div className="canvas-toolbar">
                <button
                    type="button"
                    className="terminal-toolbar-btn canvas-auto-layout-btn"
                    title={
                        language === "en"
                            ? "Auto tree layout"
                            : t("canvas.autoLayout", "自動ツリーレイアウト")
                    }
                    onClick={handleAutoLayout}
                >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.4">
                        <rect x="5" y="0.5" width="4" height="3" rx="0.5"/>
                        <rect x="0.5" y="10.5" width="4" height="3" rx="0.5"/>
                        <rect x="9.5" y="10.5" width="4" height="3" rx="0.5"/>
                        <path d="M7 3.5V7M7 7L2.5 10.5M7 7L11.5 10.5"/>
                    </svg>
                    <span>
                        {language === "en" ? "Tree" : t("canvas.tree", "Tree")}
                    </span>
                </button>
                <button
                    type="button"
                    className={`terminal-toolbar-btn canvas-timeline-toggle-btn ${timelinePanelOpen ? "canvas-active" : ""}`}
                    title="タスクタイムライン"
                    onClick={() => setTimelinePanelOpen((p) => !p)}
                >
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.4">
                        <rect x="1" y="1" width="12" height="3" rx="0.5"/>
                        <rect x="1" y="5.5" width="8" height="3" rx="0.5"/>
                        <rect x="1" y="10" width="10" height="3" rx="0.5"/>
                    </svg>
                    <span>Timeline</span>
                </button>
            </div>
            <div className="canvas-content">
                <ReactFlow
                    nodes={rfNodes}
                    edges={rfEdges}
                    onNodesChange={handleNodesChange}
                    onEdgesChange={handleEdgesChange}
                    nodeTypes={nodeTypes}
                    edgeTypes={edgeTypes}
                    elevateEdgesOnSelect
                    fitView
                    minZoom={0.1}
                    maxZoom={2}
                    snapToGrid
                    snapGrid={[20, 20]}
                    proOptions={{hideAttribution: true}}
                >
                    <Background variant={BackgroundVariant.Dots} gap={20} size={1}/>
                    <Controls/>
                    <MiniMap
                        nodeColor={getNodeColor}
                        maskColor="rgba(0, 0, 0, 0.7)"
                    />
                </ReactFlow>
                {timelinePanelOpen && (
                    <TaskTimelinePanel
                        sessionName={props.sessionName}
                        onClose={() => setTimelinePanelOpen(false)}
                    />
                )}
            </div>
        </div>
    );
}
