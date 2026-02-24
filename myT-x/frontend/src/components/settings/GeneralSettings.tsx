import {useCallback} from "react";
import {api} from "../../api";
import {useNotificationStore} from "../../stores/notificationStore";
import type {FormDispatch, FormState} from "./types";
import {ShortcutInput} from "./ShortcutInput";

interface GeneralSettingsProps {
    s: FormState;
    dispatch: FormDispatch;
}

export function GeneralSettings({s, dispatch}: GeneralSettingsProps) {
    const addNotification = useNotificationStore((s) => s.addNotification);
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
            addNotification("ディレクトリの選択に失敗しました", "warn");
        }
    }, [dispatch, addNotification]);

    return (
        <div className="settings-section">
            <div className="settings-section-title">基本設定</div>

            <div className="form-group">
                <label className="form-label" htmlFor="shell-select">Shell</label>
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
          ターミナルペインで使用するシェル（デフォルト: powershell.exe）
        </span>
            </div>

            <div className="form-group">
                <label className="shortcut-label">Prefix</label>
                <ShortcutInput
                    value={s.prefix}
                    onChange={(v) => dispatch({type: "SET_FIELD", field: "prefix", value: v})}
                    placeholder="Ctrl+b"
                    ariaLabel="Prefix shortcut"
                />
                <span className="settings-desc">
          tmux互換プレフィックスキー。このキーに続けてアクションキーを入力して操作します
        </span>
            </div>

            <div className="form-checkbox-row">
                <input
                    type="checkbox"
                    id="quake-mode"
                    checked={s.quakeMode}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "quakeMode", value: e.target.checked})}
                />
                <label htmlFor="quake-mode">Quake Mode</label>
            </div>
            <span className="settings-desc">
        グローバルホットキーでウィンドウの表示/非表示を切替
      </span>

            <div className="form-group">
                <label className="shortcut-label">Global Hotkey</label>
                <ShortcutInput
                    value={s.globalHotkey}
                    onChange={(v) => dispatch({type: "SET_FIELD", field: "globalHotkey", value: v})}
                    placeholder="Ctrl+Shift+F12"
                    disabled={!s.quakeMode}
                    ariaLabel="Global hotkey shortcut"
                />
                <span className="settings-desc">
          Quakeモードのトグルキー（Quakeモード有効時のみ使用）（デフォルト: Ctrl+Shift+F12）
        </span>
            </div>

            <div className="form-group">
                <label className="form-label" htmlFor={defaultSessionDirInputId}>
                    デフォルトセッションディレクトリ
                </label>
                <div className="form-input-with-button">
                    <input
                        id={defaultSessionDirInputId}
                        className={`form-input ${defaultSessionDirError ? "input-error" : ""}`}
                        value={s.defaultSessionDir}
                        onChange={(e) => dispatch({
                            type: "SET_FIELD",
                            field: "defaultSessionDir",
                            value: e.target.value
                        })}
                        placeholder="未設定（起動ディレクトリを使用）"
                        aria-invalid={defaultSessionDirError ? "true" : "false"}
                    />
                    <button type="button" className="modal-btn" onClick={handlePickDefaultDir} title="フォルダを選択">
                        参照...
                    </button>
                    {s.defaultSessionDir && (
                        <button
                            type="button"
                            className="modal-btn"
                            onClick={() => dispatch({type: "SET_FIELD", field: "defaultSessionDir", value: ""})}
                            title="クリア"
                            aria-label="デフォルトセッションディレクトリをクリア"
                        >
                            &times;
                        </button>
                    )}
                </div>
                <span className="settings-desc">
          クイックスタートで使用する作業ディレクトリ。未設定の場合はアプリ起動時のディレクトリが使用されます
        </span>
                {defaultSessionDirError && <span className="form-error">{defaultSessionDirError}</span>}
            </div>
        </div>
    );
}
