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
	anyModelFlagPattern  = regexp.MustCompile(`(?i)(--model\s+)\S+(\s|$)`)

	modelConfigLoadMu sync.Mutex
	modelConfigCached *config.AgentModel
	modelConfigLoaded bool
)

var modelTransformCommands = map[string]struct{}{
	"new-session":  {},
	"split-window": {},
	"send-keys":    {},
}

type modelConfigLoader func() (*config.AgentModel, error)

type modelTransformer struct {
	modelFrom    string
	modelTo      string
	modelPattern *regexp.Regexp
	overrides    []modelOverrideRule
}

type modelOverrideRule struct {
	namePattern *regexp.Regexp
	model       string
}

// applyModelTransform rewrites --model values according to agent_model config.
// It returns (false, nil) when the command is out of scope or no rewrite applies.
// It returns (false, err) when config loading fails or cannot be parsed.
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
		return false, err
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
		transformer.modelPattern = regexp.MustCompile(
			`(?i)(--model\s+)` + regexp.QuoteMeta(transformer.modelFrom) + `(\s|$)`,
		)
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

	if transformer.modelPattern == nil && len(transformer.overrides) == 0 {
		return nil
	}
	return transformer
}

func (t *modelTransformer) transform(args []string) {
	if len(args) == 0 {
		return
	}

	if overrideModel, ok := t.findOverrideModel(args); ok {
		if t.applyModelOverride(args, overrideModel) {
			return
		}
	}

	if t.modelPattern != nil {
		t.applyFromToReplacement(args)
	}
}

func (t *modelTransformer) findOverrideModel(args []string) (string, bool) {
	if len(t.overrides) == 0 {
		return "", false
	}

	for i := 0; i < len(args); i++ {
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
			prefix, _, ok := splitModelEqualsArg(currentArg)
			if !ok {
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
			prefix, modelValue, ok := splitModelEqualsArg(currentArg)
			if ok && strings.EqualFold(modelValue, t.modelFrom) {
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
