import {useEffect} from "react";
import {useI18n} from "../../../../i18n";
import {ConfirmDialog} from "../../../ConfirmDialog";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {AddTermMemberQuickScreen} from "./AddTermMemberQuickScreen";
import {AddTermMemberScreen} from "./AddTermMemberScreen";
import {AddTermMemberSourceScreen} from "./AddTermMemberSourceScreen";
import {MemberEditor} from "./MemberEditor";
import {MemberPicker} from "./MemberPicker";
import {StartDialog} from "./StartDialog";
import {TeamEditor} from "./TeamEditor";
import {TeamList} from "./TeamList";
import {isSystemTeam} from "./orchestratorTeamUtils";
import {useTeamCRUD} from "./useTeamCRUD";

export function OrchestratorTeamsView() {
    const {t} = useI18n();
    const crud = useTeamCRUD();
    const viewContext = useViewerStore((s) => s.viewContext);

    // Consume addTermMemberPaneId from viewContext when arriving at list screen.
    useEffect(() => {
        const paneId = viewContext?.addTermMemberPaneId;
        if (typeof paneId === "string" && crud.screen === "list") {
            crud.handleInitAddTermMember(paneId);
            // Clear context so it doesn't re-trigger.
            useViewerStore.setState({viewContext: null});
        }
    }, [viewContext, crud.screen, crud.handleInitAddTermMember]);

    return (
        <ViewerPanelShell
            className="orchestrator-teams-view"
            title={t("viewer.orchestratorTeams.title", "チーム")}
            onClose={crud.handleGuardedClose}
            onRefresh={crud.refresh}
        >
            <div className="orchestrator-teams-body">
                {crud.error && (
                    <div className="orchestrator-teams-banner error">
                        <span>{crud.error}</span>
                        <button type="button"
                                onClick={() => crud.setError(null)}>{t("viewer.orchestratorTeams.dismiss", "閉じる")}</button>
                    </div>
                )}
                {crud.notice && (
                    <div className="orchestrator-teams-banner notice">
                        <span>{crud.notice}</span>
                        <button type="button"
                                onClick={() => crud.setNotice(null)}>{t("viewer.orchestratorTeams.dismiss", "閉じる")}</button>
                    </div>
                )}

                {crud.screen === "list" && (
                    <TeamList
                        teams={crud.teams}
                        selectedTeamID={crud.selectedTeamID}
                        activeSession={crud.activeSession}
                        loading={crud.loading}
                        onSelect={crud.setSelectedTeamID}
                        onNew={crud.handleNewTeam}
                        onEdit={crud.handleEditTeam}
                        onCopy={crud.handleCopyTeam}
                        onDelete={crud.handleRequestDelete}
                        onOpenStart={() => crud.setScreen("start")}
                        onOpenUnaffiliated={() => void crud.handleOpenUnaffiliated()}
                        onMoveUp={(teamID) => crud.moveTeamUp(teamID).catch(() => { /* hook側でsetError済み */
                        })}
                        onMoveDown={(teamID) => crud.moveTeamDown(teamID).catch(() => { /* hook側でsetError済み */
                        })}
                    />
                )}

                {crud.screen === "team" && crud.teamDraft !== null && (
                    <TeamEditor
                        key={crud.teamDraft.id}
                        draft={crud.teamDraft}
                        saving={crud.saving}
                        canSave={crud.canSaveTeam}
                        teamNameDuplicate={crud.teamNameDuplicate}
                        activeSession={crud.activeSession}
                        systemTeam={isSystemTeam(crud.teamDraft.id)}
                        onChange={crud.setTeamDraft}
                        onBack={crud.handleTeamBack}
                        onSave={() => void crud.handleSaveTeam()}
                        onAddMember={crud.handleOpenNewMember}
                        onCopyMember={crud.handleOpenCopyMember}
                        onEditMember={crud.handleEditMember}
                        onDeleteMember={crud.handleDeleteMember}
                    />
                )}

                {crud.screen === "member" && crud.memberDraft !== null && crud.teamDraft !== null && (
                    <MemberEditor
                        draft={crud.memberDraft}
                        existingPaneTitles={crud.teamDraft.members
                            .filter((m) => m.id !== crud.memberDraft!.id)
                            .map((m) => m.paneTitle.trim())
                            .filter((title) => title !== "")}
                        onChange={crud.setMemberDraft}
                        onBack={crud.handleMemberBack}
                        onSave={crud.handleSaveMember}
                    />
                )}

                {crud.screen === "copy-member" && crud.teamDraft !== null && (
                    <MemberPicker
                        teams={crud.teams}
                        currentTeamID={crud.teamDraft.id}
                        onBack={() => crud.setScreen("team")}
                        onAdd={crud.handleAddCopiedMembers}
                    />
                )}

                {crud.screen === "start" && crud.selectedTeam !== null && (
                    <StartDialog
                        team={crud.selectedTeam}
                        activeSession={crud.activeSession}
                        onBack={crud.returnToList}
                        onStart={crud.handleStartTeam}
                    />
                )}

                {crud.screen === "add-term-member" && crud.addTermMemberPaneId !== null && (
                    <AddTermMemberSourceScreen
                        paneId={crud.addTermMemberPaneId}
                        ensureUnaffiliatedTeam={crud.ensureUnaffiliatedTeam}
                        onNewMember={crud.handleAddTermMemberNewMember}
                        onPickMember={crud.handleAddTermMemberPickMember}
                        onQuickStart={crud.handleAddTermMemberQuickStart}
                        onBack={crud.returnToList}
                    />
                )}

                {crud.screen === "add-term-member-pick" && (
                    <MemberPicker
                        teams={crud.teams}
                        currentTeamID=""
                        onBack={() => crud.setScreen("add-term-member")}
                        onAdd={crud.handleAddTermMemberPickDone}
                    />
                )}

                {crud.screen === "add-term-member-edit" && crud.addTermMemberDraft !== null && crud.addTermMemberPaneId !== null && (
                    <AddTermMemberScreen
                        paneId={crud.addTermMemberPaneId}
                        draft={crud.addTermMemberDraft}
                        saving={crud.saving}
                        onChange={crud.setAddTermMemberDraft}
                        onBack={() => crud.setScreen("add-term-member")}
                        onBootstrap={(paneState, storageLocation, bootstrapDelayMs) => {
                            void crud.handleBootstrapMemberToPane(paneState, storageLocation, bootstrapDelayMs);
                        }}
                    />
                )}

                {crud.screen === "add-term-member-quick" && crud.addTermMemberPaneId !== null && (
                    <AddTermMemberQuickScreen
                        paneId={crud.addTermMemberPaneId}
                        saving={crud.saving}
                        ensureUnaffiliatedTeam={crud.ensureUnaffiliatedTeam}
                        onBack={() => crud.setScreen("add-term-member")}
                        onBootstrap={(member, paneState, bootstrapDelayMs) => {
                            void crud.handleQuickBootstrap(member, paneState, bootstrapDelayMs);
                        }}
                    />
                )}
            </div>

            <ConfirmDialog
                open={crud.pendingDeleteTeam !== null}
                title={t("viewer.orchestratorTeams.delete.title", "チームの削除")}
                message={t(
                    "viewer.orchestratorTeams.delete.confirm",
                    "「{name}」（{location}）を削除しますか？この操作は元に戻せません。",
                    {
                        name: crud.pendingDeleteTeam?.name ?? "",
                        location: crud.pendingDeleteTeam?.storage_location === "project"
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
                    if (value === "delete") void crud.handleConfirmDelete();
                }}
                onClose={() => crud.setPendingDeleteTeam(null)}
            />

            <ConfirmDialog
                open={crud.pendingNavigation !== null}
                title={crud.unsavedDialogTitle}
                message={crud.unsavedDialogMessage}
                actions={crud.unsavedDialogActions}
                onAction={(value) => void crud.handleUnsavedAction(value)}
                onClose={() => crud.setPendingNavigation(null)}
            />
        </ViewerPanelShell>
    );
}
