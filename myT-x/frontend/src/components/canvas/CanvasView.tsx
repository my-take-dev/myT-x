import {useCallback, useEffect, useMemo} from "react";
import {
    Background,
    BackgroundVariant,
    Controls,
    MiniMap,
    ReactFlow,
    type Edge,
    type Node,
    type NodeChange,
    useNodesState,
    useEdgesState,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import type {PaneSnapshot} from "../../types/tmux";
import {useCanvasStore} from "../../stores/canvasStore";
import {useCanvasTaskSync} from "../../hooks/useCanvasTaskSync";
import {computeTreeLayout} from "../../utils/canvasLayout";
import {CanvasTerminalNode} from "./CanvasTerminalNode";
import {TaskEdge} from "./TaskEdge";
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
    const nodePositions = useCanvasStore((s) => s.nodePositions);
    const setNodePosition = useCanvasStore((s) => s.setNodePosition);
    const taskEdgeMap = useCanvasStore((s) => s.taskEdgeMap);

    // 3秒ポーリング
    useCanvasTaskSync(props.sessionName);

    // ペインからReact Flowノードを生成
    const nodes: Node[] = useMemo(() => {
        return props.panes.map((pane, index) => {
            const pos = nodePositions[pane.id] ?? {
                x: 100 + index * 40,
                y: 100 + index * 40,
            };
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
                style: {width: 450, height: 350},
            };
        });
    }, [props.panes, props.activePaneId, nodePositions,
        props.onFocusPane, props.onSplitVertical, props.onSplitHorizontal,
        props.onToggleZoom, props.onKillPane, props.onRenamePane,
        props.onSwapPane, props.onDetachSession]);

    // タスクからReact Flowエッジを生成
    const edges: Edge[] = useMemo(() => {
        const paneIdSet = new Set(props.panes.map((p) => p.id));
        return Object.values(taskEdgeMap)
            .filter((task) =>
                task.sender_pane_id && task.assignee_pane_id
                && paneIdSet.has(task.sender_pane_id)
                && paneIdSet.has(task.assignee_pane_id),
            )
            .map((task) => ({
                id: `task-${task.task_id}`,
                source: `pane-${task.sender_pane_id}`,
                target: `pane-${task.assignee_pane_id}`,
                sourceHandle: "output",
                targetHandle: "input",
                type: "task" as const,
                data: {
                    senderName: task.sender_name,
                    agentName: task.agent_name,
                    status: task.status,
                    sentAt: task.sent_at,
                },
            }));
    }, [taskEdgeMap, props.panes]);

    const [rfNodes, setRfNodes, onNodesChange] = useNodesState(nodes);
    const [rfEdges, setRfEdges, onEdgesChange] = useEdgesState(edges);

    // ノード/エッジが変化した場合に同期
    useEffect(() => setRfNodes(nodes), [nodes, setRfNodes]);
    useEffect(() => setRfEdges(edges), [edges, setRfEdges]);

    const handleNodesChange = useCallback(
        (changes: NodeChange[]) => {
            onNodesChange(changes);
            // ドラッグ完了時に位置を保存
            for (const change of changes) {
                if (change.type === "position" && change.dragging === false && change.position) {
                    const paneId = change.id.replace("pane-", "");
                    setNodePosition(paneId, change.position);
                }
            }
        },
        [onNodesChange, setNodePosition],
    );

    const handleAutoLayout = useCallback(() => {
        const paneIds = props.panes.map((p) => p.id);
        const tasks = Object.values(taskEdgeMap);
        const positions = computeTreeLayout(paneIds, tasks);
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
    }, [props.panes, taskEdgeMap, setNodePosition, setRfNodes]);

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
            </div>
            <ReactFlow
                nodes={rfNodes}
                edges={rfEdges}
                onNodesChange={handleNodesChange}
                onEdgesChange={onEdgesChange}
                nodeTypes={nodeTypes}
                edgeTypes={edgeTypes}
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
        </div>
    );
}
