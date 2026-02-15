import { useEffect, useMemo, useState } from "react";
import "@xterm/xterm/css/xterm.css";
import { api } from "./api";
import { MenuBar } from "./components/MenuBar";
import { QuickSearch } from "./components/QuickSearch";
import { SessionView } from "./components/SessionView";
import { SettingsModal } from "./components/SettingsModal";
import { Sidebar } from "./components/Sidebar";
import { StatusBar } from "./components/StatusBar";
import { ToastContainer } from "./components/ToastContainer";
import { useBackendSync } from "./hooks/useBackendSync";
import { useFileDrop } from "./hooks/useFileDrop";
import { usePrefixKeyMode } from "./hooks/usePrefixKeyMode";
import { useTmuxStore } from "./stores/tmuxStore";
import { isImeTransitionalEvent } from "./utils/ime";

function activePaneIdFromSession(session: ReturnType<typeof useTmuxStore.getState>["sessions"][number] | null): string | null {
  if (!session || session.windows.length === 0) {
    return null;
  }
  const window = session.windows[0];
  const active = window.panes.find((pane) => pane.active);
  return active?.id ?? window.panes[0]?.id ?? null;
}

function App() {
  useBackendSync();
  const [quickSearchOpen, setQuickSearchOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);

  const sessions = useTmuxStore((s) => s.sessions);
  const activeSession = useTmuxStore((s) => s.activeSession);

  const current = useMemo(
    () => sessions.find((session) => session.name === activeSession) ?? sessions[0] ?? null,
    [activeSession, sessions],
  );

  const activePaneId = useMemo(() => activePaneIdFromSession(current), [current]);
  usePrefixKeyMode({ activePaneId });
  useFileDrop(activePaneId);

  useEffect(() => {
    if (current?.name) {
      void api.SetActiveSession(current.name);
    }
  }, [current?.name]);

  // Ctrl+P: クイック検索パレット
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (isImeTransitionalEvent(e)) {
        return;
      }
      if (e.ctrlKey && (e.key === "p" || e.key === "P")) {
        e.preventDefault();
        setQuickSearchOpen((prev) => !prev);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  return (
    <div className="app-root">
      <MenuBar onOpenSettings={() => setSettingsOpen(true)} />
      <div className="app-body">
        <Sidebar sessions={sessions} activeSession={current?.name ?? null} />
        <main className="main-content">
          <SessionView session={current} />
          <StatusBar />
        </main>
      </div>
      <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
      <ToastContainer />
      <QuickSearch open={quickSearchOpen} onClose={() => setQuickSearchOpen(false)} />
    </div>
  );
}

export default App;
