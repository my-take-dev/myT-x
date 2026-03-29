package main

import (
	"myT-x/internal/scheduler"
)

// SchedulerEntryStatus is the frontend-safe representation of a scheduler.
type SchedulerEntryStatus = scheduler.EntryStatus

// SchedulerTemplate is a reusable scheduler preset.
type SchedulerTemplate = scheduler.Template

// StartScheduler creates and starts a new pane scheduler.
// Wails-bound: called from the frontend scheduler panel.
// Returns the UUID of the new scheduler entry.
func (a *App) StartScheduler(title, paneID, message string, intervalSeconds, maxCount int) (string, error) {
	return a.schedulerService.Start(title, paneID, message, intervalSeconds, maxCount)
}

// ResumeScheduler restarts a stopped scheduler with its saved configuration.
// Wails-bound: called from the frontend scheduler panel.
func (a *App) ResumeScheduler(id string) error {
	return a.schedulerService.Resume(id)
}

// StopScheduler stops the scheduler with the given ID and keeps it in the list.
// Wails-bound: called from the frontend scheduler panel.
func (a *App) StopScheduler(id string) error {
	return a.schedulerService.Stop(id)
}

// DeleteScheduler removes a scheduler from the list entirely.
// Wails-bound: called from the frontend scheduler panel.
func (a *App) DeleteScheduler(id string) error {
	return a.schedulerService.Delete(id)
}

// StopAllSchedulers stops all running schedulers and keeps their entries visible.
// Wails-bound: called during shutdown and from the frontend.
func (a *App) StopAllSchedulers() error {
	return a.schedulerService.StopAll()
}

// GetSchedulerStatuses returns the status of all scheduler entries.
// Wails-bound: called from the frontend scheduler panel.
func (a *App) GetSchedulerStatuses() []SchedulerEntryStatus {
	return a.schedulerService.Statuses()
}

// SaveSchedulerTemplate saves a template (overwrites if Title matches).
// Wails-bound: called from the frontend scheduler panel.
func (a *App) SaveSchedulerTemplate(sessionName string, tmpl SchedulerTemplate) error {
	return a.schedulerService.SaveTemplate(sessionName, tmpl)
}

// LoadSchedulerTemplates returns all templates for the given session.
// Wails-bound: called from the frontend scheduler panel.
func (a *App) LoadSchedulerTemplates(sessionName string) ([]SchedulerTemplate, error) {
	return a.schedulerService.LoadTemplates(sessionName)
}

// DeleteSchedulerTemplate removes a template by Title.
// Wails-bound: called from the frontend scheduler panel.
func (a *App) DeleteSchedulerTemplate(sessionName, title string) error {
	return a.schedulerService.DeleteTemplate(sessionName, title)
}
