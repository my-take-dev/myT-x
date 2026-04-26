import type {usagedashboard} from "../../../../../wailsjs/go/models";
import {OverviewCards} from "./OverviewCards";
import {DailyActivityChart} from "./DailyActivityChart";
import {SourceHealthBanner} from "./SourceHealthBanner";
import {UsageCategorySection, type UsageCategory, type UsageCategoryKey} from "./UsageCategorySection";
import {formatNumber} from "./formatters";
import {useUsageDashboardI18n} from "./i18n";

interface ClaudePanelProps {
    readonly stats: usagedashboard.ClaudeUsageStats | null | undefined;
    readonly compact?: boolean;
    readonly titlePrefix?: string;
}

export function ClaudePanel({stats, compact = false, titlePrefix}: ClaudePanelProps) {
    const tr = useUsageDashboardI18n();

    if (!stats) {
        return (
            <div className="usage-dashboard-empty">
                {tr(
                    "viewer.usageDashboard.claude.unavailable",
                    "このセッションフォルダではClaude Codeのデータを利用できません。",
                    "Claude Code data is unavailable for this session folder.",
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
        {id: "messages", label: tr("viewer.usageDashboard.messages", "メッセージ", "Messages"), value: formatNumber(stats.total_messages)},
        {id: "tool-calls", label: tr("viewer.usageDashboard.toolCalls", "ツール呼び出し", "Tool Calls"), value: formatNumber(stats.total_tool_uses)},
    ];

    const categories: ReadonlyArray<UsageCategory<UsageCategoryKey>> = [
        {
            key: "skills",
            label: tr("viewer.usageDashboard.skills", "スキル", "Skills"),
            ranking: stats.skills,
            dailySeries: stats.skills_daily ?? [],
            emptyLabel: rankingEmptyLabel("skills", tr),
        },
        {
            key: "agents",
            label: tr("viewer.usageDashboard.agents", "エージェント", "Agents"),
            ranking: stats.agents,
            dailySeries: stats.agents_daily ?? [],
            emptyLabel: rankingEmptyLabel("agents", tr),
        },
        {
            key: "slash",
            label: tr("viewer.usageDashboard.slash", "スラッシュ", "Slash"),
            ranking: stats.slash_commands,
            dailySeries: stats.slash_commands_daily ?? [],
            emptyLabel: rankingEmptyLabel("slash", tr),
        },
    ];

    return (
        <div className={compact ? "usage-dashboard-claude compact" : "usage-dashboard-claude"}>
            {titlePrefix ? (
                <h3 className="usage-dashboard-panel-title usage-dashboard-panel-claude">
                    {titlePrefix}
                </h3>
            ) : null}
            <SourceHealthBanner
                health={stats.health}
                sourceLabel="Claude"
                includeSqlite={false}
            />
            <OverviewCards items={overview}/>
            <UsageCategorySection
                categories={categories}
                initialCategory="skills"
                ariaLabel={tr("viewer.usageDashboard.claude.ranking", "Claudeランキング", "Claude ranking")}
                colorVar="--udash-color-claude"
            />
            <section className="usage-dashboard-section">
                <h4 className="usage-dashboard-section-title">
                    {tr("viewer.usageDashboard.dailyActivity", "日次アクティビティ（30日）", "Daily Activity (30 days)")}
                </h4>
                <DailyActivityChart
                    buckets={stats.daily_activity}
                    labelSessions={tr("viewer.usageDashboard.sessions", "セッション", "Sessions")}
                    labelSecondary={tr("viewer.usageDashboard.messages", "メッセージ", "Messages")}
                    labelToolCalls={tr("viewer.usageDashboard.toolCalls", "ツール呼び出し", "Tool Calls")}
                    colorVar="--udash-color-claude"
                />
            </section>
        </div>
    );
}

function rankingEmptyLabel(key: UsageCategoryKey, tr: ReturnType<typeof useUsageDashboardI18n>): string {
    switch (key) {
        case "skills":
            return tr(
                "viewer.usageDashboard.claude.emptySkills",
                "このフォルダではまだスキル使用が確認されていません。",
                "No skill activations observed in this folder yet.",
            );
        case "agents":
            return tr(
                "viewer.usageDashboard.claude.emptyAgents",
                "このフォルダではまだサブエージェント実行が確認されていません。",
                "No subagent tasks observed in this folder yet.",
            );
        case "slash":
            return tr(
                "viewer.usageDashboard.claude.emptySlash",
                "このフォルダではまだスラッシュコマンドが確認されていません。",
                "No slash commands observed in this folder yet.",
            );
    }
}
