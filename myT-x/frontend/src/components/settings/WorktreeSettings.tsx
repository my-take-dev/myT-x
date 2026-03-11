import type {FormDispatch, FormState} from "./types";
import {DynamicStringList} from "./DynamicStringList";
import {useSettingsI18n} from "./settingsI18n";

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

export function WorktreeSettings({s, dispatch}: WorktreeSettingsProps) {
    const {t} = useSettingsI18n();
    const copyFileItemErrors = pickIndexedValidationErrors(s.validationErrors, "wt_copy_files_");
    const copyDirItemErrors = pickIndexedValidationErrors(s.validationErrors, "wt_copy_dirs_");

    return (
        <div className="settings-section">
            <div className="settings-section-title">{t("settings.worktree.title", "Worktree", "Worktree")}</div>

            <div className="form-checkbox-row">
                <input
                    type="checkbox"
                    id="wt-enabled"
                    checked={s.wtEnabled}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "wtEnabled", value: e.target.checked})}
                />
                <label htmlFor="wt-enabled">{t("settings.worktree.enabled.label", "有効化", "Enable")}</label>
            </div>
            <span className="settings-desc">
                {t(
                    "settings.worktree.enabled.description",
                    "Git worktreeを利用したセッション作成を有効化",
                    "Enable session creation using Git worktree.",
                )}
            </span>

            <div className="form-checkbox-row" style={{marginTop: 8}}>
                <input
                    type="checkbox"
                    id="wt-force-cleanup"
                    checked={s.wtForceCleanup}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "wtForceCleanup", value: e.target.checked})}
                />
                <label htmlFor="wt-force-cleanup">{t("settings.worktree.forceCleanup.label", "強制削除", "Force cleanup")}</label>
            </div>
            <span className="settings-desc">
                {t(
                    "settings.worktree.forceCleanup.description",
                    "worktree削除時に未コミット変更があっても強制削除する（データ損失の可能性あり）",
                    "Force-remove worktree even with uncommitted changes (may cause data loss).",
                )}
            </span>

            <div className="form-group" style={{marginTop: 10}}>
                <label className="form-label">{t("settings.worktree.setupScripts.label", "セットアップスクリプト", "Setup scripts")}</label>
                <span className="settings-desc">
                    {t(
                        "settings.worktree.setupScripts.description",
                        "worktree作成後に自動実行するスクリプト（上から順に実行、各5分タイムアウト。最初の失敗で中止）",
                        "Scripts executed after worktree creation (top-down, 5-minute timeout each, stops on first failure).",
                    )}
                </span>
                <DynamicStringList
                    items={s.wtSetupScripts}
                    onChange={(items) => dispatch({type: "SET_FIELD", field: "wtSetupScripts", value: items})}
                    placeholder={t(
                        "settings.worktree.setupScripts.placeholderExample",
                        "例: npm install",
                        "e.g. npm install",
                    )}
                    addLabel={t("settings.worktree.setupScripts.add", "スクリプト追加", "Add script")}
                />
            </div>

            <div className="form-group" style={{marginTop: 6}}>
                <label className="form-label">{t("settings.worktree.copyFiles.label", "コピーファイル", "Copy files")}</label>
                <span className="settings-desc">
                    {t(
                        "settings.worktree.copyFiles.description",
                        "リポジトリルートからworktreeにコピーするファイル（相対パスのみ。.env等のgit管理外ファイル向け）",
                        "Files copied from repo root to worktree (relative paths only, for non-git files like .env).",
                    )}
                </span>
                <DynamicStringList
                    items={s.wtCopyFiles}
                    onChange={(items) => dispatch({type: "SET_FIELD", field: "wtCopyFiles", value: items})}
                    placeholder={t("settings.worktree.copyFiles.placeholderExample", "例: .env", "e.g. .env")}
                    addLabel={t("settings.worktree.copyFiles.add", "ファイル追加", "Add file")}
                    itemErrors={copyFileItemErrors}
                />
                {s.validationErrors["wt_copy_files"] && (
                    <span className="form-error">{s.validationErrors["wt_copy_files"]}</span>
                )}
            </div>

            <div className="form-group" style={{marginTop: 6}}>
                <label className="form-label">{t("settings.worktree.copyDirs.label", "コピーディレクトリ", "Copy directories")}</label>
                <span className="settings-desc">
                    {t(
                        "settings.worktree.copyDirs.description",
                        "リポジトリルートからworktreeにコピーするディレクトリ（相対パスのみ。ディレクトリ全体を再帰的にコピー）",
                        "Directories copied from repo root to worktree (relative paths only, recursively copied).",
                    )}
                </span>
                <DynamicStringList
                    items={s.wtCopyDirs}
                    onChange={(items) => dispatch({type: "SET_FIELD", field: "wtCopyDirs", value: items})}
                    placeholder={t("settings.worktree.copyDirs.placeholderExample", "例: .vscode", "e.g. .vscode")}
                    addLabel={t("settings.worktree.copyDirs.add", "ディレクトリ追加", "Add directory")}
                    itemErrors={copyDirItemErrors}
                />
                {s.validationErrors["wt_copy_dirs"] && (
                    <span className="form-error">{s.validationErrors["wt_copy_dirs"]}</span>
                )}
            </div>
        </div>
    );
}
