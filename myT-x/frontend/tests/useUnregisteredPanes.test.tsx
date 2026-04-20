import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useUnregisteredPanes} from "../src/hooks/useUnregisteredPanes";
import type {PaneSnapshot} from "../src/types/tmux";

const runtimeMock = vi.hoisted(() => ({
    EventsOn: vi.fn(),
}));

const appMock = vi.hoisted(() => ({
    GetSessionEnlistmentContext: vi.fn(),
}));

vi.mock("../wailsjs/runtime/runtime", () => runtimeMock);
vi.mock("../wailsjs/go/main/App", async (importOriginal) => {
    const actual = await importOriginal<typeof import("../wailsjs/go/main/App")>();
    return {
        ...actual,
        GetSessionEnlistmentContext: (sessionName: string) => appMock.GetSessionEnlistmentContext(sessionName),
    };
});

interface ProbeProps {
    sessionName: string | null;
    panes: PaneSnapshot[];
}

type HookResult = ReturnType<typeof useUnregisteredPanes>;

let latestResult: HookResult | null = null;

function UseUnregisteredPanesProbe({sessionName, panes}: ProbeProps) {
    latestResult = useUnregisteredPanes(sessionName, panes);
    return null;
}

function createDeferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return {promise, resolve, reject};
}

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
    });
}

describe("useUnregisteredPanes", () => {
    let container: HTMLDivElement;
    let root: Root;
    let eventHandlers: Map<string, (payload: unknown) => void>;

    beforeEach(() => {
        latestResult = null;
        eventHandlers = new Map<string, (payload: unknown) => void>();
        runtimeMock.EventsOn.mockImplementation((eventName: string, handler: (payload: unknown) => void) => {
            eventHandlers.set(eventName, handler);
            return () => {
                eventHandlers.delete(eventName);
            };
        });
        appMock.GetSessionEnlistmentContext.mockReset();
        appMock.GetSessionEnlistmentContext.mockResolvedValue({
            teams: [
                {
                    id: "team-alpha",
                    name: "Alpha",
                    description: "",
                    order: 0,
                    storage_location: "global",
                    members: [{
                        id: "member-lead",
                        team_id: "team-alpha",
                        order: 0,
                        pane_title: "Leader",
                        role: "Lead engineer",
                        command: "claude",
                        args: [],
                        custom_message: "",
                        skills: [{name: "Go", description: "Service design"}],
                    }],
                },
                {
                    id: "__unaffiliated__",
                    name: "Unaffiliated",
                    description: "",
                    order: 1,
                    storage_location: "global",
                    members: [],
                },
            ],
            unaffiliated_members: [],
            role_catalog: ["Lead engineer"],
            skill_catalog: [{name: "Go", description: "Service design"}],
            registered_pane_ids: ["%2"],
        });

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
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("excludes the initial and registered panes and suggests the parent team", async () => {
        const panes: PaneSnapshot[] = [
            {id: "%1", index: 0, title: "Leader", active: true, width: 120, height: 40},
            {id: "%2", index: 1, title: "Registered", active: false, width: 120, height: 40},
            {id: "%3", index: 2, title: "Fresh split", active: false, width: 120, height: 40},
        ];

        act(() => {
            root.render(<UseUnregisteredPanesProbe sessionName="alpha" panes={panes}/>);
        });
        await flushEffects();

        expect(appMock.GetSessionEnlistmentContext).toHaveBeenCalledWith("alpha");
        expect(latestResult?.unregisteredPanes).toHaveLength(1);
        expect(latestResult?.unregisteredPanes[0]?.pane.id).toBe("%3");

        const paneCreatedHandler = eventHandlers.get("tmux:pane-created");
        expect(paneCreatedHandler).toBeTypeOf("function");

        act(() => {
            paneCreatedHandler?.({sessionName: "alpha", paneId: "%3", parentPaneId: "%1"});
        });
        await flushEffects();

        expect(latestResult?.unregisteredPanes).toHaveLength(1);
        expect(latestResult?.unregisteredPanes[0]).toMatchObject({
            parentPaneId: "%1",
            parentPaneTitle: "Leader",
            suggestedTeamID: "team-alpha",
            suggestedStorageLocation: "global",
            suggestedRole: "Lead engineer",
        });
    });

    it("treats pane ids as numeric when index ties occur", async () => {
        const panes: PaneSnapshot[] = [
            {id: "%10", index: 1, title: "Later", active: false, width: 120, height: 40},
            {id: "%2", index: 1, title: "Earlier", active: true, width: 120, height: 40},
            {id: "%11", index: 2, title: "Fresh split", active: false, width: 120, height: 40},
        ];

        act(() => {
            root.render(<UseUnregisteredPanesProbe sessionName="alpha" panes={panes}/>);
        });
        await flushEffects();

        expect(latestResult?.unregisteredPanes.map((entry) => entry.pane.id)).toEqual(["%10", "%11"]);
    });

    it("ignores stale same-session reload responses after a newer reload completes", async () => {
        const firstResponse = createDeferred<{
            teams: unknown[];
            unaffiliated_members: unknown[];
            role_catalog: string[];
            skill_catalog: unknown[];
            registered_pane_ids: string[];
        }>();
        const secondResponse = createDeferred<{
            teams: unknown[];
            unaffiliated_members: unknown[];
            role_catalog: string[];
            skill_catalog: unknown[];
            registered_pane_ids: string[];
        }>();
        appMock.GetSessionEnlistmentContext
            .mockReset()
            .mockImplementationOnce(() => firstResponse.promise)
            .mockImplementationOnce(() => secondResponse.promise);

        const panes: PaneSnapshot[] = [
            {id: "%1", index: 0, title: "Leader", active: true, width: 120, height: 40},
            {id: "%3", index: 1, title: "Fresh split", active: false, width: 120, height: 40},
        ];

        act(() => {
            root.render(<UseUnregisteredPanesProbe sessionName="alpha" panes={panes}/>);
        });
        await flushEffects();

        const handler = eventHandlers.get("orchestrator:agents-updated");
        expect(handler).toBeTypeOf("function");

        act(() => {
            handler?.({sessionName: "alpha"});
        });
        await flushEffects();

        await act(async () => {
            secondResponse.resolve({
                teams: [],
                unaffiliated_members: [],
                role_catalog: [],
                skill_catalog: [],
                registered_pane_ids: ["%3"],
            });
            await Promise.resolve();
            await Promise.resolve();
        });
        await flushEffects();

        expect(latestResult?.unregisteredPanes).toHaveLength(0);

        await act(async () => {
            firstResponse.resolve({
                teams: [],
                unaffiliated_members: [],
                role_catalog: [],
                skill_catalog: [],
                registered_pane_ids: [],
            });
            await Promise.resolve();
            await Promise.resolve();
        });
        await flushEffects();

        expect(latestResult?.unregisteredPanes).toHaveLength(0);
    });
});
