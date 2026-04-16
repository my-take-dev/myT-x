import {create} from "zustand";

interface PromptPresetStoreState {
    version: number;
    bumpVersion: () => void;
}

export const usePromptPresetStore = create<PromptPresetStoreState>((set) => ({
    version: 0,
    bumpVersion: () => set((state) => ({version: state.version + 1})),
}));
