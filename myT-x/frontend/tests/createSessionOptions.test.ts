import {describe, expect, it} from "vitest";
import {buildCreateSessionWithWorktreeOptions} from "../src/components/new-session/createSessionOptions";
import {INITIAL_STATE} from "../src/components/new-session/newSessionReducer";

describe("buildCreateSessionWithWorktreeOptions", () => {
    it("includes continue_on_pull_failure in the submit payload", () => {
        const payload = buildCreateSessionWithWorktreeOptions({
            ...INITIAL_STATE,
            branchName: " feature/test ",
            baseBranch: "main",
            pullBefore: true,
            continueOnPullFailure: true,
            enableAgentTeam: true,
            useClaudeEnv: true,
            usePaneEnv: false,
            useSessionPaneScope: true,
        });

        expect(payload).toEqual({
            branch_name: "feature/test",
            base_branch: "main",
            pull_before_create: true,
            continue_on_pull_failure: true,
            enable_agent_team: true,
            use_claude_env: true,
            use_pane_env: false,
            use_session_pane_scope: true,
        });
    });
});
