interface DiscardConfirmDialogProps {
    filePath: string;
    onConfirm: () => void;
    onCancel: () => void;
}

export function DiscardConfirmDialog({filePath, onConfirm, onCancel}: DiscardConfirmDialogProps) {
    return (
        <div className="discard-dialog-overlay" onClick={onCancel}>
            <div
                className="discard-dialog"
                role="alertdialog"
                aria-label="Confirm discard"
                onClick={(e) => e.stopPropagation()}
            >
                <p className="discard-dialog-message">
                    Discard changes to <strong>{filePath}</strong>?
                </p>
                <p className="discard-dialog-warning">
                    This cannot be undone.
                </p>
                <div className="discard-dialog-actions">
                    <button
                        type="button"
                        className="discard-dialog-btn discard-dialog-btn--cancel"
                        onClick={onCancel}
                    >
                        Cancel
                    </button>
                    <button
                        type="button"
                        className="discard-dialog-btn discard-dialog-btn--confirm"
                        onClick={onConfirm}
                        autoFocus
                    >
                        Discard
                    </button>
                </div>
            </div>
        </div>
    );
}
