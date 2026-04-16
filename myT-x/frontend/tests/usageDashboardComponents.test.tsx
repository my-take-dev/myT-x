import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it} from "vitest";
import {ClaudePanel} from "../src/components/viewer/views/usage-dashboard/ClaudePanel";
import {CodexPanel} from "../src/components/viewer/views/usage-dashboard/CodexPanel";
import {RankingTable} from "../src/components/viewer/views/usage-dashboard/RankingTable";
import {SourceHealthBanner} from "../src/components/viewer/views/usage-dashboard/SourceHealthBanner";
import {setLanguage} from "../src/i18n";
import {usagedashboard as usageDashboardModels} from "../wailsjs/go/models";
import type {usagedashboard} from "../wailsjs/go/models";

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
