import {act, type KeyboardEvent as ReactKeyboardEvent} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useChatInput} from "../src/components/useChatInput";
import {type InputHistoryEntry, useInputHistoryStore} from "../src/stores/inputHistoryStore";

const apiMock = vi.hoisted(() => ({
    SendChatMessage: vi.fn<(paneId: string, message: string) => Promise<void>>(),
}));

vi.mock("../src/api", () => ({
    api: {
        SendChatMessage: (paneId: string, message: string) => apiMock.SendChatMessage(paneId, message),
    },
}));

interface ChatInputProbeProps {
    activePaneId: string | null;
    paneIds: readonly string[];
}

type ChatInputProbeState = ReturnType<typeof useChatInput>;

let currentState: ChatInputProbeState | null = null;

function ChatInputProbe({activePaneId, paneIds}: ChatInputProbeProps) {
    currentState = useChatInput({
        activePaneId,
        paneIds,
        autoClose: false,
        expanded: true,
        setExpanded: () => undefined,
    });

    return null;
}

function getCurrentState(): ChatInputProbeState {
    expect(currentState).not.toBeNull();
    return currentState as ChatInputProbeState;
}

function inputHistoryEntry(seq: number, overrides: Partial<InputHistoryEntry> = {}): InputHistoryEntry {
    return {
        seq,
        ts: "20260429120000",
        pane_id: "%0",
        input: `input-${seq}`,
        source: "chat",
        session: "session-1",
        ...overrides,
    };
}

function setInputHistoryEntries(entries: InputHistoryEntry[], scopeKey = ""): void {
    useInputHistoryStore.getState().setSnapshot({scope_key: scopeKey, entries});
}

interface KeyDownEventOptions {
    readonly altKey?: boolean;
    readonly ctrlKey?: boolean;
    readonly metaKey?: boolean;
    readonly shiftKey?: boolean;
    readonly isComposing?: boolean;
    readonly keyCode?: number;
    readonly currentTarget?: Pick<HTMLTextAreaElement, "selectionEnd" | "selectionStart" | "value">;
}

function keyDownEvent(key: string, options: KeyDownEventOptions = {}): {
    readonly event: ReactKeyboardEvent<HTMLTextAreaElement>;
    readonly preventDefault: ReturnType<typeof vi.fn>;
} {
    const preventDefault = vi.fn();
    const fallbackValue = currentState?.text ?? "";
    const fallbackTarget = {
        value: fallbackValue,
        selectionStart: fallbackValue.length,
        selectionEnd: fallbackValue.length,
    };
    return {
        event: {
            key,
            altKey: options.altKey ?? false,
            ctrlKey: options.ctrlKey ?? false,
            metaKey: options.metaKey ?? false,
            shiftKey: options.shiftKey ?? false,
            currentTarget: (options.currentTarget ?? fallbackTarget) as HTMLTextAreaElement,
            nativeEvent: {
                isComposing: options.isComposing ?? false,
                key,
                keyCode: options.keyCode ?? 0,
            } as KeyboardEvent,
            preventDefault,
        } as ReactKeyboardEvent<HTMLTextAreaElement>,
        preventDefault,
    };
}

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

describe("useChatInput", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        currentState = null;
        apiMock.SendChatMessage.mockReset();
        apiMock.SendChatMessage.mockResolvedValue(undefined);
        useInputHistoryStore.setState({
            scopeKey: "",
            entries: [],
            unreadCount: 0,
            lastReadSeq: 0,
            readSeqByScope: {},
        });
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        useInputHistoryStore.setState({
            scopeKey: "",
            entries: [],
            unreadCount: 0,
            lastReadSeq: 0,
            readSeqByScope: {},
        });
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("falls back to the current active pane when the selected pane is no longer available", async () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1", "pane-2"]}/>);
        });

        act(() => {
            getCurrentState().setSelectedPaneId("pane-2");
        });

        expect(getCurrentState().selectedPaneId).toBe("pane-2");
        expect(getCurrentState().targetPaneId).toBe("pane-2");

        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-9" paneIds={["pane-9"]}/>);
        });
        await flushEffects();

        expect(getCurrentState().selectedPaneId).toBeNull();
        expect(getCurrentState().targetPaneId).toBe("pane-9");

        act(() => {
            getCurrentState().setText("send to active pane");
        });

        await act(async () => {
            await getCurrentState().handleSend();
        });

        expect(apiMock.SendChatMessage).toHaveBeenCalledOnce();
        expect(apiMock.SendChatMessage).toHaveBeenCalledWith("pane-9", "send to active pane");
    });

    it("keeps an explicit pane selection while the pane remains in the current pane list", async () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1", "pane-2"]}/>);
        });

        act(() => {
            getCurrentState().setSelectedPaneId("pane-2");
        });

        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1", "pane-2", "pane-3"]}/>);
        });
        await flushEffects();

        expect(getCurrentState().selectedPaneId).toBe("pane-2");
        expect(getCurrentState().targetPaneId).toBe("pane-2");
    });

    it("recalls non-empty chat input history from all panes in newest-first order", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1", "pane-2"]}/>);
        });

        act(() => {
            setInputHistoryEntries([
                inputHistoryEntry(1, {pane_id: "pane-1", input: "old pane message"}),
                inputHistoryEntry(2, {pane_id: "pane-1", input: "terminal command", source: "keyboard"}),
                inputHistoryEntry(3, {pane_id: "pane-1", input: "   "}),
                inputHistoryEntry(4, {pane_id: "pane-2", input: "latest pane message"}),
            ]);
        });

        const firstUp = keyDownEvent("ArrowUp");
        act(() => {
            getCurrentState().handleExpandedKeyDown(firstUp.event);
        });

        expect(firstUp.preventDefault).toHaveBeenCalledOnce();
        expect(getCurrentState().text).toBe("latest pane message");

        const secondUp = keyDownEvent("ArrowUp");
        act(() => {
            getCurrentState().handleExpandedKeyDown(secondUp.event);
        });

        expect(secondUp.preventDefault).toHaveBeenCalledOnce();
        expect(getCurrentState().text).toBe("old pane message");
    });

    it("collapses consecutive duplicate chat history entries to the newest matching entry", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([
                inputHistoryEntry(1, {input: "repeat"}),
                inputHistoryEntry(2, {input: "repeat"}),
                inputHistoryEntry(3, {input: "newest"}),
            ]);
        });

        act(() => {
            getCurrentState().handleExpandedKeyDown(keyDownEvent("ArrowUp").event);
        });
        expect(getCurrentState().text).toBe("newest");

        act(() => {
            getCurrentState().handleExpandedKeyDown(keyDownEvent("ArrowUp").event);
        });
        expect(getCurrentState().text).toBe("repeat");

        const oldestAgain = keyDownEvent("ArrowUp");
        act(() => {
            getCurrentState().handleExpandedKeyDown(oldestAgain.event);
        });
        expect(oldestAgain.preventDefault).toHaveBeenCalledOnce();
        expect(getCurrentState().text).toBe("repeat");
    });

    it("stops safely at the oldest recalled history entry", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([
                inputHistoryEntry(1, {input: "oldest"}),
                inputHistoryEntry(2, {input: "newest"}),
            ]);
        });

        for (const expectedText of ["newest", "oldest", "oldest"]) {
            const up = keyDownEvent("ArrowUp");
            act(() => {
                getCurrentState().handleExpandedKeyDown(up.event);
            });
            expect(up.preventDefault).toHaveBeenCalledOnce();
            expect(getCurrentState().text).toBe(expectedText);
        }
    });

    it("returns through newer history and then clears after the newest entry", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([
                inputHistoryEntry(1, {input: "oldest"}),
                inputHistoryEntry(2, {input: "newest"}),
            ]);
        });

        for (const key of ["ArrowUp", "ArrowUp"]) {
            act(() => {
                getCurrentState().handleExpandedKeyDown(keyDownEvent(key).event);
            });
        }
        expect(getCurrentState().text).toBe("oldest");

        const firstDown = keyDownEvent("ArrowDown");
        act(() => {
            getCurrentState().handleExpandedKeyDown(firstDown.event);
        });

        expect(firstDown.preventDefault).toHaveBeenCalledOnce();
        expect(getCurrentState().text).toBe("newest");

        const secondDown = keyDownEvent("ArrowDown");
        act(() => {
            getCurrentState().handleExpandedKeyDown(secondDown.event);
        });

        expect(secondDown.preventDefault).toHaveBeenCalledOnce();
        expect(getCurrentState().text).toBe("");

        const thirdDown = keyDownEvent("ArrowDown");
        act(() => {
            getCurrentState().handleExpandedKeyDown(thirdDown.event);
        });

        expect(thirdDown.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("");
    });

    it("does not intercept arrow keys while the user is typing normal text", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([inputHistoryEntry(1, {input: "history"})]);
            getCurrentState().setDirectText("manual input");
        });

        const up = keyDownEvent("ArrowUp");
        act(() => {
            getCurrentState().handleExpandedKeyDown(up.event);
        });

        expect(up.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("manual input");
    });

    it("does not intercept history arrows when the caret is before the end of recalled text", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([
                inputHistoryEntry(1, {input: "oldest"}),
                inputHistoryEntry(2, {input: "newest"}),
            ]);
        });

        act(() => {
            getCurrentState().handleExpandedKeyDown(keyDownEvent("ArrowUp").event);
        });
        expect(getCurrentState().text).toBe("newest");

        for (const key of ["ArrowUp", "ArrowDown"]) {
            const arrow = keyDownEvent(key, {
                currentTarget: {
                    value: "newest",
                    selectionStart: "new".length,
                    selectionEnd: "new".length,
                },
            });
            act(() => {
                getCurrentState().handleExpandedKeyDown(arrow.event);
            });

            expect(arrow.preventDefault).not.toHaveBeenCalled();
            expect(getCurrentState().text).toBe("newest");
        }
    });

    it("does not intercept modified arrow keys during history browsing", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([
                inputHistoryEntry(1, {input: "oldest"}),
                inputHistoryEntry(2, {input: "newest"}),
            ]);
        });

        act(() => {
            getCurrentState().handleExpandedKeyDown(keyDownEvent("ArrowUp").event);
        });
        expect(getCurrentState().text).toBe("newest");

        for (const modifier of [
            {altKey: true},
            {ctrlKey: true},
            {metaKey: true},
            {shiftKey: true},
        ]) {
            const up = keyDownEvent("ArrowUp", modifier);
            act(() => {
                getCurrentState().handleExpandedKeyDown(up.event);
            });
            expect(up.preventDefault).not.toHaveBeenCalled();
            expect(getCurrentState().text).toBe("newest");

            const down = keyDownEvent("ArrowDown", modifier);
            act(() => {
                getCurrentState().handleExpandedKeyDown(down.event);
            });
            expect(down.preventDefault).not.toHaveBeenCalled();
            expect(getCurrentState().text).toBe("newest");
        }
    });

    it("does not intercept history arrows while IME composition is active", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([inputHistoryEntry(1, {input: "history"})]);
            getCurrentState().handleCompositionStart();
        });

        const up = keyDownEvent("ArrowUp");
        act(() => {
            getCurrentState().handleExpandedKeyDown(up.event);
        });

        expect(up.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("");
    });

    it("keeps recalled history stable when the history list shifts", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([
                inputHistoryEntry(1, {input: "oldest"}),
                inputHistoryEntry(2, {input: "recalled"}),
                inputHistoryEntry(3, {input: "newer"}),
            ]);
        });

        for (const key of ["ArrowUp", "ArrowUp"]) {
            act(() => {
                getCurrentState().handleExpandedKeyDown(keyDownEvent(key).event);
            });
        }
        expect(getCurrentState().text).toBe("recalled");

        act(() => {
            setInputHistoryEntries([
                inputHistoryEntry(2, {input: "recalled"}),
                inputHistoryEntry(3, {input: "newer"}),
                inputHistoryEntry(4, {input: "newest"}),
            ]);
        });

        const down = keyDownEvent("ArrowDown");
        act(() => {
            getCurrentState().handleExpandedKeyDown(down.event);
        });

        expect(down.preventDefault).toHaveBeenCalledOnce();
        expect(getCurrentState().text).toBe("newer");
    });

    it("does not use history navigation inside recalled multiline text or text selections", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([
                inputHistoryEntry(1, {input: "old"}),
                inputHistoryEntry(2, {input: "line 1\nline 2\nline 3"}),
            ]);
        });

        act(() => {
            getCurrentState().handleExpandedKeyDown(keyDownEvent("ArrowUp").event);
        });
        expect(getCurrentState().text).toBe("line 1\nline 2\nline 3");

        const upFromMiddleLine = keyDownEvent("ArrowUp", {
            currentTarget: {
                value: "line 1\nline 2\nline 3",
                selectionStart: "line 1\n".length,
                selectionEnd: "line 1\n".length,
            },
        });
        act(() => {
            getCurrentState().handleExpandedKeyDown(upFromMiddleLine.event);
        });

        expect(upFromMiddleLine.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("line 1\nline 2\nline 3");

        const downFromMiddleLine = keyDownEvent("ArrowDown", {
            currentTarget: {
                value: "line 1\nline 2\nline 3",
                selectionStart: "line 1\nline 2".length,
                selectionEnd: "line 1\nline 2".length,
            },
        });
        act(() => {
            getCurrentState().handleExpandedKeyDown(downFromMiddleLine.event);
        });

        expect(downFromMiddleLine.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("line 1\nline 2\nline 3");

        const upWithSelection = keyDownEvent("ArrowUp", {
            currentTarget: {
                value: "line 1\nline 2\nline 3",
                selectionStart: 0,
                selectionEnd: "line 1".length,
            },
        });
        act(() => {
            getCurrentState().handleExpandedKeyDown(upWithSelection.event);
        });

        expect(upWithSelection.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("line 1\nline 2\nline 3");
    });

    it("uses history navigation only at the absolute text end", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([
                inputHistoryEntry(1, {input: "old"}),
                inputHistoryEntry(2, {input: "line 1\nline 2"}),
            ]);
        });

        act(() => {
            getCurrentState().handleExpandedKeyDown(keyDownEvent("ArrowUp").event);
        });
        expect(getCurrentState().text).toBe("line 1\nline 2");

        const upFromFirstLine = keyDownEvent("ArrowUp", {
            currentTarget: {
                value: "line 1\nline 2",
                selectionStart: "line".length,
                selectionEnd: "line".length,
            },
        });
        act(() => {
            getCurrentState().handleExpandedKeyDown(upFromFirstLine.event);
        });

        expect(upFromFirstLine.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("line 1\nline 2");

        const downFromLastLine = keyDownEvent("ArrowDown", {
            currentTarget: {
                value: "line 1\nline 2",
                selectionStart: "line 1\nline 2".length,
                selectionEnd: "line 1\nline 2".length,
            },
        });
        act(() => {
            getCurrentState().handleExpandedKeyDown(downFromLastLine.event);
        });

        expect(downFromLastLine.preventDefault).toHaveBeenCalledOnce();
        expect(getCurrentState().text).toBe("");
    });

    it("does not intercept ArrowUp when history is empty", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        const up = keyDownEvent("ArrowUp");
        act(() => {
            getCurrentState().handleExpandedKeyDown(up.event);
        });

        expect(up.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("");
    });

    it("resets history recall after direct user input or preset-style text updates", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([inputHistoryEntry(1, {input: "history"})]);
        });

        act(() => {
            getCurrentState().handleExpandedKeyDown(keyDownEvent("ArrowUp").event);
        });
        expect(getCurrentState().text).toBe("history");

        act(() => {
            getCurrentState().setDirectText("manual input");
        });

        const downAfterManualInput = keyDownEvent("ArrowDown");
        act(() => {
            getCurrentState().handleExpandedKeyDown(downAfterManualInput.event);
        });

        expect(downAfterManualInput.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("manual input");

        act(() => {
            getCurrentState().setText("");
        });
        act(() => {
            getCurrentState().handleExpandedKeyDown(keyDownEvent("ArrowUp").event);
        });

        act(() => {
            getCurrentState().setText((previous) => `${previous} with preset`);
        });

        const downAfterPresetUpdate = keyDownEvent("ArrowDown");
        act(() => {
            getCurrentState().handleExpandedKeyDown(downAfterPresetUpdate.event);
        });

        expect(downAfterPresetUpdate.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("history with preset");
    });

    it("allows history recall after direct input is cleared", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([inputHistoryEntry(1, {input: "history"})]);
            getCurrentState().setDirectText("manual input");
        });

        act(() => {
            getCurrentState().setDirectText("");
        });

        const up = keyDownEvent("ArrowUp");
        act(() => {
            getCurrentState().handleExpandedKeyDown(up.event);
        });

        expect(up.preventDefault).toHaveBeenCalledOnce();
        expect(getCurrentState().text).toBe("history");
    });

    it("allows history recall from non-empty programmatic text", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([inputHistoryEntry(1, {input: "history"})]);
            getCurrentState().setText("preset text");
        });

        const up = keyDownEvent("ArrowUp");
        act(() => {
            getCurrentState().handleExpandedKeyDown(up.event);
        });

        expect(up.preventDefault).toHaveBeenCalledOnce();
        expect(getCurrentState().text).toBe("history");
    });

    it("keeps history recall disabled when programmatic text is appended after direct input", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            setInputHistoryEntries([inputHistoryEntry(1, {input: "history"})]);
            getCurrentState().setDirectText("manual input");
            getCurrentState().setText((previous) => `${previous}\npreset text`);
        });

        const up = keyDownEvent("ArrowUp");
        act(() => {
            getCurrentState().handleExpandedKeyDown(up.event);
        });

        expect(up.preventDefault).not.toHaveBeenCalled();
        expect(getCurrentState().text).toBe("manual input\npreset text");
    });

    it("recalls only entries from the active input-history scope", () => {
        act(() => {
            root.render(<ChatInputProbe activePaneId="pane-1" paneIds={["pane-1"]}/>);
        });

        act(() => {
            useInputHistoryStore.getState().setSnapshot({
                scope_key: "scope-a",
                entries: [inputHistoryEntry(1, {input: "scope-a command"})],
            });
            useInputHistoryStore.getState().setSnapshot({
                scope_key: "scope-b",
                entries: [inputHistoryEntry(1, {input: "scope-b command"})],
            });
        });

        act(() => {
            getCurrentState().handleExpandedKeyDown(keyDownEvent("ArrowUp").event);
        });

        expect(getCurrentState().text).toBe("scope-b command");
    });
});
