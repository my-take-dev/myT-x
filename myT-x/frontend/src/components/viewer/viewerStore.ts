import { create } from "zustand";
import { clampDockRatio, DOCK_RATIO_DEFAULT } from "./viewerDocking";

interface ViewerState {
  activeViewId: string | null;
  viewContext: Record<string, unknown> | null;
  toggleView: (viewId: string) => void;
  closeView: () => void;
  openViewWithContext: (viewId: string, context: Record<string, unknown>) => void;
  dockRatio: number;
  setDockRatio: (ratio: number) => void;
  resetDockRatio: () => void;
}

export const useViewerStore = create<ViewerState>((set) => ({
  activeViewId: null,
  viewContext: null,
  toggleView: (viewId) =>
    set((state) => ({
      activeViewId: state.activeViewId === viewId ? null : viewId,
      viewContext: null,
    })),
  closeView: () => set({ activeViewId: null, viewContext: null }),
  openViewWithContext: (viewId, context) =>
    set({ activeViewId: viewId, viewContext: context }),
  dockRatio: DOCK_RATIO_DEFAULT,
  // clampDockRatio keeps both panes usable at the 980px minimum window width.
  setDockRatio: (ratio) => set({ dockRatio: clampDockRatio(ratio) }),
  resetDockRatio: () => set({ dockRatio: DOCK_RATIO_DEFAULT }),
}));
