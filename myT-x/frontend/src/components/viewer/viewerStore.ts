import { create } from "zustand";

interface ViewerState {
  activeViewId: string | null;
  toggleView: (viewId: string) => void;
  closeView: () => void;
}

export const useViewerStore = create<ViewerState>((set) => ({
  activeViewId: null,
  toggleView: (viewId) =>
    set((state) => ({
      activeViewId: state.activeViewId === viewId ? null : viewId,
    })),
  closeView: () => set({ activeViewId: null }),
}));
