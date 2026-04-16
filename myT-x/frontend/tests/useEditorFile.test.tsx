import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import type {editor as MonacoEditor} from "monaco-editor";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const apiMock = vi.hoisted(() => ({
    DevPanelGetFileInfo: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    DevPanelReadFile: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    DevPanelWriteFile: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

let mockActiveSession: string | null = "test-session";
let mockSessions = [{id: 1, name: "test-session"}];

vi.mock("../src/api", () => ({
    api: {
        DevPanelGetFileInfo: (...args: unknown[]) => apiMock.DevPanelGetFileInfo(...args),
        DevPanelReadFile: (...args: unknown[]) => apiMock.DevPanelReadFile(...args),
        DevPanelWriteFile: (...args: unknown[]) => apiMock.DevPanelWriteFile(...args),
    },
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: {
        activeSession: string | null;
        sessions: Array<{ id: number; name: string }>
    }) => unknown) =>
        selector({activeSession: mockActiveSession, sessions: mockSessions}),
}));

import {useEditorFile, type UseEditorFileResult} from "../src/components/viewer/views/editor/useEditorFile";

let hookResult: UseEditorFileResult | null = null;

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return {promise, resolve, reject};
}

function EditorFileProbe() {
    hookResult = useEditorFile();
    return null;
}

interface StubEditorState {
    value: string;
}

function createEditorStub(initialValue: string): {
    readonly editor: MonacoEditor.IStandaloneCodeEditor;
    readonly state: StubEditorState;
    readonly setValue: ReturnType<typeof vi.fn>;
} {
    const state: StubEditorState = {value: initialValue};
    const setValue = vi.fn((value: string) => {
        state.value = value;
    });
    const editor = {
        setValue,
        getValue: vi.fn(() => state.value),
        layout: vi.fn(),
        addCommand: vi.fn(),
        focus: vi.fn(),
        dispose: vi.fn(),
    } as unknown as MonacoEditor.IStandaloneCodeEditor;

    return {editor, state, setValue};
}

const monacoStub = {
    KeyCode: {KeyS: 1},
    KeyMod: {CtrlCmd: 2},
} as const;

describe("useEditorFile", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        hookResult = null;
        mockActiveSession = "test-session";
        mockSessions = [{id: 1, name: "test-session"}];
        apiMock.DevPanelGetFileInfo.mockReset();
        apiMock.DevPanelReadFile.mockReset();
        apiMock.DevPanelWriteFile.mockReset();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("restores the latest saved buffer when the editor mounts again", async () => {
        const firstEditor = createEditorStub("");
        const remountedEditor = createEditorStub("");

        apiMock.DevPanelGetFileInfo.mockResolvedValue({
            size: 4,
            is_dir: false,
        });
        apiMock.DevPanelReadFile.mockResolvedValue({
            path: "src/app.ts",
            content: "seed",
            line_count: 1,
            size: 4,
            truncated: false,
            binary: false,
        });
        apiMock.DevPanelWriteFile.mockResolvedValue({
            path: "src/app.ts",
            size: 6,
        });

        act(() => {
            root.render(<EditorFileProbe/>);
        });

        act(() => {
            hookResult!.handleEditorMount(
                firstEditor.editor,
                monacoStub as unknown as Parameters<UseEditorFileResult["handleEditorMount"]>[1],
            );
        });

        await act(async () => {
            await hookResult!.loadFile("src/app.ts");
        });

        expect(firstEditor.setValue).toHaveBeenCalledWith("seed");

        firstEditor.state.value = "edited";
        act(() => {
            hookResult!.handleChange("edited");
        });
        expect(hookResult!.isModified).toBe(true);

        await act(async () => {
            await expect(hookResult!.saveFile()).resolves.toBe(true);
        });

        expect(hookResult!.isModified).toBe(false);

        act(() => {
            hookResult!.handleEditorMount(
                remountedEditor.editor,
                monacoStub as unknown as Parameters<UseEditorFileResult["handleEditorMount"]>[1],
            );
        });

        expect(remountedEditor.setValue).toHaveBeenCalledWith("edited");
        expect(remountedEditor.state.value).toBe("edited");
    });

    it("keeps saveFile stable after loading changes the editor state", async () => {
        apiMock.DevPanelGetFileInfo.mockResolvedValue({
            size: 4,
            is_dir: false,
        });
        apiMock.DevPanelReadFile.mockResolvedValue({
            path: "src/app.ts",
            content: "seed",
            line_count: 1,
            size: 4,
            truncated: false,
            binary: false,
        });

        act(() => {
            root.render(<EditorFileProbe/>);
        });

        const initialSaveFile = hookResult!.saveFile;

        await act(async () => {
            await hookResult!.loadFile("src/app.ts");
        });

        expect(hookResult!.loadingState).toBe("loaded");
        expect(hookResult!.saveFile).toBe(initialSaveFile);
    });

    it("treats binary files as read-only and rejects saves", async () => {
        apiMock.DevPanelGetFileInfo.mockResolvedValue({
            size: 4,
            is_dir: false,
        });
        apiMock.DevPanelReadFile.mockResolvedValue({
            path: "src/data.bin",
            content: "",
            line_count: 0,
            size: 4,
            truncated: false,
            binary: true,
        });

        act(() => {
            root.render(<EditorFileProbe/>);
        });

        await act(async () => {
            await hookResult!.loadFile("src/data.bin");
        });

        expect(hookResult!.loadingState).toBe("error");
        expect(hookResult!.readOnly).toBe(true);
        await act(async () => {
            await expect(hookResult!.saveFile()).resolves.toBe(false);
        });
        expect(hookResult!.error).toBe("Only fully loaded text files can be saved.");
    });

    it("treats directories as read-only and rejects saves", async () => {
        apiMock.DevPanelGetFileInfo.mockResolvedValue({
            size: 0,
            is_dir: true,
        });

        act(() => {
            root.render(<EditorFileProbe/>);
        });

        await act(async () => {
            await hookResult!.loadFile("src");
        });

        expect(hookResult!.loadingState).toBe("error");
        expect(hookResult!.readOnly).toBe(true);
        await act(async () => {
            await expect(hookResult!.saveFile()).resolves.toBe(false);
        });
        expect(hookResult!.error).toBe("Only fully loaded text files can be saved.");
    });

    it("ignores stale load results after the active session changes until the parent view clears the buffer", async () => {
        const fileInfo = deferred<{ size: number; is_dir: boolean }>();
        apiMock.DevPanelGetFileInfo.mockReturnValueOnce(fileInfo.promise);
        apiMock.DevPanelReadFile.mockResolvedValue({
            path: "src/app.ts",
            content: "stale",
            line_count: 1,
            size: 5,
            truncated: false,
            binary: false,
        });

        act(() => {
            root.render(<EditorFileProbe/>);
        });

        let firstLoad!: Promise<void>;
        act(() => {
            firstLoad = hookResult!.loadFile("src/app.ts");
        });

        mockActiveSession = "other-session";
        act(() => {
            root.render(<EditorFileProbe/>);
        });

        await act(async () => {
            fileInfo.resolve({size: 5, is_dir: false});
            await fileInfo.promise;
        });
        await act(async () => {
            await firstLoad;
        });

        expect(hookResult!.currentPath).toBe("src/app.ts");
        expect(hookResult!.loadingState).toBe("loading");
        expect(hookResult!.error).toBeNull();
    });

    it("blocks saves when the active session is recreated with the same name", async () => {
        apiMock.DevPanelGetFileInfo.mockResolvedValue({
            size: 4,
            is_dir: false,
        });
        apiMock.DevPanelReadFile.mockResolvedValue({
            path: "src/app.ts",
            content: "seed",
            line_count: 1,
            size: 4,
            truncated: false,
            binary: false,
        });

        act(() => {
            root.render(<EditorFileProbe/>);
        });

        await act(async () => {
            await hookResult!.loadFile("src/app.ts");
        });
        expect(hookResult!.currentPath).toBe("src/app.ts");

        mockSessions = [{id: 99, name: "test-session"}];
        act(() => {
            root.render(<EditorFileProbe/>);
        });

        expect(hookResult!.currentPath).toBe("src/app.ts");
        expect(hookResult!.loadingState).toBe("loaded");
        await act(async () => {
            await expect(hookResult!.saveFile()).resolves.toBe(false);
        });
        expect(hookResult!.error).toBe("The open file no longer belongs to the active session.");
    });
});
