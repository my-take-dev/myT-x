import {useEffect, useState} from "react";
import {useI18n} from "../../../../i18n";
import type {OrchestratorMemberDraft, OrchestratorMemberDraftSkill} from "./types";
import {isMemberDraftValid} from "./useOrchestratorTeams";

interface MemberEditorProps {
    initialDraft: OrchestratorMemberDraft;
    existingPaneTitles: string[];
    onBack: () => void;
    onSave: (draft: OrchestratorMemberDraft) => void;
}

const maxPaneTitle = 30;
const maxRole = 50;
const maxCommand = 100;

export function MemberEditor({initialDraft, existingPaneTitles, onBack, onSave}: MemberEditorProps) {
    const {t} = useI18n();
    const [draft, setDraft] = useState(initialDraft);
    const maxSkills = 20;

    const paneTitleDuplicate = draft.paneTitle.trim() !== "" &&
        existingPaneTitles.some((title) => title === draft.paneTitle.trim());

    function createDraftSkill(): OrchestratorMemberDraftSkill {
        return {
            id: globalThis.crypto?.randomUUID?.() ?? `skill-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
            name: "",
            description: "",
        };
    }

    function updateSkill(index: number, field: keyof Omit<OrchestratorMemberDraftSkill, "id">, value: string) {
        const skills = [...draft.skills];
        skills[index] = {...skills[index], [field]: value};
        setDraft({...draft, skills});
    }

    function removeSkill(index: number) {
        setDraft({...draft, skills: draft.skills.filter((_, i) => i !== index)});
    }

    function addSkill() {
        if (draft.skills.length >= maxSkills) return;
        setDraft({...draft, skills: [...draft.skills, createDraftSkill()]});
    }

    useEffect(() => {
        setDraft(initialDraft);
    }, [initialDraft]);

    return (
        <div className="orchestrator-member-editor">
            <button type="button" className="orchestrator-teams-back-btn" onClick={onBack}>
                &larr; {t("viewer.orchestratorTeams.member.back", "戻る")}
            </button>

            <div className="form-group">
                <label className="form-label">
                    {t("viewer.orchestratorTeams.member.paneTitle", "ペインタイトル")}
                    <span className="orchestrator-teams-count"> ({draft.paneTitle.length}/{maxPaneTitle})</span>
                </label>
                <input
                    className={`form-input${paneTitleDuplicate ? " orchestrator-teams-input-error" : ""}`}
                    type="text"
                    value={draft.paneTitle}
                    onChange={(event) => setDraft({...draft, paneTitle: event.target.value})}
                    placeholder="Lead"
                    maxLength={maxPaneTitle}
                />
                {paneTitleDuplicate && (
                    <span className="orchestrator-teams-field-error">
                        {t("viewer.orchestratorTeams.member.duplicatePaneTitle", "同名のペインタイトルが既に存在します")}
                    </span>
                )}
            </div>

            <div className="form-group">
                <label className="form-label">
                    {t("viewer.orchestratorTeams.member.role", "役割名")}
                    <span className="orchestrator-teams-count"> ({draft.role.length}/{maxRole})</span>
                </label>
                <input
                    className="form-input"
                    type="text"
                    value={draft.role}
                    onChange={(event) => setDraft({...draft, role: event.target.value})}
                    placeholder="リードエンジニア"
                    maxLength={maxRole}
                />
            </div>

            <div className="orchestrator-teams-grid">
                <div className="form-group">
                    <label className="form-label">
                        {t("viewer.orchestratorTeams.member.command", "起動コマンド")}
                        <span className="orchestrator-teams-count"> ({draft.command.length}/{maxCommand})</span>
                    </label>
                    <input
                        className="form-input"
                        type="text"
                        value={draft.command}
                        onChange={(event) => setDraft({...draft, command: event.target.value})}
                        placeholder="codex"
                        maxLength={maxCommand}
                    />
                </div>
                <div className="form-group">
                    <label className="form-label">{t("viewer.orchestratorTeams.member.args", "引数")}</label>
                    <textarea
                        className="form-input orchestrator-teams-textarea small"
                        value={draft.argsText}
                        onChange={(event) => setDraft({...draft, argsText: event.target.value})}
                        placeholder={"--sandbox\nworkspace-write"}
                    />
                </div>
            </div>

            <div className="form-group">
                <label className="form-label">{t("viewer.orchestratorTeams.member.customMessage", "役割プロンプト")}</label>
                <p className="orchestrator-teams-field-note">
                    {t("viewer.orchestratorTeams.member.rolePromptContextWarning", "AIが参照する情報として保存されます。長文や重複した表現はコンテキストを圧迫するため、簡潔な記述を心がけてください。")}
                </p>
                <textarea
                    className="form-input orchestrator-teams-textarea"
                    value={draft.customMessage}
                    onChange={(event) => setDraft({...draft, customMessage: event.target.value})}
                    placeholder="この役割の詳細な責務や行動指針を記述してください"
                />
            </div>

            <div className="form-group">
                <label className="form-label">
                    {t("viewer.orchestratorTeams.member.skills", "スキル")}
                    <span className="orchestrator-teams-count"> ({draft.skills.length}/{maxSkills})</span>
                </label>
                <p className="orchestrator-teams-field-note">
                    {t("viewer.orchestratorTeams.member.contextWarning", "AIが参照する情報として保存されます。長文や重複した表現はコンテキストを圧迫するため、簡潔な記述を心がけてください。")}
                </p>
                {draft.skills.map((skill, index) => (
                    <div key={skill.id} className="orchestrator-teams-skill-row">
                        <div className="orchestrator-teams-skill-fields">
                            <input
                                className="form-input"
                                type="text"
                                value={skill.name}
                                onChange={(e) => updateSkill(index, "name", e.target.value)}
                                placeholder={t("viewer.orchestratorTeams.member.skillName", "スキル名")}
                                maxLength={100}
                            />
                            <textarea
                                className="form-input orchestrator-teams-textarea small"
                                value={skill.description}
                                onChange={(e) => updateSkill(index, "description", e.target.value)}
                                placeholder={t("viewer.orchestratorTeams.member.skillDescription", "説明（最大400文字）")}
                                maxLength={400}
                            />
                        </div>
                        <button
                            type="button"
                            className="orchestrator-teams-remove-btn"
                            onClick={() => removeSkill(index)}
                            title={t("viewer.orchestratorTeams.member.removeSkill", "スキルを削除")}
                        >
                            &times;
                        </button>
                    </div>
                ))}
                <button
                    type="button"
                    className="orchestrator-teams-add-btn"
                    disabled={draft.skills.length >= maxSkills}
                    onClick={addSkill}
                >
                    + {t("viewer.orchestratorTeams.member.addSkill", "スキルを追加")}
                </button>
            </div>

            <div className="orchestrator-team-editor-footer">
                <button
                    type="button"
                    className="orchestrator-teams-primary-btn"
                    disabled={!isMemberDraftValid(draft) || paneTitleDuplicate}
                    onClick={() => onSave(draft)}
                >
                    {t("viewer.orchestratorTeams.member.save", "メンバーを保存")}
                </button>
            </div>
        </div>
    );
}
