import {lazy, Suspense, type ReactElement} from "react";
import type {OnMount} from "@monaco-editor/react";
import {formatFileSize} from "../file-tree/treeUtils";
import {MONACO_OPTIONS} from "./editorConstants";
import type {LoadingState} from "./editorTypes";

const MonacoEditor = lazy(() => import("@monaco-editor/react"));

interface EditorPaneProps {
    readonly currentPath: string | null;
    readonly detectedLanguage: string;
    readonly error: string | null;
    readonly fileSize: number;
    readonly isModified: boolean;
    readonly loadingState: LoadingState;
    readonly readOnly: boolean;
    readonly truncated: boolean;
    readonly onChange: (value: string | undefined) => void;
    readonly onEditorMount: OnMount;
    readonly onSave: () => Promise<boolean>;
}

export function EditorPane({
                               currentPath,
                               detectedLanguage,
                               error,
                               fileSize,
                               isModified,
                               loadingState,
                               readOnly,
                               truncated,
                               onChange,
                               onEditorMount,
                               onSave,
                           }: EditorPaneProps) {
    const fileName = currentPath?.split("/").pop() ?? "No file selected";
    const showOverlay = loadingState === "loading" || error !== null || currentPath === null;

    return (
        <div className="editor-pane">
            <div className="editor-toolbar">
                <div className="editor-toolbar-main">
                    <span className="editor-toolbar-file">{fileName}</span>
                    {isModified && <span className="editor-toolbar-dirty" title="Unsaved changes">*</span>}
                </div>
                <div className="editor-toolbar-meta">
                    {currentPath && <span className="editor-toolbar-path" title={currentPath}>{currentPath}</span>}
                    <span className="editor-toolbar-language">{detectedLanguage}</span>
                    {currentPath && <span className="editor-toolbar-size">{formatFileSize(fileSize)}</span>}
                </div>
                <div className="editor-toolbar-spacer"/>
                <button
                    type="button"
                    className="editor-toolbar-btn editor-toolbar-btn--primary"
                    disabled={!isModified || readOnly || currentPath === null}
                    onClick={() => {
                        void onSave();
                    }}
                >
                    Save
                </button>
            </div>
            {truncated && (
                <div className="editor-toolbar-warning">
                    Read-only preview. This file exceeds 1 MB and has been truncated.
                </div>
            )}
            <div className="editor-monaco-wrapper">
                <Suspense fallback={editorLoadingOverlay()}>
                    <MonacoEditor
                        language={detectedLanguage}
                        theme="vs-dark"
                        onMount={onEditorMount}
                        onChange={onChange}
                        options={{
                            ...MONACO_OPTIONS,
                            readOnly,
                        }}
                    />
                </Suspense>
                {showOverlay && (
                    <div className={`editor-overlay${error ? " editor-overlay--error" : ""}`}>
                        <div className="editor-overlay-message">
                            {loadingState === "loading"
                                ? "Loading file..."
                                : error ?? "Select a file to edit."}
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}

function editorLoadingOverlay(): ReactElement {
    return (
        <div className="editor-overlay">
            <div className="editor-overlay-message">Loading editor...</div>
        </div>
    );
}
