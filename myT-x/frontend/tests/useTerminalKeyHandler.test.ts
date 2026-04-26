import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {TerminalEventShared} from "../src/hooks/useTerminalKeyHandler";
import {setupKeyHandler} from "../src/hooks/useTerminalKeyHandler";

const sendInputMock = vi.hoisted(() => vi.fn<(paneId: string, input: string) => Promise<void>>());
const sendSyncInputMock = vi.hoisted(() => vi.fn<(paneId: string, input: string) => Promise<void>>());
const clipboardGetTextMock = vi.hoisted(() => vi.fn<() => Promise<string>>());
const writeClipboardTextMock = vi.hoisted(() => vi.fn<(text: string) => Promise<void>>());
const notifyAndLogMock = vi.hoisted(() => vi.fn());
const shouldRecoverTerminalFocusMock = vi.hoisted(() => vi.fn(() => false));
const pasteTextSafelyMock = vi.hoisted(() => vi.fn(() => true));
const resolveActivePaneIDMock = vi.hoisted(() => vi.fn(() => "pane-1"));
const tmuxStoreStateMock = vi.hoisted(() => vi.fn(() => ({
    activeSession: "session-1",
    sessions: [{name: "session-1"}],
})));

vi.mock("../src/api", () => ({
    api: {
        SendInput: (paneId: string, input: string) => sendInputMock(paneId, input),
        SendSyncInput: (paneId: string, input: string) => sendSyncInputMock(paneId, input),
    },
}));

vi.mock("../../wailsjs/runtime/runtime", () => ({
    ClipboardGetText: () => clipboardGetTextMock(),
}));

vi.mock("../src/utils/clipboardUtils", () => ({
    writeClipboardText: (text: string) => writeClipboardTextMock(text),
}));

vi.mock("../src/utils/notifyUtils", () => ({
    createConsecutiveFailureCounter: () => ({
        recordFailure: (callback: () => void) => callback(),
        recordSuccess: () => undefined,
    }),
    notifyAndLog: (...args: unknown[]) => notifyAndLogMock(...args),
}));

vi.mock("../src/utils/terminalFocus", () => ({
    shouldRecoverTerminalFocus: (...args: unknown[]) => shouldRecoverTerminalFocusMock(...args),
}));

vi.mock("../src/utils/terminalPaste", () => ({
    pasteTextSafely: (...args: unknown[]) => pasteTextSafelyMock(...args),
}));

vi.mock("../src/utils/session", () => ({
    resolveActivePaneID: (...args: unknown[]) => resolveActivePaneIDMock(...args),
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: {
        getState: (...args: unknown[]) => tmuxStoreStateMock(...args),
    },
}));

interface FakeTerminal {
    attachCustomKeyEventHandler: (handler: (event: KeyboardEvent) => boolean) => void;
    clearSelection: () => void;
    element: HTMLDivElement;
    focus: () => void;
    getSelection: () => string;
    onData: (handler: (input: string) => void) => { dispose: () => void };
    onSelectionChange: (handler: () => void) => { dispose: () => void };
    paste: (data: string) => void;
    textarea: HTMLTextAreaElement;
}

function createInputEvent(inputType: string, data: string): Event {
    const event = new Event("input", {bubbles: true, cancelable: true, composed: false});
    Object.defineProperty(event, "inputType", {value: inputType});
    Object.defineProperty(event, "data", {value: data});
    return event;
}

function createCompositionStartEvent(): Event {
    return new Event("compositionstart", {bubbles: true});
}

function createCompositionEndEvent(data: string): Event {
    const event = new Event("compositionend", {bubbles: true});
    Object.defineProperty(event, "data", {value: data});
    return event;
}

function createFakeTerminal(): FakeTerminal {
    const element = document.createElement("div");
    const textarea = document.createElement("textarea");
    element.appendChild(textarea);
    document.body.appendChild(element);

    return {
        attachCustomKeyEventHandler: () => undefined,
        clearSelection: () => undefined,
        element,
        focus: () => undefined,
        getSelection: () => "",
        onData: () => ({dispose: () => undefined}),
        onSelectionChange: () => ({dispose: () => undefined}),
        paste: () => undefined,
        textarea,
    };
}

interface Harness {
    cleanup: () => void;
    finishComposition: ReturnType<typeof vi.fn>;
    markCompositionStart: ReturnType<typeof vi.fn>;
    term: FakeTerminal;
    textareaInputListener: ReturnType<typeof vi.fn>;
}

function setupHarness(): Harness {
    const term = createFakeTerminal();
    const shared: TerminalEventShared = {
        disposed: false,
        pageHidden: false,
        isComposing: false,
        composingOutput: [],
    };
    const isComposingRef = {current: false};
    const syncInputModeRef = {current: false};
    const finishComposition = vi.fn(() => undefined);
    const markCompositionStart = vi.fn(() => undefined);

    const cleanup = setupKeyHandler({
        term,
        shared,
        ime: {
            gate: {
                markCompositionStart,
                filterInput: (input: string) => input,
            },
            compositionTextarea: term.textarea,
            finishComposition,
        },
        paneId: "pane-1",
        isComposingRef,
        syncInputModeRef,
        setSearchOpen: () => undefined,
        setPrefixMode: () => undefined,
    });

    const textareaInputListener = vi.fn();
    term.textarea.addEventListener("input", textareaInputListener);

    return {cleanup, finishComposition, markCompositionStart, term, textareaInputListener};
}

describe("setupKeyHandler IME capture guard", () => {
    beforeEach(() => {
        sendInputMock.mockReset();
        sendSyncInputMock.mockReset();
        clipboardGetTextMock.mockReset();
        writeClipboardTextMock.mockReset();
        notifyAndLogMock.mockReset();
        shouldRecoverTerminalFocusMock.mockReset();
        shouldRecoverTerminalFocusMock.mockReturnValue(false);
        pasteTextSafelyMock.mockReset();
        pasteTextSafelyMock.mockReturnValue(true);
        resolveActivePaneIDMock.mockReset();
        resolveActivePaneIDMock.mockReturnValue("pane-1");
        tmuxStoreStateMock.mockReset();
        tmuxStoreStateMock.mockReturnValue({
            activeSession: "session-1",
            sessions: [{name: "session-1"}],
        });
        vi.spyOn(console, "warn").mockImplementation(() => undefined);
        vi.spyOn(console, "debug").mockImplementation(() => undefined);
    });

    afterEach(() => {
        document.body.innerHTML = "";
        vi.restoreAllMocks();
    });

    it("blocks insertText input events immediately after compositionend", () => {
        const {cleanup, finishComposition, term, textareaInputListener} = setupHarness();

        let now = 1_000;
        vi.spyOn(performance, "now").mockImplementation(() => now);

        term.textarea.dispatchEvent(createCompositionEndEvent("漢字"));
        expect(finishComposition).toHaveBeenCalledWith(true, "漢字");

        now = 1_020;
        term.textarea.dispatchEvent(createInputEvent("insertText", "漢字"));

        expect(textareaInputListener).not.toHaveBeenCalled();

        cleanup();
    });

    it("blocks insertText at 49ms after compositionend", () => {
        const {cleanup, term, textareaInputListener} = setupHarness();

        let now = 2_000;
        vi.spyOn(performance, "now").mockImplementation(() => now);

        term.textarea.dispatchEvent(createCompositionEndEvent("漢字"));

        now = 2_049;
        term.textarea.dispatchEvent(createInputEvent("insertText", "漢字"));

        expect(textareaInputListener).not.toHaveBeenCalled();

        cleanup();
    });

    it("does not block insertText exactly at 50ms after compositionend", () => {
        const {cleanup, term, textareaInputListener} = setupHarness();

        let now = 3_000;
        vi.spyOn(performance, "now").mockImplementation(() => now);

        term.textarea.dispatchEvent(createCompositionEndEvent("漢字"));

        now = 3_050;
        term.textarea.dispatchEvent(createInputEvent("insertText", "漢字"));

        expect(textareaInputListener).toHaveBeenCalledTimes(1);

        cleanup();
    });

    it("does not block insertText input events after the 50ms guard window", () => {
        const {cleanup, term, textareaInputListener} = setupHarness();

        let now = 4_000;
        vi.spyOn(performance, "now").mockImplementation(() => now);

        term.textarea.dispatchEvent(createCompositionEndEvent("漢字"));

        now = 4_060;
        term.textarea.dispatchEvent(createInputEvent("insertText", "漢字"));

        expect(textareaInputListener).toHaveBeenCalledTimes(1);

        cleanup();
    });

    it("resets the guard window on compositionstart so subsequent insertText is not blocked", () => {
        const {cleanup, markCompositionStart, term, textareaInputListener} = setupHarness();

        let now = 5_000;
        vi.spyOn(performance, "now").mockImplementation(() => now);

        term.textarea.dispatchEvent(createCompositionEndEvent("漢字"));
        now = 5_010;
        term.textarea.dispatchEvent(createCompositionStartEvent());
        expect(markCompositionStart).toHaveBeenCalledTimes(1);

        now = 5_020;
        term.textarea.dispatchEvent(createInputEvent("insertText", "漢字"));

        expect(textareaInputListener).toHaveBeenCalledTimes(1);

        cleanup();
    });

    it("never blocks non-insertText input types", () => {
        const {cleanup, term, textareaInputListener} = setupHarness();

        let now = 6_000;
        vi.spyOn(performance, "now").mockImplementation(() => now);

        term.textarea.dispatchEvent(createCompositionEndEvent("漢字"));
        now = 6_010;
        term.textarea.dispatchEvent(createInputEvent("insertCompositionText", "漢"));
        term.textarea.dispatchEvent(createInputEvent("deleteContentBackward", ""));

        expect(textareaInputListener).toHaveBeenCalledTimes(2);

        cleanup();
    });

    it("does not block insertText when compositionend has not fired", () => {
        const {cleanup, term, textareaInputListener} = setupHarness();

        let now = 7_000;
        vi.spyOn(performance, "now").mockImplementation(() => now);

        term.textarea.dispatchEvent(createInputEvent("insertText", "a"));

        expect(textareaInputListener).toHaveBeenCalledTimes(1);

        cleanup();
    });
});
