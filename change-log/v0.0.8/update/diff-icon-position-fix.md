# Diff アイコン位置修正

## 問題

`plans/v0.0.7/アイコン配置変更.md` の実装でエラーログアイコンを右サイドバー最下部に移動した際、
Diff アイコンも誤って一緒に最下部へ移動してしまっていた。

## 原因

`myT-x/frontend/src/components/viewer/views/diff-view/index.ts` のビュー登録に
誤って `position: "bottom"` が設定されていた。

```typescript
// 誤った状態
registerView({
  id: "diff",
  icon: DiffIcon,
  label: "Diff",
  component: DiffView,
  shortcut: "Ctrl+Shift+D",
  position: "bottom", // ← これが原因
});
```

## 修正内容

### 対象ファイル
- `myT-x/frontend/src/components/viewer/views/diff-view/index.ts`

### 変更内容
`position: "bottom"` を削除。`position` 未指定のデフォルトは `"top"` のため、
Diff アイコンは git graph の下（上部グループ）に正しく配置される。

```typescript
// 修正後
registerView({
  id: "diff",
  icon: DiffIcon,
  label: "Diff",
  component: DiffView,
  shortcut: "Ctrl+Shift+D",
});
```

## 期待される結果

| アイコン | 配置 |
|---------|------|
| Git Graph, Diff（など上部グループ） | 上部 |
| Error Log | 最下部（スペーサーで分離） |
