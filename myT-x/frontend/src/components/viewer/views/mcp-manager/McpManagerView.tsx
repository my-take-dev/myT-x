import {useEffect, useState} from "react";
import {useI18n} from "../../../../i18n";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {McpDetailPanel} from "./McpDetailPanel";
import {OrchestratorDetailPanel} from "./OrchestratorDetailPanel";
import {useMcpManager} from "./useMcpManager";

type McpCategory = "lsp" | "orchestrator";

export function McpManagerView() {
    const {language, t} = useI18n();
    const closeView = useViewerStore((s) => s.closeView);
    const {
        lspMcpList,
        orchMcpList,
        representativeMCP,
        orchRepresentativeMCP,
        isLoading,
        error,
        activeSession,
        retryLoad,
        dismissError,
    } = useMcpManager();

    const [selectedCategory, setSelectedCategory] = useState<McpCategory>("lsp");

    // Correct the selected category after async data load completes.
    useEffect(() => {
        if (!isLoading && orchMcpList.length > 0 && lspMcpList.length === 0) {
            setSelectedCategory("orchestrator");
        } else if (!isLoading && orchMcpList.length > 0) {
            setSelectedCategory("orchestrator");
        }
    }, [isLoading, orchMcpList.length, lspMcpList.length]);

    if (activeSession == null) {
        return (
            <ViewerPanelShell
                className="mcp-manager-view"
                title="MCP Manager"
                onClose={closeView}
                message={t(
                    "viewer.mcpManager.noActiveSession",
                    language === "ja" ? "アクティブなセッションがありません" : "No active session",
                )}
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
                    <div className="viewer-message">
                        {t(
                            "viewer.mcpManager.loadingProfiles",
                            language === "ja" ? "MCPプロファイルを読み込み中..." : "Loading MCP profiles...",
                        )}
                    </div>
                ) : !hasContent ? (
                    <div className="viewer-message">
                        {error
                            ? t(
                                "viewer.mcpManager.loadFailed",
                                language === "ja"
                                    ? "このセッションのMCPプロファイルを読み込めませんでした。"
                                    : "Could not load MCP profiles for this session.",
                            )
                            : t(
                                "viewer.mcpManager.noProfiles",
                                language === "ja"
                                    ? "このセッションで利用可能なMCPプロファイルはありません。"
                                    : "No MCP profiles are available in this session.",
                            )}
                    </div>
                ) : (
                    <>
                        <aside className="mcp-list-sidebar" aria-label="MCP categories">
                            <ul className="mcp-category-list">
                                {orchMcpList.length > 0 && (
                                    <li
                                        className={`mcp-category-item${selectedCategory === "orchestrator" ? " mcp-category-item-selected" : ""}`}
                                        onClick={() => setSelectedCategory("orchestrator")}
                                    >
                                        <span className="mcp-list-name">Agent Orchestrator</span>
                                        <span className="mcp-category-count">{orchMcpList.length}</span>
                                    </li>
                                )}
                                {lspMcpList.length > 0 && (
                                    <li
                                        className={`mcp-category-item${selectedCategory === "lsp" ? " mcp-category-item-selected" : ""}`}
                                        onClick={() => setSelectedCategory("lsp")}
                                    >
                                        <span className="mcp-list-name">LSP-MCP</span>
                                        <span className="mcp-category-count">{lspMcpList.length}</span>
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
                                totalLspCount={lspMcpList.length}
                            />
                        )}
                    </>
                )}
            </div>
        </ViewerPanelShell>
    );
}
