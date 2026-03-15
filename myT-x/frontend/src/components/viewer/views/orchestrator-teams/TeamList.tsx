import {useI18n} from "../../../../i18n";
import type {OrchestratorTeamDefinition} from "./types";

interface TeamListProps {
    teams: OrchestratorTeamDefinition[];
    selectedTeamID: string | null;
    activeSession: string | null;
    loading: boolean;
    onSelect: (teamID: string) => void;
    onNew: () => void;
    onEdit: (team: OrchestratorTeamDefinition) => void;
    onCopy: (team: OrchestratorTeamDefinition) => void;
    onDelete: (team: OrchestratorTeamDefinition) => void;
    onOpenStart: () => void;
    onMoveUp: (teamID: string) => void;
    onMoveDown: (teamID: string) => void;
}

const MAX_VISIBLE_CHIPS = 4;

export function TeamList({
    teams,
    selectedTeamID,
    activeSession,
    loading,
    onSelect,
    onNew,
    onEdit,
    onCopy,
    onDelete,
    onOpenStart,
    onMoveUp,
    onMoveDown,
}: TeamListProps) {
    const {t} = useI18n();

    return (
        <div className="orchestrator-teams-list">
            <div className="orchestrator-teams-toolbar">
                <button type="button" className="orchestrator-teams-primary-btn" onClick={onNew}>
                    {t("viewer.orchestratorTeams.list.new", "+ 新規")}
                </button>
                <button
                    type="button"
                    className="orchestrator-teams-start-btn"
                    onClick={onOpenStart}
                    disabled={selectedTeamID === null || activeSession === null}
                >
                    {t("viewer.orchestratorTeams.list.start", "開始")}
                </button>
            </div>

            {activeSession === null && (
                <div className="orchestrator-teams-inline-note">
                    {t("viewer.orchestratorTeams.list.noActiveSession", "アクティブなセッションが選択されるまで開始できません。")}
                </div>
            )}

            {loading ? (
                <div className="orchestrator-teams-empty">{t("viewer.orchestratorTeams.list.loading", "チームを読み込み中...")}</div>
            ) : teams.length === 0 ? (
                <div className="orchestrator-teams-empty">{t("viewer.orchestratorTeams.list.empty", "保存されたチームはありません。")}</div>
            ) : (
                <div className="orchestrator-teams-cards">
                    {teams.map((team, index) => (
                        <div
                            key={team.id}
                            className={`orchestrator-team-card${selectedTeamID === team.id ? " selected" : ""}`}
                            role="button"
                            tabIndex={0}
                            onClick={() => onSelect(team.id)}
                            onKeyDown={(event) => {
                                if (event.key === "Enter" || event.key === " ") {
                                    event.preventDefault();
                                    onSelect(team.id);
                                }
                            }}
                        >
                            <div className="orchestrator-team-card-info">
                                <span className="orchestrator-team-card-title">{team.name}</span>
                                <span className="orchestrator-team-card-meta">
                                    {t("viewer.orchestratorTeams.list.memberCount", "{count} メンバー", {count: team.members.length})}
                                </span>
                            </div>

                            <div className="orchestrator-team-member-list">
                                {team.members.length === 0 ? (
                                    <span className="orchestrator-team-member-empty">
                                        {t("viewer.orchestratorTeams.list.noMembers", "メンバーが設定されていません")}
                                    </span>
                                ) : (
                                    <>
                                        {team.members.slice(0, MAX_VISIBLE_CHIPS).map((member) => (
                                            <span key={member.id} className="orchestrator-team-member-chip">
                                                {member.pane_title}
                                            </span>
                                        ))}
                                        {team.members.length > MAX_VISIBLE_CHIPS && (
                                            <span className="orchestrator-team-member-chip more">
                                                +{team.members.length - MAX_VISIBLE_CHIPS}
                                            </span>
                                        )}
                                    </>
                                )}
                            </div>

                            <div className="orchestrator-team-card-actions">
                                <span className="orchestrator-team-card-tag">
                                    {selectedTeamID === team.id
                                        ? t("viewer.orchestratorTeams.list.selected", "選択中")
                                        : t("viewer.orchestratorTeams.list.stored", "保存済み")}
                                </span>
                                <span className={`orchestrator-team-card-tag ${team.storage_location === "project" ? "project" : "global"}`}>
                                    {team.storage_location === "project"
                                        ? t("viewer.orchestratorTeams.list.storageProject", "プロジェクト")
                                        : t("viewer.orchestratorTeams.list.storageGlobal", "グローバル")}
                                </span>
                                <button
                                    type="button"
                                    className="orchestrator-team-card-btn icon"
                                    disabled={(() => {
                                        const loc = team.storage_location ?? "global";
                                        const sameSource = teams.filter((t) => (t.storage_location ?? "global") === loc);
                                        return sameSource.indexOf(team) === 0;
                                    })()}
                                    onClick={(event) => {
                                        event.stopPropagation();
                                        onMoveUp(team.id);
                                    }}
                                    aria-label={t("viewer.orchestratorTeams.list.moveUp", "上に移動")}
                                >
                                    ▲
                                </button>
                                <button
                                    type="button"
                                    className="orchestrator-team-card-btn icon"
                                    disabled={(() => {
                                        const loc = team.storage_location ?? "global";
                                        const sameSource = teams.filter((t) => (t.storage_location ?? "global") === loc);
                                        return sameSource.indexOf(team) === sameSource.length - 1;
                                    })()}
                                    onClick={(event) => {
                                        event.stopPropagation();
                                        onMoveDown(team.id);
                                    }}
                                    aria-label={t("viewer.orchestratorTeams.list.moveDown", "下に移動")}
                                >
                                    ▼
                                </button>
                                <button
                                    type="button"
                                    className="orchestrator-team-card-btn"
                                    onClick={(event) => {
                                        event.stopPropagation();
                                        onEdit(team);
                                    }}
                                >
                                    {t("viewer.orchestratorTeams.list.edit", "編集")}
                                </button>
                                <button
                                    type="button"
                                    className="orchestrator-team-card-btn"
                                    onClick={(event) => {
                                        event.stopPropagation();
                                        onCopy(team);
                                    }}
                                >
                                    {t("viewer.orchestratorTeams.list.copy", "コピー")}
                                </button>
                                <button
                                    type="button"
                                    className="orchestrator-team-card-btn danger"
                                    onClick={(event) => {
                                        event.stopPropagation();
                                        onDelete(team);
                                    }}
                                >
                                    {t("viewer.orchestratorTeams.list.delete", "削除")}
                                </button>
                            </div>

                            {team.description && (
                                <div
                                    className="orchestrator-team-card-description"
                                    title={team.description}
                                >
                                    {team.description}
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
