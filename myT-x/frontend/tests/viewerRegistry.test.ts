import {beforeEach, describe, expect, it, vi} from "vitest";
import {getRegisteredViews, registerView, subscribeRegistry, type ViewPlugin} from "../src/components/viewer/viewerRegistry";

const DummyIcon = () => null;
const DummyComponent = () => null;

function plugin(id: string, overrides: Partial<ViewPlugin> = {}): ViewPlugin {
    return {
        id,
        icon: DummyIcon,
        label: id,
        component: DummyComponent,
        ...overrides,
    };
}

describe("viewerRegistry", () => {
    beforeEach(() => {
        const mutable = getRegisteredViews() as ViewPlugin[];
        mutable.splice(0, mutable.length);
    });

    it("defaults position to top", () => {
        registerView(plugin("error-log"));

        const views = getRegisteredViews();
        expect(views).toHaveLength(1);
        expect(views[0].position).toBe("top");
    });

    it("preserves explicit bottom position", () => {
        registerView(plugin("diff", {position: "bottom"}));

        const views = getRegisteredViews();
        expect(views).toHaveLength(1);
        expect(views[0].position).toBe("bottom");
    });

    it("falls back invalid runtime position to top", () => {
        const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
        try {
            registerView(plugin("invalid-position", {position: "left" as unknown as ViewPlugin["position"]}));
            const views = getRegisteredViews();
            expect(views[0].position).toBe("top");
        } finally {
            warnSpy.mockRestore();
        }
    });

    it("replaces existing view by id", () => {
        registerView(plugin("shared", {label: "first"}));
        registerView(plugin("shared", {label: "second"}));

        const views = getRegisteredViews();
        expect(views).toHaveLength(1);
        expect(views[0].label).toBe("second");
    });

    it("notifies subscribers on registry changes", () => {
        const listener = vi.fn();
        const unsubscribe = subscribeRegistry(listener);
        try {
            registerView(plugin("one"));
            registerView(plugin("one", {label: "updated"}));
            expect(listener).toHaveBeenCalledTimes(2);
        } finally {
            unsubscribe();
        }
    });
});
