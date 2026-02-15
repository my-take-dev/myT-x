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
  const timerRef = useRef<number | null>(null);

  useEffect(() => {
    const clearPrefixTimer = () => {
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };

    const armPrefixTimer = () => {
      clearPrefixTimer();
      timerRef.current = window.setTimeout(() => setPrefixMode(false), 1200);
    };

    const handle = (event: KeyboardEvent) => {
      if (isImeTransitionalEvent(event)) {
        return;
      }
      if (event.ctrlKey && (event.key === "b" || event.key === "B")) {
        event.preventDefault();
        setPrefixMode(true);
        armPrefixTimer();
        return;
      }

      if (!prefixMode) {
        return;
      }

      event.preventDefault();
      clearPrefixTimer();
      setPrefixMode(false);

      const paneId = options.activePaneId;
      if (!paneId) {
        return;
      }

      const currentSession = sessions.find((session) => session.name === activeSession);
      const panes = currentSession?.windows[0]?.panes ?? [];
      const currentIndex = panes.findIndex((pane) => pane.id === paneId);

      const key = event.key;
      if (key === "%") {
        void api.SplitPane(paneId, true);
        return;
      }
      if (key === '"') {
        void api.SplitPane(paneId, false);
        return;
      }
      if (key === "z" || key === "Z") {
        setZoomPaneId(zoomPaneId === paneId ? null : paneId);
        return;
      }
      if (key === "s" || key === "S") {
        useTmuxStore.getState().toggleSyncInputMode();
        return;
      }
      if (key === "x" || key === "X") {
        void api.KillPane(paneId);
        return;
      }
      if (key === "d" || key === "D") {
        if (activeSession) {
          void api.DetachSession(activeSession);
        }
        return;
      }
      if ((key === "ArrowLeft" || key === "ArrowUp") && panes.length > 0) {
        const nextIndex = Math.max(0, currentIndex - 1);
        const target = panes[nextIndex];
        if (target) {
          void api.FocusPane(target.id);
        }
        return;
      }
      if ((key === "ArrowRight" || key === "ArrowDown") && panes.length > 0) {
        const nextIndex = Math.min(panes.length - 1, currentIndex + 1);
        const target = panes[nextIndex];
        if (target) {
          void api.FocusPane(target.id);
        }
        return;
      }
    };

    window.addEventListener("keydown", handle);
    return () => {
      window.removeEventListener("keydown", handle);
      clearPrefixTimer();
    };
  }, [activeSession, options.activePaneId, prefixMode, sessions, setPrefixMode, setZoomPaneId, zoomPaneId]);
}
