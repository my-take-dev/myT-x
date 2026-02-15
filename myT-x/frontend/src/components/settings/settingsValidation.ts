import type { OverrideEntry, PaneEnvEntry } from "./types";
import { EFFORT_LEVEL_KEY, MIN_OVERRIDE_NAME_LEN_FALLBACK, VALID_EFFORT_LEVELS } from "./constants";

/** Environment variable names that must not be overridden (mirrors backend blockedEnvironmentKeys). */
const BLOCKED_ENV_KEYS = new Set([
  "PATH", "PATHEXT", "COMSPEC", "SYSTEMROOT", "WINDIR",
  "SYSTEMDRIVE", "APPDATA", "LOCALAPPDATA", "PSMODULEPATH",
  "TEMP", "TMP", "USERPROFILE",
]);

/** POSIX準拠の環境変数名パターン: 英字またはアンダースコアで始まり、英数字とアンダースコアのみ */
const ENV_VAR_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;

export function validateAgentModelSettings(
  agentFrom: string,
  agentTo: string,
  overrides: OverrideEntry[],
  minOverrideNameLen = MIN_OVERRIDE_NAME_LEN_FALLBACK,
): Record<string, string> {
  const errors: Record<string, string> = {};

  const from = agentFrom.trim();
  const to = agentTo.trim();
  if ((from && !to) || (!from && to)) {
    errors["agent_model"] = "from\u3068to\u306f\u4e21\u65b9\u540c\u6642\u306b\u6307\u5b9a\u304c\u5fc5\u8981\u3067\u3059";
  }

  overrides.forEach((ov, i) => {
    const name = ov.name.trim();
    const model = ov.model.trim();
    if (!name && !model) {
      return;
    }
    if (!name && model) {
      errors[`override_name_${i}`] = "\u30e2\u30c7\u30eb\u304c\u6307\u5b9a\u3055\u308c\u3066\u3044\u308b\u5834\u5408\u3001\u540d\u524d\u306f\u5fc5\u9808\u3067\u3059";
      return;
    }
    const runeLen = [...name].length;
    if (runeLen < minOverrideNameLen) {
      errors[`override_name_${i}`] =
        `\u540d\u524d\u306f${minOverrideNameLen}\u6587\u5b57\u4ee5\u4e0a\u5fc5\u8981\u3067\u3059 (\u73fe\u5728: ${runeLen}\u6587\u5b57)`;
    }
    if (name && !model) {
      errors[`override_model_${i}`] = "\u30e2\u30c7\u30eb\u306f\u5fc5\u9808\u3067\u3059";
    }
  });

  return errors;
}

export function validatePaneEnvSettings(
  entries: PaneEnvEntry[],
  effortLevel: string,
): Record<string, string> {
  const errors: Record<string, string> = {};

  if (!VALID_EFFORT_LEVELS.has(effortLevel.trim().toLowerCase())) {
    errors["pane_env_effort"] = "low, medium, high のいずれかを指定してください";
  }

  const seenKeys = new Set<string>();

  entries.forEach((entry, i) => {
    const key = entry.key.trim();
    const value = entry.value.trim();
    if (!key && !value) {
      return;
    }
    if (!key && value) {
      errors[`pane_env_key_${i}`] = "変数名は必須です";
      return;
    }
    if (!ENV_VAR_NAME_PATTERN.test(key)) {
      errors[`pane_env_key_${i}`] = "変数名は英字・アンダースコアで始まり、英数字・アンダースコアのみ使用できます";
      return;
    }
    const upper = key.toUpperCase();
    if (key && !value) {
      errors[`pane_env_val_${i}`] = "値は必須です";
      return;
    }
    if (upper === EFFORT_LEVEL_KEY) {
      errors[`pane_env_key_${i}`] = "この変数は上部の専用フィールドで設定してください";
      return;
    }
    if (BLOCKED_ENV_KEYS.has(upper)) {
      errors[`pane_env_key_${i}`] = "システム変数は設定できません";
      return;
    }
    if (seenKeys.has(upper)) {
      errors[`pane_env_key_${i}`] = "変数名が重複しています";
    }
    seenKeys.add(upper);
  });

  return errors;
}
