import {beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    logFrontendEvent: vi.fn(),
}));

vi.mock("../src/api", () => ({
    api: {
        LogFrontendEvent: (...args: [string, string, string]) => mocked.logFrontendEvent(...args),
    },
}));

import {api} from "../src/api";
import {logFrontendEventSafe} from "../src/utils/logFrontendEventSafe";

describe("logFrontendEventSafe", () => {
    beforeEach(() => {
        mocked.logFrontendEvent.mockReset();
    });

    it("calls LogFrontendEvent when binding is available", () => {
        mocked.logFrontendEvent.mockReturnValue(Promise.resolve());

        logFrontendEventSafe("error", "boom", "frontend/test");

        expect(mocked.logFrontendEvent).toHaveBeenCalledTimes(1);
        expect(mocked.logFrontendEvent).toHaveBeenCalledWith("error", "boom", "frontend/test");
    });

    it("does not throw on rejected promise", async () => {
        mocked.logFrontendEvent.mockReturnValue(Promise.reject(new Error("async failure")));

        expect(() => logFrontendEventSafe("warn", "msg", "frontend/test")).not.toThrow();

        await Promise.resolve();
    });

    it("does not throw on synchronous throw", () => {
        mocked.logFrontendEvent.mockImplementation(() => {
            throw new Error("sync failure");
        });

        expect(() => logFrontendEventSafe("error", "msg", "frontend/test")).not.toThrow();
    });

    it("returns early when binding is missing", () => {
        const mockedAPI = api as { LogFrontendEvent?: (level: string, msg: string, source: string) => Promise<void> };
        const original = mockedAPI.LogFrontendEvent;
        mockedAPI.LogFrontendEvent = undefined;
        try {
            expect(() => logFrontendEventSafe("error", "msg", "frontend/test")).not.toThrow();
            expect(mocked.logFrontendEvent).not.toHaveBeenCalled();
        } finally {
            mockedAPI.LogFrontendEvent = original;
        }
    });
});
