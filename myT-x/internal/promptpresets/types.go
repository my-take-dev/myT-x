package promptpresets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	StorageLocationGlobal  = "global"
	StorageLocationProject = "project"
	presetsFileName        = "prompt-presets.json"
	MaxPresets             = 200
)

type PromptPreset struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Body            string `json:"body"`
	Order           int    `json:"order"`
	StorageLocation string `json:"storage_location,omitempty"`
}

type LoadResult struct {
	Presets  []PromptPreset `json:"presets"`
	Warnings []string       `json:"warnings,omitempty"`
}

func (p *PromptPreset) Normalize() {
	if p == nil {
		return
	}

	p.ID = strings.TrimSpace(p.ID)
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Order < 0 {
		p.Order = 0
	}
}

func (p *PromptPreset) Validate() error {
	if p == nil {
		return errors.New("prompt preset is required")
	}
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("prompt preset id is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("prompt preset name is required")
	}
	if strings.TrimSpace(p.Body) == "" {
		return errors.New("prompt preset body is required")
	}
	if p.Order < 0 {
		return fmt.Errorf("prompt preset order must be non-negative")
	}
	return nil
}
