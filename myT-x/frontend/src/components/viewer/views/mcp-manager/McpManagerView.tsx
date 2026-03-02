import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {McpDetailPanel} from "./McpDetailPanel";
import {McpListSidebar} from "./McpListSidebar";
import {useMcpManager} from "./useMcpManager";

export function McpManagerView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {mcpList, selectedMCP, isLoading, error, toggleMCP, togglingIds, selectMCP, activeSession, retryLoad, dismissError} =
        useMcpManager();

    if (!activeSession) {
        return (
            <ViewerPanelShell
                className="mcp-manager-view"
                title="MCP Manager"
                onClose={closeView}
                // Intentionally no refresh action in this state:
                // retryLoad requires an active session and would be a no-op.
                message="No active session"
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
                    <div className="viewer-message">Loading MCP servers...</div>
                ) : (
                    <>
                        <McpListSidebar
                            items={mcpList}
                            selectedId={selectedMCP?.id ?? null}
                            onSelect={selectMCP}
                            onToggle={toggleMCP}
                            togglingIds={togglingIds}
                        />
                        <McpDetailPanel mcp={selectedMCP}/>
                    </>
                )}
            </div>
        </ViewerPanelShell>
    );
}
