import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    t: vi.fn((_: string, fallback: string) => fallback),
}));

vi.mock("../src/i18n", () => ({
    useI18n: () => ({
        language: "en",
        t: mocked.t,
    }),
}));

import {EnlistPaneModal} from "../src/components/canvas/EnlistPaneModal";
import type {
    EnlistPaneResult,
    OrchestratorSessionEnlistmentContext,
} from "../src/components/viewer/views/orchestrator-teams/types";
import type {PaneSnapshot} from "../src/types/tmux";

type ModalProps = Parameters<typeof EnlistPaneModal>[0];

const parentPane: PaneSnapshot = {id: "%1", index: 0, title: "Leader", active: true, width: 120, height: 40};
const targetPane: PaneSnapshot = {id: "%3", index: 2, title: "Fresh split", active: false, width: 120, height: 40};

function createContext(): OrchestratorSessionEnlistmentContext {
    return {
        teams: [
            {
                id: "team-alpha",
                name: "Alpha",
                description: "Delivery team",
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
                id: "team-project",
                name: "Project",
                description: "Project-scoped team",
                order: 1,
                storage_location: "project",
                members: [{
                    id: "member-project",
                    team_id: "team-project",
                    order: 0,
                    pane_title: "Planner",
                    role: "Planner",
                    command: "codex",
                    args: [],
                    custom_message: "",
                    skills: [{name: "React", description: "UI delivery"}],
                }],
            },
            {
                id: "__unaffiliated__",
                name: "Unaffiliated",
                description: "Saved templates",
                order: 2,
                storage_location: "global",
                members: [{
                    id: "member-ops",
                    team_id: "__unaffiliated__",
                    order: 0,
                    pane_title: "Ops helper",
                    role: "Operations",
                    command: "claude",
                    args: [],
                    custom_message: "",
                    skills: [{name: "Shell", description: "Operations support"}],
                }],
            },
        ],
        unaffiliated_members: [],
        role_catalog: ["Lead engineer", "Planner", "Operations"],
        skill_catalog: [
            {name: "Go", description: "Service design"},
            {name: "React", description: "UI delivery"},
            {name: "Shell", description: "Operations support"},
        ],
        registered_pane_ids: ["%1", "%2"],
    };
}

function findButtonByText(container: HTMLElement, label: string): HTMLButtonElement | null {
    return Array.from(container.querySelectorAll("button"))
        .find((candidate) => candidate.textContent?.includes(label)) as HTMLButtonElement | undefined
        ?? null;
}

function setControlValue(
    element: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement,
    value: string,
): void {
    const prototype = Object.getPrototypeOf(element);
    const descriptor = Object.getOwnPropertyDescriptor(prototype, "value");
    descriptor?.set?.call(element, value);
    element.dispatchEvent(new Event("input", {bubbles: true}));
    element.dispatchEvent(new Event("change", {bubbles: true}));
}

async function flushEffects(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
    });
}

describe("EnlistPaneModal", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        mocked.t.mockImplementation((_: string, fallback: string) => fallback);
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

    async function renderModal(overrides: Partial<ModalProps> = {}): Promise<ModalProps> {
        const props: ModalProps = {
            open: true,
            sessionName: "alpha",
            pane: targetPane,
            parentPane,
            context: createContext(),
            suggestedTeamID: "team-alpha",
            suggestedStorageLocation: "global",
            suggestedRole: "Lead engineer",
            onClose: vi.fn(),
            onEnlist: vi.fn(async () => ({warnings: []} as EnlistPaneResult)),
            ...overrides,
        };

        act(() => {
            root.render(<EnlistPaneModal {...props} />);
        });
        await flushEffects();
        return props;
    }

    it("renders team, role, skill, and template context from the saved session data", async () => {
        await renderModal();

        const teamSelect = container.querySelector("#enlist-team-select") as HTMLSelectElement | null;
        expect(teamSelect).not.toBeNull();
        const optionLabels = Array.from(teamSelect?.options ?? []).map((option) => option.textContent);
        expect(optionLabels).toContain("Alpha (global)");
        expect(optionLabels).toContain("Project (project)");
        expect(optionLabels).toContain("Unaffiliated (global)");

        const roleOptions = Array.from(container.querySelectorAll("datalist option")).map((option) => option.getAttribute("value"));
        expect(roleOptions).toContain("Lead engineer");
        expect(roleOptions).toContain("Operations");

        const skillSuggestions = Array.from(container.querySelectorAll(".orchestrator-team-suggestion-chip"))
            .map((chip) => chip.textContent?.trim());
        expect(skillSuggestions).toContain("Go");
        expect(skillSuggestions).toContain("React");

        const quickStartButton = findButtonByText(container, "Start with unaffiliated member");
        expect(quickStartButton).not.toBeNull();
        act(() => {
            quickStartButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await flushEffects();

        expect(container.textContent).toContain("Ops helper");
    });

    it("preselects the suggested team and storage pair", async () => {
        await renderModal({
            suggestedTeamID: "team-project",
            suggestedStorageLocation: "project",
            suggestedRole: "Planner",
        });

        const teamSelect = container.querySelector("#enlist-team-select") as HTMLSelectElement | null;
        expect(teamSelect?.value).toBe("team-project::project");
    });

    it("submits the normalized enlist request for the selected pane", async () => {
        const onClose = vi.fn();
        const onEnlist = vi.fn(async () => ({warnings: []} as EnlistPaneResult));
        await renderModal({onClose, onEnlist});

        const commandInput = container.querySelector('input[placeholder="codex"]') as HTMLInputElement | null;
        expect(commandInput).not.toBeNull();
        act(() => {
            setControlValue(commandInput!, " codex ");
        });

        const goSkillButton = findButtonByText(container, "Go");
        expect(goSkillButton).not.toBeNull();
        act(() => {
            goSkillButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        const submitButton = findButtonByText(container, "Register Pane");
        expect(submitButton?.disabled).toBe(false);
        await act(async () => {
            submitButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(onEnlist).toHaveBeenCalledTimes(1);
        expect(onEnlist.mock.calls[0]?.[0]).toMatchObject({
            session_name: "alpha",
            pane_id: "%3",
            team_id: "team-alpha",
            storage_location: "global",
            pane_state: "cli_running",
            bootstrap_delay_ms: 3000,
            member: {
                pane_title: "Fresh split",
                role: "Lead engineer",
                command: "codex",
                args: [],
                custom_message: "",
                skills: [{name: "Go", description: "Service design"}],
            },
        });
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("shows an error banner when enlistment fails", async () => {
        const onClose = vi.fn();
        const onEnlist = vi.fn(async () => {
            throw new Error("backend failed");
        });
        await renderModal({onClose, onEnlist});

        const commandInput = container.querySelector('input[placeholder="codex"]') as HTMLInputElement | null;
        expect(commandInput).not.toBeNull();
        act(() => {
            setControlValue(commandInput!, "codex");
        });

        const submitButton = findButtonByText(container, "Register Pane");
        expect(submitButton?.disabled).toBe(false);

        await act(async () => {
            submitButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(onEnlist).toHaveBeenCalledTimes(1);
        expect(onClose).not.toHaveBeenCalled();
        expect(container.textContent).toContain("backend failed");
    });
});
