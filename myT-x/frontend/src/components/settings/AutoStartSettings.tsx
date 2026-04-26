import {MAX_AUTO_START_COMMANDS} from "./settingsValidation";
import {useSettingsI18n} from "./settingsI18n";
import type {FormDispatch, FormState} from "./types";
import {generateId} from "./types";

interface AutoStartSettingsProps {
    s: FormState;
    dispatch: FormDispatch;
}

export function AutoStartSettings({s, dispatch}: AutoStartSettingsProps) {
    const {t} = useSettingsI18n();
    const autoStartError = s.validationErrors.auto_start;

    return (
        <div className="settings-section">
            <div className="settings-section-title">
                {t("settings.autoStart.title", "自動起動", "AutoStart")}
            </div>

            <div className="form-group auto-start-settings">
                <div className="settings-row-heading">
                    <label className="form-label">
                        {t("settings.autoStart.label", "AutoStart", "AutoStart")}
                    </label>
                    <button
                        type="button"
                        className="modal-btn dynamic-list-add"
                        disabled={s.autoStart.length >= MAX_AUTO_START_COMMANDS}
                        onClick={() => dispatch({
                            type: "SET_AUTO_START_ENTRIES",
                            entries: [
                                ...s.autoStart,
                                {id: generateId(), name: "", command: "", args: ""},
                            ],
                        })}
                    >
                        +
                    </button>
                </div>
                <span className="settings-desc">
                    {t(
                        "settings.autoStart.description",
                        "ペインツールバーから新規ペインで即時起動するコマンドを登録します",
                        "Commands launched into a new pane from the pane toolbar.",
                    )}
                </span>
                {autoStartError && <span className="settings-field-error">{autoStartError}</span>}

                <div className="auto-start-list">
                    {s.autoStart.map((entry, index) => {
                        const preview = [entry.command.trim(), (entry.args ?? "").trim()].filter(Boolean).join(" ");
                        const commandError = s.validationErrors[`auto_start_command_${index}`];
                        const nameError = s.validationErrors[`auto_start_name_${index}`];
                        const argsError = s.validationErrors[`auto_start_args_${index}`];
                        return (
                            <div key={entry.id} className="auto-start-row">
                                <div className="auto-start-fields">
                                    <input
                                        className={`form-input ${nameError ? "input-error" : ""}`}
                                        value={entry.name}
                                        onChange={(e) => dispatch({
                                            type: "UPDATE_AUTO_START_ENTRY",
                                            index,
                                            field: "name",
                                            value: e.target.value,
                                        })}
                                        placeholder={t(
                                            "settings.autoStart.namePlaceholder",
                                            "表示名",
                                            "Display name",
                                        )}
                                        aria-label={t(
                                            "settings.autoStart.nameAria",
                                            "AutoStart display name",
                                            "AutoStart display name",
                                        )}
                                    />
                                    <input
                                        className={`form-input ${commandError ? "input-error" : ""}`}
                                        value={entry.command}
                                        onChange={(e) => dispatch({
                                            type: "UPDATE_AUTO_START_ENTRY",
                                            index,
                                            field: "command",
                                            value: e.target.value,
                                        })}
                                        placeholder="codex"
                                        aria-label={t(
                                            "settings.autoStart.commandAria",
                                            "AutoStart command",
                                            "AutoStart command",
                                        )}
                                    />
                                    <input
                                        className={`form-input ${argsError ? "input-error" : ""}`}
                                        value={entry.args}
                                        onChange={(e) => dispatch({
                                            type: "UPDATE_AUTO_START_ENTRY",
                                            index,
                                            field: "args",
                                            value: e.target.value,
                                        })}
                                        placeholder="--model gpt-5.4-mini"
                                        aria-label={t(
                                            "settings.autoStart.argsAria",
                                            "AutoStart arguments",
                                            "AutoStart arguments",
                                        )}
                                    />
                                </div>
                                <button
                                    type="button"
                                    className="dynamic-list-remove"
                                    onClick={() => dispatch({
                                        type: "SET_AUTO_START_ENTRIES",
                                        entries: s.autoStart.filter((_, removeIndex) => removeIndex !== index),
                                    })}
                                    aria-label={t(
                                        "settings.autoStart.removeAria",
                                        "AutoStart エントリを削除",
                                        "Remove AutoStart entry",
                                    )}
                                >
                                    &times;
                                </button>
                                <div className="auto-start-preview">[{preview || "command"}]</div>
                                {(nameError || commandError || argsError) && (
                                    <span className="settings-field-error">{nameError || commandError || argsError}</span>
                                )}
                            </div>
                        );
                    })}
                </div>
            </div>
        </div>
    );
}
