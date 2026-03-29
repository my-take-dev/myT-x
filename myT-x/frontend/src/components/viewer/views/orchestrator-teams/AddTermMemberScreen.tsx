import {useState} from "react";
import {useI18n} from "../../../../i18n";
import {MemberEditor} from "./MemberEditor";
import {isMemberDraftValid} from "./orchestratorTeamUtils";
import type {OrchestratorMemberDraft, OrchestratorStorageLocation, PaneState} from "./types";
import {useBootstrapDelayInput} from "./useBootstrapDelayInput";

interface AddTermMemberScreenProps {
    paneId: string;
    draft: OrchestratorMemberDraft;
    saving: boolean;
    onChange: (draft: OrchestratorMemberDraft) => void;
    onBack: () => void;
    onBootstrap: (paneState: PaneState, storageLocation: OrchestratorStorageLocation, bootstrapDelayMs: number) => void;
}

export function AddTermMemberScreen({paneId, draft, saving, onChange, onBack, onBootstrap}: AddTermMemberScreenProps) {
    const {t} = useI18n();
    const [paneState, setPaneState] = useState<PaneState>("cli_not_running");
    const [storageLocation, setStorageLocation] = useState<OrchestratorStorageLocation>("global");
    const requiresBootstrapDelay = paneState === "cli_not_running";
    const bootstrapDelay = useBootstrapDelayInput({enabled: requiresBootstrapDelay});
    const bootstrapDelayErrorText = bootstrapDelay.validationError === "min"
        ? t("viewer.orchestratorTeams.bootstrapDelay.minError", "1秒以上を入力してください")
        : bootstrapDelay.validationError === "max"
            ? t("viewer.orchestratorTeams.bootstrapDelay.maxError", "30秒以下を入力してください")
            : null;

    return (
        <div className="orchestrator-add-term-member">
            <button type="button" className="orchestrator-teams-back-btn" onClick={onBack}>
                &larr; {t("viewer.orchestratorTeams.addTermMember.back", "戻る")}
            </button>

            <div className="orchestrator-add-term-member-body">
                <div className="orchestrator-teams-start-hero">
                    <div className="orchestrator-team-card-tag">
                        {t("viewer.orchestratorTeams.addTermMember.target", "対象ペイン")}
                    </div>
                    <h3>{paneId}</h3>
                </div>

                <MemberEditor
                    draft={draft}
                    existingPaneTitles={[]}
                    onChange={onChange}
                    onBack={onBack}
                    onSave={() => {}}
                    hideNavigation
                />

                <div className="form-group">
                    <label className="form-label">
                        {t("viewer.orchestratorTeams.addTermMember.storageLocation", "保存先")}
                    </label>
                    <div className="orchestrator-teams-start-grid">
                        <div className={`orchestrator-teams-mode-card${storageLocation === "global" ? " selected" : ""}`}>
                            <label>
                                <input
                                    type="radio"
                                    name="storage-location"
                                    checked={storageLocation === "global"}
                                    onChange={() => setStorageLocation("global")}
                                />
                                <span>{t("viewer.orchestratorTeams.addTermMember.storageGlobal", "グローバル")}</span>
                            </label>
                        </div>
                        <div className={`orchestrator-teams-mode-card${storageLocation === "project" ? " selected" : ""}`}>
                            <label>
                                <input
                                    type="radio"
                                    name="storage-location"
                                    checked={storageLocation === "project"}
                                    onChange={() => setStorageLocation("project")}
                                />
                                <span>{t("viewer.orchestratorTeams.addTermMember.storageProject", "プロジェクト")}</span>
                            </label>
                        </div>
                    </div>
                </div>

                <div className="form-group">
                    <label className="form-label">
                        {t("viewer.orchestratorTeams.addTermMember.paneState", "ペイン状態")}
                    </label>
                    <div className="orchestrator-teams-start-grid">
                        <div className={`orchestrator-teams-mode-card${paneState === "cli_running" ? " selected" : ""}`}>
                            <label>
                                <input
                                    type="radio"
                                    name="pane-state"
                                    checked={paneState === "cli_running"}
                                    onChange={() => setPaneState("cli_running")}
                                />
                                <span>{t("viewer.orchestratorTeams.addTermMember.cliRunning", "CLIが起動済み")}</span>
                            </label>
                            <p>{t("viewer.orchestratorTeams.addTermMember.cliRunningDesc", "ブートストラップメッセージのみ送信します")}</p>
                        </div>
                        <div className={`orchestrator-teams-mode-card${paneState === "cli_not_running" ? " selected" : ""}`}>
                            <label>
                                <input
                                    type="radio"
                                    name="pane-state"
                                    checked={paneState === "cli_not_running"}
                                    onChange={() => setPaneState("cli_not_running")}
                                />
                                <span>{t("viewer.orchestratorTeams.addTermMember.cliNotRunning", "CLIが未起動")}</span>
                            </label>
                            <p>{t("viewer.orchestratorTeams.addTermMember.cliNotRunningDesc", "cd → コマンド起動 → ブートストラップメッセージの順で実行します")}</p>
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
            </div>

            <div className="orchestrator-team-editor-footer">
                <button
                    type="button"
                    className="orchestrator-teams-primary-btn"
                    disabled={saving || !isMemberDraftValid(draft) || !bootstrapDelay.isValid}
                    onClick={() => onBootstrap(paneState, storageLocation, bootstrapDelay.delayMs)}
                >
                    {saving
                        ? t("viewer.orchestratorTeams.addTermMember.bootstrapping", "起動中...")
                        : t("viewer.orchestratorTeams.addTermMember.bootstrap", "メンバーを追加して起動")}
                </button>
            </div>
        </div>
    );
}
