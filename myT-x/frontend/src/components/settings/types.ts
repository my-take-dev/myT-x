import type { Dispatch } from "react";
import type { AppConfigAgentModelOverride } from "../../types/tmux";

export type OverrideEntry = AppConfigAgentModelOverride & { id: string };

export type PaneEnvEntry = { id: string; key: string; value: string };

export type ClaudeEnvEntry = { id: string; key: string; value: string };

export type SettingsCategory = "general" | "keybinds" | "worktree" | "agent-model" | "claude-env" | "pane-env";

export interface FormState {
  shell: string;
  prefix: string;
  quakeMode: boolean;
  globalHotkey: string;
  keys: Record<string, string>;
  viewerShortcuts: Record<string, string>;
  defaultSessionDir: string;
  wtEnabled: boolean;
  wtForceCleanup: boolean;
  wtSetupScripts: string[];
  wtCopyFiles: string[];
  wtCopyDirs: string[];
  agentFrom: string;
  agentTo: string;
  overrides: OverrideEntry[];
  effortLevel: string;
  paneEnvEntries: PaneEnvEntry[];
  paneEnvDefaultEnabled: boolean;
  claudeEnvDefaultEnabled: boolean;
  claudeEnvEntries: ClaudeEnvEntry[];
  minOverrideNameLen: number;
  allowedShells: string[];
  loading: boolean;
  loadFailed: boolean;
  saving: boolean;
  error: string;
  validationErrors: Record<string, string>;
  activeCategory: SettingsCategory;
}

export type SetFieldAction = {
  [K in keyof FormState]: { type: "SET_FIELD"; field: K; value: FormState[K] };
}[keyof FormState];

export type FormAction =
  | { type: "RESET_FOR_LOAD" }
  | SetFieldAction
  | { type: "START_SAVE" }
  | { type: "LOAD_CONFIG"; config: import("../../types/tmux").AppConfig; shells: string[] }
  | { type: "UPDATE_KEY"; key: string; value: string }
  | { type: "SET_OVERRIDES"; overrides: OverrideEntry[] }
  | { type: "UPDATE_OVERRIDE"; index: number; field: "name" | "model"; value: string }
  | { type: "SET_PANE_ENV_ENTRIES"; entries: PaneEnvEntry[] }
  | { type: "UPDATE_PANE_ENV_ENTRY"; index: number; field: "key" | "value"; value: string }
  | { type: "SET_CLAUDE_ENV_ENTRIES"; entries: ClaudeEnvEntry[] }
  | { type: "UPDATE_CLAUDE_ENV_ENTRY"; index: number; field: "key" | "value"; value: string }
  | { type: "UPDATE_VIEWER_SHORTCUT"; viewId: string; value: string };

export type FormDispatch = Dispatch<FormAction>;

export function generateId(): string {
  // Wails targets modern WebView2 where crypto.randomUUID() is available.
  return crypto.randomUUID();
}
