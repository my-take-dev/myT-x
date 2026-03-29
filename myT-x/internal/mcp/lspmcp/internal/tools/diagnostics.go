package tools

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
)

// --- 診断系ハンドラ（pull/push diagnostics） ---

func (s *service) handleDiagnostics(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	usePull, err := boolArg(args, "usePull", true)
	if err != nil {
		return nil, err
	}

	if usePull && s.client.SupportsCapability("diagnosticProvider") {
		raw, reqErr := s.client.Request(ctx, "textDocument/diagnostic", map[string]any{
			"textDocument": map[string]any{"uri": snapshot.URI},
		})
		if reqErr != nil {
			s.logToolWarning("pull diagnostics request failed (path=%q), falling back to push-cache: %v", target.RelativePath, reqErr)
			// フォールバック: pull失敗の情報を push-cache レスポンスに付与
			return s.pushCacheDiagnostics(ctx, target, snapshot, args, reqErr)
		}

		report, err := decodeAny(raw)
		if err != nil {
			return nil, err
		}

		// SUG-6: null レスポンスは「診断結果なし」として扱い、push-cache にフォールバックしない
		if report == nil {
			return map[string]any{
				"path":        target.RelativePath,
				"diagnostics": []any{},
				"count":       0,
				"source":      "pull",
			}, nil
		}

		if reportMap, ok := report.(map[string]any); ok {
			items := extractDiagnosticsFromPullReport(reportMap)
			if items == nil {
				items = []any{}
			}
			return map[string]any{
				"path":        target.RelativePath,
				"diagnostics": items,
				"count":       len(items),
				"source":      "pull",
				"pullReport":  reportMap,
			}, nil
		}
		s.logToolWarning("pull diagnostics report is not a JSON object (path=%q, got %T), falling back to push-cache", target.RelativePath, report)
	}

	return s.pushCacheDiagnostics(ctx, target, snapshot, args, nil)
}

// pushCacheDiagnostics は push-cache から診断を返す。pullErr が非 nil の場合、レスポンスに pull 失敗情報を付与する。
func (s *service) pushCacheDiagnostics(ctx context.Context, target documentTarget, snapshot lsp.DocumentSnapshot, args map[string]any, pullErr error) (any, error) {
	waitMs, _, err := optionalInt(args, "waitMs")
	if err != nil {
		return nil, err
	}
	if waitMs <= 0 {
		waitMs = 250
	}
	if err := sleep(ctx, time.Duration(waitMs)*time.Millisecond); err != nil {
		return nil, err
	}

	diagnostics := s.client.Diagnostics(snapshot.URI)
	out := make([]any, len(diagnostics))
	for i := range diagnostics {
		out[i] = diagnostics[i]
	}

	result := map[string]any{
		"path":        target.RelativePath,
		"diagnostics": out,
		"count":       len(out),
		"source":      "push-cache",
	}
	// SUG-7: pull 失敗時にフォールバック理由を明示
	if pullErr != nil {
		result["pullFailed"] = true
		result["pullError"] = pullErr.Error()
	}
	return result, nil
}

func (s *service) handleWorkspaceDiagnostics(ctx context.Context, args map[string]any) (any, error) {
	rawCaps := s.client.Capabilities()
	if !supportsCapabilityField(rawCaps, "diagnosticProvider", "workspaceDiagnostics") {
		return nil, fmt.Errorf("workspace diagnostics not supported by server (diagnosticProvider.workspaceDiagnostics required)")
	}

	previousResultIDs, err := optionalArrayArg(args, "previousResultIds")
	if err != nil {
		return nil, err
	}
	identifier := optionalString(args, "identifier")

	params := map[string]any{
		"previousResultIds": previousResultIDs,
	}
	if strings.TrimSpace(identifier) != "" {
		params["identifier"] = identifier
	}

	raw, err := s.client.Request(ctx, "workspace/diagnostic", params)
	if err != nil {
		return nil, err
	}

	report, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}
	items := extractDiagnosticsFromWorkspaceReport(report)

	return map[string]any{
		"diagnostics":       items,
		"count":             len(items),
		"source":            "workspace-pull",
		"workspaceReport":   report,
		"previousResultIds": previousResultIDs,
		"identifier":        identifier,
	}, nil
}

// --- 診断レポート抽出ヘルパー ---

// collectRelatedDiagnostics は relatedDocuments マップからソート済みキー順で診断を収集する。
func collectRelatedDiagnostics(related map[string]any) []any {
	var out []any
	for _, key := range slices.Sorted(maps.Keys(related)) {
		doc, ok := related[key].(map[string]any)
		if !ok {
			continue
		}
		if items, ok := doc["items"].([]any); ok {
			out = append(out, items...)
		}
	}
	return out
}

func extractDiagnosticsFromPullReport(report any) []any {
	root, ok := report.(map[string]any)
	if !ok {
		return nil
	}
	var out []any
	if items, ok := root["items"].([]any); ok {
		out = append(out, items...)
	}
	if related, ok := root["relatedDocuments"].(map[string]any); ok {
		out = append(out, collectRelatedDiagnostics(related)...)
	}
	return out
}

func extractDiagnosticsFromWorkspaceReport(report any) []any {
	root, ok := report.(map[string]any)
	if !ok {
		return nil
	}

	reports, ok := root["items"].([]any)
	if !ok {
		return nil
	}

	out := make([]any, 0)
	for _, item := range reports {
		docReport, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if diagnostics, ok := docReport["items"].([]any); ok {
			out = append(out, diagnostics...)
		}
		if related, ok := docReport["relatedDocuments"].(map[string]any); ok {
			out = append(out, collectRelatedDiagnostics(related)...)
		}
	}
	return out
}
