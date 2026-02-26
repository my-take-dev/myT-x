# Changelog


## v0.0.9
- 右サイドバーのファイルツリーにPathコピー機能を追加
- Edge起動時にカーソルがとられて日本語変換できない件での対応を追加
- 右サイドバーに組み込みMCP枠を追加

---

## v0.0.8
- 入力履歴機能の追加（右サイドバーに Input History ビューを新設。JSONL永続化・リアルタイム表示・バッジ通知・Ctrl+Shift+H ショートカット対応）
- 入力履歴のラインバッファリング方式への変更（100msバッチングを廃止し、Enter区切りのコマンド単位記録に改善。CSI/OSCシーケンス除去・Backspace編集再現・Ctrl+C/D記録対応）
- エラー表示画面の対象情報拡充（フロントエンド未捕捉例外・React Error Boundary・API通信失敗をバックエンドログへ一元化）
- 右サイドバーショートカット設定機能の追加（Viewer各ビューのキーボードショートカットを設定画面からカスタマイズ・`config.yaml` に永続化）
- クイックスタートセッション機能の追加（セッション未選択画面からワンクリックでデフォルトディレクトリのセッションを即座に作成）
- サイドバーアイコン並び順の修正（Input HistoryをDiff直下に配置、Error Logを最下部固定。import順序ルールをコメントで明記）
- Diff アイコン位置修正（誤って最下部に配置されていた `position: "bottom"` を削除）
- Input History の `position: "bottom"` 誤設定修正（上部グループに正しく配置）

---

## v0.0.7
- 右サイドバー（Activity Strip）のアイコン配置変更（エラーログを最下部固定）
- DIFFサイドバー: サブディレクトリ・日本語ファイル名の非表示バグ修正（`core.quotepath`・NUL区切り対応）
- Diff View プラグインの追加（`git diff HEAD` をファイルツリー＋行単位diffで表示）
- セッションエラーログビューアの追加（Warn/ErrorをJSONL永続化・リアルタイム表示・バッジ通知）
- Diff View ディレクトリ階層修正 & 「Expand hidden lines」折りたたみ機能追加
- `new-window` をセッション作成に変更（1セッション=1ウィンドウモデルへ移行・タブUI削除）
- コードレビュー指摘対応（DevPanel API・セッションログ・フロントエンド品質改善）
- Diff View サブディレクトリ表示バグ修正（`addedDirs`管理・state更新順序）

---

## v0.0.6
- Viewer System（プラグイン型ビューア基盤）の追加
  - File Tree ビュー（仮想化ツリー＋ファイルプレビュー）
  - Git Graph ビュー（SVGグラフ＋diff表示）
  - File Content Viewer: パスコピー・Ctrl+Cコピー・マウス選択コピー機能追加
- 速度改善（バッファサイズ拡大・ロック早期解放・フロントエンド描画最適化）
- WebSocket化による高スループットペインデータ転送（pane:dataをWebSocketバイナリストリームに移行）
  - `workerutil` パッケージ新規作成（パニックリカバリ共通ヘルパー）
  - `wsserver` パッケージ新規作成（WebSocketサーバー）

---

## v0.0.5
- Claude Code環境変数（`claude_env`）設定機能の追加
  - 設定画面からClaude Code環境変数を管理
  - セッション作成時のチェックボックスで適用制御（`use_claude_env` / `use_pane_env`）
  - `pane_env` にもデフォルトON/OFFチェック追加

---

## v0.0.4
- レガシーShimパス問題の修正（旧`%LOCALAPPDATA%\github.com\my-take-dev\myT-x\bin`の自動クリーンアップ）
- モデル置換 `from: "ALL"` ワイルドカード対応（全モデルを一括置換先に変更可能）
- フロントエンド マルチウィンドウ対応（tmux-shim連携ギャップ修正・ウィンドウタブUI追加）
- Claude-Code-Communication互換対応（`select-pane -T`フラグ・`attach-session`コマンド追加）
- コードレビュー指摘対応（フォントサイズ競合・SearchBar再オープン時クエリ残存・バッファ定数化など）

---

## v0.0.3
- ドラッグ＆ドロップ時のパス表示を最適化（`\\` → `\`）
- パフォーマンス改善調査レポート & 実装計画
- ペイン分割時の作業ディレクトリ修正
- IME変換障害 調査結果 & 修正計画

---

## v0.0.2
- プログラム一式アップロード
- gitworktreeの不要な情報のクリーンアップを追加
  - 実態が削除済みなのに、UIから選択可能な状態の解消
- アプリケーション起動時にコマンドプロンプト画面が2連続で画面表示される状態を修正

---

## v0.0.1
- プレビュー版
