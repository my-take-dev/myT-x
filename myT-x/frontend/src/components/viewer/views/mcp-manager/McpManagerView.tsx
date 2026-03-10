import {useState} from "react";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {McpDetailPanel} from "./McpDetailPanel";
import {OrchestratorDetailPanel} from "./OrchestratorDetailPanel";
import {useMcpManager} from "./useMcpManager";

type McpCategory = "lsp" | "orchestrator";

export function McpManagerView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {
        lspMcpList,
        orchMcpList,
        representativeMCP,
        orchRepresentativeMCP,
        aggregateStatus,
        isLoading,
        error,
        activeSession,
        retryLoad,
        dismissError,
    } = useMcpManager();

    const [selectedCategory, setSelectedCategory] = useState<McpCategory>(
        orchMcpList.length > 0 && lspMcpList.length === 0 ? "orchestrator" : "lsp",
    );

    if (activeSession == null) {
        return (
            <ViewerPanelShell
                className="mcp-manager-view"
                title="MCP Manager"
                onClose={closeView}
                message="アクティブなセッションがありません"
            />
        );
    }

    const hasContent = lspMcpList.length > 0 || orchMcpList.length > 0;

    return (
        <ViewerPanelShell
            className="mcp-manager-view"
            title="MCP Manager"
            onClose={closeView}
            onRefresh={retryLoad}
        >
            <div className="mcp-manager-body">
                {error && (
                    <div className="mcp-manager-error-banner">
                        <span className="mcp-manager-error-text">{error}</span>
                        <div className="mcp-manager-error-actions">
                            <button type="button" className="viewer-header-btn" onClick={retryLoad} title="Retry" aria-label="Retry">
                                Retry
                            </button>
                            <button type="button" className="viewer-header-btn" onClick={dismissError} title="Dismiss" aria-label="Dismiss">
                                Dismiss
                            </button>
                        </div>
                    </div>
                )}
                {isLoading ? (
                    <div className="viewer-message">MCPプロファイルを読み込み中...</div>
                ) : !hasContent ? (
                    <div className="viewer-message">
                        {error ? "このセッションのMCPプロファイルを読み込めませんでした。" : "このセッションで利用可能なMCPプロファイルはありません。"}
                    </div>
                ) : (
                    <>
                        <aside className="mcp-list-sidebar" aria-label="MCP categories">
                            <ul className="mcp-category-list">
                                {lspMcpList.length > 0 && (
                                    <li
                                        className={`mcp-category-item${selectedCategory === "lsp" ? " mcp-category-item-selected" : ""}`}
                                        onClick={() => setSelectedCategory("lsp")}
                                    >
                                        {aggregateStatus != null && (
                                            <span className={`mcp-status-dot ${aggregateStatus}`} title={aggregateStatus}/>
                                        )}
                                        <span className="mcp-list-name">LSP-MCP</span>
                                        <span className="mcp-category-count">{lspMcpList.length}</span>
                                    </li>
                                )}
                                {orchMcpList.length > 0 && (
                                    <li
                                        className={`mcp-category-item${selectedCategory === "orchestrator" ? " mcp-category-item-selected" : ""}`}
                                        onClick={() => setSelectedCategory("orchestrator")}
                                    >
                                        <span className="mcp-list-name">Agent Orchestrator</span>
                                        <span className="mcp-category-count">{orchMcpList.length}</span>
                                    </li>
                                )}
                            </ul>
                        </aside>
                        {selectedCategory === "orchestrator" ? (
                            <OrchestratorDetailPanel
                                representativeMCP={orchRepresentativeMCP}
                                activeSession={activeSession}
                            />
                        ) : (
                            <McpDetailPanel
                                representativeMCP={representativeMCP}
                                activeSession={activeSession}
                                aggregateStatus={aggregateStatus}
                                totalLspCount={lspMcpList.length}
                            />
                        )}
                    </>
                )}
            </div>
        </ViewerPanelShell>
    );
}
