import type {usagedashboard} from "../../../../../wailsjs/go/models";
import {ClaudePanel} from "./ClaudePanel";
import {CodexPanel} from "./CodexPanel";
import {useUsageDashboardI18n} from "./i18n";

interface BothPanelProps {
    readonly claude: usagedashboard.ClaudeUsageStats | null | undefined;
    readonly codex: usagedashboard.CodexUsageStats | null | undefined;
}

export function BothPanel({claude, codex}: BothPanelProps) {
    const tr = useUsageDashboardI18n();
    return (
        <div className="usage-dashboard-both">
            <ClaudePanel stats={claude} compact titlePrefix={tr("viewer.usageDashboard.claude.title", "Claude Code", "Claude Code")}/>
            <CodexPanel stats={codex} compact titlePrefix={tr("viewer.usageDashboard.codex.title", "Codex", "Codex")}/>
        </div>
    );
}
