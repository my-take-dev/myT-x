import {act, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {ClaudePanel} from "../src/components/viewer/views/usage-dashboard/ClaudePanel";
import {CodexPanel} from "../src/components/viewer/views/usage-dashboard/CodexPanel";
import {DailyActivityChart} from "../src/components/viewer/views/usage-dashboard/DailyActivityChart";
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
    Tooltip: () => <g data-testid="tooltip"/>,
    Bar: ({fill, dataKey}: {fill?: string; dataKey?: string}) => (
        <rect
            data-testid="chart-bar"
            data-fill={fill}
            data-key={String(dataKey ?? "")}
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

    it("hydrates generated usage models with non-null health defaults", () => {
        const claude = new usageDashboardModels.ClaudeUsageStats({});
        const codex = new usageDashboardModels.CodexUsageStats({});

        expect(claude.health.partial_errors).toEqual([]);
        expect(codex.health.partial_errors).toEqual([]);
        expect(claude.skills).toEqual([]);
        expect(codex.daily_activity).toEqual([]);
    });
});
