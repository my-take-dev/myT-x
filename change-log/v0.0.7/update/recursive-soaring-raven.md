# Review Remediation Plan: review-202602220942.md

## Context

Code review `review-202602220942.md` identified 34 issues (6 Critical, 14 Important, 14 Suggestion) across Go backend and TypeScript frontend. This plan addresses all items with parallel execution via 5 independent agent groups. Defensive coding checklist (`SKILL.md`) governs all changes.

---

## Parallel Execution Groups

### G1: Go - DevPanel API (`golang-expert` agent)

**Files:**
- `myT-x/app_devpanel_api.go`
- `myT-x/app_devpanel_api_test.go`
- `myT-x/app_devpanel_types.go`

#### C-1: Mojibake comments (lines 504, 566-568)

- **Line 504**: Replace `窶・` with ` -- ` in `// Not a git repository 窶・return empty result`
- **Lines 566-568**: Replace Shift-JIS garbled comments in `collectUntrackedFiles` with English:
  ```go
  // git ls-files --others --exclude-standard is preferred over git status -uall:
  // - Always returns individual file paths (not directory entries)
  // - Behavior is stable across git versions
  // - Correctly applies .gitignore rules
  ```

#### C-5: Space-in-filename diff header parsing (lines 777-812)

Rewrite `parseDiffHeaderPaths` to handle unquoted paths with spaces:
```go
func parseDiffHeaderPaths(header string) (oldPath, newPath string, ok bool) {
    header = strings.TrimSpace(header)

    // Quoted paths (handles special chars, spaces, non-ASCII).
    if strings.HasPrefix(header, "\"") {
        return parseDiffHeaderPathsQuoted(header)
    }

    // Unquoted paths: header format is "a/<old> b/<new>".
    // NOTE: git diff without --no-prefix always uses "a/" and "b/" prefixes.
    if !strings.HasPrefix(header, "a/") {
        return "", "", false
    }

    // For non-rename diffs (majority), old == new, so header = "a/<P> b/<P>".
    // Use path-length symmetry to find the split point.
    content := header[2:] // strip "a/"
    if len(content) >= 3 {
        pathLen := (len(content) - 3) / 2
        if 2*pathLen+3 == len(content) {
            candidate := content[:pathLen]
            if content[pathLen:pathLen+3] == " b/" && content[pathLen+3:] == candidate {
                return candidate, candidate, true
            }
        }
    }

    // Fallback for renames or asymmetric paths: split at first " b/".
    sepIdx := strings.Index(header, " b/")
    if sepIdx < 0 {
        return "", "", false
    }
    return header[2:sepIdx], header[sepIdx+3:], true
}
```

Extract existing quoted-path logic into `parseDiffHeaderPathsQuoted` helper.

Add test cases:
- `"my file.txt"` (space in path, unquoted)
- `"dir/sub dir/file.txt"` (nested space)
- Rename with space in old/new

#### C-6: diff-tree `--` argument order (line 474)

```go
// Before:
output, gitErr := gitpkg.RunGitCLIPublic(repoDir, []string{"diff-tree", "--root", "-p", "--", commitHash})
// After:
output, gitErr := gitpkg.RunGitCLIPublic(repoDir, []string{"diff-tree", "--root", "-p", commitHash, "--"})
```

#### I-9: git diff HEAD error discrimination (lines 512-516)

```go
if gitErr != nil {
    errMsg := gitErr.Error()
    isFreshRepo := strings.Contains(errMsg, "unknown revision") ||
        strings.Contains(errMsg, "bad revision")
    if !isFreshRepo {
        return WorkingDiffResult{}, fmt.Errorf("git diff HEAD failed: %w", gitErr)
    }
    // NOTE: HEAD may not exist in a fresh repository (no commits yet).
    // Continue with untracked-only diff.
    slog.Warn("[DEVPANEL] git diff HEAD failed (fresh repo), continuing with untracked only", "error", gitErr)
    output = nil
}
```

#### I-10: WalkDir error handling (lines 619-641)

```go
walkErr := filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
    if err != nil {
        slog.Warn("[DEVPANEL] failed to walk untracked path", "path", path, "error", err)
        if d != nil && d.IsDir() {
            return filepath.SkipDir
        }
        return nil
    }
    // ... rest unchanged
})
if walkErr != nil {
    slog.Warn("[DEVPANEL] WalkDir completed with error", "path", absPath, "error", walkErr)
}
```

#### S-1: Status type constants (`app_devpanel_types.go` line 46)

```go
// WorkingDiffStatus represents the change status of a file.
type WorkingDiffStatus = string

const (
    WorkingDiffStatusModified  WorkingDiffStatus = "modified"
    WorkingDiffStatusAdded     WorkingDiffStatus = "added"
    WorkingDiffStatusDeleted   WorkingDiffStatus = "deleted"
    WorkingDiffStatusRenamed   WorkingDiffStatus = "renamed"
    WorkingDiffStatusUntracked WorkingDiffStatus = "untracked"
)

type WorkingDiffFile struct {
    // ...
    Status    WorkingDiffStatus `json:"status"` // "modified" | "added" | "deleted" | "renamed" | "untracked"
}
```

Use `type WorkingDiffStatus = string` (type alias) to keep Wails binding compatibility. Update all assignments in `app_devpanel_api.go` to use constants.

#### S-9: `--no-prefix` assumption comment

Add to `parseDiffHeaderPaths`:
```go
// parseDiffHeaderPaths extracts old/new paths from a "diff --git" header line.
// Assumes default git diff format with "a/" and "b/" prefixes (NOT --no-prefix).
```

#### S-13: `\r\n` normalization in `buildUntrackedFileDiffSingle` (line 676-677)

```go
content := string(data)
// Normalize Windows line endings for consistent line splitting.
content = strings.ReplaceAll(content, "\r\n", "\n")
content = strings.ReplaceAll(content, "\r", "\n")
lines := strings.Split(content, "\n")
```

---

### G2: Go - Session Log & Types (`golang-expert` agent)

**Files:**
- `myT-x/app_session_log.go`
- `myT-x/app_session_log_types.go`

#### C-2: Mojibake em-dash (lines 87, 156)

```go
// Line 87: Before
// NOTE(A-0): The event model uses "ping + fetch" 窶・the emitted event
// After
// NOTE(A-0): The event model uses "ping + fetch" -- the emitted event

// Line 156: Before
// NOTE(A-0): Emit lightweight ping outside lock. No payload 窶・the frontend
// After
// NOTE(A-0): Emit lightweight ping outside lock. No payload -- the frontend
```

#### I-1: Seq uint64 precision comment (`app_session_log_types.go` line 8)

```go
Seq uint64 `json:"seq"` // auto-incrementing sequence number (assigned by writeSessionLogEntry)
// NOTE: uint64 values above 2^53-1 (Number.MAX_SAFE_INTEGER) lose precision
// in JavaScript. Current max (sessionLogMaxEntries=10000) is well within safe range.
```

#### I-2: Sync() os.ErrClosed guard (line 143)

Add `"errors"` import. Change:
```go
if syncFile != nil {
    if syncErr := syncFile.Sync(); syncErr != nil {
        // NOTE: os.ErrClosed may occur during shutdown if closeSessionLog()
        // races with this post-lock Sync(). This is benign -- the file was
        // already flushed and closed by closeSessionLog().
        if !errors.Is(syncErr, os.ErrClosed) {
            fmt.Fprintf(os.Stderr, "[session-log] failed to sync log file: %v\n", syncErr)
        }
    }
}
```

#### I-3: cleanupOldSessionLogs PID sort comment (after line 65)

```go
// NOTE: sort.Strings sorts lexicographically. Files with the same timestamp but
// different PIDs are ordered by PID string value (not numeric). This is acceptable
// because cleanup only needs approximate age ordering -- the timestamp prefix
// ensures files are primarily ordered by creation time.
sort.Strings(logFiles)
```

---

### G3: Go - Internal sessionlog handler (`golang-expert` agent)

**Files:**
- `myT-x/internal/sessionlog/handler.go`
- `myT-x/internal/sessionlog/handler_test.go`

#### C-3: WithGroup("") early return (line 76)

```go
func (h *TeeHandler) WithGroup(name string) slog.Handler {
    if name == "" {
        return h // slog.Handler spec: empty group name returns the receiver unchanged.
    }
    newGroup := name
    if h.group != "" {
        newGroup = h.group + "." + name
    }
    // ... rest unchanged
}
```

#### I-14: Callback panic recovery (lines 52-60)

```go
func (h *TeeHandler) Handle(ctx context.Context, record slog.Record) error {
    err := h.base.Handle(ctx, record)

    if h.callback != nil && record.Level >= h.minLevel {
        func() {
            defer func() {
                if r := recover(); r != nil {
                    // NOTE: Callback panic is logged to stderr (not slog) to avoid
                    // recursive TeeHandler invocation. The base handler result is
                    // preserved and returned to the caller.
                    fmt.Fprintf(os.Stderr, "[session-log] callback panicked: %v\n", r)
                }
            }()
            h.callback(record.Time, record.Level, record.Message, h.group)
        }()
    }

    return err
}
```

Add `"fmt"` and `"os"` imports.

#### S-10: Test error-ignore intent comment (`handler_test.go`)

Add comment to `TestTeeHandler_BaseHandlerError_CallbackStillCalled`:
```go
// Intentionally ignore the returned error -- this test verifies that the
// callback is invoked even when the base handler returns an error.
_ = handler.Handle(context.Background(), record)
```

#### New tests:

- `TestTeeHandler_WithGroupEmpty`: verify `WithGroup("")` returns same handler
- `TestTeeHandler_WithGroupEmpty_PreservesExistingGroup`: verify `h.WithGroup("foo").WithGroup("")` preserves "foo" group
- `TestTeeHandler_CallbackPanic_DoesNotPropagate`: verify callback panic is recovered and base error still returned

---

### G4: Go - Test improvements (`golang-expert` agent)

**Files:**
- `myT-x/internal/tmux/session_manager_window_test.go`
- `myT-x/internal/tmux/session_manager_directional_test.go`
- `myT-x/internal/tmux/command_router_handlers_window_test.go`

#### I-4: wantWindowID: 0 clarification

Add comment to test cases where `wantOK: false` and `wantID: 0`:
```go
{
    name: "empty windows returns false",
    // wantID is ignored when wantOK=false (zero value is not meaningful).
    wantID: 0,
    wantOK: false,
},
```

#### S-14: Test helper extraction

Create `injectTestWindow` helper in appropriate test file (using `t.Helper()`):
```go
func injectTestWindow(t *testing.T, sm *SessionManager, sessionName, windowName string) {
    t.Helper()
    sm.mu.Lock()
    defer sm.mu.Unlock()
    // ... window creation logic
}
```

Evaluate scope: if pattern appears in 5+ places across files, extract to `internal/testutil/`. Otherwise, keep file-local.

---

### G5: Frontend - TypeScript/React/CSS (`golang-expert` agent with frontend knowledge)

**Files:**
- `myT-x/frontend/src/components/viewer/views/diff-view/diffViewTypes.ts`
- `myT-x/frontend/src/stores/errorLogStore.ts`
- `myT-x/frontend/src/hooks/useBackendSync.ts`
- `myT-x/frontend/src/components/viewer/views/diff-view/DiffFileSidebar.tsx`
- `myT-x/frontend/src/components/viewer/views/diff-view/DiffContentViewer.tsx`
- `myT-x/frontend/src/components/viewer/views/diff-view/DiffView.tsx`
- `myT-x/frontend/src/components/viewer/views/diff-view/useDiffView.ts`
- `myT-x/frontend/src/components/viewer/ViewerSystem.tsx`
- `myT-x/frontend/src/components/viewer/viewerRegistry.ts`
- `myT-x/frontend/src/components/viewer/ActivityStrip.tsx`
- `myT-x/frontend/src/styles/viewer.css`

#### C-4: Type duplication elimination (`diffViewTypes.ts`)

Replace duplicate interfaces with re-exports from Wails-generated models:
```typescript
// Re-export from auto-generated Wails bindings (single source of truth).
import { main } from "../../../../../wailsjs/go/models";
export type WorkingDiffFile = main.WorkingDiffFile;
export type WorkingDiffResult = main.WorkingDiffResult;

// DiffTreeNode is a frontend-only view model.
export interface DiffTreeNode {
  name: string;
  path: string;
  isDir: boolean;
  depth: number;
  isExpanded: boolean;
  file?: WorkingDiffFile;
}
```

Verify all import sites (`useDiffView.ts`, `DiffContentViewer.tsx`, `DiffFileSidebar.tsx`) remain compatible.

#### I-5: .diff-expand-bar:hover dead CSS (viewer.css line 848)

```css
.diff-expand-bar:hover {
    background: var(--surface-3, var(--accent-10));
    color: var(--fg-main);
    cursor: pointer;
}
```

Also update base `.diff-expand-bar` cursor from `default` to `not-allowed` or remove it (since hover now shows pointer).

#### I-6: useBackendSync stale promise guard

The current implementation is safe by design (setEntries always replaces with full snapshot). Add defensive seq-based comment:
```typescript
// IMP-02: Load initial error log entries after subscribing to the ping event.
// By subscribing first, then loading the snapshot, we ensure no entries
// are missed: pings arriving during the fetch trigger setEntries which
// always replaces with the latest full snapshot. No seq-based guard is
// needed because each setEntries call is a complete replacement, not a merge.
```

#### I-7: normalizeEntries sort (`errorLogStore.ts` line 19-21)

```typescript
function normalizeEntries(incoming: ErrorLogEntry[]): ErrorLogEntry[] {
    return incoming
        .filter((entry) => isValidSeq(entry.seq))
        .sort((a, b) => a.seq - b.seq)
        .slice(-MAX_ENTRIES);
}
```

#### I-8: DiffFileSidebar accessibility (`DiffFileSidebar.tsx` Row)

```tsx
<div
    role="treeitem"
    tabIndex={0}
    aria-selected={isSelected}
    aria-expanded={node.isDir ? node.isExpanded : undefined}
    className={`tree-node-row${isSelected ? " selected" : ""}`}
    style={{ ...style, paddingLeft: 8 + node.depth * 16 }}
    onClick={handleClick}
    onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            handleClick();
        }
    }}
>
```

Also add `role="tree"` and `aria-label="Changed files"` to `FixedSizeList`'s container.

#### I-11: unreadCount optimization (`errorLogStore.ts`)

Replace full-array `reduce` with reverse scan:
```typescript
setEntries: (incoming) =>
    set((state) => {
        const entries = normalizeEntries(incoming);
        // Entries are sorted by seq (ascending). Scan from end for unread count.
        let unreadCount = 0;
        for (let i = entries.length - 1; i >= 0; i--) {
            if (entries[i].seq <= state.lastReadSeq) break;
            unreadCount++;
        }
        return { entries, unreadCount };
    }),
```

#### I-12: Eviction spec comment (`errorLogStore.ts`)

Add above `normalizeEntries`:
```typescript
// NOTE: When eviction occurs (entries exceed MAX_ENTRIES), the oldest entries
// are discarded. If evicted entries were unread, the unread count decreases
// accordingly because unreadCount is recomputed from the current entries array
// on every setEntries call. This is the intended behavior: very old unread
// entries are silently aged out.
```

#### I-13: Shortcut duplication elimination (`ViewerSystem.tsx`)

All 4 views already register with `shortcut` field. Refactor `ViewerSystem.tsx` to read from registry:
```tsx
useEffect(() => {
    const views = getRegisteredViews();

    // Build shortcut map from registry (single source of truth).
    const shortcutMap = new Map<string, string>();
    for (const view of views) {
        if (view.shortcut) {
            shortcutMap.set(view.shortcut.toLowerCase(), view.id);
        }
    }

    const handler = (e: KeyboardEvent) => {
        if (e.defaultPrevented) return;
        if (isImeTransitionalEvent(e)) return;

        if (e.key === "Escape" && activeViewId !== null) {
            e.preventDefault();
            closeView();
            return;
        }

        if (e.ctrlKey && e.shiftKey) {
            const combo = `ctrl+shift+${e.key.toLowerCase()}`;
            const viewId = shortcutMap.get(combo);
            if (viewId) {
                e.preventDefault();
                toggleView(viewId);
            }
        }
    };

    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
}, [activeViewId, toggleView, closeView]);
```

Also add `shortcut` type enforcement in `viewerRegistry.ts`:
```typescript
export interface ViewPlugin {
    // ... existing fields
    /** Keyboard shortcut in format "Ctrl+Shift+X". Used by ViewerSystem for dynamic binding. */
    shortcut?: string;
}
```

#### S-3: ActivityStrip early return optimization

```typescript
export function ActivityStrip() {
    const views = getRegisteredViews();
    if (views.length === 0) return null;

    const activeViewId = useViewerStore((s) => s.activeViewId);
    // ... rest of component
```

**WAIT**: React hooks must not be called conditionally. Keep current structure but note: the early return at line 33 is already after hooks. The optimization of moving it before filters is negligible -- skip this item.

#### S-6: Renamed file color (`DiffFileSidebar.tsx`)

```typescript
function statusColor(status: string): string {
    switch (status) {
        case "added":
        case "untracked":
            return "var(--git-staged)";
        case "deleted":
            return "var(--danger)";
        case "renamed":
            return "var(--warning, var(--fg-main))";
        default:
            return "var(--fg-main)";
    }
}
```

#### S-7: useMemo dependency optimization (`DiffContentViewer.tsx` line 115)

```typescript
const parsed = useMemo(
    () => (file ? parseFileDiff(file.diff) : { hunks: [], gaps: new Map() }),
    [file?.diff],
);
```

#### S-8: old_path display for renamed files (`DiffContentViewer.tsx`)

```tsx
<div className="diff-content-header">
    <span className="diff-content-path">
        {file.status === "renamed" && file.old_path && file.old_path !== file.path
            ? `${file.old_path} → ${file.path}`
            : file.path}
    </span>
    // ... stats unchanged
</div>
```

#### S-11: Inline style to CSS class (`DiffView.tsx` line 68)

```tsx
// Before
<span className="diff-header-file-count" style={{ color: "var(--warning)" }}>
// After
<span className="diff-header-truncated">
```

Add to `viewer.css`:
```css
.diff-header-truncated {
    color: var(--warning);
    font-size: 0.82rem;
}
```

#### S-12: Indent unification

Unify all diff-view TSX/TS files to 4-space indent (matching `DiffContentViewer.tsx` and `useDiffView.ts`). Files to reformat:
- `DiffView.tsx` (2→4)
- `DiffFileSidebar.tsx` (2→4)

---

## Execution Order

```
Phase 1: Launch 5 parallel agents (G1-G5)
  G1: golang-expert — DevPanel API fixes (C-1, C-5, C-6, I-9, I-10, S-1, S-9, S-13)
  G2: golang-expert — Session Log fixes (C-2, I-1, I-2, I-3)
  G3: golang-expert — Internal handler fixes + tests (C-3, I-14, S-10)
  G4: golang-expert — Test improvements (I-4, S-14)
  G5: golang-expert — Frontend fixes (C-4, I-5, I-6, I-7, I-8, I-11, I-12, I-13, S-6, S-7, S-8, S-11, S-12)

Phase 2: Run all tests
  go test ./myT-x/... -count=1
  Fix any failures across all packages (including pre-existing failures)

Phase 3: Build verification
  go build ./myT-x/...
  go vet ./myT-x/...
```

## Items NOT requiring changes (verified OK)

| ID | Reason |
|----|--------|
| S-3 | React hooks cannot be called conditionally; current early return is correctly placed after hooks |
| S-4 | Large diff virtualization is a feature enhancement beyond this review scope |
| S-5 | `setDiffResult(null)` already exists in session-change reset effect (useDiffView.ts:91) |
| I-6 | Race condition is mitigated by design (full snapshot replacement). Comment added instead of code change |

## Verification

1. **Go tests**: `go test ./myT-x/... -count=1` -- all must pass
2. **Go build**: `go build ./myT-x/...` -- no errors
3. **Go vet**: `go vet ./myT-x/...` -- no warnings
4. **Frontend build**: `cd myT-x/frontend && npm run build` -- no TS errors
5. **Spot checks**:
   - Verify no `窶・` or Shift-JIS remnants: `grep -r "窶" myT-x/`
   - Verify diff-tree argument order visually
   - Verify WithGroup("") returns receiver via new test
