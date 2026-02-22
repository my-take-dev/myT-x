import {useEffect, useMemo, useRef, useState} from "react";
import "@xterm/xterm/css/xterm.css";
import {api} from "./api";
import {ConfirmDialog} from "./components/ConfirmDialog";
import {MenuBar} from "./components/MenuBar";
import {QuickSearch} from "./components/QuickSearch";
import {SessionView} from "./components/SessionView";
import {SettingsModal} from "./components/SettingsModal";
import {Sidebar} from "./components/Sidebar";
import {StatusBar} from "./components/StatusBar";
import {ToastContainer} from "./components/ToastContainer";
import {ViewerSystem} from "./components/viewer";
import {useBackendSync} from "./hooks/useBackendSync";
import {useFileDrop} from "./hooks/useFileDrop";
import {usePrefixKeyMode} from "./hooks/usePrefixKeyMode";
import {useTmuxStore} from "./stores/tmuxStore";
import {isImeTransitionalEvent} from "./utils/ime";
import {resolveActivePaneID} from "./utils/session";

function App() {
  useBackendSync();
  const [quickSearchOpen, setQuickSearchOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const lastSyncedSessionRef = useRef<string | null>(null);

  const sessions = useTmuxStore((s) => s.sessions);
  const activeSession = useTmuxStore((s) => s.activeSession);
  const pendingPrefixKillPaneId = useTmuxStore((s) => s.pendingPrefixKillPaneId);
  const setPendingPrefixKillPaneId = useTmuxStore((s) => s.setPendingPrefixKillPaneId);

  const current = useMemo(
    () => sessions.find((session) => session.name === activeSession) ?? sessions[0] ?? null,
    [activeSession, sessions],
  );

  const activePaneId = useMemo(() => resolveActivePaneID(current), [current]);
  usePrefixKeyMode({activePaneId});
  useFileDrop(activePaneId);

  useEffect(() => {
    const currentSessionName = current?.name ?? null;
    if (currentSessionName === null) {
      lastSyncedSessionRef.current = null;
      return;
    }
    if (lastSyncedSessionRef.current === currentSessionName) {
      return;
    }
    lastSyncedSessionRef.current = currentSessionName;
    void api.SetActiveSession(currentSessionName).catch((err) => {
      lastSyncedSessionRef.current = null;
      console.warn("[app] SetActiveSession failed", err);
    });
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
      <MenuBar onOpenSettings={() => setSettingsOpen(true)}/>
      <div className="app-body">
        <Sidebar sessions={sessions} activeSession={current?.name ?? null}/>
        <main className="main-content">
          <SessionView session={current}/>
          <StatusBar/>
        </main>
        <ViewerSystem />
      </div>
      <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)}/>
      <ToastContainer/>
      <QuickSearch open={quickSearchOpen} onClose={() => setQuickSearchOpen(false)}/>
      <ConfirmDialog
        open={pendingPrefixKillPaneId !== null}
        title="Close pane"
        message={pendingPrefixKillPaneId ? `Close pane "${pendingPrefixKillPaneId}"?` : ""}
        actions={[{label: "Close", value: "close", variant: "danger"}]}
        onAction={(value) => {
          if (value !== "close") {
            setPendingPrefixKillPaneId(null);
            return;
          }
          const paneID = pendingPrefixKillPaneId;
          setPendingPrefixKillPaneId(null);
          if (!paneID) {
            return;
          }
          void api.KillPane(paneID).catch((err) => {
            console.warn("[prefix] kill pane failed", err);
          });
        }}
        onClose={() => setPendingPrefixKillPaneId(null)}
      />
    </div>
  );
}

export default App;
