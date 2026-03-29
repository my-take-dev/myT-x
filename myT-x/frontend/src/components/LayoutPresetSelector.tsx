import {api} from "../api";
import {useI18n} from "../i18n";
import {notifyAndLog} from "../utils/notifyUtils";

interface LayoutPresetSelectorProps {
    sessionName: string;
    paneCount: number;
}

interface PresetDef {
    id: string;
    labelKey: string;
    labelJa: string;
    labelEn: string;
    minPanes: number;
    svgPath: string;
}

const presets: PresetDef[] = [
    {
        id: "even-horizontal",
        labelKey: "layoutPreset.evenHorizontal",
        labelJa: "横均等",
        labelEn: "Even Horizontal",
        minPanes: 2,
        svgPath: "M1 1h12v12H1zM7 1v12",
    },
    {
        id: "even-vertical",
        labelKey: "layoutPreset.evenVertical",
        labelJa: "縦均等",
        labelEn: "Even Vertical",
        minPanes: 2,
        svgPath: "M1 1h12v12H1zM1 7h12",
    },
    {
        id: "main-vertical",
        labelKey: "layoutPreset.mainVertical",
        labelJa: "左メイン",
        labelEn: "Main Left",
        minPanes: 3,
        svgPath: "M1 1h12v12H1zM8.5 1v12M8.5 7h4.5",
    },
    {
        id: "main-horizontal",
        labelKey: "layoutPreset.mainHorizontal",
        labelJa: "上メイン",
        labelEn: "Main Top",
        minPanes: 3,
        svgPath: "M1 1h12v12H1zM1 8.5h12M7 8.5v4.5",
    },
    {
        id: "tiled",
        labelKey: "layoutPreset.tiled",
        labelJa: "タイル",
        labelEn: "Tiled",
        minPanes: 4,
        svgPath: "M1 1h12v12H1zM7 1v12M1 7h12",
    },
];

export function LayoutPresetSelector({sessionName, paneCount}: LayoutPresetSelectorProps) {
    const {language, t} = useI18n();

    if (paneCount < 2) return null;

    return (
        <div className="layout-preset-bar">
            {presets
                .filter((preset) => paneCount >= preset.minPanes)
                .map((preset) => {
                    const label = language === "en" ? preset.labelEn : t(preset.labelKey, preset.labelJa);
                    const ariaLabel =
                        language === "en"
                            ? `Layout: ${label}`
                            : t("layoutPreset.aria.layout", "Layout: {label}", {label});
                    return (
                        <button
                            key={preset.id}
                            type="button"
                            className="terminal-toolbar-btn layout-preset-btn"
                            title={label}
                            aria-label={ariaLabel}
                            onClick={() => {
                                void api.ApplyLayoutPreset(sessionName, preset.id).catch((err) => {
                                    console.warn("[layout-preset] ApplyLayoutPreset failed", err);
                                    notifyAndLog("Apply layout preset", "warn", err, "LayoutPresetSelector");
                                });
                            }}
                        >
                            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.2">
                                <path d={preset.svgPath}/>
                            </svg>
                        </button>
                    );
                })}
        </div>
    );
}
