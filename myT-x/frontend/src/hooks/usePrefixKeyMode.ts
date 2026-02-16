import { useEffect, useRef } from "react";
import { api } from "../api";
import { useTmuxStore } from "../stores/tmuxStore";
import { isImeTransitionalEvent } from "../utils/ime";

interface UsePrefixKeyModeOptions {
  activePaneId: string | null;
}

export function usePrefixKeyMode(options: UsePrefixKeyModeOptions) {
  const sessions = useTmuxStore((s) => s.sessions);
  const activeSession = useTmuxStore((s) => s.activeSession);
  const prefixMode = useTmuxStore((s) => s.prefixMode);
  const setPrefixMode = useTmuxStore((s) => s.setPrefixMode);
  const zoomPaneId = useTmuxStore((s) => s.zoomPaneId);
  const setZoomPaneId = useTmuxStore((s) => s.setZoomPaneId);
  const toggleSyncInputMode = useTmuxStore((s) => s.toggleSyncInputMode);
  const timerRef = useRef<number | null>(null);
  const sessionsRef = useRef(sessions);
  const activeSessionRef = useRef(activeSession);
  const prefixModeRef = useRef(prefixMode);
  const zoomPaneIdRef = useRef(zoomPaneId);

  useEffect(() => {
    sessionsRef.current = sessions;
  }, [sessions]);

  useEffect(() => {
    activeSessionRef.current = activeSession;
  }, [activeSession]);

  useEffect(() => {
    prefixModeRef.current = prefixMode;
  }, [prefixMode]);

  useEffect(() => {
    zoomPaneIdRef.current = zoomPaneId;
  }, [zoomPaneId]);

  useEffect(() => {
    const clearPrefixTimer = () => {
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };

    const armPrefixTimer = () => {
      clearPrefixTimer();
      timerRef.current = window.setTimeout(() => {
        prefixModeRef.current = false;
        setPrefixMode(false);
      }, 1200);
    };

    const handle = (event: KeyboardEvent) => {
      if (isImeTransitionalEvent(event)) {
        return;
      }
      if (event.ctrlKey && (event.key === "b" || event.key === "B")) {
        event.preventDefault();
        prefixModeRef.current = true;
        setPrefixMode(true);
        armPrefixTimer();
        return;
      }

      if (!prefixModeRef.current) {
        return;
      }

      event.preventDefault();
      clearPrefixTimer();
      prefixModeRef.current = false;
      setPrefixMode(false);

      const paneId = options.activePaneId;
      if (!paneId) {
        return;
      }

      const currentSession = sessionsRef.current.find(
        (session) => session.name === activeSessionRef.current,
      );
      const panes = currentSession?.windows[0]?.panes ?? [];
      const currentIndex = panes.findIndex((pane) => pane.id === paneId);

      const key = event.key;
      if (key === "%") {
        void api.SplitPane(paneId, true).catch((err) => {
          console.warn("[prefix] split vertical failed", err);
        });
        return;
      }
      if (key === '"') {
        void api.SplitPane(paneId, false).catch((err) => {
          console.warn("[prefix] split horizontal failed", err);
        });
        return;
      }
      if (key === "z" || key === "Z") {
        const currentZoomPaneId = zoomPaneIdRef.current;
        setZoomPaneId(currentZoomPaneId === paneId ? null : paneId);
        return;
      }
      if (key === "s" || key === "S") {
        toggleSyncInputMode();
        return;
      }
      if (key === "x" || key === "X") {
        void api.KillPane(paneId).catch((err) => {
          console.warn("[prefix] kill pane failed", err);
        });
        return;
      }
      if (key === "d" || key === "D") {
        const currentActiveSession = activeSessionRef.current;
        if (currentActiveSession) {
          void api.DetachSession(currentActiveSession).catch((err) => {
            console.warn("[prefix] detach session failed", err);
          });
        }
        return;
      }
      if ((key === "ArrowLeft" || key === "ArrowUp") && panes.length > 0) {
        const nextIndex = Math.max(0, currentIndex - 1);
        const target = panes[nextIndex];
        if (target) {
          void api.FocusPane(target.id).catch((err) => {
            console.warn("[prefix] focus pane failed", err);
          });
        }
        return;
      }
      if ((key === "ArrowRight" || key === "ArrowDown") && panes.length > 0) {
        const nextIndex = Math.min(panes.length - 1, currentIndex + 1);
        const target = panes[nextIndex];
        if (target) {
          void api.FocusPane(target.id).catch((err) => {
            console.warn("[prefix] focus pane failed", err);
          });
        }
        return;
      }
    };

    window.addEventListener("keydown", handle);
    return () => {
      window.removeEventListener("keydown", handle);
      clearPrefixTimer();
    };
  }, [options.activePaneId, setPrefixMode, setZoomPaneId, toggleSyncInputMode]);
}
