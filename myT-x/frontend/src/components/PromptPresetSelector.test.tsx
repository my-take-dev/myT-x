import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {usePromptPresetStore} from "../stores/promptPresetStore";
import {useTmuxStore} from "../stores/tmuxStore";
import {appendPromptPresetBody, PromptPresetSelector} from "./PromptPresetSelector";

const loadPromptPresetsMock = vi.fn<(sessionName: string) => Promise<unknown>>();

vi.mock("../api", () => ({
    api: {
        LoadPromptPresets: (sessionName: string) => loadPromptPresetsMock(sessionName),
    },
}));

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((innerResolve, innerReject) => {
        resolve = innerResolve;
        reject = innerReject;
    });
    return {promise, resolve, reject};
}

describe("appendPromptPresetBody", () => {
    it("returns the preset body when the textarea is empty", () => {
        expect(appendPromptPresetBody("", "Run tests first.")).toBe("Run tests first.");
    });

    it("appends the preset body on a new line when the textarea already has content", () => {
        expect(appendPromptPresetBody("Existing text", "Run tests first.")).toBe("Existing text\nRun tests first.");
    });
});

describe("PromptPresetSelector", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        loadPromptPresetsMock.mockReset();
        usePromptPresetStore.setState({version: 0});
        useTmuxStore.setState({
            config: null,
            sessions: [{id: 1, name: "alpha", created_at: "", is_idle: false, active_window_id: 0, windows: []}],
            sessionOrder: ["alpha"],
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
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("ignores stale same-session responses and keeps the latest preset list", async () => {
        const firstLoad = deferred<unknown>();
        const secondLoad = deferred<unknown>();
        loadPromptPresetsMock
            .mockReturnValueOnce(firstLoad.promise)
            .mockReturnValueOnce(secondLoad.promise);

        await act(async () => {
            root.render(<PromptPresetSelector setText={vi.fn()} />);
        });

        await act(async () => {
            usePromptPresetStore.getState().bumpVersion();
            secondLoad.resolve({
                presets: [{id: "latest", name: "Latest", body: "latest", order: 0, storage_location: "global"}],
                warnings: [],
            });
            await Promise.resolve();
        });

        let options = Array.from(container.querySelectorAll("option"));
        expect(options.map((option) => option.textContent)).toContain("Latest");

        await act(async () => {
            firstLoad.resolve({
                presets: [{id: "stale", name: "Stale", body: "stale", order: 0, storage_location: "global"}],
                warnings: [],
            });
            await Promise.resolve();
        });

        options = Array.from(container.querySelectorAll("option"));
        expect(options.map((option) => option.textContent)).toContain("Latest");
        expect(options.map((option) => option.textContent)).not.toContain("Stale");
    });

    it("shows a warning indicator when the load succeeds with warnings", async () => {
        loadPromptPresetsMock.mockResolvedValue({
            presets: [{id: "preset", name: "Preset", body: "body", order: 0, storage_location: "global"}],
            warnings: ["Project prompt presets could not be loaded for alpha."],
        });

        await act(async () => {
            root.render(<PromptPresetSelector setText={vi.fn()} />);
        });
        await act(async () => {
            await Promise.resolve();
        });

        const warning = container.querySelector(".prompt-preset-selector-warning");
        expect(warning?.textContent).toBe("!");
        expect(warning?.getAttribute("title")).toContain("Project prompt presets could not be loaded");
    });
});
