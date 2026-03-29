import {beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    addNotification: vi.fn(),
    logFrontendEventSafe: vi.fn(),
}));

vi.mock("../src/stores/notificationStore", () => ({
    useNotificationStore: {
        getState: () => ({
            addNotification: mocked.addNotification,
        }),
    },
}));

vi.mock("../src/utils/logFrontendEventSafe", () => ({
    logFrontendEventSafe: mocked.logFrontendEventSafe,
}));

type NotifyUtilsModule = typeof import("../src/utils/notifyUtils");

let notifyClipboardFailure: NotifyUtilsModule["notifyClipboardFailure"];
let notifyPasteFailure: NotifyUtilsModule["notifyPasteFailure"];
let notifyHighlightFailure: NotifyUtilsModule["notifyHighlightFailure"];
let notifyLinkOpenFailure: NotifyUtilsModule["notifyLinkOpenFailure"];
let notifyOperationFailure: NotifyUtilsModule["notifyOperationFailure"];
let notifyAndLog: NotifyUtilsModule["notifyAndLog"];
let createConsecutiveFailureCounter: NotifyUtilsModule["createConsecutiveFailureCounter"];

describe("notifyUtils", () => {
    beforeEach(async () => {
        mocked.addNotification.mockReset();
        mocked.logFrontendEventSafe.mockReset();
        vi.useRealTimers();
        vi.resetModules();
        const notifyUtils = await import("../src/utils/notifyUtils");
        notifyClipboardFailure = notifyUtils.notifyClipboardFailure;
        notifyPasteFailure = notifyUtils.notifyPasteFailure;
        notifyHighlightFailure = notifyUtils.notifyHighlightFailure;
        notifyLinkOpenFailure = notifyUtils.notifyLinkOpenFailure;
        notifyOperationFailure = notifyUtils.notifyOperationFailure;
        notifyAndLog = notifyUtils.notifyAndLog;
        createConsecutiveFailureCounter = notifyUtils.createConsecutiveFailureCounter;
    });

    it("notifies clipboard failure", () => {
        notifyClipboardFailure();
        expect(mocked.addNotification).toHaveBeenCalledWith("Failed to copy to clipboard.", "warn");
        expect(mocked.logFrontendEventSafe).toHaveBeenCalledWith("warn", "Failed to copy to clipboard", "Clipboard");
    });

    it("notifies link open failure", () => {
        notifyLinkOpenFailure();
        expect(mocked.addNotification).toHaveBeenCalledWith("Failed to open link", "warn");
        expect(mocked.logFrontendEventSafe).toHaveBeenCalledWith("warn", "Failed to open link", "LinkOpen");
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
        expect(mocked.logFrontendEventSafe).toHaveBeenCalledWith("warn", "Syntax highlighting failed", "Highlight");
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

    it("notifies paste failure with distinct message from clipboard write failure", () => {
        notifyPasteFailure();
        expect(mocked.addNotification).toHaveBeenCalledWith("Failed to paste from clipboard.", "warn");
        expect(mocked.logFrontendEventSafe).toHaveBeenCalledWith("warn", "Failed to paste from clipboard", "Clipboard");
    });

    describe("notifyOperationFailure", () => {
        it.each([
            {
                scenario: "without err",
                args: ["Split pane"] as const,
                expectedMsg: "Split pane failed",
                expectedLevel: "warn",
            },
            {
                scenario: "with Error instance",
                args: ["Kill pane", "warn", new Error("backend unreachable")] as const,
                expectedMsg: "backend unreachable",
                expectedLevel: "warn",
            },
            {
                scenario: "with string err",
                args: ["Rename pane", "warn", "session not found"] as const,
                expectedMsg: "session not found",
                expectedLevel: "warn",
            },
            {
                scenario: "with non-Error/non-string err falls back",
                args: ["Detach session", "error", 42] as const,
                expectedMsg: "Detach session failed",
                expectedLevel: "error",
            },
            {
                scenario: "with custom level",
                args: ["Save team", "error"] as const,
                expectedMsg: "Save team failed",
                expectedLevel: "error",
            },
        ])("$scenario", ({args, expectedMsg, expectedLevel}) => {
            notifyOperationFailure(...args);
            expect(mocked.addNotification).toHaveBeenCalledWith(expectedMsg, expectedLevel);
        });
    });

    describe("notifyAndLog", () => {
        it("sends toast notification and logs to error panel", () => {
            const err = new Error("connection lost");
            notifyAndLog("Push changes", "error", err, "DiffView");

            expect(mocked.addNotification).toHaveBeenCalledWith("connection lost", "error");
            expect(mocked.logFrontendEventSafe).toHaveBeenCalledWith(
                "error",
                "connection lost",
                "DiffView",
            );
        });

        it("uses fallback message for non-Error err", () => {
            notifyAndLog("Start team", "warn", null, "TeamCRUD");

            expect(mocked.addNotification).toHaveBeenCalledWith("Start team failed", "warn");
            expect(mocked.logFrontendEventSafe).toHaveBeenCalledWith(
                "warn",
                "Start team failed",
                "TeamCRUD",
            );
        });
    });

    describe("createConsecutiveFailureCounter", () => {
        it("does not fire callback before threshold is reached", () => {
            const counter = createConsecutiveFailureCounter(3);
            const cb = vi.fn();

            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).not.toHaveBeenCalled();
        });

        it("fires callback exactly at threshold", () => {
            const counter = createConsecutiveFailureCounter(3);
            const cb = vi.fn();

            counter.recordFailure(cb);
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);
        });

        it("resets count on recordSuccess and requires full threshold again", () => {
            const counter = createConsecutiveFailureCounter(3);
            const cb = vi.fn();

            counter.recordFailure(cb);
            counter.recordFailure(cb);
            counter.recordSuccess();
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).not.toHaveBeenCalled();

            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);
        });

        it("resets count after firing and fires again after another full threshold", () => {
            vi.useFakeTimers();
            vi.setSystemTime(new Date("2026-03-01T12:00:00Z"));

            const counter = createConsecutiveFailureCounter(2, 0);
            const cb = vi.fn();

            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);

            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(2);
        });

        it("applies cooldown after threshold notification", () => {
            vi.useFakeTimers();
            vi.setSystemTime(new Date("2026-03-01T12:00:00Z"));

            const counter = createConsecutiveFailureCounter(2, 30_000);
            const cb = vi.fn();

            // First threshold — fires
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);

            // Second threshold within cooldown — suppressed
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);

            // Advance past cooldown — fires again
            vi.setSystemTime(new Date("2026-03-01T12:00:31Z"));
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(2);
        });

        it("allows notification exactly at cooldown boundary", () => {
            vi.useFakeTimers();
            vi.setSystemTime(new Date("2026-03-01T12:00:00Z"));

            const counter = createConsecutiveFailureCounter(1, 10_000);
            const cb = vi.fn();

            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);

            vi.setSystemTime(new Date("2026-03-01T12:00:10Z"));
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(2);
        });

        it("suppresses notification just before cooldown boundary", () => {
            vi.useFakeTimers();
            vi.setSystemTime(new Date("2026-03-01T12:00:00Z"));

            const counter = createConsecutiveFailureCounter(1, 10_000);
            const cb = vi.fn();

            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);

            vi.setSystemTime(new Date("2026-03-01T12:00:09.999Z"));
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);
        });

        it("threshold of 1 fires on every failure (respecting cooldown)", () => {
            vi.useFakeTimers();
            vi.setSystemTime(new Date("2026-03-01T12:00:00Z"));

            const counter = createConsecutiveFailureCounter(1, 0);
            const cb = vi.fn();

            counter.recordFailure(cb);
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(3);
        });

        it("accepts different callbacks on each recordFailure call", () => {
            const counter = createConsecutiveFailureCounter(2);
            const cb1 = vi.fn();
            const cb2 = vi.fn();

            counter.recordFailure(cb1);
            counter.recordFailure(cb2);

            // The callback at threshold (2nd call) fires, not the 1st.
            expect(cb1).not.toHaveBeenCalled();
            expect(cb2).toHaveBeenCalledTimes(1);
        });

        it("recordSuccess is idempotent when count is already zero", () => {
            const counter = createConsecutiveFailureCounter(3);
            const cb = vi.fn();

            counter.recordSuccess();
            counter.recordSuccess();
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);
        });

        it("fires immediately after cooldown expires when failures accumulated during cooldown", () => {
            vi.useFakeTimers();
            vi.setSystemTime(new Date("2026-03-01T12:00:00Z"));

            const counter = createConsecutiveFailureCounter(2, 30_000);
            const cb = vi.fn();

            // First threshold — fires.
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);

            // Failures during cooldown — suppressed but count accumulates.
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(1);

            // Advance past cooldown — very next failure should fire immediately
            // because count is already above threshold.
            vi.setSystemTime(new Date("2026-03-01T12:00:31Z"));
            counter.recordFailure(cb);
            expect(cb).toHaveBeenCalledTimes(2);
        });
    });
});
