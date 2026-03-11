import type {FormDispatch, FormState} from "./types";
import {generateId} from "./types";
import {useSettingsI18n} from "./settingsI18n";

interface PaneEnvSettingsProps {
    s: FormState;
    dispatch: FormDispatch;
}

export function PaneEnvSettings({s, dispatch}: PaneEnvSettingsProps) {
    const {t} = useSettingsI18n();

    return (
        <div className="settings-section">
            <div className="settings-section-title">
                {t("settings.paneEnv.title", "追加ペイン用 環境変数", "Additional pane environment variables")}
            </div>
            <span className="settings-desc" style={{marginBottom: 8, display: "block"}}>
                {t(
                    "settings.paneEnv.description",
                    "セッション開始後に追加されるペインに自動で埋め込む環境変数を設定します。セッション開始時の初期ターミナルには適用されません。コマンドの -e フラグで同じキーを指定した場合、-e の値が優先されます。",
                    "Configure environment variables injected into panes created after session start. Not applied to the initial terminal. If command -e specifies the same key, -e value wins.",
                )}
            </span>

            <div className="form-checkbox-row" style={{marginBottom: 12}}>
                <input
                    type="checkbox"
                    id="pane-env-default-enabled"
                    checked={s.paneEnvDefaultEnabled}
                    onChange={(e) =>
                        dispatch({type: "SET_FIELD", field: "paneEnvDefaultEnabled", value: e.target.checked})
                    }
                />
                <label htmlFor="pane-env-default-enabled">
                    {t("settings.paneEnv.defaultEnabled", "セッション作成時にデフォルトON", "Enabled by default when creating session")}
                </label>
            </div>

            <div className="form-group">
                <label className="form-label">{t("settings.paneEnv.effortLevel.label", "思考Level (CLAUDE_CODE_EFFORT_LEVEL)", "Effort level (CLAUDE_CODE_EFFORT_LEVEL)")}</label>
                <select
                    className={`form-input ${s.validationErrors["pane_env_effort"] ? "input-error" : ""}`}
                    value={s.effortLevel}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "effortLevel", value: e.target.value})}
                >
                    <option value="">{t("settings.paneEnv.effortLevel.unset", "未設定", "Unset")}</option>
                    <option value="low">low</option>
                    <option value="medium">medium</option>
                    <option value="high">high</option>
                </select>
                {s.validationErrors["pane_env_effort"] && (
                    <span className="form-error">{s.validationErrors["pane_env_effort"]}</span>
                )}
                <span className="settings-desc">
                    {t(
                        "settings.paneEnv.effortLevel.description",
                        "CLAUDE_CODE_EFFORT_LEVEL 環境変数として設定されます",
                        "Stored as the CLAUDE_CODE_EFFORT_LEVEL environment variable.",
                    )}
                </span>
            </div>

            <div className="form-group" style={{marginTop: 8}}>
                <label className="form-label">{t("settings.paneEnv.customVars.label", "カスタム環境変数", "Custom environment variables")}</label>
                <div className="settings-note">
                    {t(
                        "settings.paneEnv.customVars.note",
                        "追加の環境変数を設定します。システム変数(PATH等)は上書きできません。",
                        "Configure additional environment variables. System variables (e.g. PATH) cannot be overridden.",
                    )}
                </div>

                <div className="dynamic-list">
                    {s.paneEnvEntries.map((entry, index) => (
                        <div key={entry.id} className="override-row">
                            <div className="override-fields">
                                <div className="form-group">
                                    <input
                                        className={`form-input ${s.validationErrors[`pane_env_key_${index}`] ? "input-error" : ""}`}
                                        value={entry.key}
                                        onChange={(e) => dispatch({
                                            type: "UPDATE_PANE_ENV_ENTRY",
                                            index,
                                            field: "key",
                                            value: e.target.value,
                                        })}
                                        placeholder={t("settings.paneEnv.key.placeholder", "変数名", "Variable name")}
                                        aria-label={t(
                                            "settings.paneEnv.key.ariaTemplate",
                                            "環境変数名 {index}",
                                            "Environment variable name {index}",
                                            {index: index + 1},
                                        )}
                                    />
                                    {s.validationErrors[`pane_env_key_${index}`] && (
                                        <span className="form-error">
                                            {s.validationErrors[`pane_env_key_${index}`]}
                                        </span>
                                    )}
                                </div>
                                <div className="form-group">
                                    <input
                                        className={`form-input ${s.validationErrors[`pane_env_val_${index}`] ? "input-error" : ""}`}
                                        value={entry.value}
                                        onChange={(e) => dispatch({
                                            type: "UPDATE_PANE_ENV_ENTRY",
                                            index,
                                            field: "value",
                                            value: e.target.value,
                                        })}
                                        placeholder={t("settings.paneEnv.value.placeholder", "値", "Value")}
                                        aria-label={t(
                                            "settings.paneEnv.value.ariaTemplate",
                                            "環境変数値 {index}",
                                            "Environment variable value {index}",
                                            {index: index + 1},
                                        )}
                                    />
                                    {s.validationErrors[`pane_env_val_${index}`] && (
                                        <span className="form-error">
                                            {s.validationErrors[`pane_env_val_${index}`]}
                                        </span>
                                    )}
                                </div>
                            </div>
                            <button
                                type="button"
                                className="dynamic-list-remove"
                                onClick={() =>
                                    dispatch({
                                        type: "SET_PANE_ENV_ENTRIES",
                                        entries: s.paneEnvEntries.filter((e) => e.id !== entry.id),
                                    })
                                }
                                title={t("settings.paneEnv.remove.title", "削除", "Remove")}
                                aria-label={t(
                                    "settings.paneEnv.remove.ariaTemplate",
                                    "環境変数 {name} を削除",
                                    "Remove environment variable {name}",
                                    {
                                        name: entry.key || t(
                                            "settings.paneEnv.remove.ariaFallbackItem",
                                            "項目{index}",
                                            "Item {index}",
                                            {index: index + 1},
                                        ),
                                    },
                                )}
                            >
                                &times;
                            </button>
                        </div>
                    ))}
                    <button
                        type="button"
                        className="modal-btn dynamic-list-add"
                        onClick={() =>
                            dispatch({
                                type: "SET_PANE_ENV_ENTRIES",
                                entries: [...s.paneEnvEntries, {id: generateId(), key: "", value: ""}],
                            })
                        }
                    >
                        + {t("settings.paneEnv.add", "環境変数追加", "Add environment variable")}
                    </button>
                </div>
            </div>
        </div>
    );
}
