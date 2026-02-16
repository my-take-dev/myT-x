import { useEffect, useState } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { api } from "../api";
import { useTmuxStore } from "../stores/tmuxStore";

export function StatusBar() {
  const [statusLine, setStatusLine] = useState("[セッションなし] | --:--");
  const prefixMode = useTmuxStore((s) => s.prefixMode);
  const syncInputMode = useTmuxStore((s) => s.syncInputMode);

  useEffect(() => {
    let mounted = true;
    let refreshTimer: number | null = null;
    const refreshDebounceMs = 75;

    const refresh = async () => {
      try {
        const line = await api.BuildStatusLine();
        if (mounted) {
          setStatusLine(line);
        }
      } catch (err) {
        console.warn("[status] BuildStatusLine failed", err);
      }
    };
    const requestRefresh = () => {
      if (refreshTimer !== null) {
        window.clearTimeout(refreshTimer);
      }
      refreshTimer = window.setTimeout(() => {
        refreshTimer = null;
        void refresh();
      }, refreshDebounceMs);
    };

    void refresh();
    const eventNames = [
      "tmux:snapshot",
      "tmux:snapshot-delta",
      "tmux:active-session",
      "tmux:session-detached",
    ];
    const cleanups: Array<() => void> = [];
    for (const eventName of eventNames) {
      cleanups.push(EventsOn(eventName, requestRefresh));
    }
    const timer = window.setInterval(() => {
      requestRefresh();
    }, 10000);

    return () => {
      mounted = false;
      if (refreshTimer !== null) {
        window.clearTimeout(refreshTimer);
        refreshTimer = null;
      }
      window.clearInterval(timer);
      for (const cleanup of cleanups) {
        cleanup();
      }
    };
  }, []);

  return (
    <footer className={`status-bar ${prefixMode ? "prefix" : ""} ${syncInputMode ? "sync" : ""}`}>
      <span>{statusLine}</span>
      <span style={{ display: "flex", gap: 8 }}>
        {syncInputMode ? <span className="sync-indicator">SYNC</span> : null}
        {prefixMode ? <span className="prefix-indicator">プレフィックス</span> : null}
      </span>
    </footer>
  );
}
