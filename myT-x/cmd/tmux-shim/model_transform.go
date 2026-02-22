package main

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"sync"

	"myT-x/internal/config"
	"myT-x/internal/ipc"
)

var (
	agentNameFlagPattern = regexp.MustCompile(`--agent-name(?:\s+|=)(\S+)`)
	// anyModelFlagPattern matches both "--model <value>" (space-separated) and
	// "--model=<value>" (equals-joined) forms when they appear inline within a
	// single argument string (e.g. "claude --model opus --flag" or
	// "claude --model=opus --flag").
	//
	// I-27: \S+ is greedy but this is correct — \S+ stops at whitespace by
	// definition, so greedy vs non-greedy (\S+?) produces identical results.
	// With "..--model=v1 --model=v2", \S+ matches "v1" (stops at space),
	// then ReplaceAllString finds the second "--model=v2" as a separate match.
	// Both occurrences are replaced as expected.
	anyModelFlagPattern = regexp.MustCompile(`(?i)(--model(?:\s+|=))\S+(\s|$)`)

	modelConfigLoadMu sync.Mutex
	modelConfigCached *config.AgentModel
	modelConfigLoaded bool
)

var modelTransformCommands = map[string]struct{}{
	"new-session":  {},
	"new-window":   {},
	"split-window": {},
	"send-keys":    {},
}

type modelConfigLoader func() (*config.AgentModel, error)

type modelTransformer struct {
	modelFrom    string
	modelTo      string
	matchAll     bool // true when From == "ALL" (case-insensitive wildcard)
	modelPattern *regexp.Regexp
	overrides    []modelOverrideRule
}

type modelOverrideRule struct {
	namePattern *regexp.Regexp
	model       string
}

// applyModelTransform rewrites --model values according to agent_model config.
// It returns (false, nil) when the command is out of scope, no rewrite applies,
// or config loading fails (shim spec: never block on transform failure).
func applyModelTransform(req *ipc.TmuxRequest, load modelConfigLoader) (bool, error) {
	// Defensive guard: upstream currently always provides non-nil requests.
	// Keep this to prevent future call-site regressions from crashing shim flow.
	if req == nil {
		return false, errors.New("tmux request is nil")
	}
	if !isModelTransformCommand(req.Command) || len(req.Args) == 0 {
		return false, nil
	}
	if load == nil {
		load = loadAgentModelConfig
	}

	agentModel, err := load()
	if err != nil {
		// Shim spec: transform failure must not block forwarding.
		// Log the error and skip model transformation.
		debugLog("applyModelTransform: config load failed: %v", err)
		return false, nil
	}

	transformer := newModelTransformer(agentModel)
	if transformer == nil {
		return false, nil
	}

	before := append([]string(nil), req.Args...)
	transformer.transform(req.Args)
	return !slices.Equal(before, req.Args), nil
}

// loadAgentModelConfig loads agent_model from the default config path.
// Successful reads are cached per process. Read errors are not cached so that
// transient failures (e.g. temporary lock/parse race) can recover on retry.
//
// Thread safety: guarded by modelConfigLoadMu. Safe for concurrent calls from
// multiple goroutines. The cached result pointer is immutable after first
// successful load; callers must not modify the returned *AgentModel.
//
// NOTE: Global cache is not refreshed during process lifetime. This is
// acceptable because the shim is a short-lived process (one invocation per
// tmux command). If the shim becomes long-lived (daemon mode), add a TTL or
// file-watcher reset mechanism. Tests that exercise this function must reset
// the cache via resetModelConfigLoadState and must NOT use t.Parallel().
func loadAgentModelConfig() (*config.AgentModel, error) {
	modelConfigLoadMu.Lock()
	defer modelConfigLoadMu.Unlock()

	if modelConfigLoaded {
		return modelConfigCached, nil
	}

	configPath := config.DefaultPath()
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	modelConfigCached = cfg.AgentModel
	modelConfigLoaded = true
	return modelConfigCached, nil
}

func isModelTransformCommand(command string) bool {
	_, ok := modelTransformCommands[strings.ToLower(strings.TrimSpace(command))]
	return ok
}

// isAllModelFrom returns true when from is the special wildcard "ALL" (case-insensitive).
// When ALL is specified, all --model values are replaced with the target model.
func isAllModelFrom(from string) bool {
	return strings.EqualFold(strings.TrimSpace(from), "ALL")
}

func newModelTransformer(agentModel *config.AgentModel) *modelTransformer {
	if agentModel == nil {
		return nil
	}

	transformer := &modelTransformer{
		modelFrom: strings.TrimSpace(agentModel.From),
		modelTo:   strings.TrimSpace(agentModel.To),
		overrides: make([]modelOverrideRule, 0, len(agentModel.Overrides)),
	}

	if transformer.modelFrom != "" && transformer.modelTo != "" {
		if isAllModelFrom(transformer.modelFrom) {
			transformer.matchAll = true
		} else {
			// Match both "--model <from>" (space-separated) and "--model=<from>"
			// (equals-joined) forms within inline argument strings.
			transformer.modelPattern = regexp.MustCompile(
				`(?i)(--model(?:\s+|=))` + regexp.QuoteMeta(transformer.modelFrom) + `(\s|$)`,
			)
		}
	}

	for _, override := range agentModel.Overrides {
		name := strings.TrimSpace(override.Name)
		model := strings.TrimSpace(override.Model)
		if name == "" || model == "" {
			continue
		}
		transformer.overrides = append(transformer.overrides, modelOverrideRule{
			namePattern: regexp.MustCompile("(?i)" + regexp.QuoteMeta(name)),
			model:       model,
		})
	}

	if transformer.modelPattern == nil && !transformer.matchAll && len(transformer.overrides) == 0 {
		return nil
	}
	return transformer
}

// transform applies the model replacement strategy in priority order:
//
// S-07: 2-step fallthrough logic —
//
//	Step 1 (Override): If --agent-name matches a configured override rule, the
//	  override's model is applied unconditionally via applyModelOverride. No further
//	  steps run because overrides are agent-specific and take absolute precedence.
//	Step 2 (Wildcard / From-To): When no override matched, the generic replacement
//	  runs. "ALL" (matchAll) replaces every --model value; otherwise only values
//	  matching modelFrom are replaced. These two branches are mutually exclusive
//	  because newModelTransformer sets either matchAll or modelPattern, never both.
//
// Both steps use applyModelOverride for the actual replacement when the target
// model is already known (override and ALL paths). applyFromToReplacement is
// used only for the specific From->To case where value matching is required.
func (t *modelTransformer) transform(args []string) {
	if len(args) == 0 {
		return
	}

	// Step 1: Agent-specific override (highest priority, exclusive).
	if overrideModel, ok := t.findOverrideModel(args); ok {
		// Override found: apply it exclusively. Do not fall through to matchAll/fromTo
		// even if --model flag is absent in args (agent may set model via other means).
		t.applyModelOverride(args, overrideModel)
		return
	}

	// Step 2: Generic model replacement (wildcard ALL or specific From->To).
	if t.matchAll {
		t.applyModelOverride(args, t.modelTo)
		return
	}

	if t.modelPattern != nil {
		t.applyFromToReplacement(args)
	}
}

// findOverrideModel scans args for an --agent-name value and matches it against
// the configured override rules. Overrides are evaluated in declaration order
// (first match wins). If no override matches, ("", false) is returned.
func (t *modelTransformer) findOverrideModel(args []string) (string, bool) {
	if len(t.overrides) == 0 {
		return "", false
	}

	for i := range args {
		candidate, found := extractAgentName(args, i)
		if !found {
			continue
		}
		for _, rule := range t.overrides {
			if rule.namePattern == nil {
				continue
			}
			if rule.namePattern.MatchString(candidate) {
				return rule.model, true
			}
		}
	}
	return "", false
}

func (t *modelTransformer) applyModelOverride(args []string, targetModel string) bool {
	replaced := false
	safeTarget := escapeRegexpReplacement(targetModel)
	for i, currentArg := range args {
		if anyModelFlagPattern.MatchString(currentArg) {
			args[i] = anyModelFlagPattern.ReplaceAllString(currentArg, "${1}"+safeTarget+"${2}")
			replaced = true
			continue
		}

		if !isModelFlagToken(currentArg) {
			prefix, value, ok := splitModelEqualsArg(currentArg)
			if !ok {
				continue
			}
			// Defensive guard: skip replacement when --model= has empty value.
			if strings.TrimSpace(value) == "" {
				debugLog("applyModelOverride: skipping empty --model= value at args[%d]", i)
				continue
			}
			args[i] = prefix + targetModel
			replaced = true
			continue
		}
		if i+1 >= len(args) {
			continue
		}
		if isLikelyOptionToken(args[i+1]) {
			continue
		}
		// Defensive guard: skip replacement when next arg is empty/whitespace.
		// In normal agent-teams flow this should not occur, but prevents
		// accidental empty model assignment if args are malformed.
		if strings.TrimSpace(args[i+1]) == "" {
			debugLog("applyModelOverride: skipping empty model value at args[%d]", i+1)
			continue
		}
		args[i+1] = targetModel
		replaced = true
	}
	return replaced
}

func (t *modelTransformer) applyFromToReplacement(args []string) bool {
	// Defense-in-depth: newModelTransformer guards against reaching here with
	// empty from/to, but an explicit check prevents silent no-ops if call
	// sites are added in the future.
	if t.modelFrom == "" || t.modelTo == "" {
		return false
	}
	replaced := false
	safeModelTo := escapeRegexpReplacement(t.modelTo)
	for i, currentArg := range args {
		replacedInline := t.modelPattern.ReplaceAllString(currentArg, "${1}"+safeModelTo+"${2}")
		if replacedInline != currentArg {
			args[i] = replacedInline
			replaced = true
			continue
		}
		if !isModelFlagToken(currentArg) {
			// S-23: Defense-in-depth — this splitModelEqualsArg path handles the case
			// where --model=<value> appears as a standalone token (not embedded in a
			// longer inline string). In practice, the regex pattern above should catch
			// all --model=<value> forms, making this branch effectively dead code.
			// Retained as a safety net against future regex changes or edge cases.
			prefix, modelValue, ok := splitModelEqualsArg(currentArg)
			if !ok {
				continue
			}
			// I-28: Empty value guard — symmetric with applyModelOverride's guard.
			// Skip replacement when --model= has empty value to prevent writing
			// an empty model name into the argument.
			if strings.TrimSpace(modelValue) == "" {
				debugLog("applyFromToReplacement: skipping empty --model= value at args[%d]", i)
				continue
			}
			if strings.EqualFold(modelValue, t.modelFrom) {
				args[i] = prefix + t.modelTo
				replaced = true
			}
			continue
		}
		if i+1 >= len(args) {
			continue
		}
		if isLikelyOptionToken(args[i+1]) {
			continue
		}
		// I-28: Empty value guard — symmetric with applyModelOverride's guard.
		// Skip replacement when next arg is empty/whitespace to prevent
		// accidental empty model assignment if args are malformed.
		if strings.TrimSpace(args[i+1]) == "" {
			debugLog("applyFromToReplacement: skipping empty model value at args[%d]", i+1)
			continue
		}
		if strings.EqualFold(strings.TrimSpace(args[i+1]), t.modelFrom) {
			args[i+1] = t.modelTo
			replaced = true
		}
	}
	return replaced
}

func extractAgentName(args []string, index int) (string, bool) {
	arg := strings.TrimSpace(args[index])
	if arg == "" {
		return "", false
	}

	matches := agentNameFlagPattern.FindStringSubmatch(arg)
	if len(matches) >= 2 {
		candidate := normalizeFlagValue(matches[1])
		if candidate == "" || isLikelyOptionToken(candidate) {
			return "", false
		}
		return candidate, true
	}
	if !strings.EqualFold(arg, "--agent-name") {
		return "", false
	}
	if index+1 >= len(args) {
		return "", false
	}
	candidate := normalizeFlagValue(firstToken(args[index+1]))
	if candidate == "" || isLikelyOptionToken(candidate) {
		return "", false
	}
	return candidate, true
}

func firstToken(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func normalizeFlagValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"'`)
}

func escapeRegexpReplacement(value string) string {
	return strings.ReplaceAll(value, "$", "$$")
}

func isModelFlagToken(arg string) bool {
	return strings.EqualFold(strings.TrimSpace(arg), "--model")
}

func splitModelEqualsArg(arg string) (prefix, value string, ok bool) {
	const modelEqualsPrefix = "--model="
	trimmed := strings.TrimSpace(arg)
	if !strings.HasPrefix(strings.ToLower(trimmed), modelEqualsPrefix) {
		return "", "", false
	}
	return trimmed[:len(modelEqualsPrefix)], strings.TrimSpace(trimmed[len(modelEqualsPrefix):]), true
}

func isLikelyOptionToken(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "-")
}
