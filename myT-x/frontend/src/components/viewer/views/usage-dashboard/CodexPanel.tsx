import type {usagedashboard} from "../../../../../wailsjs/go/models";
import {OverviewCards} from "./OverviewCards";
import {DailyActivityChart} from "./DailyActivityChart";
import {SourceHealthBanner} from "./SourceHealthBanner";
import {UsageCategorySection, type UsageCategory, type UsageCategoryKey} from "./UsageCategorySection";
import {formatNumber} from "./formatters";
import {useUsageDashboardI18n} from "./i18n";

type CodexCategoryKey = Extract<UsageCategoryKey, "agents" | "skills">;

interface CodexPanelProps {
    readonly stats: usagedashboard.CodexUsageStats | null | undefined;
    readonly compact?: boolean;
    readonly titlePrefix?: string;
}

export function CodexPanel({stats, compact = false, titlePrefix}: CodexPanelProps) {
    const tr = useUsageDashboardI18n();

    if (!stats) {
        return (
            <div className="usage-dashboard-empty">
                {tr(
                    "viewer.usageDashboard.codex.unavailable",
                    "このセッションフォルダではCodexのデータを利用できません。",
                    "Codex data is unavailable for this session folder.",
                )}
            </div>
        );
    }

    const overview = [
        {id: "sessions", label: tr("viewer.usageDashboard.sessions", "セッション", "Sessions"), value: formatNumber(stats.total_sessions)},
        {
            id: "active-days",
            label: tr("viewer.usageDashboard.activeDays", "稼働日数", "Active Days"),
            value: formatNumber(stats.active_days),
            sub: tr("viewer.usageDashboard.last30Days", "直近30日", "in last 30 days"),
        },
        {id: "spawned-agents", label: tr("viewer.usageDashboard.spawnedAgents", "起動エージェント", "Spawned Agents"), value: formatNumber(stats.total_spawned_agents)},
    ];

    const categories: ReadonlyArray<UsageCategory<CodexCategoryKey>> = [
        {
            key: "agents",
            label: tr("viewer.usageDashboard.agents", "エージェント", "Agents"),
            ranking: stats.agents,
            dailySeries: stats.agents_daily ?? [],
            emptyLabel: rankingEmptyLabel("agents", tr),
        },
        {
            key: "skills",
            label: tr("viewer.usageDashboard.skills", "スキル", "Skills"),
            ranking: stats.skills,
            dailySeries: stats.skills_daily ?? [],
            emptyLabel: rankingEmptyLabel("skills", tr),
        },
    ];

    return (
        <div className={compact ? "usage-dashboard-codex compact" : "usage-dashboard-codex"}>
            {titlePrefix ? (
                <h3 className="usage-dashboard-panel-title usage-dashboard-panel-codex">
                    {titlePrefix}
                </h3>
            ) : null}
            <SourceHealthBanner
                health={stats.health}
                sourceLabel="Codex"
                includeSqlite={true}
            />
            <OverviewCards items={overview}/>
            <UsageCategorySection
                categories={categories}
                initialCategory="agents"
                ariaLabel={tr("viewer.usageDashboard.codex.ranking", "Codexランキング", "Codex ranking")}
                colorVar="--udash-color-codex"
            />
            <section className="usage-dashboard-section">
                <h4 className="usage-dashboard-section-title">
                    {tr("viewer.usageDashboard.dailyActivity", "日次アクティビティ（30日）", "Daily Activity (30 days)")}
                </h4>
                <DailyActivityChart
                    buckets={stats.daily_activity}
                    labelSessions={tr("viewer.usageDashboard.sessions", "セッション", "Sessions")}
                    labelSecondary={tr("viewer.usageDashboard.prompts", "プロンプト", "Prompts")}
                    labelToolCalls={tr("viewer.usageDashboard.spawnedAgents", "起動エージェント", "Spawned Agents")}
                    colorVar="--udash-color-codex"
                />
            </section>
        </div>
    );
}

function rankingEmptyLabel(key: CodexCategoryKey, tr: ReturnType<typeof useUsageDashboardI18n>): string {
    switch (key) {
        case "agents":
            return tr(
                "viewer.usageDashboard.codex.emptyAgents",
                "このフォルダではまだspawn_agent呼び出しが確認されていません。",
                "No spawn_agent calls observed in this folder yet.",
            );
        case "skills":
            return tr(
                "viewer.usageDashboard.codex.emptySkills",
                "このフォルダではまだSKILL.md読み込みが確認されていません。",
                "No SKILL.md reads observed in this folder yet.",
            );
    }
}
