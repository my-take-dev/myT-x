import type {usagedashboard} from "../../../../../wailsjs/go/models";
import {useUsageDashboardI18n} from "./i18n";

interface RankingTableProps {
    readonly entries: ReadonlyArray<usagedashboard.UsageEntry>;
    readonly emptyLabel: string;
    readonly ariaLabel?: string;
}

export function RankingTable({entries, emptyLabel, ariaLabel}: RankingTableProps) {
    const tr = useUsageDashboardI18n();
    const resolvedAriaLabel = ariaLabel ?? tr("viewer.usageDashboard.ranking", "利用ランキング", "Usage ranking");

    if (entries.length === 0) {
        return <div className="usage-dashboard-ranking-empty">{emptyLabel}</div>;
    }
    return (
        <div className="usage-dashboard-ranking" role="table" aria-label={resolvedAriaLabel}>
            <div role="rowgroup">
                {entries.map((entry, idx) => (
                    <div key={entry.name} className="usage-dashboard-ranking-row" role="row">
                        <span
                            className="usage-dashboard-ranking-rank"
                            role="cell"
                            aria-label={tr("viewer.usageDashboard.rank", `順位 ${idx + 1}`, `rank ${idx + 1}`)}
                        >
                            {idx + 1}.
                        </span>
                        <span className="usage-dashboard-ranking-name" role="cell" title={entry.name}>
                            {entry.name}
                        </span>
                        <span className="usage-dashboard-ranking-count" role="cell">{entry.count}</span>
                        <span className="usage-dashboard-ranking-last" role="cell" title={formatTimestamp(entry.last_used_at)}>
                            {formatRelative(entry.last_used_at, tr)}
                        </span>
                    </div>
                ))}
            </div>
        </div>
    );
}

function formatTimestamp(raw: string | null | undefined): string {
    if (!raw) return "";
    const d = new Date(raw);
    if (isNaN(d.getTime())) return "";
    return d.toISOString().replace("T", " ").slice(0, 16);
}

function formatRelative(raw: string | null | undefined, tr: ReturnType<typeof useUsageDashboardI18n>): string {
    if (!raw) return "";
    const d = new Date(raw);
    if (isNaN(d.getTime())) return "";
    const diffMs = Date.now() - d.getTime();
    if (diffMs < 0) return formatTimestamp(raw);
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 1) return tr("viewer.usageDashboard.relative.now", "たった今", "just now");
    if (diffMin < 60) return tr("viewer.usageDashboard.relative.minutes", `${diffMin}分前`, `${diffMin}m ago`);
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return tr("viewer.usageDashboard.relative.hours", `${diffHr}時間前`, `${diffHr}h ago`);
    const diffDay = Math.floor(diffHr / 24);
    if (diffDay < 30) return tr("viewer.usageDashboard.relative.days", `${diffDay}日前`, `${diffDay}d ago`);
    return d.toISOString().slice(0, 10);
}
