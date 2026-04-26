import type {AutoStartEntry, ClaudeEnvEntry, FormAction, FormState, PaneEnvEntry} from "./types";
import {generateId} from "./types";
import {DEFAULT_SETUP_SCRIPT_TIMEOUT_SECONDS, EFFORT_LEVEL_KEY, MIN_OVERRIDE_NAME_LEN_FALLBACK} from "./constants";
import {normalizeViewerShortcutConfig} from "../viewer/viewerShortcutDefinitions";
import {normalizeViewerSidebarMode} from "../../utils/viewerSidebarMode";

export const INITIAL_FORM: FormState = {
    shell: "powershell.exe",
    prefix: "Ctrl+b",
    quakeMode: true,
    globalHotkey: "Ctrl+Shift+F12",
    autoStart: [],
    viewerSidebarMode: "overlay",
    keys: {},
    viewerShortcuts: {},
    defaultSessionDir: "",
    websocketPort: 0,
    wtEnabled: true,
    wtForceCleanup: false,
    wtSetupScripts: [],
    wtSetupScriptTimeoutSeconds: DEFAULT_SETUP_SCRIPT_TIMEOUT_SECONDS,
    wtCopyFiles: [],
    wtCopyDirs: [],
    mcpServers: [],
    mcpServersLoaded: false,
    agentFrom: "",
    agentTo: "",
    overrides: [],
    effortLevel: "",
    paneEnvEntries: [],
    paneEnvDefaultEnabled: false,
    claudeEnvDefaultEnabled: false,
    claudeEnvEntries: [],
    taskScheduler: undefined,
    chatOverlayPercentage: 40,
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
            return {...INITIAL_FORM, loading: true};
        case "SET_FIELD":
            return {...state, [action.field]: action.value};
        case "START_SAVE":
            return {...state, validationErrors: {}, error: "", saving: true};
        case "LOAD_CONFIG": {
            const {config: cfg, shells} = action;
            const wt = cfg.worktree;
            const am = cfg.agent_model;
            const pe = cfg.pane_env || {};
            const paneEnvEntries: PaneEnvEntry[] = Object.entries(pe)
                .filter(([k]) => k.toUpperCase() !== EFFORT_LEVEL_KEY)
                .map(([key, value]) => ({id: generateId(), key, value}));
            const effortKey = Object.keys(pe).find(k => k.toUpperCase() === EFFORT_LEVEL_KEY);
            const ce = cfg.claude_env;
            const autoStart: AutoStartEntry[] = (cfg.auto_start ?? []).map((entry) => ({
                id: generateId(),
                name: entry.name,
                command: entry.command,
                args: entry.args ?? "",
            }));
            const claudeEnvEntries: ClaudeEnvEntry[] = ce?.vars
                ? Object.entries(ce.vars).map(([key, value]) => ({id: generateId(), key, value}))
                : [];
            const taskScheduler = cfg.task_scheduler
                ? {
                    pre_exec_reset_delay_s: cfg.task_scheduler.pre_exec_reset_delay_s,
                    pre_exec_idle_timeout_s: cfg.task_scheduler.pre_exec_idle_timeout_s,
                    pre_exec_target_mode: cfg.task_scheduler.pre_exec_target_mode,
                    message_templates: cfg.task_scheduler.message_templates?.map((template) => ({
                        name: template.name,
                        message: template.message,
                    })),
                }
                : undefined;
            return {
                ...state,
                shell: cfg.shell || "powershell.exe",
                prefix: cfg.prefix || "Ctrl+b",
                quakeMode: cfg.quake_mode ?? true,
                globalHotkey: cfg.global_hotkey || "Ctrl+Shift+F12",
                autoStart,
                viewerSidebarMode: normalizeViewerSidebarMode(cfg.viewer_sidebar_mode),
                keys: cfg.keys || {},
                viewerShortcuts: normalizeViewerShortcutConfig(cfg.viewer_shortcuts),
                defaultSessionDir: cfg.default_session_dir || "",
                websocketPort: cfg.websocket_port ?? 0,
                wtEnabled: wt?.enabled ?? true,
                wtForceCleanup: wt?.force_cleanup ?? false,
                wtSetupScripts: wt?.setup_scripts || [],
                wtSetupScriptTimeoutSeconds:
                    typeof wt?.setup_script_timeout_seconds === "number" && wt.setup_script_timeout_seconds > 0
                        ? wt.setup_script_timeout_seconds
                        : DEFAULT_SETUP_SCRIPT_TIMEOUT_SECONDS,
                wtCopyFiles: wt?.copy_files || [],
                wtCopyDirs: wt?.copy_dirs || [],
                mcpServers: (cfg.mcp_servers ?? []).map((server) => ({
                    id: server.id,
                    name: server.name,
                    description: server.description,
                    kind: server.kind,
                    command: server.command,
                    args: server.args ? [...server.args] : undefined,
                    env: server.env ? {...server.env} : undefined,
                    enabled: server.enabled,
                    usage_sample: server.usage_sample,
                    config_params: server.config_params?.map((param) => ({
                        key: param.key,
                        label: param.label,
                        default_value: param.default_value,
                        description: param.description,
                    })),
                })),
                mcpServersLoaded: true,
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
                chatOverlayPercentage: cfg.chat_overlay_percentage ?? 40,
                claudeEnvDefaultEnabled: ce?.default_enabled ?? false,
                claudeEnvEntries,
                taskScheduler,
                allowedShells: shells || [],
                loading: false,
                loadFailed: false,
                error: "",
            };
        }
        case "UPDATE_KEY":
            return {...state, keys: {...state.keys, [action.key]: action.value}};
        case "SET_OVERRIDES":
            return {...state, overrides: action.overrides};
        case "UPDATE_OVERRIDE": {
            const next = [...state.overrides];
            if (action.index >= 0 && action.index < next.length) {
                next[action.index] = {...next[action.index], [action.field]: action.value};
            }
            return {...state, overrides: next};
        }
        case "SET_PANE_ENV_ENTRIES":
            return {...state, paneEnvEntries: action.entries};
        case "UPDATE_PANE_ENV_ENTRY": {
            const nextEntries = [...state.paneEnvEntries];
            if (action.index >= 0 && action.index < nextEntries.length) {
                nextEntries[action.index] = {...nextEntries[action.index], [action.field]: action.value};
            }
            return {...state, paneEnvEntries: nextEntries};
        }
        case "SET_CLAUDE_ENV_ENTRIES":
            return {...state, claudeEnvEntries: action.entries};
        case "UPDATE_CLAUDE_ENV_ENTRY": {
            const nextClaude = [...state.claudeEnvEntries];
            if (action.index >= 0 && action.index < nextClaude.length) {
                nextClaude[action.index] = {...nextClaude[action.index], [action.field]: action.value};
            }
            return {...state, claudeEnvEntries: nextClaude};
        }
        case "SET_AUTO_START_ENTRIES":
            return {...state, autoStart: action.entries};
        case "UPDATE_AUTO_START_ENTRY": {
            const nextEntries = [...state.autoStart];
            if (action.index >= 0 && action.index < nextEntries.length) {
                nextEntries[action.index] = {...nextEntries[action.index], [action.field]: action.value};
            }
            return {...state, autoStart: nextEntries};
        }
        case "UPDATE_VIEWER_SHORTCUT":
            return {
                ...state,
                viewerShortcuts: {...state.viewerShortcuts, [action.viewId]: action.value},
            };
        default: {
            const _exhaustive: never = action;
            void _exhaustive;
            return state;
        }
    }
}
