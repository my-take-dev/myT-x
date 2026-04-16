import {useState} from "react";
import type {usagedashboard} from "../../../../../wailsjs/go/models";
import {OverviewCards} from "./OverviewCards";
import {RankingTable} from "./RankingTable";
import {DailyActivityChart} from "./DailyActivityChart";
import {SourceHealthBanner} from "./SourceHealthBanner";
import {formatNumber} from "./formatters";
import {useUsageDashboardI18n} from "./i18n";

type RankingKey = "agents" | "skills";

interface CodexPanelProps {
    readonly stats: usagedashboard.CodexUsageStats | null | undefined;
    readonly compact?: boolean;
    readonly titlePrefix?: string;
}

const CODEX_COLOR = "#61afef";

export function CodexPanel({stats, compact = false, titlePrefix}: CodexPanelProps) {
    const tr = useUsageDashboardI18n();
    const [ranking, setRanking] = useState<RankingKey>("agents");

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

    const rankingEntries = ranking === "agents" ? stats.agents : stats.skills;

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
            <section className="usage-dashboard-section">
                <div
                    className="usage-dashboard-subtabs"
                    role="tablist"
                    aria-label={tr("viewer.usageDashboard.codex.ranking", "Codexランキング", "Codex ranking")}
                >
                    <RankingTab current={ranking} target="agents" label={`${tr("viewer.usageDashboard.agents", "エージェント", "Agents")} (${stats.agents.length})`} onSelect={setRanking}/>
                    <RankingTab current={ranking} target="skills" label={`${tr("viewer.usageDashboard.skills", "スキル", "Skills")} (${stats.skills.length})`} onSelect={setRanking}/>
                </div>
                <RankingTable
                    entries={rankingEntries}
                    emptyLabel={rankingEmptyLabel(ranking, tr)}
                    ariaLabel={tr("viewer.usageDashboard.codex.ranking", "Codexランキング", "Codex ranking")}
                />
            </section>
            <section className="usage-dashboard-section">
                <h4 className="usage-dashboard-section-title">
                    {tr("viewer.usageDashboard.dailyActivity", "日次アクティビティ（30日）", "Daily Activity (30 days)")}
                </h4>
                <DailyActivityChart
                    buckets={stats.daily_activity}
                    labelSessions={tr("viewer.usageDashboard.sessions", "セッション", "Sessions")}
                    labelSecondary={tr("viewer.usageDashboard.prompts", "プロンプト", "Prompts")}
                    labelToolCalls={tr("viewer.usageDashboard.spawnedAgents", "起動エージェント", "Spawned Agents")}
                    color={CODEX_COLOR}
                />
            </section>
        </div>
    );
}

interface RankingTabProps {
    readonly current: RankingKey;
    readonly target: RankingKey;
    readonly label: string;
    readonly onSelect: (next: RankingKey) => void;
}

function RankingTab({current, target, label, onSelect}: RankingTabProps) {
    const selected = current === target;
    return (
        <button
            type="button"
            role="tab"
            aria-selected={selected}
            className="usage-dashboard-subtab"
            onClick={() => {
                if (!selected) onSelect(target);
            }}
        >
            {label}
        </button>
    );
}

function rankingEmptyLabel(key: RankingKey, tr: ReturnType<typeof useUsageDashboardI18n>): string {
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
