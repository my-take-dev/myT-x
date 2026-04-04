import {useEffect, useLayoutEffect, useRef, useState} from "react";
import type {KeyboardEvent as ReactKeyboardEvent} from "react";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {writeClipboardText} from "../../../../utils/clipboardUtils";
import {notifyClipboardFailure} from "../../../../utils/notifyUtils";
import type {FlatNode} from "../file-tree/fileTreeTypes";
import {buildAbsoluteEditorPath} from "./editorPathUtils";

interface EditorContextMenuProps {
    readonly node: FlatNode;
    readonly x: number;
    readonly y: number;
    readonly onClose: () => void;
    readonly onCreateDirectory: () => void;
    readonly onCreateFile: () => void;
    readonly onDelete: () => void;
    readonly onRename: () => void;
}

export function EditorContextMenu({
    node,
    x,
    y,
    onClose,
    onCreateDirectory,
    onCreateFile,
    onDelete,
    onRename,
}: EditorContextMenuProps) {
    const menuRef = useRef<HTMLDivElement>(null);
    const [adjustedPos, setAdjustedPos] = useState({x, y});
    const previousFocusRef = useRef<Element | null>(null);
    const activeSessionRoot = useTmuxStore((state) => {
        if (!state.activeSession) {
            return "";
        }
        const session = state.sessions.find((entry) => entry.name === state.activeSession);
        return session?.worktree?.path?.trim() || session?.root_path?.trim() || "";
    });
    const absolutePath = buildAbsoluteEditorPath(activeSessionRoot, node.path);
    const copyPathLabel = activeSessionRoot ? "Copy Path" : "Copy Windows Path";

    useEffect(() => {
        previousFocusRef.current = document.activeElement;
        return () => {
            if (previousFocusRef.current instanceof HTMLElement) {
                previousFocusRef.current.focus();
            }
        };
    }, []);

    useEffect(() => {
        const firstItem = menuRef.current?.querySelector<HTMLElement>("[role=\"menuitem\"]");
        firstItem?.focus();
    }, []);

    useLayoutEffect(() => {
        if (!menuRef.current) {
            return;
        }

        const rect = menuRef.current.getBoundingClientRect();
        let nextX = x;
        let nextY = y;
        if (x + rect.width > window.innerWidth) {
            nextX = window.innerWidth - rect.width - 4;
        }
        if (y + rect.height > window.innerHeight) {
            nextY = window.innerHeight - rect.height - 4;
        }
        setAdjustedPos({
            x: Math.max(0, nextX),
            y: Math.max(0, nextY),
        });
    }, [x, y]);

    useEffect(() => {
        let mounted = true;
        const handleClickOutside = (event: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
                onClose();
            }
        };
        const handleEscape = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                event.stopPropagation();
                onClose();
            }
        };
        const handleWheel = () => {
            onClose();
        };

        const timerID = setTimeout(() => {
            if (!mounted) {
                return;
            }
            document.addEventListener("mousedown", handleClickOutside);
            document.addEventListener("keydown", handleEscape);
            document.addEventListener("wheel", handleWheel, {passive: true});
        }, 0);

        return () => {
            mounted = false;
            clearTimeout(timerID);
            document.removeEventListener("mousedown", handleClickOutside);
            document.removeEventListener("keydown", handleEscape);
            document.removeEventListener("wheel", handleWheel);
        };
    }, [onClose]);

    const handleKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
        if (event.key !== "ArrowDown" && event.key !== "ArrowUp") {
            return;
        }

        event.preventDefault();
        const items = Array.from(menuRef.current?.querySelectorAll<HTMLElement>("[role=\"menuitem\"]") ?? []);
        if (items.length === 0) {
            return;
        }

        const currentIndex = items.indexOf(document.activeElement as HTMLElement);
        const nextIndex = event.key === "ArrowDown"
            ? (currentIndex + 1 + items.length) % items.length
            : (currentIndex - 1 + items.length) % items.length;
        items[nextIndex]?.focus();
    };

    const runAction = (action: () => void | Promise<void>) =>
        Promise.resolve(action()).finally(onClose);

    const handleCopy = async (text: string) => {
        try {
            await writeClipboardText(text);
        } catch (err: unknown) {
            notifyClipboardFailure();
            console.warn("[editor-context-menu] clipboard write failed", {path: node.path, err});
        }
    };

    return (
        <div
            ref={menuRef}
            className="editor-context-menu"
            role="menu"
            aria-label={`Editor actions for ${node.name}`}
            style={{left: adjustedPos.x, top: adjustedPos.y}}
            onKeyDown={handleKeyDown}
        >
            <button
                type="button"
                className="editor-context-menu-item"
                role="menuitem"
                onClick={() => { void runAction(async () => handleCopy(absolutePath)); }}
            >
                {copyPathLabel}
            </button>
            <button
                type="button"
                className="editor-context-menu-item"
                role="menuitem"
                onClick={() => { void runAction(async () => handleCopy(node.path)); }}
            >
                Copy Relative Path
            </button>
            <div className="editor-context-menu-divider" />
            <button
                type="button"
                className="editor-context-menu-item"
                role="menuitem"
                onClick={() => { void runAction(onCreateFile); }}
            >
                New File
            </button>
            <button
                type="button"
                className="editor-context-menu-item"
                role="menuitem"
                onClick={() => { void runAction(onCreateDirectory); }}
            >
                New Folder
            </button>
            <div className="editor-context-menu-divider" />
            <button
                type="button"
                className="editor-context-menu-item"
                role="menuitem"
                onClick={() => { void runAction(onRename); }}
            >
                Rename
            </button>
            <button
                type="button"
                className="editor-context-menu-item editor-context-menu-item--danger"
                role="menuitem"
                onClick={() => { void runAction(onDelete); }}
            >
                Delete
            </button>
        </div>
    );
}
