package main

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
	"time"

	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
	"myT-x/internal/workerutil"

	"github.com/google/uuid"
)

// ------------------------------------------------------------
// Types
// ------------------------------------------------------------

// schedulerEntry is the internal state of a single scheduler instance.
// Protected by App.schedulerMu.
type schedulerEntry struct {
	ID              string
	Title           string
	PaneID          string
	Message         string
	IntervalMinutes int
	MaxCount        int // 0 = infinite (until manual stop or pane gone)
	CurrentCount    int
	Running         bool
	RunToken        uint64
	StopReason      string
	cancel          context.CancelFunc
}

// SchedulerEntryStatus is the frontend-safe representation of a scheduler.
type SchedulerEntryStatus struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	PaneID          string `json:"pane_id"`
	Message         string `json:"message"`
	IntervalMinutes int    `json:"interval_minutes"`
	MaxCount        int    `json:"max_count"`
	CurrentCount    int    `json:"current_count"`
	Running         bool   `json:"running"`
}

const schedulerInfiniteCount = 0

func (a *App) newSchedulerContext() (context.Context, context.CancelFunc) {
	parentCtx := a.runtimeContext()
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	return context.WithCancel(parentCtx)
}

func (a *App) validateSchedulerStartRequest(title, paneID, message string, intervalMinutes, maxCount int) (SchedulerTemplate, string, error) {
	paneID = strings.TrimSpace(paneID)
	template := SchedulerTemplate{
		Title:           title,
		Message:         message,
		IntervalMinutes: intervalMinutes,
		MaxCount:        maxCount,
	}
	template.Normalize()
	if err := template.Validate(); err != nil {
		return SchedulerTemplate{}, "", err
	}
	if paneID == "" {
		return SchedulerTemplate{}, "", errors.New("pane id is required")
	}
	if err := a.ensureSchedulerPaneExists(paneID); err != nil {
		return SchedulerTemplate{}, "", err
	}
	return template, paneID, nil
}

func (a *App) ensureSchedulerPaneExists(paneID string) error {
	sessions, err := a.requireSessions()
	if err != nil {
		return err
	}
	if !isPaneAlive(sessions, paneID) {
		return fmt.Errorf("pane %s does not exist", paneID)
	}
	return nil
}

func (a *App) launchSchedulerWorker(id string, runToken uint64, ctx context.Context) {
	entryID := id
	recoveryOpts := a.defaultRecoveryOptions()
	recoveryOpts.MaxRetries = 1 // No retry on panic; keep entry in stopped state.
	origOnFatal := recoveryOpts.OnFatal
	recoveryOpts.OnFatal = func(worker string, maxRetries int) {
		stoppedEntry := a.stopSchedulerEntry(entryID, runToken, "internal panic")
		if stoppedEntry != nil {
			a.emitSchedulerStopped(stoppedEntry.ID, stoppedEntry.Title, "internal panic")
			a.emitSchedulerUpdated()
		}
		if origOnFatal != nil {
			origOnFatal(worker, maxRetries)
		}
	}
	workerutil.RunWithPanicRecovery(ctx, "scheduler-"+id, &a.bgWG, func(ctx context.Context) {
		a.runSchedulerLoop(ctx, entryID, runToken)
	}, recoveryOpts)
}

// ------------------------------------------------------------
// Wails-bound API methods
// ------------------------------------------------------------

// StartScheduler creates and starts a new pane scheduler.
// Returns the UUID of the new scheduler entry.
func (a *App) StartScheduler(title, paneID, message string, intervalMinutes, maxCount int) (string, error) {
	template, normalizedPaneID, err := a.validateSchedulerStartRequest(title, paneID, message, intervalMinutes, maxCount)
	if err != nil {
		return "", err
	}

	id := uuid.New().String()
	ctx, cancel := a.newSchedulerContext()

	entry := &schedulerEntry{
		ID:              id,
		Title:           template.Title,
		PaneID:          normalizedPaneID,
		Message:         template.Message,
		IntervalMinutes: template.IntervalMinutes,
		MaxCount:        template.MaxCount,
		CurrentCount:    0,
		Running:         true,
		RunToken:        1,
		cancel:          cancel,
	}

	a.schedulerMu.Lock()
	a.schedulerEntries[id] = entry
	a.schedulerMu.Unlock()

	a.launchSchedulerWorker(id, entry.RunToken, ctx)

	slog.Info("[SCHEDULER] started",
		"id", id,
		"title", template.Title,
		"paneID", normalizedPaneID,
		"intervalMinutes", template.IntervalMinutes,
		"maxCount", template.MaxCount,
	)

	a.emitSchedulerUpdated()
	return id, nil
}

// ResumeScheduler restarts a stopped scheduler with its saved configuration.
func (a *App) ResumeScheduler(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("scheduler id is required")
	}

	a.schedulerMu.Lock()
	entry, ok := a.schedulerEntries[id]
	if !ok {
		a.schedulerMu.Unlock()
		return fmt.Errorf("scheduler %s not found", id)
	}
	if entry.Running {
		a.schedulerMu.Unlock()
		return fmt.Errorf("scheduler %s is already running", id)
	}

	template := SchedulerTemplate{
		Title:           entry.Title,
		Message:         entry.Message,
		IntervalMinutes: entry.IntervalMinutes,
		MaxCount:        entry.MaxCount,
	}
	paneID := entry.PaneID
	a.schedulerMu.Unlock()

	if _, _, err := a.validateSchedulerStartRequest(template.Title, paneID, template.Message, template.IntervalMinutes, template.MaxCount); err != nil {
		return err
	}

	ctx, cancel := a.newSchedulerContext()

	a.schedulerMu.Lock()
	entry, ok = a.schedulerEntries[id]
	if !ok {
		a.schedulerMu.Unlock()
		cancel()
		return fmt.Errorf("scheduler %s not found", id)
	}
	entry.CurrentCount = 0
	entry.Running = true
	entry.StopReason = ""
	entry.RunToken++
	entry.cancel = cancel
	runToken := entry.RunToken
	title := entry.Title
	a.schedulerMu.Unlock()

	a.launchSchedulerWorker(id, runToken, ctx)

	slog.Info("[SCHEDULER] resumed", "id", id, "title", title)
	a.emitSchedulerUpdated()
	return nil
}

// StopScheduler stops the scheduler with the given ID and keeps it in the list.
func (a *App) StopScheduler(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("scheduler id is required")
	}

	entry := a.stopSchedulerEntry(id, 0, "stopped")
	if entry == nil {
		return fmt.Errorf("scheduler %s not found", id)
	}

	slog.Info("[SCHEDULER] stopped", "id", id, "title", entry.Title)
	a.emitSchedulerUpdated()
	return nil
}

// DeleteScheduler removes a scheduler from the list entirely.
func (a *App) DeleteScheduler(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("scheduler id is required")
	}

	entry := a.removeSchedulerEntry(id)
	if entry == nil {
		return fmt.Errorf("scheduler %s not found", id)
	}

	slog.Info("[SCHEDULER] deleted", "id", id, "title", entry.Title)
	a.emitSchedulerUpdated()
	return nil
}

// StopAllSchedulers stops all running schedulers and keeps their entries visible.
func (a *App) StopAllSchedulers() error {
	a.schedulerMu.Lock()
	entries := make([]*schedulerEntry, 0, len(a.schedulerEntries))
	for _, e := range a.schedulerEntries {
		if !e.Running {
			continue
		}
		if e.cancel != nil {
			e.cancel()
			e.cancel = nil
		}
		e.Running = false
		e.StopReason = "stopped"
		entries = append(entries, e)
	}
	a.schedulerMu.Unlock()
	for _, e := range entries {
		slog.Info("[SCHEDULER] stopped (bulk)", "id", e.ID, "title", e.Title)
	}

	if len(entries) > 0 {
		a.emitSchedulerUpdated()
	}
	return nil
}

// GetSchedulerStatuses returns the status of all scheduler entries.
func (a *App) GetSchedulerStatuses() []SchedulerEntryStatus {
	a.schedulerMu.Lock()
	defer a.schedulerMu.Unlock()

	result := make([]SchedulerEntryStatus, 0, len(a.schedulerEntries))
	for _, e := range a.schedulerEntries {
		result = append(result, SchedulerEntryStatus{
			ID:              e.ID,
			Title:           e.Title,
			PaneID:          e.PaneID,
			Message:         e.Message,
			IntervalMinutes: e.IntervalMinutes,
			MaxCount:        e.MaxCount,
			CurrentCount:    e.CurrentCount,
			Running:         e.Running,
		})
	}
	// Sort by ID for deterministic ordering across map iterations.
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ------------------------------------------------------------
// Internal methods
// ------------------------------------------------------------

// runSchedulerLoop is the goroutine body for a single scheduler entry generation.
// Launched via workerutil.RunWithPanicRecovery which manages bgWG tracking.
func (a *App) runSchedulerLoop(ctx context.Context, entryID string, runToken uint64) {
	for {
		// Read entry config under lock.
		a.schedulerMu.Lock()
		entry, ok := a.schedulerEntries[entryID]
		if !ok || !entry.Running || entry.RunToken != runToken {
			a.schedulerMu.Unlock()
			return
		}
		interval := time.Duration(entry.IntervalMinutes) * time.Minute
		paneID := entry.PaneID
		message := entry.Message
		maxCount := entry.MaxCount
		currentCount := entry.CurrentCount
		a.schedulerMu.Unlock()

		// Wait for the configured interval (context-aware).
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		// Check if this entry still exists (may have been removed by StopScheduler).
		a.schedulerMu.Lock()
		entry, exists := a.schedulerEntries[entryID]
		if !exists || !entry.Running || entry.RunToken != runToken {
			a.schedulerMu.Unlock()
			return
		}
		a.schedulerMu.Unlock()

		// Check pane is alive before sending.
		sessions, err := a.requireSessions()
		if err != nil {
			slog.Warn("[SCHEDULER] sessions unavailable, stopping",
				"id", entryID, "err", err)
			a.stopSchedulerWithReason(entryID, "session manager is unavailable")
			return
		}
		if !isPaneAlive(sessions, paneID) {
			slog.Info("[SCHEDULER] pane gone, stopping",
				"id", entryID, "paneID", paneID)
			a.stopSchedulerWithReason(entryID, "target pane is no longer available")
			return
		}

		// Send the message.
		router, routerErr := a.requireRouter()
		if routerErr != nil {
			slog.Warn("[SCHEDULER] router unavailable, stopping",
				"id", entryID, "err", routerErr)
			a.stopSchedulerWithReason(entryID, "command router is unavailable")
			return
		}

		sendErr := schedulerSendMessage(router, paneID, message)
		if sendErr != nil {
			slog.Warn("[SCHEDULER] send failed, stopping",
				"id", entryID, "paneID", paneID, "err", sendErr)
			a.stopSchedulerWithReason(entryID, fmt.Sprintf("message delivery failed: %v", sendErr))
			return
		}

		// Update count.
		currentCount++
		a.schedulerMu.Lock()
		entry, ok = a.schedulerEntries[entryID]
		if !ok || !entry.Running || entry.RunToken != runToken {
			a.schedulerMu.Unlock()
			return
		}
		entry.CurrentCount = currentCount
		a.schedulerMu.Unlock()

		slog.Debug("[DEBUG-SCHEDULER] sent",
			"id", entryID, "paneID", paneID,
			"count", currentCount, "maxCount", maxCount)

		a.emitSchedulerUpdated()

		// Check if max count reached (0 = infinite).
		if maxCount != schedulerInfiniteCount && currentCount >= maxCount {
			slog.Info("[SCHEDULER] completed",
				"id", entryID, "totalSent", currentCount)
			if stoppedEntry := a.stopSchedulerEntry(entryID, runToken, "completed"); stoppedEntry != nil {
				a.emitSchedulerUpdated()
			}
			return
		}
	}
}

// removeSchedulerEntry removes an entry from the map and cancels its active worker if present.
func (a *App) removeSchedulerEntry(id string) *schedulerEntry {
	a.schedulerMu.Lock()
	entry, ok := a.schedulerEntries[id]
	if ok {
		if entry.cancel != nil {
			entry.cancel()
			entry.cancel = nil
		}
		delete(a.schedulerEntries, id)
	}
	a.schedulerMu.Unlock()
	if !ok {
		return nil
	}
	return entry
}

func (a *App) stopSchedulerEntry(id string, runToken uint64, reason string) *schedulerEntry {
	a.schedulerMu.Lock()
	entry, ok := a.schedulerEntries[id]
	if !ok {
		a.schedulerMu.Unlock()
		return nil
	}
	if runToken != 0 && entry.RunToken != runToken {
		a.schedulerMu.Unlock()
		return nil
	}
	if entry.cancel != nil {
		entry.cancel()
		entry.cancel = nil
	}
	entry.Running = false
	entry.StopReason = reason
	a.schedulerMu.Unlock()
	return entry
}

func (a *App) stopSchedulerWithReason(id, reason string) {
	entry := a.stopSchedulerEntry(id, 0, reason)
	if entry == nil {
		return
	}
	a.emitSchedulerStopped(entry.ID, entry.Title, reason)
	a.emitSchedulerUpdated()
}

// emitSchedulerUpdated sends the current scheduler list to the frontend.
func (a *App) emitSchedulerUpdated() {
	statuses := a.GetSchedulerStatuses()
	a.emitRuntimeEvent("scheduler:updated", statuses)
}

func (a *App) emitSchedulerStopped(id, title, reason string) {
	a.emitRuntimeEvent("scheduler:stopped", map[string]string{
		"id":     id,
		"title":  title,
		"reason": reason,
	})
}

// schedulerSendMessage sends a message to a pane via the command router,
// using CRLF mode (-N flag) for ConPTY Enter key compatibility.
func schedulerSendMessage(router *tmux.CommandRouter, paneID, message string) error {
	resp := executeRouterRequestFn(router, ipc.TmuxRequest{
		Command: "send-keys",
		Flags: map[string]any{
			"-t": paneID,
			"-N": true,
		},
		Args: []string{message, "Enter"},
	})
	if resp.ExitCode != 0 {
		return fmt.Errorf("send-keys failed: %s", strings.TrimSpace(resp.Stderr))
	}
	return nil
}

// isPaneAlive checks whether a pane with the given ID exists in any session.
func isPaneAlive(sessions *tmux.SessionManager, paneID string) bool {
	for _, sess := range sessions.Snapshot() {
		for _, win := range sess.Windows {
			for _, pane := range win.Panes {
				if pane.ID == paneID {
					return true
				}
			}
		}
	}
	return false
}

// ------------------------------------------------------------
// Scheduler Template Persistence
// ------------------------------------------------------------

// SchedulerTemplate is a reusable scheduler preset.
// PaneID is not included (specified at start time).
// Title is the unique key (same-name save overwrites).
type SchedulerTemplate struct {
	Title           string `json:"title"`
	Message         string `json:"message"`
	IntervalMinutes int    `json:"interval_minutes"`
	MaxCount        int    `json:"max_count"`
}

func (t *SchedulerTemplate) Normalize() {
	if t == nil {
		return
	}
	t.Title = strings.TrimSpace(t.Title)
}

func (t *SchedulerTemplate) Validate() error {
	if t == nil {
		return errors.New("template is required")
	}
	if strings.TrimSpace(t.Title) == "" {
		return errors.New("title is required")
	}
	if t.Message == "" {
		return errors.New("message is required")
	}
	if t.IntervalMinutes < 1 {
		return errors.New("interval must be at least 1 minute")
	}
	if t.MaxCount < schedulerInfiniteCount {
		return errors.New("send count must be 0 for infinite or at least 1")
	}
	return nil
}

// SaveSchedulerTemplate saves a template (overwrites if Title matches).
func (a *App) SaveSchedulerTemplate(sessionName string, tmpl SchedulerTemplate) error {
	sessionName = strings.TrimSpace(sessionName)
	tmpl.Normalize()
	if err := tmpl.Validate(); err != nil {
		return err
	}

	path, err := a.resolveSchedulerTemplatePath(sessionName)
	if err != nil {
		return err
	}

	a.schedulerTemplateMu.Lock()
	defer a.schedulerTemplateMu.Unlock()

	templates, err := readSchedulerTemplatesForWrite(path)
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

	return writeSchedulerTemplates(path, templates)
}

// LoadSchedulerTemplates returns all templates for the given session.
func (a *App) LoadSchedulerTemplates(sessionName string) ([]SchedulerTemplate, error) {
	sessionName = strings.TrimSpace(sessionName)
	path, err := a.resolveSchedulerTemplatePath(sessionName)
	if err != nil {
		return []SchedulerTemplate{}, err
	}

	a.schedulerTemplateMu.Lock()
	defer a.schedulerTemplateMu.Unlock()

	templates, err := readSchedulerTemplates(path)
	if err != nil {
		return []SchedulerTemplate{}, fmt.Errorf("read templates: %w", err)
	}
	return templates, nil
}

// DeleteSchedulerTemplate removes a template by Title.
func (a *App) DeleteSchedulerTemplate(sessionName, title string) error {
	sessionName = strings.TrimSpace(sessionName)
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("title is required")
	}

	path, err := a.resolveSchedulerTemplatePath(sessionName)
	if err != nil {
		return err
	}

	a.schedulerTemplateMu.Lock()
	defer a.schedulerTemplateMu.Unlock()

	templates, err := readSchedulerTemplatesForWrite(path)
	if err != nil {
		return fmt.Errorf("read templates: %w", err)
	}

	// Filter out the matching title.
	filtered := make([]SchedulerTemplate, 0, len(templates))
	for _, t := range templates {
		if t.Title != title {
			filtered = append(filtered, t)
		}
	}

	return writeSchedulerTemplates(path, filtered)
}

// resolveSchedulerTemplatePath returns the file path for scheduler templates
// in the session's root directory.
func (a *App) resolveSchedulerTemplatePath(sessionName string) (string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return "", errors.New("session name is required")
	}
	sessions, err := a.requireSessions()
	if err != nil {
		return "", err
	}
	for _, snap := range sessions.Snapshot() {
		if snap.Name == sessionName {
			if snap.RootPath == "" {
				return "", errors.New("session has no root path")
			}
			return filepath.Join(snap.RootPath, ".myT-x", "scheduler-templates.json"), nil
		}
	}
	return "", fmt.Errorf("session %s not found", sessionName)
}

// readSchedulerTemplates reads templates from file.
// Returns an empty slice if the file does not exist or is malformed.
func readSchedulerTemplates(path string) ([]SchedulerTemplate, error) {
	return readSchedulerTemplatesWithMode(path, true)
}

func readSchedulerTemplatesForWrite(path string) ([]SchedulerTemplate, error) {
	return readSchedulerTemplatesWithMode(path, false)
}

func readSchedulerTemplatesWithMode(path string, allowMalformed bool) ([]SchedulerTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []SchedulerTemplate{}, nil
		}
		return nil, err
	}

	var templates []SchedulerTemplate
	if err := json.Unmarshal(data, &templates); err != nil {
		if allowMalformed {
			slog.Warn("[SCHEDULER] failed to parse templates, returning empty",
				"path", path, "err", err)
			return []SchedulerTemplate{}, nil
		}
		slog.Warn("[SCHEDULER] failed to parse templates, refusing to overwrite",
			"path", path, "err", err)
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return templates, nil
}

// writeSchedulerTemplates writes templates to file with indented JSON.
func writeSchedulerTemplates(path string, templates []SchedulerTemplate) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal templates: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}
