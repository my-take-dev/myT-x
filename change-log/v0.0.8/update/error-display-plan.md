# エラー表示画面の対象情報および出力調整に関する実装計画（詳細版）

## 1. 現状の課題と一元管理の必要性
現状の `myT-x` アプリケーションでは、バックエンドでフックした `slog` のログレコードだけが `TeeHandler` および `writeSessionLogEntry` によって一元化され、フロントエンドから `GetSessionErrorLog()` を呼び出して画面表示しています。
これはファイル (`session-logs/session-*.jsonl`) にも永続化され高い追跡性を持ちますが、**「フロントエンド内で完結しているエラー」がここに到達しない** という欠点があります。全てのシステム状態を完全に一元化するためには、UIやAPI境界のエラーも同一のログ基盤（JSONLファイルと表示画面）へ流し込む必要があります。

---

## 2. 実装方針（アーキテクチャの改善）
フロントエンドのストアだけでエラー表示を混在させるのではなく、**Wailsのバインディングを経由してすべてをバックエンドのログシステムに流し込む（API追加）設計**が最も堅牢であり、「ファイルのログエクスポート」機能とも整合性が取れます。

### ターゲットとするエラー情報群
1. **[UI] 予期せぬ実行時エラー (Uncaught Exception / Promise Rejection)**
   - `window.onerror` および `window.onunhandledrejection`
2. **[UI] React コンポーネントのクラッシュ**
   - React Error Boundary に到達したエラー情報
3. **[API] Wails API の通信失敗・サーバーからの拒否**
   - ローカル関数呼び出しの失敗（例: `useBackendSync.ts` で行われている `api.GetConfig().catch(...)` や各種 `Promise.allSettled` の拒否オブジェクト）
4. **[イベント] バックエンド発生だがフロントで処理されている警告**
   - `tmux:worker-panic`, `worktree:cleanup-failed` など、現在 `notifyWarn()` だけでToast表示し、履歴に残っていないイベント

---

## 3. 具体的な実装ステップ（アクションプラン）

### Step 1: バックエンドのエンドポイント追加 (Go)
`app_session_log.go` に Wails にバインドされる `LogFrontendEvent`（仮称）を新設します。このメソッドにより、フロントエンドから `writeSessionLogEntry` へ直接ログエントリーを書き込みます。
```go
// app_session_log.go の適当な場所に追加
func (a *App) LogFrontendEvent(level, msg, source string) {
	// エラーレベルのバリデーションやクリッピングを実施後
	a.writeSessionLogEntry(SessionLogEntry{
		Timestamp: time.Now().Format("20060102150405"),
		Level:     level, // "error", "warn", "info" etc
		Message:   msg,
		Source:    source, // 例: "frontend/ui", "frontend/api"
	})
}
```
※ この実装によりフロントエンドで発生したエラーも JSONL に記録され、直後に（既存の仕様により）バックエンドから `app:session-log-updated` イベントが発火するため、フロントエンドは自動的に再取得（リロード）して最新エラーを表示できます。

### Step 2: フロントエンドAPIの更新
Go側で関数を追加したら、`wails dev` （またはビルド） を1番実行して `api.ts` 用のバインディング（`LogFrontendEvent`）を生成・エクスポートします。
* `myT-x/frontend/src/api.ts` を更新し `LogFrontendEvent` を統合。

### Step 3: グローバルの予期せぬ例外の捕捉
UIのエントリーポイント (`main.tsx` または `App.tsx` の上位層) にて、グローバルなイベントリスナを配置します。
```typescript
window.addEventListener("error", (event) => {
    void api.LogFrontendEvent("error", event.message, "frontend/unhandled").catch(console.error);
});
window.addEventListener("unhandledrejection", (event) => {
    void api.LogFrontendEvent("error", String(event.reason), "frontend/promise").catch(console.error);
});
```
※ Toast（`useNotificationStore`）での表示を併用しても良いですが、システム全体に及ばないエラーはログ表示画面のみに留め、ユーザーの邪魔をしない設計も検討します。

### Step 4: React Error Boundary コンポーネントの導入
コンポーネントがクラッシュした際に画面全体が真っ白になる現象を防ぎつつ、エラー内容をバックエンドログに永続化させる `ErrorBoundary.tsx` を作成し、`App.tsx` または `Layout` 階層をラップします。
* ライフサイクルメソッド（または専用フック）の `componentDidCatch` 内で `api.LogFrontendEvent` を発行。
* エラー時は専用のリカバリー用UI（「エラーが発生しました」や再起動ボタンなど）を表示。

### Step 5: `useBackendSync.ts` 等の部分的なエラーハンドリング修正
ソースコード内で手動で `notifyWarn()` されている箇所を探し、必要に応じて `api.LogFrontendEvent` を併記・置換します。
* **対象1:** `useBackendSync.ts` での API フェッチ（`ListSessions`, `GetConfigAndFlushWarnings`）の失敗ハンドラ部分
* **対象2:** 既に `useBackendSync.ts` にある `tmux:worker-panic` などのイベントハンドラ内。現在は `notifyWarn(...)` されていますが、このイベントの重要度は高いため Session Log 本体にも `warn` 以上のレベルとして出力（あるいはバックエンド側で直接 `slog.Warn` するように責務を変える）を検討。
    *(※バックエンドで発生したイベントは、もともとGoレイヤーで`slog.Error`していればこの処理は不要になります。ただし、`worktree`関連などは実装によっては `slog` していない可能性があるため確認要)*

### Step 6: 該当箇所の修正およびレビュー
以上の変更後、一度Wailsアプリを起動し、フロントエンド側で手動的にエラー（例: コンソールのボタンから未定義の関数を呼び出す等）を発生させ、以下を確認します。
1. `session-logs/session-*.jsonl` にエラーログが記録されること
2. 画面上のError Logパネルを開いた際、`[frontend/unhandled]` などのタグ付きで表示されること
3. ReactのクラッシュがErrorBoundaryで表示され、Toast以外からも確実に状況が残ること。
