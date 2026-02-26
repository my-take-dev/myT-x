import {useViewerStore} from "../../viewerStore";
import {McpDetailPanel} from "./McpDetailPanel";
import {McpListSidebar} from "./McpListSidebar";
import {useMcpManager} from "./useMcpManager";

export function McpManagerView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {mcpList, selectedMCP, isLoading, error, toggleMCP, togglingIds, selectMCP, activeSession, retryLoad, dismissError} =
        useMcpManager();

    const header = (
        <div className="viewer-header">
            <h2 className="viewer-header-title">MCP Manager</h2>
            <div className="viewer-header-spacer"/>
            <button type="button" className="viewer-header-btn" onClick={closeView} title="Close">
                {"\u2715"}
            </button>
        </div>
    );

    if (!activeSession) {
        return (
            <div className="mcp-manager-view">
                {header}
                <div className="viewer-message">No active session</div>
            </div>
        );
    }

    return (
        <div className="mcp-manager-view">
            {header}
            <div className="mcp-manager-body">
                {error && (
                    <div className="mcp-manager-error-banner">
                        <span className="mcp-manager-error-text">{error}</span>
                        <div className="mcp-manager-error-actions">
                            <button type="button" className="viewer-header-btn" onClick={retryLoad} title="Retry">
                                Retry
                            </button>
                            <button type="button" className="viewer-header-btn" onClick={dismissError} title="Dismiss">
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
        </div>
    );
}
