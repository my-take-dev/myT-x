import {describe, expect, it, vi} from "vitest";
import {normalizeTerminalPasteText, pasteTextSafely} from "../src/utils/terminalPaste";

describe("normalizeTerminalPasteText", () => {
    it("keeps plain single-line text unchanged", () => {
        expect(normalizeTerminalPasteText("hello")).toBe("hello");
    });

    it("strips one trailing newline", () => {
        expect(normalizeTerminalPasteText("hello\n")).toBe("hello");
    });

    it("strips one trailing carriage return", () => {
        expect(normalizeTerminalPasteText("hello\r")).toBe("hello");
    });

    it("strips one trailing CRLF pair", () => {
        expect(normalizeTerminalPasteText("hello\r\n")).toBe("hello");
    });

    it("preserves internal newlines while stripping the trailing one", () => {
        expect(normalizeTerminalPasteText("line1\nline2\n")).toBe("line1\nline2");
    });

    it("removes only one trailing line ending under safe mode", () => {
        expect(normalizeTerminalPasteText("line1\n\n")).toBe("line1\n");
    });
});

describe("pasteTextSafely", () => {
    it("pastes normalized text into the target", () => {
        const target = {paste: vi.fn()};

        const pasted = pasteTextSafely(target, "hello\n");

        expect(pasted).toBe(true);
        expect(target.paste).toHaveBeenCalledWith("hello");
    });

    it("does not paste when the clipboard contains only a newline", () => {
        const target = {paste: vi.fn()};

        const pasted = pasteTextSafely(target, "\n");

        expect(pasted).toBe(false);
        expect(target.paste).not.toHaveBeenCalled();
    });

    it("does not paste an empty clipboard payload", () => {
        const target = {paste: vi.fn()};

        const pasted = pasteTextSafely(target, "");

        expect(pasted).toBe(false);
        expect(target.paste).not.toHaveBeenCalled();
    });

    it("returns false when target.paste() throws", () => {
        const target = {
            paste: vi.fn(() => {
                throw new Error("terminal disposed");
            }),
        };

        const pasted = pasteTextSafely(target, "hello");

        expect(pasted).toBe(false);
        expect(target.paste).toHaveBeenCalledWith("hello");
    });
});
