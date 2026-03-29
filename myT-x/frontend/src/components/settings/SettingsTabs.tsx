import {useMemo} from "react";
import {GeneralSettings} from "./GeneralSettings";
import {KeybindSettings} from "./KeybindSettings";
import {WorktreeSettings} from "./WorktreeSettings";
import {AgentModelSettings} from "./AgentModelSettings";
import {PaneEnvSettings} from "./PaneEnvSettings";
import {ClaudeEnvSettings} from "./ClaudeEnvSettings";
import {useSettingsI18n} from "./settingsI18n";
import type {FormDispatch, FormState, SettingsCategory} from "./types";

interface SettingsCategoryDefinition {
    readonly id: SettingsCategory;
    readonly labelKey: string;
    readonly labelJa: string;
    readonly labelEn: string;
}

const SETTINGS_CATEGORIES: SettingsCategoryDefinition[] = [
    {id: "general", labelKey: "settings.modal.categories.general", labelJa: "基本設定", labelEn: "General"},
    {id: "keybinds", labelKey: "settings.modal.categories.keybinds", labelJa: "キーバインド", labelEn: "Keybinds"},
    {id: "worktree", labelKey: "settings.modal.categories.worktree", labelJa: "Worktree", labelEn: "Worktree"},
    {id: "agent-model", labelKey: "settings.modal.categories.agentModel", labelJa: "Agent Model", labelEn: "Agent Model"},
    {id: "claude-env", labelKey: "settings.modal.categories.claudeEnv", labelJa: "CLAUDE CODE環境変数", labelEn: "Claude Code Environment Variables"},
    {id: "pane-env", labelKey: "settings.modal.categories.paneEnv", labelJa: "追加ペイン環境変数", labelEn: "Additional Pane Environment Variables"},
];

interface SettingsTabsProps {
    readonly s: FormState;
    readonly dispatch: FormDispatch;
}

export function SettingsTabs({s, dispatch}: SettingsTabsProps) {
    const {t} = useSettingsI18n();

    const handleCategoryKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>, category: SettingsCategory) => {
        const currentIndex = SETTINGS_CATEGORIES.findIndex((item) => item.id === category);
        if (currentIndex < 0) {
            return;
        }

        let nextIndex = currentIndex;
        switch (event.key) {
            case "ArrowRight":
            case "ArrowDown":
                nextIndex = (currentIndex + 1) % SETTINGS_CATEGORIES.length;
                break;
            case "ArrowLeft":
            case "ArrowUp":
                nextIndex = (currentIndex - 1 + SETTINGS_CATEGORIES.length) % SETTINGS_CATEGORIES.length;
                break;
            case "Home":
                nextIndex = 0;
                break;
            case "End":
                nextIndex = SETTINGS_CATEGORIES.length - 1;
                break;
            default:
                return;
        }

        event.preventDefault();
        const nextCategory = SETTINGS_CATEGORIES[nextIndex]!.id;
        dispatch({type: "SET_FIELD", field: "activeCategory", value: nextCategory});
        const nextTab = document.getElementById(`settings-tab-${nextCategory}`);
        if (nextTab instanceof HTMLElement) {
            nextTab.focus();
        }
    };

    const categoryPanels = useMemo<Record<SettingsCategory, () => JSX.Element>>(() => ({
        general: () => <GeneralSettings s={s} dispatch={dispatch}/>,
        keybinds: () => <KeybindSettings s={s} dispatch={dispatch}/>,
        worktree: () => <WorktreeSettings s={s} dispatch={dispatch}/>,
        "agent-model": () => <AgentModelSettings s={s} dispatch={dispatch}/>,
        "claude-env": () => <ClaudeEnvSettings s={s} dispatch={dispatch}/>,
        "pane-env": () => <PaneEnvSettings s={s} dispatch={dispatch}/>,
    }), [s, dispatch]);

    return (
        <div className="settings-layout">
            <nav
                className="settings-sidebar"
                role="tablist"
                aria-label={t("settings.modal.categoriesAria", "設定カテゴリ", "Settings categories")}
            >
                {SETTINGS_CATEGORIES.map((cat) => {
                    const tabID = `settings-tab-${cat.id}`;
                    const panelID = `settings-panel-${cat.id}`;
                    return (
                        <button
                            key={cat.id}
                            id={tabID}
                            role="tab"
                            aria-selected={s.activeCategory === cat.id}
                            aria-controls={panelID}
                            tabIndex={s.activeCategory === cat.id ? 0 : -1}
                            className={`settings-sidebar-item ${s.activeCategory === cat.id ? "active" : ""}`}
                            onClick={() => dispatch({
                                type: "SET_FIELD",
                                field: "activeCategory",
                                value: cat.id,
                            })}
                            onKeyDown={(event) => handleCategoryKeyDown(event, cat.id)}
                        >
                            {t(cat.labelKey, cat.labelJa, cat.labelEn)}
                        </button>
                    );
                })}
            </nav>

            <div className="settings-body">
                {SETTINGS_CATEGORIES.map((cat) => {
                    const isActive = s.activeCategory === cat.id;
                    return (
                        <div
                            key={cat.id}
                            id={`settings-panel-${cat.id}`}
                            role="tabpanel"
                            aria-labelledby={`settings-tab-${cat.id}`}
                            hidden={!isActive}
                        >
                            {isActive ? categoryPanels[cat.id]() : null}
                        </div>
                    );
                })}
            </div>
        </div>
    );
}
