import {act, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {PromptPresetsView} from "./PromptPresetsView";
import type {PromptPreset, PromptPresetDraft} from "./types";

const closeViewMock = vi.fn();
const refreshMock = vi.fn<() => Promise<void>>().mockResolvedValue();
const savePresetMock = vi.fn();
const deletePresetMock = vi.fn();
const moveUpMock = vi.fn();
const moveDownMock = vi.fn();
const setErrorMock = vi.fn();
const setWarningMock = vi.fn();
let hookState: {
    presets: PromptPreset[];
    loading: boolean;
    error: string | null;
    warning: string | null;
    activeSession: string | null;
};

vi.mock("../../viewerStore", () => ({
    useViewerStore: (selector: (state: {closeView: () => void}) => unknown) => selector({closeView: closeViewMock}),
}));

vi.mock("./usePromptPresets", () => ({
    usePromptPresets: () => ({
        ...hookState,
        refresh: refreshMock,
        savePreset: savePresetMock,
        deletePreset: deletePresetMock,
        moveUp: moveUpMock,
        moveDown: moveDownMock,
        setError: setErrorMock,
        setWarning: setWarningMock,
    }),
}));

vi.mock("../shared/ViewerPanelShell", () => ({
    ViewerPanelShell: ({
        children,
        onClose,
        onRefresh,
    }: {
        children: ReactNode;
        onClose: () => void;
        onRefresh?: () => void;
    }) => (
        <div>
            <button type="button" onClick={onClose}>Shell Close</button>
            <button type="button" onClick={onRefresh}>Shell Refresh</button>
            {children}
        </div>
    ),
}));

vi.mock("../../../ConfirmDialog", () => ({
    ConfirmDialog: ({
        open,
        title,
        message,
        actions,
        onAction,
        onClose,
    }: {
        open: boolean;
        title: string;
        message: string;
        actions: Array<{label: string; value: string}>;
        onAction: (value: string) => void;
        onClose: () => void;
    }) => open ? (
        <div>
            <div>{title}</div>
            <div>{message}</div>
            <button type="button" onClick={onClose}>Cancel</button>
            {actions.map((action) => (
                <button key={action.value} type="button" onClick={() => onAction(action.value)}>
                    {action.label}
                </button>
            ))}
        </div>
    ) : null,
}));

vi.mock("./PromptPresetEditor", () => ({
    PromptPresetEditor: ({
        draft,
        onChange,
        onBack,
        onSave,
    }: {
        draft: PromptPresetDraft;
        onChange: (draft: PromptPresetDraft) => void;
        onBack: () => void;
        onSave: () => void;
    }) => (
        <div>
            <button type="button" onClick={onBack}>Back</button>
            <button
                type="button"
                onClick={() => onChange({
                    ...draft,
                    body: "Draft body",
                    name: "Dirty preset",
                })}
            >
                Make Dirty
            </button>
            <button type="button" onClick={onSave}>Save</button>
        </div>
    ),
}));

describe("PromptPresetsView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        closeViewMock.mockReset();
        refreshMock.mockClear();
        savePresetMock.mockReset();
        deletePresetMock.mockReset().mockResolvedValue(undefined);
        moveUpMock.mockReset().mockResolvedValue(undefined);
        moveDownMock.mockReset().mockResolvedValue(undefined);
        setErrorMock.mockReset();
        setWarningMock.mockReset();
        hookState = {
            presets: [],
            loading: false,
            error: null,
            warning: null,
            activeSession: null,
        };
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("asks for confirmation before discarding an unsaved draft on back", () => {
        act(() => {
            root.render(<PromptPresetsView/>);
        });

        const newButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent?.includes("+ New"));
        if (!(newButton instanceof HTMLButtonElement)) {
            throw new Error("expected + New button");
        }

        act(() => {
            newButton.click();
        });

        const dirtyButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent === "Make Dirty");
        if (!(dirtyButton instanceof HTMLButtonElement)) {
            throw new Error("expected Make Dirty button");
        }

        act(() => {
            dirtyButton.click();
        });

        const backButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent === "Back");
        if (!(backButton instanceof HTMLButtonElement)) {
            throw new Error("expected Back button");
        }

        act(() => {
            backButton.click();
        });

        expect(container.textContent).toContain("Unsaved prompt preset");
        expect(closeViewMock).not.toHaveBeenCalled();
    });

    it("asks for confirmation before closing an unsaved draft", () => {
        act(() => {
            root.render(<PromptPresetsView/>);
        });

        const buttons = Array.from(container.querySelectorAll("button"));
        const newButton = buttons.find((button) => button.textContent?.includes("+ New"));
        const closeButton = buttons.find((button) => button.textContent === "Shell Close");
        if (!(newButton instanceof HTMLButtonElement) || !(closeButton instanceof HTMLButtonElement)) {
            throw new Error("expected shell buttons");
        }

        act(() => {
            newButton.click();
        });

        const dirtyButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent === "Make Dirty");
        if (!(dirtyButton instanceof HTMLButtonElement)) {
            throw new Error("expected Make Dirty button");
        }

        act(() => {
            dirtyButton.click();
        });

        act(() => {
            closeButton.click();
        });

        expect(closeViewMock).not.toHaveBeenCalled();
        const discardButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent === "Discard and close");
        if (!(discardButton instanceof HTMLButtonElement)) {
            throw new Error("expected discard button");
        }

        act(() => {
            discardButton.click();
        });

        expect(closeViewMock).toHaveBeenCalledTimes(1);
    });

    it("keeps the original project session when saving after the active session changes", async () => {
        hookState = {
            presets: [{
                id: "project-preset",
                name: "Project preset",
                body: "Run tests first.",
                order: 0,
                storage_location: "project",
            }],
            loading: false,
            error: null,
            warning: null,
            activeSession: "alpha",
        };

        act(() => {
            root.render(<PromptPresetsView/>);
        });

        const editButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent === "Edit");
        if (!(editButton instanceof HTMLButtonElement)) {
            throw new Error("expected Edit button");
        }

        act(() => {
            editButton.click();
        });

        hookState = {
            ...hookState,
            activeSession: "beta",
        };

        act(() => {
            root.render(<PromptPresetsView/>);
        });

        const saveButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent === "Save");
        if (!(saveButton instanceof HTMLButtonElement)) {
            throw new Error("expected Save button");
        }

        await act(async () => {
            saveButton.click();
            await Promise.resolve();
        });

        expect(savePresetMock).toHaveBeenCalledTimes(1);
        expect(savePresetMock.mock.calls[0]?.[0]).toMatchObject({
            storageLocation: "project",
            projectSessionName: "alpha",
        });
    });

    it("keeps the original project session when confirming delete after the active session changes", async () => {
        hookState = {
            presets: [{
                id: "project-preset",
                name: "Project preset",
                body: "Run tests first.",
                order: 0,
                storage_location: "project",
            }],
            loading: false,
            error: null,
            warning: null,
            activeSession: "alpha",
        };

        act(() => {
            root.render(<PromptPresetsView/>);
        });

        const deleteButton = Array.from(container.querySelectorAll("button"))
            .find((button) => button.textContent === "Delete");
        if (!(deleteButton instanceof HTMLButtonElement)) {
            throw new Error("expected Delete button");
        }

        act(() => {
            deleteButton.click();
        });

        hookState = {
            ...hookState,
            activeSession: "beta",
        };

        act(() => {
            root.render(<PromptPresetsView/>);
        });

        const confirmButton = Array.from(container.querySelectorAll("button"))
            .filter((button) => button.textContent === "Delete")
            .at(-1);
        if (!(confirmButton instanceof HTMLButtonElement)) {
            throw new Error("expected confirmation Delete button");
        }

        await act(async () => {
            confirmButton.click();
            await Promise.resolve();
        });

        expect(deletePresetMock).toHaveBeenCalledTimes(1);
        expect(deletePresetMock).toHaveBeenCalledWith("project-preset", "project", "alpha");
    });
});
