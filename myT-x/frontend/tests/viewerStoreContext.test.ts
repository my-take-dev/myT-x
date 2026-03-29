import {describe, expect, it, beforeEach} from "vitest";
import {useViewerStore} from "../src/components/viewer/viewerStore";

describe("viewerStore viewContext", () => {
    beforeEach(() => {
        useViewerStore.setState({activeViewId: null, viewContext: null});
    });

    it("openViewWithContext sets activeViewId and viewContext", () => {
        useViewerStore.getState().openViewWithContext("test-view", {key: "value"});
        const state = useViewerStore.getState();
        expect(state.activeViewId).toBe("test-view");
        expect(state.viewContext).toEqual({key: "value"});
    });

    it("closeView clears viewContext", () => {
        useViewerStore.getState().openViewWithContext("test-view", {key: "value"});
        useViewerStore.getState().closeView();
        const state = useViewerStore.getState();
        expect(state.activeViewId).toBeNull();
        expect(state.viewContext).toBeNull();
    });

    it("toggleView clears viewContext", () => {
        useViewerStore.getState().openViewWithContext("test-view", {key: "value"});
        useViewerStore.getState().toggleView("test-view");
        const state = useViewerStore.getState();
        expect(state.activeViewId).toBeNull();
        expect(state.viewContext).toBeNull();
    });

    it("toggleView to different view clears viewContext", () => {
        useViewerStore.getState().openViewWithContext("view-a", {key: "value"});
        useViewerStore.getState().toggleView("view-b");
        const state = useViewerStore.getState();
        expect(state.activeViewId).toBe("view-b");
        expect(state.viewContext).toBeNull();
    });

    it("openViewWithContext always opens (does not toggle)", () => {
        useViewerStore.getState().openViewWithContext("test-view", {a: 1});
        useViewerStore.getState().openViewWithContext("test-view", {b: 2});
        const state = useViewerStore.getState();
        expect(state.activeViewId).toBe("test-view");
        expect(state.viewContext).toEqual({b: 2});
    });
});
