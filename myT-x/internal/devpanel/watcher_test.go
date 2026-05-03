package devpanel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

type stubManagedWatcher struct {
	stopErr  error
	degraded bool
}

func (w *stubManagedWatcher) Stop() error             { return w.stopErr }
func (w *stubManagedWatcher) ignorePaths(...string)   {}
func (w *stubManagedWatcher) isReusable() bool        { return !w.degraded }
func (w *stubManagedWatcher) markDegraded(string)     { w.degraded = true }
func (w *stubManagedWatcher) unignorePaths(...string) {}

type panicInvalidationEmitter struct {
	events *testEmitter
}

func (e *panicInvalidationEmitter) Emit(name string, payload any) {
	if name == treeInvalidatedEventName {
		panic("emit panic")
	}
	e.events.Emit(name, payload)
}

func (e *panicInvalidationEmitter) EmitWithContext(_ context.Context, name string, payload any) {
	e.Emit(name, payload)
}

func TestTreeWatcherStartMarksStoppedAfterRunPanic(t *testing.T) {
	watcher := &treeWatcher{
		sessionName:  "session-a",
		emitter:      &testEmitter{},
		pendingPaths: make(map[string]struct{}),
		ignoredPaths: make(map[string]time.Time),
	}

	watcher.Start()
	watcher.wg.Wait()

	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	if !watcher.stopped {
		t.Fatal("watcher should mark itself stopped after recovering from a run panic")
	}
}

func TestTreeWatcherStartEmitsWatcherFailedEventAfterRunPanic(t *testing.T) {
	emitter := &testEmitter{}
	watcher := &treeWatcher{
		sessionName:  "session-a",
		emitter:      emitter,
		pendingPaths: make(map[string]struct{}),
		ignoredPaths: make(map[string]time.Time),
	}

	watcher.Start()
	event := emitter.waitForEvent(t, watcherFailedEventName, time.Second)
	payload, ok := event.payload.(WatcherFailedEvent)
	if !ok {
		t.Fatalf("payload type = %T, want WatcherFailedEvent", event.payload)
	}
	if payload.SessionName != "session-a" {
		t.Fatalf("SessionName = %q, want %q", payload.SessionName, "session-a")
	}
	if payload.Message == "" {
		t.Fatal("Message should describe the watcher failure")
	}
}

func TestTreeWatcherFlushMarksStoppedAfterPanic(t *testing.T) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	t.Cleanup(func() {
		_ = fsWatcher.Close()
	})

	emitter := &panicInvalidationEmitter{events: &testEmitter{}}
	watcher := &treeWatcher{
		sessionName:  "session-a",
		emitter:      emitter,
		watcher:      fsWatcher,
		pendingPaths: map[string]struct{}{"": {}},
		ignoredPaths: make(map[string]time.Time),
	}

	watcher.flush()

	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	if !watcher.stopped {
		t.Fatal("watcher should mark itself stopped after recovering from a flush panic")
	}
}

func TestTreeWatcherFlushEmitsWatcherFailedEventAfterPanic(t *testing.T) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	t.Cleanup(func() {
		_ = fsWatcher.Close()
	})

	events := &testEmitter{}
	watcher := &treeWatcher{
		sessionName:  "session-a",
		emitter:      &panicInvalidationEmitter{events: events},
		watcher:      fsWatcher,
		pendingPaths: map[string]struct{}{"": {}},
		ignoredPaths: make(map[string]time.Time),
	}

	watcher.flush()

	event := events.waitForEvent(t, watcherFailedEventName, time.Second)
	payload, ok := event.payload.(WatcherFailedEvent)
	if !ok {
		t.Fatalf("payload type = %T, want WatcherFailedEvent", event.payload)
	}
	if payload.SessionName != "session-a" {
		t.Fatalf("SessionName = %q, want %q", payload.SessionName, "session-a")
	}
	if payload.Message == "" {
		t.Fatal("Message should describe the watcher failure")
	}
}

func TestTreeWatcherEmitWatcherDegradedEmitsOnce(t *testing.T) {
	emitter := &testEmitter{}
	watcher := &treeWatcher{
		sessionName:  "session-a",
		emitter:      emitter,
		pendingPaths: make(map[string]struct{}),
		ignoredPaths: make(map[string]time.Time),
	}

	watcher.emitWatcherDegraded("Automatic refresh is partially unavailable.")
	watcher.emitWatcherDegraded("Automatic refresh is partially unavailable.")

	event := emitter.waitForEvent(t, watcherFailedEventName, time.Second)
	payload, ok := event.payload.(WatcherFailedEvent)
	if !ok {
		t.Fatalf("payload type = %T, want WatcherFailedEvent", event.payload)
	}
	if payload.SessionName != "session-a" {
		t.Fatalf("SessionName = %q, want %q", payload.SessionName, "session-a")
	}

	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	count := 0
	for _, recorded := range emitter.events {
		if recorded.name == watcherFailedEventName {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("watcherFailed event count = %d, want 1", count)
	}
}

func TestTreeWatcherIsReusableReturnsFalseAfterDegraded(t *testing.T) {
	watcher := &treeWatcher{
		sessionName:  "session-a",
		emitter:      &testEmitter{},
		pendingPaths: make(map[string]struct{}),
		ignoredPaths: make(map[string]time.Time),
	}
	if !watcher.isReusable() {
		t.Fatal("fresh watcher should be reusable")
	}

	watcher.emitWatcherDegraded("Automatic refresh is unavailable after a watcher error.")
	if watcher.isReusable() {
		t.Fatal("degraded watcher should not be reusable")
	}
}

func TestTreeWatcherHandleEventUsesRenamedSessionForInvalidation(t *testing.T) {
	rootDir := t.TempDir()
	filePath := filepath.Join(rootDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	emitter := &testEmitter{}
	dirCache := NewDirCache(time.Second)
	dirCache.Set("new-session", "", []FileEntry{{Name: "file.txt", Path: "file.txt", Size: 4}})
	dirCache.Set("new-session", "file.txt", []FileEntry{{Name: "stale", Path: "stale", Size: 1}})

	watcher := &treeWatcher{
		sessionName:      "old-session",
		rootDir:          rootDir,
		dirCache:         dirCache,
		emitter:          emitter,
		debounceInterval: testWatcherDebounceInterval,
		pendingPaths:     make(map[string]struct{}),
		ignoredPaths:     make(map[string]time.Time),
		watchedDirs:      make(map[string]struct{}),
	}
	watcher.renameSession("new-session")

	watcher.handleEvent(fsnotify.Event{Name: filePath, Op: fsnotify.Write})

	event := emitter.waitForEvent(t, treeInvalidatedEventName, time.Second)
	payload, ok := event.payload.(TreeInvalidationEvent)
	if !ok {
		t.Fatalf("payload type = %T, want TreeInvalidationEvent", event.payload)
	}
	if payload.SessionName != "new-session" {
		t.Fatalf("SessionName = %q, want %q", payload.SessionName, "new-session")
	}
	watcher.wg.Wait()

	if _, ok := dirCache.Get("new-session", "file.txt"); ok {
		t.Fatal("renamed session cache should be invalidated by the active watcher")
	}
}

func TestTreeWatcherHandleEventInvalidatesAncestorViewTargetPaths(t *testing.T) {
	rootDir := t.TempDir()
	filePath := filepath.Join(rootDir, "docs", "nested", "guide.md")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("# guide"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	emitter := &testEmitter{}
	dirCache := NewDirCache(time.Second)
	cachedPaths := []string{"", "docs", "docs/nested", "docs/nested/guide.md"}
	for _, path := range cachedPaths {
		dirCache.Set("session-a", path, []FileEntry{{Name: "stale", Path: path}})
	}

	watcher := &treeWatcher{
		sessionName:      "session-a",
		rootDir:          rootDir,
		dirCache:         dirCache,
		emitter:          emitter,
		debounceInterval: testWatcherDebounceInterval,
		pendingPaths:     make(map[string]struct{}),
		ignoredPaths:     make(map[string]time.Time),
		watchedDirs:      make(map[string]struct{}),
	}

	watcher.handleEvent(fsnotify.Event{Name: filePath, Op: fsnotify.Write})

	event := emitter.waitForEvent(t, treeInvalidatedEventName, time.Second)
	payload, ok := event.payload.(TreeInvalidationEvent)
	if !ok {
		t.Fatalf("payload type = %T, want TreeInvalidationEvent", event.payload)
	}
	if payload.SessionName != "session-a" {
		t.Fatalf("SessionName = %q, want %q", payload.SessionName, "session-a")
	}
	wantPaths := []string{"", "docs", "docs/nested", "docs/nested/guide.md"}
	if !reflect.DeepEqual(payload.Paths, wantPaths) {
		t.Fatalf("Paths = %v, want %v", payload.Paths, wantPaths)
	}
	watcher.wg.Wait()

	for _, path := range cachedPaths {
		if _, ok := dirCache.Get("session-a", path); ok {
			t.Fatalf("cache path %q should be invalidated", path)
		}
	}
}

func TestTreeWatcherAddRecursiveDirectoryLimitEmitsWatcherFailedEvent(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootDir, "child"), 0o755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	t.Cleanup(func() {
		_ = fsWatcher.Close()
	})

	emitter := &testEmitter{}
	watcher := &treeWatcher{
		sessionName:  "session-a",
		rootDir:      rootDir,
		emitter:      emitter,
		watcher:      fsWatcher,
		pendingPaths: make(map[string]struct{}),
		ignoredPaths: make(map[string]time.Time),
		watchedDirs:  make(map[string]struct{}),
		watchedCount: defaultWatcherMaxDirectories,
	}

	if err := watcher.addRecursive(rootDir); err != nil {
		t.Fatalf("addRecursive failed: %v", err)
	}

	event := emitter.waitForEvent(t, watcherFailedEventName, time.Second)
	payload, ok := event.payload.(WatcherFailedEvent)
	if !ok {
		t.Fatalf("payload type = %T, want WatcherFailedEvent", event.payload)
	}
	if payload.SessionName != "session-a" {
		t.Fatalf("SessionName = %q, want %q", payload.SessionName, "session-a")
	}
	if !strings.Contains(payload.Message, "directory limit") {
		t.Fatalf("Message = %q, want directory-limit guidance", payload.Message)
	}
}

func TestTreeInvalidationEventFieldCount(t *testing.T) {
	if got := reflect.TypeFor[TreeInvalidationEvent]().NumField(); got != 2 {
		t.Fatalf("TreeInvalidationEvent field count = %d, want 2; update this test when TreeInvalidationEvent changes", got)
	}
}

func TestWatcherManagerStartRotatesWatcherWhenRootChanges(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	dirCache := NewDirCache(time.Second)
	dirCache.Set("session-a", "", []FileEntry{{Name: "stale", Path: "stale", IsDir: true}})

	manager := newWatcherManager(dirCache, &testEmitter{})
	manager.debounceInterval = testWatcherDebounceInterval
	if err := manager.start("session-a", rootA); err != nil {
		t.Fatalf("start rootA failed: %v", err)
	}
	t.Cleanup(func() {
		_ = manager.stopAll()
	})

	manager.mu.Lock()
	firstEntry := manager.watchers["session-a"]
	manager.mu.Unlock()
	if firstEntry == nil {
		t.Fatal("expected watcher entry after initial start")
	}

	if err := manager.start("session-a", rootB); err != nil {
		t.Fatalf("start rootB failed: %v", err)
	}

	manager.mu.Lock()
	rotatedEntry := manager.watchers["session-a"]
	manager.mu.Unlock()
	if rotatedEntry == nil {
		t.Fatal("expected watcher entry after rotation")
	}
	if rotatedEntry.rootDir != rootB {
		t.Fatalf("rotated rootDir = %q, want %q", rotatedEntry.rootDir, rootB)
	}
	if rotatedEntry.refCount != 1 {
		t.Fatalf("rotated refCount = %d, want 1", rotatedEntry.refCount)
	}
	if rotatedEntry.watcher == firstEntry.watcher {
		t.Fatal("watcher should be recreated when the root directory changes")
	}
	if _, ok := dirCache.Get("session-a", ""); ok {
		t.Fatal("dir cache should be invalidated when rotating the watcher root")
	}
}

func TestTreeWatcherIgnorePathsExpire(t *testing.T) {
	watcher := &treeWatcher{
		ignoreWindow: 20 * time.Millisecond,
		pendingPaths: make(map[string]struct{}),
		ignoredPaths: make(map[string]time.Time),
	}

	watcher.ignorePaths("src/file.txt")
	if !watcher.shouldIgnorePath("src/file.txt") {
		t.Fatal("path should be ignored before the ignore window expires")
	}

	time.Sleep(50 * time.Millisecond)

	if watcher.shouldIgnorePath("src/file.txt") {
		t.Fatal("path should stop being ignored after the ignore window expires")
	}
	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	if len(watcher.ignoredPaths) != 0 {
		t.Fatalf("ignoredPaths len = %d, want 0 after expiry pruning", len(watcher.ignoredPaths))
	}
}

func TestTreeWatcherIgnoresInternalTempFiles(t *testing.T) {
	watcher := &treeWatcher{
		pendingPaths: make(map[string]struct{}),
		ignoredPaths: make(map[string]time.Time),
	}

	if !watcher.shouldIgnorePath("src/.mytx-tmp-12345") {
		t.Fatal("internal temp files should be ignored")
	}
	if watcher.shouldIgnorePath("src/real-file.txt") {
		t.Fatal("regular files should not be ignored without an explicit ignore rule")
	}
}

func TestWatchPathDepthBoundary(t *testing.T) {
	maxDepthPath := strings.Repeat("dir/", defaultWatcherMaxDepth-1) + "dir"
	overDepthPath := maxDepthPath + "/extra"

	if got := watchPathDepth(""); got != 0 {
		t.Fatalf("watchPathDepth(\"\") = %d, want 0", got)
	}
	if got := watchPathDepth(maxDepthPath); got != defaultWatcherMaxDepth {
		t.Fatalf("watchPathDepth(maxDepthPath) = %d, want %d", got, defaultWatcherMaxDepth)
	}
	if got := watchPathDepth(overDepthPath); got != defaultWatcherMaxDepth+1 {
		t.Fatalf("watchPathDepth(overDepthPath) = %d, want %d", got, defaultWatcherMaxDepth+1)
	}
}

func TestWatcherManagerStopAllPreservesEntriesWhenStopFails(t *testing.T) {
	manager := newWatcherManager(NewDirCache(time.Second), &testEmitter{})
	manager.watchers["session-a"] = &sessionWatcher{
		refCount: 1,
		rootDir:  "root",
		watcher:  &stubManagedWatcher{stopErr: errors.New("stop failed")},
	}

	if err := manager.stopAll(); err == nil {
		t.Fatal("stopAll() expected aggregated stop error")
	}
	entry := manager.watchers["session-a"]
	if entry == nil {
		t.Fatal("watcher entry should be preserved when Stop() fails")
	}
	stub, ok := entry.watcher.(*stubManagedWatcher)
	if !ok {
		t.Fatalf("watcher type = %T, want *stubManagedWatcher", entry.watcher)
	}
	if !stub.degraded {
		t.Fatal("watcher should be marked degraded after stopAll failure")
	}
}

func TestWatcherManagerStartReplacesStoppedWatcherWhenRootIsUnchanged(t *testing.T) {
	rootDir := t.TempDir()
	manager := newWatcherManager(NewDirCache(time.Second), &testEmitter{})
	manager.debounceInterval = testWatcherDebounceInterval
	stoppedWatcher := &treeWatcher{
		sessionName:  "session-a",
		rootDir:      rootDir,
		pendingPaths: make(map[string]struct{}),
		ignoredPaths: make(map[string]time.Time),
		stopped:      true,
	}
	manager.watchers["session-a"] = &sessionWatcher{
		refCount: 1,
		rootDir:  rootDir,
		watcher:  stoppedWatcher,
	}

	if err := manager.start("session-a", rootDir); err != nil {
		t.Fatalf("start(root unchanged) failed: %v", err)
	}
	t.Cleanup(func() {
		_ = manager.stopAll()
	})

	manager.mu.Lock()
	entry := manager.watchers["session-a"]
	manager.mu.Unlock()
	if entry == nil {
		t.Fatal("expected watcher entry after replacing stopped watcher")
	}
	if entry.refCount != 1 {
		t.Fatalf("replacement refCount = %d, want 1", entry.refCount)
	}
	if entry.watcher == stoppedWatcher {
		t.Fatal("start should replace a stopped watcher instead of reusing it")
	}
}
