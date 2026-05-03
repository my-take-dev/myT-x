import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {devpanel} from "../wailsjs/go/models";

const apiMock = vi.hoisted(() => ({
    DevPanelWorkingDiff: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    DevPanelGitStatus: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

let mockActiveSession: string | null = "session-a";

vi.mock("../src/api", () => ({
    api: {
        DevPanelWorkingDiff: (...args: unknown[]) => apiMock.DevPanelWorkingDiff(...args),
        DevPanelGitStatus: (...args: unknown[]) => apiMock.DevPanelGitStatus(...args),
    },
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: { activeSession: string | null }) => unknown) =>
        selector({activeSession: mockActiveSession}),
}));

import {useDiffData} from "../src/components/viewer/views/diff-view/useDiffData";

function makeGitStatus(overrides: Partial<devpanel.GitStatusResult> = {}): devpanel.GitStatusResult {
    return {
        branch: "main",
        modified: [],
        staged: [],
        untracked: [],
        conflicted: [],
        ahead: 0,
        behind: 0,
        upstream_configured: true,
        ...overrides,
    } satisfies devpanel.GitStatusResult;
}

function DiffDataProbe() {
    useDiffData();
    return null;
}

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

describe("useDiffData", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        mockActiveSession = "session-a";
        apiMock.DevPanelWorkingDiff.mockReset();
        apiMock.DevPanelWorkingDiff.mockResolvedValue({
            files: [],
            total_added: 0,
            total_deleted: 0,
            truncated: false,
        });
        apiMock.DevPanelGitStatus.mockReset();
        apiMock.DevPanelGitStatus.mockResolvedValue(makeGitStatus());
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.useRealTimers();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("silently refreshes the active session on window focus", async () => {
        act(() => {
            root.render(<DiffDataProbe/>);
        });
        await flushEffects();

        expect(apiMock.DevPanelWorkingDiff).toHaveBeenCalledTimes(1);
        expect(apiMock.DevPanelWorkingDiff).toHaveBeenCalledWith("session-a");

        act(() => {
            window.dispatchEvent(new Event("focus"));
        });
        await flushEffects();

        expect(apiMock.DevPanelWorkingDiff).toHaveBeenCalledTimes(2);
        expect(apiMock.DevPanelWorkingDiff).toHaveBeenNthCalledWith(2, "session-a");
    });

    it("does not poll the working diff after the initial load", async () => {
        vi.useFakeTimers();

        act(() => {
            root.render(<DiffDataProbe/>);
        });
        await flushEffects();

        act(() => {
            vi.advanceTimersByTime(60_000);
        });
        await flushEffects();

        expect(apiMock.DevPanelWorkingDiff).toHaveBeenCalledTimes(1);
    });
});
