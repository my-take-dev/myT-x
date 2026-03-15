import {useCallback} from "react";
import {api} from "../../api";
import {useNotificationStore} from "../../stores/notificationStore";
import type {FormDispatch, FormState} from "./types";
import {ShortcutInput} from "./ShortcutInput";
import {useSettingsI18n} from "./settingsI18n";

interface GeneralSettingsProps {
    s: FormState;
    dispatch: FormDispatch;
}

export function GeneralSettings({s, dispatch}: GeneralSettingsProps) {
    const {t} = useSettingsI18n();
    const addNotification = useNotificationStore((state) => state.addNotification);
    const defaultSessionDirInputId = "default-session-dir";
    const defaultSessionDirError = s.validationErrors.default_session_dir;

    const handlePickDefaultDir = useCallback(async () => {
        try {
            const dir = await api.PickSessionDirectory();
            if (dir) {
                dispatch({type: "SET_FIELD", field: "defaultSessionDir", value: dir});
            }
        } catch (err) {
            console.warn("[settings] PickSessionDirectory failed", err);
            addNotification(
                t(
                    "settings.general.notification.pickDirectoryFailed",
                    "ディレクトリの選択に失敗しました",
                    "Failed to select directory.",
                ),
                "warn",
            );
        }
    }, [dispatch, addNotification, t]);

    return (
        <div className="settings-section">
            <div className="settings-section-title">{t("settings.general.title", "基本設定", "General")}</div>

            <div className="form-group">
                <label className="form-label" htmlFor="shell-select">{t("settings.general.shell.label", "Shell", "Shell")}</label>
                <select
                    id="shell-select"
                    className="form-select"
                    value={s.shell}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "shell", value: e.target.value})}
                >
                    {s.allowedShells.map((sh) => (
                        <option key={sh} value={sh}>
                            {sh}
                        </option>
                    ))}
                    {!s.allowedShells.includes(s.shell) && (
                        <option value={s.shell}>{s.shell}</option>
                    )}
                </select>
                <span className="settings-desc">
                    {t(
                        "settings.general.shell.description",
                        "ターミナルペインで使用するシェル（デフォルト: powershell.exe）",
                        "Shell used in terminal panes (default: powershell.exe).",
                    )}
                </span>
            </div>

            <div className="form-group">
                <label className="shortcut-label">{t("settings.general.prefix.label", "Prefix", "Prefix")}</label>
                <ShortcutInput
                    value={s.prefix}
                    onChange={(v) => dispatch({type: "SET_FIELD", field: "prefix", value: v})}
                    placeholder="Ctrl+b"
                    ariaLabel={t("settings.general.prefix.aria", "Prefix shortcut", "Prefix shortcut")}
                />
                <span className="settings-desc">
                    {t(
                        "settings.general.prefix.description",
                        "tmux互換プレフィックスキー。このキーに続けてアクションキーを入力して操作します",
                        "tmux-compatible prefix key. Press this key before action keys.",
                    )}
                </span>
            </div>

            <div className="form-checkbox-row">
                <input
                    type="checkbox"
                    id="quake-mode"
                    checked={s.quakeMode}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "quakeMode", value: e.target.checked})}
                />
                <label htmlFor="quake-mode">{t("settings.general.quakeMode.label", "Quake Mode", "Quake Mode")}</label>
            </div>
            <span className="settings-desc">
                {t(
                    "settings.general.quakeMode.description",
                    "グローバルホットキーでウィンドウの表示/非表示を切替",
                    "Toggle window visibility via global hotkey.",
                )}
            </span>

            <div className="form-group">
                <label className="shortcut-label">{t("settings.general.globalHotkey.label", "Global Hotkey", "Global Hotkey")}</label>
                <ShortcutInput
                    value={s.globalHotkey}
                    onChange={(v) => dispatch({type: "SET_FIELD", field: "globalHotkey", value: v})}
                    placeholder="Ctrl+Shift+F12"
                    disabled={!s.quakeMode}
                    ariaLabel={t("settings.general.globalHotkey.aria", "Global hotkey shortcut", "Global hotkey shortcut")}
                />
                <span className="settings-desc">
                    {t(
                        "settings.general.globalHotkey.description",
                        "Quakeモードのトグルキー（Quakeモード有効時のみ使用）（デフォルト: Ctrl+Shift+F12）",
                        "Toggle key for Quake mode (used only when Quake mode is enabled, default: Ctrl+Shift+F12).",
                    )}
                </span>
            </div>

            <div className="form-group">
                <label className="form-label" htmlFor={defaultSessionDirInputId}>
                    {t(
                        "settings.general.defaultSessionDir.label",
                        "デフォルトセッションディレクトリ",
                        "Default session directory",
                    )}
                </label>
                <div className="form-input-with-button">
                    <input
                        id={defaultSessionDirInputId}
                        className={`form-input ${defaultSessionDirError ? "input-error" : ""}`}
                        value={s.defaultSessionDir}
                        onChange={(e) => dispatch({
                            type: "SET_FIELD",
                            field: "defaultSessionDir",
                            value: e.target.value,
                        })}
                        placeholder={t(
                            "settings.general.defaultSessionDir.placeholder",
                            "未設定（起動ディレクトリを使用）",
                            "Not set (uses startup directory)",
                        )}
                        aria-invalid={defaultSessionDirError ? "true" : "false"}
                    />
                    <button
                        type="button"
                        className="modal-btn"
                        onClick={handlePickDefaultDir}
                        title={t("settings.general.defaultSessionDir.browseTitle", "フォルダを選択", "Choose folder")}
                    >
                        {t("settings.general.defaultSessionDir.browse", "参照...", "Browse...")}
                    </button>
                    {s.defaultSessionDir && (
                        <button
                            type="button"
                            className="modal-btn"
                            onClick={() => dispatch({type: "SET_FIELD", field: "defaultSessionDir", value: ""})}
                            title={t("settings.general.defaultSessionDir.clearTitle", "クリア", "Clear")}
                            aria-label={t(
                                "settings.general.defaultSessionDir.clearAria",
                                "デフォルトセッションディレクトリをクリア",
                                "Clear default session directory",
                            )}
                        >
                            &times;
                        </button>
                    )}
                </div>
                <span className="settings-desc">
                    {t(
                        "settings.general.defaultSessionDir.description",
                        "クイックスタートで使用する作業ディレクトリ。未設定の場合はアプリ起動時のディレクトリが使用されます",
                        "Working directory used by Quick Start. If unset, the app startup directory is used.",
                    )}
                </span>
                {defaultSessionDirError && <span className="form-error">{defaultSessionDirError}</span>}
            </div>

            <div className="form-group">
                <label className="form-label" htmlFor="chat-overlay-percentage">
                    {t(
                        "settings.general.chatOverlayPercentage.label",
                        "チャットオーバーレイの高さ (%)",
                        "Chat overlay height (%)",
                    )}
                </label>
                <input
                    id="chat-overlay-percentage"
                    className="form-input"
                    type="number"
                    min={30}
                    max={95}
                    value={s.chatOverlayPercentage}
                    onChange={(e) => {
                        const v = Math.max(30, Math.min(95, Number(e.target.value) || 80));
                        dispatch({type: "SET_FIELD", field: "chatOverlayPercentage", value: v});
                    }}
                    style={{width: "100px"}}
                />
                <span className="settings-desc">
                    {t(
                        "settings.general.chatOverlayPercentage.description",
                        "チャット入力欄を拡大した時のターミナル領域に対する高さの割合（30-95%、デフォルト: 80）",
                        "Height ratio of the expanded chat overlay relative to the terminal area (30-95%, default: 80).",
                    )}
                </span>
            </div>
        </div>
    );
}
