# Cross-cutting Concerns Review

**Reviewer**: Architecture (cross-module focus)  
**Date**: 2026-02-18  
**Scope**: `git diff HEAD -- myT-x/` (64 files, ~11,658 lines)  
**Project rules**: CLAUDE.md / AGENTS.md applied

---

## Summary of Changes

This changeset contains several major themes:

1. **Event system refactor**: `snapshotEventPolicies` table replaces dual switch-case in `shouldEmitSnapshotForEvent` / `shouldBypassSnapshotDebounceForEvent`
2. **Pane output event extraction**: Monolithic `emitBackendEvent` handler broken into `handlePaneOutputEvent` / `handleLegacyMapPaneOutput`
3. **TOCTOU fix**: `ApplyLayoutPreset` now uses `ApplyLayoutPresetToActiveWindow` which atomically resolves activeWindowID inside the lock
4. **Shim spec compliance**: `applyModelTransform` now swallows config loader errors per shim spec ("never block on transform failure")
5. **Model transform `ALL` wildcard**: New `from: ALL` config replaces all `--model` values regardless of source model
6. **Legacy shim cleanup**: `CleanupLegacyShimInstalls` removes stale PATH entries and directories on startup
7. **Frontend component extraction**: TerminalPane decomposed into `useTerminalSetup`, `useTerminalEvents`, `useTerminalResize`, `useTerminalFontSize` hooks; window rename extracted to `useWindowRename`
8. **Shim parser improvements**: resize-pane direction flags, `set-environment` empty string VALUE, `--` terminator, `list-sessions`, redundant `name` field removed from `commandSpec`
9. **Formatting / documentation**: CSS re-indentation, extensive doc comments, `tmux.ts` drift risk warnings

---

## Critical Issues (must fix)

**(None found.)**

The changeset is architecturally sound. All cross-cutting interactions examined are consistent.

---

## Important Issues (should fix)

### I-1 [Backend-Frontend Contract] `SessionSnapshotDelta.removed` should document it contains session names

**Backend** (`app_snapshot_delta.go`): `delta.Removed` is `[]string` — containing session **names**.  
**Frontend** (`useBackendSync.ts`): `const removed = asArray<string>(delta.removed)` — correctly typed.  
**Frontend** (`tmuxStore.ts` `applySessionDelta`): Filters sessions by `name` — **correct**.

However, `tmux.ts` type definition:
```ts
export interface SessionSnapshotDelta {
    upserts: SessionSnapshot[];
    removed: string[];
}
```

The `removed` field has no doc comment specifying it contains **session names** (not IDs). Future frontend developers could assume these are IDs. Adding `/** Session names of removed sessions */` would prevent misinterpretation.

**Impact**: Low (currently correct), but drift-prone.

### I-2 [Shim Error Handling] `applyModelTransformSafe` wrapper now has unreachable error branch

The diff changes `applyModelTransform` to return `(false, nil)` instead of `(false, err)` on config load failure. In the production shim code `applyModelTransformSafe` wraps `applyModelTransform`. The wrapper's `if err != nil` branch is now unreachable dead code since the inner function never returns a non-nil error.

**Files**: `cmd/tmux-shim/model_transform.go` — both `applyModelTransform` and `applyModelTransformSafe`

**Recommendation**: Clean up `applyModelTransformSafe` to remove the unreachable error branch, or add a comment explaining it exists as defense-in-depth.

### I-3 [Lock Ordering Documentation] `snapshotMu` acquired independently by `shouldSyncPaneStates`

`emitSnapshot()` call sequence:
1. `sessions.Snapshot()` — acquires `SessionManager.mu`
2. `a.shouldSyncPaneStates(...)` — acquires `snapshotMu` **alone**
3. `a.snapshotDelta(...)` — acquires `snapshotDeltaMu` → then `snapshotMu`

Step 2 acquires `snapshotMu` alone and releases it. Step 3 acquires `snapshotDeltaMu` then `snapshotMu`. No deadlock since step 2 always releases `snapshotMu` before step 3 starts. However, the lock ordering comment in `app.go`:
```go
// Lock ordering (outer -> inner):
//   snapshotDeltaMu -> snapshotMu
```
should note that `snapshotMu` is also acquired **independently** (without `snapshotDeltaMu`) by `shouldSyncPaneStates`. This prevents future developers from incorrectly assuming that `snapshotMu` is always acquired inside `snapshotDeltaMu`.

**Impact**: No bug. Documentation improvement to prevent future lock-ordering violations.

---

## Suggestions (nice to have)

### S-1 [Test Stability] Hardcoded `2*time.Second` test timeouts

Three snapshot test functions changed `snapshotCoalesceWindow+300*time.Millisecond` to `2*time.Second`. While safe, consider deriving from `snapshotCoalesceWindow` (e.g., `3*snapshotCoalesceWindow`) to maintain coupling. If `snapshotCoalesceWindow` is increased later, the tests would still pass but stop validating actual timing.

### S-2 [Frontend] `safeAddonOp` in SearchBar catches all errors

The wrapper catches all errors for the disposed-addon scenario. This also swallows genuine bugs (API changes in xterm SearchAddon). Consider narrowing the catch if xterm provides specific error types in the future.

### S-3 [Event Consistency] Verify pane-kill snapshot trigger path

`snapshotEventPolicies` has `tmux:pane-created` but no `tmux:pane-closed` / `tmux:pane-killed`. Verified: `KillPane` in `app_pane_api.go` emits `tmux:layout-changed` which **is** in the policies map. This is correct. However, adding a comment in the policies map noting that pane-kill triggers snapshot via `layout-changed` would aid discoverability.

### S-4 [Resource Lifecycle] `CleanupLegacyShimInstalls` runs on every startup

After first cleanup, subsequent calls are effectively no-ops but still read/write registry. Consider a sentinel mechanism to skip redundant cleanups.

### S-5 [Frontend Architecture] Hook declaration order in TerminalPane

TerminalPane's 4 extracted hooks must maintain declaration order so that React's useEffect cleanup runs correctly (events detached before terminal disposed). Currently correct (`useTerminalSetup` → `useTerminalEvents` → `useTerminalResize` → `useTerminalFontSize`), but a code comment noting this ordering constraint would prevent accidental reordering.

---

## Positive Observations

1. **`snapshotEventPolicies` table-driven approach**: Consolidates dual switch-cases into a single policy map. Adding new events now requires one entry instead of coordinating two switches — eliminates a class of consistency bugs.

2. **TOCTOU fix in `ApplyLayoutPresetToActiveWindow`**: Moving active-window resolution inside the `SessionManager.mu` lock eliminates a real race condition. The fallback logic with `activeWindowInSessionLocked` is well-designed.

3. **`requireSessionsWithPaneID` DRY refactor**: Six pane-targeting API methods shared identical boilerplate (TrimSpace + empty check + requireSessions). The new helper consolidates correctly via pointer parameter. Notes in `SplitPane` explain why it doesn't use the helper (delegates to CommandRouter).

4. **Shim spec alignment**: `applyModelTransform` error-swallowing aligns with CLAUDE.md's "tmux-shim: on failure, forward original text" spec. Debug logging ensures observability via `debugLog`.

5. **`useWindowRename` double-invocation guard**: The `inflightWindowIdRef` pattern correctly handles Enter → onBlur race. `Promise.resolve().then()` for microtask-deferred reset is clean.

6. **Snapshot delta field-count guards**: "IMPORTANT: update this function when fields are added/removed" comments with references to `TestSnapshotFieldCounts` create a durable safety net against struct drift.

7. **Lock ordering documentation in `app.go`**: Clear separation of `snapshotDeltaMu -> snapshotMu` ordering from independent locks. This is exactly what prevents deadlocks in concurrent systems.

8. **`ALL` wildcard with override priority**: Override → ALL → from-to → passthrough. The `matchAll` boolean avoids regex overhead for the wildcard case.

9. **Type drift risk warning in `tmux.ts`**: Explicit "Rule: whenever a Go struct is modified, BOTH this file AND models.ts must be updated" comment is practical given Wails code generation model.

10. **`KillSession` redundant `var` removal**: `var worktreeInfo *tmux.SessionWorktreeInfo` was correctly identified as redundant given the subsequent `:=` declaration.
