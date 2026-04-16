import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {usePromptPresetStore} from "../../../../stores/promptPresetStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {PromptPresetDraft, PromptPresetLoadResult} from "./types";
import {usePromptPresets} from "./usePromptPresets";

const loadPromptPresetsMock = vi.fn<(sessionName: string) => Promise<unknown>>();
const savePromptPresetMock = vi.fn<(payload: unknown, sessionName: string) => Promise<void>>();
const deletePromptPresetMock = vi.fn<(presetID: string, storageLocation: string, sessionName: string) => Promise<void>>();
const reorderPromptPresetsMock = vi.fn<(presetIDs: string[], storageLocation: string, sessionName: string) => Promise<void>>();

vi.mock("../../../../api", () => ({
    api: {
        LoadPromptPresets: (sessionName: string) => loadPromptPresetsMock(sessionName),
        SavePromptPreset: (payload: unknown, sessionName: string) => savePromptPresetMock(payload, sessionName),
        DeletePromptPreset: (presetID: string, storageLocation: string, sessionName: string) => (
            deletePromptPresetMock(presetID, storageLocation, sessionName)
        ),
        ReorderPromptPresets: (presetIDs: string[], storageLocation: string, sessionName: string) => (
            reorderPromptPresetsMock(presetIDs, storageLocation, sessionName)
        ),
    },
}));

let latestHook: ReturnType<typeof usePromptPresets> | null = null;

function HookHarness() {
    latestHook = usePromptPresets();
    return null;
}

function createLoadResult(overrides: Partial<PromptPresetLoadResult> = {}): PromptPresetLoadResult {
    return {
        presets: [],
        warnings: [],
        ...overrides,
    };
}

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((innerResolve, innerReject) => {
        resolve = innerResolve;
        reject = innerReject;
    });
    return {promise, resolve, reject};
}

describe("usePromptPresets", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
        latestHook = null;
        loadPromptPresetsMock.mockReset();
        savePromptPresetMock.mockReset();
        deletePromptPresetMock.mockReset();
        reorderPromptPresetsMock.mockReset();
        usePromptPresetStore.setState({version: 0});
        useTmuxStore.setState({
            config: null,
            sessions: [
                {id: 1, name: "alpha", created_at: "", is_idle: false, active_window_id: 0, windows: []},
                {id: 2, name: "beta", created_at: "", is_idle: false, active_window_id: 0, windows: []},
            ],
            sessionOrder: ["alpha", "beta"],
            activeSession: "alpha",
            activeWindowId: null,
            zoomPaneId: null,
            pendingPrefixKillPaneId: null,
            prefixMode: false,
            syncInputMode: false,
            fontSize: 13,
            imeResetSignal: 0,
        });
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        consoleWarnSpy.mockRestore();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("keeps the saved draft id stable when reload fails after a successful save", async () => {
        loadPromptPresetsMock
            .mockResolvedValueOnce(createLoadResult())
            .mockRejectedValueOnce(new Error("reload failed"));
        savePromptPresetMock.mockResolvedValue();

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        const draft: PromptPresetDraft = {
            id: "draft-123",
            name: "Preset",
            body: "Run tests first.",
            order: 0,
            storageLocation: "global",
            projectSessionName: null,
        };

        await act(async () => {
            await latestHook?.savePreset(draft);
        });

        expect(savePromptPresetMock).toHaveBeenCalledTimes(1);
        expect(savePromptPresetMock.mock.calls[0]?.[0]).toMatchObject({id: "draft-123"});
        expect(loadPromptPresetsMock).toHaveBeenCalledTimes(2);
        expect(latestHook?.error).toBe("Prompt preset saved, but reloading the list failed.");
        expect(usePromptPresetStore.getState().version).toBe(1);
    });

    it("uses the draft project session instead of the current active session when saving", async () => {
        loadPromptPresetsMock.mockResolvedValue(createLoadResult());
        savePromptPresetMock.mockResolvedValue();
        useTmuxStore.setState((state) => ({...state, activeSession: "beta"}));

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        await act(async () => {
            await latestHook?.savePreset({
                id: "project-draft",
                name: "Project preset",
                body: "Keep project context.",
                order: 0,
                storageLocation: "project",
                projectSessionName: "alpha",
            });
        });

        expect(savePromptPresetMock).toHaveBeenCalledWith(expect.anything(), "alpha");
    });

    it("uses the provided project session when deleting and reordering project presets", async () => {
        loadPromptPresetsMock.mockResolvedValue(createLoadResult({
            presets: [
                {id: "a", name: "A", body: "one", order: 0, storage_location: "project"},
                {id: "b", name: "B", body: "two", order: 1, storage_location: "project"},
            ],
        }));
        deletePromptPresetMock.mockResolvedValue();
        reorderPromptPresetsMock.mockResolvedValue();
        useTmuxStore.setState((state) => ({...state, activeSession: "beta"}));

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        await act(async () => {
            await latestHook?.deletePreset("a", "project", "alpha");
            await latestHook?.moveDown("a", "project", "alpha");
        });

        expect(deletePromptPresetMock).toHaveBeenCalledWith("a", "project", "alpha");
        expect(reorderPromptPresetsMock).toHaveBeenCalledWith(["b", "a"], "project", "alpha");
    });

    it("ignores stale same-session load responses", async () => {
        const firstLoad = deferred<PromptPresetLoadResult>();
        const secondLoad = deferred<PromptPresetLoadResult>();
        loadPromptPresetsMock
            .mockReturnValueOnce(firstLoad.promise)
            .mockReturnValueOnce(secondLoad.promise);

        await act(async () => {
            root.render(<HookHarness/>);
        });

        await act(async () => {
            const refreshPromise = latestHook?.refresh("alpha:1");
            secondLoad.resolve(createLoadResult({
                presets: [{id: "new", name: "New", body: "latest", order: 0, storage_location: "global"}],
            }));
            await refreshPromise;
        });

        expect(latestHook?.presets.map((preset) => preset.id)).toEqual(["new"]);

        await act(async () => {
            firstLoad.resolve(createLoadResult({
                presets: [{id: "old", name: "Old", body: "stale", order: 0, storage_location: "global"}],
            }));
            await Promise.resolve();
        });

        expect(latestHook?.presets.map((preset) => preset.id)).toEqual(["new"]);
    });

    it("surfaces non-fatal load warnings without treating them as errors", async () => {
        loadPromptPresetsMock.mockResolvedValue(createLoadResult({
            presets: [{id: "global", name: "Global", body: "body", order: 0, storage_location: "global"}],
            warnings: ["Project prompt presets could not be loaded for alpha."],
        }));

        await act(async () => {
            root.render(<HookHarness/>);
        });
        await act(async () => {
            await Promise.resolve();
        });

        expect(latestHook?.error).toBeNull();
        expect(latestHook?.warning).toBe("Project prompt presets could not be loaded for alpha.");
        expect(latestHook?.presets.map((preset) => preset.id)).toEqual(["global"]);
    });
});
