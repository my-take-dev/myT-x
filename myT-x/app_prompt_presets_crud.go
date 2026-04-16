package main

// SavePromptPreset saves or updates a prompt preset definition.
func (a *App) SavePromptPreset(preset PromptPreset, sessionName string) error {
	return a.promptPresetsService.Save(preset, sessionName)
}

// LoadPromptPresets loads global and project-local prompt presets.
func (a *App) LoadPromptPresets(sessionName string) (PromptPresetLoadResult, error) {
	return a.promptPresetsService.Load(sessionName)
}

// DeletePromptPreset deletes a prompt preset definition.
func (a *App) DeletePromptPreset(presetID string, storageLocation string, sessionName string) error {
	return a.promptPresetsService.Delete(presetID, storageLocation, sessionName)
}

// ReorderPromptPresets reorders prompt preset definitions.
func (a *App) ReorderPromptPresets(presetIDs []string, storageLocation string, sessionName string) error {
	return a.promptPresetsService.Reorder(presetIDs, storageLocation, sessionName)
}
