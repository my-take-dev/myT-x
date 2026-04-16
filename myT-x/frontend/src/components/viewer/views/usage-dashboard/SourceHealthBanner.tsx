import type {usagedashboard} from "../../../../../wailsjs/go/models";
import {useUsageDashboardI18n} from "./i18n";

interface SourceHealthBannerProps {
    readonly health: usagedashboard.SourceHealth | null | undefined;
    readonly sourceLabel: string;
    readonly includeSqlite: boolean;
}

export function SourceHealthBanner(props: SourceHealthBannerProps) {
    const tr = useUsageDashboardI18n();
    const {health, sourceLabel, includeSqlite} = props;
    if (!health) return null;

    const availabilityWarnings: string[] = [];
    if (!health.jsonl_available) {
        availabilityWarnings.push(
            tr(
                "viewer.usageDashboard.health.jsonlUnavailable",
                `${sourceLabel}: JSONLログを利用できません`,
                `${sourceLabel}: JSONL logs not available`,
            ),
        );
    }
    if (!health.history_available) {
        availabilityWarnings.push(
            tr(
                "viewer.usageDashboard.health.historyUnavailable",
                `${sourceLabel}: historyログを利用できません`,
                `${sourceLabel}: history log not available`,
            ),
        );
    }
    if (includeSqlite && !health.sqlite_available) {
        availabilityWarnings.push(
            tr(
                "viewer.usageDashboard.health.sqliteUnavailable",
                `${sourceLabel}: state SQLiteを利用できません`,
                `${sourceLabel}: state SQLite not available`,
            ),
        );
    }
    const partialErrors = (health.partial_errors ?? []).filter((error) => error.trim().length > 0);
    if (availabilityWarnings.length === 0 && partialErrors.length === 0) return null;

    return (
        <div className="usage-dashboard-source-banner" role="status" aria-live="polite">
            <span aria-hidden="true">⚠</span>
            <div>
                {availabilityWarnings.map((w, i) => (
                    <div key={i}>{w}</div>
                ))}
                {partialErrors.length > 0 ? (
                    <details className="usage-dashboard-source-banner-details">
                        <summary>
                            {tr(
                                "viewer.usageDashboard.health.partialErrors",
                                `${sourceLabel}: ${partialErrors.length}件の解析警告を表示`,
                                `${sourceLabel}: show ${partialErrors.length} parse warning${partialErrors.length === 1 ? "" : "s"}`,
                            )}
                        </summary>
                        <ul className="usage-dashboard-source-banner-list">
                            {partialErrors.map((error, index) => (
                                <li key={`${sourceLabel}-partial-${index}`}>{error}</li>
                            ))}
                        </ul>
                    </details>
                ) : null}
            </div>
        </div>
    );
}
