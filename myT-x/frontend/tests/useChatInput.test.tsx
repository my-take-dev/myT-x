import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useChatInput} from "../src/components/useChatInput";

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
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
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
});
