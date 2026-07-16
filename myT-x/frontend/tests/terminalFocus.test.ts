import {describe, expect, it} from "vitest";
import {findTextEntryAncestor, shouldRecoverTerminalFocus} from "../src/utils/terminalFocus";

describe("shouldRecoverTerminalFocus", () => {
    it("recovers focus when the active element is document.body", () => {
        const terminalElement = document.createElement("div");

        expect(
            shouldRecoverTerminalFocus("window-focus", document.body, terminalElement, null),
        ).toBe(true);
    });

    it("recovers focus when a non-protected control is focused after window focus", () => {
        const terminalElement = document.createElement("div");
        const button = document.createElement("button");

        document.body.append(button);

        expect(
            shouldRecoverTerminalFocus("window-focus", button, terminalElement, null),
        ).toBe(true);

        button.remove();
    });

    it("does not steal focus from an intentional control interaction during composition blur", () => {
        const terminalElement = document.createElement("div");
        const button = document.createElement("button");

        document.body.append(button);

        expect(
            shouldRecoverTerminalFocus("composition-blur", button, terminalElement, null),
        ).toBe(false);

        button.remove();
    });

    it("does not steal focus from editable elements", () => {
        const terminalElement = document.createElement("div");
        const input = document.createElement("input");

        document.body.append(input);

        expect(
            shouldRecoverTerminalFocus("window-focus", input, terminalElement, null),
        ).toBe(false);

        input.remove();
    });

    it("does not treat non-text inputs as text entry targets", () => {
        const checkbox = document.createElement("input");
        checkbox.type = "checkbox";
        const radio = document.createElement("input");
        radio.type = "radio";
        const text = document.createElement("input");
        text.type = "text";

        expect(findTextEntryAncestor(checkbox)).toBeNull();
        expect(findTextEntryAncestor(radio)).toBeNull();
        expect(findTextEntryAncestor(text)).toBe(text);
    });

    it("does not steal focus from dialog descendants", () => {
        const terminalElement = document.createElement("div");
        const dialog = document.createElement("div");
        dialog.setAttribute("role", "dialog");
        const button = document.createElement("button");
        dialog.append(button);
        document.body.append(dialog);

        expect(
            shouldRecoverTerminalFocus("window-focus", button, terminalElement, null),
        ).toBe(false);

        dialog.remove();
    });

    it("does not steal focus from auto-enter popover descendants", () => {
        const terminalElement = document.createElement("div");
        const popover = document.createElement("div");
        popover.className = "auto-enter-popover";
        const button = document.createElement("button");
        popover.append(button);
        document.body.append(popover);

        expect(
            shouldRecoverTerminalFocus("window-focus", button, terminalElement, null),
        ).toBe(false);

        popover.remove();
    });

    it("does not steal focus from auto-start popover descendants", () => {
        const terminalElement = document.createElement("div");
        const popover = document.createElement("div");
        popover.className = "auto-start-popover";
        const button = document.createElement("button");
        popover.append(button);
        document.body.append(popover);

        expect(
            shouldRecoverTerminalFocus("window-focus", button, terminalElement, null),
        ).toBe(false);

        popover.remove();
    });

    it("does not recover focus when the composition textarea is already focused", () => {
        const terminalElement = document.createElement("div");
        const textarea = document.createElement("textarea");

        terminalElement.append(textarea);

        expect(
            shouldRecoverTerminalFocus("window-focus", textarea, terminalElement, textarea),
        ).toBe(false);
    });

    it("recovers focus when another element inside the terminal is focused", () => {
        const terminalElement = document.createElement("div");
        const toolbarButton = document.createElement("button");
        terminalElement.append(toolbarButton);

        expect(
            shouldRecoverTerminalFocus("window-focus", toolbarButton, terminalElement, null),
        ).toBe(true);
    });
});
