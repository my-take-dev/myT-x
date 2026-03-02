import {describe, expect, it} from "vitest";
import {isImeTransitionalEvent} from "../src/utils/ime";

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
