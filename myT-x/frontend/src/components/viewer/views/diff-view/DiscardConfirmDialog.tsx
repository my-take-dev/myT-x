import {useI18n} from "../../../../i18n";

interface DiscardConfirmDialogProps {
    filePath: string;
    onConfirm: () => void;
    onCancel: () => void;
}

export function DiscardConfirmDialog({filePath, onConfirm, onCancel}: DiscardConfirmDialogProps) {
    const {t} = useI18n();

    return (
        <div className="discard-dialog-overlay" onClick={onCancel}>
            <div
                className="discard-dialog"
                role="alertdialog"
                aria-label={t("viewer.diff.discardConfirm.aria", "破棄の確認")}
                onClick={(e) => e.stopPropagation()}
            >
                <p className="discard-dialog-message">
                    {t("viewer.diff.discardConfirm.message", "{filePath} の変更を破棄しますか？", {filePath})}
                </p>
                <p className="discard-dialog-warning">
                    {t("viewer.diff.discardConfirm.warning", "元に戻せません。")}
                </p>
                <div className="discard-dialog-actions">
                    <button
                        type="button"
                        className="discard-dialog-btn discard-dialog-btn--cancel"
                        onClick={onCancel}
                    >
                        {t("viewer.diff.discardConfirm.cancel", "キャンセル")}
                    </button>
                    <button
                        type="button"
                        className="discard-dialog-btn discard-dialog-btn--confirm"
                        onClick={onConfirm}
                        autoFocus
                    >
                        {t("viewer.diff.discardConfirm.discard", "破棄")}
                    </button>
                </div>
            </div>
        </div>
    );
}
