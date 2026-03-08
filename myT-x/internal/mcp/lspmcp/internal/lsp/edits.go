package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// ApplyWorkspaceEdit は WorkspaceEdit 内のテキスト編集をローカルファイルに適用する。
// rootDir はワークスペースルートの絶対パス。edit は LSP の WorkspaceEdit。
// 戻り値は適用結果のサマリ。パストラバーサルや不正な編集がある場合はエラーを返す。
func ApplyWorkspaceEdit(rootDir string, edit WorkspaceEdit) (ApplySummary, error) {
	merged, err := collectWorkspaceEdits(edit)
	if err != nil {
		return ApplySummary{}, err
	}
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return ApplySummary{}, fmt.Errorf("resolve root dir: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)

	summary := ApplySummary{
		Files: make([]ApplyFileSummary, 0, len(merged)),
	}

	paths := make([]string, 0, len(merged))
	for p := range merged {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		absolutePath, err := ensurePathWithinRoot(rootAbs, path)
		if err != nil {
			return ApplySummary{}, err
		}
		edits := merged[path]
		if len(edits) == 0 {
			continue
		}

		originalBytes, err := os.ReadFile(absolutePath)
		if err != nil {
			return ApplySummary{}, fmt.Errorf("read %s: %w", absolutePath, err)
		}

		updated, err := ApplyTextEdits(string(originalBytes), edits)
		if err != nil {
			return ApplySummary{}, fmt.Errorf("apply edits to %s: %w", absolutePath, err)
		}

		if updated != string(originalBytes) {
			if err := WriteFilePreserveMode(absolutePath, []byte(updated)); err != nil {
				return ApplySummary{}, err
			}
		}

		summary.AppliedFiles++
		summary.TotalEdits += len(edits)
		summary.Files = append(summary.Files, ApplyFileSummary{
			Path:         absolutePath,
			RelativePath: RelativePath(rootAbs, absolutePath),
			EditCount:    len(edits),
		})
	}

	return summary, nil
}

// collectWorkspaceEdits は WorkspaceEdit をファイル単位の TextEdit 配列に正規化する。
// changes と documentChanges の両方に対応。CreateFile / RenameFile / DeleteFile は未対応。
func collectWorkspaceEdits(edit WorkspaceEdit) (map[string][]TextEdit, error) {
	result := map[string][]TextEdit{}

	for uri, edits := range edit.Changes {
		path, err := URIToPath(uri)
		if err != nil {
			return nil, fmt.Errorf("decode URI %q: %w", uri, err)
		}
		result[filepath.Clean(path)] = append(result[filepath.Clean(path)], edits...)
	}

	for _, raw := range edit.DocumentChanges {
		var candidate struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Edits []TextEdit `json:"edits"`
		}

		if err := json.Unmarshal(raw, &candidate); err != nil {
			return nil, fmt.Errorf("decode documentChanges entry: %w", err)
		}
		if candidate.TextDocument.URI == "" || len(candidate.Edits) == 0 {
			return nil, fmt.Errorf("documentChanges entry must include textDocument.uri and edits")
		}

		path, err := URIToPath(candidate.TextDocument.URI)
		if err != nil {
			return nil, fmt.Errorf("decode documentChanges URI %q: %w", candidate.TextDocument.URI, err)
		}
		path = filepath.Clean(path)
		result[path] = append(result[path], candidate.Edits...)
	}

	return result, nil
}

// WriteFilePreserveMode は既存ファイルのパーミッションを維持して書き込む。
func WriteFilePreserveMode(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ensurePathWithinRoot は対象パスが rootDir 配下にあることを検証し、絶対パスを返す。
func ensurePathWithinRoot(rootDir string, path string) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	absolutePath = filepath.Clean(absolutePath)
	rel, err := filepath.Rel(rootDir, absolutePath)
	if err != nil {
		return "", fmt.Errorf("resolve path %q against root %q: %w", absolutePath, rootDir, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("workspace edit path escapes root directory: %s", absolutePath)
	}
	return absolutePath, nil
}

// ApplyTextEdits は LSP テキスト編集を指定テキストに適用する。
// text は元テキスト、edits は適用する編集の配列（逆順で適用される）。
// 戻り値は適用後のテキスト。Position が範囲外の場合はエラーを返す。
func ApplyTextEdits(text string, edits []TextEdit) (string, error) {
	if len(edits) == 0 {
		return text, nil
	}

	working := append([]TextEdit(nil), edits...)
	sort.SliceStable(working, func(i, j int) bool {
		a := working[i].Range.Start
		b := working[j].Range.Start
		if a.Line != b.Line {
			return a.Line > b.Line
		}
		if a.Character != b.Character {
			return a.Character > b.Character
		}
		aEnd := working[i].Range.End
		bEnd := working[j].Range.End
		if aEnd.Line != bEnd.Line {
			return aEnd.Line > bEnd.Line
		}
		return aEnd.Character > bEnd.Character
	})

	current := text
	for _, edit := range working {
		start, err := positionToOffset(current, edit.Range.Start)
		if err != nil {
			return "", err
		}
		end, err := positionToOffset(current, edit.Range.End)
		if err != nil {
			return "", err
		}
		if start > end {
			return "", fmt.Errorf("invalid edit range: start(%d) > end(%d)", start, end)
		}

		current = current[:start] + edit.NewText + current[end:]
	}

	return current, nil
}

// positionToOffset は UTF-16 ベースの Position をバイトオフセットに変換する。
func positionToOffset(text string, pos Position) (int, error) {
	if pos.Line < 0 || pos.Character < 0 {
		return 0, fmt.Errorf("negative position is not allowed: %+v", pos)
	}

	lines := strings.Split(text, "\n")
	if pos.Line >= len(lines) {
		return 0, fmt.Errorf("line out of range: %d (max %d)", pos.Line, len(lines)-1)
	}

	offset := 0
	for i := 0; i < pos.Line; i++ {
		offset += len(lines[i]) + 1
	}

	columnOffset, err := utf16ColumnToByteOffset(lines[pos.Line], pos.Character)
	if err != nil {
		return 0, err
	}
	return offset + columnOffset, nil
}

// utf16ColumnToByteOffset は UTF-16 カラム位置をバイトオフセットに変換する。サロゲートペア対応。
func utf16ColumnToByteOffset(line string, character int) (int, error) {
	if character < 0 {
		return 0, fmt.Errorf("negative character index: %d", character)
	}
	if character == 0 {
		return 0, nil
	}

	byteOffset := 0
	utf16Units := 0
	for _, r := range line {
		runeUnits := len(utf16.Encode([]rune{r}))
		if utf16Units+runeUnits > character {
			return byteOffset, nil
		}
		utf16Units += runeUnits
		byteOffset += utf8.RuneLen(r)
		if utf16Units == character {
			return byteOffset, nil
		}
	}

	if character > utf16Units {
		// Allow one-past-end style columns.
		return len(line), nil
	}

	return byteOffset, nil
}
