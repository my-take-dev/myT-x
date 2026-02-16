import { useEffect } from "react";
import { OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime";
import { api } from "../api";

/** ファイルパスをシェル安全にクォートする */
function quotePathForShell(path: string): string {

  const escaped = path.replace(/[`$]/g, "`$&");
  return `"${escaped}"`;
}

/**
 * Wails OnFileDrop をグローバルに1回だけ登録し、
 * ドロップされたファイルパスをアクティブペインに送信する。
 *
 * OnFileDrop はグローバルシングルトンのため、
 * 各TerminalPaneではなくApp等の親コンポーネントで呼ぶ。
 */
export function useFileDrop(activePaneId: string | null) {
  useEffect(() => {
    OnFileDrop((_x: number, _y: number, paths: string[]) => {
      if (paths.length === 0 || !activePaneId) return;
      const quoted = paths.map(quotePathForShell).join(" ");
      void api.SendInput(activePaneId, quoted).catch((err) => {
        console.warn("[file-drop] SendInput failed", err);
      });
    }, true);

    return () => {
      OnFileDropOff();
    };
  }, [activePaneId]);
}
