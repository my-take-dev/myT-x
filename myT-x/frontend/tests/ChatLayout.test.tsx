import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {ChatLayout} from "../src/components/ChatLayout";
import {useChatStore} from "../src/stores/chatStore";

const apiMock = vi.hoisted(() => ({
    SendChatMessage: vi.fn<(paneId: string, message: string) => Promise<void>>(),
    LoadPromptPresets: vi.fn<(sessionName: string) => Promise<unknown>>(),
}));

vi.mock("../src/api", () => ({
    api: {
        SendChatMessage: (paneId: string, message: string) => apiMock.SendChatMessage(paneId, message),
        LoadPromptPresets: (sessionName: string) => apiMock.LoadPromptPresets(sessionName),
    },
}));

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

function unresolvedPromptPresetLoad(): Promise<unknown> {
    return new Promise<unknown>(() => {
        // ChatLayout tests do not exercise prompt preset loading.
    });
}

describe("ChatLayout", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        useChatStore.setState({requestedPaneId: null});
        apiMock.SendChatMessage.mockReset();
        apiMock.SendChatMessage.mockResolvedValue(undefined);
        apiMock.LoadPromptPresets.mockReset();
        apiMock.LoadPromptPresets.mockImplementation(unresolvedPromptPresetLoad);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("renders the collapsed input bar until the user opens the panel", () => {
        act(() => {
            root.render(
                <ChatLayout
                    activePaneId="%1"
                    activePaneTitle="shell"
                    panes={[{id: "%1", title: "shell"} as any]}
                    chatOverlayPercentage={40}
                >
                    <div className="session-probe">session</div>
                </ChatLayout>,
            );
        });

        expect(container.querySelector(".chat-input-bar")).not.toBeNull();
        expect(container.querySelector(".chat-docked-panel")).toBeNull();

        const collapsedBar = container.querySelector<HTMLDivElement>(".chat-input-bar");
        expect(collapsedBar).not.toBeNull();

        act(() => {
            collapsedBar?.dispatchEvent(new KeyboardEvent("keydown", {bubbles: true, key: "Enter"}));
        });

        expect(container.querySelector(".chat-input-bar")).toBeNull();
        expect(container.querySelector(".chat-docked-panel")).not.toBeNull();
    });

    it("moves the chat panel before session content when anchored to the left", () => {
        act(() => {
            root.render(
                <ChatLayout
                    activePaneId="%1"
                    activePaneTitle="shell"
                    panes={[
                        {id: "%1", title: "shell"},
                        {id: "%2", title: "logs"},
                    ] as any}
                    chatOverlayPercentage={40}
                >
                    <div className="session-probe">session</div>
                </ChatLayout>,
            );
        });

        const collapsedBar = container.querySelector<HTMLDivElement>(".chat-input-bar");
        act(() => {
            collapsedBar?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const anchorButtons = container.querySelectorAll<HTMLButtonElement>(".chat-panel-anchor-btn");
        expect(anchorButtons.length).toBeGreaterThanOrEqual(4);

        act(() => {
            anchorButtons[0]?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const layout = container.querySelector<HTMLDivElement>(".chat-layout");
        expect(layout?.classList.contains("chat-layout--horizontal")).toBe(true);
        expect(layout?.firstElementChild?.classList.contains("chat-docked-panel")).toBe(true);
        expect(layout?.lastElementChild?.classList.contains("chat-layout__content")).toBe(true);
    });

    it("opens the requested pane from the store, consumes the request, and supports repeating the same request", async () => {
        act(() => {
            root.render(
                <ChatLayout
                    activePaneId="%1"
                    activePaneTitle="shell"
                    panes={[
                        {id: "%1", title: "shell"},
                        {id: "%2", title: "logs"},
                    ] as any}
                    chatOverlayPercentage={40}
                >
                    <div className="session-probe">session</div>
                </ChatLayout>,
            );
        });

        act(() => {
            useChatStore.getState().requestOpen("%2");
        });
        await flushEffects();

        const selectedButton = Array.from(container.querySelectorAll<HTMLButtonElement>(".chat-panel-pane-icon"))
            .find((button) => button.classList.contains("selected"));
        const textarea = container.querySelector<HTMLTextAreaElement>(".chat-panel-textarea");

        expect(container.querySelector(".chat-input-bar")).toBeNull();
        expect(container.querySelector(".chat-docked-panel")).not.toBeNull();
        expect(selectedButton?.textContent).toContain("%2");
        expect(useChatStore.getState().requestedPaneId).toBeNull();
        expect(document.activeElement).toBe(textarea);

        act(() => {
            container.querySelector<HTMLButtonElement>(".chat-panel-close")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(container.querySelector(".chat-docked-panel")).toBeNull();
        expect(container.querySelector(".chat-input-bar")).not.toBeNull();

        act(() => {
            useChatStore.getState().requestOpen("%2");
        });
        await flushEffects();

        const reopenedSelectedButton = Array.from(container.querySelectorAll<HTMLButtonElement>(".chat-panel-pane-icon"))
            .find((button) => button.classList.contains("selected"));

        expect(container.querySelector(".chat-docked-panel")).not.toBeNull();
        expect(reopenedSelectedButton?.textContent).toContain("%2");
        expect(useChatStore.getState().requestedPaneId).toBeNull();
    });

    it("switches panes and refocuses the textarea when a request arrives while expanded", async () => {
        act(() => {
            root.render(
                <ChatLayout
                    activePaneId="%1"
                    activePaneTitle="shell"
                    panes={[
                        {id: "%1", title: "shell"},
                        {id: "%2", title: "logs"},
                    ] as any}
                    chatOverlayPercentage={40}
                >
                    <div className="session-probe">session</div>
                </ChatLayout>,
            );
        });

        act(() => {
            container.querySelector<HTMLDivElement>(".chat-input-bar")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const closeButton = container.querySelector<HTMLButtonElement>(".chat-panel-close");
        closeButton?.focus();
        expect(document.activeElement).toBe(closeButton);

        act(() => {
            useChatStore.getState().requestOpen("%2");
        });
        await flushEffects();

        const selectedButton = Array.from(container.querySelectorAll<HTMLButtonElement>(".chat-panel-pane-icon"))
            .find((button) => button.classList.contains("selected"));
        const textarea = container.querySelector<HTMLTextAreaElement>(".chat-panel-textarea");

        expect(selectedButton?.textContent).toContain("%2");
        expect(useChatStore.getState().requestedPaneId).toBeNull();
        expect(document.activeElement).toBe(textarea);
    });

    it("keeps the collapsed target label aligned with the actual send target", async () => {
        act(() => {
            root.render(
                <ChatLayout
                    activePaneId="%1"
                    activePaneTitle="shell"
                    panes={[
                        {id: "%1", title: "shell"},
                        {id: "%2", title: "logs"},
                    ] as any}
                    chatOverlayPercentage={40}
                >
                    <div className="session-probe">session</div>
                </ChatLayout>,
            );
        });

        act(() => {
            container.querySelector<HTMLDivElement>(".chat-input-bar")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const paneButtons = container.querySelectorAll<HTMLButtonElement>(".chat-panel-pane-icon");
        act(() => {
            paneButtons[1]?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const textarea = container.querySelector<HTMLTextAreaElement>(".chat-panel-textarea");
        expect(textarea).not.toBeNull();
        act(() => {
            if (textarea === null) {
                return;
            }
            const valueSetter = Object.getOwnPropertyDescriptor(
                HTMLTextAreaElement.prototype,
                "value",
            )?.set;
            valueSetter?.call(textarea, "send to logs");
            textarea.dispatchEvent(new Event("input", {bubbles: true}));
        });

        act(() => {
            container.querySelector<HTMLButtonElement>(".chat-panel-close")
                ?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const collapsedPane = container.querySelector(".chat-input-bar-pane");
        expect(collapsedPane?.textContent).toContain("%2");
        expect(collapsedPane?.textContent).toContain("logs");

        const sendButton = container.querySelector<HTMLButtonElement>(".chat-input-bar-send");
        expect(sendButton?.disabled).toBe(false);

        await act(async () => {
            sendButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        expect(apiMock.SendChatMessage).toHaveBeenCalledOnce();
        expect(apiMock.SendChatMessage).toHaveBeenCalledWith("%2", "send to logs");
    });
});
