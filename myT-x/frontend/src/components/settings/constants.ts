// Fallback value. Must match config.MinOverrideNameLen().
export const MIN_OVERRIDE_NAME_LEN_FALLBACK = 5;

export const EFFORT_LEVEL_KEY = "CLAUDE_CODE_EFFORT_LEVEL";

// "" (空文字列) は「未設定」を表す意図的な仕様値。UIのselectで「未設定」選択時にこの値となる。
export const VALID_EFFORT_LEVELS = new Set(["low", "medium", "high", ""]);
