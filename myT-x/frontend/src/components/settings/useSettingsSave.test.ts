import {describe, expect, it} from "vitest";
import {INITIAL_FORM} from "./settingsReducer";
import {buildSettingsSavePayload} from "./useSettingsSave";

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
});
