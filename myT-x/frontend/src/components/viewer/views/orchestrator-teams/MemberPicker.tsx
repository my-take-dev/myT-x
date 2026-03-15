import {useMemo, useState} from "react";
import {useI18n} from "../../../../i18n";
import type {OrchestratorMemberDraft, OrchestratorTeamDefinition} from "./types";
import {copyMemberToDraft} from "./useOrchestratorTeams";

interface MemberPickerProps {
    teams: OrchestratorTeamDefinition[];
    currentTeamID: string;
    onBack: () => void;
    onAdd: (members: OrchestratorMemberDraft[]) => void;
}

export function MemberPicker({teams, currentTeamID, onBack, onAdd}: MemberPickerProps) {
    const {t} = useI18n();
    const [selected, setSelected] = useState<Set<string>>(new Set());

    const otherTeams = useMemo(
        () => teams.filter((team) => team.id !== currentTeamID && team.members.length > 0),
        [teams, currentTeamID],
    );

    const toggleMember = (memberID: string) => {
        setSelected((prev) => {
            const next = new Set(prev);
            if (next.has(memberID)) {
                next.delete(memberID);
            } else {
                next.add(memberID);
            }
            return next;
        });
    };

    const handleAdd = () => {
        const drafts: OrchestratorMemberDraft[] = [];
        for (const team of otherTeams) {
            for (const member of team.members) {
                if (selected.has(member.id)) {
                    drafts.push(copyMemberToDraft(member));
                }
            }
        }
        onAdd(drafts);
    };

    return (
        <div className="orchestrator-team-editor">
            <button type="button" className="orchestrator-teams-back-btn" onClick={onBack}>
                &larr; {t("viewer.orchestratorTeams.memberPicker.back", "戻る")}
            </button>

            <div className="orchestrator-team-editor-header">
                <div>
                    <div className="orchestrator-team-editor-title">
                        {t("viewer.orchestratorTeams.memberPicker.title", "他チームからメンバーをコピー")}
                    </div>
                    <div className="orchestrator-team-editor-subtitle">
                        {t("viewer.orchestratorTeams.memberPicker.description", "追加したいメンバーを選択してください。IDは新しく生成されます。")}
                    </div>
                </div>
            </div>

            {otherTeams.length === 0 ? (
                <div className="orchestrator-team-member-empty-panel">
                    {t("viewer.orchestratorTeams.memberPicker.noOtherTeams", "コピー可能なメンバーがいる他のチームがありません。")}
                </div>
            ) : (
                <div className="orchestrator-team-editor-members">
                    {otherTeams.map((team) => (
                        <div key={team.id}>
                            <div
                                className="orchestrator-team-editor-title"
                                style={{fontSize: "0.85rem", margin: "12px 0 4px 0", opacity: 0.7}}
                            >
                                {team.name}
                            </div>
                            {team.members.map((member) => (
                                <label
                                    key={member.id}
                                    className="orchestrator-team-editor-member"
                                    style={{cursor: "pointer", display: "flex", alignItems: "center", gap: "8px"}}
                                >
                                    <input
                                        type="checkbox"
                                        checked={selected.has(member.id)}
                                        onChange={() => toggleMember(member.id)}
                                    />
                                    <div className="orchestrator-team-editor-member-main" style={{flex: 1}}>
                                        <div className="orchestrator-team-editor-member-title-row">
                                            <span className="orchestrator-team-editor-member-title">
                                                {member.pane_title}
                                            </span>
                                        </div>
                                        <div className="orchestrator-team-editor-member-meta">
                                            <span>{member.role}</span>
                                            <span>{member.command}</span>
                                        </div>
                                    </div>
                                </label>
                            ))}
                        </div>
                    ))}
                </div>
            )}

            <div className="orchestrator-team-editor-footer">
                <button
                    type="button"
                    className="orchestrator-teams-primary-btn"
                    disabled={selected.size === 0}
                    onClick={handleAdd}
                >
                    {selected.size === 0
                        ? t("viewer.orchestratorTeams.memberPicker.addNone", "メンバーを選択してください")
                        : t("viewer.orchestratorTeams.memberPicker.add", "{count} 件のメンバーを追加", {count: selected.size})}
                </button>
            </div>
        </div>
    );
}
