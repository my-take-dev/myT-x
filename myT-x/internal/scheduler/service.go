package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"myT-x/internal/apptypes"
	"myT-x/internal/workerutil"

	"github.com/google/uuid"
)

// Deps holds external dependencies injected at construction time.
// All function fields except Emitter, IsShuttingDown, and IsPaneQuiet must be non-nil.
// NewService panics if any required function field is nil.
//
// Optional fields:
//   - Emitter: defaults to a no-op emitter if nil.
//   - IsShuttingDown: defaults to func() bool { return false } if nil.
//   - IsPaneQuiet: defaults to func(string) bool { return true } if nil.
type Deps struct {
	// Emitter sends runtime events to the frontend.
	// Optional: defaults to a no-op emitter if nil.
	Emitter apptypes.RuntimeEventEmitter

	// IsShuttingDown returns true when the application is tearing down.
	// Optional: defaults to func() bool { return false } if nil.
	IsShuttingDown func() bool

	// IsPaneQuiet returns true when the target pane has had no terminal output
	// recently. The scheduler defers message delivery while the pane is busy
	// to avoid interrupting ongoing AI conversations.
	// Optional: defaults to func(string) bool { return true } (always quiet) if nil.
	IsPaneQuiet func(paneID string) bool

	// CheckPaneAlive returns nil if the pane exists, or an error describing
	// why it is unavailable (session manager down, pane not found, etc.).
	CheckPaneAlive func(paneID string) error

	// SendMessage delivers a text message to the target pane with Enter key.
	// Abstracts the tmux send-keys command.
	SendMessage func(paneID, message string) error

	// ResolveSessionRootPath returns the filesystem root path for the
	// named session. Returns error if the session does not exist or has
	// no root path.
	ResolveSessionRootPath func(sessionName string) (string, error)

	// NewContext creates a cancellable context derived from the app
	// runtime context for a new scheduler worker.
	NewContext func() (context.Context, context.CancelFunc)

	// LaunchWorker starts a background goroutine with panic recovery
	// and WaitGroup tracking.
	LaunchWorker func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions)

	// BaseRecoveryOptions returns the default RecoveryOptions for
	// background workers (with app-level OnPanic/OnFatal/IsShutdown).
	BaseRecoveryOptions func() workerutil.RecoveryOptions
}

// Service manages pane scheduler entries and template persistence.
//
// Thread-safety is managed internally via mu (entries) and templateMu
// (template file I/O). No external locking is required.
type Service struct {
	deps       Deps
	mu         sync.Mutex
	entries    map[string]*entry
	templateMu sync.Mutex
}

// NewService creates a scheduler service with the given dependencies.
// Panics if any required function field in deps is nil.
func NewService(deps Deps) *Service {
	if deps.CheckPaneAlive == nil || deps.SendMessage == nil ||
		deps.ResolveSessionRootPath == nil || deps.NewContext == nil ||
		deps.LaunchWorker == nil || deps.BaseRecoveryOptions == nil {
		panic("scheduler.NewService: required function fields in Deps must be non-nil " +
			"(CheckPaneAlive, SendMessage, ResolveSessionRootPath, NewContext, LaunchWorker, BaseRecoveryOptions)")
	}
	if deps.IsShuttingDown == nil {
		deps.IsShuttingDown = func() bool { return false }
	}
	if deps.IsPaneQuiet == nil {
		deps.IsPaneQuiet = func(string) bool { return true }
	}
	if deps.Emitter == nil {
		deps.Emitter = apptypes.NoopEmitter{}
	}
	return &Service{
		deps:    deps,
		entries: map[string]*entry{},
	}
}

// ------------------------------------------------------------
// Entry lifecycle
// ------------------------------------------------------------

// Start creates and starts a new pane scheduler.
// Returns the UUID of the new scheduler entry.
func (s *Service) Start(title, paneID, message string, intervalSeconds, maxCount int) (string, error) {
	template, normalizedPaneID, err := s.validateStartRequest(title, paneID, message, intervalSeconds, maxCount)
	if err != nil {
		return "", err
	}

	id := uuid.New().String()
	ctx, cancel := s.deps.NewContext()

	e := &entry{
		ID:              id,
		Title:           template.Title,
		PaneID:          normalizedPaneID,
		Message:         template.Message,
		IntervalSeconds: template.IntervalSeconds,
		MaxCount:        template.MaxCount,
		CurrentCount:    0,
		Running:         true,
		RunToken:        1,
		cancel:          cancel,
	}

	s.mu.Lock()
	s.entries[id] = e
	s.mu.Unlock()

	s.launchWorker(id, e.RunToken, ctx)

	slog.Info("[SCHEDULER] started",
		"id", id,
		"title", template.Title,
		"paneID", normalizedPaneID,
		"intervalSeconds", template.IntervalSeconds,
		"maxCount", template.MaxCount,
	)

	s.emitUpdated()
	return id, nil
}

// Resume restarts a stopped scheduler with its saved configuration.
func (s *Service) Resume(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("scheduler id is required")
	}

	s.mu.Lock()
	e, ok := s.entries[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("scheduler %s not found", id)
	}
	if e.Running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler %s is already running", id)
	}

	template := Template{
		Title:           e.Title,
		Message:         e.Message,
		IntervalSeconds: e.IntervalSeconds,
		MaxCount:        e.MaxCount,
	}
	paneID := e.PaneID
	s.mu.Unlock()

	if _, _, err := s.validateStartRequest(template.Title, paneID, template.Message, template.IntervalSeconds, template.MaxCount); err != nil {
		return err
	}

	ctx, cancel := s.deps.NewContext()

	// TOCTOU note: the entry may have been removed or modified between the
	// unlock above and the re-lock below. The re-check with ok guards against
	// the entry disappearing; if it was concurrently restarted, the RunToken
	// mechanism ensures only one runLoop instance is active.
	s.mu.Lock()
	e, ok = s.entries[id]
	if !ok {
		s.mu.Unlock()
		cancel()
		return fmt.Errorf("scheduler %s not found", id)
	}
	e.CurrentCount = 0
	e.Running = true
	e.StopReason = ""
	e.RunToken++
	e.cancel = cancel
	runToken := e.RunToken
	title := e.Title
	s.mu.Unlock()

	s.launchWorker(id, runToken, ctx)

	slog.Info("[SCHEDULER] resumed", "id", id, "title", title)
	s.emitUpdated()
	return nil
}

// Stop stops the scheduler with the given ID and keeps it in the list.
func (s *Service) Stop(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("scheduler id is required")
	}

	e := s.stopEntry(id, 0, "stopped")
	if e == nil {
		return fmt.Errorf("scheduler %s not found", id)
	}

	slog.Info("[SCHEDULER] stopped", "id", id, "title", e.Title)
	s.emitUpdated()
	return nil
}

// Delete removes a scheduler from the list entirely.
func (s *Service) Delete(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("scheduler id is required")
	}

	e := s.removeEntry(id)
	if e == nil {
		return fmt.Errorf("scheduler %s not found", id)
	}

	slog.Info("[SCHEDULER] deleted", "id", id, "title", e.Title)
	s.emitUpdated()
	return nil
}

// StopAll stops all running schedulers and keeps their entries visible.
// Unlike stopWithReason, individual scheduler:stopped events are NOT emitted
// because StopAll is a user-initiated bulk action. Only scheduler:updated is
// emitted once after all entries are stopped.
func (s *Service) StopAll() error {
	type stoppedInfo struct {
		e      *entry
		cancel context.CancelFunc
	}

	s.mu.Lock()
	stopped := make([]stoppedInfo, 0, len(s.entries))
	for _, e := range s.entries {
		if !e.Running {
			continue
		}
		cancel := e.cancel
		e.cancel = nil
		e.Running = false
		e.StopReason = "stopped"
		stopped = append(stopped, stoppedInfo{e: e, cancel: cancel})
	}
	s.mu.Unlock()

	for _, si := range stopped {
		if si.cancel != nil {
			si.cancel()
		}
		slog.Info("[SCHEDULER] stopped (bulk)", "id", si.e.ID, "title", si.e.Title)
	}

	if len(stopped) > 0 {
		s.emitUpdated()
	}
	return nil
}

// Statuses returns the status of all scheduler entries, sorted by ID.
func (s *Service) Statuses() []EntryStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]EntryStatus, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, EntryStatus{
			ID:              e.ID,
			Title:           e.Title,
			PaneID:          e.PaneID,
			Message:         e.Message,
			IntervalSeconds: e.IntervalSeconds,
			MaxCount:        e.MaxCount,
			CurrentCount:    e.CurrentCount,
			Running:         e.Running,
			StopReason:      e.StopReason,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ------------------------------------------------------------
// Internal entry management
// ------------------------------------------------------------

func (s *Service) validateStartRequest(title, paneID, message string, intervalSeconds, maxCount int) (Template, string, error) {
	paneID = strings.TrimSpace(paneID)
	template := Template{
		Title:           title,
		Message:         message,
		IntervalSeconds: intervalSeconds,
		MaxCount:        maxCount,
	}
	template.Normalize()
	if err := template.Validate(); err != nil {
		return Template{}, "", err
	}
	if paneID == "" {
		return Template{}, "", errors.New("pane id is required")
	}
	if err := s.deps.CheckPaneAlive(paneID); err != nil {
		return Template{}, "", err
	}
	return template, paneID, nil
}

func (s *Service) launchWorker(id string, runToken runGeneration, ctx context.Context) {
	entryID := id
	recoveryOpts := s.deps.BaseRecoveryOptions()
	recoveryOpts.MaxRetries = 1 // No retry on panic; keep entry in stopped state.
	origOnFatal := recoveryOpts.OnFatal
	recoveryOpts.OnFatal = func(worker string, maxRetries int) {
		stoppedEntry := s.stopEntry(entryID, runToken, "internal panic")
		if stoppedEntry != nil {
			s.emitStopped(stoppedEntry.ID, stoppedEntry.Title, "internal panic")
			s.emitUpdated()
		}
		if origOnFatal != nil {
			origOnFatal(worker, maxRetries)
		}
	}
	s.deps.LaunchWorker("scheduler-"+id, ctx, func(ctx context.Context) {
		s.runLoop(ctx, entryID, runToken)
	}, recoveryOpts)
}

// runLoop is the goroutine body for a single scheduler entry.
func (s *Service) runLoop(ctx context.Context, entryID string, runToken runGeneration) {
	var consecutiveSkips int
	for {
		// Read entry config under lock.
		s.mu.Lock()
		e, ok := s.entries[entryID]
		if !ok || !e.Running || e.RunToken != runToken {
			s.mu.Unlock()
			return
		}
		interval := time.Duration(e.IntervalSeconds) * time.Second
		paneID := e.PaneID
		message := e.Message
		maxCount := e.MaxCount
		currentCount := e.CurrentCount
		s.mu.Unlock()

		// Wait for the configured interval (context-aware).
		// Note: IntervalSeconds=0 produces a zero-duration timer that fires
		// immediately. This is used by tests to exercise runLoop without delays;
		// Validate() enforces IntervalSeconds >= 10 in production paths.
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		// Check if this entry still exists (may have been removed).
		s.mu.Lock()
		e, exists := s.entries[entryID]
		if !exists || !e.Running || e.RunToken != runToken {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		// Check pane is alive before sending.
		if err := s.deps.CheckPaneAlive(paneID); err != nil {
			slog.Info("[SCHEDULER] pane gone, stopping",
				"id", entryID, "paneID", paneID, "err", err)
			s.stopWithReason(entryID, "target pane is no longer available")
			return
		}

		// Skip if pane is actively receiving output (e.g. AI generating a response).
		// The message will be sent on the next interval when the pane becomes quiet.
		if !s.deps.IsPaneQuiet(paneID) {
			consecutiveSkips++
			if consecutiveSkips >= 5 {
				slog.Warn("[SCHEDULER] pane still busy after multiple intervals",
					"id", entryID, "paneID", paneID, "consecutiveSkips", consecutiveSkips)
			} else {
				slog.Info("[SCHEDULER] pane is busy (output active), deferring to next interval",
					"id", entryID, "paneID", paneID, "consecutiveSkips", consecutiveSkips)
			}
			continue
		}
		consecutiveSkips = 0

		// Send the message.
		if sendErr := s.deps.SendMessage(paneID, message); sendErr != nil {
			slog.Warn("[SCHEDULER] send failed, stopping",
				"id", entryID, "paneID", paneID, "err", sendErr)
			s.stopWithReason(entryID, fmt.Sprintf("message delivery failed: %v", sendErr))
			return
		}

		// Update count.
		currentCount++
		s.mu.Lock()
		e, ok = s.entries[entryID]
		if !ok || !e.Running || e.RunToken != runToken {
			s.mu.Unlock()
			return
		}
		e.CurrentCount = currentCount
		s.mu.Unlock()

		slog.Debug("[DEBUG-SCHEDULER] sent",
			"id", entryID, "paneID", paneID,
			"count", currentCount, "maxCount", maxCount)

		s.emitUpdated()

		// Check if max count reached (0 = infinite).
		if maxCount != InfiniteCount && currentCount >= maxCount {
			slog.Info("[SCHEDULER] completed",
				"id", entryID, "totalSent", currentCount)
			if stoppedEntry := s.stopEntry(entryID, runToken, "completed"); stoppedEntry != nil {
				s.emitUpdated()
			}
			return
		}
	}
}

func (s *Service) removeEntry(id string) *entry {
	s.mu.Lock()
	e, ok := s.entries[id]
	var cancel context.CancelFunc
	if ok {
		cancel = e.cancel
		e.cancel = nil
		delete(s.entries, id)
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if !ok {
		return nil
	}
	return e
}

func (s *Service) stopEntry(id string, runToken runGeneration, reason string) *entry {
	s.mu.Lock()
	e, ok := s.entries[id]
	if !ok {
		s.mu.Unlock()
		return nil
	}
	if runToken != 0 && e.RunToken != runToken {
		s.mu.Unlock()
		return nil
	}
	cancel := e.cancel
	e.cancel = nil
	e.Running = false
	e.StopReason = reason
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return e
}

func (s *Service) stopWithReason(id, reason string) {
	e := s.stopEntry(id, 0, reason)
	if e == nil {
		return
	}
	s.emitStopped(e.ID, e.Title, reason)
	s.emitUpdated()
}

// ------------------------------------------------------------
// Event emission
// ------------------------------------------------------------

func (s *Service) emitUpdated() {
	statuses := s.Statuses()
	s.deps.Emitter.Emit("scheduler:updated", statuses)
}

func (s *Service) emitStopped(id, title, reason string) {
	s.deps.Emitter.Emit("scheduler:stopped", map[string]string{
		"id":     id,
		"title":  title,
		"reason": reason,
	})
}

// ------------------------------------------------------------
// Template persistence
// ------------------------------------------------------------

// SaveTemplate saves a template (overwrites if Title matches).
func (s *Service) SaveTemplate(sessionName string, tmpl Template) error {
	sessionName = strings.TrimSpace(sessionName)
	tmpl.Normalize()
	if err := tmpl.Validate(); err != nil {
		return err
	}

	path, err := s.resolveTemplatePath(sessionName)
	if err != nil {
		return err
	}

	s.templateMu.Lock()
	defer s.templateMu.Unlock()

	templates, err := readTemplatesForWrite(path)
	if err != nil {
		return fmt.Errorf("read templates: %w", err)
	}

	// Upsert: overwrite if Title matches, otherwise append.
	found := false
	for i, t := range templates {
		if t.Title == tmpl.Title {
			templates[i] = tmpl
			found = true
			break
		}
	}
	if !found {
		templates = append(templates, tmpl)
	}

	// Sort by Title for deterministic ordering.
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Title < templates[j].Title
	})

	return writeTemplates(path, templates)
}

// LoadTemplates returns all templates for the given session.
func (s *Service) LoadTemplates(sessionName string) ([]Template, error) {
	sessionName = strings.TrimSpace(sessionName)
	path, err := s.resolveTemplatePath(sessionName)
	if err != nil {
		return []Template{}, err
	}

	s.templateMu.Lock()
	defer s.templateMu.Unlock()

	templates, err := readTemplates(path)
	if err != nil {
		return []Template{}, fmt.Errorf("read templates: %w", err)
	}
	return templates, nil
}

// DeleteTemplate removes a template by Title.
// Returns an error if the title does not exist.
func (s *Service) DeleteTemplate(sessionName, title string) error {
	sessionName = strings.TrimSpace(sessionName)
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("title is required")
	}

	path, err := s.resolveTemplatePath(sessionName)
	if err != nil {
		return err
	}

	s.templateMu.Lock()
	defer s.templateMu.Unlock()

	templates, err := readTemplatesForWrite(path)
	if err != nil {
		return fmt.Errorf("read templates: %w", err)
	}

	filtered := make([]Template, 0, len(templates))
	found := false
	for _, t := range templates {
		if t.Title == title {
			found = true
			continue
		}
		filtered = append(filtered, t)
	}
	if !found {
		return fmt.Errorf("template %q not found", title)
	}

	return writeTemplates(path, filtered)
}

// resolveTemplatePath returns the file path for scheduler templates
// in the session's root directory.
func (s *Service) resolveTemplatePath(sessionName string) (string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return "", errors.New("session name is required")
	}
	rootPath, err := s.deps.ResolveSessionRootPath(sessionName)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, templateDir, templateFileName), nil
}

// readTemplates reads templates from file.
// Returns an empty slice if the file does not exist or is malformed.
func readTemplates(path string) ([]Template, error) {
	return readTemplatesWithMode(path, true)
}

func readTemplatesForWrite(path string) ([]Template, error) {
	return readTemplatesWithMode(path, false)
}

func readTemplatesWithMode(path string, allowMalformed bool) ([]Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Template{}, nil
		}
		return nil, err
	}

	var templates []Template
	if err := json.Unmarshal(data, &templates); err != nil {
		if allowMalformed {
			slog.Error("[SCHEDULER] failed to parse templates, returning empty",
				"path", path, "err", err)
			return []Template{}, nil
		}
		slog.Warn("[SCHEDULER] failed to parse templates, refusing to overwrite",
			"path", path, "err", err)
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return templates, nil
}

// writeTemplates writes templates to file with indented JSON.
// Uses write-to-temp + rename for atomic write safety (defensive-coding-checklist #60).
func writeTemplates(path string, templates []Template) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal templates: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		// Clean up temp file on rename failure (best-effort).
		os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
