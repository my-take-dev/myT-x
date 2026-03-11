import {getLanguage, translate, useI18n} from "../../i18n";

type TranslationParams = Record<string, string | number>;

function formatTemplate(template: string, params?: TranslationParams): string {
    if (!params) {
        return template;
    }
    return template.replace(/\{(\w+)\}/g, (_, key: string) => {
        const value = params[key];
        return value === undefined ? "" : String(value);
    });
}

function toLocalizedText(
    key: string,
    japaneseText: string,
    englishText: string,
    params?: TranslationParams,
): string {
    if (getLanguage() === "en") {
        return formatTemplate(englishText, params);
    }
    return translate(key, japaneseText, params);
}

export function translateSettings(
    key: string,
    japaneseText: string,
    englishText: string,
    params?: TranslationParams,
): string {
    return toLocalizedText(key, japaneseText, englishText, params);
}

export function useSettingsI18n() {
    const {language, setLanguage, t} = useI18n();
    const settingsT = (
        key: string,
        japaneseText: string,
        englishText: string,
        params?: TranslationParams,
    ): string => {
        if (language === "en") {
            return formatTemplate(englishText, params);
        }
        return t(key, japaneseText, params);
    };

    return {
        language,
        setLanguage,
        t: settingsT,
    };
}
