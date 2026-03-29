import {useCallback, useEffect, useMemo, useState} from "react";
import type {Dispatch, SetStateAction} from "react";
import type {ConfirmAction} from "../../../ConfirmDialog";
import {useI18n} from "../../../../i18n";
import {notifyAndLog} from "../../../../utils/notifyUtils";
import {useViewerStore} from "../../viewerStore";
import type {
    OrchestratorLaunchMode,
    OrchestratorMemberDraft,
    OrchestratorStorageLocation,
    OrchestratorTeamDefinition,
    OrchestratorTeamDraft,
    PaneState,
} from "./types";
import {
    buildStartRequest,
    buildTeamPayload,
    copyMemberToDraft,
    createCopiedTeamDraft,
    createEmptyMemberDraft,
    createEmptyTeamDraft,
    createTeamDraft,
    DEFAULT_BOOTSTRAP_DELAY_MS,
    isMemberDraftValid,
    isSystemTeam,
    isTeamDraftValid,
    isTeamNameDuplicate,
    removeDraftMember,
    UNAFFILIATED_TEAM_ID,
    UNAFFILIATED_TEAM_NAME,
    upsertDraftMember,
} from "./orchestratorTeamUtils";
import {useOrchestratorTeams} from "./useOrchestratorTeams";

export type Screen = "list" | "team" | "member" | "start" | "copy-member"
    | "add-term-member" | "add-term-member-pick" | "add-term-member-edit" | "add-term-member-quick";
type PendingNavigationType = "team-back" | "member-back" | "team-close" | "member-close";

export interface UseTeamCRUDResult {
    // From useOrchestratorTeams
    readonly teams: OrchestratorTeamDefinition[];
    readonly activeSession: string | null;
    readonly error: string | null;
    readonly notice: string | null;
    readonly loading: boolean;
    readonly setError: Dispatch<SetStateAction<string | null>>;
    readonly setNotice: Dispatch<SetStateAction<string | null>>;
    readonly refresh: () => Promise<void>;
    readonly moveTeamUp: (teamID: string) => Promise<void>;
    readonly moveTeamDown: (teamID: string) => Promise<void>;
    // Screen routing
    readonly screen: Screen;
    readonly setScreen: Dispatch<SetStateAction<Screen>>;
    readonly selectedTeamID: string | null;
    readonly setSelectedTeamID: Dispatch<SetStateAction<string | null>>;
    readonly selectedTeam: OrchestratorTeamDefinition | null;
    // Team draft
    readonly teamDraft: OrchestratorTeamDraft | null;
    readonly setTeamDraft: Dispatch<SetStateAction<OrchestratorTeamDraft | null>>;
    readonly saving: boolean;
    readonly teamNameDuplicate: boolean;
    // Member draft
    readonly memberDraft: OrchestratorMemberDraft | null;
    readonly setMemberDraft: Dispatch<SetStateAction<OrchestratorMemberDraft | null>>;
    // CRUD handlers
    readonly handleNewTeam: () => void;
    readonly handleEditTeam: (team: OrchestratorTeamDefinition) => void;
    readonly handleCopyTeam: (team: OrchestratorTeamDefinition) => void;
    readonly handleRequestDelete: (team: OrchestratorTeamDefinition) => void;
    readonly handleConfirmDelete: () => Promise<void>;
    readonly pendingDeleteTeam: OrchestratorTeamDefinition | null;
    readonly setPendingDeleteTeam: Dispatch<SetStateAction<OrchestratorTeamDefinition | null>>;
    readonly handleSaveTeam: () => Promise<void>;
    readonly handleOpenNewMember: () => void;
    readonly handleEditMember: (index: number) => void;
    readonly handleSaveMember: () => void;
    readonly handleDeleteMember: (memberID: string) => void;
    readonly handleOpenCopyMember: () => void;
    readonly handleAddCopiedMembers: (members: OrchestratorMemberDraft[]) => void;
    // Launch
    readonly handleStartTeam: (launchMode: OrchestratorLaunchMode, newSessionName: string) => void;
    // Unaffiliated team
    readonly handleOpenUnaffiliated: () => Promise<void>;
    // addTermMember
    readonly addTermMemberPaneId: string | null;
    readonly addTermMemberDraft: OrchestratorMemberDraft | null;
    readonly setAddTermMemberDraft: Dispatch<SetStateAction<OrchestratorMemberDraft | null>>;
    readonly handleInitAddTermMember: (paneId: string) => void;
    readonly handleAddTermMemberNewMember: () => void;
    readonly handleAddTermMemberPickMember: () => void;
    readonly handleAddTermMemberPickDone: (drafts: OrchestratorMemberDraft[]) => void;
    readonly handleAddTermMemberQuickStart: () => void;
    readonly handleQuickBootstrap: (member: OrchestratorMemberDraft, paneState: PaneState, bootstrapDelayMs: number) => Promise<void>;
    readonly handleBootstrapMemberToPane: (paneState: PaneState, storageLocation: OrchestratorStorageLocation, bootstrapDelayMs: number) => Promise<void>;
    readonly ensureUnaffiliatedTeam: (storageLocation: OrchestratorStorageLocation) => Promise<OrchestratorTeamDefinition>;
    // Navigation
    readonly returnToList: () => void;
    readonly handleTeamBack: () => void;
    readonly handleMemberBack: () => void;
    readonly handleGuardedClose: () => void;
    // Unsaved dialog
    readonly pendingNavigation: PendingNavigationType | null;
    readonly setPendingNavigation: Dispatch<SetStateAction<PendingNavigationType | null>>;
    readonly unsavedDialogTitle: string;
    readonly unsavedDialogMessage: string;
    readonly unsavedDialogActions: ConfirmAction[];
    readonly handleUnsavedAction: (value: string) => Promise<void>;
}

/** Compares a draft object against its JSON snapshot to detect unsaved changes. */
function isDraftDirty(draft: unknown, snapshot: string | null): boolean {
    if (draft === null || snapshot === null) return false;
    return JSON.stringify(draft) !== snapshot;
}

export function useTeamCRUD(): UseTeamCRUDResult {
    const {t} = useI18n();
    const closeView = useViewerStore((state) => state.closeView);
    const {
        teams,
        activeSession,
        error,
        notice,
        loading,
        setError,
        setNotice,
        refresh,
        saveTeam,
        deleteTeam,
        startTeam,
        moveTeamUp,
        moveTeamDown,
        bootstrapMemberToPane,
        addMemberToUnaffiliatedTeam,
        ensureUnaffiliatedTeam,
    } = useOrchestratorTeams();

    const [screen, setScreen] = useState<Screen>("list");
    const [selectedTeamID, setSelectedTeamID] = useState<string | null>(null);
    const [teamDraft, setTeamDraft] = useState<OrchestratorTeamDraft | null>(null);
    const [memberDraft, setMemberDraft] = useState<OrchestratorMemberDraft | null>(null);
    const [memberEditIndex, setMemberEditIndex] = useState<number | null>(null);
    const [saving, setSaving] = useState(false);
    const [pendingDeleteTeam, setPendingDeleteTeam] = useState<OrchestratorTeamDefinition | null>(null);

    // --- addTermMember state ---
    const [addTermMemberPaneId, setAddTermMemberPaneId] = useState<string | null>(null);
    const [addTermMemberDraft, setAddTermMemberDraft] = useState<OrchestratorMemberDraft | null>(null);

    // --- dirty検出用スナップショット ---
    const [teamDraftSnapshot, setTeamDraftSnapshot] = useState<string | null>(null);
    const [memberDraftSnapshot, setMemberDraftSnapshot] = useState<string | null>(null);
    const [pendingNavigation, setPendingNavigation] = useState<PendingNavigationType | null>(null);

    const isTeamDirty = useMemo(
        () => isDraftDirty(teamDraft, teamDraftSnapshot),
        [teamDraft, teamDraftSnapshot],
    );

    const isMemberDirty = useMemo(
        () => isDraftDirty(memberDraft, memberDraftSnapshot),
        [memberDraft, memberDraftSnapshot],
    );

    useEffect(() => {
        const userTeams = teams.filter((team) => !isSystemTeam(team));
        if (userTeams.length === 0) {
            setSelectedTeamID(null);
            return;
        }
        if (selectedTeamID !== null && userTeams.some((team) => team.id === selectedTeamID)) {
            return;
        }
        setSelectedTeamID(userTeams[0].id);
    }, [selectedTeamID, teams]);

    const selectedTeam = useMemo<OrchestratorTeamDefinition | null>(() => {
        if (selectedTeamID === null) {
            return null;
        }
        return teams.find((team) => team.id === selectedTeamID) ?? null;
    }, [selectedTeamID, teams]);

    // --- Team CRUD handlers ---

    const openTeamEditor = useCallback((draft: OrchestratorTeamDraft) => {
        setError(null);
        setNotice(null);
        setTeamDraft(draft);
        setTeamDraftSnapshot(JSON.stringify(draft));
        setMemberDraft(null);
        setMemberDraftSnapshot(null);
        setMemberEditIndex(null);
        setScreen("team");
    }, [setError, setNotice]);

    const handleNewTeam = useCallback(() => {
        openTeamEditor(createEmptyTeamDraft());
    }, [openTeamEditor]);

    const handleEditTeam = useCallback((team: OrchestratorTeamDefinition) => {
        openTeamEditor(createTeamDraft(team));
    }, [openTeamEditor]);

    const handleCopyTeam = useCallback((team: OrchestratorTeamDefinition) => {
        openTeamEditor(createCopiedTeamDraft(team));
    }, [openTeamEditor]);

    const handleRequestDelete = useCallback((team: OrchestratorTeamDefinition) => {
        setPendingDeleteTeam(team);
    }, []);

    const handleConfirmDelete = useCallback(async () => {
        if (pendingDeleteTeam === null) return;
        const {id, storage_location} = pendingDeleteTeam;
        setPendingDeleteTeam(null);
        try {
            await deleteTeam(id, storage_location ?? "global");
        } catch (err) {
            console.warn("[orchestrator-teams] deleteTeam failed", err);
            notifyAndLog("Delete team", "error", err, "useTeamCRUD");
        }
    }, [deleteTeam, pendingDeleteTeam]);

    const returnToList = useCallback(() => {
        setScreen("list");
        setTeamDraftSnapshot(null);
        setMemberDraftSnapshot(null);
        void refresh();
    }, [refresh]);

    const handleSaveTeam = useCallback(async () => {
        if (teamDraft === null) {
            return;
        }
        setSaving(true);
        try {
            await saveTeam(teamDraft);
            setSelectedTeamID(teamDraft.id);
            setTeamDraftSnapshot(null);
            setMemberDraftSnapshot(null);
            setScreen("list");
        } catch (err) {
            console.warn("[orchestrator-teams] saveTeam failed", err);
            notifyAndLog("Save team", "error", err, "useTeamCRUD");
        } finally {
            setSaving(false);
        }
    }, [saveTeam, teamDraft]);

    // --- Member CRUD handlers ---

    const handleOpenNewMember = useCallback(() => {
        const newDraft = createEmptyMemberDraft();
        setMemberEditIndex(null);
        setMemberDraft(newDraft);
        setMemberDraftSnapshot(JSON.stringify(newDraft));
        setScreen("member");
    }, []);

    const handleEditMember = useCallback((index: number) => {
        if (teamDraft === null) {
            return;
        }
        const target = teamDraft.members[index];
        if (!target) {
            return;
        }
        setMemberEditIndex(index);
        setMemberDraft(target);
        setMemberDraftSnapshot(JSON.stringify(target));
        setScreen("member");
    }, [teamDraft]);

    const handleSaveMember = useCallback(() => {
        if (teamDraft === null || memberDraft === null) {
            return;
        }
        const updatedTeam = upsertDraftMember(teamDraft, memberDraft, memberEditIndex);
        setTeamDraft(updatedTeam);
        setTeamDraftSnapshot(JSON.stringify(updatedTeam));
        setMemberDraft(null);
        setMemberDraftSnapshot(null);
        setMemberEditIndex(null);
        setScreen("team");
    }, [memberEditIndex, teamDraft, memberDraft]);

    const handleDeleteMember = useCallback((memberID: string) => {
        if (teamDraft === null) {
            return;
        }
        setTeamDraft(removeDraftMember(teamDraft, memberID));
    }, [teamDraft]);

    const handleOpenCopyMember = useCallback(() => {
        setScreen("copy-member");
    }, []);

    const handleAddCopiedMembers = useCallback((members: OrchestratorMemberDraft[]) => {
        if (teamDraft === null) return;
        setTeamDraft({
            ...teamDraft,
            members: [...teamDraft.members, ...members],
        });
        setScreen("team");
    }, [teamDraft]);

    // --- Validation ---

    const teamNameDuplicate = useMemo(() => {
        if (teamDraft === null) return false;
        return isTeamNameDuplicate(teamDraft.name, teamDraft.id, teams, teamDraft.storageLocation);
    }, [teamDraft, teams]);

    // --- Launch ---

    const handleStartTeam = useCallback((launchMode: OrchestratorLaunchMode, newSessionName: string) => {
        if (selectedTeam === null) {
            return;
        }
        closeView();
        startTeam(buildStartRequest(selectedTeam.id, activeSession, launchMode, newSessionName))
            .catch((err) => {
                console.warn("[orchestrator-teams] startTeam failed", err);
                notifyAndLog("Start team", "error", err, "useTeamCRUD");
            });
    }, [activeSession, selectedTeam, startTeam, closeView]);

    // --- Unaffiliated team handler ---

    const handleOpenUnaffiliated = useCallback(async () => {
        setError(null);
        setNotice(null);
        try {
            const team = await ensureUnaffiliatedTeam("global");
            openTeamEditor(createTeamDraft(team));
        } catch (err) {
            console.warn("[orchestrator-teams] ensureUnaffiliatedTeam failed", err);
            notifyAndLog("Open unaffiliated team", "error", err, "useTeamCRUD");
        }
    }, [ensureUnaffiliatedTeam, openTeamEditor, setError, setNotice]);

    // --- addTermMember handlers ---

    const handleInitAddTermMember = useCallback((paneId: string) => {
        setAddTermMemberPaneId(paneId);
        setAddTermMemberDraft(null);
        setScreen("add-term-member");
    }, []);

    const handleAddTermMemberNewMember = useCallback(() => {
        setAddTermMemberDraft(createEmptyMemberDraft());
        setScreen("add-term-member-edit");
    }, []);

    const handleAddTermMemberPickMember = useCallback(() => {
        setScreen("add-term-member-pick");
    }, []);

    const handleAddTermMemberPickDone = useCallback((drafts: OrchestratorMemberDraft[]) => {
        if (drafts.length === 0) {
            setScreen("add-term-member");
            return;
        }
        setAddTermMemberDraft(drafts[0]);
        setScreen("add-term-member-edit");
    }, []);

    const handleAddTermMemberQuickStart = useCallback(() => {
        setScreen("add-term-member-quick");
    }, []);

    const handleQuickBootstrap = useCallback(async (
        member: OrchestratorMemberDraft,
        paneState: PaneState,
        bootstrapDelayMs: number,
    ) => {
        if (addTermMemberPaneId === null) return;
        setSaving(true);
        try {
            const payload = buildTeamPayload({
                id: UNAFFILIATED_TEAM_ID,
                name: UNAFFILIATED_TEAM_NAME,
                description: "",
                bootstrapDelayMs: DEFAULT_BOOTSTRAP_DELAY_MS,
                storageLocation: "global",
                members: [member],
            });
            const wireMember = payload.members[0];
            const result = await bootstrapMemberToPane({
                pane_id: addTermMemberPaneId,
                pane_state: paneState,
                team_name: UNAFFILIATED_TEAM_NAME,
                member: wireMember,
                bootstrap_delay_ms: bootstrapDelayMs,
                session_name: activeSession ?? "",
            });
            if (result.warnings.length > 0) {
                setNotice(result.warnings.join("\n"));
            }
            closeView();
        } catch (err) {
            console.warn("[orchestrator-teams] quickBootstrap failed", err);
            notifyAndLog("Quick bootstrap", "error", err, "useTeamCRUD");
        } finally {
            setSaving(false);
        }
    }, [addTermMemberPaneId, activeSession, bootstrapMemberToPane, closeView, setNotice]);

    const handleBootstrapMemberToPane = useCallback(async (
        paneState: PaneState,
        storageLocation: OrchestratorStorageLocation,
        bootstrapDelayMs: number,
    ) => {
        if (addTermMemberPaneId === null || addTermMemberDraft === null) return;
        setSaving(true);
        try {
            const payload = buildTeamPayload({
                id: UNAFFILIATED_TEAM_ID,
                name: UNAFFILIATED_TEAM_NAME,
                description: "",
                bootstrapDelayMs: DEFAULT_BOOTSTRAP_DELAY_MS,
                storageLocation,
                members: [addTermMemberDraft],
            });
            const wireMember = payload.members[0];

            await addMemberToUnaffiliatedTeam(wireMember, storageLocation);

            try {
                const result = await bootstrapMemberToPane({
                    pane_id: addTermMemberPaneId,
                    pane_state: paneState,
                    team_name: UNAFFILIATED_TEAM_NAME,
                    member: wireMember,
                    bootstrap_delay_ms: bootstrapDelayMs,
                    session_name: activeSession ?? "",
                });
                if (result.warnings.length > 0) {
                    setNotice(result.warnings.join("\n"));
                }
                closeView();
            } catch (bootstrapErr) {
                // Member was saved but bootstrap failed — inform the user.
                console.warn("[orchestrator-teams] bootstrap failed (member saved)", bootstrapErr);
                setNotice(t(
                    "viewer.orchestratorTeams.addTermMember.partialFailure",
                    "メンバーは保存されましたが、ブートストラップに失敗しました。「無所属メンバーで開始」から再実行できます。",
                ));
            }
        } catch (err) {
            console.warn("[orchestrator-teams] addMember/bootstrap failed", err);
            notifyAndLog("Bootstrap member", "error", err, "useTeamCRUD");
        } finally {
            setSaving(false);
        }
    }, [addTermMemberPaneId, addTermMemberDraft, activeSession, bootstrapMemberToPane, addMemberToUnaffiliatedTeam, closeView, setNotice, t]);

    // --- ガード付きナビゲーション ---

    const handleTeamBack = useCallback(() => {
        if (isTeamDirty) {
            setPendingNavigation("team-back");
            return;
        }
        returnToList();
    }, [isTeamDirty, returnToList]);

    const clearMemberAndReturnToTeam = useCallback(() => {
        setMemberDraft(null);
        setMemberDraftSnapshot(null);
        setMemberEditIndex(null);
        setScreen("team");
    }, []);

    const handleMemberBack = useCallback(() => {
        if (isMemberDirty) {
            setPendingNavigation("member-back");
            return;
        }
        clearMemberAndReturnToTeam();
    }, [isMemberDirty, clearMemberAndReturnToTeam]);

    const handleGuardedClose = useCallback(() => {
        if (screen === "member") {
            if (isMemberDirty) {
                setPendingNavigation("member-close");
                return;
            }
            if (isTeamDirty) {
                setPendingNavigation("team-close");
                return;
            }
        } else if (screen === "team") {
            if (isTeamDirty) {
                setPendingNavigation("team-close");
                return;
            }
        }
        closeView();
    }, [screen, isMemberDirty, isTeamDirty, closeView]);

    // --- 未保存確認ダイアログの構成 ---

    const canSaveTeam = useMemo(() => {
        if (teamDraft === null) return false;
        return isTeamDraftValid(teamDraft) && !teamNameDuplicate;
    }, [teamDraft, teamNameDuplicate]);

    const canSaveMember = useMemo(() => {
        if (memberDraft === null || teamDraft === null) return false;
        if (!isMemberDraftValid(memberDraft)) return false;
        const paneTitleDuplicate = memberDraft.paneTitle.trim() !== "" &&
            teamDraft.members
                .filter((m) => m.id !== memberDraft.id)
                .map((m) => m.paneTitle.trim())
                .filter((title) => title !== "")
                .some((title) => title === memberDraft.paneTitle.trim());
        return !paneTitleDuplicate;
    }, [memberDraft, teamDraft]);

    const canSaveAll = useMemo(() => {
        if (memberDraft === null || teamDraft === null) return false;
        if (!canSaveMember) return false;
        const hypothetical = upsertDraftMember(teamDraft, memberDraft, memberEditIndex);
        if (!isTeamDraftValid(hypothetical)) return false;
        return !isTeamNameDuplicate(hypothetical.name, hypothetical.id, teams, hypothetical.storageLocation);
    }, [memberDraft, teamDraft, memberEditIndex, teams, canSaveMember]);

    const {unsavedDialogTitle, unsavedDialogMessage, unsavedDialogActions} = useMemo(() => {
        const empty = {unsavedDialogTitle: "", unsavedDialogMessage: "", unsavedDialogActions: [] as ConfirmAction[]};
        if (!pendingNavigation) return empty;

        const isTeamNav = pendingNavigation === "team-back" || pendingNavigation === "team-close";
        const isBack = pendingNavigation === "team-back" || pendingNavigation === "member-back";

        const title = isTeamNav
            ? t("viewer.orchestratorTeams.unsaved.teamTitle", "チームの編集内容が未保存です")
            : t("viewer.orchestratorTeams.unsaved.memberTitle", "メンバーの編集内容が未反映です");

        const messageKeys: Record<PendingNavigationType, [string, string]> = {
            "team-back": ["viewer.orchestratorTeams.unsaved.teamMessage", "変更を保存せずに戻りますか？"],
            "team-close": ["viewer.orchestratorTeams.unsaved.teamCloseMessage", "変更を保存せずに閉じますか？"],
            "member-back": ["viewer.orchestratorTeams.unsaved.memberMessage", "変更を反映せずに戻りますか？"],
            "member-close": ["viewer.orchestratorTeams.unsaved.memberCloseMessage", "変更を保存せずに閉じますか？"],
        };
        const [msgKey, msgDefault] = messageKeys[pendingNavigation];

        const discardLabel = isBack
            ? t("viewer.orchestratorTeams.unsaved.discard", "破棄して戻る")
            : t("viewer.orchestratorTeams.unsaved.discardAndClose", "破棄して閉じる");

        const saveLabels: Record<PendingNavigationType, [string, string]> = {
            "team-back": ["viewer.orchestratorTeams.unsaved.saveAndBack", "保存して戻る"],
            "member-back": ["viewer.orchestratorTeams.unsaved.applyAndBack", "反映して戻る"],
            "team-close": ["viewer.orchestratorTeams.unsaved.saveAndClose", "保存して閉じる"],
            "member-close": ["viewer.orchestratorTeams.unsaved.saveAllAndClose", "すべて保存して閉じる"],
        };
        const canSaveMap: Record<PendingNavigationType, boolean> = {
            "team-back": canSaveTeam, "member-back": canSaveMember,
            "team-close": canSaveTeam, "member-close": canSaveAll,
        };

        const actions: ConfirmAction[] = [{label: discardLabel, value: "discard", variant: "danger"}];
        if (canSaveMap[pendingNavigation]) {
            const [saveKey, saveDefault] = saveLabels[pendingNavigation];
            actions.push({label: t(saveKey, saveDefault), value: "save", variant: "primary"});
        }

        return {unsavedDialogTitle: title, unsavedDialogMessage: t(msgKey, msgDefault), unsavedDialogActions: actions};
    }, [pendingNavigation, canSaveTeam, canSaveMember, canSaveAll, t]);

    const handleUnsavedAction = useCallback(async (value: string) => {
        const nav = pendingNavigation;
        if (nav === null) return;

        if (value === "discard") {
            setPendingNavigation(null);
            switch (nav) {
                case "team-back":
                    returnToList();
                    break;
                case "member-back":
                    clearMemberAndReturnToTeam();
                    break;
                case "team-close":
                case "member-close":
                    closeView();
                    break;
            }
            return;
        }

        if (value === "save") {
            // IMP-3: Keep dialog open until save succeeds — on failure the user
            // can retry without re-triggering the unsaved-changes flow.
            switch (nav) {
                case "team-back":
                    await handleSaveTeam();
                    setPendingNavigation(null);
                    break;
                case "member-back":
                    handleSaveMember();
                    setPendingNavigation(null);
                    break;
                case "team-close":
                    if (teamDraft === null) break;
                    setSaving(true);
                    try {
                        await saveTeam(teamDraft);
                        setPendingNavigation(null);
                        closeView();
                    } catch (err) {
                        console.warn("[orchestrator-teams] saveTeam failed", err);
                        notifyAndLog("Save team", "error", err, "useTeamCRUD");
                    } finally {
                        setSaving(false);
                    }
                    break;
                case "member-close":
                    if (memberDraft === null || teamDraft === null) break;
                    setSaving(true);
                    try {
                        const updatedTeam = upsertDraftMember(teamDraft, memberDraft, memberEditIndex);
                        await saveTeam(updatedTeam);
                        setPendingNavigation(null);
                        closeView();
                    } catch (err) {
                        console.warn("[orchestrator-teams] saveTeam failed", err);
                        notifyAndLog("Save team", "error", err, "useTeamCRUD");
                    } finally {
                        setSaving(false);
                    }
                    break;
            }
        }
    }, [pendingNavigation, returnToList, clearMemberAndReturnToTeam, closeView, handleSaveTeam, handleSaveMember, teamDraft, memberDraft, memberEditIndex, saveTeam]);

    return {
        // From useOrchestratorTeams
        teams,
        activeSession,
        error,
        notice,
        loading,
        setError,
        setNotice,
        refresh,
        moveTeamUp,
        moveTeamDown,
        // Screen routing
        screen,
        setScreen,
        selectedTeamID,
        setSelectedTeamID,
        selectedTeam,
        // Team draft
        teamDraft,
        setTeamDraft,
        saving,
        teamNameDuplicate,
        // Member draft
        memberDraft,
        setMemberDraft,
        // CRUD handlers
        handleNewTeam,
        handleEditTeam,
        handleCopyTeam,
        handleRequestDelete,
        handleConfirmDelete,
        pendingDeleteTeam,
        setPendingDeleteTeam,
        handleSaveTeam,
        handleOpenNewMember,
        handleEditMember,
        handleSaveMember,
        handleDeleteMember,
        handleOpenCopyMember,
        handleAddCopiedMembers,
        // Launch
        handleStartTeam,
        // Unaffiliated team
        handleOpenUnaffiliated,
        // addTermMember
        addTermMemberPaneId,
        addTermMemberDraft,
        setAddTermMemberDraft,
        handleInitAddTermMember,
        handleAddTermMemberNewMember,
        handleAddTermMemberPickMember,
        handleAddTermMemberPickDone,
        handleAddTermMemberQuickStart,
        handleQuickBootstrap,
        handleBootstrapMemberToPane,
        ensureUnaffiliatedTeam,
        // Navigation
        returnToList,
        handleTeamBack,
        handleMemberBack,
        handleGuardedClose,
        // Unsaved dialog
        pendingNavigation,
        setPendingNavigation,
        unsavedDialogTitle,
        unsavedDialogMessage,
        unsavedDialogActions,
        handleUnsavedAction,
    };
}
