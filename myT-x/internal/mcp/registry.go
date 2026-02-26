package mcp

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

// Registry holds the set of known MCP definitions.
// Thread-safe for concurrent read/write access.
type Registry struct {
	mu          sync.RWMutex
	definitions map[string]Definition // keyed by ID
}

// NewRegistry creates an empty MCP definition registry.
func NewRegistry() *Registry {
	return &Registry{
		definitions: make(map[string]Definition),
	}
}

// Register adds a definition to the registry.
// Returns an error if a definition with the same ID already exists.
func (r *Registry) Register(def Definition) error {
	id := strings.TrimSpace(def.ID)
	if id == "" {
		return fmt.Errorf("mcp definition ID is required")
	}
	name := strings.TrimSpace(def.Name)
	if name == "" {
		return fmt.Errorf("mcp definition name is required (id=%q)", id)
	}
	command := strings.TrimSpace(def.Command)
	if command == "" {
		return fmt.Errorf("mcp definition command is required (id=%q)", id)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.definitions[id]; exists {
		return fmt.Errorf("mcp definition with ID %q already exists", id)
	}
	def.ID = id
	def.Name = name
	def.Command = command
	r.definitions[id] = cloneDefinition(def)
	return nil
}

// Get returns the definition for the given ID and whether it was found.
func (r *Registry) Get(id string) (Definition, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Definition{}, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	def, ok := r.definitions[id]
	if !ok {
		return Definition{}, false
	}
	return cloneDefinition(def), true
}

// All returns a snapshot of all registered definitions, sorted by ID
// for deterministic ordering.
func (r *Registry) All() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Definition, 0, len(r.definitions))
	for _, def := range r.definitions {
		result = append(result, cloneDefinition(def))
	}
	slices.SortFunc(result, func(a, b Definition) int {
		return strings.Compare(a.ID, b.ID)
	})
	return result
}

// LoadFromConfig registers multiple definitions from a config-sourced list.
// Invalid entries are logged and skipped (non-fatal).
func (r *Registry) LoadFromConfig(defs []Definition) []error {
	var errs []error
	for _, def := range defs {
		if err := r.Register(def); err != nil {
			slog.Warn("[WARN-MCP] LoadFromConfig: skipping invalid definition",
				"id", strings.TrimSpace(def.ID), "error", err)
			errs = append(errs, err)
		}
	}
	return errs
}

func cloneDefinition(def Definition) Definition {
	cloned := def
	if def.Args != nil {
		cloned.Args = append([]string(nil), def.Args...)
	}
	if def.DefaultEnv != nil {
		cloned.DefaultEnv = make(map[string]string, len(def.DefaultEnv))
		for k, v := range def.DefaultEnv {
			cloned.DefaultEnv[k] = v
		}
	}
	cloned.ConfigParams = cloneConfigParams(def.ConfigParams)
	return cloned
}

func cloneConfigParams(src []ConfigParam) []ConfigParam {
	if src == nil {
		return nil
	}
	dst := make([]ConfigParam, len(src))
	copy(dst, src)
	return dst
}
