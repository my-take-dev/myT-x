package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"myT-x/internal/config"
	"myT-x/internal/ipc"
)

// NOTE: This file resets package-level model loader cache globals.
// Do not use t.Parallel() in this file.

func staticModelLoader(agentModel *config.AgentModel) modelConfigLoader {
	return func() (*config.AgentModel, error) {
		return agentModel, nil
	}
}

func resetModelConfigLoadState() {
	modelConfigLoadMu.Lock()
	modelConfigCached = nil
	modelConfigLoaded = false
	modelConfigLoadMu.Unlock()
}

func TestLoadAgentModelConfigReadsDefaultPath(t *testing.T) {
	resetModelConfigLoadState()
	t.Cleanup(resetModelConfigLoadState)

	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")

	configPath := config.DefaultPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	raw := []byte("agent_model:\n  from: claude-opus-4-6\n  to: claude-sonnet-4-5\n")
	if err := os.WriteFile(configPath, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	model, loadErr := loadAgentModelConfig()
	if loadErr != nil {
		t.Fatalf("loadAgentModelConfig() error = %v", loadErr)
	}
	if model == nil {
		t.Fatal("model should not be nil")
	}
	if model.From != "claude-opus-4-6" || model.To != "claude-sonnet-4-5" {
		t.Fatalf("model = %+v, want from/to populated from config", model)
	}
}

func TestLoadAgentModelConfigCachesFirstResult(t *testing.T) {
	resetModelConfigLoadState()
	t.Cleanup(resetModelConfigLoadState)

	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")

	configPath := config.DefaultPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("agent_model:\n  from: a\n  to: b\n"), 0o600); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	first, firstErr := loadAgentModelConfig()
	if firstErr != nil {
		t.Fatalf("first loadAgentModelConfig() error = %v", firstErr)
	}
	if first == nil || first.From != "a" || first.To != "b" {
		t.Fatalf("first load = %+v, want from=a to=b", first)
	}

	if err := os.WriteFile(configPath, []byte("agent_model:\n  from: x\n  to: y\n"), 0o600); err != nil {
		t.Fatalf("overwrite config: %v", err)
	}

	second, secondErr := loadAgentModelConfig()
	if secondErr != nil {
		t.Fatalf("second loadAgentModelConfig() error = %v", secondErr)
	}
	if second == nil || second.From != "a" || second.To != "b" {
		t.Fatalf("second load = %+v, want cached from=a to=b", second)
	}
}

func TestLoadAgentModelConfigRetriesAfterInitialError(t *testing.T) {
	resetModelConfigLoadState()
	t.Cleanup(resetModelConfigLoadState)

	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")

	configPath := config.DefaultPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	// First load fails due to invalid YAML.
	if err := os.WriteFile(configPath, []byte("worktree: [\n"), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}
	first, firstErr := loadAgentModelConfig()
	if firstErr == nil {
		t.Fatal("first loadAgentModelConfig() expected parse error")
	}
	if first != nil {
		t.Fatalf("first load should not return a model when parse fails: %+v", first)
	}

	// After fixing the file, loader should retry and succeed.
	if err := os.WriteFile(configPath, []byte("agent_model:\n  from: a\n  to: b\n"), 0o600); err != nil {
		t.Fatalf("write valid config: %v", err)
	}
	second, secondErr := loadAgentModelConfig()
	if secondErr != nil {
		t.Fatalf("second loadAgentModelConfig() error = %v", secondErr)
	}
	if second == nil {
		t.Fatal("second load should return model after retry")
	}
	if second.From != "a" || second.To != "b" {
		t.Fatalf("second load = %+v, want from=a to=b", second)
	}
}

func TestLoadAgentModelConfigConcurrentCalls(t *testing.T) {
	resetModelConfigLoadState()
	t.Cleanup(resetModelConfigLoadState)

	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")

	configPath := config.DefaultPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("agent_model:\n  from: a\n  to: b\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	const workers = 16
	type result struct {
		model *config.AgentModel
		err   error
	}
	results := make(chan result, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			model, err := loadAgentModelConfig()
			results <- result{model: model, err: err}
		}()
	}
	wg.Wait()
	close(results)

	var first *config.AgentModel
	for res := range results {
		if res.err != nil {
			t.Fatalf("loadAgentModelConfig() error = %v", res.err)
		}
		if res.model == nil {
			t.Fatal("loadAgentModelConfig() returned nil model")
		}
		if res.model.From != "a" || res.model.To != "b" {
			t.Fatalf("model = %+v, want from=a to=b", res.model)
		}
		if first == nil {
			first = res.model
			continue
		}
		if res.model != first {
			t.Fatal("concurrent loads should return the cached model pointer")
		}
	}
}

func TestApplyModelTransformCommandScope(t *testing.T) {
	modelCfg := &config.AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5",
	}

	targetReq := ipc.TmuxRequest{
		Command: "split-window",
		Args:    []string{"--model claude-opus-4-6"},
	}
	changed, err := applyModelTransform(&targetReq, staticModelLoader(modelCfg))
	if err != nil {
		t.Fatalf("applyModelTransform() error = %v", err)
	}
	if !changed {
		t.Fatal("expected targeted command to be transformed")
	}
	if got := targetReq.Args[0]; got != "--model claude-sonnet-4-5" {
		t.Fatalf("transformed args[0] = %q", got)
	}

	nonTargetReq := ipc.TmuxRequest{
		Command: "list-sessions",
		Args:    []string{"--model claude-opus-4-6"},
	}
	changed, err = applyModelTransform(&nonTargetReq, staticModelLoader(modelCfg))
	if err != nil {
		t.Fatalf("applyModelTransform(non-target) error = %v", err)
	}
	if changed {
		t.Fatal("expected non-targeted command to stay unchanged")
	}
	if got := nonTargetReq.Args[0]; got != "--model claude-opus-4-6" {
		t.Fatalf("non-target args changed: %q", got)
	}
}

func TestExtractAgentNameEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		index     int
		wantName  string
		wantFound bool
	}{
		{
			name:      "inline flag with value",
			args:      []string{"--agent-name reviewer"},
			index:     0,
			wantName:  "reviewer",
			wantFound: true,
		},
		{
			name:      "equals style",
			args:      []string{"--agent-name=reviewer"},
			index:     0,
			wantName:  "reviewer",
			wantFound: true,
		},
		{
			name:      "tokenized next argument",
			args:      []string{"--agent-name", " reviewer "},
			index:     0,
			wantName:  "reviewer",
			wantFound: true,
		},
		{
			name:      "missing tokenized value",
			args:      []string{"--agent-name"},
			index:     0,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "next token is another flag",
			args:      []string{"--agent-name", "--model"},
			index:     0,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "inline value starts with dash",
			args:      []string{"--agent-name -model"},
			index:     0,
			wantName:  "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotFound := extractAgentName(tt.args, tt.index)
			if gotFound != tt.wantFound {
				t.Fatalf("extractAgentName(%v, %d) found = %v, want %v", tt.args, tt.index, gotFound, tt.wantFound)
			}
			if gotName != tt.wantName {
				t.Fatalf("extractAgentName(%v, %d) name = %q, want %q", tt.args, tt.index, gotName, tt.wantName)
			}
		})
	}
}

func TestExtractAgentNameAdditionalEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		index     int
		wantName  string
		wantFound bool
	}{
		{
			name:      "empty string",
			args:      []string{""},
			index:     0,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "whitespace-only string",
			args:      []string{"   "},
			index:     0,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "equals with empty value",
			args:      []string{"--agent-name="},
			index:     0,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "equals with empty quoted value",
			args:      []string{"--agent-name=''"},
			index:     0,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "tokenized with empty next arg",
			args:      []string{"--agent-name", ""},
			index:     0,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "tokenized with whitespace-only next arg",
			args:      []string{"--agent-name", "   "},
			index:     0,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "equals with single-quoted value",
			args:      []string{"--agent-name='reviewer'"},
			index:     0,
			wantName:  "reviewer",
			wantFound: true,
		},
		{
			name:      "equals with double-quoted value",
			args:      []string{`--agent-name="reviewer"`},
			index:     0,
			wantName:  "reviewer",
			wantFound: true,
		},
		{
			name:      "trailing --agent-name at end of single-element args",
			args:      []string{"--agent-name"},
			index:     0,
			wantName:  "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotFound := extractAgentName(tt.args, tt.index)
			if gotFound != tt.wantFound {
				t.Fatalf("extractAgentName(%v, %d) found = %v, want %v", tt.args, tt.index, gotFound, tt.wantFound)
			}
			if gotName != tt.wantName {
				t.Fatalf("extractAgentName(%v, %d) name = %q, want %q", tt.args, tt.index, gotName, tt.wantName)
			}
		})
	}
}

func TestApplyModelOverrideEmptyValueGuard(t *testing.T) {
	// Verify that applyModelOverride skips replacement when args[i+1]
	// is empty/whitespace in tokenized --model form.
	modelCfg := &config.AgentModel{
		Overrides: []config.AgentModelOverride{
			{Name: "security", Model: "claude-opus-4-6"},
		},
	}

	tests := []struct {
		name        string
		args        []string
		wantArgs    []string
		wantChanged bool
	}{
		{
			name:        "empty model value after --model",
			args:        []string{"--agent-name", "security-bot", "--model", ""},
			wantArgs:    []string{"--agent-name", "security-bot", "--model", ""},
			wantChanged: false,
		},
		{
			name:        "whitespace model value after --model",
			args:        []string{"--agent-name", "security-bot", "--model", "   "},
			wantArgs:    []string{"--agent-name", "security-bot", "--model", "   "},
			wantChanged: false,
		},
		{
			name:        "valid model value after --model",
			args:        []string{"--agent-name", "security-bot", "--model", "claude-sonnet-4-5"},
			wantArgs:    []string{"--agent-name", "security-bot", "--model", "claude-opus-4-6"},
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.TmuxRequest{
				Command: "send-keys",
				Args:    append([]string(nil), tt.args...),
			}
			changed, err := applyModelTransform(&req, staticModelLoader(modelCfg))
			if err != nil {
				t.Fatalf("applyModelTransform() error = %v", err)
			}
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if !slices.Equal(req.Args, tt.wantArgs) {
				t.Fatalf("args = %v, want %v", req.Args, tt.wantArgs)
			}
		})
	}
}

func TestApplyModelTransformOverrideAndFallback(t *testing.T) {
	modelCfg := &config.AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5",
		Overrides: []config.AgentModelOverride{
			{Name: "security", Model: "claude-opus-4-6"},
			{Name: "reviewer", Model: "claude-haiku-4"},
		},
	}

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "override wins",
			args: []string{`--agent-name security-reviewer --model claude-sonnet-4-5`},
			want: []string{`--agent-name security-reviewer --model claude-opus-4-6`},
		},
		{
			name: "fallback applies when override does not match",
			args: []string{`--agent-name explorer-3 --model claude-opus-4-6`},
			want: []string{`--agent-name explorer-3 --model claude-sonnet-4-5`},
		},
		{
			name: "no model no change",
			args: []string{`--agent-name security-reviewer --agent-type Explore`},
			want: []string{`--agent-name security-reviewer --agent-type Explore`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.TmuxRequest{
				Command: "new-session",
				Args:    append([]string(nil), tt.args...),
			}
			changed, err := applyModelTransform(&req, staticModelLoader(modelCfg))
			if err != nil {
				t.Fatalf("applyModelTransform() error = %v", err)
			}
			if !slices.Equal(req.Args, tt.want) {
				t.Fatalf("args = %#v, want %#v", req.Args, tt.want)
			}
			wantChanged := !slices.Equal(tt.args, tt.want)
			if changed != wantChanged {
				t.Fatalf("changed = %v, want %v", changed, wantChanged)
			}
		})
	}
}

func TestApplyModelTransformTokenizedAndModelEqualsFlags(t *testing.T) {
	modelCfg := &config.AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5",
		Overrides: []config.AgentModelOverride{
			{Name: "security", Model: "claude$sonnet"},
		},
	}

	tokenizedReq := ipc.TmuxRequest{
		Command: "send-keys",
		Args:    []string{"--agent-name", "security-reviewer", "--model", "claude-opus-4-6"},
	}
	changed, err := applyModelTransform(&tokenizedReq, staticModelLoader(modelCfg))
	if err != nil {
		t.Fatalf("applyModelTransform(tokenized) error = %v", err)
	}
	if !changed {
		t.Fatal("expected tokenized command to be transformed")
	}
	if got := tokenizedReq.Args[3]; got != "claude$sonnet" {
		t.Fatalf("tokenized model = %q, want claude$sonnet", got)
	}

	equalsReq := ipc.TmuxRequest{
		Command: "split-window",
		Args:    []string{"--model=claude-opus-4-6"},
	}
	changed, err = applyModelTransform(&equalsReq, staticModelLoader(modelCfg))
	if err != nil {
		t.Fatalf("applyModelTransform(model=) error = %v", err)
	}
	if !changed {
		t.Fatal("expected --model= flag to be transformed")
	}
	if got := equalsReq.Args[0]; got != "--model=claude-sonnet-4-5" {
		t.Fatalf("model= args[0] = %q, want --model=claude-sonnet-4-5", got)
	}
}

func TestApplyModelTransformMultipleModelFlagsInSingleArgument(t *testing.T) {
	modelCfg := &config.AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5",
	}

	req := ipc.TmuxRequest{
		Command: "split-window",
		Args: []string{
			`claude --model claude-opus-4-6 --flag x --model claude-opus-4-6`,
		},
	}

	changed, err := applyModelTransform(&req, staticModelLoader(modelCfg))
	if err != nil {
		t.Fatalf("applyModelTransform() error = %v", err)
	}
	if !changed {
		t.Fatal("expected multiple --model replacements to be applied")
	}
	want := `claude --model claude-sonnet-4-5 --flag x --model claude-sonnet-4-5`
	if got := req.Args[0]; got != want {
		t.Fatalf("args[0] = %q, want %q", got, want)
	}
}

// I-18: Verify that --model=value inline form (within a single argument string)
// is detected and replaced by both anyModelFlagPattern and fromTo pattern.
func TestApplyModelTransformInlineModelEqualsForm(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.AgentModel
		args        []string
		wantArgs    []string
		wantChanged bool
	}{
		{
			name: "fromTo: inline --model=FROM replaced",
			cfg: &config.AgentModel{
				From: "claude-opus-4-6",
				To:   "claude-sonnet-4-5",
			},
			args:        []string{"claude --model=claude-opus-4-6 --flag x"},
			wantArgs:    []string{"claude --model=claude-sonnet-4-5 --flag x"},
			wantChanged: true,
		},
		{
			name: "fromTo: inline --model=OTHER not replaced",
			cfg: &config.AgentModel{
				From: "claude-opus-4-6",
				To:   "claude-sonnet-4-5",
			},
			args:        []string{"claude --model=claude-haiku-4 --flag x"},
			wantArgs:    []string{"claude --model=claude-haiku-4 --flag x"},
			wantChanged: false,
		},
		{
			name: "ALL: inline --model=value replaced",
			cfg: &config.AgentModel{
				From: "ALL",
				To:   "claude-sonnet-4-5",
			},
			args:        []string{"claude --model=claude-opus-4-6 --flag x"},
			wantArgs:    []string{"claude --model=claude-sonnet-4-5 --flag x"},
			wantChanged: true,
		},
		{
			name: "override: inline --model=value replaced via override",
			cfg: &config.AgentModel{
				From: "claude-opus-4-6",
				To:   "claude-sonnet-4-5",
				Overrides: []config.AgentModelOverride{
					{Name: "security", Model: "claude-opus-4-6"},
				},
			},
			args:        []string{"--agent-name security-bot --model=claude-haiku-4"},
			wantArgs:    []string{"--agent-name security-bot --model=claude-opus-4-6"},
			wantChanged: true,
		},
		{
			name: "fromTo: inline --model=FROM at end of string",
			cfg: &config.AgentModel{
				From: "claude-opus-4-6",
				To:   "claude-sonnet-4-5",
			},
			args:        []string{"claude --model=claude-opus-4-6"},
			wantArgs:    []string{"claude --model=claude-sonnet-4-5"},
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.TmuxRequest{
				Command: "send-keys",
				Args:    append([]string(nil), tt.args...),
			}
			changed, err := applyModelTransform(&req, staticModelLoader(tt.cfg))
			if err != nil {
				t.Fatalf("applyModelTransform() error = %v", err)
			}
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if !slices.Equal(req.Args, tt.wantArgs) {
				t.Fatalf("args = %v, want %v", req.Args, tt.wantArgs)
			}
		})
	}
}

func TestApplyModelTransformOverridePriorityByDeclarationOrder(t *testing.T) {
	modelCfg := &config.AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5",
		Overrides: []config.AgentModelOverride{
			{Name: "review", Model: "claude-haiku-4"},
			{Name: "reviewer", Model: "claude-opus-4-6"},
		},
	}

	req := ipc.TmuxRequest{
		Command: "new-session",
		Args:    []string{`--agent-name reviewer --model claude-sonnet-4-5`},
	}

	changed, err := applyModelTransform(&req, staticModelLoader(modelCfg))
	if err != nil {
		t.Fatalf("applyModelTransform() error = %v", err)
	}
	if !changed {
		t.Fatal("expected override replacement")
	}
	if got := req.Args[0]; got != `--agent-name reviewer --model claude-haiku-4` {
		t.Fatalf("args[0] = %q, want first override model", got)
	}
}

func TestApplyModelTransformAcrossArgElements(t *testing.T) {
	modelCfg := &config.AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5",
		Overrides: []config.AgentModelOverride{
			{Name: "security", Model: "claude-opus-4-6"},
		},
	}

	req := ipc.TmuxRequest{
		Command: "split-window",
		Args: []string{
			`--agent-name security-bot --agent-type Explore`,
			`--model claude-sonnet-4-5`,
		},
	}
	changed, err := applyModelTransform(&req, staticModelLoader(modelCfg))
	if err != nil {
		t.Fatalf("applyModelTransform() error = %v", err)
	}
	if !changed {
		t.Fatal("expected override to apply across arg elements")
	}
	if got := req.Args[1]; got != `--model claude-opus-4-6` {
		t.Fatalf("args[1] = %q, want --model claude-opus-4-6", got)
	}
}

func TestApplyModelTransformSkipsOnLoaderError(t *testing.T) {
	// Shim spec: config load failure must not block forwarding.
	// applyModelTransform logs the error via debugLog and returns (false, nil).
	//
	// T-8: Verify that the error is written to the debug log file per CLAUDE.md
	// spec "エラーはログにだけ出力しておくこと".
	logDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", logDir)

	req := ipc.TmuxRequest{
		Command: "split-window",
		Args:    []string{"--model claude-opus-4-6"},
	}
	before := append([]string(nil), req.Args...)

	changed, err := applyModelTransform(&req, func() (*config.AgentModel, error) {
		return nil, errors.New("load failed")
	})
	if err != nil {
		t.Fatalf("expected nil error (shim spec: swallow load errors), got: %v", err)
	}
	if changed {
		t.Fatal("changed should be false on loader error")
	}
	if !slices.Equal(req.Args, before) {
		t.Fatalf("args mutated on loader error: got %#v, want %#v", req.Args, before)
	}

	// Verify the error was written to the debug log file.
	logPath := filepath.Join(logDir, "myT-x", "shim-debug.log")
	logContent, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("debug log file should exist at %s: %v", logPath, readErr)
	}
	if !strings.Contains(string(logContent), "config load failed") {
		t.Fatalf("debug log should contain error message, got: %s", string(logContent))
	}
}

func TestApplyModelTransformNilRequest(t *testing.T) {
	changed, err := applyModelTransform(nil, staticModelLoader(&config.AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5",
	}))
	if changed {
		t.Fatal("nil request should not be transformed")
	}
	if err == nil {
		t.Fatal("applyModelTransform(nil) expected error")
	}
	if err.Error() != "tmux request is nil" {
		t.Fatalf("applyModelTransform(nil) error = %v, want %q", err, "tmux request is nil")
	}
}

func TestApplyModelTransformNilAgentModel(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Args:    []string{"--model claude-opus-4-6"},
	}

	changed, err := applyModelTransform(&req, staticModelLoader(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatal("nil AgentModel should not transform")
	}
}

func TestApplyModelTransformEmptyArgsNoop(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "new-session",
		Args:    []string{},
	}
	changed, err := applyModelTransform(&req, staticModelLoader(&config.AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5",
	}))
	if err != nil {
		t.Fatalf("applyModelTransform() error = %v", err)
	}
	if changed {
		t.Fatal("empty args should stay unchanged")
	}
	if req.Args == nil {
		t.Fatal("args slice should remain non-nil")
	}
}

// resetModelConfigCache is a convenience alias for resetModelConfigLoadState.
// It resets the global model config cache so that subsequent calls to
// loadAgentModelConfig will re-read from disk.
func resetModelConfigCache() {
	resetModelConfigLoadState()
}

func TestResetModelConfigCacheClearsLoadedState(t *testing.T) {
	// Verify that resetModelConfigCache properly resets the cache,
	// allowing loadAgentModelConfig to re-read from disk.
	resetModelConfigCache()
	t.Cleanup(resetModelConfigCache)

	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")

	configPath := config.DefaultPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("agent_model:\n  from: a\n  to: b\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	first, err := loadAgentModelConfig()
	if err != nil {
		t.Fatalf("first load error: %v", err)
	}
	if first == nil || first.From != "a" {
		t.Fatalf("first load = %+v, want from=a", first)
	}

	// Write a different config
	if err := os.WriteFile(configPath, []byte("agent_model:\n  from: x\n  to: y\n"), 0o600); err != nil {
		t.Fatalf("overwrite config: %v", err)
	}

	// Without reset, cache returns stale data
	cached, err := loadAgentModelConfig()
	if err != nil {
		t.Fatalf("cached load error: %v", err)
	}
	if cached.From != "a" {
		t.Fatalf("cached should return stale from=a, got %q", cached.From)
	}

	// After reset, fresh data is loaded
	resetModelConfigCache()
	fresh, err := loadAgentModelConfig()
	if err != nil {
		t.Fatalf("fresh load error: %v", err)
	}
	if fresh.From != "x" || fresh.To != "y" {
		t.Fatalf("fresh load = %+v, want from=x to=y", fresh)
	}
}

func TestApplyModelOverrideModelEqualsEmptyValue(t *testing.T) {
	// I-25: Verify that --model= with empty value after equals is skipped.
	modelCfg := &config.AgentModel{
		Overrides: []config.AgentModelOverride{
			{Name: "security", Model: "claude-opus-4-6"},
		},
	}

	tests := []struct {
		name        string
		args        []string
		wantArgs    []string
		wantChanged bool
	}{
		{
			name:        "--model= empty value (override path)",
			args:        []string{"--agent-name", "security-bot", "--model="},
			wantArgs:    []string{"--agent-name", "security-bot", "--model="},
			wantChanged: false,
		},
		{
			name:        "--model= whitespace value (override path)",
			args:        []string{"--agent-name", "security-bot", "--model=  "},
			wantArgs:    []string{"--agent-name", "security-bot", "--model=  "},
			wantChanged: false,
		},
		{
			name:        "--model= valid value (override path)",
			args:        []string{"--agent-name", "security-bot", "--model=claude-sonnet-4-5"},
			wantArgs:    []string{"--agent-name", "security-bot", "--model=claude-opus-4-6"},
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.TmuxRequest{
				Command: "send-keys",
				Args:    append([]string(nil), tt.args...),
			}
			changed, err := applyModelTransform(&req, staticModelLoader(modelCfg))
			if err != nil {
				t.Fatalf("applyModelTransform() error = %v", err)
			}
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if !slices.Equal(req.Args, tt.wantArgs) {
				t.Fatalf("args = %v, want %v", req.Args, tt.wantArgs)
			}
		})
	}
}

func TestIsAllModelFrom(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ALL", true},
		{"all", true},
		{"All", true},
		{"aLl", true},
		{" ALL ", true},
		{"  all  ", true},
		{"ALLX", false},
		{"XALL", false},
		{"model", false},
		{"", false},
		{" ", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isAllModelFrom(tt.input); got != tt.want {
				t.Fatalf("isAllModelFrom(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestApplyModelTransformAllWildcard(t *testing.T) {
	const targetModel = "claude-sonnet-4-5"

	tests := []struct {
		name        string
		from        string
		overrides   []config.AgentModelOverride
		args        []string
		wantArgs    []string
		wantChanged bool
	}{
		{
			name:        "ALL replaces any model (inline)",
			from:        "ALL",
			args:        []string{"--model claude-opus-4-6"},
			wantArgs:    []string{"--model claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name:        "ALL replaces any model (tokenized)",
			from:        "ALL",
			args:        []string{"--model", "claude-opus-4-6"},
			wantArgs:    []string{"--model", "claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name:        "ALL replaces any model (--model=)",
			from:        "ALL",
			args:        []string{"--model=claude-opus-4-6"},
			wantArgs:    []string{"--model=claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name:        "case insensitivity: all",
			from:        "all",
			args:        []string{"--model claude-opus-4-6"},
			wantArgs:    []string{"--model claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name:        "case insensitivity: All",
			from:        "All",
			args:        []string{"--model claude-opus-4-6"},
			wantArgs:    []string{"--model claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name: "override wins over ALL",
			from: "ALL",
			overrides: []config.AgentModelOverride{
				{Name: "security", Model: "claude-opus-4-6"},
			},
			args:        []string{"--agent-name security-bot --model claude-haiku-4"},
			wantArgs:    []string{"--agent-name security-bot --model claude-opus-4-6"},
			wantChanged: true,
		},
		{
			name: "ALL fallback when override does not match",
			from: "ALL",
			overrides: []config.AgentModelOverride{
				{Name: "security", Model: "claude-opus-4-6"},
			},
			args:        []string{"--agent-name explorer --model claude-haiku-4"},
			wantArgs:    []string{"--agent-name explorer --model claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name:        "ALL with no --model flag",
			from:        "ALL",
			args:        []string{"--agent-name foo --agent-type Explore"},
			wantArgs:    []string{"--agent-name foo --agent-type Explore"},
			wantChanged: false,
		},
		{
			name:        "ALL replaces multiple --model in single arg",
			from:        "ALL",
			args:        []string{"--model claude-opus-4-6 --flag x --model claude-haiku-4"},
			wantArgs:    []string{"--model claude-sonnet-4-5 --flag x --model claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name:        "ALL with empty --model= value skipped",
			from:        "ALL",
			args:        []string{"--model="},
			wantArgs:    []string{"--model="},
			wantChanged: false,
		},
		{
			name:        "ALL with whitespace --model value skipped",
			from:        "ALL",
			args:        []string{"--model", "   "},
			wantArgs:    []string{"--model", "   "},
			wantChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelCfg := &config.AgentModel{
				From:      tt.from,
				To:        targetModel,
				Overrides: tt.overrides,
			}
			req := ipc.TmuxRequest{
				Command: "send-keys",
				Args:    append([]string(nil), tt.args...),
			}
			changed, err := applyModelTransform(&req, staticModelLoader(modelCfg))
			if err != nil {
				t.Fatalf("applyModelTransform() error = %v", err)
			}
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if !slices.Equal(req.Args, tt.wantArgs) {
				t.Fatalf("args = %v, want %v", req.Args, tt.wantArgs)
			}
		})
	}
}

func TestApplyModelTransformConfiguredOverridesFromUserConfig(t *testing.T) {
	modelCfg := &config.AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5-20250929",
		Overrides: []config.AgentModelOverride{
			{Name: "security", Model: "claude-opus-4-6"},
			{Name: "reviewer", Model: "claude-sonnet-4-5-20250929"},
			{Name: "coder", Model: "claude-haiku-4-5-20251001"},
		},
	}

	tests := []struct {
		name      string
		agentName string
		inModel   string
		wantModel string
	}{
		{
			name:      "security override forces opus",
			agentName: "security-checker",
			inModel:   "claude-sonnet-4-5-20250929",
			wantModel: "claude-opus-4-6",
		},
		{
			name:      "reviewer override sets sonnet",
			agentName: "reviewer",
			inModel:   "claude-opus-4-6",
			wantModel: "claude-sonnet-4-5-20250929",
		},
		{
			name:      "coder override sets haiku",
			agentName: "coder",
			inModel:   "claude-opus-4-6",
			wantModel: "claude-haiku-4-5-20251001",
		},
		{
			name:      "fallback uses from-to mapping",
			agentName: "engineer",
			inModel:   "claude-opus-4-6",
			wantModel: "claude-sonnet-4-5-20250929",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.TmuxRequest{
				Command: "send-keys",
				Args: []string{
					`--agent-name ` + tt.agentName + ` --model ` + tt.inModel,
				},
			}

			changed, err := applyModelTransform(&req, staticModelLoader(modelCfg))
			if err != nil {
				t.Fatalf("applyModelTransform() error = %v", err)
			}
			if !changed {
				t.Fatal("expected transformation")
			}
			want := `--agent-name ` + tt.agentName + ` --model ` + tt.wantModel
			if got := req.Args[0]; got != want {
				t.Fatalf("args[0] = %q, want %q", got, want)
			}
		})
	}
}

// I-27: Verify anyModelFlagPattern greedy \S+ handles multiple --model flags correctly.
// The greedy quantifier is correct because \S+ stops at whitespace, ensuring each
// --model occurrence is matched independently by ReplaceAllString.
func TestAnyModelFlagPatternMultipleFlagCoexistence(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.AgentModel
		args        []string
		wantArgs    []string
		wantChanged bool
	}{
		{
			name: "fromTo: two --model space-separated in single arg",
			cfg: &config.AgentModel{
				From: "claude-opus-4-6",
				To:   "claude-sonnet-4-5",
			},
			args:        []string{"--model claude-opus-4-6 --flag x --model claude-opus-4-6"},
			wantArgs:    []string{"--model claude-sonnet-4-5 --flag x --model claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name: "fromTo: two --model= equals-joined in single arg",
			cfg: &config.AgentModel{
				From: "claude-opus-4-6",
				To:   "claude-sonnet-4-5",
			},
			args:        []string{"--model=claude-opus-4-6 --flag x --model=claude-opus-4-6"},
			wantArgs:    []string{"--model=claude-sonnet-4-5 --flag x --model=claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name: "fromTo: mixed space and equals in single arg",
			cfg: &config.AgentModel{
				From: "claude-opus-4-6",
				To:   "claude-sonnet-4-5",
			},
			args:        []string{"--model claude-opus-4-6 --flag x --model=claude-opus-4-6"},
			wantArgs:    []string{"--model claude-sonnet-4-5 --flag x --model=claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name: "fromTo: only first matches from value, second is different model",
			cfg: &config.AgentModel{
				From: "claude-opus-4-6",
				To:   "claude-sonnet-4-5",
			},
			args:        []string{"--model claude-opus-4-6 --flag x --model claude-haiku-4"},
			wantArgs:    []string{"--model claude-sonnet-4-5 --flag x --model claude-haiku-4"},
			wantChanged: true,
		},
		{
			name: "ALL: replaces both different models in single arg",
			cfg: &config.AgentModel{
				From: "ALL",
				To:   "claude-sonnet-4-5",
			},
			args:        []string{"--model claude-opus-4-6 --flag x --model claude-haiku-4"},
			wantArgs:    []string{"--model claude-sonnet-4-5 --flag x --model claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name: "fromTo: multiple flags across separate arg elements",
			cfg: &config.AgentModel{
				From: "claude-opus-4-6",
				To:   "claude-sonnet-4-5",
			},
			args:        []string{"--model claude-opus-4-6", "--model claude-opus-4-6"},
			wantArgs:    []string{"--model claude-sonnet-4-5", "--model claude-sonnet-4-5"},
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.TmuxRequest{
				Command: "send-keys",
				Args:    append([]string(nil), tt.args...),
			}
			changed, err := applyModelTransform(&req, staticModelLoader(tt.cfg))
			if err != nil {
				t.Fatalf("applyModelTransform() error = %v", err)
			}
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if !slices.Equal(req.Args, tt.wantArgs) {
				t.Fatalf("args = %v, want %v", req.Args, tt.wantArgs)
			}
		})
	}
}

// I-28: Verify applyFromToReplacement empty value guards are symmetric with applyModelOverride.
func TestApplyFromToReplacementEmptyValueGuard(t *testing.T) {
	modelCfg := &config.AgentModel{
		From: "claude-opus-4-6",
		To:   "claude-sonnet-4-5",
	}

	tests := []struct {
		name        string
		args        []string
		wantArgs    []string
		wantChanged bool
	}{
		{
			name:        "tokenized --model with empty next arg",
			args:        []string{"--model", ""},
			wantArgs:    []string{"--model", ""},
			wantChanged: false,
		},
		{
			name:        "tokenized --model with whitespace next arg",
			args:        []string{"--model", "   "},
			wantArgs:    []string{"--model", "   "},
			wantChanged: false,
		},
		{
			name:        "--model= with empty value",
			args:        []string{"--model="},
			wantArgs:    []string{"--model="},
			wantChanged: false,
		},
		{
			name:        "--model= with whitespace value",
			args:        []string{"--model=  "},
			wantArgs:    []string{"--model=  "},
			wantChanged: false,
		},
		{
			name:        "tokenized --model with matching value replaces",
			args:        []string{"--model", "claude-opus-4-6"},
			wantArgs:    []string{"--model", "claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name:        "--model= with matching value replaces",
			args:        []string{"--model=claude-opus-4-6"},
			wantArgs:    []string{"--model=claude-sonnet-4-5"},
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ipc.TmuxRequest{
				Command: "send-keys",
				Args:    append([]string(nil), tt.args...),
			}
			changed, err := applyModelTransform(&req, staticModelLoader(modelCfg))
			if err != nil {
				t.Fatalf("applyModelTransform() error = %v", err)
			}
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if !slices.Equal(req.Args, tt.wantArgs) {
				t.Fatalf("args = %v, want %v", req.Args, tt.wantArgs)
			}
		})
	}
}

// S-23: Exercise the splitModelEqualsArg defense-in-depth path in applyFromToReplacement.
// This path is normally dead code (the regex handles --model=<value>), but is retained
// as a safety net. This test verifies it works correctly if ever reached.
func TestApplyFromToReplacementSplitModelEqualsDefenseInDepth(t *testing.T) {
	// To exercise the splitModelEqualsArg path in applyFromToReplacement, we need
	// a --model=<value> token that is NOT matched by the regex but IS parseable by
	// splitModelEqualsArg. The regex uses (?i), so case won't help. Instead we
	// directly test the transformer's applyFromToReplacement method with a
	// modelPattern that intentionally does not match a specific value, forcing
	// fallthrough to the splitModelEqualsArg path.
	transformer := &modelTransformer{
		modelFrom: "claude-opus-4-6",
		modelTo:   "claude-sonnet-4-5",
		// Pattern that matches only the space-separated form, not equals form.
		// This is artificial — in production the pattern covers both. Used here
		// to exercise the defense-in-depth splitModelEqualsArg fallback.
		modelPattern: regexp.MustCompile(`(?i)(--model\s+)claude-opus-4-6(\s|$)`),
	}

	tests := []struct {
		name        string
		args        []string
		wantArgs    []string
		wantChanged bool
	}{
		{
			name:        "splitModelEqualsArg path matches from value",
			args:        []string{"--model=claude-opus-4-6"},
			wantArgs:    []string{"--model=claude-sonnet-4-5"},
			wantChanged: true,
		},
		{
			name:        "splitModelEqualsArg path skips non-matching value",
			args:        []string{"--model=claude-haiku-4"},
			wantArgs:    []string{"--model=claude-haiku-4"},
			wantChanged: false,
		},
		{
			name:        "splitModelEqualsArg path skips empty value",
			args:        []string{"--model="},
			wantArgs:    []string{"--model="},
			wantChanged: false,
		},
		{
			name:        "splitModelEqualsArg path skips whitespace value",
			args:        []string{"--model=  "},
			wantArgs:    []string{"--model=  "},
			wantChanged: false,
		},
		{
			name:        "space-separated form still works via regex",
			args:        []string{"--model claude-opus-4-6"},
			wantArgs:    []string{"--model claude-sonnet-4-5"},
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string(nil), tt.args...)
			changed := transformer.applyFromToReplacement(args)
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if !slices.Equal(args, tt.wantArgs) {
				t.Fatalf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}
