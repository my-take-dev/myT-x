import {describe, expect, it} from "vitest";
import {isImeTransitionalEvent} from "../src/utils/ime";
import {shouldLetXtermHandleImeEvent} from "../src/utils/terminalIme";

function keyboardEvent(init: KeyboardEventInit & { keyCode?: number } = {}): KeyboardEvent {
    const event = new KeyboardEvent("keydown", init);
    if (typeof init.keyCode === "number") {
        Object.defineProperty(event, "keyCode", {value: init.keyCode});
    }
    return event;
}

describe("isImeTransitionalEvent", () => {
    it("returns true for composing events", () => {
        const event = keyboardEvent({isComposing: true});
        expect(isImeTransitionalEvent(event)).toBe(true);
    });

    it("returns true for Process key events", () => {
        const event = keyboardEvent({key: "Process"});
        expect(isImeTransitionalEvent(event)).toBe(true);
    });

    it("returns true for keyCode=229 fallback", () => {
        const event = keyboardEvent({key: "Unidentified", keyCode: 229});
        expect(isImeTransitionalEvent(event)).toBe(true);
    });

    it("returns false for regular keys", () => {
        const event = keyboardEvent({key: "a"});
        expect(isImeTransitionalEvent(event)).toBe(false);
    });
});

describe("shouldLetXtermHandleImeEvent", () => {
    it("returns true while composition is active", () => {
        const event = keyboardEvent({key: "a"});
        expect(shouldLetXtermHandleImeEvent(event, true)).toBe(true);
    });

    it("returns true for transitional IME key events", () => {
        const event = keyboardEvent({key: "Process"});
        expect(shouldLetXtermHandleImeEvent(event, false)).toBe(true);
    });

    it("returns true for keyCode=229 transitional IME events", () => {
        const event = keyboardEvent({key: "Unidentified", keyCode: 229});
        expect(shouldLetXtermHandleImeEvent(event, false)).toBe(true);
    });

    it("returns false for regular key events outside composition", () => {
        const event = keyboardEvent({key: "a"});
        expect(shouldLetXtermHandleImeEvent(event, false)).toBe(false);
    });
});
