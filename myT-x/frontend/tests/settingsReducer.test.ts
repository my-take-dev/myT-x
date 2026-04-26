import {describe, expect, it} from "vitest";
import {INITIAL_FORM, formReducer} from "../src/components/settings/settingsReducer";

describe("settingsReducer INITIAL_FORM guard", () => {
    it("keeps expected top-level form keys", () => {
        const keys = Object.keys(INITIAL_FORM).sort();
        expect(keys).toEqual([
            "activeCategory",
            "agentFrom",
            "agentTo",
            "allowedShells",
            "autoStart",
            "chatOverlayPercentage",
            "claudeEnvDefaultEnabled",
            "claudeEnvEntries",
            "defaultSessionDir",
            "effortLevel",
            "error",
            "globalHotkey",
            "keys",
            "loadFailed",
            "loading",
            "mcpServers",
            "mcpServersLoaded",
            "minOverrideNameLen",
            "overrides",
            "paneEnvDefaultEnabled",
            "paneEnvEntries",
            "prefix",
            "quakeMode",
            "saving",
            "shell",
            "taskScheduler",
            "validationErrors",
            "viewerSidebarMode",
            "viewerShortcuts",
            "websocketPort",
            "wtCopyDirs",
            "wtCopyFiles",
            "wtEnabled",
            "wtForceCleanup",
            "wtSetupScripts",
            "wtSetupScriptTimeoutSeconds",
        ].sort());
    });

    it("RESET_FOR_LOAD preserves schema and sets loading", () => {
        const reset = formReducer(INITIAL_FORM, {type: "RESET_FOR_LOAD"});
        expect(Object.keys(reset).sort()).toEqual(Object.keys(INITIAL_FORM).sort());
        expect(reset.loading).toBe(true);
    });

    it("defaults viewerSidebarMode to overlay", () => {
        expect(INITIAL_FORM.viewerSidebarMode).toBe("overlay");
    });

    it("defaults chatOverlayPercentage to the docked panel baseline", () => {
        expect(INITIAL_FORM.chatOverlayPercentage).toBe(40);
    });

    it("LOAD_CONFIG maps viewer_sidebar_mode and falls back to overlay", () => {
        const baseConfig = {
            shell: "powershell.exe",
            prefix: "Ctrl+b",
            keys: {},
            quake_mode: true,
            global_hotkey: "Ctrl+Shift+F12",
            worktree: {
                enabled: true,
                force_cleanup: false,
                setup_scripts: [],
                setup_script_timeout_seconds: 300,
                copy_files: [],
                copy_dirs: [],
            },
        } as any;

        const docked = formReducer(INITIAL_FORM, {
            type: "LOAD_CONFIG",
            config: {...baseConfig, viewer_sidebar_mode: "docked"},
            shells: [],
        });
        expect(docked.viewerSidebarMode).toBe("docked");

        const fallback = formReducer(INITIAL_FORM, {
            type: "LOAD_CONFIG",
            config: baseConfig,
            shells: [],
        });
        expect(fallback.viewerSidebarMode).toBe("overlay");
        expect(fallback.chatOverlayPercentage).toBe(40);
    });

    it("LOAD_CONFIG preserves task_scheduler for later full-config saves", () => {
        const loaded = formReducer(INITIAL_FORM, {
            type: "LOAD_CONFIG",
            config: {
                shell: "powershell.exe",
                prefix: "Ctrl+b",
                keys: {},
                quake_mode: true,
                global_hotkey: "Ctrl+Shift+F12",
                auto_start: [
                    {
                        name: "Mini Codex",
                        command: "codex",
                        args: "--model gpt-5.4-mini",
                    },
                ],
                worktree: {
                    enabled: true,
                    force_cleanup: false,
                    setup_scripts: [],
                    setup_script_timeout_seconds: 300,
                    copy_files: [],
                    copy_dirs: [],
                },
                task_scheduler: {
                    pre_exec_reset_delay_s: 5,
                    pre_exec_idle_timeout_s: 45,
                    pre_exec_target_mode: "all_panes",
                    message_templates: [
                        {
                            name: "daily",
                            message: "Run daily checks",
                        },
                    ],
                },
            } as any,
            shells: [],
        });

        expect(loaded.taskScheduler).toEqual({
            pre_exec_reset_delay_s: 5,
            pre_exec_idle_timeout_s: 45,
            pre_exec_target_mode: "all_panes",
            message_templates: [
                {
                    name: "daily",
                    message: "Run daily checks",
                },
            ],
        });
        expect(loaded.autoStart).toHaveLength(1);
        expect(loaded.autoStart[0]).toMatchObject({
            name: "Mini Codex",
            command: "codex",
            args: "--model gpt-5.4-mini",
        });
    });

    it("LOAD_CONFIG preserves mcp_servers and websocket_port for later full-config saves", () => {
        const loaded = formReducer(INITIAL_FORM, {
            type: "LOAD_CONFIG",
            config: {
                shell: "powershell.exe",
                prefix: "Ctrl+b",
                keys: {},
                quake_mode: true,
                global_hotkey: "Ctrl+Shift+F12",
                websocket_port: 43210,
                mcp_servers: [
                    {
                        id: "custom",
                        name: "Custom MCP",
                        description: "custom server",
                        kind: "custom",
                        command: "custom-mcp",
                        args: ["serve"],
                        env: {API_KEY: "secret"},
                        enabled: true,
                        usage_sample: "custom-mcp serve",
                        config_params: [
                            {
                                key: "endpoint",
                                label: "Endpoint",
                                default_value: "http://localhost:9000",
                                description: "Base URL",
                            },
                        ],
                    },
                ],
                worktree: {
                    enabled: true,
                    force_cleanup: false,
                    setup_scripts: [],
                    setup_script_timeout_seconds: 300,
                    copy_files: [],
                    copy_dirs: [],
                },
            } as any,
            shells: [],
        });

        expect(loaded.websocketPort).toBe(43210);
        expect(loaded.mcpServersLoaded).toBe(true);
        expect(loaded.mcpServers).toEqual([
            {
                id: "custom",
                name: "Custom MCP",
                description: "custom server",
                kind: "custom",
                command: "custom-mcp",
                args: ["serve"],
                env: {API_KEY: "secret"},
                enabled: true,
                usage_sample: "custom-mcp serve",
                config_params: [
                    {
                        key: "endpoint",
                        label: "Endpoint",
                        default_value: "http://localhost:9000",
                        description: "Base URL",
                    },
                ],
            },
        ]);
    });

    it("LOAD_CONFIG falls back to overlay for invalid viewer_sidebar_mode values", () => {
        const baseConfig = {
            shell: "powershell.exe",
            prefix: "Ctrl+b",
            keys: {},
            quake_mode: true,
            global_hotkey: "Ctrl+Shift+F12",
            worktree: {
                enabled: true,
                force_cleanup: false,
                setup_scripts: [],
                setup_script_timeout_seconds: 300,
                copy_files: [],
                copy_dirs: [],
            },
        } as any;

        const custom = formReducer(INITIAL_FORM, {
            type: "LOAD_CONFIG",
            config: {...baseConfig, viewer_sidebar_mode: "stacked"},
            shells: [],
        });

        expect(custom.viewerSidebarMode).toBe("overlay");
    });

    it("LOAD_CONFIG migrates legacy file-tree shortcuts into file-view", () => {
        const loaded = formReducer(INITIAL_FORM, {
            type: "LOAD_CONFIG",
            config: {
                shell: "powershell.exe",
                prefix: "Ctrl+b",
                keys: {},
                quake_mode: true,
                global_hotkey: "Ctrl+Shift+F12",
                viewer_shortcuts: {
                    "file-tree": "Ctrl+Shift+1",
                },
                worktree: {
                    enabled: true,
                    force_cleanup: false,
                    setup_scripts: [],
                    setup_script_timeout_seconds: 300,
                    copy_files: [],
                    copy_dirs: [],
                },
            } as any,
            shells: [],
        });

        expect(loaded.viewerShortcuts).toEqual({
            "file-view": "Ctrl+Shift+1",
        });
    });
});
