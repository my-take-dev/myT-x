import {useI18n} from "../../../../i18n";
import type {OrchestratorStorageLocation, OrchestratorTeamDraft} from "./types";
import {isMemberDraftValid, isTeamDraftValid} from "./useOrchestratorTeams";

interface TeamEditorProps {
    draft: OrchestratorTeamDraft;
    saving: boolean;
    teamNameDuplicate: boolean;
    activeSession: string | null;
    onChange: (draft: OrchestratorTeamDraft) => void;
    onBack: () => void;
    onSave: () => void;
    onAddMember: () => void;
    onCopyMember: () => void;
    onEditMember: (index: number) => void;
    onDeleteMember: (memberID: string) => void;
}

export function TeamEditor({
    draft,
    saving,
    teamNameDuplicate,
    activeSession,
    onChange,
    onBack,
    onSave,
    onAddMember,
    onCopyMember,
    onEditMember,
    onDeleteMember,
}: TeamEditorProps) {
    const {t} = useI18n();

    return (
        <div className="orchestrator-team-editor">
            <button type="button" className="orchestrator-teams-back-btn" onClick={onBack}>
                &larr; {t("viewer.orchestratorTeams.editor.back", "戻る")}
            </button>

            <div className="form-group">
                <label className="form-label">{t("viewer.orchestratorTeams.editor.teamName", "チーム名")}</label>
                <input
                    className="form-input"
                    type="text"
                    value={draft.name}
                    onChange={(event) => onChange({...draft, name: event.target.value})}
                    placeholder="Release swarm"
                />
                {teamNameDuplicate && (
                    <span className="orchestrator-team-editor-member-warning">
                        {t("viewer.orchestratorTeams.editor.duplicateTeamName", "同じ名前のチームが既に存在します")}
                    </span>
                )}
            </div>

            <div className="form-group">
                <label className="form-label">
                    {t("viewer.orchestratorTeams.editor.storageLocation", "保存先")}
                </label>
                <div className="orchestrator-team-storage-selector">
                    <label className="orchestrator-team-storage-option">
                        <input
                            type="radio"
                            name="storageLocation"
                            value="global"
                            checked={draft.storageLocation === "global"}
                            onChange={() => onChange({...draft, storageLocation: "global"})}
                        />
                        {t("viewer.orchestratorTeams.editor.storageGlobal", "グローバル設定")}
                    </label>
                    <label className={`orchestrator-team-storage-option${activeSession === null ? " disabled" : ""}`}>
                        <input
                            type="radio"
                            name="storageLocation"
                            value="project"
                            checked={draft.storageLocation === "project"}
                            disabled={activeSession === null}
                            onChange={() => onChange({...draft, storageLocation: "project"})}
                        />
                        {t("viewer.orchestratorTeams.editor.storageProject", "プロジェクト (.myT-x)")}
                    </label>
                </div>
            </div>

            <div className="form-group">
                <label className="form-label">
                    {t("viewer.orchestratorTeams.editor.description", "チーム説明")}
                </label>
                <p className="orchestrator-teams-field-note">
                    {t("viewer.orchestratorTeams.editor.descriptionHint", "一覧画面では先頭約50文字が表示されます。要点を先頭に書くと識別しやすくなります。（最大400文字）")}
                </p>
                <textarea
                    className="form-input"
                    rows={3}
                    maxLength={400}
                    value={draft.description}
                    onChange={(event) => onChange({...draft, description: event.target.value})}
                    placeholder={t("viewer.orchestratorTeams.editor.descriptionPlaceholder", "このチームの目的や用途を記入")}
                />
                <span className="orchestrator-team-card-meta" style={{textAlign: "right"}}>
                    {draft.description.length} / 400
                </span>
            </div>

            <div className="form-group">
                <label className="form-label">
                    {t("viewer.orchestratorTeams.editor.bootstrapDelay", "役割挿入待機時間（秒）")}
                </label>
                <p className="orchestrator-teams-field-note">
                    {t("viewer.orchestratorTeams.editor.bootstrapDelayDescription", "コマンド起動後、役割メッセージ送信までの待機時間です。エージェントの起動に時間がかかる場合は増やしてください。")}
                </p>
                <input
                    className="form-input"
                    type="number"
                    min={1}
                    max={30}
                    step={0.5}
                    value={draft.bootstrapDelayMs / 1000}
                    onChange={(event) => {
                        const sec = Number.parseFloat(event.target.value);
                        if (!Number.isNaN(sec)) {
                            onChange({...draft, bootstrapDelayMs: Math.round(sec * 1000)});
                        }
                    }}
                />
            </div>

            <div className="orchestrator-team-editor-header">
                <div>
                    <div className="orchestrator-team-editor-title">
                        {t("viewer.orchestratorTeams.editor.members", "メンバー")}
                    </div>
                    <div className="orchestrator-team-editor-subtitle">
                        {t("viewer.orchestratorTeams.editor.membersDescription", "各メンバーにはペインタイトル、役割、起動コマンド、引数、カスタムブートストラップメッセージを定義します。")}
                    </div>
                </div>
                <div style={{display: "flex", gap: "6px"}}>
                    <button type="button" className="orchestrator-teams-secondary-btn" onClick={onAddMember}>
                        {t("viewer.orchestratorTeams.editor.addMember", "+ メンバー")}
                    </button>
                    <button type="button" className="orchestrator-teams-secondary-btn" onClick={onCopyMember}>
                        {t("viewer.orchestratorTeams.editor.copyMember", "他チームからコピー")}
                    </button>
                </div>
            </div>

            {draft.members.length === 0 ? (
                <div className="orchestrator-team-member-empty-panel">
                    {t("viewer.orchestratorTeams.editor.noMembers", "メンバーがまだ設定されていません。")}
                </div>
            ) : (
                <div className="orchestrator-team-editor-members">
                    {draft.members.map((member, index) => (
                        <div key={member.id} className="orchestrator-team-editor-member">
                            <div className="orchestrator-team-editor-member-main">
                                <div className="orchestrator-team-editor-member-title-row">
                                    <span className="orchestrator-team-editor-member-title">
                                        {member.paneTitle.trim() || t("viewer.orchestratorTeams.editor.untitledMember", "無名メンバー")}
                                    </span>
                                    {!isMemberDraftValid(member) && (
                                        <span className="orchestrator-team-editor-member-warning">
                                            {t("viewer.orchestratorTeams.editor.missingFields", "必須項目が未入力です")}
                                        </span>
                                    )}
                                </div>
                                <div className="orchestrator-team-editor-member-meta">
                                    <span>{member.role.trim() || t("viewer.orchestratorTeams.editor.roleNotSet", "役割名未設定")}</span>
                                    <span>{member.command.trim() || t("viewer.orchestratorTeams.editor.commandNotSet", "コマンド未設定")}</span>
                                    <span>
                                        {member.argsText.trim() === ""
                                            ? t("viewer.orchestratorTeams.editor.noArgs", "引数なし")
                                            : t("viewer.orchestratorTeams.editor.argCount", "{count} 個の引数", {count: member.argsText.split(/\r?\n/).length})}
                                    </span>
                                </div>
                            </div>
                            <div className="orchestrator-team-editor-member-actions">
                                <button type="button" className="orchestrator-team-card-btn" onClick={() => onEditMember(index)}>
                                    {t("viewer.orchestratorTeams.editor.edit", "編集")}
                                </button>
                                <button
                                    type="button"
                                    className="orchestrator-team-card-btn danger"
                                    onClick={() => onDeleteMember(member.id)}
                                >
                                    {t("viewer.orchestratorTeams.editor.remove", "削除")}
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            <div className="orchestrator-team-editor-footer">
                <button
                    type="button"
                    className="orchestrator-teams-primary-btn"
                    disabled={!isTeamDraftValid(draft) || teamNameDuplicate || saving}
                    onClick={onSave}
                >
                    {saving
                        ? t("viewer.orchestratorTeams.editor.saving", "保存中...")
                        : t("viewer.orchestratorTeams.editor.saveTeam", "チームを保存")}
                </button>
            </div>
        </div>
    );
}
