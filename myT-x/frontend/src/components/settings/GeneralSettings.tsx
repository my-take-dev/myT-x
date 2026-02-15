import type { FormDispatch, FormState } from "./types";
import { ShortcutInput } from "./ShortcutInput";

interface GeneralSettingsProps {
  s: FormState;
  dispatch: FormDispatch;
}

export function GeneralSettings({ s, dispatch }: GeneralSettingsProps) {
  return (
    <div className="settings-section">
      <div className="settings-section-title">基本設定</div>

      <div className="form-group">
        <label className="form-label">Shell</label>
        <select
          className="form-select"
          value={s.shell}
          onChange={(e) => dispatch({ type: "SET_FIELD", field: "shell", value: e.target.value })}
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
          onChange={(v) => dispatch({ type: "SET_FIELD", field: "prefix", value: v })}
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
          onChange={(e) => dispatch({ type: "SET_FIELD", field: "quakeMode", value: e.target.checked })}
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
          onChange={(v) => dispatch({ type: "SET_FIELD", field: "globalHotkey", value: v })}
          placeholder="Ctrl+Shift+F12"
          disabled={!s.quakeMode}
          ariaLabel="Global hotkey shortcut"
        />
        <span className="settings-desc">
          Quakeモードのトグルキー（Quakeモード有効時のみ使用）（デフォルト: Ctrl+Shift+F12）
        </span>
      </div>
    </div>
  );
}
