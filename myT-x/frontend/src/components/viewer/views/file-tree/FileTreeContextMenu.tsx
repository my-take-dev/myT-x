import {useEffect, useLayoutEffect, useRef, useState} from "react";
import {ClipboardSetText} from "../../../../../wailsjs/runtime/runtime";
import type {FlatNode} from "./fileTreeTypes";

interface FileTreeContextMenuProps {
    x: number;
    y: number;
    node: FlatNode;
    onClose: () => void;
}

export function FileTreeContextMenu({x, y, node, onClose}: FileTreeContextMenuProps) {
    const menuRef = useRef<HTMLDivElement>(null);
    const [adjustedPos, setAdjustedPos] = useState({x, y});
    const previousFocusRef = useRef<Element | null>(null);

    // Save the previously focused element before mount
    useEffect(() => {
        previousFocusRef.current = document.activeElement;
        return () => {
            // Restore focus on unmount
            if (previousFocusRef.current instanceof HTMLElement) {
                previousFocusRef.current.focus();
            }
        };
    }, []);

    // Auto-focus first menu item on mount
    useEffect(() => {
        if (menuRef.current) {
            const firstItem = menuRef.current.querySelector<HTMLElement>('[role="menuitem"]');
            firstItem?.focus();
        }
    }, []);

    // Adjust position to stay within viewport (useLayoutEffect to prevent flicker)
    useLayoutEffect(() => {
        if (menuRef.current) {
            const rect = menuRef.current.getBoundingClientRect();
            let newX = x;
            let newY = y;
            if (x + rect.width > window.innerWidth) {
                newX = window.innerWidth - rect.width - 4;
            }
            if (y + rect.height > window.innerHeight) {
                newY = window.innerHeight - rect.height - 4;
            }
            newX = Math.max(0, newX);
            newY = Math.max(0, newY);
            setAdjustedPos({x: newX, y: newY});
        }
    }, [x, y]);

    // Close on click outside, Escape, or scroll
    useEffect(() => {
        let mounted = true;
        const handleClickOutside = (e: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
                onClose();
            }
        };
        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === "Escape") {
                e.stopPropagation();
                onClose();
            }
        };
        const handleWheel = () => {
            onClose();
        };
        // Defer listener registration to avoid immediate close from the same click
        const timerId = setTimeout(() => {
            if (mounted) {
                document.addEventListener("mousedown", handleClickOutside);
                document.addEventListener("keydown", handleEscape);
                document.addEventListener("wheel", handleWheel, {passive: true});
            }
        }, 0);
        return () => {
            mounted = false;
            clearTimeout(timerId);
            document.removeEventListener("mousedown", handleClickOutside);
            document.removeEventListener("keydown", handleEscape);
            document.removeEventListener("wheel", handleWheel);
        };
    }, [onClose]);

    // Keyboard navigation between menu items
    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
        e.preventDefault();
        const menu = menuRef.current;
        if (!menu) return;
        const items = Array.from(menu.querySelectorAll<HTMLElement>('[role="menuitem"]'));
        if (items.length === 0) return;
        const currentIndex = items.indexOf(document.activeElement as HTMLElement);
        let nextIndex: number;
        if (e.key === "ArrowDown") {
            nextIndex = currentIndex < items.length - 1 ? currentIndex + 1 : 0;
        } else {
            nextIndex = currentIndex > 0 ? currentIndex - 1 : items.length - 1;
        }
        items[nextIndex].focus();
    };

    const handleCopyPath = () => {
        ClipboardSetText(node.path).catch((err: unknown) =>
            console.warn("[FileTreeContextMenu] clipboard write failed", err),
        );
        onClose();
    };

    return (
        <div
            ref={menuRef}
            className="file-tree-context-menu"
            role="menu"
            aria-label="ファイルツリーコンテキストメニュー"
            style={{left: adjustedPos.x, top: adjustedPos.y}}
            onKeyDown={handleKeyDown}
        >
            <button
                className="file-tree-context-menu-item"
                role="menuitem"
                onClick={handleCopyPath}
            >
                パスのコピー
            </button>
        </div>
    );
}
