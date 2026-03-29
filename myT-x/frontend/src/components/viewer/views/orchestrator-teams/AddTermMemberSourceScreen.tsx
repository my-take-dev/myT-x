import {useCallback, useEffect, useState} from "react";
import {useI18n} from "../../../../i18n";
import type {OrchestratorStorageLocation, OrchestratorTeamDefinition} from "./types";

interface AddTermMemberSourceScreenProps {
    paneId: string;
    ensureUnaffiliatedTeam: (storageLocation: OrchestratorStorageLocation) => Promise<OrchestratorTeamDefinition>;
    onNewMember: () => void;
    onPickMember: () => void;
    onQuickStart: () => void;
    onBack: () => void;
}

export function AddTermMemberSourceScreen({
    paneId,
    ensureUnaffiliatedTeam,
    onNewMember,
    onPickMember,
    onQuickStart,
    onBack,
}: AddTermMemberSourceScreenProps) {
    const {t} = useI18n();
    const [hasUnaffiliatedMembers, setHasUnaffiliatedMembers] = useState(false);
    const [checked, setChecked] = useState(false);

    const checkMembers = useCallback(async () => {
        try {
            const [globalResult, projectResult] = await Promise.allSettled([
                ensureUnaffiliatedTeam("global"),
                ensureUnaffiliatedTeam("project"),
            ]);
            const hasMembers =
                (globalResult.status === "fulfilled" && (globalResult.value.members?.length ?? 0) > 0) ||
                (projectResult.status === "fulfilled" && (projectResult.value.members?.length ?? 0) > 0);
            setHasUnaffiliatedMembers(hasMembers);
        } catch {
            setHasUnaffiliatedMembers(false);
        } finally {
            setChecked(true);
        }
    }, [ensureUnaffiliatedTeam]);

    useEffect(() => {
        void checkMembers();
    }, [checkMembers]);

    return (
        <div className="orchestrator-add-term-member-source">
            <button type="button" className="orchestrator-teams-back-btn" onClick={onBack}>
                &larr; {t("viewer.orchestratorTeams.addTermMemberSource.back", "戻る")}
            </button>

            <div className="orchestrator-teams-start-hero">
                <div className="orchestrator-team-card-tag">
                    {t("viewer.orchestratorTeams.addTermMemberSource.title", "メンバー追加")}
                </div>
                <h3>{paneId}</h3>
                <p>{t("viewer.orchestratorTeams.addTermMemberSource.description", "メンバーの追加方法を選択してください。")}</p>
            </div>

            <div className="orchestrator-teams-cards">
                <div
                    className="orchestrator-team-card"
                    role="button"
                    tabIndex={0}
                    onClick={onNewMember}
                    onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            onNewMember();
                        }
                    }}
                >
                    <div className="orchestrator-team-card-info">
                        <span className="orchestrator-team-card-title">
                            {t("viewer.orchestratorTeams.addTermMemberSource.newMember", "新規メンバーを作成")}
                        </span>
                        <span className="orchestrator-team-card-meta">
                            {t("viewer.orchestratorTeams.addTermMemberSource.newMemberDesc", "新しいメンバー設定を作成して無所属チームに保存します")}
                        </span>
                    </div>
                </div>

                <div
                    className="orchestrator-team-card"
                    role="button"
                    tabIndex={0}
                    onClick={onPickMember}
                    onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            onPickMember();
                        }
                    }}
                >
                    <div className="orchestrator-team-card-info">
                        <span className="orchestrator-team-card-title">
                            {t("viewer.orchestratorTeams.addTermMemberSource.copyMember", "既存メンバーからコピー")}
                        </span>
                        <span className="orchestrator-team-card-meta">
                            {t("viewer.orchestratorTeams.addTermMemberSource.copyMemberDesc", "他のチームからメンバー設定をコピーして使用します")}
                        </span>
                    </div>
                </div>

                <div
                    className={`orchestrator-team-card${!checked || !hasUnaffiliatedMembers ? " disabled" : ""}`}
                    role="button"
                    tabIndex={hasUnaffiliatedMembers ? 0 : -1}
                    aria-disabled={!hasUnaffiliatedMembers}
                    onClick={() => {
                        if (hasUnaffiliatedMembers) onQuickStart();
                    }}
                    onKeyDown={(e) => {
                        if ((e.key === "Enter" || e.key === " ") && hasUnaffiliatedMembers) {
                            e.preventDefault();
                            onQuickStart();
                        }
                    }}
                >
                    <div className="orchestrator-team-card-info">
                        <span className="orchestrator-team-card-title">
                            {t("viewer.orchestratorTeams.addTermMemberSource.quickStart", "無所属メンバーで開始")}
                        </span>
                        <span className="orchestrator-team-card-meta">
                            {hasUnaffiliatedMembers
                                ? t("viewer.orchestratorTeams.addTermMemberSource.quickStartDesc", "無所属チームに保存済みのメンバー設定をそのまま使用します")
                                : t("viewer.orchestratorTeams.addTermMemberSource.quickStartDisabled", "無所属チームにメンバーがいません")}
                        </span>
                    </div>
                </div>
            </div>
        </div>
    );
}
