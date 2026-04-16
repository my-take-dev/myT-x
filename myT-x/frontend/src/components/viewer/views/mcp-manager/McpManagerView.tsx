import {useEffect, useRef, useState} from "react";
import {useI18n} from "../../../../i18n";
import {useViewerStore} from "../../viewerStore";
import {CustomMcpDetailPanel} from "./CustomMcpDetailPanel";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {McpDetailPanel} from "./McpDetailPanel";
import {OrchestratorDetailPanel} from "./OrchestratorDetailPanel";
import {SingleTaskRunnerDetailPanel} from "./SingleTaskRunnerDetailPanel";
import {useMcpManager} from "./useMcpManager";

type McpCategory = "lsp" | "orchestrator" | "single-task-runner" | "custom";

function getPreferredCategory(
    lspCount: number,
    orchestratorCount: number,
    singleTaskRunnerCount: number,
    customCount: number,
): McpCategory | null {
    if (orchestratorCount > 0) {
        return "orchestrator";
    }
    if (singleTaskRunnerCount > 0) {
        return "single-task-runner";
    }
    if (lspCount > 0) {
        return "lsp";
    }
    if (customCount > 0) {
        return "custom";
    }
    return null;
}

function isCategoryAvailable(
    category: McpCategory,
    lspCount: number,
    orchestratorCount: number,
    singleTaskRunnerCount: number,
    customCount: number,
): boolean {
    switch (category) {
        case "orchestrator":
            return orchestratorCount > 0;
        case "single-task-runner":
            return singleTaskRunnerCount > 0;
        case "lsp":
            return lspCount > 0;
        case "custom":
            return customCount > 0;
        default:
            return false;
    }
}

export function McpManagerView() {
    const {language, t} = useI18n();
    const closeView = useViewerStore((s) => s.closeView);
    const {
        lspMcpList,
        customMcpList,
        orchMcpList,
        strMcpList,
        representativeMCP,
        customRepresentativeMCP,
        orchRepresentativeMCP,
        strRepresentativeMCP,
        isLoading,
        error,
        activeSession,
        activeSessionKey,
        retryLoad,
        dismissError,
    } = useMcpManager();

    const [selectedCategory, setSelectedCategory] = useState<McpCategory>("lsp");
    const hasAutoSelectedCategoryRef = useRef(false);
    const autoSelectSessionStateRef = useRef({
        sessionKey: activeSessionKey,
        loadingObserved: false,
        isInitialSession: true,
    });

    // After the async load resolves, auto-select the preferred category once per
    // active session. Later refreshes preserve the user's selection unless the
    // current category no longer exists in the latest snapshot set. Session
    // switches wait for the new session's first resolved load so stale snapshots
    // from the previous session cannot lock in the wrong category.
    useEffect(() => {
        if (autoSelectSessionStateRef.current.sessionKey !== activeSessionKey) {
            autoSelectSessionStateRef.current = {
                sessionKey: activeSessionKey,
                loadingObserved: false,
                isInitialSession: false,
            };
            hasAutoSelectedCategoryRef.current = false;
        }

        if (isLoading) {
            autoSelectSessionStateRef.current.loadingObserved = true;
            return;
        }

        const preferredCategory = getPreferredCategory(
            lspMcpList.length,
            orchMcpList.length,
            strMcpList.length,
            customMcpList.length,
        );
        if (preferredCategory == null) {
            return;
        }

        setSelectedCategory((currentCategory) => {
            if (!hasAutoSelectedCategoryRef.current) {
                if (
                    !autoSelectSessionStateRef.current.isInitialSession
                    && !autoSelectSessionStateRef.current.loadingObserved
                ) {
                    return currentCategory;
                }
                hasAutoSelectedCategoryRef.current = true;
                return preferredCategory;
            }

            return isCategoryAvailable(
                currentCategory,
                lspMcpList.length,
                orchMcpList.length,
                strMcpList.length,
                customMcpList.length,
            )
                ? currentCategory
                : preferredCategory;
        });
    }, [activeSessionKey, isLoading, orchMcpList.length, strMcpList.length, lspMcpList.length, customMcpList.length]);

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

    const hasContent =
        lspMcpList.length > 0 || orchMcpList.length > 0 || strMcpList.length > 0 || customMcpList.length > 0;

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
                            <button type="button" className="viewer-header-btn" onClick={retryLoad} title="Retry"
                                    aria-label="Retry">
                                Retry
                            </button>
                            <button type="button" className="viewer-header-btn" onClick={dismissError} title="Dismiss"
                                    aria-label="Dismiss">
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
                                {strMcpList.length > 0 && (
                                    <li
                                        className={`mcp-category-item${selectedCategory === "single-task-runner" ? " mcp-category-item-selected" : ""}`}
                                        onClick={() => setSelectedCategory("single-task-runner")}
                                    >
                                        <span className="mcp-list-name">Single Task Runner</span>
                                        <span className="mcp-category-count">{strMcpList.length}</span>
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
                                {customMcpList.length > 0 && (
                                    <li
                                        className={`mcp-category-item${selectedCategory === "custom" ? " mcp-category-item-selected" : ""}`}
                                        onClick={() => setSelectedCategory("custom")}
                                    >
                                        <span className="mcp-list-name">Custom MCP</span>
                                        <span className="mcp-category-count">{customMcpList.length}</span>
                                    </li>
                                )}
                            </ul>
                        </aside>
                        {selectedCategory === "orchestrator" ? (
                            <OrchestratorDetailPanel
                                representativeMCP={orchRepresentativeMCP}
                                activeSession={activeSession}
                            />
                        ) : selectedCategory === "single-task-runner" ? (
                            <SingleTaskRunnerDetailPanel
                                representativeMCP={strRepresentativeMCP}
                                activeSession={activeSession}
                            />
                        ) : selectedCategory === "custom" ? (
                            <CustomMcpDetailPanel
                                representativeMCP={customRepresentativeMCP}
                                activeSession={activeSession}
                                totalCustomCount={customMcpList.length}
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
