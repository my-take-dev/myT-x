package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
)

// --- フォーマット系ハンドラ（ドキュメント/範囲/入力時フォーマット） ---

// formattingOptions は3つのフォーマットハンドラで共通のオプション。
type formattingOptions struct {
	tabSize      int
	insertSpaces bool
	applyEdits   bool
}

// parseFormattingOptions は tabSize / insertSpaces / applyEdits を args から解析する。
func parseFormattingOptions(args map[string]any) (formattingOptions, error) {
	tabSize, _, err := optionalInt(args, "tabSize")
	if err != nil {
		return formattingOptions{}, err
	}
	if tabSize <= 0 {
		tabSize = 2
	}
	insertSpaces, err := boolArg(args, "insertSpaces", true)
	if err != nil {
		return formattingOptions{}, err
	}
	applyEdits, err := boolArg(args, "applyEdits", false)
	if err != nil {
		return formattingOptions{}, err
	}
	return formattingOptions{
		tabSize:      tabSize,
		insertSpaces: insertSpaces,
		applyEdits:   applyEdits,
	}, nil
}

// lspOptions は LSP リクエスト用のオプション map を返す。
func (o formattingOptions) lspOptions() map[string]any {
	return map[string]any{
		"tabSize":      o.tabSize,
		"insertSpaces": o.insertSpaces,
	}
}

// applyAndSyncEdits はテキストエディットをファイルに適用し、LSP クライアントへ再同期する。
func (s *service) applyAndSyncEdits(ctx context.Context, target documentTarget, content string, edits []lsp.TextEdit) error {
	updated, err := lsp.ApplyTextEdits(content, edits)
	if err != nil {
		return err
	}
	if err := lsp.WriteFilePreserveMode(target.AbsolutePath, []byte(updated)); err != nil {
		return err
	}
	_, err = s.client.EnsureDocument(ctx, target.AbsolutePath)
	return err
}

func (s *service) handleFormatting(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	opts, err := parseFormattingOptions(args)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/formatting", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"options":      opts.lspOptions(),
	})
	if err != nil {
		return nil, err
	}

	var edits []lsp.TextEdit
	if err := json.Unmarshal(raw, &edits); err != nil {
		return nil, fmt.Errorf("parse formatting edits: %w", err)
	}

	if !opts.applyEdits {
		return map[string]any{
			"path":  target.RelativePath,
			"count": len(edits),
			"edits": edits,
		}, nil
	}

	if err := s.applyAndSyncEdits(ctx, target, snapshot.Content, edits); err != nil {
		return nil, err
	}

	return map[string]any{
		"path":         target.RelativePath,
		"count":        len(edits),
		"applied":      true,
		"absolutePath": target.AbsolutePath,
	}, nil
}

func (s *service) handleRangeFormatting(ctx context.Context, args map[string]any) (any, error) {
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

	opts, err := parseFormattingOptions(args)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/rangeFormatting", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"range": map[string]any{
			"start": map[string]any{"line": line, "character": character},
			"end":   map[string]any{"line": endLine, "character": endCharacter},
		},
		"options": opts.lspOptions(),
	})
	if err != nil {
		return nil, err
	}

	var edits []lsp.TextEdit
	if err := json.Unmarshal(raw, &edits); err != nil {
		return nil, fmt.Errorf("parse range formatting edits: %w", err)
	}

	rangeInfo := map[string]any{
		"start": map[string]any{"line": line + 1, "character": character},
		"end":   map[string]any{"line": endLine + 1, "character": endCharacter},
	}

	if !opts.applyEdits {
		return map[string]any{
			"path":  target.RelativePath,
			"count": len(edits),
			"edits": edits,
			"range": rangeInfo,
		}, nil
	}

	if err := s.applyAndSyncEdits(ctx, target, snapshot.Content, edits); err != nil {
		return nil, err
	}

	return map[string]any{
		"path":         target.RelativePath,
		"count":        len(edits),
		"applied":      true,
		"absolutePath": target.AbsolutePath,
		"range":        rangeInfo,
	}, nil
}

func (s *service) handleOnTypeFormatting(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}
	ch, err := requiredString(args, "ch")
	if err != nil {
		return nil, err
	}

	opts, err := parseFormattingOptions(args)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/onTypeFormatting", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
		"ch":           ch,
		"options":      opts.lspOptions(),
	})
	if err != nil {
		return nil, err
	}

	var edits []lsp.TextEdit
	if err := json.Unmarshal(raw, &edits); err != nil {
		return nil, fmt.Errorf("parse on-type formatting edits: %w", err)
	}

	if !opts.applyEdits {
		return map[string]any{
			"path":     target.RelativePath,
			"count":    len(edits),
			"edits":    edits,
			"position": map[string]any{"line": line + 1, "character": character},
			"ch":       ch,
		}, nil
	}

	if err := s.applyAndSyncEdits(ctx, target, snapshot.Content, edits); err != nil {
		return nil, err
	}

	return map[string]any{
		"path":         target.RelativePath,
		"count":        len(edits),
		"applied":      true,
		"absolutePath": target.AbsolutePath,
		"position":     map[string]any{"line": line + 1, "character": character},
		"ch":           ch,
	}, nil
}
