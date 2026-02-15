import type { FormDispatch, FormState } from "./types";
import { generateId } from "./types";

interface AgentModelSettingsProps {
  s: FormState;
  dispatch: FormDispatch;
}

export function AgentModelSettings({ s, dispatch }: AgentModelSettingsProps) {
  return (
    <div className="settings-section">
      <div className="settings-section-title">Agent Model (モデル置換)</div>
      <span className="settings-desc" style={{ marginBottom: 8, display: "block" }}>
        Claude Codeが子エージェントを起動する際のモデル自動置換設定。
        子プロセスの --model フラグを置換元から置換先に変更します。
      </span>

      <div className="form-group">
        <label className="form-label">置換元 (from)</label>
        <input
          className={`form-input ${s.validationErrors["agent_model"] ? "input-error" : ""}`}
          value={s.agentFrom}
          onChange={(e) => dispatch({ type: "SET_FIELD", field: "agentFrom", value: e.target.value })}
          placeholder="claude-opus-4-6"
        />
      </div>

      <div className="form-group">
        <label className="form-label">置換先 (to)</label>
        <input
          className={`form-input ${s.validationErrors["agent_model"] ? "input-error" : ""}`}
          value={s.agentTo}
          onChange={(e) => dispatch({ type: "SET_FIELD", field: "agentTo", value: e.target.value })}
          placeholder="claude-sonnet-4-5-20250929"
        />
        {s.validationErrors["agent_model"] && (
          <span className="form-error">{s.validationErrors["agent_model"]}</span>
        )}
        <span className="settings-desc">
          fromとtoは両方同時に指定が必要です
        </span>
      </div>

      <div className="form-group" style={{ marginTop: 8 }}>
        <label className="form-label">オーバーライド</label>
        <div className="settings-note">
          上から順に評価されます。条件重複時は登録が先のルールが優先されます。
          エージェント名は --agent-name の部分一致（大文字小文字を区別しない）で検索されます。
        </div>

        <div className="dynamic-list">
          {s.overrides.map((ov, index) => (
            <div key={ov.id} className="override-row">
              <span className="override-priority">#{index + 1}</span>
              <div className="override-fields">
                <div className="form-group">
                  <input
                    className={`form-input ${s.validationErrors[`override_name_${index}`] ? "input-error" : ""}`}
                    value={ov.name}
                    onChange={(e) => dispatch({ type: "UPDATE_OVERRIDE", index, field: "name", value: e.target.value })}
                    placeholder={`エージェント名 (${s.minOverrideNameLen}文字以上)`}
                  />
                  {s.validationErrors[`override_name_${index}`] && (
                    <span className="form-error">
                      {s.validationErrors[`override_name_${index}`]}
                    </span>
                  )}
                </div>
                <div className="form-group">
                  <input
                    className={`form-input ${s.validationErrors[`override_model_${index}`] ? "input-error" : ""}`}
                    value={ov.model}
                    onChange={(e) => dispatch({ type: "UPDATE_OVERRIDE", index, field: "model", value: e.target.value })}
                    placeholder="モデル名"
                  />
                  {s.validationErrors[`override_model_${index}`] && (
                    <span className="form-error">
                      {s.validationErrors[`override_model_${index}`]}
                    </span>
                  )}
                </div>
              </div>
              <button
                type="button"
                className="dynamic-list-remove"
                onClick={() =>
                  dispatch({ type: "SET_OVERRIDES", overrides: s.overrides.filter((_, i) => i !== index) })
                }
                title="削除"
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
                type: "SET_OVERRIDES",
                overrides: [...s.overrides, { id: generateId(), name: "", model: "" }],
              })
            }
          >
            + オーバーライド追加
          </button>
        </div>
      </div>
    </div>
  );
}
