package devpanel

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"myT-x/internal/apptypes"
)

const (
	treeInvalidatedEventName       = "devpanel:tree-invalidated"
	defaultWatcherDebounceInterval = 100 * time.Millisecond
	defaultWatcherIgnoreWindow     = 750 * time.Millisecond
	defaultWatcherMaxDepth         = 64
	defaultWatcherMaxDirectories   = 65536
)

// TreeInvalidationEvent notifies the frontend that one or more loaded
// directories should be refreshed for a session.
type TreeInvalidationEvent struct {
	SessionName string   `json:"session_name"`
	Paths       []string `json:"paths"`
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

type sessionWatcher struct {
	refCount int
	rootDir  string
	watcher  *treeWatcher
}

type pendingWatcherStart struct {
	done chan struct{}
}

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
		)

		m.mu.Lock()
		if pending = m.starting[sessionName]; pending != nil {
			pendingDone = pending.done
			m.mu.Unlock()
			<-pendingDone
			continue
		}
		if existing = m.watchers[sessionName]; existing != nil {
			if existing.rootDir == rootDir {
				existing.refCount++
				m.mu.Unlock()
				return nil
			}
			delete(m.watchers, sessionName)
			shouldRotate = true
		}
		pending = &pendingWatcherStart{done: make(chan struct{})}
		m.starting[sessionName] = pending
		m.mu.Unlock()

		if shouldRotate {
			if stopErr := existing.watcher.Stop(); stopErr != nil {
				m.mu.Lock()
				delete(m.starting, sessionName)
				close(pending.done)
				m.mu.Unlock()
				return stopErr
			}
		}

		watcher, err := newTreeWatcher(
			sessionName,
			rootDir,
			m.dirCache,
			m.emitter,
			m.debounceInterval,
			m.ignoreWindow,
		)
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
			if shouldRotate && m.dirCache != nil {
				m.dirCache.InvalidateAll(sessionName)
			}
		}
		close(pending.done)
		m.mu.Unlock()
		if err != nil {
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
		delete(m.watchers, sessionName)
		m.mu.Unlock()
		return entry.watcher.Stop()
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
		watchers := make([]*treeWatcher, 0, len(m.watchers))
		for sessionName, entry := range m.watchers {
			watchers = append(watchers, entry.watcher)
			delete(m.watchers, sessionName)
		}
		m.mu.Unlock()

		var stopErr error
		for _, watcher := range watchers {
			if err := watcher.Stop(); err != nil && stopErr == nil {
				stopErr = err
			}
		}
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
			delete(m.watchers, sessionName)
		}
		m.mu.Unlock()
		if m.dirCache != nil {
			m.dirCache.InvalidateAll(sessionName)
		}
		if !ok {
			return nil
		}
		return entry.watcher.Stop()
	}
}

func (m *watcherManager) ignorePaths(sessionName string, paths ...string) {
	m.mu.Lock()
	entry := m.watchers[sessionName]
	m.mu.Unlock()
	if entry == nil {
		return
	}
	entry.watcher.ignorePaths(paths...)
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
	pendingPaths map[string]struct{}
	ignoredPaths map[string]time.Time
	debounce     *time.Timer
	stopped      bool
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
	}
	if err := watcher.addRecursive(rootDir); err != nil {
		_ = fsWatcher.Close()
		return nil, err
	}
	return watcher, nil
}

func (w *treeWatcher) Start() {
	w.wg.Go(func() {
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
	if w.debounce != nil {
		w.debounce.Stop()
	}
	w.mu.Unlock()

	closeErr := w.watcher.Close()
	w.wg.Wait()
	return closeErr
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
			slog.Warn("[DEVPANEL-WATCHER] watcher error", "session", w.sessionName, "error", err)
		}
	}
}

func (w *treeWatcher) handleEvent(event fsnotify.Event) {
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

	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if addErr := w.addRecursive(event.Name); addErr != nil {
				slog.Warn("[DEVPANEL-WATCHER] failed to watch new directory", "session", w.sessionName, "path", event.Name, "error", addErr)
			}
		}
	}

	parentPath := parentPanelPath(relPath)
	if w.dirCache != nil {
		w.dirCache.Invalidate(w.sessionName, relPath)
		w.dirCache.Invalidate(w.sessionName, parentPath)
	}

	w.queueInvalidation(parentPath)
}

func (w *treeWatcher) addRecursive(root string) error {
	watchedCount := 0
	depthLimitLogged := false
	watchLimitLogged := false
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
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
				slog.Warn(
					"[DEVPANEL-WATCHER] watcher depth limit reached; skipping deeper directories",
					"session", w.sessionName,
					"path", path,
					"maxDepth", defaultWatcherMaxDepth,
				)
				depthLimitLogged = true
			}
			return filepath.SkipDir
		}
		if watchedCount >= defaultWatcherMaxDirectories {
			if !watchLimitLogged {
				slog.Warn(
					"[DEVPANEL-WATCHER] watcher directory limit reached; skipping additional directories",
					"session", w.sessionName,
					"path", path,
					"maxDirectories", defaultWatcherMaxDirectories,
				)
				watchLimitLogged = true
			}
			return filepath.SkipDir
		}

		if err := w.watcher.Add(path); err != nil {
			return fmt.Errorf(
				"failed to watch %q after %d directories (filesystem watch limit may be exhausted): %w",
				path,
				watchedCount,
				err,
			)
		}
		watchedCount++
		return nil
	})
}

func (w *treeWatcher) relativePath(path string) (string, bool) {
	relPath, err := filepath.Rel(w.rootDir, path)
	if err != nil {
		slog.Debug("[DEVPANEL-WATCHER] failed to compute relative path", "session", w.sessionName, "path", path, "error", err)
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

	if w.debounce == nil {
		w.debounce = time.AfterFunc(w.debounceInterval, w.flush)
		return
	}
	w.debounce.Reset(w.debounceInterval)
}

func (w *treeWatcher) flush() {
	w.mu.Lock()
	if w.stopped || len(w.pendingPaths) == 0 {
		w.mu.Unlock()
		return
	}

	paths := make([]string, 0, len(w.pendingPaths))
	for path := range w.pendingPaths {
		paths = append(paths, path)
	}
	clear(w.pendingPaths)
	w.mu.Unlock()

	slices.Sort(paths)
	w.emitter.Emit(treeInvalidatedEventName, TreeInvalidationEvent{
		SessionName: w.sessionName,
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

func (w *treeWatcher) shouldIgnorePath(path string) bool {
	normalized := normalizePanelPath(path)
	if normalized == "" {
		return false
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

func (s *Service) StopAllWatchers() error {
	if s.watcherManager == nil {
		return nil
	}
	return s.watcherManager.stopAll()
}
