import {act, type ReactNode} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {OrchestratorTeamsView} from "./OrchestratorTeamsView";
import {useViewerStore} from "../../viewerStore";

const handleInitAddTermMemberMock = vi.fn<(paneId: string) => void>();

vi.mock("../../../../i18n", () => ({
    useI18n: () => ({
        t: (_key: string, fallback: string) => fallback,
    }),
}));

vi.mock("../../../../stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: {sessions: Array<{name: string}>}) => unknown) => selector({
        sessions: [{name: "alpha"}],
    }),
}));

vi.mock("../../../../utils/session", () => ({
    resolveActiveWindow: () => null,
}));

vi.mock("../../../../hooks/useUnregisteredPanes", () => ({
    useUnregisteredPanes: () => ({
        context: null,
        unregisteredPanes: [],
    }),
}));

vi.mock("../shared/ViewerPanelShell", () => ({
    ViewerPanelShell: ({children, headerChildren, title}: {
        children: ReactNode;
        headerChildren?: ReactNode;
        title: string;
    }) => (
        <div>
            <header>
                <h2>{title}</h2>
                {headerChildren}
            </header>
            {children}
        </div>
    ),
}));

vi.mock("./useTeamCRUD", () => ({
    useTeamCRUD: () => ({
        screen: "list",
        error: null,
        notice: null,
        teams: [],
        selectedTeamID: null,
        activeSession: "alpha",
        loading: false,
        selectedTeam: null,
        teamDraft: null,
        memberDraft: null,
        saving: false,
        canSaveTeam: false,
        teamNameDuplicate: false,
        pendingDeleteTeam: null,
        unsavedDialogTitle: "",
        unsavedDialogMessage: "",
        unsavedDialogActions: [],
        pendingNavigation: null,
        addTermMemberPaneId: null,
        addTermMemberDraft: null,
        refresh: vi.fn(),
        handleGuardedClose: vi.fn(),
        setError: vi.fn(),
        setNotice: vi.fn(),
        setSelectedTeamID: vi.fn(),
        handleNewTeam: vi.fn(),
        handleEditTeam: vi.fn(),
        handleCopyTeam: vi.fn(),
        handleRequestDelete: vi.fn(),
        setScreen: vi.fn(),
        handleOpenUnaffiliated: vi.fn(),
        moveTeamUp: vi.fn().mockResolvedValue(undefined),
        moveTeamDown: vi.fn().mockResolvedValue(undefined),
        setTeamDraft: vi.fn(),
        handleTeamBack: vi.fn(),
        handleSaveTeam: vi.fn(),
        handleOpenNewMember: vi.fn(),
        handleOpenCopyMember: vi.fn(),
        handleEditMember: vi.fn(),
        handleDeleteMember: vi.fn(),
        setMemberDraft: vi.fn(),
        handleSaveMember: vi.fn(),
        handleAddCopiedMembers: vi.fn(),
        returnToList: vi.fn(),
        handleStartTeam: vi.fn(),
        ensureUnaffiliatedTeam: vi.fn(),
        handleAddTermMemberNewMember: vi.fn(),
        handleAddTermMemberPickMember: vi.fn(),
        handleAddTermMemberQuickStart: vi.fn(),
        handleAddTermMemberPickDone: vi.fn(),
        setAddTermMemberDraft: vi.fn(),
        handleBootstrapMemberToPane: vi.fn(),
        handleQuickBootstrap: vi.fn(),
        setPendingDeleteTeam: vi.fn(),
        handleConfirmDelete: vi.fn(),
        handleUnsavedAction: vi.fn(),
        setPendingNavigation: vi.fn(),
        enlistPane: vi.fn(),
        handleInitAddTermMember: handleInitAddTermMemberMock,
    }),
}));

vi.mock("../../../ConfirmDialog", () => ({
    ConfirmDialog: () => null,
}));

vi.mock("../../../canvas/EnlistPaneModal", () => ({
    EnlistPaneModal: () => null,
}));

vi.mock("./AddTermMemberQuickScreen", () => ({
    AddTermMemberQuickScreen: () => null,
}));

vi.mock("./AddTermMemberScreen", () => ({
    AddTermMemberScreen: () => null,
}));

vi.mock("./AddTermMemberSourceScreen", () => ({
    AddTermMemberSourceScreen: () => null,
}));

vi.mock("./MemberEditor", () => ({
    MemberEditor: () => null,
}));

vi.mock("./MemberPicker", () => ({
    MemberPicker: () => null,
}));

vi.mock("./StartDialog", () => ({
    StartDialog: () => null,
}));

vi.mock("./TeamEditor", () => ({
    TeamEditor: () => null,
}));

vi.mock("./TeamList", () => ({
    TeamList: () => <div data-testid="team-list">team-list</div>,
}));

vi.mock("./orchestratorTeamUtils", () => ({
    isSystemTeam: () => false,
}));

describe("OrchestratorTeamsView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
        handleInitAddTermMemberMock.mockReset();
        useViewerStore.setState({
            activeViewId: "orchestrator-teams",
            viewContext: {
                kind: "orchestrator-teams-add-term-member",
                addTermMemberPaneId: "%7",
            },
        });
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("consumes add-term-member context on the list screen and clears it", async () => {
        await act(async () => {
            root.render(<OrchestratorTeamsView/>);
            await Promise.resolve();
        });

        expect(handleInitAddTermMemberMock).toHaveBeenCalledWith("%7");
        expect(useViewerStore.getState().viewContext).toBeNull();
        expect(container.textContent).toContain("チーム");
        expect(container.textContent).toContain("（オーケストレータMCP専用機能です）");
        expect(container.querySelector("[data-testid='team-list']")).not.toBeNull();
    });
});
