import { useEffect, useState } from "react";
import { api } from "../api";
import { useTmuxStore } from "../stores/tmuxStore";

export function StatusBar() {
  const [statusLine, setStatusLine] = useState("[セッションなし] | --:--");
  const prefixMode = useTmuxStore((s) => s.prefixMode);
  const syncInputMode = useTmuxStore((s) => s.syncInputMode);

  useEffect(() => {
    let mounted = true;

    const refresh = async () => {
      try {
        const line = await api.BuildStatusLine();
        if (mounted) {
          setStatusLine(line);
        }
      } catch {
        // no-op
      }
    };

    void refresh();
    const timer = window.setInterval(() => {
      void refresh();
    }, 1000);

    return () => {
      mounted = false;
      window.clearInterval(timer);
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
