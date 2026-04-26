import {describe, expect, it} from "vitest";
import {INITIAL_FORM} from "../src/components/settings/settingsReducer";
import {buildSettingsSavePayload} from "../src/components/settings/useSettingsSave";

describe("buildSettingsSavePayload", () => {
    it("preserves task_scheduler during general settings saves", () => {
        const state = {
            ...INITIAL_FORM,
            mcpServersLoaded: true,
            taskScheduler: {
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
        };

        const payload = buildSettingsSavePayload(state);

        expect(payload.task_scheduler).toEqual({
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
        expect(payload.task_scheduler).not.toBe(state.taskScheduler);
        expect(payload.task_scheduler?.message_templates).not.toBe(state.taskScheduler.message_templates);
    });

    it("round-trips websocket_port and mcp_servers during full-overwrite saves", () => {
        const state = {
            ...INITIAL_FORM,
            websocketPort: 43210,
            wtSetupScriptTimeoutSeconds: 123,
            mcpServersLoaded: true,
            mcpServers: [
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
        };

        const payload = buildSettingsSavePayload(state);

        expect(payload.websocket_port).toBe(43210);
        expect(payload.worktree.setup_script_timeout_seconds).toBe(123);
        expect(payload.mcp_servers).toEqual(state.mcpServers);
        expect(payload.mcp_servers).not.toBe(state.mcpServers);
        expect(payload.mcp_servers?.[0]?.args).not.toBe(state.mcpServers[0].args);
        expect(payload.mcp_servers?.[0]?.env).not.toBe(state.mcpServers[0].env);
        expect(payload.mcp_servers?.[0]?.config_params).not.toBe(state.mcpServers[0].config_params);
    });

    it("sends an explicit empty mcp_servers array when the user removes every MCP server", () => {
        const payload = buildSettingsSavePayload({
            ...INITIAL_FORM,
            mcpServersLoaded: true,
            mcpServers: [],
        });

        expect(payload.mcp_servers).toEqual([]);
    });

    it("omits mcp_servers until the initial config load resolves", () => {
        const payload = buildSettingsSavePayload({
            ...INITIAL_FORM,
            mcpServers: [
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
                    config_params: [],
                },
            ],
        });

        expect(payload.mcp_servers).toBeUndefined();
    });

    it("serializes viewer docking fields during full-overwrite saves", () => {
        const state = {
            ...INITIAL_FORM,
            chatOverlayPercentage: 60,
            viewerSidebarMode: "docked" as const,
        };

        const payload = buildSettingsSavePayload(state);

        expect(payload.chat_overlay_percentage).toBe(60);
        expect(payload.viewer_sidebar_mode).toBe("docked");
    });

    it("keeps every top-level config field in the full-overwrite payload", () => {
        const payload = buildSettingsSavePayload({
            ...INITIAL_FORM,
            mcpServersLoaded: true,
        });

        expect(Object.keys(payload).sort()).toEqual([
            "agent_model",
            "auto_start",
            "chat_overlay_percentage",
            "claude_env",
            "default_session_dir",
            "global_hotkey",
            "keys",
            "mcp_servers",
            "pane_env",
            "pane_env_default_enabled",
            "prefix",
            "quake_mode",
            "shell",
            "task_scheduler",
            "viewer_shortcuts",
            "viewer_sidebar_mode",
            "websocket_port",
            "worktree",
        ]);
    });

    it("drops the legacy file-tree shortcut key during save payload normalization", () => {
        const payload = buildSettingsSavePayload({
            ...INITIAL_FORM,
            viewerShortcuts: {
                "file-tree": "Ctrl+Shift+1",
            },
        });

        expect(payload.viewer_shortcuts).toEqual({
            "file-view": "ctrl+shift+1",
        });
        expect(payload.viewer_shortcuts).not.toHaveProperty("file-tree");
    });

    it("prefers the current file-view shortcut over the legacy alias during save", () => {
        const payload = buildSettingsSavePayload({
            ...INITIAL_FORM,
            viewerShortcuts: {
                "file-view": "Ctrl+Shift+2",
                "file-tree": "Ctrl+Shift+1",
            },
        });

        expect(payload.viewer_shortcuts).toEqual({
            "file-view": "ctrl+shift+2",
        });
    });
});
