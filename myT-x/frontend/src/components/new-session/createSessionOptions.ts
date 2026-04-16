import type {NewSessionState} from "./types";

type NewWorktreeSessionOptionFields = Pick<
    NewSessionState,
    | "branchName"
    | "baseBranch"
    | "pullBefore"
    | "continueOnPullFailure"
    | "enableAgentTeam"
    | "useClaudeEnv"
    | "usePaneEnv"
    | "useSessionPaneScope"
>;

export function buildCreateSessionWithWorktreeOptions(state: NewWorktreeSessionOptionFields) {
    return {
        branch_name: state.branchName.trim(),
        base_branch: state.baseBranch,
        pull_before_create: state.pullBefore,
        continue_on_pull_failure: state.continueOnPullFailure,
        enable_agent_team: state.enableAgentTeam,
        use_claude_env: state.useClaudeEnv,
        use_pane_env: state.usePaneEnv,
        use_session_pane_scope: state.useSessionPaneScope,
    };
}
