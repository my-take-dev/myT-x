import {useConfigSync} from "./sync/useConfigSync";
import {useInputHistorySync} from "./sync/useInputHistorySync";
import {useMCPSync} from "./sync/useMCPSync";
import {useSessionLogSync} from "./sync/useSessionLogSync";
import {useSnapshotSync} from "./sync/useSnapshotSync";

/**
 * Orchestrates all backend event subscriptions and initial data loading.
 *
 * Each domain-specific hook manages its own lifecycle:
 * - useSnapshotSync: Session snapshots, pane data stream, worktree & worker events
 * - useConfigSync: Configuration loading and real-time updates
 * - useSessionLogSync: Session error log (ping + fetch pattern)
 * - useInputHistorySync: Input history (ping + fetch pattern)
 * - useMCPSync: MCP server state changes
 */
export function useBackendSync(): void {
    useSnapshotSync();
    useConfigSync();
    useSessionLogSync();
    useInputHistorySync();
    useMCPSync();
}
