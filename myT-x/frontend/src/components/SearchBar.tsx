import {useEffect, useRef, useState} from "react";
import type {SearchAddon} from "@xterm/addon-search";

interface SearchBarProps {
    open: boolean;
    onClose: () => void;
    searchAddon: SearchAddon | null;
}

/**
 * searchAddon に対する操作は try-catch で保護する。
 *
 * TerminalPane のクリーンアップで searchAddonRef.current が null 化されるが、
 * ペインの再マウント前後の短い窓で SearchBar がまだマウントされており、
 * 操作が呼ばれると dispose 済みの addon にアクセスしてクラッシュする可能性がある。
 * null チェック + try-catch により disposed な addon への操作を安全に無視する。
 */
function safeAddonOp(addon: SearchAddon | null, op: (a: SearchAddon) => void): void {
    if (!addon) return;
    try {
        op(addon);
    } catch (err) {
        console.warn("[DEBUG-search] searchAddon operation failed (possibly disposed)", err);
    }
}

export function SearchBar({open, onClose, searchAddon}: SearchBarProps) {
    const inputRef = useRef<HTMLInputElement | null>(null);
    const [query, setQuery] = useState("");

    useEffect(() => {
        if (open) {
            // Clear the previous query and decorations every time the bar reopens
            // so users start fresh rather than seeing stale search state.
            setQuery("");
            safeAddonOp(searchAddon, (a) => a.clearDecorations());
            inputRef.current?.focus();
            inputRef.current?.select();
        }
    }, [open, searchAddon]);

    if (!open) return null;

    const findNext = () => {
        if (!query) return;
        safeAddonOp(searchAddon, (a) =>
            a.findNext(query, {caseSensitive: false, regex: false}),
        );
    };

    const findPrev = () => {
        if (!query) return;
        safeAddonOp(searchAddon, (a) =>
            a.findPrevious(query, {caseSensitive: false, regex: false}),
        );
    };

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === "Escape") {
            e.preventDefault();
            // I-25: 上位ハンドラ（usePrefixKeyMode 等）への不要な伝播を防止する。
            e.stopPropagation();
            safeAddonOp(searchAddon, (a) => a.clearDecorations());
            onClose();
            return;
        }
        if (e.key === "Enter") {
            e.preventDefault();
            e.stopPropagation();
            if (e.shiftKey) {
                findPrev();
            } else {
                findNext();
            }
        }
    };

    return (
        <div className="terminal-search-bar">
            <input
                ref={inputRef}
                className="terminal-search-input"
                type="text"
                placeholder="Search..."
                value={query}
                onChange={(e) => {
                    const value = e.target.value;
                    setQuery(value);
                    if (value) {
                        safeAddonOp(searchAddon, (a) =>
                            a.findNext(value, {caseSensitive: false, regex: false}),
                        );
                    }
                }}
                onKeyDown={handleKeyDown}
            />
            <button
                type="button"
                className="terminal-search-btn"
                title="Previous (Shift+Enter)"
                onClick={findPrev}
            >
                &#x25B2;
            </button>
            <button
                type="button"
                className="terminal-search-btn"
                title="Next (Enter)"
                onClick={findNext}
            >
                &#x25BC;
            </button>
            <button
                type="button"
                className="terminal-search-btn terminal-search-btn-close"
                title="Close (Esc)"
                onClick={() => {
                    safeAddonOp(searchAddon, (a) => a.clearDecorations());
                    onClose();
                }}
            >
                &times;
            </button>
        </div>
    );
}
