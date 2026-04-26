import {act, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {ClaudePanel} from "../src/components/viewer/views/usage-dashboard/ClaudePanel";
import {CodexPanel} from "../src/components/viewer/views/usage-dashboard/CodexPanel";
import {DailyActivityChart} from "../src/components/viewer/views/usage-dashboard/DailyActivityChart";
import {
    buildStackedItemDailyUsageData,
    buildStackedTooltipRows,
} from "../src/components/viewer/views/usage-dashboard/ItemDailyUsageChart";
import {OverviewCards} from "../src/components/viewer/views/usage-dashboard/OverviewCards";
import {RankingTable} from "../src/components/viewer/views/usage-dashboard/RankingTable";
import {SourceHealthBanner} from "../src/components/viewer/views/usage-dashboard/SourceHealthBanner";
import {UsageModeTabs} from "../src/components/viewer/views/usage-dashboard/UsageModeTabs";
import {setLanguage} from "../src/i18n";
import {usagedashboard as usageDashboardModels} from "../wailsjs/go/models";
import type {usagedashboard} from "../wailsjs/go/models";

vi.mock("recharts", () => ({
    ResponsiveContainer: ({children}: {children?: ReactNode}) => (
        <div data-testid="responsive-container">{children}</div>
    ),
    BarChart: ({children}: {children?: ReactNode}) => (
        <svg data-testid="bar-chart">{children}</svg>
    ),
    CartesianGrid: () => <g data-testid="cartesian-grid"/>,
    XAxis: () => <g data-testid="x-axis"/>,
    YAxis: () => <g data-testid="y-axis"/>,
    Tooltip: ({wrapperStyle}: {wrapperStyle?: React.CSSProperties}) => (
        <g
            data-testid="tooltip"
            data-pointer-events={String(wrapperStyle?.pointerEvents ?? "")}
        />
    ),
    Bar: ({fill, dataKey, name, stackId}: {fill?: string; dataKey?: string; name?: string; stackId?: string}) => (
        <rect
            data-testid="chart-bar"
            data-fill={fill}
            data-key={String(dataKey ?? "")}
            data-name={String(name ?? "")}
            data-stack-id={String(stackId ?? "")}
        />
    ),
}));

describe("usage dashboard components", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        setLanguage("en");
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        setLanguage("ja");
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("shows expandable parse warning details instead of truncating them", () => {
        const health = {
            jsonl_available: true,
            history_available: false,
            sqlite_available: true,
            project_dir: "D:/myT-x/dev-myT-x",
            partial_errors: [
                "line 10: malformed payload",
                "line 42: truncated record",
                "line 77: invalid timestamp",
            ],
        } satisfies usagedashboard.SourceHealth;

        act(() => {
            root.render(
                <SourceHealthBanner
                    health={health}
                    sourceLabel="Claude"
                    includeSqlite={false}
                />,
            );
        });

        expect(container.textContent).toContain("Claude: history log not available");
        expect(container.textContent).toContain("Claude: show 3 parse warnings");
        expect(container.textContent).toContain("line 10: malformed payload");
        expect(container.textContent).toContain("line 42: truncated record");
        expect(container.textContent).toContain("line 77: invalid timestamp");
    });

    it("renders ranking rows with accessible table semantics", () => {
        const entries = [
            {
                name: "golang-expert",
                count: 5,
                last_used_at: "2026-04-15T20:00:00Z",
            },
        ] satisfies usagedashboard.UsageEntry[];

        act(() => {
            root.render(
                <RankingTable
                    entries={entries}
                    emptyLabel="No ranking data"
                    ariaLabel="Claude ranking"
                />,
            );
        });

        const table = container.querySelector('[role="table"][aria-label="Claude ranking"]');
        expect(table).toBeTruthy();
        expect(container.querySelector('[role="rowgroup"]')).toBeTruthy();
        expect(container.querySelectorAll('[role="cell"]')).toHaveLength(4);
    });

    it("marks top-3 ranking rows and writes normalized usage-bar percentages", () => {
        const entries = [
            {name: "alpha", count: 10, last_used_at: "2026-04-15T20:00:00Z"},
            {name: "beta", count: 5, last_used_at: "2026-04-15T20:00:00Z"},
            {name: "gamma", count: 2, last_used_at: "2026-04-15T20:00:00Z"},
            {name: "delta", count: 1, last_used_at: "2026-04-15T20:00:00Z"},
        ] satisfies usagedashboard.UsageEntry[];

        act(() => {
            root.render(<RankingTable entries={entries} emptyLabel="No ranking data"/>);
        });

        const rows = container.querySelectorAll<HTMLElement>(".usage-dashboard-ranking-row");
        expect(rows).toHaveLength(4);
        expect(rows[0]?.style.getPropertyValue("--entry-pct")).toBe("100%");
        expect(rows[1]?.style.getPropertyValue("--entry-pct")).toBe("50%");
        expect(rows[2]?.style.getPropertyValue("--entry-pct")).toBe("20%");
        expect(rows[3]?.style.getPropertyValue("--entry-pct")).toBe("10%");
        expect(rows[0]?.querySelector(".rank-top-1")).toBeTruthy();
        expect(rows[1]?.querySelector(".rank-top-2")).toBeTruthy();
        expect(rows[2]?.querySelector(".rank-top-3")).toBeTruthy();
        expect(rows[3]?.querySelector('[class*="rank-top-"]')).toBeNull();
    });

    it("clamps invalid ranking percentages instead of leaking NaN or negative values", () => {
        const entries = [
            {name: "zero", count: 0, last_used_at: "2026-04-15T20:00:00Z"},
            {name: "negative", count: -3, last_used_at: "2026-04-15T20:00:00Z"},
        ] satisfies usagedashboard.UsageEntry[];

        act(() => {
            root.render(<RankingTable entries={entries} emptyLabel="No ranking data"/>);
        });

        const rows = container.querySelectorAll<HTMLElement>(".usage-dashboard-ranking-row");
        expect(rows).toHaveLength(2);
        expect(rows[0]?.style.getPropertyValue("--entry-pct")).toBe("0%");
        expect(rows[1]?.style.getPropertyValue("--entry-pct")).toBe("0%");
        for (const row of rows) {
            expect(row.style.getPropertyValue("--entry-pct")).not.toContain("NaN");
            expect(row.style.getPropertyValue("--entry-pct")).not.toContain("-");
        }
    });

    it("keeps non-zero ranking bars visible for long-tail usage counts", () => {
        const entries = [
            {name: "dominant", count: 1000, last_used_at: "2026-04-15T20:00:00Z"},
            {name: "tail", count: 1, last_used_at: "2026-04-15T20:00:00Z"},
        ] satisfies usagedashboard.UsageEntry[];

        act(() => {
            root.render(<RankingTable entries={entries} emptyLabel="No ranking data"/>);
        });

        const rows = container.querySelectorAll<HTMLElement>(".usage-dashboard-ranking-row");
        expect(rows).toHaveLength(2);
        expect(rows[0]?.style.getPropertyValue("--entry-pct")).toBe("100%");
        expect(rows[1]?.style.getPropertyValue("--entry-pct")).toBe("1%");
    });

    it("renders overview cards with stable data-card-id attributes", () => {
        act(() => {
            root.render(
                <OverviewCards
                    items={[
                        {id: "sessions", label: "Sessions", value: "10"},
                        {id: "tool-calls", label: "Tool Calls", value: "24", sub: "last 30 days"},
                    ]}
                />,
            );
        });

        const cards = container.querySelectorAll<HTMLElement>(".usage-dashboard-card");
        expect(cards).toHaveLength(2);
        expect(cards[0]?.dataset.cardId).toBe("sessions");
        expect(cards[1]?.dataset.cardId).toBe("tool-calls");
    });

    it("renders mode tabs with data-mode attributes for CSS targeting", () => {
        act(() => {
            root.render(
                <UsageModeTabs
                    mode="codex"
                    onModeChange={() => undefined}
                    labelClaude="Claude"
                    labelCodex="Codex"
                    labelBoth="Both"
                />,
            );
        });

        const tabs = container.querySelectorAll<HTMLButtonElement>(".usage-dashboard-tab");
        expect(tabs).toHaveLength(3);
        expect(tabs[0]?.dataset.mode).toBe("claude");
        expect(tabs[1]?.dataset.mode).toBe("codex");
        expect(tabs[2]?.dataset.mode).toBe("both");
    });

    it("wires each daily activity chart instance to its own gradient fill", () => {
        const buckets = [
            {date: "2026-04-15", sessions: 3, secondary: 1, tool_calls: 5},
        ] satisfies usagedashboard.DailyBucket[];

        act(() => {
            root.render(
                <>
                    <DailyActivityChart
                        buckets={buckets}
                        labelSessions="Sessions"
                        labelSecondary="Secondary"
                        labelToolCalls="Tool Calls"
                        colorVar="--udash-color-claude"
                    />
                    <DailyActivityChart
                        buckets={buckets}
                        labelSessions="Sessions"
                        labelSecondary="Secondary"
                        labelToolCalls="Tool Calls"
                        colorVar="--udash-color-claude"
                    />
                </>,
            );
        });

        const gradients = Array.from(
            container.querySelectorAll<SVGElement>('[id^="udash-bar-grad-"]'),
        );
        const bars = Array.from(container.querySelectorAll<SVGElement>('[data-testid="chart-bar"]'));

        expect(gradients).toHaveLength(2);
        expect(bars).toHaveLength(2);

        const ids = gradients.map((gradient) => gradient.getAttribute("id"));
        expect(ids[0]).toBeTruthy();
        expect(ids[1]).toBeTruthy();
        expect(ids[0]).not.toBe(ids[1]);
        expect(bars[0]?.getAttribute("data-fill")).toBe(`url(#${ids[0]})`);
        expect(bars[1]?.getAttribute("data-fill")).toBe(`url(#${ids[1]})`);
    });

    it("switches the daily activity chart bar series when a tab is selected", () => {
        const buckets = [
            {date: "2026-04-15", sessions: 3, secondary: 1, tool_calls: 5},
        ] satisfies usagedashboard.DailyBucket[];

        act(() => {
            root.render(
                <DailyActivityChart
                    buckets={buckets}
                    labelSessions="Sessions"
                    labelSecondary="Secondary"
                    labelToolCalls="Tool Calls"
                    colorVar="--udash-color-codex"
                />,
            );
        });

        const tabs = container.querySelectorAll<HTMLButtonElement>('.usage-dashboard-chart-series-btn[role="tab"]');
        expect(tabs).toHaveLength(3);
        expect(container.querySelector('[data-testid="chart-bar"]')?.getAttribute("data-key")).toBe("sessions");

        act(() => {
            tabs[2]?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.querySelector('[data-testid="chart-bar"]')?.getAttribute("data-key")).toBe("tool_calls");
    });

    it("renders usage dashboard panels in Japanese when the UI language is ja", () => {
        setLanguage("ja");
        const claudeStats = {
            total_sessions: 1,
            active_days: 1,
            total_messages: 2,
            total_tool_uses: 3,
            skills: [],
            agents: [],
            slash_commands: [],
            skills_daily: [],
            agents_daily: [],
            slash_commands_daily: [],
            daily_activity: [],
            health: {
                jsonl_available: true,
                history_available: true,
                sqlite_available: false,
                project_dir: "D:/myT-x/dev-myT-x",
                partial_errors: [],
            },
        } satisfies usagedashboard.ClaudeUsageStats;
        const codexStats = {
            total_sessions: 1,
            active_days: 1,
            total_prompts: 2,
            total_spawned_agents: 3,
            skills: [],
            agents: [],
            skills_daily: [],
            agents_daily: [],
            daily_activity: [],
            health: {
                jsonl_available: true,
                history_available: true,
                sqlite_available: true,
                project_dir: "D:/myT-x/dev-myT-x",
                partial_errors: [],
            },
        } satisfies usagedashboard.CodexUsageStats;

        act(() => {
            root.render(
                <>
                    <ClaudePanel stats={claudeStats}/>
                    <CodexPanel stats={codexStats}/>
                </>,
            );
        });

        expect(container.textContent).toContain("日次アクティビティ");
        expect(container.textContent).toContain("このフォルダではまだスキル使用が確認されていません。");
        expect(container.textContent).toContain("このフォルダではまだspawn_agent呼び出しが確認されていません。");
    });

    it("defaults a category daily graph to all stacked items", () => {
        const claudeStats = {
            total_sessions: 1,
            active_days: 2,
            total_messages: 2,
            total_tool_uses: 3,
            skills: [
                {name: "alpha", count: 2, last_used_at: "2026-04-15T20:00:00Z"},
                {name: "beta", count: 1, last_used_at: "2026-04-14T20:00:00Z"},
            ],
            agents: [
                {name: "agent-one", count: 1, last_used_at: "2026-04-15T20:00:00Z"},
            ],
            slash_commands: [],
            skills_daily: [
                {
                    name: "alpha",
                    total_count: 2,
                    last_used_at: "2026-04-15T20:00:00Z",
                    buckets: [
                        {date: "2026-04-14", count: 1},
                        {date: "2026-04-15", count: 1},
                    ],
                },
                {
                    name: "beta",
                    total_count: 1,
                    last_used_at: "2026-04-14T20:00:00Z",
                    buckets: [
                        {date: "2026-04-14", count: 1},
                        {date: "2026-04-15", count: 0},
                    ],
                },
            ],
            agents_daily: [
                {
                    name: "agent-one",
                    total_count: 1,
                    last_used_at: "2026-04-15T20:00:00Z",
                    buckets: [
                        {date: "2026-04-14", count: 0},
                        {date: "2026-04-15", count: 1},
                    ],
                },
            ],
            slash_commands_daily: [],
            daily_activity: [],
            health: {
                jsonl_available: true,
                history_available: true,
                sqlite_available: false,
                project_dir: "D:/myT-x/dev-myT-x",
                partial_errors: [],
            },
        } satisfies usagedashboard.ClaudeUsageStats;

        act(() => {
            root.render(<ClaudePanel stats={claudeStats}/>);
        });

        expect(container.querySelector(".usage-dashboard-ranking-row")?.textContent).toContain("alpha");

        const dailyTab = Array.from(container.querySelectorAll<HTMLButtonElement>(".usage-dashboard-view-toggle-btn"))
            .find((button) => button.textContent === "Daily");
        expect(dailyTab).toBeTruthy();

        act(() => {
            dailyTab?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const selector = container.querySelector<HTMLSelectElement>(".usage-dashboard-item-select");
        expect(selector).toBeTruthy();
        expect(selector?.value).toBe("all");

        const bars = container.querySelectorAll('.usage-dashboard-item-chart [data-testid="chart-bar"]');
        expect(bars).toHaveLength(2);
        expect(bars[0]?.getAttribute("data-stack-id")).toBe("usage-items");
        expect(bars[0]?.getAttribute("data-name")).toBe("alpha");
        expect(bars[1]?.getAttribute("data-stack-id")).toBe("usage-items");
        expect(bars[1]?.getAttribute("data-name")).toBe("beta");
        expect(container.querySelector('.usage-dashboard-item-chart [data-testid="tooltip"]')?.getAttribute("data-pointer-events"))
            .toBe("auto");

        act(() => {
            if (!selector) return;
            selector.value = "item:beta";
            selector.dispatchEvent(new Event("change", {bubbles: true}));
        });

        expect(container.querySelector<HTMLSelectElement>(".usage-dashboard-item-select")?.value).toBe("item:beta");
        const singleBar = container.querySelector('.usage-dashboard-item-chart [data-testid="chart-bar"]');
        expect(singleBar?.getAttribute("data-key")).toBe("count");
        expect(singleBar?.getAttribute("data-stack-id")).toBe("");

        const agentsTab = Array.from(container.querySelectorAll<HTMLButtonElement>(".usage-dashboard-subtab"))
            .find((button) => button.textContent === "Agents (1)");
        act(() => {
            agentsTab?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.querySelector<HTMLSelectElement>(".usage-dashboard-item-select")?.value).toBe("all");
        const agentBar = container.querySelector('.usage-dashboard-item-chart [data-testid="chart-bar"]');
        expect(agentBar?.getAttribute("data-stack-id")).toBe("usage-items");
        expect(agentBar?.getAttribute("data-name")).toBe("agent-one");
    });

    it("builds all-item daily tooltip totals and sorted non-zero rows", () => {
        const dailySeries = [
            {
                name: "alpha",
                total_count: 4,
                last_used_at: "2026-04-15T20:00:00Z",
                buckets: [
                    {date: "2026-04-14", count: 1},
                    {date: "2026-04-15", count: 3},
                    {date: "2026-04-16", count: 0},
                ],
            },
            {
                name: "beta",
                total_count: 2,
                last_used_at: "2026-04-15T20:00:00Z",
                buckets: [
                    {date: "2026-04-14", count: 2},
                    {date: "2026-04-15", count: 0},
                    {date: "2026-04-16", count: 0},
                ],
            },
        ] satisfies usagedashboard.DailyUsageSeries[];

        const chartData = buildStackedItemDailyUsageData(dailySeries);

        expect(chartData.series.map((series) => series.key)).toEqual(["item_0", "item_1"]);
        expect(chartData.data).toHaveLength(3);
        expect(chartData.data[0]?.date).toBe("2026-04-14");
        expect(chartData.data[0]?.total).toBe(3);
        const firstDay = chartData.data[0];
        const zeroDay = chartData.data[2];
        if (!firstDay || !zeroDay) {
            throw new Error("missing chart day");
        }
        expect(buildStackedTooltipRows(firstDay).map((row) => `${row.name}:${row.count}`))
            .toEqual(["beta:2", "alpha:1"]);
        expect(zeroDay.total).toBe(0);
        expect(buildStackedTooltipRows(zeroDay)).toEqual([]);
    });

    it("uses dedicated stack colors and collapses overflow series into Other", () => {
        const dailySeries = Array.from({length: 10}, (_, index) => ({
            name: `item-${index + 1}`,
            total_count: index + 1,
            last_used_at: `2026-04-${String(index + 1).padStart(2, "0")}T20:00:00Z`,
            buckets: [
                {date: "2026-04-15", count: index + 1},
            ],
        })) satisfies usagedashboard.DailyUsageSeries[];

        const chartData = buildStackedItemDailyUsageData(dailySeries);

        expect(chartData.series).toHaveLength(8);
        expect(chartData.series.map((series) => series.color)).toEqual([
            "var(--udash-stack-color-1)",
            "var(--udash-stack-color-2)",
            "var(--udash-stack-color-3)",
            "var(--udash-stack-color-4)",
            "var(--udash-stack-color-5)",
            "var(--udash-stack-color-6)",
            "var(--udash-stack-color-7)",
            "var(--udash-stack-color-8)",
        ]);
        expect(chartData.series[7]?.name).toBe("Other");
        expect(chartData.data[0]?.item_7).toBe(27);
    });

    it("uses ranking item counts and hides unranked daily series", () => {
        const codexStats = {
            total_sessions: 1,
            active_days: 1,
            total_prompts: 0,
            total_spawned_agents: 12,
            skills: [],
            agents: Array.from({length: 10}, (_, index) => ({
                name: `agent-${index + 1}`,
                count: 1,
                last_used_at: "2026-04-15T20:00:00Z",
            })),
            skills_daily: [],
            agents_daily: Array.from({length: 12}, (_, index) => ({
                name: `agent-${index + 1}`,
                total_count: 1,
                last_used_at: "2026-04-15T20:00:00Z",
                buckets: [{date: "2026-04-15", count: 1}],
            })),
            daily_activity: [],
            health: {
                jsonl_available: true,
                history_available: true,
                sqlite_available: true,
                project_dir: "D:/myT-x/dev-myT-x",
                partial_errors: [],
            },
        } satisfies usagedashboard.CodexUsageStats;

        act(() => {
            root.render(<CodexPanel stats={codexStats}/>);
        });

        expect(container.querySelector(".usage-dashboard-subtab")?.textContent).toBe("Agents (10)");

        const dailyTab = Array.from(container.querySelectorAll<HTMLButtonElement>(".usage-dashboard-view-toggle-btn"))
            .find((button) => button.textContent === "Daily");
        act(() => {
            dailyTab?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const options = Array.from(container.querySelectorAll<HTMLOptionElement>(".usage-dashboard-item-select option"))
            .map((option) => option.textContent);
        expect(options).toHaveLength(11);
        expect(options).toContain("agent-10 (1)");
        expect(options).not.toContain("agent-11 (1)");
        expect(options).not.toContain("agent-12 (1)");
    });

    it("shows the existing empty style when a daily category has no items", () => {
        const codexStats = {
            total_sessions: 1,
            active_days: 0,
            total_prompts: 0,
            total_spawned_agents: 0,
            skills: [],
            agents: [],
            skills_daily: [],
            agents_daily: [],
            daily_activity: [],
            health: {
                jsonl_available: true,
                history_available: true,
                sqlite_available: true,
                project_dir: "D:/myT-x/dev-myT-x",
                partial_errors: [],
            },
        } satisfies usagedashboard.CodexUsageStats;

        act(() => {
            root.render(<CodexPanel stats={codexStats}/>);
        });

        const dailyTab = Array.from(container.querySelectorAll<HTMLButtonElement>(".usage-dashboard-view-toggle-btn"))
            .find((button) => button.textContent === "Daily");
        act(() => {
            dailyTab?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.querySelector(".usage-dashboard-ranking-empty")?.textContent)
            .toContain("No spawn_agent calls observed in this folder yet.");
    });

    it("hydrates generated usage models when nested source data is provided", () => {
        const claude = usageDashboardModels.ClaudeUsageStats.createFrom({
            skills: [{name: "skill-a", count: 1, last_used_at: "2026-04-15T20:00:00Z"}],
            skills_daily: [
                {
                    name: "skill-a",
                    total_count: 1,
                    last_used_at: "2026-04-15T20:00:00Z",
                    buckets: [{date: "2026-04-15", count: 1}],
                },
            ],
            health: {
                jsonl_available: true,
                history_available: true,
                sqlite_available: false,
                project_dir: "D:/myT-x/dev-myT-x",
                partial_errors: [],
            },
        });
        const codex = usageDashboardModels.CodexUsageStats.createFrom({
            daily_activity: [{date: "2026-04-15", sessions: 1, secondary: 0, tool_calls: 2}],
            agents_daily: [
                {
                    name: "agent-a",
                    total_count: 1,
                    last_used_at: "2026-04-15T20:00:00Z",
                    buckets: [{date: "2026-04-15", count: 1}],
                },
            ],
            health: {
                jsonl_available: true,
                history_available: true,
                sqlite_available: true,
                project_dir: "D:/myT-x/dev-myT-x",
                partial_errors: [],
            },
        });

        expect(claude.health).toBeInstanceOf(usageDashboardModels.SourceHealth);
        expect(claude.health.partial_errors).toEqual([]);
        expect(claude.skills[0]).toBeInstanceOf(usageDashboardModels.UsageEntry);
        expect(claude.skills_daily[0]).toBeInstanceOf(usageDashboardModels.DailyUsageSeries);
        expect(claude.skills_daily[0]?.buckets[0]).toBeInstanceOf(usageDashboardModels.DailyUsageBucket);
        expect(codex.health).toBeInstanceOf(usageDashboardModels.SourceHealth);
        expect(codex.daily_activity[0]).toBeInstanceOf(usageDashboardModels.DailyBucket);
        expect(codex.agents_daily[0]).toBeInstanceOf(usageDashboardModels.DailyUsageSeries);
    });
});
