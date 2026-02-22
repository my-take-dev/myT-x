import type { ClaudeEnvEntry, FormAction, FormState, PaneEnvEntry } from "./types";
import { generateId } from "./types";
import { EFFORT_LEVEL_KEY, MIN_OVERRIDE_NAME_LEN_FALLBACK } from "./constants";

export const INITIAL_FORM: FormState = {
  shell: "powershell.exe",
  prefix: "Ctrl+b",
  quakeMode: true,
  globalHotkey: "Ctrl+Shift+F12",
  keys: {},
  wtEnabled: true,
  wtForceCleanup: false,
  wtSetupScripts: [],
  wtCopyFiles: [],
  wtCopyDirs: [],
  agentFrom: "",
  agentTo: "",
  overrides: [],
  effortLevel: "",
  paneEnvEntries: [],
  paneEnvDefaultEnabled: false,
  claudeEnvDefaultEnabled: false,
  claudeEnvEntries: [],
  minOverrideNameLen: MIN_OVERRIDE_NAME_LEN_FALLBACK,
  allowedShells: [],
  loading: false,
  loadFailed: false,
  saving: false,
  error: "",
  validationErrors: {},
  activeCategory: "general",
};

export function formReducer(state: FormState, action: FormAction): FormState {
  switch (action.type) {
    case "RESET_FOR_LOAD":
      return { ...INITIAL_FORM, loading: true };
    case "SET_FIELD":
      return { ...state, [action.field]: action.value };
    case "START_SAVE":
      return { ...state, validationErrors: {}, error: "", saving: true };
    case "LOAD_CONFIG": {
      const { config: cfg, shells } = action;
      const wt = cfg.worktree;
      const am = cfg.agent_model;
      const pe = cfg.pane_env || {};
      const paneEnvEntries: PaneEnvEntry[] = Object.entries(pe)
        .filter(([k]) => k.toUpperCase() !== EFFORT_LEVEL_KEY)
        .map(([key, value]) => ({ id: generateId(), key, value }));
      const effortKey = Object.keys(pe).find(k => k.toUpperCase() === EFFORT_LEVEL_KEY);
      const ce = cfg.claude_env;
      const claudeEnvEntries: ClaudeEnvEntry[] = ce?.vars
        ? Object.entries(ce.vars).map(([key, value]) => ({ id: generateId(), key, value }))
        : [];
      return {
        ...state,
        shell: cfg.shell || "powershell.exe",
        prefix: cfg.prefix || "Ctrl+b",
        quakeMode: cfg.quake_mode ?? true,
        globalHotkey: cfg.global_hotkey || "Ctrl+Shift+F12",
        keys: cfg.keys || {},
        wtEnabled: wt?.enabled ?? true,
        wtForceCleanup: wt?.force_cleanup ?? false,
        wtSetupScripts: wt?.setup_scripts || [],
        wtCopyFiles: wt?.copy_files || [],
        wtCopyDirs: wt?.copy_dirs || [],
        agentFrom: am?.from || "",
        agentTo: am?.to || "",
        overrides: (am?.overrides || []).map((o) => ({
          id: generateId(),
          name: o.name,
          model: o.model,
        })),
        effortLevel: effortKey ? pe[effortKey] || "" : "",
        paneEnvEntries,
        paneEnvDefaultEnabled: cfg.pane_env_default_enabled ?? false,
        claudeEnvDefaultEnabled: ce?.default_enabled ?? false,
        claudeEnvEntries,
        allowedShells: shells || [],
        loading: false,
        loadFailed: false,
        error: "",
      };
    }
    case "UPDATE_KEY":
      return { ...state, keys: { ...state.keys, [action.key]: action.value } };
    case "SET_OVERRIDES":
      return { ...state, overrides: action.overrides };
    case "UPDATE_OVERRIDE": {
      const next = [...state.overrides];
      if (action.index >= 0 && action.index < next.length) {
        next[action.index] = { ...next[action.index], [action.field]: action.value };
      }
      return { ...state, overrides: next };
    }
    case "SET_PANE_ENV_ENTRIES":
      return { ...state, paneEnvEntries: action.entries };
    case "UPDATE_PANE_ENV_ENTRY": {
      const nextEntries = [...state.paneEnvEntries];
      if (action.index >= 0 && action.index < nextEntries.length) {
        nextEntries[action.index] = { ...nextEntries[action.index], [action.field]: action.value };
      }
      return { ...state, paneEnvEntries: nextEntries };
    }
    case "SET_CLAUDE_ENV_ENTRIES":
      return { ...state, claudeEnvEntries: action.entries };
    case "UPDATE_CLAUDE_ENV_ENTRY": {
      const nextClaude = [...state.claudeEnvEntries];
      if (action.index >= 0 && action.index < nextClaude.length) {
        nextClaude[action.index] = { ...nextClaude[action.index], [action.field]: action.value };
      }
      return { ...state, claudeEnvEntries: nextClaude };
    }
    default: {
      const _exhaustive: never = action;
      void _exhaustive;
      return state;
    }
  }
}
