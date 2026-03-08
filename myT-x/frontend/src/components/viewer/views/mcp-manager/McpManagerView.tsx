import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {McpDetailPanel} from "./McpDetailPanel";
import {useMcpManager} from "./useMcpManager";

export function McpManagerView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {
        lspMcpList,
        representativeMCP,
        aggregateStatus,
        isLoading,
        error,
        activeSession,
        retryLoad,
        dismissError,
    } = useMcpManager();

    if (activeSession == null) {
        return (
            <ViewerPanelShell
                className="mcp-manager-view"
                title="MCP Manager"
                onClose={closeView}
                // Intentionally no refresh action in this state:
                // retryLoad requires an active session and would be a no-op.
                message="アクティブなセッションがありません"
            />
        );
    }

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
                    <div className="viewer-message">LSP-MCPプロファイルを読み込み中...</div>
                ) : lspMcpList.length === 0 || aggregateStatus == null ? (
                    // aggregateStatus is currently null only when the list is empty,
                    // but the hook contract still models it as nullable.
                    <div className="viewer-message">
                        {error ? "このセッションのLSP-MCPプロファイルを読み込めませんでした。" : "このセッションで利用可能なLSP-MCPプロファイルはありません。"}
                    </div>
                ) : (
                    <>
                        <aside className="mcp-list-sidebar" aria-label="MCP categories">
                            <ul className="mcp-category-list">
                                <li className="mcp-category-item mcp-category-item-selected">
                                    <span className={`mcp-status-dot ${aggregateStatus}`} title={aggregateStatus}/>
                                    <span className="mcp-list-name">LSP-MCP</span>
                                    <span className="mcp-category-count">{lspMcpList.length}</span>
                                </li>
                            </ul>
                        </aside>
                        <McpDetailPanel
                            representativeMCP={representativeMCP}
                            activeSession={activeSession}
                            aggregateStatus={aggregateStatus}
                            totalLspCount={lspMcpList.length}
                        />
                    </>
                )}
            </div>
        </ViewerPanelShell>
    );
}
