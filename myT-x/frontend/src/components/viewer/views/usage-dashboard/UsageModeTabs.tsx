import type {UsageMode} from "./useUsageDashboard";
import {useUsageDashboardI18n} from "./i18n";

interface UsageModeTabsProps {
    readonly mode: UsageMode;
    readonly onModeChange: (mode: UsageMode) => void;
    readonly labelClaude: string;
    readonly labelCodex: string;
    readonly labelBoth: string;
}

export function UsageModeTabs(props: UsageModeTabsProps) {
    const tr = useUsageDashboardI18n();
    const {mode, onModeChange, labelClaude, labelCodex, labelBoth} = props;
    return (
        <div
            role="tablist"
            aria-label={tr("viewer.usageDashboard.modeTabsAria", "使用ソース", "Usage source")}
            className="usage-dashboard-tabs"
        >
            <ModeTab current={mode} target="claude" label={labelClaude} onSelect={onModeChange}/>
            <ModeTab current={mode} target="codex" label={labelCodex} onSelect={onModeChange}/>
            <ModeTab current={mode} target="both" label={labelBoth} onSelect={onModeChange}/>
        </div>
    );
}

interface ModeTabProps {
    readonly current: UsageMode;
    readonly target: UsageMode;
    readonly label: string;
    readonly onSelect: (mode: UsageMode) => void;
}

function ModeTab({current, target, label, onSelect}: ModeTabProps) {
    const selected = current === target;
    return (
        <button
            type="button"
            role="tab"
            aria-selected={selected}
            className="usage-dashboard-tab"
            onClick={() => {
                if (!selected) onSelect(target);
            }}
        >
            {label}
        </button>
    );
}
