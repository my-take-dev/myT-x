import {create} from "zustand";
import type {AppConfig, SessionSnapshot} from "../types/tmux";

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
    activeWindowId: string | null;
    zoomPaneId: string | null;
    pendingPrefixKillPaneId: string | null;
    prefixMode: boolean;
    syncInputMode: boolean;
    fontSize: number;
    setConfig: (config: AppConfig) => void;
    setSessions: (sessions: SessionSnapshot[]) => void;
    applySessionDelta: (upserts: SessionSnapshot[], removed: string[]) => void;
    setActiveSession: (sessionName: string | null) => void;
    /** S-22: アクティブウィンドウIDを更新する。null または空文字はそのまま null に正規化する。 */
    setActiveWindowId: (windowId: string | null) => void;
    setZoomPaneId: (paneId: string | null) => void;
    setPendingPrefixKillPaneId: (paneId: string | null) => void;
    setPrefixMode: (enabled: boolean) => void;
    setSyncInputMode: (enabled: boolean) => void;
    toggleSyncInputMode: () => void;
    setFontSize: (size: number) => void;
    reorderSession: (fromIndex: number, toIndex: number) => void;
}

/**
 * T-12: Safely resolve active_window_id from a session snapshot.
 *
 * active_window_id is typed as number, but the backend may omit the field
 * (JSON omitempty or zero-value), making it undefined at runtime.
 * String(undefined) === "undefined" would cause window lookup failures, so
 * a typeof guard ensures the value is a number before conversion.
 * 0 is a valid window ID, therefore a falsy check must not be used.
 */
function resolveActiveWindowId(sessionSnapshot: SessionSnapshot | null): string | null {
    const rawWindowId = sessionSnapshot?.active_window_id;
    return typeof rawWindowId === "number" ? String(rawWindowId) : null;
}

/**
 * I-36: setSessions / applySessionDelta で重複していた
 * buildOrder -> sortByOrder -> activeSession フォールバック処理を共通ヘルパーに抽出する。
 *
 * S-18: この関数は現在 module-private (export なし) である。テスト容易性のために
 * export を検討可能だが、Zustand store の内部ロジックであるため、store 全体を
 * 通したインテグレーションテスト (setSessions / applySessionDelta 経由) が推奨される。
 * 単体テストが必要な場合は export して直接テスト可能。
 */
function resolveSessionState(
    next: SessionSnapshot[],
    prevOrder: string[],
    prevActiveSession: string | null,
): {
    sessions: SessionSnapshot[];
    sessionOrder: string[];
    activeSession: string | null;
    activeWindowId: string | null
} {
    const order = buildOrder(next, prevOrder);
    const sessions = sortByOrder(next, order);
    const activeSession =
        prevActiveSession && sessions.some((s) => s.name === prevActiveSession)
            ? prevActiveSession
            : sessions[0]?.name ?? null;
    // C-02: Resolve active window ID from the active session snapshot.
    const activeSessionSnapshot = activeSession
        ? sessions.find((s) => s.name === activeSession) ?? null
        : null;
    const activeWindowId = resolveActiveWindowId(activeSessionSnapshot);
    return {sessions, sessionOrder: order, activeSession, activeWindowId};
}

export const useTmuxStore = create<TmuxState>((set) => ({
    config: null,
    sessions: [],
    sessionOrder: [],
    activeSession: null,
    activeWindowId: null,
    zoomPaneId: null,
    pendingPrefixKillPaneId: null,
    prefixMode: false,
    syncInputMode: false,
    fontSize: 13,
    setConfig: (config) => set({config}),
    setSessions: (sessions) =>
        set((state) => resolveSessionState(sessions, state.sessionOrder, state.activeSession)),
    applySessionDelta: (upserts, removed) =>
        set((state) => {
            const byName = new Map(state.sessions.map((session) => [session.name, session]));
            // SUG-13: Runtime guard — filter out undefined/null entries in removed array.
            // If the backend sends a malformed delta with undefined items, String(undefined)
            // would produce a "undefined" key, silently corrupting the session map.
            for (const name of removed) {
                if (typeof name !== "string") {
                    console.warn("[DEBUG-tmuxStore] applySessionDelta: invalid removed item filtered out", name);
                    continue;
                }
                byName.delete(name);
            }
            // I-25: Runtime guard — filter out null/undefined items and items with
            // non-string name. Without this, session.name of undefined would produce
            // a Map key of "undefined", corrupting the session map.
            for (const session of upserts) {
                if (session == null || typeof session.name !== "string") {
                    console.warn("[DEBUG-tmuxStore] applySessionDelta: invalid upsert item filtered out", session);
                    continue;
                }
                byName.set(session.name, session);
            }
            return resolveSessionState(
                Array.from(byName.values()),
                state.sessionOrder,
                state.activeSession,
            );
        }),
    // I-19: セッション変更時に activeWindowId も同期更新する。
    // setSessions / applySessionDelta と同様に resolveSessionState が行う
    // activeWindowId 解決ロジックを再利用する。
    setActiveSession: (activeSession) =>
        set((state) => {
            // activeSession が null の場合、activeWindowId もクリアする。
            if (activeSession == null) {
                return {activeSession: null, activeWindowId: null};
            }
            const sessionSnapshot = state.sessions.find((s) => s.name === activeSession) ?? null;
            const activeWindowId = resolveActiveWindowId(sessionSnapshot);
            return {activeSession, activeWindowId};
        }),
    // S-22: null または空文字は null に正規化する。型安全のため string 以外は無視する。
    setActiveWindowId: (windowId) =>
        set({activeWindowId: typeof windowId === "string" && windowId.trim() !== "" ? windowId : null}),
    setZoomPaneId: (zoomPaneId) => set({zoomPaneId}),
    setPendingPrefixKillPaneId: (pendingPrefixKillPaneId) => set({pendingPrefixKillPaneId}),
    setPrefixMode: (prefixMode) => set({prefixMode}),
    setSyncInputMode: (syncInputMode) => set({syncInputMode}),
    toggleSyncInputMode: () => set((state) => ({syncInputMode: !state.syncInputMode})),
    setFontSize: (fontSize) => set({fontSize}),
    reorderSession: (fromIndex, toIndex) =>
        set((state) => {
            const order = [...state.sessionOrder];
            if (fromIndex < 0 || fromIndex >= order.length || toIndex < 0 || toIndex >= order.length) {
                return state;
            }
            const [moved] = order.splice(fromIndex, 1);
            order.splice(toIndex, 0, moved);
            const sorted = sortByOrder(state.sessions, order);
            return {sessions: sorted, sessionOrder: order};
        }),
}));
