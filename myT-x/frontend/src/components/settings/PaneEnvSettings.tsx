import type { FormDispatch, FormState } from "./types";
import { generateId } from "./types";

interface PaneEnvSettingsProps {
  s: FormState;
  dispatch: FormDispatch;
}

export function PaneEnvSettings({ s, dispatch }: PaneEnvSettingsProps) {
  return (
    <div className="settings-section">
      <div className="settings-section-title">環境変数</div>
      <span className="settings-desc" style={{ marginBottom: 8, display: "block" }}>
        ペイン生成時に自動で埋め込む環境変数を設定します。
        コマンドの -e フラグで同じキーを指定した場合、-e の値が優先されます。
      </span>

      <div className="form-group">
        <label className="form-label">思考Level (CLAUDE_CODE_EFFORT_LEVEL)</label>
        <select
          className={`form-input ${s.validationErrors["pane_env_effort"] ? "input-error" : ""}`}
          value={s.effortLevel}
          onChange={(e) => dispatch({ type: "SET_FIELD", field: "effortLevel", value: e.target.value })}
        >
          <option value="">未設定</option>
          <option value="low">low</option>
          <option value="medium">medium</option>
          <option value="high">high</option>
        </select>
        {s.validationErrors["pane_env_effort"] && (
          <span className="form-error">{s.validationErrors["pane_env_effort"]}</span>
        )}
        <span className="settings-desc">
          CLAUDE_CODE_EFFORT_LEVEL 環境変数として設定されます
        </span>
      </div>

      <div className="form-group" style={{ marginTop: 8 }}>
        <label className="form-label">カスタム環境変数</label>
        <div className="settings-note">
          追加の環境変数を設定します。システム変数(PATH等)は上書きできません。
        </div>

        <div className="dynamic-list">
          {s.paneEnvEntries.map((entry, index) => (
            <div key={entry.id} className="override-row">
              <div className="override-fields">
                <div className="form-group">
                  <input
                    className={`form-input ${s.validationErrors[`pane_env_key_${index}`] ? "input-error" : ""}`}
                    value={entry.key}
                    onChange={(e) => dispatch({ type: "UPDATE_PANE_ENV_ENTRY", index, field: "key", value: e.target.value })}
                    placeholder="変数名"
                    aria-label={`環境変数名 ${index + 1}`}
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
                    onChange={(e) => dispatch({ type: "UPDATE_PANE_ENV_ENTRY", index, field: "value", value: e.target.value })}
                    placeholder="値"
                    aria-label={`環境変数値 ${index + 1}`}
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
                  dispatch({ type: "SET_PANE_ENV_ENTRIES", entries: s.paneEnvEntries.filter((_, i) => i !== index) })
                }
                title="削除"
                aria-label={`環境変数 ${entry.key || `項目${index + 1}`} を削除`}
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
                entries: [...s.paneEnvEntries, { id: generateId(), key: "", value: "" }],
              })
            }
          >
            + 環境変数追加
          </button>
        </div>
      </div>
    </div>
  );
}
