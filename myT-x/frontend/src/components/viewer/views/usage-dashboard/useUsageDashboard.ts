import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {GetUsageDashboard} from "../../../../../wailsjs/go/main/App";
import {usagedashboard} from "../../../../../wailsjs/go/models";
import {useTmuxStore} from "../../../../stores/tmuxStore";

export type UsageSource = "claude" | "codex";
export type UsageDashboardSelection = UsageSource | "compare";
// Must match internal/usagedashboard.ModeBoth; the current React view asks
// for the full snapshot and performs source filtering locally.
export const COMBINED_USAGE_MODE = "both" as const;

interface UseUsageDashboardResult {
    snapshot: usagedashboard.UsageDashboardSnapshot | null;
    isLoading: boolean;
    error: string | null;
    hasActiveSession: boolean;
    activeSessionName: string;
    /**
     * Trigger a fetch. Pass `force=true` to bypass the on-disk JSON cache
     * (the "Refresh" button uses this); omit or pass `false` for automatic
     * loads that should reuse a fresh cached snapshot when available.
     */
    refresh: (force?: boolean) => void;
}

/**
 * useUsageDashboard fetches aggregated usage statistics for the currently
 * active tmux session. The hook always fetches the combined snapshot so
 * view-only source selection can switch without another Wails IPC call.
 *
 * The hook implements defensive-coding-checklist #186 post-await session guard:
 * the active session key (name:id) is captured before the Wails IPC call and
 * compared afterwards so state updates from stale requests are dropped when
 * the user has switched sessions during the roundtrip.
 */
export function useUsageDashboard(): UseUsageDashboardResult {
    const activeSession = useTmuxStore((s) => s.activeSession);
    const sessions = useTmuxStore((s) => s.sessions);
    const activeSessionSnapshot = useMemo(
        () => (activeSession ? sessions.find((s) => s.name === activeSession) ?? null : null),
        [sessions, activeSession],
    );
    const activeSessionKey = activeSessionSnapshot
        ? `${activeSessionSnapshot.name}:${activeSessionSnapshot.id}`
        : activeSession
            ? `${activeSession}:`
            : "";

    const [snapshot, setSnapshot] = useState<usagedashboard.UsageDashboardSnapshot | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const isMountedRef = useRef(false);
    const latestSessionKeyRef = useRef(activeSessionKey);
    const pendingRequestCountRef = useRef(0);

    useEffect(() => {
        isMountedRef.current = true;
        return () => {
            isMountedRef.current = false;
        };
    }, []);

    useEffect(() => {
        latestSessionKeyRef.current = activeSessionKey;
    }, [activeSessionKey]);

    const refresh = useCallback((force: boolean = false) => {
        if (!activeSession) {
            setSnapshot(null);
            setError(null);
            pendingRequestCountRef.current = 0;
            setIsLoading(false);
            return;
        }
        const capturedSessionKey = activeSessionKey;
        pendingRequestCountRef.current += 1;
        setIsLoading(true);
        void GetUsageDashboard(activeSession, COMBINED_USAGE_MODE, force)
            .then((result) => {
                if (!isMountedRef.current) return;
                if (capturedSessionKey !== latestSessionKeyRef.current) return;
                setSnapshot(usagedashboard.UsageDashboardSnapshot.createFrom(result));
                setError(null);
            })
            .catch((err: unknown) => {
                if (!isMountedRef.current) return;
                if (capturedSessionKey !== latestSessionKeyRef.current) return;
                setError(err instanceof Error ? err.message : String(err));
            })
            .finally(() => {
                // Loading cleanup must remain independent from the session
                // guard, but it also needs to respect newer in-flight requests.
                // Track outstanding requests so a stale completion cannot clear
                // the spinner for a newer fetch that is still running.
                if (!isMountedRef.current) return;
                pendingRequestCountRef.current = Math.max(0, pendingRequestCountRef.current - 1);
                setIsLoading(pendingRequestCountRef.current > 0);
            });
    }, [activeSession, activeSessionKey]);

    // Auto-refresh when the callback identity changes because the session
    // changed. force=false so the on-disk JSON cache is reused when fresh.
    useEffect(() => {
        refresh(false);
    }, [refresh]);

    return {
        snapshot,
        isLoading,
        error,
        hasActiveSession: Boolean(activeSession),
        activeSessionName: activeSession ?? "",
        refresh,
    };
}
