import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {setLanguage} from "../i18n";
import {SettingsModal} from "./SettingsModal";

const getConfigMock = vi.fn();
const getAllowedShellsMock = vi.fn();
const getValidationRulesMock = vi.fn();

vi.mock("../api", () => ({
    api: {
        GetConfig: () => getConfigMock(),
        GetAllowedShells: () => getAllowedShellsMock(),
        GetValidationRules: () => getValidationRulesMock(),
        SaveConfig: vi.fn(),
    },
}));

vi.mock("../hooks/useEscapeClose", () => ({
    useEscapeClose: vi.fn(),
}));

describe("SettingsModal", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        const pending = new Promise<never>(() => undefined);
        getConfigMock.mockReset();
        getAllowedShellsMock.mockReset();
        getValidationRulesMock.mockReset();
        getConfigMock.mockReturnValue(pending);
        getAllowedShellsMock.mockReturnValue(pending);
        getValidationRulesMock.mockReturnValue(pending);

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

    it("keeps the header, footer, and actions available while settings are loading", async () => {
        await act(async () => {
            root.render(<SettingsModal open onClose={vi.fn()} />);
        });

        const panel = container.querySelector(".settings-panel");
        const loading = container.querySelector(".settings-loading");
        const footer = container.querySelector(".settings-footer");
        const buttons = container.querySelectorAll(".settings-footer .modal-btn");
        if (!(panel instanceof HTMLDivElement) || !(loading instanceof HTMLDivElement) || !(footer instanceof HTMLDivElement)) {
            throw new Error("expected settings modal loading state");
        }

        expect(panel.querySelector(".modal-header")).not.toBeNull();
        expect(loading.textContent).toContain("Loading settings");
        expect(footer.textContent).toContain("Cancel");
        expect(footer.textContent).toContain("Save");
        expect(buttons).toHaveLength(2);
    });
});
