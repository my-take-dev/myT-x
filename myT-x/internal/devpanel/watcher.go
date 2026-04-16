package devpanel

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"myT-x/internal/apptypes"
)

const (
	treeInvalidatedEventName       = "devpanel:tree-invalidated"
	watcherFailedEventName         = "devpanel:watcher-failed"
	defaultWatcherDebounceInterval = 100 * time.Millisecond
	defaultWatcherIgnoreWindow     = 750 * time.Millisecond
	defaultWatcherMaxDepth         = 64
	defaultWatcherMaxDirectories   = 65536
	internalTempFilePrefix         = ".mytx-tmp-"
	watcherStopFailureMessage      = "Automatic refresh is unavailable because the watcher could not be stopped cleanly. Reload the directory manually."
	watcherRestartFailureMessage   = "Automatic refresh is unavailable because the watcher could not be restarted. Reload the directory manually."
)

// TreeInvalidationEvent notifies the frontend that one or more loaded
// directories should be refreshed for a session.
type TreeInvalidationEvent struct {
	SessionName string   `json:"session_name"`
	Paths       []string `json:"paths"`
}

// WatcherFailedEvent notifies the frontend that automatic refresh stopped for a
// session and manual reload is required until the watcher is restarted.
type WatcherFailedEvent struct {
	SessionName string `json:"session_name"`
	Message     string `json:"message"`
}

type watcherManager struct {
	mu               sync.Mutex
	dirCache         *DirCache
	emitter          apptypes.RuntimeEventEmitter
	debounceInterval time.Duration
	ignoreWindow     time.Duration
	watchers         map[string]*sessionWatcher
	starting         map[string]*pendingWatcherStart
}

type managedWatcher interface {
	Stop() error
	ignorePaths(paths ...string)
	isReusable() bool
	markDegraded(message string)
	unignorePaths(paths ...string)
}

type sessionWatcher struct {
	refCount int
	rootDir  string
	watcher  managedWatcher
	// pendingRenameHandoffs tracks how many StartWatcher calls should adopt the
	// migrated watcher instead of incrementing refCount. Set to the current
	// refCount at rename time so that every subscriber whose cleanup fires
	// against the old (now-absent) session name gets one free re-attach under
	// the new name without inflating the counter.
	pendingRenameHandoffs int
}

type pendingWatcherStart struct {
	done chan struct{}
}

var newTreeWatcherFunc = newTreeWatcher

func newWatcherManager(dirCache *DirCache, emitter apptypes.RuntimeEventEmitter) *watcherManager {
	// NewService already normalizes a nil emitter, but keep this guard so tests
	// can construct watcherManager directly without depending on Service wiring.
	if emitter == nil {
		emitter = apptypes.NoopEmitter{}
	}

	return &watcherManager{
		dirCache:         dirCache,
		emitter:          emitter,
		debounceInterval: defaultWatcherDebounceInterval,
		ignoreWindow:     defaultWatcherIgnoreWindow,
		watchers:         make(map[string]*sessionWatcher),
		starting:         make(map[string]*pendingWatcherStart),
	}
}

func (m *watcherManager) start(sessionName, rootDir string) error {
	for {
		var (
			pending      *pendingWatcherStart
			pendingDone  chan struct{}
			existing     *sessionWatcher
			shouldRotate bool
			rotated      bool
		)

		m.mu.Lock()
		if pending = m.starting[sessionName]; pending != nil {
			pendingDone = pending.done
			m.mu.Unlock()
			<-pendingDone
			continue
		}
		if existing = m.watchers[sessionName]; existing != nil {
			if existing.rootDir == rootDir && existing.watcher.isReusable() {
				if existing.pendingRenameHandoffs > 0 {
					existing.pendingRenameHandoffs--
					m.mu.Unlock()
					return nil
				}
				existing.refCount++
				m.mu.Unlock()
				return nil
			}
			shouldRotate = true
		}
		pending = &pendingWatcherStart{done: make(chan struct{})}
		m.starting[sessionName] = pending
		m.mu.Unlock()

		watcher, err := newTreeWatcherFunc(
			sessionName,
			rootDir,
			m.dirCache,
			m.emitter,
			m.debounceInterval,
			m.ignoreWindow,
		)
		if err == nil && shouldRotate {
			if stopErr := existing.watcher.Stop(); stopErr != nil {
				if cleanupErr := watcher.Stop(); cleanupErr != nil {
					stopErr = errors.Join(stopErr, cleanupErr)
				}
				existing.watcher.markDegraded(watcherStopFailureMessage)
				m.mu.Lock()
				delete(m.starting, sessionName)
				close(pending.done)
				m.mu.Unlock()
				return stopErr
			}
			rotated = true
			if m.dirCache != nil {
				m.dirCache.InvalidateAll(sessionName)
			}
		}
		if err == nil {
			watcher.Start()
		}
		m.mu.Lock()
		delete(m.starting, sessionName)
		if err == nil {
			m.watchers[sessionName] = &sessionWatcher{
				refCount: 1,
				rootDir:  rootDir,
				watcher:  watcher,
			}
		}
		close(pending.done)
		m.mu.Unlock()
		if err != nil {
			if rotated && existing != nil {
				existing.watcher.markDegraded(watcherRestartFailureMessage)
			}
			return err
		}
		return nil
	}
}

func (m *watcherManager) stop(sessionName string) error {
	for {
		m.mu.Lock()
		if pending := m.starting[sessionName]; pending != nil {
			done := pending.done
			m.mu.Unlock()
			<-done
			continue
		}
		entry, ok := m.watchers[sessionName]
		if !ok {
			m.mu.Unlock()
			return nil
		}
		if entry.refCount > 1 {
			entry.refCount--
			m.mu.Unlock()
			return nil
		}
		pending := &pendingWatcherStart{done: make(chan struct{})}
		m.starting[sessionName] = pending
		m.mu.Unlock()

		stopErr := entry.watcher.Stop()
		if stopErr != nil {
			entry.watcher.markDegraded(watcherStopFailureMessage)
		}

		m.mu.Lock()
		delete(m.starting, sessionName)
		if stopErr == nil && m.watchers[sessionName] == entry {
			delete(m.watchers, sessionName)
		}
		close(pending.done)
		m.mu.Unlock()
		return stopErr
	}
}

func (m *watcherManager) stopAll() error {
	for {
		m.mu.Lock()
		if len(m.starting) > 0 {
			var pending *pendingWatcherStart
			for _, entry := range m.starting {
				pending = entry
				break
			}
			done := pending.done
			m.mu.Unlock()
			<-done
			continue
		}
		watchers := make(map[string]*sessionWatcher, len(m.watchers))
		pendingBySession := make(map[string]*pendingWatcherStart, len(m.watchers))
		for sessionName, entry := range m.watchers {
			watchers[sessionName] = entry
			pending := &pendingWatcherStart{done: make(chan struct{})}
			pendingBySession[sessionName] = pending
			m.starting[sessionName] = pending
		}
		m.mu.Unlock()

		var stopErr error
		for sessionName, entry := range watchers {
			if err := entry.watcher.Stop(); err != nil {
				entry.watcher.markDegraded(watcherStopFailureMessage)
				stopErr = errors.Join(stopErr, err)
			} else {
				m.mu.Lock()
				if m.watchers[sessionName] == entry {
					delete(m.watchers, sessionName)
				}
				m.mu.Unlock()
			}
		}

		m.mu.Lock()
		for sessionName, pending := range pendingBySession {
			delete(m.starting, sessionName)
			close(pending.done)
		}
		m.mu.Unlock()
		return stopErr
	}
}

func (m *watcherManager) stopAllForSession(sessionName string) error {
	for {
		m.mu.Lock()
		if pending := m.starting[sessionName]; pending != nil {
			done := pending.done
			m.mu.Unlock()
			<-done
			continue
		}
		entry, ok := m.watchers[sessionName]
		if ok {
			pending := &pendingWatcherStart{done: make(chan struct{})}
			m.starting[sessionName] = pending
			m.mu.Unlock()
			stopErr := entry.watcher.Stop()
			if stopErr != nil {
				entry.watcher.markDegraded(watcherStopFailureMessage)
			}
			if m.dirCache != nil {
				m.dirCache.InvalidateAll(sessionName)
			}
			m.mu.Lock()
			delete(m.starting, sessionName)
			if stopErr == nil && m.watchers[sessionName] == entry {
				delete(m.watchers, sessionName)
			}
			close(pending.done)
			m.mu.Unlock()
			return stopErr
		}
		m.mu.Unlock()
		if m.dirCache != nil {
			m.dirCache.InvalidateAll(sessionName)
		}
		return nil
	}
}

func (m *watcherManager) ignorePaths(sessionName string, paths ...string) {
	m.applyToWatcherChain(sessionName, func(w managedWatcher) {
		w.ignorePaths(paths...)
	})
}

func (m *watcherManager) renameSession(oldSessionName, newSessionName string) error {
	oldSessionName = strings.TrimSpace(oldSessionName)
	newSessionName = strings.TrimSpace(newSessionName)
	if oldSessionName == "" || newSessionName == "" || oldSessionName == newSessionName {
		return nil
	}

	for {
		m.mu.Lock()
		if pending := m.starting[oldSessionName]; pending != nil {
			done := pending.done
			m.mu.Unlock()
			<-done
			continue
		}
		if pending := m.starting[newSessionName]; pending != nil {
			done := pending.done
			m.mu.Unlock()
			<-done
			continue
		}
		entry, ok := m.watchers[oldSessionName]
		if !ok {
			m.mu.Unlock()
			return nil
		}
		if _, exists := m.watchers[newSessionName]; exists {
			m.mu.Unlock()
			return errors.New("watcher entry already exists for renamed session")
		}
		delete(m.watchers, oldSessionName)
		m.watchers[newSessionName] = entry
		entry.pendingRenameHandoffs = entry.refCount
		if watcher, ok := entry.watcher.(*treeWatcher); ok {
			watcher.renameSession(newSessionName)
		}
		m.mu.Unlock()
		return nil
	}
}

func (m *watcherManager) unignorePaths(sessionName string, paths ...string) {
	m.applyToWatcherChain(sessionName, func(w managedWatcher) {
		w.unignorePaths(paths...)
	})
}

func (m *watcherManager) applyToWatcherChain(sessionName string, apply func(managedWatcher)) {
	var previous *sessionWatcher
	for {
		m.mu.Lock()
		entry := m.watchers[sessionName]
		m.mu.Unlock()
		if entry == nil || entry == previous {
			return
		}
		apply(entry.watcher)
		previous = entry
	}
}

type treeWatcher struct {
	sessionName      string
	rootDir          string
	dirCache         *DirCache
	emitter          apptypes.RuntimeEventEmitter
	watcher          *fsnotify.Watcher
	debounceInterval time.Duration
	ignoreWindow     time.Duration

	mu           sync.Mutex
	pendingPaths map[string]struct{}  // paths queued for the next debounced flush (mu)
	ignoredPaths map[string]time.Time // path → expiry time; events for these paths are suppressed (mu)
	debounce     *time.Timer          // current debounce timer, nil when not scheduled (mu)
	stopped      bool                 // true after Stop() is called (mu)
	degraded     bool                 // true after the frontend has been told auto-refresh is degraded (mu)

	// watchedCount and watchedDirs are accessed only from the run()
	// goroutine (via handleEvent/addRecursive) and during initial setup
	// (newTreeWatcher), so they do not require mu protection.
	watchedCount int
	watchedDirs  map[string]struct{} // tracks explicitly watched directory paths for accurate count management
	wg           sync.WaitGroup
}

func newTreeWatcher(
	sessionName string,
	rootDir string,
	dirCache *DirCache,
	emitter apptypes.RuntimeEventEmitter,
	debounceInterval time.Duration,
	ignoreWindow time.Duration,
) (*treeWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if debounceInterval <= 0 {
		debounceInterval = defaultWatcherDebounceInterval
	}
	if ignoreWindow <= 0 {
		ignoreWindow = defaultWatcherIgnoreWindow
	}

	watcher := &treeWatcher{
		sessionName:      sessionName,
		rootDir:          rootDir,
		dirCache:         dirCache,
		emitter:          emitter,
		watcher:          fsWatcher,
		debounceInterval: debounceInterval,
		ignoreWindow:     ignoreWindow,
		pendingPaths:     make(map[string]struct{}),
		ignoredPaths:     make(map[string]time.Time),
		watchedDirs:      make(map[string]struct{}),
	}
	if err := watcher.addRecursive(rootDir); err != nil {
		if closeErr := fsWatcher.Close(); closeErr != nil {
			slog.Warn("[DEVPANEL-WATCHER] failed to close fsnotify watcher after addRecursive error",
				"session", sessionName, "error", closeErr)
		}
		return nil, err
	}
	return watcher, nil
}

func (w *treeWatcher) emitWatcherFailed(message string) {
	sessionName := w.sessionNameSnapshot()
	w.emitter.Emit(watcherFailedEventName, WatcherFailedEvent{
		SessionName: sessionName,
		Message:     message,
	})
}

func (w *treeWatcher) stopAfterPanic(logMessage string, panicValue any) {
	w.mu.Lock()
	w.stopped = true
	w.degraded = true
	timerStopped := false
	if w.debounce != nil {
		timerStopped = w.debounce.Stop()
		w.debounce = nil
	}
	w.mu.Unlock()

	if timerStopped {
		w.wg.Done()
	}

	if w.watcher != nil {
		if closeErr := w.watcher.Close(); closeErr != nil {
			sessionName := w.sessionNameSnapshot()
			slog.Warn("[DEVPANEL-WATCHER] failed to close watcher during panic recovery",
				"session", sessionName, "error", closeErr)
		}
	}

	sessionName := w.sessionNameSnapshot()
	slog.Error(logMessage,
		"session", sessionName,
		"panic", panicValue,
		"stack", string(debug.Stack()),
	)
	w.emitWatcherFailed("Automatic refresh stopped after a watcher failure. Reload the directory manually.")
}

func (w *treeWatcher) emitWatcherDegraded(message string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}

	w.mu.Lock()
	if w.degraded {
		w.mu.Unlock()
		return
	}
	w.degraded = true
	w.mu.Unlock()

	w.emitWatcherFailed(trimmed)
}

func (w *treeWatcher) Start() {
	w.wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				w.stopAfterPanic("[ERROR-PANIC] treeWatcher.run recovered from panic", r)
			}
		}()
		w.run()
	})
}

func (w *treeWatcher) Stop() error {
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return nil
	}
	w.stopped = true
	timerStopped := false
	if w.debounce != nil {
		timerStopped = w.debounce.Stop()
		w.debounce = nil
	}
	w.mu.Unlock()

	// Stop the debounce timer. If Stop() returns true, the timer
	// goroutine was prevented from running, so we must call wg.Done()
	// to balance the wg.Add(1) done at schedule time. If Stop()
	// returns false, the timer goroutine has already started (or
	// completed), and it will call wg.Done() itself via trackedFlush's
	// defer.
	if timerStopped {
		w.wg.Done()
	}

	closeErr := w.watcher.Close()
	w.wg.Wait()
	return closeErr
}

func (w *treeWatcher) isReusable() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return !w.stopped && !w.degraded
}

func (w *treeWatcher) markDegraded(message string) {
	w.emitWatcherDegraded(message)
}

func isTransientWatcherError(err error) bool {
	return isRetryableFileError(err)
}

func (w *treeWatcher) handleWatcherError(err error) {
	sessionName := w.sessionNameSnapshot()
	if isTransientWatcherError(err) {
		slog.Debug("[DEVPANEL-WATCHER] transient watcher error ignored", "session", sessionName, "error", err)
		return
	}

	slog.Warn("[DEVPANEL-WATCHER] watcher error", "session", sessionName, "error", err)
	// The loop keeps running after degraded mode because fsnotify may resume
	// delivering valid events after a recoverable backend hiccup.
	w.emitWatcherDegraded("Automatic refresh is unavailable after a watcher error. Reload the directory manually.")
}

func (w *treeWatcher) run() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.handleWatcherError(err)
		}
	}
}

func (w *treeWatcher) renameSession(newSessionName string) {
	w.mu.Lock()
	w.sessionName = newSessionName
	w.mu.Unlock()
}

func (w *treeWatcher) sessionNameSnapshot() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.sessionName
}

func (w *treeWatcher) handleEvent(event fsnotify.Event) {
	sessionName := w.sessionNameSnapshot()
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}
	if event.Op&fsnotify.Write != 0 && event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			return
		}
	}

	relPath, ok := w.relativePath(event.Name)
	if !ok || isExcludedWatchPath(relPath) {
		return
	}
	if w.shouldIgnorePath(relPath) {
		return
	}

	// When a watched directory is removed or renamed, fsnotify auto-removes
	// the watch. Decrement the counter to prevent drift toward the limit.
	if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		if _, wasWatched := w.watchedDirs[event.Name]; wasWatched {
			delete(w.watchedDirs, event.Name)
			w.watchedCount--
		}
	}

	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if addErr := w.addRecursive(event.Name); addErr != nil {
				slog.Warn("[DEVPANEL-WATCHER] failed to watch new directory", "session", sessionName, "path", event.Name, "error", addErr)
				w.emitWatcherDegraded("Automatic refresh is partially unavailable because a directory could not be watched. Reload the directory manually if needed.")
			}
		}
	}

	parentPath := parentPanelPath(relPath)
	if w.dirCache != nil {
		w.dirCache.Invalidate(sessionName, relPath)
		w.dirCache.Invalidate(sessionName, parentPath)
	}

	w.queueInvalidation(parentPath)
}

func (w *treeWatcher) addRecursive(root string) error {
	depthLimitLogged := false
	watchLimitLogged := false
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			sessionName := w.sessionNameSnapshot()
			slog.Warn("[DEVPANEL-WATCHER] walk error, skipping",
				"session", sessionName, "path", path, "error", walkErr)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		relPath, ok := w.relativePath(path)
		if ok && isExcludedWatchPath(relPath) {
			return filepath.SkipDir
		}
		if ok && watchPathDepth(relPath) > defaultWatcherMaxDepth {
			if !depthLimitLogged {
				sessionName := w.sessionNameSnapshot()
				slog.Warn(
					"[DEVPANEL-WATCHER] watcher depth limit reached; skipping deeper directories",
					"session", sessionName,
					"path", path,
					"maxDepth", defaultWatcherMaxDepth,
				)
				w.emitWatcherDegraded("Automatic refresh is partially unavailable because the watcher depth limit was reached. Reload the directory manually if needed.")
				depthLimitLogged = true
			}
			return filepath.SkipDir
		}
		if w.watchedCount >= defaultWatcherMaxDirectories {
			if !watchLimitLogged {
				sessionName := w.sessionNameSnapshot()
				slog.Warn(
					"[DEVPANEL-WATCHER] watcher directory limit reached; skipping additional directories",
					"session", sessionName,
					"path", path,
					"maxDirectories", defaultWatcherMaxDirectories,
				)
				w.emitWatcherDegraded("Automatic refresh is partially unavailable because the watcher directory limit was reached. Reload the directory manually if needed.")
				watchLimitLogged = true
			}
			return filepath.SkipDir
		}

		if err := w.watcher.Add(path); err != nil {
			sessionName := w.sessionNameSnapshot()
			slog.Warn("[DEVPANEL-WATCHER] failed to add watch, skipping",
				"session", sessionName,
				"path", path,
				"watchedCount", w.watchedCount,
				"error", err,
			)
			w.emitWatcherDegraded("Automatic refresh is partially unavailable because a directory could not be watched. Reload the directory manually if needed.")
			return filepath.SkipDir
		}
		w.watchedDirs[path] = struct{}{}
		w.watchedCount++
		return nil
	})
}

func (w *treeWatcher) relativePath(path string) (string, bool) {
	relPath, err := filepath.Rel(w.rootDir, path)
	if err != nil {
		slog.Debug("[DEVPANEL-WATCHER] failed to compute relative path", "session", w.sessionNameSnapshot(), "path", path, "error", err)
		return "", false
	}
	return normalizePanelPath(relPath), true
}

func (w *treeWatcher) queueInvalidation(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopped {
		return
	}
	w.pendingPaths[path] = struct{}{}

	// Stop any existing debounce timer before creating a new one.
	// Using Reset() on AfterFunc timers is unsafe: in Go 1.23+ a Reset
	// on an already-fired timer schedules the function to run *again*,
	// causing a second wg.Done() that panics with a negative WaitGroup
	// counter.
	if w.debounce != nil {
		if w.debounce.Stop() {
			// Timer was stopped before its goroutine ran.
			// Balance the wg.Add(1) from the previous schedule.
			w.wg.Done()
		}
		w.debounce = nil
	}
	// Create a fresh timer with a new WaitGroup entry.
	w.wg.Add(1)
	w.debounce = time.AfterFunc(w.debounceInterval, w.trackedFlush)
}

// trackedFlush wraps flush with WaitGroup tracking so that Stop()
// waits for the debounced flush to complete.
func (w *treeWatcher) trackedFlush() {
	defer w.wg.Done()
	w.flush()
}

func (w *treeWatcher) flush() {
	defer func() {
		if r := recover(); r != nil {
			w.stopAfterPanic("[ERROR-PANIC] treeWatcher.flush recovered from panic", r)
		}
	}()

	w.mu.Lock()
	// Mark the timer as fired so the next queueInvalidation creates a new
	// timer with a fresh WaitGroup Add.
	w.debounce = nil
	if w.stopped || len(w.pendingPaths) == 0 {
		w.mu.Unlock()
		return
	}

	paths := make([]string, 0, len(w.pendingPaths))
	sessionName := w.sessionName
	for path := range w.pendingPaths {
		paths = append(paths, path)
	}
	clear(w.pendingPaths)
	w.mu.Unlock()

	slices.Sort(paths)
	w.emitter.Emit(treeInvalidatedEventName, TreeInvalidationEvent{
		SessionName: sessionName,
		Paths:       paths,
	})
}

func isExcludedWatchPath(relPath string) bool {
	normalized := normalizePanelPath(relPath)
	if normalized == "" {
		return false
	}

	for segment := range strings.SplitSeq(normalized, "/") {
		if slices.Contains(excludedDirs, segment) {
			return true
		}
	}
	return false
}

func watchPathDepth(relPath string) int {
	normalized := normalizePanelPath(relPath)
	if normalized == "" {
		return 0
	}

	depth := 0
	for range strings.SplitSeq(normalized, "/") {
		depth++
	}
	return depth
}

func (w *treeWatcher) ignorePaths(paths ...string) {
	now := time.Now()
	until := now.Add(w.ignoreWindow)

	w.mu.Lock()
	defer w.mu.Unlock()

	w.pruneIgnoredPathsLocked(now)
	for _, path := range paths {
		normalized := normalizePanelPath(path)
		if normalized == "" {
			continue
		}
		w.ignoredPaths[normalized] = until
	}
}

func (w *treeWatcher) unignorePaths(paths ...string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, path := range paths {
		normalized := normalizePanelPath(path)
		if normalized != "" {
			delete(w.ignoredPaths, normalized)
		}
	}
}

func (w *treeWatcher) shouldIgnorePath(path string) bool {
	normalized := normalizePanelPath(path)
	if normalized == "" {
		return false
	}
	if strings.HasPrefix(filepath.Base(filepath.FromSlash(normalized)), internalTempFilePrefix) {
		return true
	}

	now := time.Now()

	w.mu.Lock()
	defer w.mu.Unlock()

	w.pruneIgnoredPathsLocked(now)
	until, ok := w.ignoredPaths[normalized]
	return ok && now.Before(until)
}

func (w *treeWatcher) pruneIgnoredPathsLocked(now time.Time) {
	for path, until := range w.ignoredPaths {
		if !now.Before(until) {
			delete(w.ignoredPaths, path)
		}
	}
}

// StartWatcher starts or references the filesystem watcher for a session.
func (s *Service) StartWatcher(sessionName string) error {
	if s.watcherManager == nil {
		return nil
	}

	trimmedSession := strings.TrimSpace(sessionName)
	if trimmedSession == "" {
		return errors.New("session name is required")
	}

	rootDir, err := s.resolveSessionWorkDir(trimmedSession)
	if err != nil {
		return err
	}

	return s.watcherManager.start(trimmedSession, rootDir)
}

// StopWatcher releases one watcher reference for a session.
func (s *Service) StopWatcher(sessionName string) error {
	if s.watcherManager == nil {
		return nil
	}

	trimmedSession := strings.TrimSpace(sessionName)
	if trimmedSession == "" {
		// Frontend cleanup may run before a session becomes available; treat that
		// path as an intentional no-op so React effects can always dispose safely.
		return nil
	}
	return s.watcherManager.stop(trimmedSession)
}

// CleanupSession stops all watcher activity and clears cached tree data for a session.
func (s *Service) CleanupSession(sessionName string) error {
	trimmedSession := strings.TrimSpace(sessionName)
	if trimmedSession == "" {
		return nil
	}
	if s.watcherManager == nil {
		if s.dirCache != nil {
			s.dirCache.InvalidateAll(trimmedSession)
		}
		return nil
	}
	return s.watcherManager.stopAllForSession(trimmedSession)
}

// RenameSession migrates watcher and cache state to a renamed session.
func (s *Service) RenameSession(oldSessionName, newSessionName string) error {
	trimmedOldSession := strings.TrimSpace(oldSessionName)
	trimmedNewSession := strings.TrimSpace(newSessionName)
	if trimmedOldSession == "" || trimmedNewSession == "" || trimmedOldSession == trimmedNewSession {
		return nil
	}
	if s.watcherManager != nil {
		if err := s.watcherManager.renameSession(trimmedOldSession, trimmedNewSession); err != nil {
			return err
		}
	}
	if s.dirCache != nil {
		s.dirCache.RenameSession(trimmedOldSession, trimmedNewSession)
	}
	return nil
}

// StopAllWatchers stops every active filesystem watcher managed by the service.
func (s *Service) StopAllWatchers() error {
	if s.watcherManager == nil {
		return nil
	}
	return s.watcherManager.stopAll()
}
