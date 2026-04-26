import {describe, expect, it} from "vitest";
import {INITIAL_FORM} from "./settingsReducer";
import {buildSettingsSavePayload, selectSettingsValidationCategory} from "./useSettingsSave";

describe("buildSettingsSavePayload", () => {
    it("serializes every task scheduler field for full-overwrite saves", () => {
        const payload = buildSettingsSavePayload({
            ...INITIAL_FORM,
            taskScheduler: {
                pre_exec_reset_delay_s: 12,
                pre_exec_idle_timeout_s: 45,
                pre_exec_target_mode: "all_panes",
                message_templates: [
                    {name: "prepare", message: "Prepare task"},
                    {name: "finish", message: "Finish task"},
                ],
            },
        });

        expect(payload.task_scheduler).toEqual({
            pre_exec_reset_delay_s: 12,
            pre_exec_idle_timeout_s: 45,
            pre_exec_target_mode: "all_panes",
            message_templates: [
                {name: "prepare", message: "Prepare task"},
                {name: "finish", message: "Finish task"},
            ],
        });
    });

    it("omits task scheduler when the settings modal has no task scheduler snapshot", () => {
        const payload = buildSettingsSavePayload({
            ...INITIAL_FORM,
            taskScheduler: undefined,
        });

        expect(payload.task_scheduler).toBeUndefined();
    });

    it("serializes AutoStart entries for full-overwrite saves", () => {
        const payload = buildSettingsSavePayload({
            ...INITIAL_FORM,
            autoStart: [
                {
                    id: "a",
                    name: " Mini Codex ",
                    command: " codex ",
                    args: " --model gpt-5.4-mini ",
                },
                {
                    id: "blank",
                    name: "",
                    command: "   ",
                    args: "",
                },
            ],
        });

        expect(payload.auto_start).toEqual([
            {
                name: "Mini Codex",
                command: "codex",
                args: "--model gpt-5.4-mini",
            },
        ]);
    });
});

describe("selectSettingsValidationCategory", () => {
    it("routes AutoStart validation errors to the AutoStart tab", () => {
        expect(selectSettingsValidationCategory({
            auto_start_command_0: "Command is required.",
        })).toBe("auto-start");
    });
});
