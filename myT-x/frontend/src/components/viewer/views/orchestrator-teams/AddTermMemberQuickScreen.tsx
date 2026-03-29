import {useCallback, useEffect, useMemo, useState} from "react";
import {useI18n} from "../../../../i18n";
import type {
    OrchestratorMemberDraft,
    OrchestratorStorageLocation,
    OrchestratorTeamDefinition,
    PaneState,
} from "./types";
import {copyMemberToDraft, UNAFFILIATED_TEAM_ID} from "./orchestratorTeamUtils";
import {useBootstrapDelayInput} from "./useBootstrapDelayInput";

interface AddTermMemberQuickScreenProps {
    paneId: string;
    saving: boolean;
    ensureUnaffiliatedTeam: (storageLocation: OrchestratorStorageLocation) => Promise<OrchestratorTeamDefinition>;
    onBack: () => void;
    onBootstrap: (member: OrchestratorMemberDraft, paneState: PaneState, bootstrapDelayMs: number) => void;
}

export function AddTermMemberQuickScreen({
    paneId,
    saving,
    ensureUnaffiliatedTeam,
    onBack,
    onBootstrap,
}: AddTermMemberQuickScreenProps) {
    const {t} = useI18n();
    const [globalMembers, setGlobalMembers] = useState<OrchestratorTeamDefinition["members"]>([]);
    const [projectMembers, setProjectMembers] = useState<OrchestratorTeamDefinition["members"]>([]);
    const [loading, setLoading] = useState(true);
    const [selectedMemberId, setSelectedMemberId] = useState<string | null>(null);
    const [selectedLocation, setSelectedLocation] = useState<OrchestratorStorageLocation>("global");
    const [paneState, setPaneState] = useState<PaneState>("cli_not_running");
    const requiresBootstrapDelay = paneState === "cli_not_running";
    const bootstrapDelay = useBootstrapDelayInput({enabled: requiresBootstrapDelay});

    const loadMembers = useCallback(async () => {
        setLoading(true);
        try {
            const [globalTeam, projectTeam] = await Promise.allSettled([
                ensureUnaffiliatedTeam("global"),
                ensureUnaffiliatedTeam("project"),
            ]);
            if (globalTeam.status === "fulfilled") {
                setGlobalMembers(globalTeam.value.members ?? []);
            }
            if (projectTeam.status === "fulfilled") {
                setProjectMembers(projectTeam.value.members ?? []);
            }
        } finally {
            setLoading(false);
        }
    }, [ensureUnaffiliatedTeam]);

    useEffect(() => {
        void loadMembers();
    }, [loadMembers]);

    const allMembers = useMemo(() => [
        ...globalMembers.map((m) => ({...m, _location: "global" as const})),
        ...projectMembers.map((m) => ({...m, _location: "project" as const})),
    ], [globalMembers, projectMembers]);

    const selectedEntry = useMemo(
        () => allMembers.find((m) => m.id === selectedMemberId && m._location === selectedLocation),
        [allMembers, selectedMemberId, selectedLocation],
    );

    const handleSelect = useCallback((memberId: string, location: OrchestratorStorageLocation) => {
        setSelectedMemberId(memberId);
        setSelectedLocation(location);
    }, []);

    const handleBootstrap = useCallback(() => {
        if (!selectedEntry) return;
        const draft = copyMemberToDraft({...selectedEntry, team_id: UNAFFILIATED_TEAM_ID});
        onBootstrap(draft, paneState, bootstrapDelay.delayMs);
    }, [bootstrapDelay.delayMs, onBootstrap, paneState, selectedEntry]);

    const bootstrapDelayErrorText = bootstrapDelay.validationError === "min"
        ? t("viewer.orchestratorTeams.bootstrapDelay.minError", "1秒以上を入力してください")
        : bootstrapDelay.validationError === "max"
            ? t("viewer.orchestratorTeams.bootstrapDelay.maxError", "30秒以下を入力してください")
            : null;

    return (
        <div className="orchestrator-add-term-member-quick">
            <button type="button" className="orchestrator-teams-back-btn" onClick={onBack}>
                &larr; {t("viewer.orchestratorTeams.addTermMemberQuick.back", "戻る")}
            </button>

            <div className="orchestrator-add-term-member-body">
                <div className="orchestrator-teams-start-hero">
                    <div className="orchestrator-team-card-tag">
                        {t("viewer.orchestratorTeams.addTermMemberQuick.target", "対象ペイン")}
                    </div>
                    <h3>{paneId}</h3>
                    <p>{t("viewer.orchestratorTeams.addTermMemberQuick.description", "無所属チームの既存メンバーを選択して即時開始します。")}</p>
                </div>

                {loading ? (
                    <div className="orchestrator-teams-empty">
                        {t("viewer.orchestratorTeams.addTermMemberQuick.loading", "メンバーを読み込み中...")}
                    </div>
                ) : allMembers.length === 0 ? (
                    <div className="orchestrator-teams-empty">
                        {t("viewer.orchestratorTeams.addTermMemberQuick.empty", "無所属メンバーがいません。")}
                    </div>
                ) : (
                    <div className="orchestrator-teams-cards">
                        {allMembers.map((member) => (
                            <div
                                key={`${member._location}-${member.id}`}
                                className={`orchestrator-team-card${
                                    selectedMemberId === member.id && selectedLocation === member._location ? " selected" : ""
                                }`}
                                role="button"
                                tabIndex={0}
                                onClick={() => handleSelect(member.id, member._location)}
                                onKeyDown={(e) => {
                                    if (e.key === "Enter" || e.key === " ") {
                                        e.preventDefault();
                                        handleSelect(member.id, member._location);
                                    }
                                }}
                            >
                                <div className="orchestrator-team-card-info">
                                    <span className="orchestrator-team-card-title">{member.pane_title}</span>
                                    <span className="orchestrator-team-card-meta">{member.role}</span>
                                </div>
                                <div className="orchestrator-team-card-actions">
                                    <span className="orchestrator-team-card-tag">{member.command}</span>
                                    <span className={`orchestrator-team-card-tag ${member._location}`}>
                                        {member._location === "project"
                                            ? t("viewer.orchestratorTeams.list.storageProject", "プロジェクト")
                                            : t("viewer.orchestratorTeams.list.storageGlobal", "グローバル")}
                                    </span>
                                </div>
                            </div>
                        ))}
                    </div>
                )}

                {selectedEntry && (
                    <>
                        <div className="form-group">
                            <label className="form-label">
                                {t("viewer.orchestratorTeams.addTermMember.paneState", "ペイン状態")}
                            </label>
                            <div className="orchestrator-teams-start-grid">
                                <div className={`orchestrator-teams-mode-card${paneState === "cli_running" ? " selected" : ""}`}>
                                    <label>
                                        <input
                                            type="radio"
                                            name="quick-pane-state"
                                            checked={paneState === "cli_running"}
                                            onChange={() => setPaneState("cli_running")}
                                        />
                                        <span>{t("viewer.orchestratorTeams.addTermMember.cliRunning", "CLIが起動済み")}</span>
                                    </label>
                                </div>
                                <div className={`orchestrator-teams-mode-card${paneState === "cli_not_running" ? " selected" : ""}`}>
                                    <label>
                                        <input
                                            type="radio"
                                            name="quick-pane-state"
                                            checked={paneState === "cli_not_running"}
                                            onChange={() => setPaneState("cli_not_running")}
                                        />
                                        <span>{t("viewer.orchestratorTeams.addTermMember.cliNotRunning", "CLIが未起動")}</span>
                                    </label>
                                </div>
                            </div>
                        </div>

                        {paneState === "cli_not_running" && (
                            <div className="form-group">
                                <label className="form-label">
                                    {t("viewer.orchestratorTeams.addTermMember.bootstrapDelay", "役割挿入待機時間（秒）")}
                                </label>
                                <input
                                    className="form-input"
                                    type="text"
                                    inputMode="numeric"
                                    value={bootstrapDelay.delaySec}
                                    onChange={bootstrapDelay.handleChange}
                                />
                                {bootstrapDelayErrorText && (
                                    <span className="orchestrator-team-editor-member-warning">
                                        {bootstrapDelayErrorText}
                                    </span>
                                )}
                            </div>
                        )}
                    </>
                )}
            </div>

            <div className="orchestrator-team-editor-footer">
                <button
                    type="button"
                    className="orchestrator-teams-primary-btn"
                    disabled={saving || !selectedEntry || !bootstrapDelay.isValid}
                    onClick={handleBootstrap}
                >
                    {saving
                        ? t("viewer.orchestratorTeams.addTermMemberQuick.starting", "開始中...")
                        : t("viewer.orchestratorTeams.addTermMemberQuick.start", "開始")}
                </button>
            </div>
        </div>
    );
}
