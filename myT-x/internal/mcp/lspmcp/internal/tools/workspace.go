package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
)

// --- ワークスペース操作系ハンドラ（コードアクション、リネーム、コマンド実行） ---

func (s *service) handleCodeActions(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	endLine, endCharacter, err := resolveRangeEnd(args, line, character)
	if err != nil {
		return nil, err
	}

	pushDiagnostics := s.client.Diagnostics(snapshot.URI)
	rawDiagnostics := make([]any, len(pushDiagnostics))
	for i := range pushDiagnostics {
		rawDiagnostics[i] = pushDiagnostics[i]
	}

	raw, err := s.client.Request(ctx, "textDocument/codeAction", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"range": map[string]any{
			"start": map[string]any{"line": line, "character": character},
			"end":   map[string]any{"line": endLine, "character": endCharacter},
		},
		"context": map[string]any{
			"diagnostics": rawDiagnostics,
		},
	})
	if err != nil {
		return nil, err
	}

	actions, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":    target.RelativePath,
		"actions": actions,
		"range": map[string]any{
			"start": map[string]any{"line": line + 1, "character": character},
			"end":   map[string]any{"line": endLine + 1, "character": endCharacter},
		},
	}, nil
}

func (s *service) handleResolveCodeAction(ctx context.Context, args map[string]any) (any, error) {
	action, err := requiredAny(args, "action")
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "codeAction/resolve", action)
	if err != nil {
		return nil, err
	}

	resolved, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"action": resolved,
	}, nil
}

func (s *service) handleRename(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}
	newName, err := requiredString(args, "newName")
	if err != nil {
		return nil, err
	}
	applyEdits, err := boolArg(args, "applyEdits", true)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/rename", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
		"newName":      newName,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return map[string]any{
			"path":    target.RelativePath,
			"applied": false,
			"message": "No workspace edit returned by language server.",
		}, nil
	}

	var edit lsp.WorkspaceEdit
	if err := json.Unmarshal(raw, &edit); err != nil {
		return nil, fmt.Errorf("parse rename workspace edit: %w", err)
	}

	if !applyEdits {
		return map[string]any{
			"path":          target.RelativePath,
			"applied":       false,
			"workspaceEdit": edit,
		}, nil
	}

	summary, err := lsp.ApplyWorkspaceEdit(target.RootDir, edit)
	if err != nil {
		return nil, err
	}

	for _, file := range summary.Files {
		if _, err := s.client.EnsureDocument(ctx, file.Path); err != nil {
			s.logToolWarning("EnsureDocument failed after rename (path=%q): %v", file.Path, err)
		}
	}

	return map[string]any{
		"path":    target.RelativePath,
		"applied": true,
		"summary": summary,
	}, nil
}

func (s *service) handlePrepareRename(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/prepareRename", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
	})
	if err != nil {
		return nil, err
	}

	result, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":      target.RelativePath,
		"position":  map[string]any{"line": line + 1, "character": character},
		"available": result != nil,
		"result":    result,
	}, nil
}

func (s *service) handleExecuteCommand(ctx context.Context, args map[string]any) (any, error) {
	command, err := requiredString(args, "command")
	if err != nil {
		return nil, err
	}

	rawArguments, ok := args["arguments"]
	if !ok {
		rawArguments = []any{}
	}
	arguments, ok := rawArguments.([]any)
	if !ok {
		return nil, fmt.Errorf("arguments must be an array")
	}

	result, err := s.client.ExecuteCommand(ctx, command, arguments)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"command":   command,
		"arguments": arguments,
		"result":    result,
	}, nil
}
