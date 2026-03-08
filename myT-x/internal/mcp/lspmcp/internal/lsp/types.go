package lsp

import "encoding/json"

// Position は LSP の位置（0 始まりの行と UTF-16 コード単位のカラム）。
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range は LSP の範囲。
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// TextEdit は LSP のテキスト編集。
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// Diagnostic は標準の LSP 診断。
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Code     any    `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// WorkspaceEdit はフォーマット/リネーム/コードアクションで使う LSP WorkspaceEdit のサブセット。
type WorkspaceEdit struct {
	Changes         map[string][]TextEdit `json:"changes,omitempty"`
	DocumentChanges []json.RawMessage     `json:"documentChanges,omitempty"`
}

// Location は Location または LocationLink のソースに依存しない正規化された位置。
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// DocumentSnapshot は LSP リクエスト用のローカル状態。
type DocumentSnapshot struct {
	URI        string `json:"uri"`
	Path       string `json:"path"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Content    string `json:"content"`
}

// ApplyFileSummary は 1 ファイルに適用された編集を表す。
type ApplyFileSummary struct {
	Path         string `json:"path"`
	RelativePath string `json:"relativePath"`
	EditCount    int    `json:"editCount"`
}

// ApplySummary はワークスペース編集の適用結果を表す。
type ApplySummary struct {
	AppliedFiles int                `json:"appliedFiles"`
	TotalEdits   int                `json:"totalEdits"`
	Files        []ApplyFileSummary `json:"files"`
}
