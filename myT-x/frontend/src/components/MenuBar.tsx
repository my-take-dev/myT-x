interface MenuBarProps {
  onOpenSettings: () => void;
}

export function MenuBar({ onOpenSettings }: MenuBarProps) {
  return (
    <nav className="menu-bar">
      <div className="menu-bar-item">
        <button className="menu-bar-trigger" onClick={onOpenSettings}>
          設定
        </button>
      </div>
    </nav>
  );
}
