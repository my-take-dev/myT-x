import { create } from "zustand";
import {clampDockRatio, DOCK_RATIO_DEFAULT} from "./viewerDocking";

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
  // Clamp only the persisted main-pane share here so drag math can stay simple.
  // The actual rendered split still depends on viewerDocking.ts, window scaling,
  // and the viewer minimum width guard applied at layout time.
  setDockRatio: (ratio) => set({ dockRatio: clampDockRatio(ratio) }),
  resetDockRatio: () => set({ dockRatio: DOCK_RATIO_DEFAULT }),
}));
