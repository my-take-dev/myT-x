import {useI18n} from "../../../../i18n";
import type {OrchestratorReadiness} from "./useTaskScheduler";

interface TaskSchedulerAlertProps {
    readiness: OrchestratorReadiness;
    onBack: () => void;
    onRegisterMember: () => void;
}

export function TaskSchedulerAlert({readiness, onBack, onRegisterMember}: TaskSchedulerAlertProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    return (
        <div className="task-scheduler-alert">
            <button type="button" className="task-scheduler-back-btn" onClick={onBack}>
                ← {tr("viewer.taskScheduler.backToList", "一覧に戻る", "Back to list")}
            </button>

            <div className="task-scheduler-alert-content">
                <div className="task-scheduler-alert-icon">⚠️</div>

                {!readiness.db_exists && (
                    <p className="task-scheduler-alert-message">
                        {tr(
                            "viewer.taskScheduler.alert.noDb",
                            "orchestrator.db が見つかりません。エージェントチームを起動するか、ペインにメンバーを登録してください。",
                            "orchestrator.db not found. Start an agent team or register members to panes.",
                        )}
                    </p>
                )}

                {readiness.db_exists && readiness.agent_count === 0 && readiness.has_panes && (
                    <p className="task-scheduler-alert-message">
                        {tr(
                            "viewer.taskScheduler.alert.noAgentsWithPanes",
                            "エージェントが登録されていません。ペインにメンバーを登録してください。",
                            "No agents registered. Register members to panes.",
                        )}
                    </p>
                )}

                {readiness.db_exists && readiness.agent_count === 0 && !readiness.has_panes && (
                    <p className="task-scheduler-alert-message">
                        {tr(
                            "viewer.taskScheduler.alert.noPanes",
                            "ペインが存在しません。セッションにペインを作成してからエージェントを登録してください。",
                            "No panes exist. Create panes in the session, then register agents.",
                        )}
                    </p>
                )}

                {readiness.db_exists && readiness.agent_count > 0 && !readiness.has_panes && (
                    <p className="task-scheduler-alert-message">
                        {tr(
                            "viewer.taskScheduler.alert.agentsButNoPanes",
                            "エージェントは登録済みですが、ペインが存在しません。セッションにペインを作成してください。",
                            "Agents are registered, but no panes exist. Create panes in the session.",
                        )}
                    </p>
                )}

                {readiness.db_exists && readiness.agent_count > 0 && readiness.has_panes && !readiness.ready && (
                    <p className="task-scheduler-alert-message">
                        {tr(
                            "viewer.taskScheduler.alert.unknown",
                            "準備状態を確認できません。セッションと orchestrator.db を再確認してください。",
                            "Unable to verify readiness. Recheck the session and orchestrator.db.",
                        )}
                    </p>
                )}

                {readiness.has_panes && (
                    <div className="task-scheduler-alert-actions">
                        <button
                            type="button"
                            className="task-scheduler-register-btn"
                            onClick={onRegisterMember}
                        >
                            {tr(
                                "viewer.taskScheduler.alert.registerMember",
                                "ペインへメンバーを登録する",
                                "Register member to pane",
                            )}
                        </button>
                    </div>
                )}
            </div>
        </div>
    );
}
