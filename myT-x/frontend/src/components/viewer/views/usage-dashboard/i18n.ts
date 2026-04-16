import {useCallback} from "react";
import {useI18n} from "../../../../i18n";

export type UsageDashboardTranslate = (key: string, ja: string, en: string) => string;

export function useUsageDashboardI18n(): UsageDashboardTranslate {
    const {language, t} = useI18n();
    return useCallback(
        (key: string, ja: string, en: string) => t(key, language === "ja" ? ja : en),
        [language, t],
    );
}
