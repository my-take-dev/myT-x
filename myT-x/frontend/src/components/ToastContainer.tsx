import { useNotificationStore } from "../stores/notificationStore";

export function ToastContainer() {
  const notifications = useNotificationStore((s) => s.notifications);
  const removeNotification = useNotificationStore((s) => s.removeNotification);

  if (notifications.length === 0) return null;

  return (
    <div className="toast-container">
      {notifications.map((n) => (
        <div key={n.id} className={`toast toast-${n.level}`}>
          <span className="toast-message">{n.message}</span>
          <button
            type="button"
            className="toast-close"
            onClick={() => removeNotification(n.id)}
            aria-label="通知を閉じる"
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
