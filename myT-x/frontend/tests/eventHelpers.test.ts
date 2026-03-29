import {describe, expect, it, vi} from "vitest";
import {cleanupEventListeners, notifyWarn, tr} from "../src/hooks/sync/eventHelpers";

// Mock dependencies
vi.mock("../../wailsjs/runtime/runtime", () => ({
    EventsOn: vi.fn(),
}));

vi.mock("../src/i18n", () => ({
    getLanguage: vi.fn(() => "en"),
    translate: vi.fn((_key: string, text: string) => text),
}));

const mockAddNotification = vi.fn();
vi.mock("../src/stores/notificationStore", () => ({
    useNotificationStore: {
        getState: () => ({addNotification: mockAddNotification}),
    },
}));

describe("eventHelpers", () => {
    describe("cleanupEventListeners", () => {
        it("calls all cleanup functions in reverse order", () => {
            const order: number[] = [];
            const fns = [
                () => order.push(1),
                () => order.push(2),
                () => order.push(3),
            ];
            cleanupEventListeners(fns);
            expect(order).toEqual([3, 2, 1]);
        });

        it("continues cleanup even if one function throws", () => {
            const order: number[] = [];
            const fns = [
                () => order.push(1),
                () => { throw new Error("fail"); },
                () => order.push(3),
            ];
            cleanupEventListeners(fns);
            expect(order).toEqual([3, 1]);
        });

        it("handles empty array without error", () => {
            expect(() => cleanupEventListeners([])).not.toThrow();
        });

        it("handles array with undefined entries gracefully", () => {
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const fns: Array<() => void> = [undefined as any, () => {}];
            expect(() => cleanupEventListeners(fns)).not.toThrow();
        });
    });

    describe("notifyWarn", () => {
        it("calls addNotification with warn level", () => {
            mockAddNotification.mockClear();
            notifyWarn("test message");
            expect(mockAddNotification).toHaveBeenCalledWith("test message", "warn");
        });
    });

    describe("tr", () => {
        it("returns translated text for current locale", () => {
            const result = tr("test.key", "日本語テキスト", "English text");
            // Mock returns en locale, translate returns text as-is
            expect(result).toBe("English text");
        });
    });
});
