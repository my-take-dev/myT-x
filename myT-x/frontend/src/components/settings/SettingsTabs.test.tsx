import {act, useReducer} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {setLanguage} from "../../i18n";
import {SettingsTabs} from "./SettingsTabs";
import {INITIAL_FORM, formReducer} from "./settingsReducer";
import type {FormState} from "./types";

vi.mock("./GeneralSettings", () => ({
    GeneralSettings: () => <div>General panel</div>,
}));

vi.mock("./KeybindSettings", () => ({
    KeybindSettings: () => <div>Keybind panel</div>,
}));

vi.mock("./WorktreeSettings", () => ({
    WorktreeSettings: () => <div>Worktree panel</div>,
}));

vi.mock("./AgentModelSettings", () => ({
    AgentModelSettings: () => <div>Agent Model panel</div>,
}));

vi.mock("./PaneEnvSettings", () => ({
    PaneEnvSettings: () => <div>Pane Env panel</div>,
}));

vi.mock("./ClaudeEnvSettings", () => ({
    ClaudeEnvSettings: () => <div>Claude Env panel</div>,
}));

function getSettingsBody(container: HTMLDivElement): HTMLDivElement {
    const body = container.querySelector(".settings-body");
    if (!(body instanceof HTMLDivElement)) {
        throw new Error("expected settings body");
    }
    return body;
}

function renderTabs(root: Root, state: FormState): void {
    root.render(<SettingsTabs s={state} dispatch={() => undefined} validationRules={null}/>);
}

describe("SettingsTabs", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        setLanguage("en");
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("resets the shared body scroll when a category button changes the active panel", async () => {
        function Harness() {
            const [state, dispatch] = useReducer(formReducer, INITIAL_FORM);
            return <SettingsTabs s={state} dispatch={dispatch} validationRules={null}/>;
        }

        await act(async () => {
            root.render(<Harness/>);
        });

        const body = getSettingsBody(container);
        body.scrollTop = 180;

        const worktreeTab = container.querySelector("#settings-tab-worktree");
        if (!(worktreeTab instanceof HTMLButtonElement)) {
            throw new Error("expected worktree tab");
        }

        await act(async () => {
            worktreeTab.click();
        });

        expect(getSettingsBody(container).scrollTop).toBe(0);
    });

    it("resets the shared body scroll when an external state change switches categories", async () => {
        const initialState: FormState = {...INITIAL_FORM, activeCategory: "general"};

        await act(async () => {
            renderTabs(root, initialState);
        });

        const body = getSettingsBody(container);
        body.scrollTop = 140;

        await act(async () => {
            renderTabs(root, {...initialState, activeCategory: "pane-env"});
        });

        expect(getSettingsBody(container).scrollTop).toBe(0);
    });
});
