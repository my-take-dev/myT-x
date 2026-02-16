# defensive-coding-checklist ブラッシュアップ計画

## Context

v0.0.3開発で33件のレビューから200+の指摘が発生。現在のdefensive-coding-checklistは121項目あるが、
レビューで頻出した指摘パターンの一部がカバーされていない。また、項目102-121が「コード品質」セクションに
誤配置されており、本来のカテゴリ（エラー処理、並行処理等）に属していない構造的問題がある。

**目的**: レビュアーから二度と指摘されないチェックリストに仕上げる。

---

## 変更概要

### A. SKILL.md の再構成（最重要）
### B. リファレンスファイルへの新パターン追加（5ファイル）
### C. リファレンスファイル間の相互参照追加

---

## A. SKILL.md 再構成

### A-1. 項目102-121を正しいカテゴリに移動

現在「コード品質」セクション末尾に雑多に追加された項目を、本来のカテゴリに再配置する。

| 現番号 | 内容 | 移動先カテゴリ |
|--------|------|--------------|
| 102 | deferロールバックguardフラグ順序 | エラー処理 |
| 103 | rollback全副作用逆順 | エラー処理 |
| 104 | Create系rollbackパターン統一 | エラー処理 |
| 105 | ロールバック失敗→フロントエンド通知 | エラー処理 |
| 106 | `%w`ラッピング統一 | エラー処理 |
| 107 | エラー正規化1層のみ | エラー処理 |
| 108 | context.WithCancel条件分岐内複数回禁止 | 並行処理 |
| 109 | cancel全exitパス呼び出し | 並行処理 |
| 110 | rollback前goroutine終了待ち | 並行処理 |
| 111 | time.After→time.NewTimer | 並行処理 |
| 112 | CloseHandle後ゼロ化 | リソースライフサイクル |
| 113 | TerminateProcess→WaitForSingleObject | リソースライフサイクル |
| 114 | パラメータ/返り値にerror型混在禁止 | API/型設計 |
| 115 | 縮小キャスト前範囲チェック | API/型設計 |
| 116 | io.Reader/Writer契約遵守 | API/型設計 |
| 117 | ループ内重い関数キャッシュ | コード品質 |
| 118 | TrimRight/TrimSuffix使い分け | コード品質 |
| 119 | 冪等性テスト個別アサーション | テスト要件 |
| 120 | TOCTOU NOTEコメント統一 | コメント規律 |
| 121 | 公開関数入口filepath.Clean | パス安全性 |

→ 移動後、全項目を連番で振り直す。

### A-2. 新規チェック項目の追加（12項目）

レビューで繰り返し指摘されたが、現チェックリストに無いパターン:

#### コード品質カテゴリに追加（3項目）

| # | チェック | 根拠（レビュー頻度） |
|---|---------|-------------------|
| NEW-1 | 定義した関数/メソッドが実際にコード内で呼ばれているか。未使用関数が残っていないか（#31の関数版） | 8+ 指摘: acquireGitSemaphore, GenerateBranchName, CreateWorktreeDetached等 |
| NEW-2 | 別名だけの転送関数（完全なエイリアス）が存在しないか。意味的差異がなければ統合する | 4+ 指摘: executeGitCommandAt≡runGitCLI等 |
| NEW-3 | 関数名が実際の処理内容を正確に表しているか（"normalize"が実際にはvalidateのみ、"Raw"が実際にはtrim付き等） | 5+ 指摘: normalizeWorktreeBranchName, runGitCommandRaw等 |

#### エラー処理カテゴリに追加（2項目）

| # | チェック | 根拠 |
|---|---------|------|
| NEW-4 | バックエンドでユーザー可視状態に影響するエラーが発生した場合、ログだけでなくフロントエンドへイベント通知しているか（#105のrollback以外版） | 10+ 指摘: copyConfigFiles失敗→UI通知無し、emitWorktreeCleanupFailure→ctx nil時無通知等 |
| NEW-5 | 同一パッケージの複数Create系/操作系メソッドで、成功パスのイベント発行が重複（二重emit）していないか | 6+ 指摘: CreateSession defer emitSnapshot + activateCreatedSession内emitSnapshot |

#### テスト要件カテゴリに追加（5項目）

| # | チェック | 根拠 |
|---|---------|------|
| NEW-6 | リトライループの**最大リトライ到達（exhaustion）パス**にテストがあるか | 4+ 指摘: readRegistryPathRawValueWithRetry n>len(buffer) 3回連続未テスト |
| NEW-7 | ロールバック処理パスの統合テストがあるか（途中失敗→前ステップの逆操作が実行されることの検証） | 6+ 指摘: CreateSessionWithWorktree rollback未テスト、KillSession+dirtyWorktree未テスト |
| NEW-8 | 関数の**全エラーreturnパス**に対応するテストケースがあるか（正常系だけでなく各エラー分岐） | 15+ 指摘: 最頻出テーマ。error return文ごとにテストケース必須 |
| NEW-9 | 数値境界（0, 負数, MaxInt）、空入力、null文字等の**境界値テスト**がバリデーション関数以外にもあるか（#88の汎用版） | 8+ 指摘: Resize(0,0), \x00 in branch name, 1-byte UTF-16等 |
| NEW-10 | テストヘルパー内でLocale中立設定（`LC_ALL=C`等）が本番コードと同じ環境変数を設定しているか | 3+ 指摘: createBareAndClone内でlocaleNeutralGitEnv未設定 |

#### リソースライフサイクルカテゴリに追加（1項目）

| # | チェック | 根拠 |
|---|---------|------|
| NEW-11 | ファイル書き込みが**アトミック**（temp→rename）パターンを使っているか。`os.WriteFile`直接使用で中断時にデータ破損しないか | 4+ 指摘: embedded shim path非アトミック書き込み |

#### 並行処理カテゴリに追加（1項目）

| # | チェック | 根拠 |
|---|---------|------|
| NEW-12 | `os.Getenv`/`os.Setenv`のread-modify-writeがmutex保護されているか。または設計上の許容をNOTEコメントで明示しているか | 3+ 指摘: EnsureProcessPathContains非アトミック |

### A-3. 既存項目の強化（4項目）

レビューで特に頻出したため、テキストを明確化:

| 現番号 | 変更内容 |
|--------|---------|
| #9 | 「エラーを意図的にnilで返す箇所」→「エラーを意図的にnilで返す箇所、**またはerrorをログのみで処理して続行する箇所**」に拡張 |
| #34 | 「兄弟メソッドで入力バリデーションパターンが統一」→ 具体例追加「**例: CreateSessionはrootPath空チェックなし、RenameSessionはあり → 統一必須**」 |
| #82 | 「正常系+異常系」→「正常系 + **全エラーreturnパスの異常系**（エラー分岐ごとにテストケース）」に強化 |
| #106 | 「`fmt.Errorf("...: %w", err)`ラッピング統一」→ 先頭に**[最頻出指摘]**マーカー追加 |

### A-4. 参照ガイドテーブルの更新

コード品質の行に「デッドコード検出、エイリアス統合、関数名/意図整合」を追加。

---

## B. リファレンスファイルへの新パターン追加

### B-1. `references/code-quality.md` に3セクション追加

#### 4. デッドコード検出（未使用関数/メソッド）
- チェック項目: 定義した関数が実際にコールされているか
- Bad: テストでのみ使用されるpublic関数、ラッパー関数の元関数が残存
- Good: 使用箇所がない関数を削除 or internal/testutil移動
- 検出方法: `grep -rn "funcName" --include="*.go"` で呼び出し箇所確認

#### 5. 不要なエイリアス関数の統合
- チェック項目: 引数をそのまま転送するだけの関数がないか
- Bad: `executeGitCommandAt(dir, args...)` が `runGitCLI(dir, args...)` と完全同一
- Good: 一方に統合し、もう一方を削除

#### 6. 関数名と実装の意図整合
- チェック項目: 関数名が暗示する処理と実装が一致しているか
- Bad: `normalizeXxx` が実際にはTrimSpace+validateのみ（正規化していない）
- Bad: `runGitCommandRaw` が実際にはTrimRight付き（Rawではない）
- Good: 実装に合った名前に変更、またはdocコメントで意図を明記

### B-2. `references/test-requirements.md` に5セクション追加

#### 14. リトライ最大到達テスト
- チェック項目: リトライループのexhaustionパスのテスト
- パターン: maxRetries回全て失敗→最終エラー返却を検証

#### 15. ロールバックパス統合テスト
- チェック項目: 途中失敗で前ステップの副作用が確実に逆操作されるか
- パターン: Step1成功→Step2失敗→Step1のロールバック実行を検証

#### 16. 全エラーreturnパスカバレッジ
- チェック項目: 関数内の各`return err`/`return fmt.Errorf(...)`に対応するテストケース
- 検出方法: `grep -n "return.*err\|return fmt.Errorf" target.go` → テスト対応確認

#### 17. 汎用境界値テスト（バリデーション以外）
- チェック項目: 数値引数の0/負数/MaxInt、文字列のnull文字/制御文字
- パターン: Resize(0,0), UTF-16の1バイト入力、CJK文字パス等

#### 18. テストヘルパーのLocale中立性
- チェック項目: テストヘルパーが本番コードと同じ環境変数を設定しているか
- Bad: `createBareAndClone`内で`LC_ALL=C`未設定 → 日本語Windows環境でgitエラー文字列が変わり失敗
- Good: 本番の`localeNeutralGitEnv`と同じ環境変数をテストヘルパーでも設定

### B-3. `references/resource-lifecycle.md` に1セクション追加

#### 8. アトミックファイル書き込み
- チェック項目: `os.WriteFile`の直接使用を避け、temp→renameパターンを使用
- Bad: `os.WriteFile(targetPath, data, 0o644)` → 書き込み中断でデータ破損
- Good: `os.CreateTemp` → `Write` → `Close` → `os.Rename(tmpPath, targetPath)`

### B-4. `references/concurrency.md` に2セクション追加

#### 17. イベント/スナップショット二重発行防止
- チェック項目: 成功パスでemitSnapshotが2回呼ばれないか
- Bad: defer内emitSnapshot + 内部関数内emitSnapshot → 成功時二重発行
- Good: deferに`if retErr != nil`ガード、または発行箇所を1箇所に統一

#### 18. OS環境変数のスレッドセーフティ
- チェック項目: `os.Getenv`→加工→`os.Setenv`のread-modify-writeをmutex保護
- Bad: 複数goroutineが同時にPATH追記 → 競合
- Good: sync.Mutex保護、またはNOTEコメントで「プロセス初期化時のみ呼び出し」を明記

### B-5. `references/error-handling.md` に2セクション追加

#### 27. ユーザー可視状態エラーのフロントエンド通知
- チェック項目: バックエンドで発生した非致命的エラーがUI操作結果に影響する場合の通知
- Bad: copyConfigFiles失敗→ログのみ→ユーザーはファイルがコピーされたと誤認
- Good: 部分失敗をイベント経由でフロントエンドに通知、UI上で警告表示

#### 28. 成功パスのイベント二重発行防止
- concurrency.md Section 17への相互参照 + エラー処理の観点からの補足

---

## C. リファレンスファイル間の相互参照追加

以下の箇所に `> See also:` 行を追加:

| ファイル | セクション | 追加する参照先 |
|---------|----------|--------------|
| `change-propagation.md` 冒頭 | フィールド追加 | `→ テスト: [test-requirements.md Section 3](test-requirements.md#3-フィールドドリフト検出テスト)` |
| `api-type-design.md` Section 7 | フィールドドリフト | `→ 変更伝播: [change-propagation.md](change-propagation.md)` |
| `path-safety.md` Section 2 | Windowsパス比較 | `→ プラットフォーム: [platform-compat.md Section 4](platform-compat.md)` |
| `platform-compat.md` Section 4 | Windowsパス正規化 | `→ パス安全性: [path-safety.md Section 2](path-safety.md)` |
| `resource-lifecycle.md` Section 5 | goroutine追跡 | `→ 並行処理: [concurrency.md Section 1](concurrency.md#1-goroutineにはアプリケーションcontextを渡す)` |
| `comments.md` Section 14 | アトミック書き込み意図 | `→ リソース: [resource-lifecycle.md Section 8](resource-lifecycle.md)（アトミックファイル書き込み）` |

---

## 実施順序

1. **SKILL.md 再構成**: 項目移動→新規追加→既存強化→連番振り直し→参照テーブル更新
2. **code-quality.md**: 3セクション追加
3. **test-requirements.md**: 5セクション追加
4. **concurrency.md**: 2セクション追加
5. **resource-lifecycle.md**: 1セクション追加
6. **error-handling.md**: 2セクション追加
7. **相互参照追加**: 6ファイルにSee also追加

## 検証方法

1. SKILL.md の全項目が連番で抜けがないことを確認
2. 各新規チェック項目がリファレンスファイルの対応セクションにリンクしていることを確認
3. リファレンスファイルの新セクションにBad/Good例があることを確認
4. 相互参照リンクの参照先が存在することを確認
5. 全ファイルがUTF-8 BOM無しであることを確認

## 最終項目数の見積もり

- 現在: 121項目
- 移動のみ（増減なし）: 121項目
- 新規追加: +12項目
- **合計: 133項目**（目標135以内に収まる）
