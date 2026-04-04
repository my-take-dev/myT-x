package config

import "maps"

// Clone returns a deep copy of cfg.
// Use this when sharing config snapshots across goroutines or package boundaries.
func Clone(src Config) Config {
	dst := src

	if src.Keys != nil {
		dst.Keys = make(map[string]string, len(src.Keys))
		maps.Copy(dst.Keys, src.Keys)
	}

	dst.Worktree.SetupScripts = cloneStringSlice(src.Worktree.SetupScripts)
	dst.Worktree.CopyFiles = cloneStringSlice(src.Worktree.CopyFiles)
	dst.Worktree.CopyDirs = cloneStringSlice(src.Worktree.CopyDirs)

	if src.AgentModel != nil {
		agentModelCopy := *src.AgentModel
		agentModelCopy.Overrides = cloneAgentModelOverrides(src.AgentModel.Overrides)
		dst.AgentModel = &agentModelCopy
	}

	if src.PaneEnv != nil {
		dst.PaneEnv = make(map[string]string, len(src.PaneEnv))
		maps.Copy(dst.PaneEnv, src.PaneEnv)
	}

	if src.ClaudeEnv != nil {
		claudeEnvCopy := *src.ClaudeEnv
		if src.ClaudeEnv.Vars != nil {
			claudeEnvCopy.Vars = make(map[string]string, len(src.ClaudeEnv.Vars))
			maps.Copy(claudeEnvCopy.Vars, src.ClaudeEnv.Vars)
		}
		dst.ClaudeEnv = &claudeEnvCopy
	}

	if src.ViewerShortcuts != nil {
		dst.ViewerShortcuts = make(map[string]string, len(src.ViewerShortcuts))
		maps.Copy(dst.ViewerShortcuts, src.ViewerShortcuts)
	}

	if src.MCPServers != nil {
		dst.MCPServers = make([]MCPServerConfig, len(src.MCPServers))
		for i, s := range src.MCPServers {
			dst.MCPServers[i] = s
			if s.Args != nil {
				dst.MCPServers[i].Args = cloneStringSlice(s.Args)
			}
			if s.Env != nil {
				dst.MCPServers[i].Env = make(map[string]string, len(s.Env))
				maps.Copy(dst.MCPServers[i].Env, s.Env)
			}
			if s.ConfigParams != nil {
				dst.MCPServers[i].ConfigParams = cloneMCPServerConfigParams(s.ConfigParams)
			}
		}
	}

	if src.TaskScheduler != nil {
		tsCopy := *src.TaskScheduler
		tsCopy.MessageTemplates = cloneMessageTemplates(src.TaskScheduler.MessageTemplates)
		dst.TaskScheduler = &tsCopy
	}

	return dst
}

func cloneMessageTemplates(src []MessageTemplate) []MessageTemplate {
	if src == nil {
		return nil
	}
	dst := make([]MessageTemplate, len(src))
	copy(dst, src)
	return dst
}

func cloneStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneAgentModelOverrides(src []AgentModelOverride) []AgentModelOverride {
	if src == nil {
		return nil
	}
	dst := make([]AgentModelOverride, len(src))
	copy(dst, src)
	return dst
}

func cloneMCPServerConfigParams(src []MCPServerConfigParam) []MCPServerConfigParam {
	if src == nil {
		return nil
	}
	dst := make([]MCPServerConfigParam, len(src))
	copy(dst, src)
	return dst
}
