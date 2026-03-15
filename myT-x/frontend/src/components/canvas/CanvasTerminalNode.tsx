import {memo} from "react";
import {Handle, type NodeProps, NodeResizer, Position} from "@xyflow/react";
import {TerminalPane} from "../TerminalPane";
import {useCanvasStore} from "../../stores/canvasStore";

interface CanvasTerminalNodeData {
    paneId: string;
    paneTitle: string;
    active: boolean;
    onFocus: (paneId: string) => void;
    onSplitVertical: (paneId: string) => void;
    onSplitHorizontal: (paneId: string) => void;
    onToggleZoom: (paneId: string) => void;
    onKillPane: (paneId: string) => void;
    onRenamePane: (paneId: string, title: string) => void | Promise<void>;
    onSwapPane: (sourcePaneId: string, targetPaneId: string) => void | Promise<void>;
    onDetach: () => void;
    [key: string]: unknown;
}

function CanvasTerminalNodeComponent(props: NodeProps) {
    const {selected} = props;
    const data = props.data as CanvasTerminalNodeData;

    const agentInfo = useCanvasStore((s) => s.agentMap[data.paneId]);
    const hasChildProcess = useCanvasStore((s) => s.processStatusMap[data.paneId] ?? false);

    const borderClass = data.active ? "canvas-node-active"
        : hasChildProcess ? "canvas-node-running"
        : "canvas-node-idle";

    return (
        <div className={`canvas-terminal-node ${borderClass}`}>
            <NodeResizer
                minWidth={350}
                minHeight={250}
                isVisible={selected === true}
                lineClassName="canvas-resize-line"
                handleClassName="canvas-resize-handle"
            />
            <Handle type="target" position={Position.Left} id="input" className="canvas-handle"/>
            {agentInfo && (
                <div className="canvas-agent-badge">{agentInfo.name}</div>
            )}
            {/* nopan nowheel: ターミナル内のスクロール/パンがキャンバスと競合しないようにする */}
            {/* onDragStart preventDefault: TerminalPaneのHTML5 draggable属性によるD&Dを無効化 */}
            <div className="nopan nowheel canvas-terminal-body" onDragStart={(e) => e.preventDefault()}>
                <TerminalPane
                    paneId={data.paneId}
                    paneTitle={data.paneTitle}
                    active={data.active}
                    onFocus={data.onFocus}
                    onSplitVertical={data.onSplitVertical}
                    onSplitHorizontal={data.onSplitHorizontal}
                    onToggleZoom={data.onToggleZoom}
                    onKillPane={data.onKillPane}
                    onRenamePane={data.onRenamePane}
                    onSwapPane={data.onSwapPane}
                    onDetach={data.onDetach}
                />
            </div>
            <Handle type="source" position={Position.Right} id="output" className="canvas-handle"/>
        </div>
    );
}

export const CanvasTerminalNode = memo(CanvasTerminalNodeComponent);
