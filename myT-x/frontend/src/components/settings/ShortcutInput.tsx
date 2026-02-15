import { useState } from "react";

interface ShortcutInputProps {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  disabled?: boolean;
  ariaLabel?: string;
}

export function ShortcutInput({ value, onChange, placeholder, disabled, ariaLabel }: ShortcutInputProps) {
  const [capturing, setCapturing] = useState(false);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      e.nativeEvent.stopImmediatePropagation();
      setCapturing(false);
      return;
    }
    if (e.key === "Tab") {
      setCapturing(false);
      return;
    }

    e.preventDefault();
    e.stopPropagation();

    // 修飾キー単体は無視
    if (["Control", "Shift", "Alt", "Meta"].includes(e.key)) return;

    const parts: string[] = [];
    if (e.ctrlKey) parts.push("Ctrl");
    if (e.shiftKey) parts.push("Shift");
    if (e.altKey) parts.push("Alt");

    // キー名の正規化
    let keyName = e.key;
    if (keyName === " ") keyName = "Space";
    if (keyName.length === 1) keyName = keyName.toUpperCase();
    parts.push(keyName);

    onChange(parts.join("+"));
    setCapturing(false);
  };

  return (
    <input
      className={`form-input shortcut-input ${capturing ? "capturing" : ""}`}
      value={capturing ? "" : value}
      placeholder={capturing ? "キーを押してください..." : placeholder}
      data-escape-close-disabled={capturing ? "true" : undefined}
      onFocus={() => !disabled && setCapturing(true)}
      onBlur={() => setCapturing(false)}
      onKeyDown={capturing ? handleKeyDown : undefined}
      readOnly
      disabled={disabled}
      aria-label={ariaLabel || placeholder || "Shortcut input"}
    />
  );
}
