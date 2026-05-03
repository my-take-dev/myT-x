import {useCallback, useMemo, useState} from "react";
import {useI18n} from "../../../../i18n";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {useUsageDashboard, type UsageDashboardSelection, type UsageSource} from "./useUsageDashboard";
import {UsageModeList} from "./UsageModeList";
import {ClaudePanel} from "./ClaudePanel";
import {CodexPanel} from "./CodexPanel";
import {UsageComparisonPanel, type UsageComparisonSource} from "./UsageComparisonPanel";
import {useUsageDashboardI18n, useUsageDashboardLabels} from "./i18n";
import {
    getComparisonSourceLockedReason,
    getInitialComparisonSources,
    type NonEmptyReadonlyArray,
    toggleComparisonSourceSelection,
} from "./usageSourceSelection";

export function UsageDashboardView() {
    const {language} = useI18n();
    const tr = useUsageDashboardI18n();
    const labels = useUsageDashboardLabels();
    const closeView = useViewerStore((s) => s.closeView);

    const [selection, setSelection] = useState<UsageDashboardSelection>("compare");
    const [selectedSources, setSelectedSources] = useState<NonEmptyReadonlyArray<UsageSource>>(
        () => getInitialComparisonSources(),
    );
    const {snapshot, isLoading, error, hasActiveSession, activeSessionName, refresh} =
        useUsageDashboard();

    // Manual refresh always forces re-aggregation so the user gets fresh
    // numbers; automatic loads (session change) reuse the JSON cache.
    const handleRefresh = useCallback(() => {
        refresh(true);
    }, [refresh]);

    const sources = useMemo<ReadonlyArray<UsageComparisonSource>>(() => [
        {
            id: "claude",
            title: labels.claude,
        },
        {
            id: "codex",
            title: labels.codex,
        },
    ], [labels.claude, labels.codex]);

    const selectionOptions = useMemo(() => [
        {id: "claude" as const, label: labels.claude},
        {id: "codex" as const, label: labels.codex},
        {id: "compare" as const, label: labels.compare},
    ], [labels.claude, labels.codex, labels.compare]);

    const handleComparisonSourceToggle = useCallback((source: UsageSource) => {
        setSelectedSources((current) => {
            return toggleComparisonSourceSelection(current, source);
        });
    }, []);

    const comparisonHelpId = "usage-dashboard-comparison-help";

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
            <UsageModeList
                selection={selection}
                onSelectionChange={setSelection}
                options={selectionOptions}
            />
            {selection === "compare" ? (
                <div
                    className="usage-dashboard-comparison-controls"
                    role="group"
                    aria-label={tr("viewer.usageDashboard.compareSources", "比較対象", "Comparison sources")}
                    aria-describedby={comparisonHelpId}
                >
                    <p id={comparisonHelpId} className="usage-dashboard-comparison-help">
                        {labels.compareHelp}
                    </p>
                    {sources.map((source) => {
                        const checked = selectedSources.includes(source.id);
                        const lockedReason = getComparisonSourceLockedReason(
                            selectedSources,
                            source.id,
                            labels.compareHelp,
                        );
                        return (
                            <label key={source.id} className="usage-dashboard-comparison-choice">
                                <input
                                    type="checkbox"
                                    checked={checked}
                                    aria-disabled={lockedReason ? "true" : undefined}
                                    title={lockedReason ?? undefined}
                                    onClick={(event) => {
                                        if (lockedReason) {
                                            event.preventDefault();
                                        }
                                    }}
                                    onChange={(event) => {
                                        if (lockedReason) {
                                            return;
                                        }
                                        handleComparisonSourceToggle(source.id);
                                    }}
                                />
                                <span>{source.title}</span>
                            </label>
                        );
                    })}
                </div>
            ) : null}
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
                ) : selection === "claude" ? (
                    <ClaudePanel stats={snapshot?.claude}/>
                ) : selection === "codex" ? (
                    <CodexPanel stats={snapshot?.codex}/>
                ) : (
                    <UsageComparisonPanel
                        snapshot={snapshot}
                        selectedSources={selectedSources}
                        sources={sources}
                    />
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
