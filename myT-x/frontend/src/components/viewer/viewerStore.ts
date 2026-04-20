import { create } from "zustand";
import {clampDockRatio, DOCK_RATIO_DEFAULT} from "./viewerDocking";
import type {OpenViewWithContext, ViewerContext} from "./viewerContext";

interface ViewerState {
  activeViewId: string | null;
  viewContext: ViewerContext | null;
  openView: (viewId: string) => void;
  toggleView: (viewId: string) => void;
  closeView: () => void;
  clearViewContext: () => void;
  openViewWithContext: OpenViewWithContext;
  dockRatio: number;
  setDockRatio: (ratio: number) => void;
  resetDockRatio: () => void;
}

export const useViewerStore = create<ViewerState>((set) => ({
  activeViewId: null,
  viewContext: null,
  openView: (viewId) => set({ activeViewId: viewId, viewContext: null }),
  toggleView: (viewId) =>
    set((state) => ({
      activeViewId: state.activeViewId === viewId ? null : viewId,
      viewContext: null,
    })),
  closeView: () => set({ activeViewId: null, viewContext: null }),
  clearViewContext: () => set({ viewContext: null }),
  openViewWithContext: (viewId, context) =>
    set({ activeViewId: viewId, viewContext: context }),
  dockRatio: DOCK_RATIO_DEFAULT,
  // Clamp only the persisted main-pane share here so drag math can stay simple.
  // The actual rendered split still depends on viewerDocking.ts, window scaling,
  // and the viewer minimum width guard applied at layout time.
  setDockRatio: (ratio) => set({ dockRatio: clampDockRatio(ratio) }),
  resetDockRatio: () => set({ dockRatio: DOCK_RATIO_DEFAULT }),
}));
