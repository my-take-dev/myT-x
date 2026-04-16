import {useState} from "react";
import type {usagedashboard} from "../../../../../wailsjs/go/models";
import {OverviewCards} from "./OverviewCards";
import {RankingTable} from "./RankingTable";
import {DailyActivityChart} from "./DailyActivityChart";
import {SourceHealthBanner} from "./SourceHealthBanner";
import {formatNumber} from "./formatters";
import {useUsageDashboardI18n} from "./i18n";

type RankingKey = "skills" | "agents" | "slash";

interface ClaudePanelProps {
    readonly stats: usagedashboard.ClaudeUsageStats | null | undefined;
    readonly compact?: boolean;
    readonly titlePrefix?: string;
}

const CLAUDE_COLOR = "#d97757";

export function ClaudePanel({stats, compact = false, titlePrefix}: ClaudePanelProps) {
    const tr = useUsageDashboardI18n();
    const [ranking, setRanking] = useState<RankingKey>("skills");

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

    const rankingEntries =
        ranking === "skills" ? stats.skills
        : ranking === "agents" ? stats.agents
        : stats.slash_commands;

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
            <section className="usage-dashboard-section">
                <div
                    className="usage-dashboard-subtabs"
                    role="tablist"
                    aria-label={tr("viewer.usageDashboard.claude.ranking", "Claudeランキング", "Claude ranking")}
                >
                    <RankingTab current={ranking} target="skills" label={`${tr("viewer.usageDashboard.skills", "スキル", "Skills")} (${stats.skills.length})`} onSelect={setRanking}/>
                    <RankingTab current={ranking} target="agents" label={`${tr("viewer.usageDashboard.agents", "エージェント", "Agents")} (${stats.agents.length})`} onSelect={setRanking}/>
                    <RankingTab current={ranking} target="slash" label={`${tr("viewer.usageDashboard.slash", "スラッシュ", "Slash")} (${stats.slash_commands.length})`} onSelect={setRanking}/>
                </div>
                <RankingTable
                    entries={rankingEntries}
                    emptyLabel={rankingEmptyLabel(ranking, tr)}
                    ariaLabel={tr("viewer.usageDashboard.claude.ranking", "Claudeランキング", "Claude ranking")}
                />
            </section>
            <section className="usage-dashboard-section">
                <h4 className="usage-dashboard-section-title">
                    {tr("viewer.usageDashboard.dailyActivity", "日次アクティビティ（30日）", "Daily Activity (30 days)")}
                </h4>
                <DailyActivityChart
                    buckets={stats.daily_activity}
                    labelSessions={tr("viewer.usageDashboard.sessions", "セッション", "Sessions")}
                    labelSecondary={tr("viewer.usageDashboard.messages", "メッセージ", "Messages")}
                    labelToolCalls={tr("viewer.usageDashboard.toolCalls", "ツール呼び出し", "Tool Calls")}
                    color={CLAUDE_COLOR}
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
