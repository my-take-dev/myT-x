/**
 * S-23: api.ts エラーハンドリング方針
 *
 * このモジュールは Wails バインディングを薄くラップするだけの責務を持つ。
 * エラーハンドリングは呼び出し元（コンポーネント / フック）で行う。
 *
 * 統一パターン:
 *   void api.SomeMethod(arg).catch((err) => {
 *     console.warn("[context] operation failed", err);
 *   });
 *
 * - 結果を使う場合は `await api.SomeMethod(arg)` で try/catch を書く。
 * - fire-and-forget の場合は `void api.SomeMethod(arg).catch(...)` を使う。
 * - console.warn の prefix は "[コンテキスト名]" で統一する。
 * - このファイル内では Promise を握り潰さない。Promise はそのまま返却する。
 */
import {
    ApplyLayoutPreset,
    BuildStatusLine,
    CheckDirectoryConflict,
    CheckWorktreePathConflict,
    CheckWorktreeStatus,
    CleanupWorktree,
    CommitAndPushWorktree,
    CreateSession,
    CreateSessionWithExistingWorktree,
    CreateSessionWithWorktree,
    DetachSession,
    DevPanelCommitDiff,
    DevPanelGitLog,
    DevPanelGitStatus,
    DevPanelListBranches,
    DevPanelListDir,
    DevPanelReadFile,
    DevPanelWorkingDiff,
    FocusPane,
    GetActiveSession,
    GetAllowedShells,
    GetClaudeEnvVarDescriptions,
    GetCurrentBranch,
    GetPaneEnv,
    GetPaneReplay,
    GetConfig,
    GetConfigAndFlushWarnings,
    GetInputHistory,
    GetMCPDetail as GetMCPDetailRaw,
    GetInputHistoryFilePath,
    GetSessionErrorLog,
    GetSessionLogFilePath,
    GetValidationRules as GetValidationRulesWails,
    LogFrontendEvent,
    GetSessionEnv,
    GetWebSocketURL,
    IsAgentTeamsAvailable,
    IsGitRepository,
    InstallTmuxShim,
    KillPane,
    KillSession,
    ListBranches,
    ListMCPServers as ListMCPServersRaw,
    ListSessions,
    ListWorktreesByRepo,
    PickSessionDirectory,
    QuickStartSession,
    PromoteWorktreeToBranch,
    RenamePane,
    RenameSession,
    ResizePane,
    SaveConfig,
    SendInput,
    SendSyncInput,
    SetActiveSession,
    SplitPane,
    SwapPanes,
    ToggleMCPServer,
} from "../wailsjs/go/main/App";
import type {MCPSnapshot} from "./types/mcp";
import {normalizeMCPSnapshot, normalizeMCPSnapshots} from "./types/mcp";
import type {ValidationRules} from "./types/tmux";

async function ListMCPServers(sessionName: string): Promise<MCPSnapshot[]> {
    const result = await ListMCPServersRaw(sessionName);
    return normalizeMCPSnapshots(result);
}

async function GetMCPDetail(sessionName: string, mcpID: string): Promise<MCPSnapshot> {
    // Used for targeted refresh paths (per-MCP updates on backend events).
    const result = await GetMCPDetailRaw(sessionName, mcpID);
    const normalized = normalizeMCPSnapshot(result);
    if (!normalized) {
        throw new Error("GetMCPDetail returned an invalid MCP payload");
    }
    return normalized;
}

export const api = {
    ApplyLayoutPreset,
    GetAllowedShells,
    GetActiveSession,
    GetClaudeEnvVarDescriptions,
    GetConfig,
    GetConfigAndFlushWarnings,
    GetMCPDetail,
    // GetValidationRules のみ型キャストが必要（Wails 自動生成型が ValidationRules と異なるため）。
    GetValidationRules: () => GetValidationRulesWails() as Promise<ValidationRules>,
    GetSessionEnv,
    IsAgentTeamsAvailable,
    ListMCPServers,
    ListSessions,
    PickSessionDirectory,
    QuickStartSession,
    CreateSession,
    CreateSessionWithWorktree,
    CreateSessionWithExistingWorktree,
    CheckDirectoryConflict,
    CheckWorktreePathConflict,
    CheckWorktreeStatus,
    CommitAndPushWorktree,
    GetCurrentBranch,
    SetActiveSession,
    SplitPane,
    SendInput,
    SendSyncInput,
    ResizePane,
    FocusPane,
    GetPaneEnv,
    GetPaneReplay,
    KillPane,
    KillSession,
    DetachSession,
    RenamePane,
    RenameSession,
    SaveConfig,
    SwapPanes,
    BuildStatusLine,
    IsGitRepository,
    InstallTmuxShim,
    ListBranches,
    ListWorktreesByRepo,
    PromoteWorktreeToBranch,
    CleanupWorktree,
    GetWebSocketURL,
    DevPanelListDir,
    DevPanelReadFile,
    DevPanelGitLog,
    DevPanelGitStatus,
    DevPanelCommitDiff,
    DevPanelWorkingDiff,
    DevPanelListBranches,
    GetInputHistory,
    GetInputHistoryFilePath,
    GetSessionErrorLog,
    GetSessionLogFilePath,
    LogFrontendEvent,
    ToggleMCPServer,
};
