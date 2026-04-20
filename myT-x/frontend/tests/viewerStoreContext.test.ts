import {describe, expect, it, beforeEach} from "vitest";
import type {ViewerContextMap} from "../src/components/viewer/viewerContext";
import {useViewerStore} from "../src/components/viewer/viewerStore";

describe("viewerStore viewContext", () => {
    const taskSchedulerContext: ViewerContextMap["task-scheduler"] = {
        kind: "task-scheduler-template",
        key: "template:review",
        name: "Review Template",
        message: "Review the diff carefully.",
        targetPaneID: "%1",
        clearBefore: false,
        clearCommand: "",
    };
    const orchestratorTeamsContext: ViewerContextMap["orchestrator-teams"] = {
        kind: "orchestrator-teams-add-term-member",
        addTermMemberPaneId: "%9",
    };

    beforeEach(() => {
        useViewerStore.setState({activeViewId: null, viewContext: null});
    });

    it("openViewWithContext sets activeViewId and viewContext", () => {
        useViewerStore.getState().openViewWithContext("task-scheduler", taskSchedulerContext);
        const state = useViewerStore.getState();
        expect(state.activeViewId).toBe("task-scheduler");
        expect(state.viewContext).toEqual(taskSchedulerContext);
    });

    it("closeView clears viewContext", () => {
        useViewerStore.getState().openViewWithContext("task-scheduler", taskSchedulerContext);
        useViewerStore.getState().closeView();
        const state = useViewerStore.getState();
        expect(state.activeViewId).toBeNull();
        expect(state.viewContext).toBeNull();
    });

    it("toggleView clears viewContext", () => {
        useViewerStore.getState().openViewWithContext("task-scheduler", taskSchedulerContext);
        useViewerStore.getState().toggleView("task-scheduler");
        const state = useViewerStore.getState();
        expect(state.activeViewId).toBeNull();
        expect(state.viewContext).toBeNull();
    });

    it("toggleView to different view clears viewContext", () => {
        useViewerStore.getState().openViewWithContext("task-scheduler", taskSchedulerContext);
        useViewerStore.getState().toggleView("orchestrator-teams");
        const state = useViewerStore.getState();
        expect(state.activeViewId).toBe("orchestrator-teams");
        expect(state.viewContext).toBeNull();
    });

    it("openViewWithContext always opens (does not toggle)", () => {
        useViewerStore.getState().openViewWithContext("orchestrator-teams", {kind: "orchestrator-teams-default"});
        useViewerStore.getState().openViewWithContext("orchestrator-teams", orchestratorTeamsContext);
        const state = useViewerStore.getState();
        expect(state.activeViewId).toBe("orchestrator-teams");
        expect(state.viewContext).toEqual(orchestratorTeamsContext);
    });
});
