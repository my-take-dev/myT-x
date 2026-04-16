import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const runtimeMock = vi.hoisted(() => ({
    EventsOn: vi.fn(),
}));

const appMock = vi.hoisted(() => ({
    AddTaskSchedulerItem: vi.fn(),
    CheckTaskSchedulerOrchestratorReady: vi.fn(),
    GetTaskSchedulerSettings: vi.fn<() => Promise<unknown>>(),
    GetTaskSchedulerStatus: vi.fn<() => Promise<unknown>>(),
    PauseTaskScheduler: vi.fn(),
    RemoveTaskSchedulerItem: vi.fn(),
    ReorderTaskSchedulerItems: vi.fn(),
    ResumeTaskScheduler: vi.fn(),
    SaveTaskSchedulerSettings: vi.fn(),
    StartTaskScheduler: vi.fn(),
    StopTaskScheduler: vi.fn(),
    UpdateTaskSchedulerItem: vi.fn(),
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
    AddTaskSchedulerItem: (...args: unknown[]) => appMock.AddTaskSchedulerItem(...args),
    CheckTaskSchedulerOrchestratorReady: (...args: unknown[]) => appMock.CheckTaskSchedulerOrchestratorReady(...args),
    GetTaskSchedulerSettings: () => appMock.GetTaskSchedulerSettings(),
    GetTaskSchedulerStatus: (...args: unknown[]) => appMock.GetTaskSchedulerStatus(...args),
    PauseTaskScheduler: (...args: unknown[]) => appMock.PauseTaskScheduler(...args),
    RemoveTaskSchedulerItem: (...args: unknown[]) => appMock.RemoveTaskSchedulerItem(...args),
    ReorderTaskSchedulerItems: (...args: unknown[]) => appMock.ReorderTaskSchedulerItems(...args),
    ResumeTaskScheduler: (...args: unknown[]) => appMock.ResumeTaskScheduler(...args),
    SaveTaskSchedulerSettings: (...args: unknown[]) => appMock.SaveTaskSchedulerSettings(...args),
    StartTaskScheduler: (...args: unknown[]) => appMock.StartTaskScheduler(...args),
    StopTaskScheduler: (...args: unknown[]) => appMock.StopTaskScheduler(...args),
    UpdateTaskSchedulerItem: (...args: unknown[]) => appMock.UpdateTaskSchedulerItem(...args),
}));

vi.mock("../wailsjs/runtime", () => runtimeMock);

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: { sessions: unknown[]; activeSession: string | null }) => unknown) =>
        selector({sessions: mockSessions, activeSession: mockActiveSession}),
}));

import {isEditableStatus, useTaskScheduler} from "../src/components/viewer/views/task-scheduler/useTaskScheduler";
import type {TaskSchedulerSettings} from "../src/components/viewer/views/task-scheduler/useTaskScheduler";

let hookResult: ReturnType<typeof useTaskScheduler> | null = null;

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return {promise, resolve, reject};
}

function makeStatus(
    sessionName: string,
    runStatus: string,
    generationId: string = `${sessionName}-${runStatus}`,
) {
    return {
        items: [],
        run_status: runStatus,
        current_index: -1,
        session_name: sessionName,
        generation_id: generationId,
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

function makeSettings(preExecResetDelay: number) {
    return {
        pre_exec_reset_delay_s: preExecResetDelay,
        pre_exec_idle_timeout_s: 30,
        pre_exec_target_mode: "task_panes",
        message_templates: [],
    };
}

function makeConfigUpdatedEvent(preExecResetDelay: number, version: number) {
    return {
        version,
        config: {
            shell: "pwsh",
            prefix: "C-b",
            quake_mode: false,
            global_hotkey: "",
            keys: {},
            worktree: {
                enabled: false,
                force_cleanup: false,
                setup_script_timeout_seconds: 300,
            },
            task_scheduler: makeSettings(preExecResetDelay),
        },
    };
}

function Probe() {
    const result = useTaskScheduler();
    hookResult = result;
    const {status, settings, error} = result;
    return (
        <div>
            <output data-testid="run-status">{status?.run_status ?? ""}</output>
            <output data-testid="settings-reset-delay">{String(settings?.pre_exec_reset_delay_s ?? "")}</output>
            <output data-testid="error">{error ?? ""}</output>
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

describe("useTaskScheduler", () => {
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
        appMock.GetTaskSchedulerStatus.mockReset();
        appMock.GetTaskSchedulerStatus.mockResolvedValue(makeStatus("session-a", "idle"));
        appMock.CheckTaskSchedulerOrchestratorReady.mockReset();
        appMock.CheckTaskSchedulerOrchestratorReady.mockResolvedValue({
            ready: true,
            db_exists: true,
            agent_count: 1,
            has_panes: true,
        });
        appMock.GetTaskSchedulerSettings.mockReset();
        appMock.GetTaskSchedulerSettings.mockResolvedValue(makeSettings(0));
        vi.spyOn(console, "warn").mockImplementation(() => undefined);

        mockActiveSession = "session-a";
        mockSessions = [
            {id: 1, name: "session-a", windows: [{panes: [{id: "%1", index: 0, active: true, width: 80, height: 24}]}]},
            {id: 2, name: "session-b", windows: [{panes: [{id: "%2", index: 0, active: true, width: 80, height: 24}]}]},
        ];

        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        hookResult = null;
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

        expect(appMock.GetTaskSchedulerStatus).not.toHaveBeenCalled();
        expect(getProbeText(container, "error")).toBe("");

        let readiness: Awaited<ReturnType<NonNullable<typeof hookResult>["checkOrchestratorReady"]>> | null = null;
        await act(async () => {
            readiness = await hookResult?.checkOrchestratorReady() ?? null;
        });
        expect(readiness).toEqual({
            ready: false,
            db_exists: false,
            agent_count: 0,
            has_panes: false,
        });
        expect(appMock.CheckTaskSchedulerOrchestratorReady).not.toHaveBeenCalled();

        mockSessions = [
            {id: 1, name: "session-a", windows: [{panes: [{id: "%1", index: 0, active: true, width: 80, height: 24}]}]},
        ];
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(appMock.GetTaskSchedulerStatus).toHaveBeenCalledWith("session-a:1");
    });

    it("re-subscribes on session change and ignores other-session events", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        expect(appMock.GetTaskSchedulerStatus).toHaveBeenCalledWith("session-a:1");
        expect(getProbeText(container, "run-status")).toBe("idle");

        mockActiveSession = "session-b";
        appMock.GetTaskSchedulerStatus.mockResolvedValueOnce({
            ...makeStatus("session-b", "idle"),
            generation_id: "gen-b",
        });
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("task-scheduler:updated")?.(makeStatus("session-a", "running"));
        });
        expect(getProbeText(container, "run-status")).toBe("idle");

        act(() => {
            eventHandlers.get("task-scheduler:updated")?.({
                ...makeStatus("session-b", "running"),
                generation_id: "gen-b",
            });
        });
        expect(getProbeText(container, "run-status")).toBe("running");
    });

    it("refreshes status on invalid payload and reloads settings on config updates", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        appMock.GetTaskSchedulerStatus.mockResolvedValueOnce(makeStatus("session-a", "completed"));
        act(() => {
            eventHandlers.get("task-scheduler:updated")?.({invalid: true});
        });
        await flushEffects();
        expect(appMock.GetTaskSchedulerStatus).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "run-status")).toBe("completed");

        appMock.GetTaskSchedulerSettings.mockResolvedValueOnce(makeSettings(42));
        act(() => {
            eventHandlers.get("config:updated")?.({});
        });
        await flushEffects();
        expect(appMock.GetTaskSchedulerSettings).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "settings-reset-delay")).toBe("42");
    });

    it("reloads settings instead of resetting to defaults when config updates omit task scheduler data", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        appMock.GetTaskSchedulerSettings.mockResolvedValueOnce(makeSettings(11));
        act(() => {
            eventHandlers.get("config:updated")?.({
                version: 7,
                config: {
                    shell: "pwsh",
                    prefix: "C-b",
                    quake_mode: false,
                    global_hotkey: "",
                    keys: {},
                    worktree: {
                        enabled: false,
                        force_cleanup: false,
                        setup_script_timeout_seconds: 300,
                    },
                },
            });
        });
        await flushEffects();

        expect(appMock.GetTaskSchedulerSettings).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "settings-reset-delay")).toBe("11");
    });

    it("applies newer config updates directly and ignores stale versions", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("config:updated")?.(makeConfigUpdatedEvent(21, 5));
        });
        await flushEffects();

        expect(appMock.GetTaskSchedulerSettings).toHaveBeenCalledTimes(1);
        expect(getProbeText(container, "settings-reset-delay")).toBe("21");

        act(() => {
            eventHandlers.get("config:updated")?.(makeConfigUpdatedEvent(99, 4));
        });
        await flushEffects();

        expect(appMock.GetTaskSchedulerSettings).toHaveBeenCalledTimes(1);
        expect(getProbeText(container, "settings-reset-delay")).toBe("21");
    });

    it("refreshes status when an updated event contains a malformed queue item", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        appMock.GetTaskSchedulerStatus.mockResolvedValueOnce({
            ...makeStatus("session-a", "completed"),
            items: [makeItem()],
        });
        act(() => {
            eventHandlers.get("task-scheduler:updated")?.({
                ...makeStatus("session-a", "running"),
                items: [makeItem({status: undefined})],
            });
        });
        await flushEffects();

        expect(appMock.GetTaskSchedulerStatus).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "run-status")).toBe("completed");
    });

    it("ignores stale status responses after the active session changes", async () => {
        const firstStatus = deferred<ReturnType<typeof makeStatus>>();
        const secondStatus = deferred<ReturnType<typeof makeStatus>>();
        appMock.GetTaskSchedulerStatus
            .mockReset()
            .mockReturnValueOnce(firstStatus.promise)
            .mockReturnValueOnce(secondStatus.promise);

        act(() => {
            root.render(<Probe/>);
        });

        mockActiveSession = "session-b";
        act(() => {
            root.render(<Probe/>);
        });

        await act(async () => {
            secondStatus.resolve(makeStatus("session-b", "running"));
            await secondStatus.promise;
        });
        expect(getProbeText(container, "run-status")).toBe("running");

        await act(async () => {
            firstStatus.resolve(makeStatus("session-a", "completed"));
            await firstStatus.promise;
        });
        expect(getProbeText(container, "run-status")).toBe("running");
    });

    it("drops stale events and refresh responses after same-name session recreation", async () => {
        const staleStatus = deferred<ReturnType<typeof makeStatus>>();
        appMock.GetTaskSchedulerStatus
            .mockReset()
            .mockResolvedValueOnce({...makeStatus("session-a", "idle"), generation_id: "gen-old"})
            .mockReturnValueOnce(staleStatus.promise)
            .mockResolvedValueOnce({...makeStatus("session-a", "running"), generation_id: "gen-new"});

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        mockSessions = [
            {
                id: 99,
                name: "session-a",
                windows: [{panes: [{id: "%9", index: 0, active: true, width: 80, height: 24}]}]
            },
            {id: 2, name: "session-b", windows: [{panes: [{id: "%2", index: 0, active: true, width: 80, height: 24}]}]},
        ];
        act(() => {
            root.render(<Probe/>);
        });

        act(() => {
            eventHandlers.get("task-scheduler:updated")?.({
                ...makeStatus("session-a", "completed"),
                generation_id: "gen-old",
            });
        });
        expect(getProbeText(container, "run-status")).toBe("");

        await act(async () => {
            staleStatus.resolve({...makeStatus("session-a", "failed"), generation_id: "gen-old"});
            await staleStatus.promise;
        });
        expect(getProbeText(container, "run-status")).toBe("running");

        await flushEffects();
        expect(getProbeText(container, "run-status")).toBe("running");
    });

    it("ignores stale stopped events after same-name session recreation", async () => {
        appMock.GetTaskSchedulerStatus
            .mockReset()
            .mockResolvedValueOnce({...makeStatus("session-a", "idle"), generation_id: "gen-old"})
            .mockResolvedValueOnce({...makeStatus("session-a", "running"), generation_id: "gen-new"});

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        mockSessions = [
            {
                id: 99,
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
            eventHandlers.get("task-scheduler:stopped")?.({
                session_name: "session-a",
                generation_id: "gen-old",
                reason: "old worker fatal",
            });
        });

        expect(appMock.GetTaskSchedulerStatus).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "run-status")).toBe("running");
        expect(getProbeText(container, "error")).toBe("");
    });

    it("keeps the stopped error banner after refresh succeeds", async () => {
        appMock.GetTaskSchedulerStatus
            .mockReset()
            .mockResolvedValueOnce({...makeStatus("session-a", "running"), generation_id: "gen-old"})
            .mockResolvedValueOnce({...makeStatus("session-a", "running"), generation_id: "gen-new"});

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("task-scheduler:stopped")?.({
                session_name: "session-a",
                generation_id: "gen-old",
                reason: "worker fatal",
            });
        });
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("running");
        expect(getProbeText(container, "error")).toBe("Task failed: worker fatal");
    });

    it("renders manual stop events as stopped instead of failed", async () => {
        appMock.GetTaskSchedulerStatus
            .mockReset()
            .mockResolvedValueOnce({...makeStatus("session-a", "running"), generation_id: "gen-old"})
            .mockResolvedValueOnce({...makeStatus("session-a", "idle"), generation_id: "gen-old"});

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("task-scheduler:stopped")?.({
                session_name: "session-a",
                generation_id: "gen-old",
                reason: "Stopped by user",
            });
        });
        await flushEffects();

        expect(getProbeText(container, "error")).toBe("Task stopped: Stopped by user");
    });

    it("keeps the stopped error banner when background settings reload fails", async () => {
        appMock.GetTaskSchedulerStatus
            .mockReset()
            .mockResolvedValueOnce({...makeStatus("session-a", "running"), generation_id: "gen-old"})
            .mockResolvedValueOnce({...makeStatus("session-a", "running"), generation_id: "gen-new"});

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        act(() => {
            eventHandlers.get("task-scheduler:stopped")?.({
                session_name: "session-a",
                generation_id: "gen-old",
                reason: "worker fatal",
            });
        });
        await flushEffects();

        appMock.GetTaskSchedulerSettings.mockRejectedValueOnce(new Error("settings failed"));
        act(() => {
            eventHandlers.get("config:updated")?.({});
        });
        await flushEffects();

        expect(getProbeText(container, "error")).toBe("Task failed: worker fatal");
    });

    it("adopts the refreshed generation after StartTaskScheduler succeeds", async () => {
        appMock.StartTaskScheduler.mockReset();
        appMock.StartTaskScheduler.mockResolvedValue(undefined);
        appMock.GetTaskSchedulerStatus
            .mockReset()
            .mockResolvedValueOnce({...makeStatus("session-a", "idle"), generation_id: "gen-old"})
            .mockResolvedValueOnce({...makeStatus("session-a", "running"), generation_id: "gen-new"});

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        await act(async () => {
            const ok = await hookResult!.start({
                pre_exec_enabled: false,
                pre_exec_reset_delay_s: 0,
                pre_exec_idle_timeout_s: 0,
                pre_exec_target_mode: "task_panes",
            }, []);
            expect(ok).toBe(true);
        });
        await flushEffects();

        expect(appMock.StartTaskScheduler).toHaveBeenCalledWith("session-a:1", {
            pre_exec_enabled: false,
            pre_exec_reset_delay_s: 0,
            pre_exec_idle_timeout_s: 0,
            pre_exec_target_mode: "task_panes",
        }, []);
        expect(getProbeText(container, "run-status")).toBe("running");

        act(() => {
            eventHandlers.get("task-scheduler:updated")?.({
                ...makeStatus("session-a", "paused"),
                generation_id: "gen-new",
            });
        });
        expect(getProbeText(container, "run-status")).toBe("paused");
    });

    it("adopts the refreshed generation after ResumeTaskScheduler succeeds", async () => {
        appMock.ResumeTaskScheduler.mockReset();
        appMock.ResumeTaskScheduler.mockResolvedValue(undefined);
        appMock.GetTaskSchedulerStatus
            .mockReset()
            .mockResolvedValueOnce({...makeStatus("session-a", "paused"), generation_id: "gen-old"})
            .mockResolvedValueOnce({...makeStatus("session-a", "running"), generation_id: "gen-new"});

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        await act(async () => {
            await hookResult!.resume();
        });
        await flushEffects();

        expect(getProbeText(container, "run-status")).toBe("running");

        act(() => {
            eventHandlers.get("task-scheduler:updated")?.({
                ...makeStatus("session-a", "completed"),
                generation_id: "gen-new",
            });
        });
        expect(getProbeText(container, "run-status")).toBe("completed");
    });

    it("reloads normalized settings after saveSettings succeeds", async () => {
        appMock.SaveTaskSchedulerSettings.mockReset();
        appMock.SaveTaskSchedulerSettings.mockResolvedValue(undefined);
        appMock.GetTaskSchedulerSettings
            .mockReset()
            .mockResolvedValueOnce(makeSettings(0))
            .mockResolvedValueOnce(makeSettings(7));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const input = makeSettings(99) as TaskSchedulerSettings;
        await act(async () => {
            const ok = await hookResult!.saveSettings(input);
            expect(ok).toBe(true);
        });
        await flushEffects();

        expect(appMock.SaveTaskSchedulerSettings).toHaveBeenCalledWith(input);
        expect(getProbeText(container, "settings-reset-delay")).toBe("7");
    });

    it("waits for the normalized settings reload before resolving saveSettings", async () => {
        const reloadedSettings = deferred<ReturnType<typeof makeSettings>>();
        appMock.SaveTaskSchedulerSettings.mockReset();
        appMock.SaveTaskSchedulerSettings.mockResolvedValue(undefined);
        appMock.GetTaskSchedulerSettings
            .mockReset()
            .mockResolvedValueOnce(makeSettings(0))
            .mockReturnValueOnce(reloadedSettings.promise);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        const input = makeSettings(99) as TaskSchedulerSettings;
        let resolved = false;
        let result = false;
        let savePromise!: Promise<boolean>;
        await act(async () => {
            savePromise = hookResult!.saveSettings(input).then((ok) => {
                resolved = true;
                result = ok;
                return ok;
            });
            await Promise.resolve();
        });

        expect(resolved).toBe(false);
        expect(getProbeText(container, "settings-reset-delay")).toBe("0");

        await act(async () => {
            reloadedSettings.resolve(makeSettings(7));
            await savePromise;
        });
        await flushEffects();

        expect(result).toBe(true);
        expect(getProbeText(container, "settings-reset-delay")).toBe("7");
    });

    it("applies the normalized settings reload even after the active session changes", async () => {
        const reloadedSettings = deferred<ReturnType<typeof makeSettings>>();
        appMock.SaveTaskSchedulerSettings.mockReset();
        appMock.SaveTaskSchedulerSettings.mockResolvedValue(undefined);
        appMock.GetTaskSchedulerSettings
            .mockReset()
            .mockResolvedValueOnce(makeSettings(0))
            .mockReturnValueOnce(reloadedSettings.promise);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        let savePromise!: Promise<boolean>;
        await act(async () => {
            savePromise = hookResult!.saveSettings(makeSettings(99) as TaskSchedulerSettings);
            await Promise.resolve();
        });

        mockActiveSession = "session-b";
        appMock.GetTaskSchedulerStatus.mockResolvedValueOnce(makeStatus("session-b", "idle"));
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        let result = true;
        await act(async () => {
            reloadedSettings.resolve(makeSettings(12));
            result = await savePromise;
        });
        await flushEffects();

        expect(result).toBe(true);
        expect(getProbeText(container, "settings-reset-delay")).toBe("12");
    });

    it("does not start a stale save reload when a newer invalid config update already requested settings", async () => {
        const saveRequest = deferred<void>();
        const configEventReload = deferred<ReturnType<typeof makeSettings>>();
        appMock.SaveTaskSchedulerSettings.mockReset();
        appMock.SaveTaskSchedulerSettings.mockReturnValue(saveRequest.promise);
        appMock.GetTaskSchedulerSettings
            .mockReset()
            .mockResolvedValueOnce(makeSettings(0))
            .mockReturnValueOnce(configEventReload.promise);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        let savePromise!: Promise<boolean>;
        await act(async () => {
            savePromise = hookResult!.saveSettings(makeSettings(99) as TaskSchedulerSettings);
            await Promise.resolve();
        });

        act(() => {
            eventHandlers.get("config:updated")?.({});
        });
        await flushEffects();

        let result = false;
        await act(async () => {
            saveRequest.resolve(undefined);
            result = await savePromise;
        });

        expect(result).toBe(true);
        expect(appMock.GetTaskSchedulerSettings).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "settings-reset-delay")).toBe("0");

        await act(async () => {
            configEventReload.resolve(makeSettings(21));
            await Promise.resolve();
        });
        await flushEffects();

        expect(getProbeText(container, "settings-reset-delay")).toBe("21");
    });

    it("keeps the settings error visible after a save failure even when status refresh succeeds", async () => {
        appMock.SaveTaskSchedulerSettings.mockReset();
        appMock.SaveTaskSchedulerSettings.mockRejectedValueOnce(new Error("save failed"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        await act(async () => {
            const ok = await hookResult!.saveSettings(makeSettings(99) as TaskSchedulerSettings);
            expect(ok).toBe(false);
        });
        expect(getProbeText(container, "error")).toBe("save failed");

        appMock.GetTaskSchedulerStatus.mockResolvedValueOnce(makeStatus("session-a", "idle"));
        await act(async () => {
            const ok = await hookResult!.refreshStatus();
            expect(ok).toBe(true);
        });

        expect(getProbeText(container, "error")).toBe("save failed");
    });

    it.each([
        {
            name: "stop",
            arrangeRefresh: () => {
                appMock.StopTaskScheduler.mockReset();
                appMock.StopTaskScheduler.mockResolvedValue(undefined);
                appMock.GetTaskSchedulerStatus
                    .mockReset()
                    .mockResolvedValueOnce(makeStatus("session-a", "running"))
                    .mockResolvedValueOnce(makeStatus("session-a", "idle"));
                return () => hookResult!.stop();
            },
        },
        {
            name: "remove",
            arrangeRefresh: () => {
                appMock.RemoveTaskSchedulerItem.mockReset();
                appMock.RemoveTaskSchedulerItem.mockResolvedValue(undefined);
                appMock.GetTaskSchedulerStatus
                    .mockReset()
                    .mockResolvedValueOnce(makeStatus("session-a", "idle"))
                    .mockResolvedValueOnce(makeStatus("session-a", "idle"));
                return () => hookResult!.removeItem("item-1");
            },
        },
        {
            name: "reorder",
            arrangeRefresh: () => {
                appMock.ReorderTaskSchedulerItems.mockReset();
                appMock.ReorderTaskSchedulerItems.mockResolvedValue(undefined);
                appMock.GetTaskSchedulerStatus
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

        expect(appMock.GetTaskSchedulerStatus).toHaveBeenCalledTimes(2);
    });

    it.each([
        {
            name: "pause",
            arrangeMutation: () => {
                appMock.PauseTaskScheduler.mockReset();
                appMock.PauseTaskScheduler.mockResolvedValue(undefined);
                return () => hookResult!.pause();
            },
        },
        {
            name: "add",
            arrangeMutation: () => {
                appMock.AddTaskSchedulerItem.mockReset();
                appMock.AddTaskSchedulerItem.mockResolvedValue(undefined);
                return () => hookResult!.addItem("Title", "Message", "%1", false, "");
            },
        },
        {
            name: "update",
            arrangeMutation: () => {
                appMock.UpdateTaskSchedulerItem.mockReset();
                appMock.UpdateTaskSchedulerItem.mockResolvedValue(undefined);
                return () => hookResult!.updateItem("item-1", "Title", "Message", "%1", false, "");
            },
        },
    ])("does not refresh status after $name succeeds", async ({arrangeMutation}) => {
        const invoke = arrangeMutation();

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        let result: unknown = true;
        await act(async () => {
            result = await invoke();
        });
        await flushEffects();

        expect(appMock.GetTaskSchedulerStatus).toHaveBeenCalledTimes(1);
        expect(getProbeText(container, "error")).toBe("");
        if (typeof result === "boolean") {
            expect(result).toBe(true);
        } else {
            expect(result).toBeUndefined();
        }
    });

    it.each([
        {
            name: "start",
            arrangeFailure: () => {
                appMock.StartTaskScheduler.mockReset();
                appMock.StartTaskScheduler.mockRejectedValueOnce(new Error("start failed"));
                return () => hookResult!.start({
                    pre_exec_enabled: false,
                    pre_exec_reset_delay_s: 0,
                    pre_exec_idle_timeout_s: 0,
                    pre_exec_target_mode: "task_panes",
                }, []);
            },
            expectedMessage: "start failed",
        },
        {
            name: "pause",
            arrangeFailure: () => {
                appMock.PauseTaskScheduler.mockReset();
                appMock.PauseTaskScheduler.mockRejectedValueOnce(new Error("pause failed"));
                return () => hookResult!.pause();
            },
            expectedMessage: "pause failed",
        },
        {
            name: "add",
            arrangeFailure: () => {
                appMock.AddTaskSchedulerItem.mockReset();
                appMock.AddTaskSchedulerItem.mockRejectedValueOnce(new Error("add failed"));
                return () => hookResult!.addItem("Title", "Message", "%1", false, "");
            },
            expectedMessage: "add failed",
        },
        {
            name: "update",
            arrangeFailure: () => {
                appMock.UpdateTaskSchedulerItem.mockReset();
                appMock.UpdateTaskSchedulerItem.mockRejectedValueOnce(new Error("update failed"));
                return () => hookResult!.updateItem("item-1", "Title", "Message", "%1", false, "");
            },
            expectedMessage: "update failed",
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

        let result: unknown = true;
        await act(async () => {
            result = await invoke();
        });

        if (typeof result === "boolean") {
            expect(result).toBe(false);
        } else {
            expect(result).toBeUndefined();
        }
        expect(getProbeText(container, "error")).toContain(expectedMessage);
    });

    it("ignores stale mutation failures after the user switches sessions", async () => {
        const switchSession = async (sessionName: "session-a" | "session-b") => {
            mockActiveSession = sessionName;
            appMock.GetTaskSchedulerStatus.mockResolvedValueOnce(makeStatus(sessionName, "idle"));
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
                readonly pending: ReturnType<typeof deferred<void>>;
                readonly invoke: () => Promise<unknown>;
                readonly expectedResult: unknown;
            };
        }> = [
            {
                name: "start",
                begin: () => {
                    const pending = deferred<void>();
                    appMock.StartTaskScheduler.mockReset().mockReturnValueOnce(pending.promise);
                    return {
                        pending,
                        invoke: () => hookResult!.start({
                            pre_exec_enabled: false,
                            pre_exec_reset_delay_s: 0,
                            pre_exec_idle_timeout_s: 0,
                            pre_exec_target_mode: "task_panes",
                        }, []),
                        expectedResult: false,
                    };
                },
            },
            {
                name: "stop",
                begin: () => {
                    const pending = deferred<void>();
                    appMock.StopTaskScheduler.mockReset().mockReturnValueOnce(pending.promise);
                    return {pending, invoke: () => hookResult!.stop(), expectedResult: undefined};
                },
            },
            {
                name: "pause",
                begin: () => {
                    const pending = deferred<void>();
                    appMock.PauseTaskScheduler.mockReset().mockReturnValueOnce(pending.promise);
                    return {pending, invoke: () => hookResult!.pause(), expectedResult: undefined};
                },
            },
            {
                name: "resume",
                begin: () => {
                    const pending = deferred<void>();
                    appMock.ResumeTaskScheduler.mockReset().mockReturnValueOnce(pending.promise);
                    return {pending, invoke: () => hookResult!.resume(), expectedResult: undefined};
                },
            },
            {
                name: "add",
                begin: () => {
                    const pending = deferred<void>();
                    appMock.AddTaskSchedulerItem.mockReset().mockReturnValueOnce(pending.promise);
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
                    const pending = deferred<void>();
                    appMock.RemoveTaskSchedulerItem.mockReset().mockReturnValueOnce(pending.promise);
                    return {pending, invoke: () => hookResult!.removeItem("item-1"), expectedResult: undefined};
                },
            },
            {
                name: "reorder",
                begin: () => {
                    const pending = deferred<void>();
                    appMock.ReorderTaskSchedulerItems.mockReset().mockReturnValueOnce(pending.promise);
                    return {pending, invoke: () => hookResult!.reorderItems(["item-1"]), expectedResult: undefined};
                },
            },
            {
                name: "update",
                begin: () => {
                    const pending = deferred<void>();
                    appMock.UpdateTaskSchedulerItem.mockReset().mockReturnValueOnce(pending.promise);
                    return {
                        pending,
                        invoke: () => hookResult!.updateItem("item-1", "Title", "Message", "%1", false, ""),
                        expectedResult: false,
                    };
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
        const startPending = deferred<void>();
        appMock.StartTaskScheduler.mockReset().mockReturnValueOnce(startPending.promise);
        appMock.GetTaskSchedulerStatus
            .mockReset()
            .mockResolvedValueOnce(makeStatus("session-a", "idle"))
            .mockResolvedValueOnce(makeStatus("session-b", "idle"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        let startPromise!: Promise<boolean>;
        await act(async () => {
            startPromise = hookResult!.start({
                pre_exec_enabled: false,
                pre_exec_reset_delay_s: 0,
                pre_exec_idle_timeout_s: 0,
                pre_exec_target_mode: "task_panes",
            }, []);
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

        expect(appMock.GetTaskSchedulerStatus).toHaveBeenCalledTimes(2);
        expect(getProbeText(container, "run-status")).toBe("idle");
        expect(getProbeText(container, "error")).toBe("");
    });

    it("does not surface a stale readiness failure after the session changes", async () => {
        const readinessPending = deferred<void>();
        appMock.CheckTaskSchedulerOrchestratorReady.mockReset().mockReturnValueOnce(readinessPending.promise);

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        let readinessPromise!: Promise<OrchestratorReadiness>;
        await act(async () => {
            readinessPromise = hookResult!.checkOrchestratorReady();
            await Promise.resolve();
        });

        expect(appMock.CheckTaskSchedulerOrchestratorReady).toHaveBeenCalledWith("session-a:1");

        mockActiveSession = "session-b";
        appMock.GetTaskSchedulerStatus.mockResolvedValueOnce(makeStatus("session-b", "idle"));
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        await act(async () => {
            readinessPending.reject(new Error("readiness failed"));
            await expect(readinessPromise).rejects.toThrow("readiness failed");
        });

        expect(getProbeText(container, "error")).toBe("");
    });

    it("clears stale queue status and surfaces an error when refreshStatus fails", async () => {
        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();
        expect(getProbeText(container, "run-status")).toBe("idle");

        appMock.GetTaskSchedulerStatus.mockRejectedValueOnce(new Error("backend offline"));
        await act(async () => {
            const ok = await hookResult!.refreshStatus();
            expect(ok).toBe(false);
        });

        expect(getProbeText(container, "run-status")).toBe("");
        expect(getProbeText(container, "error")).toBe("backend offline");
    });

    it("surfaces readiness check failures instead of returning a misleading fallback", async () => {
        appMock.CheckTaskSchedulerOrchestratorReady.mockRejectedValueOnce(new Error("snapshot unavailable"));

        act(() => {
            root.render(<Probe/>);
        });
        await flushEffects();

        await act(async () => {
            await expect(hookResult!.checkOrchestratorReady()).rejects.toThrow("snapshot unavailable");
        });

        expect(appMock.CheckTaskSchedulerOrchestratorReady).toHaveBeenCalledWith("session-a:1");
        expect(getProbeText(container, "error")).toBe("snapshot unavailable");
    });
});
