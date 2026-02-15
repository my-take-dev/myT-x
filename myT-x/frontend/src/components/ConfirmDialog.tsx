import { useCallback, useEffect } from "react";

interface ConfirmAction {
  label: string;
  value: string;
  variant?: "primary" | "danger" | "ghost";
}

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  actions: ConfirmAction[];
  onAction: (value: string) => void;
  onClose: () => void;
}

export function ConfirmDialog({ open, title, message, actions, onAction, onClose }: ConfirmDialogProps) {
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
      }
    },
    [onClose],
  );

  useEffect(() => {
    if (!open) return;
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open, handleKeyDown]);

  if (!open) return null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>{title}</h2>
        </div>
        <div className="modal-body">
          <p style={{ margin: 0, fontSize: "0.88rem" }}>{message}</p>
        </div>
        <div className="modal-footer">
          <button type="button" className="modal-btn" onClick={onClose}>
            キャンセル
          </button>
          {actions.map((action) => (
            <button
              key={action.value}
              type="button"
              className={`modal-btn ${action.variant || ""}`}
              onClick={() => onAction(action.value)}
            >
              {action.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
