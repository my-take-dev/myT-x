import {useCallback, useMemo} from "react";
import {useI18n} from "../../../../i18n";

export type UsageDashboardTranslate = (key: string, ja: string, en: string) => string;

export interface UsageDashboardLabels {
    readonly claude: string;
    readonly codex: string;
    readonly compare: string;
    readonly compareHelp: string;
}

export function useUsageDashboardI18n(): UsageDashboardTranslate {
    const {language, t} = useI18n();
    return useCallback(
        (key: string, ja: string, en: string) => t(key, language === "ja" ? ja : en),
        [language, t],
    );
}

export function useUsageDashboardLabels(): UsageDashboardLabels {
    const tr = useUsageDashboardI18n();
    return useMemo(() => ({
        claude: tr("viewer.usageDashboard.modeClaude", "Claude Code", "Claude Code"),
        codex: tr("viewer.usageDashboard.modeCodex", "Codex", "Codex"),
        compare: tr("viewer.usageDashboard.modeCompare", "比較", "Compare"),
        compareHelp: tr(
            "viewer.usageDashboard.compareSourcesHelp",
            "最低 1 つのソースを選択してください。",
            "Select at least one source.",
        ),
    }), [tr]);
}
