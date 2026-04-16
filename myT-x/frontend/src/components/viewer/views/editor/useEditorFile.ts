import {type OnMount} from "@monaco-editor/react";
import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import type {editor as MonacoEditor} from "monaco-editor";
import {api} from "../../../../api";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {getLanguageFromPath} from "../../../../utils/monacoLanguageUtils";
import {EDITOR_CONFIG} from "./editorConstants";
import type {LoadingState} from "./editorTypes";

export interface UseEditorFileResult {
    readonly currentPath: string | null;
    readonly detectedLanguage: string;
    readonly error: string | null;
    readonly fileSize: number;
    readonly isModified: boolean;
    readonly loadingState: LoadingState;
    readonly readOnly: boolean;
    readonly truncated: boolean;
    readonly clearFile: () => void;
    readonly handleChange: (value: string | undefined) => void;
    readonly handleEditorMount: OnMount;
    readonly loadFile: (path: string) => Promise<void>;
    readonly saveFile: () => Promise<boolean>;
}

export function useEditorFile(): UseEditorFileResult {
    const sessions = useTmuxStore((state) => state.sessions);
    const activeSession = useTmuxStore((state) => state.activeSession);
    const activeSessionSnapshot = useMemo(
        () => (activeSession ? sessions.find((entry) => entry.name === activeSession) ?? null : null),
        [sessions, activeSession],
    );
    const activeSessionKey = activeSessionSnapshot ? `${activeSessionSnapshot.name}:${activeSessionSnapshot.id}` : "";

    const [currentPath, setCurrentPath] = useState<string | null>(null);
    const [loadingState, setLoadingState] = useState<LoadingState>("idle");
    const [fileSize, setFileSize] = useState(0);
    const [error, setError] = useState<string | null>(null);
    const [isModified, setIsModified] = useState(false);
    const [truncated, setTruncated] = useState(false);

    const editorRef = useRef<MonacoEditor.IStandaloneCodeEditor | null>(null);
    const isEditorReadyRef = useRef(false);
    const currentContentRef = useRef("");
    const originalContentRef = useRef("");
    const currentPathRef = useRef("");
    const abortControllerRef = useRef<AbortController | null>(null);
    const layoutTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const mountedRef = useRef(true);
    const sessionRef = useRef(activeSession);
    const latestSessionKeyRef = useRef(activeSessionKey);
    const requestIDRef = useRef(0);
    const saveRequestIDRef = useRef(0);
    const prevSessionKeyRef = useRef<string | null>(null);
    const saveFileRef = useRef<(() => Promise<boolean>) | null>(null);
    const loadedSessionKeyRef = useRef<string | null>(null);
    const loadingStateRef = useRef<LoadingState>(loadingState);
    const truncatedRef = useRef(truncated);

    loadingStateRef.current = loadingState;
    truncatedRef.current = truncated;

    const clearScheduledLayout = useCallback(() => {
        if (layoutTimeoutRef.current !== null) {
            clearTimeout(layoutTimeoutRef.current);
            layoutTimeoutRef.current = null;
        }
    }, []);

    const scheduleLayout = useCallback(() => {
        clearScheduledLayout();
        layoutTimeoutRef.current = setTimeout(() => {
            editorRef.current?.layout();
            layoutTimeoutRef.current = null;
        }, EDITOR_CONFIG.LAYOUT_DELAY_MS);
    }, [clearScheduledLayout]);

    const setEditorValue = useCallback((content: string) => {
        currentContentRef.current = content;
        if (editorRef.current && isEditorReadyRef.current) {
            editorRef.current.setValue(content);
            scheduleLayout();
            return;
        }
    }, [scheduleLayout]);

    const clearFile = useCallback(() => {
        abortControllerRef.current?.abort();
        abortControllerRef.current = null;
        requestIDRef.current += 1;
        saveRequestIDRef.current += 1;
        loadedSessionKeyRef.current = null;
        currentPathRef.current = "";
        currentContentRef.current = "";
        originalContentRef.current = "";
        setCurrentPath(null);
        setLoadingState("idle");
        setFileSize(0);
        setError(null);
        setIsModified(false);
        setTruncated(false);
        setEditorValue("");
    }, [setEditorValue]);

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
            abortControllerRef.current?.abort();
            clearScheduledLayout();
        };
    }, [clearScheduledLayout]);

    useEffect(() => {
        sessionRef.current = activeSession;
        latestSessionKeyRef.current = activeSessionKey;
    }, [activeSession, activeSessionKey]);

    useEffect(() => {
        if (prevSessionKeyRef.current === activeSessionKey) {
            return;
        }
        const previousSessionKey = prevSessionKeyRef.current;
        prevSessionKeyRef.current = activeSessionKey;
        abortControllerRef.current?.abort();
        abortControllerRef.current = null;
        requestIDRef.current += 1;
        saveRequestIDRef.current += 1;
        if (previousSessionKey !== null && currentPathRef.current !== "") {
            loadedSessionKeyRef.current = previousSessionKey;
        }
    }, [activeSessionKey]);

    const loadFile = useCallback(async (path: string) => {
        const capturedSessionKey = latestSessionKeyRef.current;
        const capturedSession = sessionRef.current?.trim();
        if (!capturedSession) {
            clearFile();
            setError("No active session.");
            setLoadingState("error");
            return;
        }
        if (!path) {
            clearFile();
            return;
        }

        abortControllerRef.current?.abort();
        const abortController = new AbortController();
        abortControllerRef.current = abortController;

        const requestID = ++requestIDRef.current;
        currentPathRef.current = path;
        setCurrentPath(path);
        setLoadingState("loading");
        setFileSize(0);
        setError(null);
        setIsModified(false);
        setTruncated(false);
        originalContentRef.current = "";
        currentContentRef.current = "";
        loadedSessionKeyRef.current = null;

        const shouldIgnore = () => (
            abortController.signal.aborted
            || !mountedRef.current
            || latestSessionKeyRef.current !== capturedSessionKey
            || sessionRef.current?.trim() !== capturedSession
            || requestIDRef.current !== requestID
            || currentPathRef.current !== path
        );

        try {
            const info = await api.DevPanelGetFileInfo(capturedSession, path);
            if (shouldIgnore()) {
                return;
            }

            setFileSize(info.size);
            if (info.is_dir) {
                setEditorValue("");
                setLoadingState("error");
                setError("Directories cannot be opened in the editor.");
                return;
            }

            const content = await api.DevPanelReadFile(capturedSession, path);
            if (shouldIgnore()) {
                return;
            }

            if (content.binary) {
                setEditorValue("");
                setLoadingState("error");
                setError("Binary files cannot be edited.");
                return;
            }

            setEditorValue(content.content);
            originalContentRef.current = content.content;
            loadedSessionKeyRef.current = capturedSessionKey;
            setTruncated(content.truncated);
            setIsModified(false);
            setError(null);
            setLoadingState(content.truncated || info.size > EDITOR_CONFIG.LARGE_FILE_THRESHOLD ? "preview" : "loaded");
        } catch (err: unknown) {
            if (shouldIgnore()) {
                return;
            }
            setEditorValue("");
            setError(toErrorMessage(err, `Failed to load file: ${path}`));
            setLoadingState("error");
            setIsModified(false);
            setTruncated(false);
        }
    }, [clearFile, setEditorValue]);

    const saveFile = useCallback(async (): Promise<boolean> => {
        const capturedSessionKey = latestSessionKeyRef.current;
        const capturedSession = sessionRef.current?.trim();
        const capturedPath = currentPathRef.current;

        if (!capturedSession) {
            setError("No active session.");
            return false;
        }
        if (!capturedSessionKey) {
            setError("Active session key is unavailable.");
            return false;
        }
        if (!capturedPath) {
            setError("No file selected.");
            return false;
        }
        if (loadedSessionKeyRef.current !== null && loadedSessionKeyRef.current !== latestSessionKeyRef.current) {
            setError("The open file no longer belongs to the active session.");
            return false;
        }
        if (loadingStateRef.current !== "loaded" || truncatedRef.current) {
            setError("Only fully loaded text files can be saved.");
            return false;
        }
        if (!editorRef.current) {
            setError("Editor is not ready.");
            return false;
        }

        const requestID = ++saveRequestIDRef.current;
        const value = editorRef.current.getValue();
        currentContentRef.current = value;
        const shouldIgnore = () => (
            !mountedRef.current
            || latestSessionKeyRef.current !== capturedSessionKey
            || sessionRef.current?.trim() !== capturedSession
            || saveRequestIDRef.current !== requestID
            || currentPathRef.current !== capturedPath
        );

        try {
            setError(null);
            const result = await api.DevPanelWriteFile(capturedSessionKey, capturedPath, value);
            if (shouldIgnore()) {
                return false;
            }

            originalContentRef.current = value;
            currentContentRef.current = value;
            setFileSize(result.size);
            setIsModified(false);
            setTruncated(false);
            setLoadingState("loaded");
            return true;
        } catch (err: unknown) {
            if (shouldIgnore()) {
                return false;
            }

            const message = toErrorMessage(err, `Failed to save file: ${capturedPath}`);
            setError(message);
            return false;
        }
    }, []);

    useEffect(() => {
        saveFileRef.current = saveFile;
    }, [saveFile]);

    const handleEditorMount: OnMount = useCallback((editor, monaco) => {
        editorRef.current = editor;
        isEditorReadyRef.current = true;
        editor.setValue(currentContentRef.current);
        scheduleLayout();

        editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
            const currentSave = saveFileRef.current;
            if (!currentSave) {
                return;
            }
            void currentSave();
        });

        editor.focus();
    }, [scheduleLayout]);

    const handleChange = useCallback((value: string | undefined) => {
        if (value === undefined) {
            return;
        }
        currentContentRef.current = value;
        setIsModified(value !== originalContentRef.current);
    }, []);

    const detectedLanguage = useMemo(() => {
        if (!currentPath) {
            return "plaintext";
        }
        return getLanguageFromPath(currentPath);
    }, [currentPath]);

    return {
        currentPath,
        detectedLanguage,
        error,
        fileSize,
        isModified,
        loadingState,
        readOnly: loadingState !== "loaded" || currentPath === null,
        truncated,
        clearFile,
        handleChange,
        handleEditorMount,
        loadFile,
        saveFile,
    };
}
