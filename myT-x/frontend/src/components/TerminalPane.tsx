import { useEffect, useMemo, useRef, useState } from "react";
import { FitAddon } from "@xterm/addon-fit";
import { SearchAddon } from "@xterm/addon-search";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { Terminal } from "@xterm/xterm";
import {
  BrowserOpenURL,
  ClipboardGetText,
  ClipboardSetText,
  EventsOff,
  EventsOn,
} from "../../wailsjs/runtime/runtime";
import { api } from "../api";
import { SearchBar } from "./SearchBar";
import { useTmuxStore } from "../stores/tmuxStore";
import { isImeTransitionalEvent } from "../utils/ime";

let webglUnavailable = false;

interface TerminalPaneProps {
  paneId: string;
  paneTitle?: string;
  active: boolean;
  onFocus: (paneId: string) => void;
  onSplitVertical: (paneId: string) => void;
  onSplitHorizontal: (paneId: string) => void;
  onToggleZoom: (paneId: string) => void;
  onKillPane: (paneId: string) => void;
  onRenamePane: (paneId: string, title: string) => void | Promise<void>;
  onSwapPane: (sourcePaneId: string, targetPaneId: string) => void | Promise<void>;
  onDetach: () => void;
}

/** クリップボードへテキストをペースト（ブラケットペースト対応） */
export function TerminalPane(props: TerminalPaneProps) {
  const setPrefixMode = useTmuxStore((s) => s.setPrefixMode);
  const syncInputModeRef = useRef(false);
  const fontSize = useTmuxStore((s) => s.fontSize);
  const setFontSize = useTmuxStore((s) => s.setFontSize);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const searchAddonRef = useRef<SearchAddon | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const skipTitleCommitRef = useRef(false);
  const [titleDraft, setTitleDraft] = useState("");
  const [titleEditing, setTitleEditing] = useState(false);
  const [renameBusy, setRenameBusy] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [scrollAtBottom, setScrollAtBottom] = useState(true);

  const paneEvent = useMemo(() => `pane:data:${props.paneId}`, [props.paneId]);
  const paneTitle = (props.paneTitle || "").trim();

  // syncInputMode をrefで追跡（term.onDataクロージャから参照するため）
  const syncInputMode = useTmuxStore((s) => s.syncInputMode);
  useEffect(() => {
    syncInputModeRef.current = syncInputMode;
  }, [syncInputMode]);

  useEffect(() => {
    if (titleEditing) {
      return;
    }
    setTitleDraft(paneTitle);
  }, [paneTitle, titleEditing]);

  const commitPaneTitle = async (): Promise<void> => {
    if (renameBusy) {
      return;
    }
    const current = paneTitle;
    const next = titleDraft.trim();
    setTitleEditing(false);
    if (next === current) {
      return;
    }
    setRenameBusy(true);
    try {
      await Promise.resolve(props.onRenamePane(props.paneId, next));
    } catch {
      setTitleDraft(current);
    } finally {
      setRenameBusy(false);
    }
  };

  useEffect(() => {
    const term = new Terminal({
      convertEol: true,
      cursorBlink: true,
      scrollback: 10000,
      scrollSensitivity: 1,
      fastScrollSensitivity: 5,
      fontFamily: `"Consolas", "JetBrains Mono", monospace`,
      fontSize,
      theme: {
        background: "#0f1b2b",
        foreground: "#dce8f4",
        cursor: "#f6d365",
        selectionBackground: "rgba(246,211,101,0.3)",
      },
      allowProposedApi: true,
    });
    const fitAddon = new FitAddon();
    const searchAddon = new SearchAddon();
    const webLinksAddon = new WebLinksAddon((_event, uri) => {
      BrowserOpenURL(uri);
    });
    term.loadAddon(fitAddon);
    term.loadAddon(searchAddon);
    term.loadAddon(webLinksAddon);
    terminalRef.current = term;
    searchAddonRef.current = searchAddon;
    fitAddonRef.current = fitAddon;
    let disposed = false;
    let rendererAddon: { dispose: () => void } | null = null;
    let rendererMode: "webgl" | "dom" = "dom";
    let resizeTimer: ReturnType<typeof window.setTimeout> | null = null;
    let lastResizeCols = -1;
    let lastResizeRows = -1;

    // --- IME composition バッファリング ---
    let isComposing = false;
    let composingOutput: string[] = [];
    let pendingResize = false;

    const flushComposedOutput = () => {
      if (composingOutput.length === 0) {
        return;
      }
      const buffered = composingOutput.join("");
      composingOutput.length = 0;
      term.write(buffered);
    };

    const setRendererMode = (next: "webgl" | "dom") => {
      if (rendererMode === next) {
        return;
      }
      rendererMode = next;
      if (import.meta.env.DEV) {
        console.debug(`[terminal-renderer] pane=${props.paneId} renderer=${next}`);
      }
    };

    const flushResize = () => {
      fitAddon.fit();
      if (term.cols === lastResizeCols && term.rows === lastResizeRows) {
        return;
      }
      lastResizeCols = term.cols;
      lastResizeRows = term.rows;
      void api.ResizePane(props.paneId, term.cols, term.rows);
    };

    const scheduleResize = () => {
      if (isComposing) {
        pendingResize = true;
        return;
      }
      if (resizeTimer !== null) {
        window.clearTimeout(resizeTimer);
      }
      resizeTimer = window.setTimeout(() => {
        resizeTimer = null;
        if (disposed) {
          return;
        }
        flushResize();
      }, 100);
    };

    if (containerRef.current) {
      term.open(containerRef.current);
      flushResize();
      term.focus();
    }

    // IME composition イベント登録（term.open() 後に textarea 利用可能）
    const compositionTextarea = term.textarea ?? null;
    const onCompositionStart = () => {
      isComposing = true;
    };
    const finishComposition = () => {
      isComposing = false;
      flushComposedOutput();
      if (pendingResize) {
        pendingResize = false;
        scheduleResize();
      }
    };
    const onCompositionEnd = () => {
      finishComposition();
    };
    // compositionend が発火しない異常時（フォーカス喪失等）の安全弁
    const onBlur = () => {
      if (isComposing) {
        finishComposition();
      }
    };
    if (compositionTextarea) {
      compositionTextarea.addEventListener("compositionstart", onCompositionStart);
      compositionTextarea.addEventListener("compositionend", onCompositionEnd);
      compositionTextarea.addEventListener("blur", onBlur);
    }

    if (!webglUnavailable) {
      void import("@xterm/addon-webgl")
        .then(({ WebglAddon }) => {
          if (disposed || webglUnavailable) {
            return;
          }
          const addon = new WebglAddon();
          rendererAddon = addon;
          term.loadAddon(addon);
          setRendererMode("webgl");
          addon.onContextLoss(() => {
            webglUnavailable = true;
            rendererAddon = null;
            addon.dispose();
            setRendererMode("dom");
            term.refresh(0, term.rows - 1);
          });
        })
        .catch((err) => {
          if (import.meta.env.DEV) {
            console.warn(`[terminal-renderer] WebGL addon failed for pane=${props.paneId}`, err);
          }
          webglUnavailable = true;
        });
    }

    void api.GetPaneReplay(props.paneId)
      .then((replay) => {
        if (replay) {
          term.write(replay);
        }
      })
      .catch((err) => {
        if (import.meta.env.DEV) {
          console.warn(`[terminal] replay load failed for pane=${props.paneId}`, err);
        }
      });

    // --- Copy on Select: 選択完了時に自動コピー（デバウンス付き） ---
    let copyOnSelectTimer: ReturnType<typeof window.setTimeout> | null = null;
    term.onSelectionChange(() => {
      if (copyOnSelectTimer !== null) window.clearTimeout(copyOnSelectTimer);
      copyOnSelectTimer = window.setTimeout(() => {
        copyOnSelectTimer = null;
        const selection = term.getSelection();
        if (selection) {
          void ClipboardSetText(selection);
        }
      }, 100);
    });

    // --- 右クリック: 選択あり→コピー / 選択なし→ペースト ---
    const termEl = term.element;
    const handleContextMenu = (e: MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();
      const selection = term.getSelection();
      if (selection) {
        void ClipboardSetText(selection);
        term.clearSelection();
      } else {
        void ClipboardGetText().then((text) => {
          if (text) {
            term.paste(text);
          }
        }).catch((err) => {
          console.error("[paste] clipboard read failed", err);
        });
      }
    };
    if (termEl) {
      termEl.addEventListener("contextmenu", handleContextMenu);
    }

    term.attachCustomKeyEventHandler((event) => {
      // Block keyboard events during IME composition to prevent double input.
      // return false = suppress xterm key handling, let browser IME handle it.
      if (isComposing || isImeTransitionalEvent(event)) {
        return false;
      }
      // Only process keydown events for shortcuts; let xterm handle keyup/keypress normally.
      if (event.type !== "keydown") {
        return true;
      }

      // Ctrl+B: tmux prefix mode
      if (event.ctrlKey && (event.key === "b" || event.key === "B")) {
        setPrefixMode(true);
        return false;
      }

      // Ctrl+F / Ctrl+Shift+F: 検索バーを開く
      if (event.ctrlKey && (event.key === "f" || event.key === "F")) {
        setSearchOpen(true);
        return false;
      }

      // Smart Ctrl+C: 選択あり→コピー、選択なし→SIGINT送信
      if (event.ctrlKey && (event.key === "c" || event.key === "C")) {
        const selection = term.getSelection();
        if (selection) {
          void ClipboardSetText(selection);
          term.clearSelection();
          return false;
        }
        return true;
      }

      // Ctrl+V: クリップボードからペースト（ブラケットペースト対応）
      if (event.ctrlKey && (event.key === "v" || event.key === "V")) {
        // Keep native paste event path and only suppress xterm key translation (^V).
        return false;
      }
      return true;
    });

    const disposable = term.onData((input) => {
      if (syncInputModeRef.current) {
        void api.SendSyncInput(props.paneId, input);
      } else {
        void api.SendInput(props.paneId, input);
      }
    });

    EventsOn(paneEvent, (data: string) => {
      if (typeof data === "string") {
        if (isComposing) {
          composingOutput.push(data);
          return;
        }
        term.write(data);
      }
    });

    // --- スクロール位置インジケータ ---
    const updateScrollState = () => {
      if (disposed) return;
      const buf = term.buffer.active;
      const atBottom = buf.viewportY >= buf.baseY;
      setScrollAtBottom(atBottom);
    };
    const scrollDisposable = term.onScroll(updateScrollState);
    const writeDisposable = term.onWriteParsed(updateScrollState);

    // --- Ctrl+ホイール: フォントサイズ変更 ---
    const mountEl = containerRef.current;
    const handleWheel = (e: WheelEvent) => {
      if (!e.ctrlKey) return;
      e.preventDefault();
      const delta = e.deltaY < 0 ? 1 : -1;
      setFontSize(Math.max(8, Math.min(32, (term.options.fontSize ?? 13) + delta)));
    };
    if (mountEl) {
      mountEl.addEventListener("wheel", handleWheel, { passive: false });
    }

    const observer = new ResizeObserver(() => {
      scheduleResize();
    });
    if (mountEl) {
      observer.observe(mountEl);
    }

    return () => {
      disposed = true;
      if (resizeTimer !== null) {
        window.clearTimeout(resizeTimer);
      }
      // IME composition クリーンアップ
      if (compositionTextarea) {
        compositionTextarea.removeEventListener("compositionstart", onCompositionStart);
        compositionTextarea.removeEventListener("compositionend", onCompositionEnd);
        compositionTextarea.removeEventListener("blur", onBlur);
      }
      isComposing = false;
      composingOutput.length = 0;
      pendingResize = false;
      if (copyOnSelectTimer !== null) window.clearTimeout(copyOnSelectTimer);

      // イベントリスナー クリーンアップ
      termEl?.removeEventListener("contextmenu", handleContextMenu);
      mountEl?.removeEventListener("wheel", handleWheel);

      rendererAddon?.dispose();
      scrollDisposable.dispose();
      writeDisposable.dispose();
      observer.disconnect();
      disposable.dispose();
      EventsOff(paneEvent);
      term.dispose();
      terminalRef.current = null;
      searchAddonRef.current = null;
      fitAddonRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [paneEvent, props.paneId]);

  // --- フォントサイズ変更の反映 ---
  useEffect(() => {
    const term = terminalRef.current;
    if (!term) return;
    term.options.fontSize = fontSize;
    fitAddonRef.current?.fit();
  }, [fontSize]);

  return (
    <div
      className={`terminal-pane ${props.active ? "active" : ""}`}
      draggable
      onDragStart={(event) => {
        event.dataTransfer.setData("text/plain", props.paneId);
      }}
      onDragOver={(event) => {
        event.preventDefault();
      }}
      onDrop={(event) => {
        event.preventDefault();
        // ファイルドロップは Wails OnFileDrop (useFileDrop hook) で処理
        if (event.dataTransfer.files.length > 0) return;
        const sourcePaneId = event.dataTransfer.getData("text/plain");
        if (sourcePaneId && sourcePaneId !== props.paneId) {
          props.onSwapPane(sourcePaneId, props.paneId);
        }
      }}
      onClick={() => props.onFocus(props.paneId)}
      onMouseDown={() => terminalRef.current?.focus()}
    >
      <div
        className="terminal-toolbar"
        draggable={false}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="terminal-toolbar-pane">
          <span className="terminal-toolbar-id">{props.paneId}</span>
          <input
            className="terminal-toolbar-title-input"
            value={titleDraft}
            placeholder="ペイン名"
            disabled={renameBusy}
            onFocus={() => setTitleEditing(true)}
            onChange={(event) => setTitleDraft(event.target.value)}
            onBlur={() => {
              if (skipTitleCommitRef.current) {
                skipTitleCommitRef.current = false;
                return;
              }
              void commitPaneTitle();
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                (event.currentTarget as HTMLInputElement).blur();
                return;
              }
              if (event.key === "Escape") {
                event.preventDefault();
                skipTitleCommitRef.current = true;
                setTitleDraft(paneTitle);
                setTitleEditing(false);
                (event.currentTarget as HTMLInputElement).blur();
              }
            }}
          />
        </div>
        <div className="terminal-toolbar-actions">
          <button
            type="button"
            className="terminal-toolbar-btn"
            draggable={false}
            title="左右分割 (Prefix: %)"
            aria-label={`Split pane ${props.paneId} left-right`}
            onClick={(event) => {
              event.stopPropagation();
              props.onFocus(props.paneId);
              props.onSplitVertical(props.paneId);
              terminalRef.current?.focus();
            }}
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
              <rect x="1" y="1" width="12" height="12" rx="1.5" />
              <line x1="7" y1="1" x2="7" y2="13" />
            </svg>
          </button>
          <button
            type="button"
            className="terminal-toolbar-btn"
            draggable={false}
            title="上下分割 (Prefix: &quot;)"
            aria-label={`Split pane ${props.paneId} top-bottom`}
            onClick={(event) => {
              event.stopPropagation();
              props.onFocus(props.paneId);
              props.onSplitHorizontal(props.paneId);
              terminalRef.current?.focus();
            }}
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
              <rect x="1" y="1" width="12" height="12" rx="1.5" />
              <line x1="1" y1="7" x2="13" y2="7" />
            </svg>
          </button>
          <button
            type="button"
            className="terminal-toolbar-btn terminal-toolbar-btn-danger terminal-toolbar-btn-close"
            draggable={false}
            title="ペインを閉じる (Prefix: x)"
            aria-label={`Close pane ${props.paneId}`}
            onClick={(event) => {
              event.stopPropagation();
              const confirmed = window.confirm(`Close pane ${props.paneId}?`);
              if (!confirmed) {
                return;
              }
              props.onKillPane(props.paneId);
            }}
          >
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.8">
              <line x1="2" y1="2" x2="10" y2="10" />
              <line x1="10" y1="2" x2="2" y2="10" />
            </svg>
          </button>
        </div>
      </div>
      <div className="terminal-pane-body">
        <SearchBar
          open={searchOpen}
          onClose={() => {
            setSearchOpen(false);
            terminalRef.current?.focus();
          }}
          searchAddon={searchAddonRef.current}
        />
        <div ref={containerRef} className="terminal-mount" />
        {!scrollAtBottom && (
          <button
            type="button"
            className="scroll-to-bottom-btn"
            title="最下部にスクロール"
            onClick={() => terminalRef.current?.scrollToBottom()}
          >
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.8">
              <polyline points="2,4 6,8 10,4" />
            </svg>
          </button>
        )}
      </div>
    </div>
  );
}

