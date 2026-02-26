package mcp

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name        string
		defs        []MCPDefinition
		wantErr     []bool
		errContains []string
	}{
		{
			name: "register single definition",
			defs: []MCPDefinition{
				{ID: "memory", Name: "Memory Server", Command: "memory-cmd"},
			},
			wantErr:     []bool{false},
			errContains: []string{""},
		},
		{
			name: "reject empty ID",
			defs: []MCPDefinition{
				{ID: "", Name: "No ID", Command: "memory-cmd"},
			},
			wantErr:     []bool{true},
			errContains: []string{"mcp definition ID is required"},
		},
		{
			name: "reject whitespace-only ID",
			defs: []MCPDefinition{
				{ID: "   ", Name: "Whitespace", Command: "memory-cmd"},
			},
			wantErr:     []bool{true},
			errContains: []string{"mcp definition ID is required"},
		},
		{
			name: "reject empty name",
			defs: []MCPDefinition{
				{ID: "memory", Name: "   ", Command: "memory-cmd"},
			},
			wantErr:     []bool{true},
			errContains: []string{"mcp definition name is required"},
		},
		{
			name: "reject duplicate ID",
			defs: []MCPDefinition{
				{ID: "memory", Name: "Memory Server", Command: "memory-cmd"},
				{ID: "memory", Name: "Memory Server Duplicate", Command: "memory-cmd"},
			},
			wantErr:     []bool{false, true},
			errContains: []string{"", "already exists"},
		},
		{
			name: "trim ID whitespace",
			defs: []MCPDefinition{
				{ID: "  memory  ", Name: "Memory Server", Command: "memory-cmd"},
			},
			wantErr:     []bool{false},
			errContains: []string{""},
		},
		{
			name: "reject empty command",
			defs: []MCPDefinition{
				{ID: "memory", Name: "Memory Server", Command: "   "},
			},
			wantErr:     []bool{true},
			errContains: []string{"mcp definition command is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			for i, def := range tt.defs {
				err := r.Register(def)
				if (err != nil) != tt.wantErr[i] {
					t.Errorf("Register(%q) error = %v, wantErr %v", def.ID, err, tt.wantErr[i])
				}
				if tt.wantErr[i] && tt.errContains[i] != "" {
					if err == nil || !strings.Contains(err.Error(), tt.errContains[i]) {
						t.Fatalf("Register(%q) error = %v, want substring %q", def.ID, err, tt.errContains[i])
					}
				}
			}

			if tt.name == "trim ID whitespace" {
				if _, ok := r.Get("memory"); !ok {
					t.Fatal("trimmed ID should be retrievable via Get(\"memory\")")
				}
			}
		})
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	def := MCPDefinition{ID: "memory", Name: "Memory Server", Description: "desc", Command: "memory-cmd"}
	if err := r.Register(def); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	tests := []struct {
		name   string
		id     string
		wantOK bool
	}{
		{name: "existing ID", id: "memory", wantOK: true},
		{name: "non-existing ID", id: "unknown", wantOK: false},
		{name: "empty ID", id: "", wantOK: false},
		{name: "whitespace ID", id: "  ", wantOK: false},
		{name: "trimmed match", id: "  memory  ", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := r.Get(tt.id)
			if ok != tt.wantOK {
				t.Errorf("Get(%q) ok = %v, want %v", tt.id, ok, tt.wantOK)
			}
			if ok && got.Name != def.Name {
				t.Errorf("Get(%q) Name = %q, want %q", tt.id, got.Name, def.Name)
			}
		})
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()

	// Empty registry.
	all := r.All()
	if len(all) != 0 {
		t.Errorf("All() on empty registry = %d items, want 0", len(all))
	}

	// Register multiple definitions.
	defs := []MCPDefinition{
		{ID: "zz-last", Name: "ZZ Last", Command: "zz-cmd"},
		{ID: "aa-first", Name: "AA First", Command: "aa-cmd"},
		{ID: "mm-middle", Name: "MM Middle", Command: "mm-cmd"},
	}
	for _, d := range defs {
		if err := r.Register(d); err != nil {
			t.Fatalf("Register(%q) failed: %v", d.ID, err)
		}
	}

	all = r.All()
	if len(all) != 3 {
		t.Fatalf("All() = %d items, want 3", len(all))
	}

	// Verify sorted by ID.
	if all[0].ID != "aa-first" {
		t.Errorf("All()[0].ID = %q, want %q", all[0].ID, "aa-first")
	}
	if all[1].ID != "mm-middle" {
		t.Errorf("All()[1].ID = %q, want %q", all[1].ID, "mm-middle")
	}
	if all[2].ID != "zz-last" {
		t.Errorf("All()[2].ID = %q, want %q", all[2].ID, "zz-last")
	}
}

func TestRegistry_LoadFromConfig(t *testing.T) {
	r := NewRegistry()
	defs := []MCPDefinition{
		{ID: "memory", Name: "Memory Server", Command: "memory-cmd"},
		{ID: "memory", Name: "Duplicate", Command: "memory-cmd"}, // duplicate, should be skipped
		{ID: "browser", Name: "Browser MCP", Command: "browser-cmd"},
	}

	errs := r.LoadFromConfig(defs)
	if len(errs) != 1 {
		t.Fatalf("LoadFromConfig errors = %d, want 1", len(errs))
	}
	if !strings.Contains(errs[0].Error(), "already exists") {
		t.Fatalf("LoadFromConfig error = %q, want duplicate-id message", errs[0])
	}

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("LoadFromConfig: All() = %d items, want 2", len(all))
	}

	// Verify first registration wins.
	mem, ok := r.Get("memory")
	if !ok {
		t.Fatal("LoadFromConfig: Get(memory) not found")
	}
	if mem.Name != "Memory Server" {
		t.Errorf("LoadFromConfig: Get(memory).Name = %q, want %q", mem.Name, "Memory Server")
	}
}

func TestRegistry_Register_DeepCopiesDefinition(t *testing.T) {
	r := NewRegistry()
	original := MCPDefinition{
		ID:      "memory",
		Name:    "Memory",
		Command: "npx",
		Args:    []string{"-y", "@anthropic/memory"},
		DefaultEnv: map[string]string{
			"MEM_DIR": "/tmp/memory",
		},
		ConfigParams: []MCPConfigParam{
			{Key: "path", Label: "Path", DefaultValue: "/tmp/memory"},
		},
	}

	if err := r.Register(original); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	original.Args[0] = "changed"
	original.DefaultEnv["MEM_DIR"] = "/changed"
	original.ConfigParams[0].Label = "Changed"

	got, ok := r.Get("memory")
	if !ok {
		t.Fatal("Get(memory) not found")
	}
	if got.Args[0] != "-y" {
		t.Fatalf("stored Args mutated: got %q, want %q", got.Args[0], "-y")
	}
	if got.DefaultEnv["MEM_DIR"] != "/tmp/memory" {
		t.Fatalf("stored DefaultEnv mutated: got %q, want %q", got.DefaultEnv["MEM_DIR"], "/tmp/memory")
	}
	if got.ConfigParams[0].Label != "Path" {
		t.Fatalf("stored ConfigParams mutated: got %q, want %q", got.ConfigParams[0].Label, "Path")
	}
}

func TestRegistry_GetAndAll_ReturnDetachedCopies(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(MCPDefinition{
		ID:      "memory",
		Name:    "Memory",
		Command: "npx",
		Args:    []string{"-y"},
		DefaultEnv: map[string]string{
			"MEM_DIR": "/tmp/memory",
		},
		ConfigParams: []MCPConfigParam{
			{Key: "path", Label: "Path", DefaultValue: "/tmp/memory"},
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, ok := r.Get("memory")
	if !ok {
		t.Fatal("Get(memory) not found")
	}
	got.Args[0] = "mutated"
	got.DefaultEnv["MEM_DIR"] = "/mutated"
	got.ConfigParams[0].Label = "Mutated"

	refetch, ok := r.Get("memory")
	if !ok {
		t.Fatal("Get(memory) not found on second call")
	}
	if refetch.Args[0] != "-y" {
		t.Fatalf("Get returned shared Args backing array: got %q", refetch.Args[0])
	}
	if refetch.DefaultEnv["MEM_DIR"] != "/tmp/memory" {
		t.Fatalf("Get returned shared map: got %q", refetch.DefaultEnv["MEM_DIR"])
	}
	if refetch.ConfigParams[0].Label != "Path" {
		t.Fatalf("Get returned shared config params: got %q", refetch.ConfigParams[0].Label)
	}

	all := r.All()
	all[0].Args[0] = "mutated-all"
	all[0].DefaultEnv["MEM_DIR"] = "/mutated-all"
	all[0].ConfigParams[0].Label = "Mutated-all"

	refetchAfterAll, ok := r.Get("memory")
	if !ok {
		t.Fatal("Get(memory) not found after All mutation")
	}
	if refetchAfterAll.Args[0] != "-y" {
		t.Fatalf("All returned shared Args backing array: got %q", refetchAfterAll.Args[0])
	}
	if refetchAfterAll.DefaultEnv["MEM_DIR"] != "/tmp/memory" {
		t.Fatalf("All returned shared map: got %q", refetchAfterAll.DefaultEnv["MEM_DIR"])
	}
	if refetchAfterAll.ConfigParams[0].Label != "Path" {
		t.Fatalf("All returned shared config params: got %q", refetchAfterAll.ConfigParams[0].Label)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	const workerCount = 16
	const defsPerWorker = 20

	var wg sync.WaitGroup
	errCh := make(chan error, workerCount*defsPerWorker)
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < defsPerWorker; i++ {
				id := fmt.Sprintf("mcp-%02d-%02d", worker, i)
				def := MCPDefinition{ID: id, Name: "Concurrent", Command: "cmd"}
				if err := r.Register(def); err != nil {
					errCh <- fmt.Errorf("Register(%q): %w", id, err)
					return
				}
				if _, ok := r.Get(id); !ok {
					errCh <- fmt.Errorf("Get(%q): not found", id)
					return
				}
				_ = r.All()
			}
		}(worker)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	all := r.All()
	want := workerCount * defsPerWorker
	if len(all) != want {
		t.Fatalf("All() length = %d, want %d", len(all), want)
	}
}
