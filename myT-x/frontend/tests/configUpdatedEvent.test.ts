import {describe, expect, it} from "vitest";
import {parseConfigUpdatedPayload} from "../src/hooks/sync/configUpdatedEvent";

const baseConfig = {
    shell: "pwsh",
    prefix: "C-b",
    quake_mode: false,
    global_hotkey: "",
    keys: {},
    worktree: {
        enabled: false,
        force_cleanup: false,
        setup_script_timeout_seconds: 300,
    },
};

describe("parseConfigUpdatedPayload", () => {
    it("parses the nested config payload shape", () => {
        const parsed = parseConfigUpdatedPayload({
            version: 7,
            updated_at_unix_milli: 1234,
            config: baseConfig,
        });

        expect(parsed).toEqual({
            config: baseConfig,
            version: 7,
            updated_at_unix_milli: 1234,
        });
    });

    it("accepts the flat legacy payload shape with synthetic version fallback", () => {
        const parsed = parseConfigUpdatedPayload({
            ...baseConfig,
            updated_at_unix_milli: 4321,
        });

        expect(parsed).toEqual({
            config: baseConfig,
            version: null,
            updated_at_unix_milli: 4321,
        });
    });

    it("rejects malformed config payloads", () => {
        expect(parseConfigUpdatedPayload({
            version: 7,
            config: {
                ...baseConfig,
                shell: "",
            },
        })).toBeNull();

        expect(parseConfigUpdatedPayload({
            version: 7,
            config: {
                ...baseConfig,
                worktree: {
                    ...baseConfig.worktree,
                    setup_script_timeout_seconds: "300",
                },
            },
        })).toBeNull();
    });
});
