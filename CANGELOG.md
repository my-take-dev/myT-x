# Changelog

## v0.0.7
- 右サイドバー（Activity Strip）のアイコン配置変更（エラーログを最下部固定）
- DIFFサイドバー: サブディレクトリ・日本語ファイル名の非表示バグ修正（`core.quotepath`・NUL区切り対応）
- Diff View プラグインの追加（`git diff HEAD` をファイルツリー＋行単位diffで表示）
- セッションエラーログビューアの追加（Warn/ErrorをJSONL永続化・リアルタイム表示・バッジ通知）
- Diff View ディレクトリ階層修正 & 「Expand hidden lines」折りたたみ機能追加
- `new-window` をセッション作成に変更（1セッション=1ウィンドウモデルへ移行・タブUI削除）
- コードレビュー指摘対応（DevPanel API・セッションログ・フロントエンド品質改善）
- Diff View サブディレクトリ表示バグ修正（`addedDirs`管理・state更新順序）
- その他
　- change-log v0.0.7 review指摘事項修正

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
- その他
　- change-log v0.0.6 review指摘事項修正

---

## v0.0.5
- Claude Code環境変数（`claude_env`）設定機能の追加
  - 設定画面からClaude Code環境変数を管理
  - セッション作成時のチェックボックスで適用制御（`use_claude_env` / `use_pane_env`）
  - `pane_env` にもデフォルトON/OFFチェック追加
- その他
　- change-log v0.0.5 review指摘事項修正

---

## v0.0.4
- レガシーShimパス問題の修正（旧`%LOCALAPPDATA%\github.com\my-take-dev\myT-x\bin`の自動クリーンアップ）
- モデル置換 `from: "ALL"` ワイルドカード対応（全モデルを一括置換先に変更可能）
- フロントエンド マルチウィンドウ対応（tmux-shim連携ギャップ修正・ウィンドウタブUI追加）
- Claude-Code-Communication互換対応（`select-pane -T`フラグ・`attach-session`コマンド追加）
- コードレビュー指摘対応（フォントサイズ競合・SearchBar再オープン時クエリ残存・バッファ定数化など）
- その他
　- change-log v0.0.4 review指摘事項修正

## v0.0.3
- ドラッグ＆ドロップ時のパス表示を最適化（`\\` → `\`）
- パフォーマンス改善調査レポート & 実装計画
- ペイン分割時の作業ディレクトリ修正
- IME変換障害 調査結果 & 修正計画
- その他
　- change-log v0.0.3 review指摘事項修正

---

## v0.0.2
- プログラム一式アップロード
- gitworktreeの不要な情報のクリーンアップを追加
  - 実態が削除済みなのに、UIから選択可能な状態の解消
- アプリケーション起動時にコマンドプロンプト画面が2連続で画面表示される状態を修正
- その他
　- change-log v0.0.2 指摘事項修正

---

## v0.0.1
- プレビュー版
