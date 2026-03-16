import {useState} from "react";
import {useI18n} from "../../../../i18n";
import type {OrchestratorLaunchMode, OrchestratorTeamDefinition} from "./types";

interface StartDialogProps {
    team: OrchestratorTeamDefinition;
    activeSession: string | null;
    onBack: () => void;
    onStart: (launchMode: OrchestratorLaunchMode, newSessionName: string) => void;
}

export function StartDialog({team, activeSession, onBack, onStart}: StartDialogProps) {
    const {t} = useI18n();
    const [launchMode, setLaunchMode] = useState<OrchestratorLaunchMode>("active_session");
    const [newSessionName, setNewSessionName] = useState(team.name.toLowerCase().replace(/\s+/g, "-"));

    return (
        <div className="orchestrator-teams-start-panel">
            <button type="button" className="orchestrator-teams-back-btn" onClick={onBack}>
                &larr; {t("viewer.orchestratorTeams.start.back", "戻る")}
            </button>

            <div className="orchestrator-teams-start-hero">
                <div className="orchestrator-team-card-tag">
                    {t("viewer.orchestratorTeams.start.launch", "起動")}
                </div>
                <h3>{team.name}</h3>
                <p>
                    {t("viewer.orchestratorTeams.start.description", "現在のアクティブセッションのルート/ワークツリーを実行コンテキストとして起動します。アクティブウィンドウ内の既存ペインが再利用されるか、新しいセッションが作成されてタイル配置された後にチームがブートストラップされます。")}
                </p>
            </div>

            <div className="orchestrator-teams-start-grid">
                <div className={`orchestrator-teams-mode-card${launchMode === "active_session" ? " selected" : ""}`}>
                    <label>
                        <input
                            type="radio"
                            name="launch-mode"
                            checked={launchMode === "active_session"}
                            onChange={() => setLaunchMode("active_session")}
                        />
                        <span>{t("viewer.orchestratorTeams.start.reuseActive", "アクティブウィンドウを再利用")}</span>
                    </label>
                    <p>{t("viewer.orchestratorTeams.start.reuseActiveDescription", "現在のアクティブセッションを使用し、チームに追加容量が必要な場合のみペインを追加します。")}</p>
                </div>

                <div className={`orchestrator-teams-mode-card${launchMode === "new_session" ? " selected" : ""}`}>
                    <label>
                        <input
                            type="radio"
                            name="launch-mode"
                            checked={launchMode === "new_session"}
                            onChange={() => setLaunchMode("new_session")}
                        />
                        <span>{t("viewer.orchestratorTeams.start.createNew", "新しいセッションを作成")}</span>
                    </label>
                    <p>{t("viewer.orchestratorTeams.start.createNewDescription", "現在のアクティブセッションのルート/ワークツリーから専用セッションを作成し、ペインをタイル配置します。")}</p>
                    <p className="orchestrator-teams-mode-card-warning">{t("viewer.orchestratorTeams.start.createNewWarning", "※ 元のセッションとの同時開発はファイル更新等がバッティングする可能性があるのでご注意ください。")}</p>
                </div>
            </div>

            <div className="form-group">
                <label className="form-label">{t("viewer.orchestratorTeams.start.sourceSession", "ソースセッション")}</label>
                <input className="form-input" type="text" value={activeSession ?? ""} disabled/>
            </div>

            {launchMode === "new_session" && (
                <div className="form-group">
                    <label className="form-label">{t("viewer.orchestratorTeams.start.newSessionName", "新しいセッション名")}</label>
                    <input
                        className="form-input"
                        type="text"
                        value={newSessionName}
                        onChange={(event) => setNewSessionName(event.target.value)}
                        placeholder="release-swarm"
                    />
                </div>
            )}

            <div className="orchestrator-team-editor-footer">
                <button
                    type="button"
                    className="orchestrator-teams-primary-btn"
                    disabled={activeSession === null || (launchMode === "new_session" && newSessionName.trim() === "")}
                    onClick={() => onStart(launchMode, newSessionName)}
                >
                    {t("viewer.orchestratorTeams.start.launchTeam", "チームを起動")}
                </button>
            </div>
        </div>
    );
}
