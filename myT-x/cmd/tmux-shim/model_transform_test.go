package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	for i := 0; i < workers; i++ {
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

func TestApplyModelTransformReturnsErrorWithoutMutatingArgs(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Args:    []string{"--model claude-opus-4-6"},
	}
	before := append([]string(nil), req.Args...)

	changed, err := applyModelTransform(&req, func() (*config.AgentModel, error) {
		return nil, errors.New("load failed")
	})
	if err == nil {
		t.Fatal("expected loader error")
	}
	if changed {
		t.Fatal("changed should be false on loader error")
	}
	if !slices.Equal(req.Args, before) {
		t.Fatalf("args mutated on loader error: got %#v, want %#v", req.Args, before)
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
