import type { FormDispatch, FormState } from "./types";
import { DynamicStringList } from "./DynamicStringList";

interface WorktreeSettingsProps {
  s: FormState;
  dispatch: FormDispatch;
}

function pickIndexedValidationErrors(
  validationErrors: Record<string, string>,
  keyPrefix: string,
): Record<number, string> | undefined {
  const entries = Object.entries(validationErrors).filter(([key]) => key.startsWith(keyPrefix));
  if (entries.length === 0) {
    return undefined;
  }

  const itemErrors: Record<number, string> = {};
  for (const [key, message] of entries) {
    const rawIndex = key.slice(keyPrefix.length);
    const index = Number.parseInt(rawIndex, 10);
    if (!Number.isInteger(index) || index < 0) {
      continue;
    }
    itemErrors[index] = message;
  }
  return Object.keys(itemErrors).length > 0 ? itemErrors : undefined;
}

export function WorktreeSettings({ s, dispatch }: WorktreeSettingsProps) {
  const copyFileItemErrors = pickIndexedValidationErrors(s.validationErrors, "wt_copy_files_");
  const copyDirItemErrors = pickIndexedValidationErrors(s.validationErrors, "wt_copy_dirs_");

  return (
    <div className="settings-section">
      <div className="settings-section-title">Worktree</div>

      <div className="form-checkbox-row">
        <input
          type="checkbox"
          id="wt-enabled"
          checked={s.wtEnabled}
          onChange={(e) => dispatch({ type: "SET_FIELD", field: "wtEnabled", value: e.target.checked })}
        />
        <label htmlFor="wt-enabled">有効化</label>
      </div>
      <span className="settings-desc">
        Git worktreeを利用したセッション作成を有効化
      </span>

      <div className="form-checkbox-row" style={{ marginTop: 8 }}>
        <input
          type="checkbox"
          id="wt-force-cleanup"
          checked={s.wtForceCleanup}
          onChange={(e) => dispatch({ type: "SET_FIELD", field: "wtForceCleanup", value: e.target.checked })}
        />
        <label htmlFor="wt-force-cleanup">強制削除</label>
      </div>
      <span className="settings-desc">
        worktree削除時に未コミット変更があっても強制削除する（データ損失の可能性あり）
      </span>

      <div className="form-group" style={{ marginTop: 10 }}>
        <label className="form-label">セットアップスクリプト</label>
        <span className="settings-desc">
          worktree作成後に自動実行するスクリプト（上から順に実行、各5分タイムアウト。最初の失敗で中止）
        </span>
        <DynamicStringList
          items={s.wtSetupScripts}
          onChange={(items) => dispatch({ type: "SET_FIELD", field: "wtSetupScripts", value: items })}
          placeholder="例: npm install"
          addLabel="スクリプト追加"
        />
      </div>

      <div className="form-group" style={{ marginTop: 6 }}>
        <label className="form-label">コピーファイル</label>
        <span className="settings-desc">
          リポジトリルートからworktreeにコピーするファイル（相対パスのみ。.env等のgit管理外ファイル向け）
        </span>
        <DynamicStringList
          items={s.wtCopyFiles}
          onChange={(items) => dispatch({ type: "SET_FIELD", field: "wtCopyFiles", value: items })}
          placeholder="例: .env"
          addLabel="ファイル追加"
          itemErrors={copyFileItemErrors}
        />
        {s.validationErrors["wt_copy_files"] && (
          <span className="form-error">{s.validationErrors["wt_copy_files"]}</span>
        )}
      </div>

      <div className="form-group" style={{ marginTop: 6 }}>
        <label className="form-label">コピーディレクトリ</label>
        <span className="settings-desc">
          リポジトリルートからworktreeにコピーするディレクトリ（相対パスのみ。ディレクトリ全体を再帰的にコピー）
        </span>
        <DynamicStringList
          items={s.wtCopyDirs}
          onChange={(items) => dispatch({ type: "SET_FIELD", field: "wtCopyDirs", value: items })}
          placeholder="例: .vscode"
          addLabel="ディレクトリ追加"
          itemErrors={copyDirItemErrors}
        />
        {s.validationErrors["wt_copy_dirs"] && (
          <span className="form-error">{s.validationErrors["wt_copy_dirs"]}</span>
        )}
      </div>
    </div>
  );
}
