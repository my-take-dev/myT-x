import type {FormDispatch, FormState} from "./types";
import {ShortcutInput} from "./ShortcutInput";
import {ViewerShortcutSettings} from "./ViewerShortcutSettings";
import type {KnownKeyBinding} from "../../types/tmux";
import {useSettingsI18n} from "./settingsI18n";

interface KeyBindingDefinition {
    key: KnownKeyBinding;
    labelKey: string;
    labelJa: string;
    labelEn: string;
    defaultVal: string;
}

const KEY_BINDINGS: KeyBindingDefinition[] = [
    {key: "split-vertical", labelKey: "settings.keybinds.actions.splitVertical", labelJa: "垂直分割", labelEn: "Split vertical", defaultVal: "%"},
    {key: "split-horizontal", labelKey: "settings.keybinds.actions.splitHorizontal", labelJa: "水平分割", labelEn: "Split horizontal", defaultVal: '"'},
    {key: "toggle-zoom", labelKey: "settings.keybinds.actions.toggleZoom", labelJa: "ズーム切替", labelEn: "Toggle zoom", defaultVal: "z"},
    {key: "kill-pane", labelKey: "settings.keybinds.actions.killPane", labelJa: "ペイン閉じる", labelEn: "Kill pane", defaultVal: "x"},
    {key: "detach-session", labelKey: "settings.keybinds.actions.detachSession", labelJa: "デタッチ", labelEn: "Detach session", defaultVal: "d"},
];

interface KeybindSettingsProps {
    s: FormState;
    dispatch: FormDispatch;
}

export function KeybindSettings({s, dispatch}: KeybindSettingsProps) {
    const {t} = useSettingsI18n();

    return (
        <>
            <div className="settings-section">
                <div className="settings-section-title">{t("settings.keybinds.title", "キーバインド", "Keybinds")}</div>
                <span className="settings-desc" style={{marginBottom: 8, display: "block"}}>
                    {t(
                        "settings.keybinds.description",
                        "プレフィックスキーに続けて入力するアクションキー",
                        "Action keys entered after the prefix key.",
                    )}
                </span>

                {KEY_BINDINGS.map((kb) => (
                    <div className="form-group" key={kb.key}>
                        <label className="shortcut-label" htmlFor={`keybind-${kb.key}`}>
                            {t(kb.labelKey, kb.labelJa, kb.labelEn)}
                        </label>
                        <ShortcutInput
                            id={`keybind-${kb.key}`}
                            value={s.keys[kb.key] || ""}
                            onChange={(v) => dispatch({type: "UPDATE_KEY", key: kb.key, value: v})}
                            placeholder={kb.defaultVal}
                            ariaLabel={t(
                                "settings.keybinds.shortcutAriaTemplate",
                                `${kb.key} shortcut`,
                                `${kb.key} shortcut`,
                            )}
                        />
                    </div>
                ))}
            </div>

            <ViewerShortcutSettings s={s} dispatch={dispatch}/>
        </>
    );
}
