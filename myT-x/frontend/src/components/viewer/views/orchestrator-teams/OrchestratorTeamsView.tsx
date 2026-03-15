import {useCallback, useEffect, useMemo, useState} from "react";
import {useI18n} from "../../../../i18n";
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
    isTeamNameDuplicate,
    removeDraftMember,
    upsertDraftMember,
    useOrchestratorTeams,
} from "./useOrchestratorTeams";

type Screen = "list" | "team" | "member" | "start" | "copy-member";

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
    const [starting, setStarting] = useState(false);
    const [pendingDeleteTeam, setPendingDeleteTeam] = useState<OrchestratorTeamDefinition | null>(null);

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
        setMemberDraft(null);
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
        } catch {
            // Errors are surfaced through the hook state.
        }
    }, [deleteTeam, pendingDeleteTeam]);

    const returnToList = useCallback(() => {
        setScreen("list");
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
            setScreen("list");
        } catch {
            // Errors are surfaced through the hook state.
        } finally {
            setSaving(false);
        }
    }, [saveTeam, teamDraft]);

    const handleOpenNewMember = useCallback(() => {
        setMemberEditIndex(null);
        setMemberDraft(createEmptyMemberDraft());
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
        setScreen("member");
    }, [teamDraft]);

    const handleSaveMember = useCallback((draft: OrchestratorMemberDraft) => {
        if (teamDraft === null) {
            return;
        }
        setTeamDraft(upsertDraftMember(teamDraft, draft, memberEditIndex));
        setMemberDraft(null);
        setMemberEditIndex(null);
        setScreen("team");
    }, [memberEditIndex, teamDraft]);

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
        startTeam(buildStartRequest(selectedTeam.id, activeSession, launchMode, newSessionName));
        closeView();
    }, [activeSession, selectedTeam, startTeam]);

    return (
        <ViewerPanelShell
            className="orchestrator-teams-view"
            title={t("viewer.orchestratorTeams.title", "チーム")}
            onClose={closeView}
            onRefresh={refresh}
        >
            <div className="orchestrator-teams-body">
                {error && (
                    <div className="orchestrator-teams-banner error">
                        <span>{error}</span>
                        <button type="button" onClick={() => setError(null)}>{t("viewer.orchestratorTeams.dismiss", "閉じる")}</button>
                    </div>
                )}
                {notice && (
                    <div className="orchestrator-teams-banner notice">
                        <span>{notice}</span>
                        <button type="button" onClick={() => setNotice(null)}>{t("viewer.orchestratorTeams.dismiss", "閉じる")}</button>
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
                        onMoveUp={(teamID) => void moveTeamUp(teamID)}
                        onMoveDown={(teamID) => void moveTeamDown(teamID)}
                    />
                )}

                {screen === "team" && teamDraft !== null && (
                    <TeamEditor
                        draft={teamDraft}
                        saving={saving}
                        teamNameDuplicate={teamNameDuplicate}
                        activeSession={activeSession}
                        onChange={setTeamDraft}
                        onBack={returnToList}
                        onSave={() => void handleSaveTeam()}
                        onAddMember={handleOpenNewMember}
                        onCopyMember={handleOpenCopyMember}
                        onEditMember={handleEditMember}
                        onDeleteMember={handleDeleteMember}
                    />
                )}

                {screen === "member" && memberDraft !== null && teamDraft !== null && (
                    <MemberEditor
                        initialDraft={memberDraft}
                        existingPaneTitles={teamDraft.members
                            .filter((m) => m.id !== memberDraft.id)
                            .map((m) => m.paneTitle.trim())
                            .filter((t) => t !== "")}
                        onBack={() => setScreen("team")}
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
                        starting={starting}
                        onBack={returnToList}
                        onStart={(launchMode, newSessionName) => void handleStartTeam(launchMode, newSessionName)}
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
        </ViewerPanelShell>
    );
}
