import {useEffect, useRef, useState} from "react";
import type {FormEvent, ReactNode} from "react";

interface EditorDialogShellProps {
    readonly ariaLabel: string;
    readonly children: ReactNode;
    readonly onCancel: () => void;
    readonly title: string;
}

interface CreateDialogProps {
    readonly errorMessage: string | null;
    readonly parentPath: string;
    readonly type: "directory" | "file";
    readonly onCancel: () => void;
    readonly onConfirm: (name: string) => void | Promise<void>;
}

interface RenameDialogProps {
    readonly currentName: string;
    readonly errorMessage: string | null;
    readonly onCancel: () => void;
    readonly onConfirm: (name: string) => void | Promise<void>;
}

interface DeleteDialogProps {
    readonly errorMessage: string | null;
    readonly isDir: boolean;
    readonly name: string;
    readonly onCancel: () => void;
    readonly onConfirm: () => void | Promise<void>;
}

interface DiscardChangesDialogProps {
    readonly onCancel: () => void;
    readonly onConfirm: () => void | Promise<void>;
}

function validateEntryName(name: string): string | null {
    const trimmed = name.trim();
    if (trimmed === "") {
        return "A name is required.";
    }
    if (trimmed === "." || trimmed.includes("..") || trimmed.includes("/") || trimmed.includes("\\") || trimmed.includes("\0")) {
        return "Name cannot contain path separators, \"..\", or null characters.";
    }
    return null;
}

function EditorDialogShell({ariaLabel, children, onCancel, title}: EditorDialogShellProps) {
    const previousFocusRef = useRef<Element | null>(null);

    useEffect(() => {
        previousFocusRef.current = document.activeElement;
        const handleEscape = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                event.preventDefault();
                onCancel();
            }
        };
        document.addEventListener("keydown", handleEscape);
        return () => {
            document.removeEventListener("keydown", handleEscape);
            if (previousFocusRef.current instanceof HTMLElement) {
                previousFocusRef.current.focus();
            }
        };
    }, [onCancel]);

    return (
        <div className="editor-dialog-overlay" onClick={onCancel}>
            <div
                className="editor-dialog"
                role="dialog"
                aria-label={ariaLabel}
                aria-modal="true"
                onClick={(event) => event.stopPropagation()}
            >
                <h3 className="editor-dialog-title">{title}</h3>
                {children}
            </div>
        </div>
    );
}

export function CreateDialog({errorMessage, parentPath, type, onCancel, onConfirm}: CreateDialogProps) {
    const [value, setValue] = useState("");
    const [localError, setLocalError] = useState<string | null>(null);
    const inputRef = useRef<HTMLInputElement>(null);

    useEffect(() => {
        inputRef.current?.focus();
    }, []);

    const handleSubmit = async (event: FormEvent) => {
        event.preventDefault();
        const validationError = validateEntryName(value);
        if (validationError) {
            setLocalError(validationError);
            return;
        }
        setLocalError(null);
        await onConfirm(value.trim());
    };

    const parentLabel = parentPath === "" ? "/" : parentPath;
    const resolvedError = localError ?? errorMessage;

    return (
        <EditorDialogShell
            ariaLabel={`Create ${type}`}
            onCancel={onCancel}
            title={type === "file" ? "Create File" : "Create Folder"}
        >
            <form onSubmit={(event) => { void handleSubmit(event); }}>
                <p className="editor-dialog-message">Parent: <span className="editor-dialog-path">{parentLabel}</span></p>
                <input
                    ref={inputRef}
                    className="editor-dialog-input"
                    type="text"
                    value={value}
                    onChange={(event) => {
                        setValue(event.target.value);
                        if (localError) {
                            setLocalError(null);
                        }
                    }}
                    placeholder={type === "file" ? "example.txt" : "new-folder"}
                />
                {resolvedError && <p className="editor-dialog-error">{resolvedError}</p>}
                <div className="editor-dialog-actions">
                    <button type="button" className="editor-dialog-btn editor-dialog-btn--secondary" onClick={onCancel}>Cancel</button>
                    <button type="submit" className="editor-dialog-btn editor-dialog-btn--primary">Create</button>
                </div>
            </form>
        </EditorDialogShell>
    );
}

export function RenameDialog({currentName, errorMessage, onCancel, onConfirm}: RenameDialogProps) {
    const [value, setValue] = useState(currentName);
    const [localError, setLocalError] = useState<string | null>(null);
    const inputRef = useRef<HTMLInputElement>(null);

    useEffect(() => {
        inputRef.current?.focus();
        inputRef.current?.select();
    }, []);

    const handleSubmit = async (event: FormEvent) => {
        event.preventDefault();
        const validationError = validateEntryName(value);
        if (validationError) {
            setLocalError(validationError);
            return;
        }
        setLocalError(null);
        await onConfirm(value.trim());
    };

    const resolvedError = localError ?? errorMessage;

    return (
        <EditorDialogShell ariaLabel="Rename item" onCancel={onCancel} title="Rename">
            <form onSubmit={(event) => { void handleSubmit(event); }}>
                <input
                    ref={inputRef}
                    className="editor-dialog-input"
                    type="text"
                    value={value}
                    onChange={(event) => {
                        setValue(event.target.value);
                        if (localError) {
                            setLocalError(null);
                        }
                    }}
                />
                {resolvedError && <p className="editor-dialog-error">{resolvedError}</p>}
                <div className="editor-dialog-actions">
                    <button type="button" className="editor-dialog-btn editor-dialog-btn--secondary" onClick={onCancel}>Cancel</button>
                    <button type="submit" className="editor-dialog-btn editor-dialog-btn--primary">Rename</button>
                </div>
            </form>
        </EditorDialogShell>
    );
}

export function DeleteDialog({errorMessage, isDir, name, onCancel, onConfirm}: DeleteDialogProps) {
    const confirmButtonRef = useRef<HTMLButtonElement>(null);

    useEffect(() => {
        confirmButtonRef.current?.focus();
    }, []);

    return (
        <EditorDialogShell ariaLabel="Delete item" onCancel={onCancel} title={isDir ? "Delete Folder" : "Delete File"}>
            <p className="editor-dialog-message">
                Delete <span className="editor-dialog-path">{name}</span>?
            </p>
            <p className="editor-dialog-warning">This action cannot be undone.</p>
            {errorMessage && <p className="editor-dialog-error">{errorMessage}</p>}
            <div className="editor-dialog-actions">
                <button type="button" className="editor-dialog-btn editor-dialog-btn--secondary" onClick={onCancel}>Cancel</button>
                <button
                    ref={confirmButtonRef}
                    type="button"
                    className="editor-dialog-btn editor-dialog-btn--danger"
                    onClick={() => { void onConfirm(); }}
                >
                    Delete
                </button>
            </div>
        </EditorDialogShell>
    );
}

export function DiscardChangesDialog({onCancel, onConfirm}: DiscardChangesDialogProps) {
    const confirmButtonRef = useRef<HTMLButtonElement>(null);

    useEffect(() => {
        confirmButtonRef.current?.focus();
    }, []);

    return (
        <EditorDialogShell ariaLabel="Discard unsaved changes" onCancel={onCancel} title="Unsaved Changes">
            <p className="editor-dialog-message">
                Discard unsaved changes?
            </p>
            <p className="editor-dialog-warning">Your current edits will be lost.</p>
            <div className="editor-dialog-actions">
                <button type="button" className="editor-dialog-btn editor-dialog-btn--secondary" onClick={onCancel}>Cancel</button>
                <button
                    ref={confirmButtonRef}
                    type="button"
                    className="editor-dialog-btn editor-dialog-btn--danger"
                    onClick={() => { void onConfirm(); }}
                >
                    Discard
                </button>
            </div>
        </EditorDialogShell>
    );
}
