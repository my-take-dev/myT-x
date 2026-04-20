import {useCallback, useState} from "react";
import {useI18n} from "../../../../i18n";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {useUsageDashboard, type UsageMode} from "./useUsageDashboard";
import {UsageModeTabs} from "./UsageModeTabs";
import {ClaudePanel} from "./ClaudePanel";
import {CodexPanel} from "./CodexPanel";
import {BothPanel} from "./BothPanel";

export function UsageDashboardView() {
    const {language, t} = useI18n();
    const tr = (key: string, ja: string, en: string) => t(key, language === "ja" ? ja : en);
    const closeView = useViewerStore((s) => s.closeView);

    const [mode, setMode] = useState<UsageMode>("both");
    const {snapshot, isLoading, error, hasActiveSession, activeSessionName, refresh} =
        useUsageDashboard(mode);

    // Manual refresh always forces re-aggregation so the user gets fresh
    // numbers; automatic loads (mode/session change) reuse the JSON cache.
    const handleRefresh = useCallback(() => {
        refresh(true);
    }, [refresh]);

    if (!hasActiveSession) {
        return (
            <ViewerPanelShell
                className="usage-dashboard-view"
                title={tr("viewer.usageDashboard.title", "Usage Dashboard", "Usage Dashboard")}
                onClose={closeView}
                message={tr(
                    "viewer.usageDashboard.noSession",
                    "ターミナルセッションを選択してください",
                    "Select a terminal session to view its usage statistics.",
                )}
            />
        );
    }

    if (error) {
        return (
            <ViewerPanelShell
                className="usage-dashboard-view"
                title={tr("viewer.usageDashboard.title", "Usage Dashboard", "Usage Dashboard")}
                onClose={closeView}
                onRefresh={handleRefresh}
                refreshTitle={tr("viewer.usageDashboard.refresh", "更新", "Refresh")}
                message={error}
            />
        );
    }

    const labels = {
        claude: tr("viewer.usageDashboard.modeClaude", "Claude", "Claude"),
        codex: tr("viewer.usageDashboard.modeCodex", "Codex", "Codex"),
        both: tr("viewer.usageDashboard.modeBoth", "両方", "Both"),
    };

    const workDir = snapshot?.work_dir ?? "";
    const lastUpdated = snapshot?.last_updated_at
        ? new Date(snapshot.last_updated_at).toLocaleString(language === "ja" ? "ja-JP" : "en-US")
        : "";

    return (
        <ViewerPanelShell
            className="usage-dashboard-view"
            title={tr("viewer.usageDashboard.title", "Usage Dashboard", "Usage Dashboard")}
            onClose={closeView}
            onRefresh={handleRefresh}
            refreshTitle={tr("viewer.usageDashboard.refresh", "更新", "Refresh")}
        >
            <UsageModeTabs
                mode={mode}
                onModeChange={setMode}
                labelClaude={labels.claude}
                labelCodex={labels.codex}
                labelBoth={labels.both}
            />
            <div className="usage-dashboard-body">
                {isLoading && !snapshot ? (
                    <div
                        role="status"
                        aria-live="polite"
                        aria-atomic="true"
                        aria-busy="true"
                        aria-label={tr("viewer.usageDashboard.loading", "集計中...", "Aggregating...")}
                    >
                        <p className="usage-dashboard-loading-copy">
                            {tr("viewer.usageDashboard.loading", "集計中...", "Aggregating...")}
                        </p>
                        {/* Overview cards skeleton */}
                        <div className="usage-dashboard-skeleton-overview">
                            <div className="usage-dashboard-skeleton-card"/>
                            <div className="usage-dashboard-skeleton-card"/>
                            <div className="usage-dashboard-skeleton-card"/>
                        </div>
                        {/* Chart skeleton */}
                        <div className="usage-dashboard-skeleton-chart"/>
                        {/* Ranking skeleton */}
                        <div className="usage-dashboard-skeleton-rows">
                            <div className="usage-dashboard-skeleton-row"/>
                            <div className="usage-dashboard-skeleton-row"/>
                            <div className="usage-dashboard-skeleton-row"/>
                            <div className="usage-dashboard-skeleton-row"/>
                            <div className="usage-dashboard-skeleton-row"/>
                        </div>
                    </div>
                ) : mode === "claude" ? (
                    <ClaudePanel stats={snapshot?.claude}/>
                ) : mode === "codex" ? (
                    <CodexPanel stats={snapshot?.codex}/>
                ) : (
                    <BothPanel claude={snapshot?.claude} codex={snapshot?.codex}/>
                )}
            </div>
            <div className="usage-dashboard-meta">
                <span title={workDir}>
                    {tr("viewer.usageDashboard.session", "セッション", "Session")}:{" "}
                    {activeSessionName}
                </span>
                {lastUpdated ? (
                    <span>
                        {tr("viewer.usageDashboard.lastUpdated", "最終更新", "Last updated")}:{" "}
                        {lastUpdated}
                    </span>
                ) : null}
            </div>
        </ViewerPanelShell>
    );
}
