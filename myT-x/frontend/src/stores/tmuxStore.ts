import { create } from "zustand";
import type { AppConfig, SessionSnapshot } from "../types/tmux";

/** sessionOrderに従ってセッション配列をソートする。orderにないセッションはID順で末尾に追加。 */
function sortByOrder(sessions: SessionSnapshot[], order: string[]): SessionSnapshot[] {
  const orderMap = new Map(order.map((name, i) => [name, i]));
  return [...sessions].sort((a, b) => {
    const ai = orderMap.get(a.name) ?? Number.MAX_SAFE_INTEGER;
    const bi = orderMap.get(b.name) ?? Number.MAX_SAFE_INTEGER;
    if (ai !== bi) return ai - bi;
    return a.id - b.id;
  });
}

/** セッション配列からorder配列を再構築する。 */
function buildOrder(sessions: SessionSnapshot[], prevOrder: string[]): string[] {
  const existing = new Set(sessions.map((s) => s.name));
  // 既存orderから削除されたものを除外
  const kept = prevOrder.filter((name) => existing.has(name));
  // 新規セッションを末尾に追加
  const inOrder = new Set(kept);
  const newNames = sessions
    .filter((s) => !inOrder.has(s.name))
    .sort((a, b) => a.id - b.id)
    .map((s) => s.name);
  return [...kept, ...newNames];
}

interface TmuxState {
  config: AppConfig | null;
  sessions: SessionSnapshot[];
  sessionOrder: string[];
  activeSession: string | null;
  zoomPaneId: string | null;
  prefixMode: boolean;
  syncInputMode: boolean;
  fontSize: number;
  setConfig: (config: AppConfig) => void;
  setSessions: (sessions: SessionSnapshot[]) => void;
  applySessionDelta: (upserts: SessionSnapshot[], removed: string[]) => void;
  setActiveSession: (sessionName: string | null) => void;
  setZoomPaneId: (paneId: string | null) => void;
  setPrefixMode: (enabled: boolean) => void;
  setSyncInputMode: (enabled: boolean) => void;
  toggleSyncInputMode: () => void;
  setFontSize: (size: number) => void;
  reorderSession: (fromIndex: number, toIndex: number) => void;
}

export const useTmuxStore = create<TmuxState>((set) => ({
  config: null,
  sessions: [],
  sessionOrder: [],
  activeSession: null,
  zoomPaneId: null,
  prefixMode: false,
  syncInputMode: false,
  fontSize: 13,
  setConfig: (config) => set({ config }),
  setSessions: (sessions) =>
    set((state) => {
      const order = buildOrder(sessions, state.sessionOrder);
      const sorted = sortByOrder(sessions, order);
      return {
        sessions: sorted,
        sessionOrder: order,
        activeSession:
          state.activeSession && sorted.some((s) => s.name === state.activeSession)
            ? state.activeSession
            : sorted[0]?.name ?? null,
      };
    }),
  applySessionDelta: (upserts, removed) =>
    set((state) => {
      const byName = new Map(state.sessions.map((session) => [session.name, session]));
      for (const name of removed) {
        byName.delete(name);
      }
      for (const session of upserts) {
        byName.set(session.name, session);
      }
      const all = Array.from(byName.values());
      const order = buildOrder(all, state.sessionOrder);
      const sessions = sortByOrder(all, order);
      return {
        sessions,
        sessionOrder: order,
        activeSession:
          state.activeSession && sessions.some((s) => s.name === state.activeSession)
            ? state.activeSession
            : sessions[0]?.name ?? null,
      };
    }),
  setActiveSession: (activeSession) => set({ activeSession }),
  setZoomPaneId: (zoomPaneId) => set({ zoomPaneId }),
  setPrefixMode: (prefixMode) => set({ prefixMode }),
  setSyncInputMode: (syncInputMode) => set({ syncInputMode }),
  toggleSyncInputMode: () => set((state) => ({ syncInputMode: !state.syncInputMode })),
  setFontSize: (fontSize) => set({ fontSize }),
  reorderSession: (fromIndex, toIndex) =>
    set((state) => {
      const order = [...state.sessionOrder];
      if (fromIndex < 0 || fromIndex >= order.length || toIndex < 0 || toIndex >= order.length) {
        return state;
      }
      const [moved] = order.splice(fromIndex, 1);
      order.splice(toIndex, 0, moved);
      const sorted = sortByOrder(state.sessions, order);
      return { sessions: sorted, sessionOrder: order };
    }),
}));
