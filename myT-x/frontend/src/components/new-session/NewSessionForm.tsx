import {useI18n} from "../../i18n";
import type {NewSessionDispatch, NewSessionState} from "./types";

interface NewSessionFormProps {
    s: NewSessionState;
    dispatch: NewSessionDispatch;
    canSubmit: boolean;
    onSubmit: () => void;
}

export function NewSessionForm({s, dispatch, canSubmit, onSubmit}: NewSessionFormProps) {
    const {language, t} = useI18n();
    const isEn = language === "en";

    return (
        <>
            {/* Session name */}
            <div className="form-group">
                <span className="form-label">
                    {isEn ? "Session Name" : t("newSession.sessionName.label", "セッション名")}
                </span>
                <input
                    className="form-input"
                    value={s.sessionName}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "sessionName", value: e.target.value})}
                    onKeyDown={(e) => {
                        if (e.key === "Enter" && canSubmit) void onSubmit();
                    }}
                    placeholder={
                        isEn
                            ? "Enter session name"
                            : t("newSession.sessionName.placeholder", "セッション名を入力")
                    }
                    autoFocus
                />
            </div>

            {/* Agent Team option */}
            <div className="form-checkbox-row">
                <input
                    type="checkbox"
                    id="enable-agent-team"
                    checked={s.enableAgentTeam}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "enableAgentTeam", value: e.target.checked})}
                    disabled={!s.shimAvailable}
                />
                <label htmlFor="enable-agent-team">
                    {isEn ? "Start as Agent Team" : t("newSession.agentTeam.enable", "Agent Team として開始")}
                    {!s.shimAvailable && (
                        <span className="form-hint">
                            {isEn
                                ? " (shim not installed)"
                                : t("newSession.agentTeam.shimMissing", " (シム未インストール)")}
                        </span>
                    )}
                </label>
            </div>

            {/* Claude Code env option */}
            <div className="form-checkbox-row">
                <input
                    type="checkbox"
                    id="use-claude-env"
                    checked={s.useClaudeEnv}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "useClaudeEnv", value: e.target.checked})}
                />
                <label htmlFor="use-claude-env">
                    {isEn
                        ? "Use Claude Code environment variables"
                        : t("newSession.env.claude", "Claude Code 環境変数を利用する")}
                </label>
            </div>

            {/* Pane env option */}
            <div className="form-checkbox-row">
                <input
                    type="checkbox"
                    id="use-pane-env"
                    checked={s.usePaneEnv}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "usePaneEnv", value: e.target.checked})}
                />
                <label htmlFor="use-pane-env">
                    {isEn
                        ? "Use additional pane-only environment variables"
                        : t("newSession.env.pane", "追加ペイン専用環境変数を利用する")}
                </label>
            </div>

            {/* Session pane scope option */}
            <div className="form-checkbox-row">
                <input
                    type="checkbox"
                    id="use-session-pane-scope"
                    checked={s.useSessionPaneScope}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "useSessionPaneScope", value: e.target.checked})}
                />
                <label htmlFor="use-session-pane-scope">
                    {isEn
                        ? "Use session-based pane management"
                        : t("newSession.env.sessionPaneScope", "セッション単位ペイン管理を利用する")}
                </label>
            </div>
        </>
    );
}
