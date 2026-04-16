import {create} from "zustand";
import {useTmuxStore} from "./tmuxStore";

interface ChatStoreState {
    requestedPaneId: string | null;
    requestOpen: (paneId: string) => void;
    clearRequest: () => void;
}

export const useChatStore = create<ChatStoreState>((set) => ({
    requestedPaneId: null,
    requestOpen: (paneId) => set({requestedPaneId: paneId}),
    clearRequest: () => set({requestedPaneId: null}),
}));

useTmuxStore.subscribe((state, prevState) => {
    if (state.activeSession === prevState.activeSession) {
        return;
    }
    if (useChatStore.getState().requestedPaneId === null) {
        return;
    }
    useChatStore.setState({requestedPaneId: null});
});
