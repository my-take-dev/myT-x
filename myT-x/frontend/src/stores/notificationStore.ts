import { create } from "zustand";

export interface Notification {
  id: string;
  message: string;
  level: "info" | "warn" | "error";
  timestamp: number;
}

interface NotificationState {
  notifications: Notification[];
  addNotification: (message: string, level: Notification["level"]) => void;
  removeNotification: (id: string) => void;
}

// Use timestamp-based IDs to avoid issues with HMR resetting module scope.
let nextId = Date.now();

export const useNotificationStore = create<NotificationState>((set) => ({
  notifications: [],
  addNotification: (message, level) => {
    const id = String(nextId++);
    set((state) => ({
      notifications: [...state.notifications, { id, message, level, timestamp: Date.now() }],
    }));
    // Auto-dismiss after 8 seconds.
    setTimeout(() => {
      set((state) => ({
        notifications: state.notifications.filter((n) => n.id !== id),
      }));
    }, 8000);
  },
  removeNotification: (id) =>
    set((state) => ({
      notifications: state.notifications.filter((n) => n.id !== id),
    })),
}));
