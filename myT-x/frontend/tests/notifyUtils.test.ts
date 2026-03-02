import {beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    addNotification: vi.fn(),
}));

vi.mock("../src/stores/notificationStore", () => ({
    useNotificationStore: {
        getState: () => ({
            addNotification: mocked.addNotification,
        }),
    },
}));

type NotifyUtilsModule = typeof import("../src/utils/notifyUtils");

let notifyClipboardFailure: NotifyUtilsModule["notifyClipboardFailure"];
let notifyHighlightFailure: NotifyUtilsModule["notifyHighlightFailure"];
let notifyLinkOpenFailure: NotifyUtilsModule["notifyLinkOpenFailure"];

describe("notifyUtils", () => {
    beforeEach(async () => {
        mocked.addNotification.mockReset();
        vi.useRealTimers();
        vi.resetModules();
        const notifyUtils = await import("../src/utils/notifyUtils");
        notifyClipboardFailure = notifyUtils.notifyClipboardFailure;
        notifyHighlightFailure = notifyUtils.notifyHighlightFailure;
        notifyLinkOpenFailure = notifyUtils.notifyLinkOpenFailure;
    });

    it("notifies clipboard failure", () => {
        notifyClipboardFailure();
        expect(mocked.addNotification).toHaveBeenCalledWith("Failed to copy to clipboard.", "warn");
    });

    it("notifies link open failure", () => {
        notifyLinkOpenFailure();
        expect(mocked.addNotification).toHaveBeenCalledWith("Failed to open link", "warn");
    });

    it("applies cooldown to repeated highlight failure notifications", () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-03-01T12:00:00Z"));

        notifyHighlightFailure();
        notifyHighlightFailure();
        expect(mocked.addNotification).toHaveBeenCalledTimes(1);

        vi.setSystemTime(new Date("2026-03-01T12:00:11Z"));
        notifyHighlightFailure();
        expect(mocked.addNotification).toHaveBeenCalledTimes(2);
    });

    it("sends the correct message and severity for notifyHighlightFailure", () => {
        notifyHighlightFailure();
        expect(mocked.addNotification).toHaveBeenCalledWith(
            expect.stringContaining("Syntax highlighting"),
            "warn"
        );
    });

    it("allows notification exactly at cooldown boundary (10000ms)", () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-03-01T12:00:00Z"));

        notifyHighlightFailure();
        vi.setSystemTime(new Date("2026-03-01T12:00:10Z"));
        notifyHighlightFailure();

        expect(mocked.addNotification).toHaveBeenCalledTimes(2);
    });

    it("does not notify again just before cooldown boundary (9999ms)", () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-03-01T12:00:00Z"));

        notifyHighlightFailure();
        vi.setSystemTime(new Date("2026-03-01T12:00:09.999Z"));
        notifyHighlightFailure();

        expect(mocked.addNotification).toHaveBeenCalledTimes(1);
    });
});
