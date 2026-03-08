import { create } from "zustand";
import { clampDockRatio, DOCK_RATIO_DEFAULT } from "./viewerDocking";

interface ViewerState {
  activeViewId: string | null;
  toggleView: (viewId: string) => void;
  closeView: () => void;
  dockRatio: number;
  setDockRatio: (ratio: number) => void;
  resetDockRatio: () => void;
}

export const useViewerStore = create<ViewerState>((set) => ({
  activeViewId: null,
  toggleView: (viewId) =>
    set((state) => ({
      activeViewId: state.activeViewId === viewId ? null : viewId,
    })),
  closeView: () => set({ activeViewId: null }),
  dockRatio: DOCK_RATIO_DEFAULT,
  // clampDockRatio keeps both panes usable at the 980px minimum window width.
  setDockRatio: (ratio) => set({ dockRatio: clampDockRatio(ratio) }),
  resetDockRatio: () => set({ dockRatio: DOCK_RATIO_DEFAULT }),
}));
