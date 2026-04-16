import {StrictMode, act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useUsageDashboard} from "../src/components/viewer/views/usage-dashboard/useUsageDashboard";
import {setLanguage} from "../src/i18n";
import {useTmuxStore} from "../src/stores/tmuxStore";
import type {usagedashboard} from "../wailsjs/go/models";

const getUsageDashboardMock = vi.hoisted(() => vi.fn());

vi.mock("../wailsjs/go/main/App", () => ({
    GetUsageDashboard: (...args: unknown[]) => getUsageDashboardMock(...args),
}));

interface ProbeState {
    snapshot: usagedashboard.UsageDashboardSnapshot | null;
    isLoading: boolean;
    error: string | null;
    hasActiveSession: boolean;
    activeSessionName: string;
}

function UsageDashboardProbe({onChange}: {onChange: (state: ProbeState) => void}) {
    const state = useUsageDashboard("both");
    onChange(state);
    return null;
}

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

describe("useUsageDashboard", () => {
    let container: HTMLDivElement;
    let root: Root;
    let latestState: ProbeState | null;

    beforeEach(() => {
        latestState = null;
        getUsageDashboardMock.mockReset();
        setLanguage("en");

        useTmuxStore.setState((state) => ({
            ...state,
            activeSession: "feature-usage",
            sessions: [
                {
                    id: 7,
                    name: "feature-usage",
                    created_at: "2026-04-15T19:00:00Z",
                    is_idle: false,
                    windows: [],
                    active_window_id: 3,
                },
            ],
            sessionOrder: ["feature-usage"],
        }));

        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        useTmuxStore.setState((state) => ({
            ...state,
            activeSession: null,
            sessions: [],
            sessionOrder: [],
        }));
        vi.restoreAllMocks();
        setLanguage("ja");
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("restores mounted state on StrictMode remount before applying async results", async () => {
        const snapshot = {
            work_dir: "D:/myT-x/dev-myT-x",
            last_updated_at: "2026-04-15T20:00:00Z",
            claude: null,
            codex: null,
        } satisfies usagedashboard.UsageDashboardSnapshot;

        const resolvers: Array<(value: usagedashboard.UsageDashboardSnapshot) => void> = [];
        getUsageDashboardMock.mockImplementation(
            () =>
                new Promise<usagedashboard.UsageDashboardSnapshot>((resolve) => {
                    resolvers.push(resolve);
                }),
        );

        act(() => {
            root.render(
                <StrictMode>
                    <UsageDashboardProbe onChange={(state) => {
                        latestState = state;
                    }}/>
                </StrictMode>,
            );
        });

        await flushEffects();
        expect(getUsageDashboardMock).toHaveBeenCalledTimes(2);
        expect(resolvers).toHaveLength(2);
        expect(latestState?.isLoading).toBe(true);

        await act(async () => {
            resolvers[1]?.(snapshot);
            await Promise.resolve();
        });

        expect(latestState?.snapshot).toEqual(snapshot);
        expect(latestState?.error).toBeNull();
        expect(latestState?.isLoading).toBe(true);

        await act(async () => {
            resolvers[0]?.(snapshot);
            await Promise.resolve();
        });

        expect(latestState?.isLoading).toBe(false);
        expect(latestState?.hasActiveSession).toBe(true);
        expect(latestState?.activeSessionName).toBe("feature-usage");
    });

    it("keeps loading=true while a newer session request is still in flight", async () => {
        const resolvers: Array<(value: usagedashboard.UsageDashboardSnapshot) => void> = [];
        getUsageDashboardMock.mockImplementation(
            () =>
                new Promise<usagedashboard.UsageDashboardSnapshot>((resolve) => {
                    resolvers.push(resolve);
                }),
        );

        act(() => {
            root.render(<UsageDashboardProbe onChange={(state) => {
                latestState = state;
            }}/>);
        });

        await flushEffects();
        expect(getUsageDashboardMock).toHaveBeenCalledTimes(1);
        expect(latestState?.isLoading).toBe(true);

        act(() => {
            useTmuxStore.setState((state) => ({
                ...state,
                activeSession: "feature-usage-2",
                sessions: [
                    ...state.sessions,
                    {
                        id: 8,
                        name: "feature-usage-2",
                        created_at: "2026-04-15T19:05:00Z",
                        is_idle: false,
                        windows: [],
                        active_window_id: 4,
                    },
                ],
                sessionOrder: ["feature-usage", "feature-usage-2"],
            }));
        });

        await flushEffects();
        expect(getUsageDashboardMock).toHaveBeenCalledTimes(2);
        expect(latestState?.isLoading).toBe(true);

        await act(async () => {
            resolvers[0]?.({
                work_dir: "D:/myT-x/dev-myT-x",
                last_updated_at: "2026-04-15T20:00:00Z",
                claude: null,
                codex: null,
            });
            await Promise.resolve();
        });

        expect(latestState?.isLoading).toBe(true);
        expect(latestState?.snapshot).toBeNull();

        const secondSnapshot = {
            work_dir: "D:/myT-x/dev-myT-x-2",
            last_updated_at: "2026-04-15T20:00:01Z",
            claude: null,
            codex: null,
        } satisfies usagedashboard.UsageDashboardSnapshot;

        await act(async () => {
            resolvers[1]?.(secondSnapshot);
            await Promise.resolve();
        });

        expect(latestState?.isLoading).toBe(false);
        expect(latestState?.snapshot).toEqual(secondSnapshot);
        expect(latestState?.activeSessionName).toBe("feature-usage-2");
    });

    it("keeps loading=true during StrictMode remount when the first request resolves before the second", async () => {
        const resolvers: Array<(value: usagedashboard.UsageDashboardSnapshot) => void> = [];
        getUsageDashboardMock.mockImplementation(
            () =>
                new Promise<usagedashboard.UsageDashboardSnapshot>((resolve) => {
                    resolvers.push(resolve);
                }),
        );

        act(() => {
            root.render(
                <StrictMode>
                    <UsageDashboardProbe onChange={(state) => {
                        latestState = state;
                    }}/>
                </StrictMode>,
            );
        });

        await flushEffects();
        expect(getUsageDashboardMock).toHaveBeenCalledTimes(2);
        expect(resolvers).toHaveLength(2);
        expect(latestState?.isLoading).toBe(true);

        await act(async () => {
            resolvers[0]?.({
                work_dir: "D:/myT-x/dev-myT-x",
                last_updated_at: "2026-04-15T20:00:00Z",
                claude: null,
                codex: null,
            });
            await Promise.resolve();
        });

        expect(latestState?.isLoading).toBe(true);

        await act(async () => {
            resolvers[1]?.({
                work_dir: "D:/myT-x/dev-myT-x",
                last_updated_at: "2026-04-15T20:00:01Z",
                claude: null,
                codex: null,
            });
            await Promise.resolve();
        });

        expect(latestState?.isLoading).toBe(false);
    });
});
