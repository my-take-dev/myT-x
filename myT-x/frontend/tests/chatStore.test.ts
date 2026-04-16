import {beforeEach, describe, expect, it} from "vitest";
import {useChatStore} from "../src/stores/chatStore";
import {useTmuxStore} from "../src/stores/tmuxStore";

describe("chatStore", () => {
    beforeEach(() => {
        useChatStore.setState({requestedPaneId: null});
        useTmuxStore.setState({activeSession: null, activeWindowId: null});
    });

    it("initializes with requestedPaneId as null", () => {
        expect(useChatStore.getState().requestedPaneId).toBeNull();
    });

    it("requestOpen sets requestedPaneId", () => {
        useChatStore.getState().requestOpen("%1");
        expect(useChatStore.getState().requestedPaneId).toBe("%1");
    });

    it("requestOpen overwrites previous requestedPaneId", () => {
        useChatStore.getState().requestOpen("%1");
        useChatStore.getState().requestOpen("%2");
        expect(useChatStore.getState().requestedPaneId).toBe("%2");
    });

    it("clearRequest resets requestedPaneId to null", () => {
        useChatStore.getState().requestOpen("%1");
        useChatStore.getState().clearRequest();
        expect(useChatStore.getState().requestedPaneId).toBeNull();
    });

    it("clearRequest is a no-op when already null", () => {
        useChatStore.getState().clearRequest();
        expect(useChatStore.getState().requestedPaneId).toBeNull();
    });

    it("can request again after clearing", () => {
        useChatStore.getState().requestOpen("%1");
        useChatStore.getState().clearRequest();
        useChatStore.getState().requestOpen("%3");
        expect(useChatStore.getState().requestedPaneId).toBe("%3");
    });

    it("clears stale requests when the active session changes", () => {
        useTmuxStore.getState().setActiveSession("session-a");
        useChatStore.getState().requestOpen("%1");

        useTmuxStore.getState().setActiveSession("session-b");

        expect(useChatStore.getState().requestedPaneId).toBeNull();
    });
});
