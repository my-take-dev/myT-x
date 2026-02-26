import {useState} from "react";
import {isFunctionKeyToken} from "../viewer/viewerShortcutUtils";

interface ShortcutInputProps {
    id?: string;
    value: string;
    onChange: (v: string) => void;
    placeholder?: string;
    disabled?: boolean;
    ariaLabel?: string;
}

export function ShortcutInput({id, value, onChange, placeholder, disabled, ariaLabel}: ShortcutInputProps) {
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
            e.stopPropagation();
            setCapturing(false);
            return;
        }

        e.preventDefault();
        e.stopPropagation();

        // 修飾キー単体は無視（no-op）。キャプチャは継続する。
        // 非修飾キー押下（ショートカット確定）、Escape/Tab（キャンセル）、blur で終了。
        if (["Control", "Shift", "Alt", "Meta"].includes(e.key)) {
            return;
        }

        const parts: string[] = [];
        if (e.ctrlKey) parts.push("Ctrl");
        if (e.shiftKey) parts.push("Shift");
        if (e.altKey) parts.push("Alt");
        if (e.metaKey) parts.push("Meta");

        // キー名の正規化
        let keyName = e.key;
        if (keyName === " ") keyName = "Space";
        if (/^f(?:[1-9]|1[0-9]|2[0-4])$/i.test(keyName)) keyName = keyName.toUpperCase();
        if (keyName.length === 1) keyName = keyName.toUpperCase();
        if (parts.length === 0 && !isFunctionKeyToken(keyName.toLowerCase())) {
            // No modifier and not a function key: treat as invalid shortcut input.
            setCapturing(false);
            return;
        }
        parts.push(keyName);

        onChange(parts.join("+"));
        setCapturing(false);
    };

    return (
        <div className="shortcut-input-wrapper">
            <input
                id={id}
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
            {!disabled && (
                <span className="shortcut-input-hint">
                    修飾キー（Ctrl/Shift/Alt）+ キー、またはファンクションキー
                </span>
            )}
        </div>
    );
}
