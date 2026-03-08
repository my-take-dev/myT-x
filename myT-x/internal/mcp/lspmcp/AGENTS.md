# AGENTS.md

このファイルは、このリポジトリで作業する Codex / Claude 向けの運用メモです。  
目的は「LSP関連の依頼に、短時間で一貫した対応をすること」です。

## 1. 対象と前提

- 対象プロジェクト: `generic-lsp-mcp`（Go製の汎用LSP向けMCPサーバー）
- 主な入口: `cmd/generic-lsp-mcp/main.go`
- 基本ツール群: `internal/tools/registry.go`
- gopls拡張（実装例）: `lsp_pkg/gopls/*`

先に読むべき補助資料:
- 利用視点: `USER_GUIDE.md`
- 実装視点: `DEVELOPER_GUIDE.md`

## 2. 最短対応フロー（LSP依頼共通）

1. 対象ファイルを特定する（`rg --files` 推奨）。
2. まず `lsp_get_diagnostics` でエラー/警告の有無を確認する。
3. 必要に応じて `lsp_check_capabilities` でサーバー対応機能を確認する。
4. 依頼タイプに応じて専用ツールを使う（下記「目的別レシピ」）。
5. 編集系（rename/format）は `applyEdits` の指定有無を明示して実行する。
6. 変更後は再度 `lsp_get_diagnostics` を取り、件数の増減を確認して報告する。

## 3. 目的別レシピ

### A. 文法エラー/警告を確認したい

使うツール:
- `lsp_get_diagnostics`

最小引数:
- `relativePath`

推奨オプション:
- `usePull: true`（対応サーバーなら pull diagnostics を優先）
- `waitMs: 500-1500`（push-cacheフォールバックの待機）

### B. 定義/参照をたどりたい

使うツール:
- `lsp_get_definitions`
- `lsp_find_references`

位置指定ルール:
- `line` は 1-based
- `character` / `column` は UTF-16 0-based
- `textTarget` が使える場合は、列を省略しても位置解決できる

### C. 補完・シグネチャ・hoverを見たい

使うツール:
- `lsp_get_completion`（`maxItems` で件数制限）
- `lsp_get_signature_help`
- `lsp_get_hover`

### D. 変更提案/整形/リネームをしたい

使うツール:
- `lsp_get_code_actions`
- `lsp_format_document`
- `lsp_rename_symbol`

副作用の扱い:
- `lsp_format_document`: `applyEdits: false` で差分確認、`true` で反映
- `lsp_rename_symbol`: `applyEdits` の既定は `true`。安全確認したい時は `false` を明示

## 4. gopls 実装例（LSP拡張のサンプル）

`gopls` は「拡張機構の一例」です。`-lsp gopls` で起動している場合のみ以下を使用:
- `gopls_list_extension_commands`
- `gopls_execute_command`

進め方:
1. `gopls_list_extension_commands` で利用可能コマンドを確認
2. 必要なコマンドだけ `gopls_execute_command` で実行
3. 実行後に `lsp_get_diagnostics` で影響を確認

## 5. 失敗時の標準チェック

1. `relativePath` が `root` から正しく解決されるか
2. `line` と `character` の基準（1-based / 0-based）を取り違えていないか
3. LSPサーバーが機能対応しているか（`lsp_check_capabilities`）
4. `applyEdits` の真偽が意図どおりか
5. `textTarget` が実際にファイル内に存在するか

## 6. 返答テンプレート（短文）

### Diagnostics確認結果

- 対象: `<path>`
- 結果: `count=<n>`
- 内訳: `source=pull|push-cache`
- 備考: 必要なら代表的な診断を数件のみ提示

### 編集実行結果

- 対象: `<path>`
- 実行: `format|rename`
- 反映: `applyEdits=true|false`
- 変更件数: `<n>`
- 事後確認: diagnostics の増減

## 7. 実装変更時の参照先

- MCPツール登録: `internal/tools/registry.go`
- LSP I/O: `internal/lsp/client.go`
- Edit適用: `internal/lsp/edits.go`
- 拡張ツール管理: `lsp_pkg/extensions.go`
- gopls拡張: `lsp_pkg/gopls/gopls.go`

## 8. LSP拡張の追加指針（重要）

結論:
- 記載しておくべき。拡張追加時の迷いを減らし、`gopls`（実装例）と同じ責務分離を保てるため。

### フォルダ配置

- 追加先: `lsp_pkg/<lsp名>/`
- 代表ファイル例: `lsp_pkg/<lsp名>/<lsp名>.go`
- 推奨テスト: `lsp_pkg/<lsp名>/<lsp名>_test.go`

### 概念（責務分離）

- `Matches(command, args) bool`  
  起動引数から「この拡張を有効化すべきか」を判定する。
- `BuildTools(client, rootDir) []mcp.Tool`  
  そのLSP専用ツール群を返す。
- `lsp_pkg/extensions.go`  
  `extensionSpecs` に拡張を登録し、共通層から解決できるようにする。

### 追加手順

1. `lsp_pkg/<lsp名>/` を作成し、`Matches` と `BuildTools` を実装
2. `lsp_pkg/extensions.go` の `extensionSpecs` に登録
3. 必要なら静的コマンドカタログなど拡張固有のメタ情報を定義
4. `go test ./...` で既存回帰を確認
5. 利用ガイド（`USER_GUIDE.md`）に拡張ツール名を追記

### 実装上の注意

- 汎用機能（hover/rename/format）は `internal/tools/registry.go` 側に残し、拡張には置かない
- 拡張側は「そのLSPでしか意味を持たない操作」に限定する
- `Matches` は実行ファイル名だけでなく引数形式（例: `go tool ...`）も考慮する
- `gopls` の実装はテンプレートとして参照するが、他LSP固有事情に合わせて最小限の差分で設計する

## 9. 説明文の書き方（厳選3項目）

目的:
- AIが「使うべきか」「必要入力」「副作用」を短文で即判断できるようにする。
- コンテキスト肥大を避けるため、説明は `when / args / effect` の3項目に限定する。

### 9.1 ツール説明（`mcp.Tool.Description`）の必須形式

- 形式（固定）:
  `when: <利用シーン> args: <最小必須引数> effect: <read|edit|exec|read or edit>.`
- 例:
  `when: Check errors and warnings for a file args: relativePath (usePull/waitMs optional) effect: read.`

ルール:
- `when`: 1文、用途だけを書く（背景説明は書かない）。
- `args`: 最小必須入力を優先。任意引数は必要な場合のみ括弧で補足。
- `effect`: 次の語彙のみ使用  
  `read` / `edit` / `exec` / `read or edit`

### 9.2 共通ツールへの適用範囲

- 対象: `internal/tools/registry.go` で登録する全ツール。
- `lsp_*` 系の `Description` は上記固定形式に揃える。
- `withScopeNote(...)` による言語スコープ補足は維持してよい（基本説明の後ろに連結）。

### 9.3 拡張ツールへの適用範囲

- 対象: `lsp_pkg/<lsp名>/*.go` の `BuildTools(...)` で登録する拡張ツール。
- 最低限、以下2ツールの `Description` を固定形式にする:
  - `<lsp>_list_extension_commands`
  - `<lsp>_execute_extension_command`
- 命名例外: `gopls` の実行ツールは `gopls_execute_command` を使う（`gopls_execute_extension_command` ではない）。

### 9.4 list系レスポンスの推奨項目

- `*_list_extension_commands` の返却 `commands[]` には、可能なら以下を含める:
  - `when`
  - `args`
  - `effect`
  - `description`（上記3項目を結合した短文）

備考:
- 既知コマンドは静的カタログから付与し、未知コマンドは保守的に  
  `effect: exec` をデフォルトとする。

### 9.5 execute系レスポンスの推奨項目

- `*_execute_*` の返却には、実行した `command` に対応するガイドを付与する:
  - `commandGuide.when`
  - `commandGuide.args`
  - `commandGuide.effect`
  - `commandGuide.description`

### 9.6 非推奨（コンテキスト肥大を招く）

- 長い背景説明、複数段落の手順、実装内部の詳細を `Description` に入れること。
- 1ツール説明内に複数ユースケースを詰め込むこと。

以上。
