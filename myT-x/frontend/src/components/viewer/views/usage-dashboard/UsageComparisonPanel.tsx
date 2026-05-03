import {useMemo} from "react";
import type {usagedashboard} from "../../../../../wailsjs/go/models";
import type {UsageSource} from "./useUsageDashboard";
import {ClaudePanel} from "./ClaudePanel";
import {CodexPanel} from "./CodexPanel";
import {useUsageDashboardI18n} from "./i18n";

export interface UsageComparisonSource {
    readonly id: UsageSource;
    readonly title: string;
}

interface UsageComparisonPanelProps {
    readonly snapshot: usagedashboard.UsageDashboardSnapshot | null;
    readonly selectedSources: ReadonlyArray<UsageSource>;
    readonly sources: ReadonlyArray<UsageComparisonSource>;
}

export function UsageComparisonPanel({snapshot, selectedSources, sources}: UsageComparisonPanelProps) {
    const tr = useUsageDashboardI18n();
    const selectedSourceSet = useMemo(() => new Set<UsageSource>(selectedSources), [selectedSources]);
    const visibleSources = useMemo(
        () => sources.filter((source) => selectedSourceSet.has(source.id)),
        [selectedSourceSet, sources],
    );
    const compact = visibleSources.length > 1;

    if (visibleSources.length === 0) {
        return (
            <div className="usage-dashboard-comparison" data-source-count="0">
                <div className="usage-dashboard-comparison-empty">
                    {tr("viewer.usageDashboard.noComparisonSources", "比較対象がありません。", "No comparison sources selected.")}
                </div>
            </div>
        );
    }

    return (
        <div className="usage-dashboard-comparison" data-source-count={visibleSources.length}>
            {visibleSources.map((source) => (
                <div key={source.id} className="usage-dashboard-comparison-item" data-source={source.id}>
                    {source.id === "claude" ? (
                        <ClaudePanel
                            stats={snapshot?.claude}
                            compact={compact}
                            titlePrefix={compact ? source.title : undefined}
                        />
                    ) : (
                        <CodexPanel
                            stats={snapshot?.codex}
                            compact={compact}
                            titlePrefix={compact ? source.title : undefined}
                        />
                    )}
                </div>
            ))}
        </div>
    );
}
