import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    clipboardSetText: vi.fn(),
}));

vi.mock("../wailsjs/runtime/runtime", () => ({
    ClipboardSetText: (...args: [string]) => mocked.clipboardSetText(...args),
}));

import {writeClipboardText} from "../src/utils/clipboardUtils";

const originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");

function setBrowserClipboard(writeTextImpl?: (text: string) => Promise<void>) {
    if (!writeTextImpl) {
        Object.defineProperty(navigator, "clipboard", {
            value: undefined,
            configurable: true,
        });
        return;
    }
    Object.defineProperty(navigator, "clipboard", {
        value: {writeText: writeTextImpl},
        configurable: true,
    });
}

describe("writeClipboardText", () => {
    beforeEach(() => {
        mocked.clipboardSetText.mockReset();
    });

    afterEach(() => {
        vi.restoreAllMocks();
        if (originalClipboardDescriptor) {
            Object.defineProperty(navigator, "clipboard", originalClipboardDescriptor);
        }
    });

    it("uses Wails ClipboardSetText when available", async () => {
        mocked.clipboardSetText.mockResolvedValue(undefined);
        const browserWrite = vi.fn().mockResolvedValue(undefined);
        setBrowserClipboard(browserWrite);

        await expect(writeClipboardText("hello")).resolves.toBeUndefined();

        expect(mocked.clipboardSetText).toHaveBeenCalledWith("hello");
        expect(browserWrite).not.toHaveBeenCalled();
    });

    it("falls back to navigator.clipboard when Wails call fails", async () => {
        mocked.clipboardSetText.mockRejectedValue(new Error("wails down"));
        const browserWrite = vi.fn().mockResolvedValue(undefined);
        setBrowserClipboard(browserWrite);

        await expect(writeClipboardText("fallback")).resolves.toBeUndefined();

        expect(mocked.clipboardSetText).toHaveBeenCalledWith("fallback");
        expect(browserWrite).toHaveBeenCalledWith("fallback");
    });

    it("throws combined error when both Wails and browser clipboard fail", async () => {
        mocked.clipboardSetText.mockRejectedValue(new Error("wails down"));
        const browserWrite = vi.fn().mockRejectedValue(new Error("browser denied"));
        setBrowserClipboard(browserWrite);

        await expect(writeClipboardText("x")).rejects.toThrow("Clipboard write failed: Wails: wails down; Browser: browser denied");
    });

    it("throws Wails-only error when browser clipboard is unavailable", async () => {
        mocked.clipboardSetText.mockRejectedValue("wails missing");
        setBrowserClipboard(undefined);

        await expect(writeClipboardText("x")).rejects.toThrow("Clipboard write failed (Wails): wails missing");
    });
});
