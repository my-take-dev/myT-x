import {useI18n} from "../../../../i18n";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {useSessionMemo} from "./useSessionMemo";

export function SessionMemoView() {
    const {t} = useI18n();
    const closeView = useViewerStore((state) => state.closeView);
    const {
        activeSession,
        content,
        loading,
        saving,
        error,
        isDirty,
        updateContent,
        refresh,
        save,
        clearError,
    } = useSessionMemo();
    const memoTitle = t("viewer.sessionMemo.title", "セッションメモ");
    const refreshTitle = t("viewer.sessionMemo.refresh", "セッションメモを再読み込み");
    const handleRefresh = () => {
        void refresh(true).catch(() => {
            // The hook already updated the user-visible error state.
        });
    };
    const handleSave = () => {
        void save();
    };

    if (!activeSession) {
        return (
            <ViewerPanelShell
                className="session-memo-view"
                title={memoTitle}
                onClose={closeView}
                message={t("viewer.sessionMemo.noSession", "編集するセッションを選択してください。")}
            />
        );
    }

    return (
        <ViewerPanelShell
            className="session-memo-view"
            title={memoTitle}
            onClose={closeView}
            onRefresh={handleRefresh}
            refreshDisabled={loading || saving}
            refreshTitle={refreshTitle}
            headerChildren={(
                <span className="session-memo-header-session" title={activeSession}>
                    {activeSession}
                </span>
            )}
        >
            {error ? (
                <div className="session-memo-error" title={error}>
                    <span>{error}</span>
                    <button type="button" onClick={clearError}>
                        {t("viewer.sessionMemo.dismiss", "閉じる")}
                    </button>
                </div>
            ) : null}
            <textarea
                className="session-memo-textarea"
                value={content}
                disabled={loading}
                onChange={(event) => updateContent(event.target.value)}
                onKeyDown={(event) => {
                    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
                        event.preventDefault();
                        if (!loading && !saving && isDirty) {
                            handleSave();
                        }
                    }
                }}
                placeholder={loading
                    ? t("viewer.sessionMemo.loading", "メモを読み込み中...")
                    : t("viewer.sessionMemo.placeholder", "このセッションのメモを入力...")}
                aria-label={t("viewer.sessionMemo.textarea", "セッションメモ")}
                spellCheck={false}
            />
            <div className="session-memo-footer">
                <span className="session-memo-status">
                    {isDirty
                        ? t("viewer.sessionMemo.unsaved", "未保存")
                        : t("viewer.sessionMemo.saved", "保存済み")}
                </span>
                <button
                    type="button"
                    className="session-memo-save"
                    disabled={loading || saving || !isDirty}
                    onClick={handleSave}
                >
                    {saving
                        ? t("viewer.sessionMemo.saving", "保存中...")
                        : t("viewer.sessionMemo.save", "保存")}
                </button>
            </div>
        </ViewerPanelShell>
    );
}
