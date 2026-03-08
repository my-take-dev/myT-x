package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyTextEditsNegativePosition(t *testing.T) {
	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: -1, Character: 0},
				End:   Position{Line: 0, Character: 0},
			},
			NewText: "x",
		},
	}
	_, err := ApplyTextEdits("abc", edits)
	if err == nil {
		t.Fatalf("ApplyTextEdits with negative Position should return error")
	}
}

func TestApplyTextEditsLineOutOfRange(t *testing.T) {
	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 10, Character: 0},
				End:   Position{Line: 10, Character: 1},
			},
			NewText: "x",
		},
	}
	_, err := ApplyTextEdits("abc", edits)
	if err == nil {
		t.Fatalf("ApplyTextEdits with line out of range should return error")
	}
}

func TestApplyWorkspaceEditPathTraversal(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	// ルート外へのパストラバーサル（親ディレクトリを指す URI）
	outsidePath, err := filepath.Abs(filepath.Join(tmp, "..", "outside.txt"))
	if err != nil {
		t.Fatalf("Abs failed: %v", err)
	}
	uri, err := PathToURI(outsidePath)
	if err != nil {
		t.Fatalf("PathToURI failed: %v", err)
	}
	edit := WorkspaceEdit{
		Changes: map[string][]TextEdit{
			uri: {{Range: Range{}, NewText: "x"}},
		},
	}
	_, err = ApplyWorkspaceEdit(root, edit)
	if err == nil {
		t.Fatalf("ApplyWorkspaceEdit with path traversal should return error")
	}
}

func TestApplyWorkspaceEditInvalidDocumentChanges(t *testing.T) {
	tmp := t.TempDir()
	edit := WorkspaceEdit{
		DocumentChanges: []json.RawMessage{json.RawMessage(`{"invalid": "json", "noTextDocument": true}`)},
	}
	_, err := ApplyWorkspaceEdit(tmp, edit)
	if err == nil {
		t.Fatalf("ApplyWorkspaceEdit with invalid DocumentChanges should return error")
	}
}

func TestApplyTextEditsSingle(t *testing.T) {
	original := "const value = oldName;\n"
	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 0, Character: 14},
				End:   Position{Line: 0, Character: 21},
			},
			NewText: "newName",
		},
	}

	got, err := ApplyTextEdits(original, edits)
	if err != nil {
		t.Fatalf("ApplyTextEdits failed: %v", err)
	}

	want := "const value = newName;\n"
	if got != want {
		t.Fatalf("unexpected result:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestApplyTextEditsMultiple(t *testing.T) {
	original := "abc\ndef\n"
	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 0, Character: 1},
				End:   Position{Line: 0, Character: 3},
			},
			NewText: "X",
		},
		{
			Range: Range{
				Start: Position{Line: 1, Character: 0},
				End:   Position{Line: 1, Character: 2},
			},
			NewText: "Y",
		},
	}

	got, err := ApplyTextEdits(original, edits)
	if err != nil {
		t.Fatalf("ApplyTextEdits failed: %v", err)
	}

	want := "aX\nYf\n"
	if got != want {
		t.Fatalf("unexpected result:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestApplyTextEditsUTF16Columns(t *testing.T) {
	original := "a🙂b\n"
	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 0, Character: 1},
				End:   Position{Line: 0, Character: 3},
			},
			NewText: "X",
		},
	}

	got, err := ApplyTextEdits(original, edits)
	if err != nil {
		t.Fatalf("ApplyTextEdits failed: %v", err)
	}

	want := "aXb\n"
	if got != want {
		t.Fatalf("unexpected result:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestApplyWorkspaceEditChangesOnly(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	filePath := filepath.Join(root, "foo.txt")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	uri, err := PathToURI(filePath)
	if err != nil {
		t.Fatalf("PathToURI failed: %v", err)
	}
	edit := WorkspaceEdit{
		Changes: map[string][]TextEdit{
			uri: {
				{
					Range: Range{
						Start: Position{Line: 0, Character: 6},
						End:   Position{Line: 0, Character: 11},
					},
					NewText: "LSP",
				},
			},
		},
	}

	summary, err := ApplyWorkspaceEdit(root, edit)
	if err != nil {
		t.Fatalf("ApplyWorkspaceEdit failed: %v", err)
	}
	if summary.AppliedFiles != 1 || summary.TotalEdits != 1 {
		t.Errorf("unexpected summary: AppliedFiles=%d TotalEdits=%d", summary.AppliedFiles, summary.TotalEdits)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "hello LSP\n" {
		t.Errorf("unexpected file content: %q", string(content))
	}
}

func TestApplyWorkspaceEditDocumentChangesOnly(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	filePath := filepath.Join(root, "bar.txt")
	if err := os.WriteFile(filePath, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	uri, err := PathToURI(filePath)
	if err != nil {
		t.Fatalf("PathToURI failed: %v", err)
	}
	docChange := json.RawMessage(`{"textDocument":{"uri":"` + uri + `"},"edits":[{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":9}},"newText":"modified"}]}`)
	edit := WorkspaceEdit{
		DocumentChanges: []json.RawMessage{docChange},
	}

	summary, err := ApplyWorkspaceEdit(root, edit)
	if err != nil {
		t.Fatalf("ApplyWorkspaceEdit failed: %v", err)
	}
	if summary.AppliedFiles != 1 || summary.TotalEdits != 1 {
		t.Errorf("unexpected summary: AppliedFiles=%d TotalEdits=%d", summary.AppliedFiles, summary.TotalEdits)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	// 編集後は "modified" または "modified\n"（プラットフォームの改行による）
	got := string(content)
	if got != "modified" && got != "modified\n" {
		t.Errorf("unexpected file content: %q", got)
	}
}
