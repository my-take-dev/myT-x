# generic-lsp-mcp 開発者ガイド

このドキュメントは、`generic-lsp-mcp` を開発・改修する人向けです。

## 1. 前提条件（開発時）

1. Go 1.22+
2. 対象LSPサーバー（手元検証用）

## 2. 構成

- エントリーポイント: `cmd/generic-lsp-mcp/main.go`
- MCPサーバー層: `internal/mcp/server.go`, `internal/mcp/types.go`
- LSPクライアント層: `internal/lsp/client.go`
- パス/編集処理: `internal/lsp/paths.go`, `internal/lsp/edits.go`
- LSP型定義: `internal/lsp/types.go`
- ツール定義: `internal/tools/registry.go`
- LSP拡張ツール定義: `lsp_pkg/*`（LSP種類単位。例: `lsp_pkg/gopls`, `lsp_pkg/python`, `lsp_pkg/typescriptls`, `lsp_pkg/rust` など）
- JSON-RPCフレーミング: `internal/jsonrpc/framing.go`

## 3. ビルド

```bash
go build -o generic-lspmcp ./cmd/generic-lspmcp
```

## 4. テスト

```bash
go test ./...
```

現在は主に以下のユニットテストがあります。

- `cmd/generic-lsp-mcp/main_test.go`
- `app_test.go`
- `internal/jsonrpc/framing_test.go`
- `internal/jsonrpc/message_test.go`
- `internal/lsp/client_test.go`
- `internal/lsp/edits_test.go`
- `internal/lsp/paths_test.go`
- `internal/mcp/registry_test.go`
- `internal/mcp/server_test.go`
- `internal/tools/registry_test.go`

## 5. ローカル実行（開発確認）

例: `gopls`

```bash
./generic-lspmcp -lsp gopls -root .
```

ログ出力付き:

```bash
./generic-lspmcp -lsp gopls -root . -log-file ./mcp.log
```

## 6. 実装方針

1. MCPはstdioのJSON-RPCとして実装
2. LSPはContent-Lengthフレーミングで実装
3. 基本ツールは `internal/tools/registry.go` の `BuildRegistry` で常時登録
4. LSP固有ツールは `lsp_pkg/*` で静的定義し、`BuildRegistry` が `-lsp`/`-lsp-arg` から判定して追加登録
5. 編集系（rename/format）は `applyEdits` フラグで副作用を制御

## 7. ツール追加手順

1. `internal/tools/registry.go` に `mcp.Tool` を追加
2. 入力スキーマ関数を追加（必要なら）
3. ハンドラ実装で `client.Request(...)` を呼ぶ
4. 必要に応じて `internal/lsp/edits.go` の適用処理を再利用
5. `go test ./...` を実行

## 8. LSP固有ツール追加手順

1. `lsp_pkg/<lsp名>/` に固有ロジックを実装
2. `Matches(command, args)` で対象LSP判定を定義
3. `BuildTools(client, rootDir)` で `[]mcp.Tool` を返す
4. `lsp_pkg/extensions.go` の `extensionSpecs` に登録
5. `go test ./...` を実行

## 9. 注意点

1. LSPの `character` はUTF-16基準です（`internal/lsp/edits.go` 参照）。
2. `WorkspaceEdit` は `changes` と `documentChanges` の両方を扱います。
3. サーバー固有機能差分があるため、`lsp_check_capabilities` を前提に設計してください。
