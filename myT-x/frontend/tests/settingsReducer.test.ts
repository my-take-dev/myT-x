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
            "minOverrideNameLen",
            "overrides",
            "paneEnvDefaultEnabled",
            "paneEnvEntries",
            "prefix",
            "quakeMode",
            "saving",
            "shell",
            "validationErrors",
            "viewerSidebarMode",
            "viewerShortcuts",
            "wtCopyDirs",
            "wtCopyFiles",
            "wtEnabled",
            "wtForceCleanup",
            "wtSetupScripts",
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

        const overlay = formReducer(INITIAL_FORM, {
            type: "LOAD_CONFIG",
            config: baseConfig,
            shells: [],
        });
        expect(overlay.viewerSidebarMode).toBe("overlay");
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
});
