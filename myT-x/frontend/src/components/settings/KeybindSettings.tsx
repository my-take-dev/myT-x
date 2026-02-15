import type { FormDispatch, FormState } from "./types";
import { ShortcutInput } from "./ShortcutInput";
import type { KnownKeyBinding } from "../../types/tmux";

const KEY_BINDINGS: { key: KnownKeyBinding; label: string; defaultVal: string }[] = [
  { key: "split-vertical", label: "垂直分割", defaultVal: "%" },
  { key: "split-horizontal", label: "水平分割", defaultVal: '"' },
  { key: "toggle-zoom", label: "ズーム切替", defaultVal: "z" },
  { key: "kill-pane", label: "ペイン閉じる", defaultVal: "x" },
  { key: "detach-session", label: "デタッチ", defaultVal: "d" },
];

interface KeybindSettingsProps {
  s: FormState;
  dispatch: FormDispatch;
}

export function KeybindSettings({ s, dispatch }: KeybindSettingsProps) {
  return (
    <div className="settings-section">
      <div className="settings-section-title">キーバインド</div>
      <span className="settings-desc" style={{ marginBottom: 8, display: "block" }}>
        プレフィックスキーに続けて入力するアクションキー
      </span>

      {KEY_BINDINGS.map((kb) => (
        <div className="form-group" key={kb.key}>
          <label className="shortcut-label">{kb.label}</label>
          <ShortcutInput
            value={s.keys[kb.key] || ""}
            onChange={(v) => dispatch({ type: "UPDATE_KEY", key: kb.key, value: v })}
            placeholder={kb.defaultVal}
            ariaLabel={`${kb.key} shortcut`}
          />
        </div>
      ))}
    </div>
  );
}
