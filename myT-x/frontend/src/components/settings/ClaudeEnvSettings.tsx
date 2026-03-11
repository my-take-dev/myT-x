import {useEffect, useMemo, useState} from "react";
import type {FormDispatch, FormState} from "./types";
import {generateId} from "./types";
import {api} from "../../api";
import {useSettingsI18n} from "./settingsI18n";
import {claudeEnvVarDescriptionsEn} from "./claudeEnvDescriptionsEn";

// Module-level cache for static descriptions data.
// Prevents redundant API calls when the settings tab is re-mounted.
let cachedDescriptions: Record<string, string> | null = null;

interface ClaudeEnvSettingsProps {
    s: FormState;
    dispatch: FormDispatch;
}

export function ClaudeEnvSettings({s, dispatch}: ClaudeEnvSettingsProps) {
    const {language, t} = useSettingsI18n();
    const [descriptions, setDescriptions] = useState<Record<string, string>>(
        cachedDescriptions ?? {},
    );
    const [descriptionLoadFailed, setDescriptionLoadFailed] = useState(false);

    useEffect(() => {
        // Skip fetch if already cached (static data, never changes at runtime).
        if (cachedDescriptions !== null) {
            setDescriptions(cachedDescriptions);
            return;
        }

        let cancelled = false;
        api.GetClaudeEnvVarDescriptions()
            .then((result) => {
                if (!cancelled) {
                    // NOTE: Even an empty response is cached to avoid retry loops.
                    // If the API returns {} it means no descriptions are available.
                    cachedDescriptions = result;
                    setDescriptions(result);
                }
            })
            .catch((err) => {
                if (!cancelled) {
                    console.warn("[ClaudeEnvSettings] failed to load descriptions", err);
                    cachedDescriptions = {}; // empty cache to prevent retry loop on next mount
                    setDescriptionLoadFailed(true);
                }
            });
        return () => {
            cancelled = true;
        };
    }, []);

    const usedKeys = useMemo(
        () => new Set(s.claudeEnvEntries.map((e) => e.key.trim()).filter(Boolean)),
        [s.claudeEnvEntries],
    );

    const availableKeys = useMemo(
        () => Object.keys(descriptions).filter((k) => !usedKeys.has(k)),
        [descriptions, usedKeys],
    );

    const datalistId = "claude-env-var-keys";

    return (
        <div className="settings-section">
            <div className="settings-section-title">
                {t("settings.claudeEnv.title", "CLAUDE CODE 環境変数", "Claude Code environment variables")}
            </div>
            <span className="settings-desc" style={{marginBottom: 8, display: "block"}}>
                {t(
                    "settings.claudeEnv.description",
                    "Claude Codeに渡す環境変数を設定します。セッション開始時の初期ターミナルを含む全てのペインに適用されます。",
                    "Configure environment variables passed to Claude Code. Applied to all panes including the initial terminal at session start.",
                )}
            </span>
            {descriptionLoadFailed && (
                <span className="settings-desc" style={{marginBottom: 4, display: "block", opacity: 0.7}}>
                    {t(
                        "settings.claudeEnv.descriptionLoadFailed",
                        "※ 変数の説明文を取得できませんでした。設定自体は問題なく利用できます。",
                        "Could not load variable descriptions. Settings can still be used.",
                    )}
                </span>
            )}

            <div className="form-checkbox-row" style={{marginBottom: 12}}>
                <input
                    type="checkbox"
                    id="claude-env-default-enabled"
                    checked={s.claudeEnvDefaultEnabled}
                    onChange={(e) =>
                        dispatch({type: "SET_FIELD", field: "claudeEnvDefaultEnabled", value: e.target.checked})
                    }
                />
                <label htmlFor="claude-env-default-enabled">
                    {t("settings.claudeEnv.defaultEnabled", "セッション作成時にデフォルトON", "Enabled by default when creating session")}
                </label>
            </div>

            <div className="form-group">
                <label className="form-label">{t("settings.claudeEnv.list.label", "環境変数一覧", "Environment variables")}</label>
                <div className="settings-note">
                    {t(
                        "settings.claudeEnv.list.note",
                        "Claude Code固有の環境変数を設定します。システム変数(PATH等)は上書きできません。",
                        "Configure Claude Code specific environment variables. System variables (e.g. PATH) cannot be overridden.",
                    )}
                </div>

                <datalist id={datalistId}>
                    {availableKeys.map((k) => (
                        <option key={k} value={k}/>
                    ))}
                </datalist>

                <div className="dynamic-list">
                    {s.claudeEnvEntries.map((entry, index) => {
                        const normalizedKey = entry.key.trim();
                        const desc = normalizedKey
                            ? (language === "en" ? claudeEnvVarDescriptionsEn[normalizedKey] ?? descriptions[normalizedKey] : descriptions[normalizedKey])
                            : undefined;
                        return (
                            <div key={entry.id} className="override-row">
                                <div className="override-fields">
                                    <div className="form-group">
                                        <input
                                            className={`form-input ${s.validationErrors[`claude_env_key_${index}`] ? "input-error" : ""}`}
                                            value={entry.key}
                                            onChange={(e) =>
                                                dispatch({
                                                    type: "UPDATE_CLAUDE_ENV_ENTRY",
                                                    index,
                                                    field: "key",
                                                    value: e.target.value,
                                                })
                                            }
                                            placeholder={t("settings.claudeEnv.key.placeholder", "変数名", "Variable name")}
                                            list={datalistId}
                                            aria-label={t(
                                                "settings.claudeEnv.key.ariaTemplate",
                                                "Claude環境変数名 {index}",
                                                "Claude environment variable name {index}",
                                                {index: index + 1},
                                            )}
                                        />
                                        {s.validationErrors[`claude_env_key_${index}`] && (
                                            <span className="form-error">
                                                {s.validationErrors[`claude_env_key_${index}`]}
                                            </span>
                                        )}
                                        {desc && (
                                            <span className="settings-desc">{desc}</span>
                                        )}
                                    </div>
                                    <div className="form-group">
                                        <input
                                            className={`form-input ${s.validationErrors[`claude_env_val_${index}`] ? "input-error" : ""}`}
                                            value={entry.value}
                                            onChange={(e) =>
                                                dispatch({
                                                    type: "UPDATE_CLAUDE_ENV_ENTRY",
                                                    index,
                                                    field: "value",
                                                    value: e.target.value,
                                                })
                                            }
                                            placeholder={t("settings.claudeEnv.value.placeholder", "値", "Value")}
                                            aria-label={t(
                                                "settings.claudeEnv.value.ariaTemplate",
                                                "Claude環境変数値 {index}",
                                                "Claude environment variable value {index}",
                                                {index: index + 1},
                                            )}
                                        />
                                        {s.validationErrors[`claude_env_val_${index}`] && (
                                            <span className="form-error">
                                                {s.validationErrors[`claude_env_val_${index}`]}
                                            </span>
                                        )}
                                    </div>
                                </div>
                                <button
                                    type="button"
                                    className="dynamic-list-remove"
                                    onClick={() =>
                                        dispatch({
                                            type: "SET_CLAUDE_ENV_ENTRIES",
                                            entries: s.claudeEnvEntries.filter((e) => e.id !== entry.id),
                                        })
                                    }
                                    title={t("settings.claudeEnv.remove.title", "削除", "Remove")}
                                    aria-label={t(
                                        "settings.claudeEnv.remove.ariaTemplate",
                                        "Claude環境変数 {name} を削除",
                                        "Remove Claude environment variable {name}",
                                        {
                                            name: entry.key || t(
                                                "settings.claudeEnv.remove.ariaFallbackItem",
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
                        );
                    })}
                    <button
                        type="button"
                        className="modal-btn dynamic-list-add"
                        onClick={() =>
                            dispatch({
                                type: "SET_CLAUDE_ENV_ENTRIES",
                                entries: [...s.claudeEnvEntries, {id: generateId(), key: "", value: ""}],
                            })
                        }
                    >
                        + {t("settings.claudeEnv.add", "環境変数追加", "Add environment variable")}
                    </button>
                </div>
            </div>
        </div>
    );
}
