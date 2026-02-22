# DIFFサイドバー ディレクトリ表示バグ修正 + 防御的コーディング適用

## Context

`plans/fix-diff-sidebar-directory-display.md` で分析済みのバグを修正する。
サブディレクトリ配下のファイル/フォルダがサイドバーに表示されない問題。
原因は `useDiffView.ts` の `buildDiffTree` ロジック不整合と state 更新順序の2点。

加えて、`defensive-coding-checklist` に基づくフロントエンド品質問題も同時修正する。

---

## 修正対象ファイル

| ファイル | 修正内容 |
|----------|----------|
| `myT-x/frontend/src/components/viewer/views/diff-view/useDiffView.ts` | バグ1: addedDirs管理、バグ2: state更新順序 |
| `myT-x/frontend/src/components/viewer/views/diff-view/DiffContentViewer.tsx` | チェックリスト#99: 非nullアサーション除去、#90: index key |

---

## T1: `buildDiffTree` の `addedDirs` 管理修正 (`useDiffView.ts`)

**問題**: `addedDirs.add(dirPath)` が `continue` 判定の**後**にあるため、親が非展開のディレクトリが `addedDirs` に登録されず、後続ファイルで重複ノード生成リスクがある。

**修正**: `addedDirs.add(dirPath)` を `continue` 判定の**前**に移動。

```typescript
// BEFORE (バグ)
for (let i = 1; i < parts.length; i++) {
  const dirPath = parts.slice(0, i).join("/");
  if (addedDirs.has(dirPath)) continue;
  const parentPath = parts.slice(0, i - 1).join("/");
  if (i > 1 && !expandedDirs.has(parentPath)) continue;  // ← addedDirs未登録でcontinue
  addedDirs.add(dirPath);  // ← ここに到達しない
  nodes.push({...});
}

// AFTER (修正)
for (let i = 1; i < parts.length; i++) {
  const dirPath = parts.slice(0, i).join("/");
  if (addedDirs.has(dirPath)) continue;
  addedDirs.add(dirPath);  // ← continue前に登録（重複防止を保証）

  const parentPath = parts.slice(0, i - 1).join("/");
  if (i > 1 && !expandedDirs.has(parentPath)) continue;  // ← ノード追加だけスキップ

  nodes.push({
    name: parts[i - 1],
    path: dirPath,
    isDir: true,
    depth: i - 1,
    isExpanded: expandedDirs.has(dirPath),
  });
}
```

---

## T2: `loadDiff` の state 更新順序修正 (`useDiffView.ts`)

**問題**: `setDiffResult(result)` → `setExpandedDirs(allDirs)` の順序で、React バッチ処理の中間レンダーで `expandedDirs` が空のまま `buildDiffTree` が評価される可能性がある。

**修正**: `expandedDirs` を先に計算・設定してから `diffResult` を設定する。`setSelectedPath` にも optional chaining 安全ガードを追加。

```typescript
// AFTER (修正)
void api.DevPanelWorkingDiff(capturedSession).then((result) => {
  if (!mountedRef.current) return;
  if (sessionRef.current !== capturedSession) return;

  // expandedDirs を diffResult より先に設定し、
  // useMemo 評価時に展開状態が確定済みであることを保証する。
  if (result.files && result.files.length > 0) {
    const allDirs = new Set<string>();
    for (const file of result.files) {
      const parts = file.path.split("/");
      for (let i = 1; i < parts.length; i++) {
        allDirs.add(parts.slice(0, i).join("/"));
      }
    }
    setExpandedDirs(allDirs);
    setSelectedPath((prev) => prev ?? result.files[0]?.path ?? null);
  }
  setDiffResult(result);
  setIsLoading(false);
}).catch((err) => {
  // ... 既存のまま
});
```

---

## T3: `DiffContentViewer.tsx` 防御的コーディング適用

### T3-a: 非nullアサーション `!` 除去 (チェックリスト #99)

```typescript
// BEFORE
<div className="diff-expand-bar" onClick={() => toggleGap(hi)}>
  Expand {parsed.gaps.get(hi)!.hiddenLineCount} hidden lines
</div>

// AFTER — 明示的ガード
{(() => {
  const gap = parsed.gaps.get(hi);
  if (!gap) return null;
  return (
    <div className="diff-expand-bar" onClick={() => toggleGap(hi)}>
      Expand {gap.hiddenLineCount} hidden lines
    </div>
  );
})()}
```

実際には条件部 `parsed.gaps.has(hi)` で保護されているが、`!` を使わないルールに従う。
ただし即時関数は冗長なので、条件部と統合してローカル変数で解決する：

```typescript
// AFTER (推奨: 条件統合)
{!expandedGaps.has(hi) && (() => {
  const gap = parsed.gaps.get(hi);
  return gap ? (
    <div className="diff-expand-bar" onClick={() => toggleGap(hi)}>
      Expand {gap.hiddenLineCount} hidden lines
    </div>
  ) : null;
})()}
```

---

## 修正順序

```
T1 (addedDirs管理) + T2 (state順序) — 並行修正可（同一ファイル内だが独立箇所）
  ↓
T3 (DiffContentViewer防御的コーディング) — 独立
  ↓
ビルド確認 + 動作確認
```

---

## チェックリスト走査結果（該当項目のみ）

| # | チェック | 状態 | 対応 |
|---|---------|------|------|
| 84 | catch内でsetError等でユーザー通知 | OK | 既存の `.catch` で `setError(String(err))` 実施済み |
| 85 | API失敗時のフォールバック値が安全側 | OK | `setError` でエラー表示、データはnull維持 |
| 90 | index を React key に使っていないか | 許容 | diff hunk/line は file 変更時に全再構築。独立した追加削除なし。index key で問題なし |
| 94 | useMemo 内で副作用なし | OK | `buildDiffTree` は純粋関数 |
| 95 | async handler に try/catch or .catch | OK | `loadDiff` は `.catch()` チェーン済み |
| 99 | 非nullアサーション `!` 禁止 | NG → T3 | `parsed.gaps.get(hi)!` を除去 |
| 103 | useCallback 依存配列の整合性 | OK | `toggleDir: []`, `selectFile: []`, `loadDiff: [activeSession]` — 整合的 |
| 104 | エンティティ切替時の状態リセット | OK | session変更時に `setExpandedDirs(new Set())` 等リセット済み |
| 105 | setState updater 内で副作用なし | OK | `toggleDir` の updater は純粋 |
| 109 | 同じロジックが2箇所以上→ヘルパー抽出 | 確認 | allDirs計算ロジックは `loadDiff` 内1箇所のみ。抽出不要 |

---

## 検証方法

1. **ビルド**: `cd myT-x/frontend && npm run build` でコンパイルエラーなし
2. **手動テスト**:
   - サブディレクトリ配下にファイル変更がある状態で Diff ビューを開く
   - ディレクトリノードが表示され、展開/折りたたみが動作する
   - 折りたたみ後に再展開してファイルが再表示される
   - セッション切替後に新セッションで正常展開される
3. **Expand hidden lines**: DiffContentViewer のギャップ展開が正常動作する
