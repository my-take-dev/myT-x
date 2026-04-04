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

vi.mock("../src/api", () => ({
    api: {
        DevPanelGetFileInfo: (...args: unknown[]) => apiMock.DevPanelGetFileInfo(...args),
        DevPanelReadFile: (...args: unknown[]) => apiMock.DevPanelReadFile(...args),
        DevPanelWriteFile: (...args: unknown[]) => apiMock.DevPanelWriteFile(...args),
    },
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: { activeSession: string | null }) => unknown) =>
        selector({activeSession: mockActiveSession}),
}));

import {useEditorFile, type UseEditorFileResult} from "../src/components/viewer/views/editor/useEditorFile";

let hookResult: UseEditorFileResult | null = null;

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
            await hookResult!.saveFile();
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
});
