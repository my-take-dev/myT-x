import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const runtimeMock = vi.hoisted(() => ({
    EventsOn: vi.fn(),
}));

const appMock = vi.hoisted(() => ({
    AddSingleTaskRunnerItem: vi.fn(),
    GetSingleTaskRunnerClearDelay: vi.fn<() => Promise<number>>(),
    GetSingleTaskRunnerStatus: vi.fn<() => Promise<unknown>>(),
    RemoveSingleTaskRunnerItem: vi.fn(),
    ReorderSingleTaskRunnerItems: vi.fn(),
    SetSingleTaskRunnerClearDelay: vi.fn(),
    StartSingleTaskRunner: vi.fn(),
    StopSingleTaskRunner: vi.fn(),
    UpdateSingleTaskRunnerItem: vi.fn(),
}));

let mockActiveSession = "session-a";
let mockSessions = [
    {
        id: 1,
        name: "session-a",
        windows: [{panes: [{id: "%1", index: 0, active: true, width: 80, height: 24}]}],
    },
    {
        id: 2,
        name: "session-b",
        windows: [{panes: [{id: "%2", index: 0, active: true, width: 80, height: 24}]}],
    },
];

vi.mock("../wailsjs/go/main/App", () => ({
    AddSingleTaskRunnerItem: (...args: unknown[]) => appMock.AddSingleTaskRunnerItem(...args),
    GetSingleTaskRunnerClearDelay: (...args: unknown[]) => appMock.GetSingleTaskRunnerClearDelay(...args),
    GetSingleTaskRunnerStatus: (...args: unknown[]) => appMock.GetSingleTaskRunnerStatus(...args),
    RemoveSingleTaskRunnerItem: (...args: unknown[]) => appMock.RemoveSingleTaskRunnerItem(...args),
    ReorderSingleTaskRunnerItems: (...args: unknown[]) => appMock.ReorderSingleTaskRunnerItems(...args),
    SetSingleTaskRunnerClearDelay: (...args: unknown[]) => appMock.SetSingleTaskRunnerClearDelay(...args),
    StartSingleTaskRunner: (...args: unknown[]) => appMock.StartSingleTaskRunner(...args),
    StopSingleTaskRunner: (...args: unknown[]) => appMock.StopSingleTaskRunner(...args),
    UpdateSingleTaskRunnerItem: (...args: unknown[]) => appMock.UpdateSingleTaskRunnerItem(...args),
}));

vi.mock("../wailsjs/runtime", () => runtimeMock);

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: { sessions: unknown[]; activeSession: string | null }) => unknown) =>
        selector({sessions: mockSessions, activeSession: mockActiveSession}),
}));

import {
    isEditableStatus,
    useSingleTaskRunner
} from "../src/components/viewer/views/single-task-runner/useSingleTaskRunner";

let hookResult: ReturnType<typeof useSingleTaskRunner> | null = null;

function makeStatus(
    sessionName: string,
    runStatus: string,
    generationId: string = `${sessionName}-${runStatus}`,
    overrides: Record<string, unknown> = {},
) {
    return {
        items: [],
        run_status: runStatus,
        current_index: -1,
        session_name: sessionName,
        generation_id: generationId,
        clear_delay_sec: 0,
        last_stop_reason: "",
        ...overrides,
    };
}

function makeItem(overrides: Record<string, unknown> = {}) {
    return {
        id: "item-1",
        title: "Title",
        message: "Message",
        target_pane_id: "%1",
        order_index: 0,
        status: "pending",
        created_at: "2026-04-09T00:00:00Z",
        clear_before: false,
        ...overrides,
    };
}

function Probe() {
    const result = useSingleTaskRunner();
    hookResult = result;
    const {status, error, availablePanes} = result;
    return (
        <div>
            <output data-testid="run-status">{status?.run_status ?? ""}</output>
            <output data-testid="error">{error ?? ""}</output>
            <output data-testid="pane-count">{String(availablePanes.length)}</output>
        </div>
    );
}

function getProbeText(container: HTMLElement, testId: string): string {
    return container.querySelector(`[data-testid="${testId}"]`)?.textContent ?? "";
}

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
    });
}

function createDeferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((innerResolve, innerReject) => {
        resolve = innerResolve;
        reject = innerReject;
    });
    return {promise, resolve, reject};
}

describe("useSingleTaskRunner", () => {
    let container: HTMLDivElement;
    let root: Root;
    let eventHandlers: Map<string, (payload: unknown) => void>;

    beforeEach(() => {
        eventHandlers = new Map<string, (payload: unknown) => void>();
        runtimeMock.EventsOn.mockReset();
        runtimeMock.EventsOn.mockImplementation((eventName: string, handler: (payload: unknown) => void) => {
            eventHandlers.set(eventName, handler);
            return () => {
                eventHandlers.delete(eventName);
            };
        });
        appMock.GetSingleTaskRunnerStatus.mockReset();
        appMock.GetSingleTaskRunnerStatus.mockResolvedValue(makeStatus("session-a", "idle"));
        appMock.GetSingleTaskRunnerClearDelay.mockReset();
        appMock.GetSingleTaskRunnerClearDelay.mockResolvedValue(2);
        vi.spyOn(console, "warn").mockImplementation(() => undefined);

        mockActiveSession = "session-a";
        mockSessions = [
            {id: 1, name: "session-a", windows: [{panes: [{id: "%1", index: 0, active: true, width: 80, height: 24}]}]},
            {id: 2, name: "session-b", windows: [{panes: [{id: "%2", index: 0, active: true, width: 80, height: 24}]}]},
        ];
        hookResult = null;

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

    it("treats missing item status as non-editable", () => {
        expect(isEditableStatus(undefined)).toBe(false);
        expect(isEditableStatus(null)).toBe(false);
        expect(isEditableStatus("pending")).toBe(true);
    });

    it("waits for the active session snapshot before session-scoped reads", async () => {
        mockActiveSession = "session-a";
        mockSessions = [
            {id: 2, name: "session-b", windows: [{panes: [{id: "%2", index: 0, active: true, width: 80, height: 24}]}]},
        ];

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(appMock.GetSingleTaskRunnerStatus).not.toHaveBeenCalled();
        expect(getProbeText(container, "error")).toBe("");

        mockSessions = [
            {id: 1, name: "session-a", windows: [{panes: [{id: "%1", index: 0, active: true, width: 80, height: 24}]}]},
        ];
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(appMock.GetSingleTaskRunnerStatus).toHaveBeenCalledWith("session-a:1");
    });

    it("re-subscribes on session change and ignores other-session events", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(appMock.GetSingleTaskRunnerStatus).toHaveBeenCalledWith("session-a:1");
        expect(getProbeText(container, "run-status")).toBe("idle");
        expect(getProbeText(container, "pane-count")).toBe("1");

        mockActiveSession = "session-b";
        appMock.GetSingleTaskRunnerStatus.mockResolvedValueOnce(makeStatus("session-b", "idle"));
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("single-task-runner:updated")?.(makeStatus("session-a", "running"));
        });
        expect(getProbeText(container, "run-status")).toBe("idle");

        act(() => {
            eventHandlers.get("single-task-runner:updated")?.(makeStatus("session-b", "running", "session-b-idle"));
        });
        expect(getProbeText(container, "run-status")).toBe("running");
    });

    it("falls back to refresh on invalid payload and cleans subscriptions on unmount", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        appMock.GetSingleTaskRunnerStatus.mockResolvedValueOnce(makeStatus("session-a", "completed"));
        act(() => {
            eventHandlers.get("single-task-runner:updated")?.({invalid: true});
        });
        await flushEffects();

        expect(appMock.GetSingleTaskRunnerStatus).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "run-status")).toBe("completed");

        act(() => {
            root.unmount();
        });
        expect(eventHandlers.size).toBe(0);
    });

    it("refreshes status when an updated event contains a malformed queue item", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        appMock.GetSingleTaskRunnerStatus.mockResolvedValueOnce({
            ...makeStatus("session-a", "completed"),
            items: [makeItem()],
        });
        act(() => {
            eventHandlers.get("single-task-runner:updated")?.({
                ...makeStatus("session-a", "running"),
                items: [makeItem({status: undefined})],
            });
        });
        await flushEffects();

        expect(appMock.GetSingleTaskRunnerStatus).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "run-status")).toBe("completed");
    });

    it("ignores stale same-session events after session recreation", async () => {
        appMock.GetSingleTaskRunnerStatus.mockResolvedValueOnce(makeStatus("session-a", "idle", "gen-old"));
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("idle");

        mockSessions = [
            {
                id: 101,
                name: "session-a",
                windows: [{panes: [{id: "%9", index: 0, active: true, width: 80, height: 24}]}]
            },
            {id: 2, name: "session-b", windows: [{panes: [{id: "%2", index: 0, active: true, width: 80, height: 24}]}]},
        ];
        appMock.GetSingleTaskRunnerStatus.mockResolvedValueOnce(makeStatus("session-a", "completed", "gen-new"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("completed");
        expect(getProbeText(container, "pane-count")).toBe("1");

        act(() => {
            eventHandlers.get("single-task-runner:updated")?.(makeStatus("session-a", "running", "gen-old"));
        });
        expect(getProbeText(container, "run-status")).toBe("completed");

        act(() => {
            eventHandlers.get("single-task-runner:updated")?.(makeStatus("session-a", "running", "gen-new"));
        });
        expect(getProbeText(container, "run-status")).toBe("running");
    });

    it("ignores stale stopped events after same-name session recreation", async () => {
        appMock.GetSingleTaskRunnerStatus
            .mockReset()
            .mockResolvedValueOnce(makeStatus("session-a", "idle", "gen-old"))
            .mockResolvedValueOnce(makeStatus("session-a", "running", "gen-new"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        mockSessions = [
            {
                id: 101,
                name: "session-a",
                windows: [{panes: [{id: "%9", index: 0, active: true, width: 80, height: 24}]}],
            },
            {id: 2, name: "session-b", windows: [{panes: [{id: "%2", index: 0, active: true, width: 80, height: 24}]}]},
        ];
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("single-task-runner:stopped")?.({
                session_name: "session-a",
                generation_id: "gen-old",
                reason: "stale worker fatal",
            });
        });

        expect(appMock.GetSingleTaskRunnerStatus).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "run-status")).toBe("running");
        expect(getProbeText(container, "error")).toBe("");
    });

    it("keeps the stopped error banner after refresh succeeds", async () => {
        appMock.GetSingleTaskRunnerStatus
            .mockReset()
            .mockResolvedValueOnce(makeStatus("session-a", "running", "gen-old"))
            .mockResolvedValueOnce(makeStatus("session-a", "running", "gen-new"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("single-task-runner:stopped")?.({
                session_name: "session-a",
                generation_id: "gen-old",
                reason: "worker fatal",
            });
        });
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("running");
        expect(getProbeText(container, "error")).toBe("Task stopped: worker fatal");
    });

    it("shows the stopped error banner from refreshed status when the event was missed", async () => {
        appMock.GetSingleTaskRunnerStatus
            .mockReset()
            .mockResolvedValueOnce(makeStatus("session-a", "idle", "gen-idle", {last_stop_reason: "worker fatal"}));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("idle");
        expect(getProbeText(container, "error")).toBe("Task stopped: worker fatal");
    });

    it("adopts a new generation after StartSingleTaskRunner succeeds, even when the previous generation is empty", async () => {
        appMock.StartSingleTaskRunner.mockReset();
        appMock.StartSingleTaskRunner.mockResolvedValue(undefined);
        appMock.GetSingleTaskRunnerStatus
            .mockReset()
            .mockResolvedValueOnce(makeStatus("session-a", "idle", ""))
            .mockResolvedValueOnce(makeStatus("session-a", "running", "gen-new"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        await act(async () => {
            const ok = await hookResult!.start();
            expect(ok).toBe(true);
        });
        await flushEffects();

        expect(appMock.StartSingleTaskRunner).toHaveBeenCalledWith("session-a:1");
        expect(getProbeText(container, "run-status")).toBe("running");

        act(() => {
            eventHandlers.get("single-task-runner:updated")?.(makeStatus("session-a", "completed", "gen-new"));
        });
        expect(getProbeText(container, "run-status")).toBe("completed");
    });

    it.each([
        {
            name: "stop",
            arrangeRefresh: () => {
                appMock.StopSingleTaskRunner.mockReset();
                appMock.StopSingleTaskRunner.mockResolvedValue(undefined);
                appMock.GetSingleTaskRunnerStatus
                    .mockReset()
                    .mockResolvedValueOnce(makeStatus("session-a", "running"))
                    .mockResolvedValueOnce(makeStatus("session-a", "idle"));
                return () => hookResult!.stop();
            },
        },
        {
            name: "remove",
            arrangeRefresh: () => {
                appMock.RemoveSingleTaskRunnerItem.mockReset();
                appMock.RemoveSingleTaskRunnerItem.mockResolvedValue(undefined);
                appMock.GetSingleTaskRunnerStatus
                    .mockReset()
                    .mockResolvedValueOnce(makeStatus("session-a", "idle"))
                    .mockResolvedValueOnce(makeStatus("session-a", "idle"));
                return () => hookResult!.removeItem("item-1");
            },
        },
        {
            name: "reorder",
            arrangeRefresh: () => {
                appMock.ReorderSingleTaskRunnerItems.mockReset();
                appMock.ReorderSingleTaskRunnerItems.mockResolvedValue(undefined);
                appMock.GetSingleTaskRunnerStatus
                    .mockReset()
                    .mockResolvedValueOnce(makeStatus("session-a", "idle"))
                    .mockResolvedValueOnce(makeStatus("session-a", "idle"));
                return () => hookResult!.reorderItems(["item-1"]);
            },
        },
    ])("refreshes status after $name succeeds", async ({arrangeRefresh}) => {
        const invoke = arrangeRefresh();

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        await act(async () => {
            await invoke();
        });
        await flushEffects();

        expect(appMock.GetSingleTaskRunnerStatus).toHaveBeenCalledTimes(2);
    });

    it.each([
        {
            name: "add",
            arrangeMutation: () => {
                appMock.AddSingleTaskRunnerItem.mockReset();
                appMock.AddSingleTaskRunnerItem.mockResolvedValue(undefined);
                return () => hookResult!.addItem("Title", "Message", "%1", false, "");
            },
        },
        {
            name: "update",
            arrangeMutation: () => {
                appMock.UpdateSingleTaskRunnerItem.mockReset();
                appMock.UpdateSingleTaskRunnerItem.mockResolvedValue(undefined);
                return () => hookResult!.updateItem("item-1", "Title", "Message", "%1", false, "");
            },
        },
        {
            name: "clearDelay",
            arrangeMutation: () => {
                appMock.SetSingleTaskRunnerClearDelay.mockReset();
                appMock.SetSingleTaskRunnerClearDelay.mockResolvedValue(undefined);
                return () => hookResult!.setClearDelay(2);
            },
        },
    ])("returns success without refreshing status after $name succeeds", async ({arrangeMutation}) => {
        const invoke = arrangeMutation();

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        let result = false;
        await act(async () => {
            result = await invoke();
        });
        await flushEffects();

        expect(result).toBe(true);
        expect(appMock.GetSingleTaskRunnerStatus).toHaveBeenCalledTimes(1);
        expect(getProbeText(container, "error")).toBe("");
    });

    it.each([
        {
            name: "start",
            arrangeFailure: () => {
                appMock.StartSingleTaskRunner.mockReset();
                appMock.StartSingleTaskRunner.mockRejectedValueOnce(new Error("start failed"));
                return () => hookResult!.start();
            },
            expectedMessage: "start failed",
        },
        {
            name: "add",
            arrangeFailure: () => {
                appMock.AddSingleTaskRunnerItem.mockReset();
                appMock.AddSingleTaskRunnerItem.mockRejectedValueOnce(new Error("add failed"));
                return () => hookResult!.addItem("Title", "Message", "%1", false, "");
            },
            expectedMessage: "add failed",
        },
        {
            name: "update",
            arrangeFailure: () => {
                appMock.UpdateSingleTaskRunnerItem.mockReset();
                appMock.UpdateSingleTaskRunnerItem.mockRejectedValueOnce(new Error("update failed"));
                return () => hookResult!.updateItem("item-1", "Title", "Message", "%1", false, "");
            },
            expectedMessage: "update failed",
        },
        {
            name: "clearDelay",
            arrangeFailure: () => {
                appMock.SetSingleTaskRunnerClearDelay.mockReset();
                appMock.SetSingleTaskRunnerClearDelay.mockRejectedValueOnce(new Error("delay failed"));
                return () => hookResult!.setClearDelay(2);
            },
            expectedMessage: "delay failed",
        },
    ])("converts $name backend failures into local error state without throwing", async ({
        arrangeFailure,
        expectedMessage,
    }) => {
        const invoke = arrangeFailure();

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        let result = true;
        await act(async () => {
            result = await invoke();
        });

        expect(result).toBe(false);
        expect(getProbeText(container, "error")).toContain(expectedMessage);
    });

    it("ignores a delayed initial refresh result after the user switches sessions", async () => {
        const sessionARefresh = createDeferred<ReturnType<typeof makeStatus>>();
        appMock.GetSingleTaskRunnerStatus
            .mockReset()
            .mockImplementationOnce(() => sessionARefresh.promise)
            .mockResolvedValueOnce(makeStatus("session-b", "running", "session-b-running"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        mockActiveSession = "session-b";
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("running");

        sessionARefresh.resolve(makeStatus("session-a", "completed", "session-a-completed"));
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("running");
        expect(getProbeText(container, "error")).toBe("");
    });

    it("ignores refresh snapshots that belong to another session", async () => {
        appMock.GetSingleTaskRunnerStatus
            .mockReset()
            .mockResolvedValueOnce(makeStatus("session-a", "idle"))
            .mockResolvedValueOnce(makeStatus("session-b", "running", "session-b-running"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("idle");

        act(() => {
            eventHandlers.get("single-task-runner:updated")?.({invalid: true});
        });
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("idle");
        expect(getProbeText(container, "error")).toBe("");
    });

    it("ignores stale mutation failures after the user switches sessions", async () => {
        const switchSession = async (sessionName: "session-a" | "session-b") => {
            mockActiveSession = sessionName;
            appMock.GetSingleTaskRunnerStatus.mockResolvedValueOnce(makeStatus(sessionName, "idle"));
            act(() => {
                root.render(<Probe/>);
            });
            await flushEffects();
            expect(getProbeText(container, "error")).toBe("");
        };

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const failureCases: Array<{
            readonly name: string;
            readonly begin: () => {
                readonly pending: ReturnType<typeof createDeferred<void>>;
                readonly invoke: () => Promise<unknown>;
                readonly expectedResult: unknown;
            };
        }> = [
            {
                name: "start",
                begin: () => {
                    const pending = createDeferred<void>();
                    appMock.StartSingleTaskRunner.mockReset().mockReturnValueOnce(pending.promise);
                    return {pending, invoke: () => hookResult!.start(), expectedResult: false};
                },
            },
            {
                name: "stop",
                begin: () => {
                    const pending = createDeferred<void>();
                    appMock.StopSingleTaskRunner.mockReset().mockReturnValueOnce(pending.promise);
                    return {pending, invoke: () => hookResult!.stop(), expectedResult: undefined};
                },
            },
            {
                name: "add",
                begin: () => {
                    const pending = createDeferred<void>();
                    appMock.AddSingleTaskRunnerItem.mockReset().mockReturnValueOnce(pending.promise);
                    return {
                        pending,
                        invoke: () => hookResult!.addItem("Title", "Message", "%1", false, ""),
                        expectedResult: false,
                    };
                },
            },
            {
                name: "remove",
                begin: () => {
                    const pending = createDeferred<void>();
                    appMock.RemoveSingleTaskRunnerItem.mockReset().mockReturnValueOnce(pending.promise);
                    return {pending, invoke: () => hookResult!.removeItem("item-1"), expectedResult: undefined};
                },
            },
            {
                name: "update",
                begin: () => {
                    const pending = createDeferred<void>();
                    appMock.UpdateSingleTaskRunnerItem.mockReset().mockReturnValueOnce(pending.promise);
                    return {
                        pending,
                        invoke: () => hookResult!.updateItem("item-1", "Title", "Message", "%1", false, ""),
                        expectedResult: false,
                    };
                },
            },
            {
                name: "reorder",
                begin: () => {
                    const pending = createDeferred<void>();
                    appMock.ReorderSingleTaskRunnerItems.mockReset().mockReturnValueOnce(pending.promise);
                    return {pending, invoke: () => hookResult!.reorderItems(["item-1"]), expectedResult: undefined};
                },
            },
            {
                name: "clearDelay",
                begin: () => {
                    const pending = createDeferred<void>();
                    appMock.SetSingleTaskRunnerClearDelay.mockReset().mockReturnValueOnce(pending.promise);
                    return {pending, invoke: () => hookResult!.setClearDelay(2), expectedResult: false};
                },
            },
        ];

        for (const testCase of failureCases) {
            await switchSession("session-a");
            const {pending, invoke, expectedResult} = testCase.begin();
            let promise!: Promise<unknown>;
            await act(async () => {
                promise = invoke();
                await Promise.resolve();
            });
            await switchSession("session-b");

            let settled: unknown = Symbol("unset");
            await act(async () => {
                pending.reject(new Error(`${testCase.name} failed`));
                settled = await promise;
            });

            expect(settled).toBe(expectedResult);
            expect(getProbeText(container, "error")).toBe("");
        }
    });

    it("does not refresh session-b after a stale start succeeds for session-a", async () => {
        const startPending = createDeferred<void>();
        appMock.StartSingleTaskRunner.mockReset().mockReturnValueOnce(startPending.promise);
        appMock.GetSingleTaskRunnerStatus
            .mockReset()
            .mockResolvedValueOnce(makeStatus("session-a", "idle"))
            .mockResolvedValueOnce(makeStatus("session-b", "idle"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        let startPromise!: Promise<boolean>;
        await act(async () => {
            startPromise = hookResult!.start();
            await Promise.resolve();
        });

        mockActiveSession = "session-b";
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        await act(async () => {
            startPending.resolve();
            await startPromise;
        });
        await flushEffects();

        expect(appMock.GetSingleTaskRunnerStatus).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "run-status")).toBe("idle");
        expect(getProbeText(container, "error")).toBe("");
    });

    it("surfaces refresh errors from GetSingleTaskRunnerStatus", async () => {
        appMock.GetSingleTaskRunnerStatus.mockRejectedValueOnce(new Error("backend offline"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("");
        expect(getProbeText(container, "error")).toContain("backend offline");
    });
});
