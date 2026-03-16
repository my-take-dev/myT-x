import {useCallback, useEffect, useMemo, useState} from "react";
import {useI18n} from "../../../../i18n";
import type {ConfirmAction} from "../../../ConfirmDialog";
import {ConfirmDialog} from "../../../ConfirmDialog";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {MemberEditor} from "./MemberEditor";
import {MemberPicker} from "./MemberPicker";
import {StartDialog} from "./StartDialog";
import {TeamEditor} from "./TeamEditor";
import {TeamList} from "./TeamList";
import type {
    OrchestratorLaunchMode,
    OrchestratorMemberDraft,
    OrchestratorTeamDefinition,
    OrchestratorTeamDraft,
} from "./types";
import {
    buildStartRequest,
    createCopiedTeamDraft,
    createEmptyMemberDraft,
    createEmptyTeamDraft,
    createTeamDraft,
    isMemberDraftValid,
    isTeamDraftValid,
    isTeamNameDuplicate,
    removeDraftMember,
    upsertDraftMember,
    useOrchestratorTeams,
} from "./useOrchestratorTeams";

type Screen = "list" | "team" | "member" | "start" | "copy-member";
type PendingNavigationType = "team-back" | "member-back" | "team-close" | "member-close";

export function OrchestratorTeamsView() {
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
    } = useOrchestratorTeams();
    const [screen, setScreen] = useState<Screen>("list");
    const [selectedTeamID, setSelectedTeamID] = useState<string | null>(null);
    const [teamDraft, setTeamDraft] = useState<OrchestratorTeamDraft | null>(null);
    const [memberDraft, setMemberDraft] = useState<OrchestratorMemberDraft | null>(null);
    const [memberEditIndex, setMemberEditIndex] = useState<number | null>(null);
    const [saving, setSaving] = useState(false);
    const [pendingDeleteTeam, setPendingDeleteTeam] = useState<OrchestratorTeamDefinition | null>(null);

    // --- dirty検出用スナップショット ---
    const [teamDraftSnapshot, setTeamDraftSnapshot] = useState<string | null>(null);
    const [memberDraftSnapshot, setMemberDraftSnapshot] = useState<string | null>(null);
    const [pendingNavigation, setPendingNavigation] = useState<PendingNavigationType | null>(null);

    const isTeamDirty = useMemo(() => {
        if (teamDraft === null || teamDraftSnapshot === null) return false;
        return JSON.stringify(teamDraft) !== teamDraftSnapshot;
    }, [teamDraft, teamDraftSnapshot]);

    const isMemberDirty = useMemo(() => {
        if (memberDraft === null || memberDraftSnapshot === null) return false;
        return JSON.stringify(memberDraft) !== memberDraftSnapshot;
    }, [memberDraft, memberDraftSnapshot]);

    useEffect(() => {
        if (teams.length === 0) {
            setSelectedTeamID(null);
            return;
        }
        if (selectedTeamID !== null && teams.some((team) => team.id === selectedTeamID)) {
            return;
        }
        setSelectedTeamID(teams[0].id);
    }, [selectedTeamID, teams]);

    const selectedTeam = useMemo<OrchestratorTeamDefinition | null>(() => {
        if (selectedTeamID === null) {
            return null;
        }
        return teams.find((team) => team.id === selectedTeamID) ?? null;
    }, [selectedTeamID, teams]);

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
            console.debug("[orchestrator-teams] deleteTeam failed", err);
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
            console.debug("[orchestrator-teams] saveTeam failed", err);
        } finally {
            setSaving(false);
        }
    }, [saveTeam, teamDraft]);

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

    const teamNameDuplicate = useMemo(() => {
        if (teamDraft === null) return false;
        return isTeamNameDuplicate(teamDraft.name, teamDraft.id, teams, teamDraft.storageLocation);
    }, [teamDraft, teams]);

    const handleStartTeam = useCallback((launchMode: OrchestratorLaunchMode, newSessionName: string) => {
        if (selectedTeam === null) {
            return;
        }
        closeView();
        startTeam(buildStartRequest(selectedTeam.id, activeSession, launchMode, newSessionName))
            .catch((err) => console.debug("[orchestrator-teams] startTeam failed", err));
    }, [activeSession, selectedTeam, startTeam, closeView]);

    // --- ガード付きナビゲーション ---

    const handleTeamBack = useCallback(() => {
        if (isTeamDirty) {
            setPendingNavigation("team-back");
            return;
        }
        returnToList();
    }, [isTeamDirty, returnToList]);

    const handleMemberBack = useCallback(() => {
        if (isMemberDirty) {
            setPendingNavigation("member-back");
            return;
        }
        setMemberDraft(null);
        setMemberDraftSnapshot(null);
        setMemberEditIndex(null);
        setScreen("team");
    }, [isMemberDirty]);

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

    const unsavedDialogTitle = useMemo(() => {
        if (pendingNavigation === "team-back" || pendingNavigation === "team-close") {
            return t("viewer.orchestratorTeams.unsaved.teamTitle", "チームの編集内容が未保存です");
        }
        if (pendingNavigation === "member-back" || pendingNavigation === "member-close") {
            return t("viewer.orchestratorTeams.unsaved.memberTitle", "メンバーの編集内容が未反映です");
        }
        return "";
    }, [pendingNavigation, t]);

    const unsavedDialogMessage = useMemo(() => {
        if (pendingNavigation === "team-back") {
            return t("viewer.orchestratorTeams.unsaved.teamMessage", "変更を保存せずに戻りますか？");
        }
        if (pendingNavigation === "team-close") {
            return t("viewer.orchestratorTeams.unsaved.teamCloseMessage", "変更を保存せずに閉じますか？");
        }
        if (pendingNavigation === "member-back") {
            return t("viewer.orchestratorTeams.unsaved.memberMessage", "変更を反映せずに戻りますか？");
        }
        if (pendingNavigation === "member-close") {
            return t("viewer.orchestratorTeams.unsaved.memberCloseMessage", "変更を保存せずに閉じますか？");
        }
        return "";
    }, [pendingNavigation, t]);

    const unsavedDialogActions = useMemo((): ConfirmAction[] => {
        const actions: ConfirmAction[] = [];
        switch (pendingNavigation) {
            case "team-back":
                actions.push({
                    label: t("viewer.orchestratorTeams.unsaved.discard", "破棄して戻る"),
                    value: "discard",
                    variant: "danger",
                });
                if (canSaveTeam) {
                    actions.push({
                        label: t("viewer.orchestratorTeams.unsaved.saveAndBack", "保存して戻る"),
                        value: "save",
                        variant: "primary",
                    });
                }
                break;
            case "member-back":
                actions.push({
                    label: t("viewer.orchestratorTeams.unsaved.discard", "破棄して戻る"),
                    value: "discard",
                    variant: "danger",
                });
                if (canSaveMember) {
                    actions.push({
                        label: t("viewer.orchestratorTeams.unsaved.applyAndBack", "反映して戻る"),
                        value: "save",
                        variant: "primary",
                    });
                }
                break;
            case "team-close":
                actions.push({
                    label: t("viewer.orchestratorTeams.unsaved.discardAndClose", "破棄して閉じる"),
                    value: "discard",
                    variant: "danger",
                });
                if (canSaveTeam) {
                    actions.push({
                        label: t("viewer.orchestratorTeams.unsaved.saveAndClose", "保存して閉じる"),
                        value: "save",
                        variant: "primary",
                    });
                }
                break;
            case "member-close":
                actions.push({
                    label: t("viewer.orchestratorTeams.unsaved.discardAndClose", "破棄して閉じる"),
                    value: "discard",
                    variant: "danger",
                });
                if (canSaveAll) {
                    actions.push({
                        label: t("viewer.orchestratorTeams.unsaved.saveAllAndClose", "すべて保存して閉じる"),
                        value: "save",
                        variant: "primary",
                    });
                }
                break;
        }
        return actions;
    }, [pendingNavigation, canSaveTeam, canSaveMember, canSaveAll, t]);

    const handleUnsavedAction = useCallback(async (value: string) => {
        const nav = pendingNavigation;
        if (nav === null) return;
        setPendingNavigation(null);

        if (value === "discard") {
            switch (nav) {
                case "team-back":
                    returnToList();
                    break;
                case "member-back":
                    setMemberDraft(null);
                    setMemberDraftSnapshot(null);
                    setMemberEditIndex(null);
                    setScreen("team");
                    break;
                case "team-close":
                case "member-close":
                    closeView();
                    break;
            }
            return;
        }

        if (value === "save") {
            switch (nav) {
                case "team-back":
                    await handleSaveTeam();
                    break;
                case "member-back":
                    handleSaveMember();
                    break;
                case "team-close":
                    if (teamDraft === null) break;
                    setSaving(true);
                    try {
                        await saveTeam(teamDraft);
                        closeView();
                    } catch (err) {
                        console.debug("[orchestrator-teams] saveTeam failed", err);
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
                        closeView();
                    } catch (err) {
                        console.debug("[orchestrator-teams] saveTeam failed", err);
                    } finally {
                        setSaving(false);
                    }
                    break;
            }
        }
    }, [pendingNavigation, returnToList, closeView, handleSaveTeam, handleSaveMember, teamDraft, memberDraft, memberEditIndex, saveTeam]);

    return (
        <ViewerPanelShell
            className="orchestrator-teams-view"
            title={t("viewer.orchestratorTeams.title", "チーム")}
            onClose={handleGuardedClose}
            onRefresh={refresh}
        >
            <div className="orchestrator-teams-body">
                {error && (
                    <div className="orchestrator-teams-banner error">
                        <span>{error}</span>
                        <button type="button"
                                onClick={() => setError(null)}>{t("viewer.orchestratorTeams.dismiss", "閉じる")}</button>
                    </div>
                )}
                {notice && (
                    <div className="orchestrator-teams-banner notice">
                        <span>{notice}</span>
                        <button type="button"
                                onClick={() => setNotice(null)}>{t("viewer.orchestratorTeams.dismiss", "閉じる")}</button>
                    </div>
                )}

                {screen === "list" && (
                    <TeamList
                        teams={teams}
                        selectedTeamID={selectedTeamID}
                        activeSession={activeSession}
                        loading={loading}
                        onSelect={setSelectedTeamID}
                        onNew={handleNewTeam}
                        onEdit={handleEditTeam}
                        onCopy={handleCopyTeam}
                        onDelete={handleRequestDelete}
                        onOpenStart={() => setScreen("start")}
                        onMoveUp={(teamID) => moveTeamUp(teamID).catch(() => { /* hook側でsetError済み */
                        })}
                        onMoveDown={(teamID) => moveTeamDown(teamID).catch(() => { /* hook側でsetError済み */
                        })}
                    />
                )}

                {screen === "team" && teamDraft !== null && (
                    <TeamEditor
                        draft={teamDraft}
                        saving={saving}
                        teamNameDuplicate={teamNameDuplicate}
                        activeSession={activeSession}
                        onChange={setTeamDraft}
                        onBack={handleTeamBack}
                        onSave={() => void handleSaveTeam()}
                        onAddMember={handleOpenNewMember}
                        onCopyMember={handleOpenCopyMember}
                        onEditMember={handleEditMember}
                        onDeleteMember={handleDeleteMember}
                    />
                )}

                {screen === "member" && memberDraft !== null && teamDraft !== null && (
                    <MemberEditor
                        draft={memberDraft}
                        existingPaneTitles={teamDraft.members
                            .filter((m) => m.id !== memberDraft.id)
                            .map((m) => m.paneTitle.trim())
                            .filter((title) => title !== "")}
                        onChange={setMemberDraft}
                        onBack={handleMemberBack}
                        onSave={handleSaveMember}
                    />
                )}

                {screen === "copy-member" && teamDraft !== null && (
                    <MemberPicker
                        teams={teams}
                        currentTeamID={teamDraft.id}
                        onBack={() => setScreen("team")}
                        onAdd={handleAddCopiedMembers}
                    />
                )}

                {screen === "start" && selectedTeam !== null && (
                    <StartDialog
                        team={selectedTeam}
                        activeSession={activeSession}
                        onBack={returnToList}
                        onStart={handleStartTeam}
                    />
                )}
            </div>

            <ConfirmDialog
                open={pendingDeleteTeam !== null}
                title={t("viewer.orchestratorTeams.delete.title", "チームの削除")}
                message={t(
                    "viewer.orchestratorTeams.delete.confirm",
                    "「{name}」（{location}）を削除しますか？この操作は元に戻せません。",
                    {
                        name: pendingDeleteTeam?.name ?? "",
                        location: pendingDeleteTeam?.storage_location === "project"
                            ? t("viewer.orchestratorTeams.list.storageProject", "プロジェクト")
                            : t("viewer.orchestratorTeams.list.storageGlobal", "グローバル"),
                    },
                )}
                actions={[{
                    label: t("viewer.orchestratorTeams.delete.confirmBtn", "削除"),
                    value: "delete",
                    variant: "danger",
                }]}
                onAction={(value) => {
                    if (value === "delete") void handleConfirmDelete();
                }}
                onClose={() => setPendingDeleteTeam(null)}
            />

            <ConfirmDialog
                open={pendingNavigation !== null}
                title={unsavedDialogTitle}
                message={unsavedDialogMessage}
                actions={unsavedDialogActions}
                onAction={(value) => void handleUnsavedAction(value)}
                onClose={() => setPendingNavigation(null)}
            />
        </ViewerPanelShell>
    );
}
