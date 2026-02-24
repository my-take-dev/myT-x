# サイドバーアイコンの並び順変更とルール明記の計画

## 1. 概要と目的
現在、右サイドバー（`ActivityStrip.tsx` でレンダリングされるビュー切り替えアイコン）において、「入力履歴（`input-history`）」が「エラーログ（`error-log`）」の下に配置されています。また、「DIFF（`diff-view`）」が一番下に配置されるなど、意図しない順序になっています。

本計画では、以下の要件を満たすようにアイコンの並び順を修正し、今後の開発者が迷わないようにソーストコード上に厳格なルールをコメントとして明記します。

### 要件
1. **入力履歴の配置**: 「DIFF表示 (`diff-view`)」の直下に配置する。
2. **エラーログの配置**: 一番下（最後）に配置する。
3. **ルール明記1**: エラーログ表示の下にはアイコンを作成（追加）しないことをソースコード上に明記する。
4. **ルール明記2**: 右サイドバーのアイコンは、エラーログ表示を除いて上から順（import順）に配置することをソースコード上に明記する。

---

## 2. アーキテクチャ上の仕様確認
右サイドバーのアイコン順序は、`ViewerSystem.tsx` における `import` 文の実行順序によって決定されます。
各モジュールがロードされる際に `viewerRegistry.ts` の `registerView()` を呼び出し、その順に配列に登録される仕様となっています。

---

## 3. 具体的な修正手順

### 変更対象ファイル
`myT-x/frontend/src/components/viewer/ViewerSystem.tsx`

### 変更内容

ファイルの `Side-effect imports` セクションの `import` 順序とコメントを以下のように変更します。

#### 新しい import 順序とコメント
```tsx
// Side-effect imports: each view self-registers into the registry.
//
// 【サイドバーアイコンの配置ルール】
// 1. エラーログ表示以外のアイコンは、上から表示させたい順にここで import してください。
// 2. エラーログ表示 (error-log) は必ず一番下に配置してください。
// 3. エラーログ表示の下には絶対に新しいアイコンを追加（import）しないでください。

import "./views/file-tree";
import "./views/git-graph";
import "./views/diff-view";
import "./views/input-history"; // DIFFの直下に配置

// ---------------------------------------------------------
// これより下にはエラーログ表示以外のアイコンを追加しないこと
// ---------------------------------------------------------
import "./views/error-log";
```

---

## 4. 期待される結果
- 右サイドバーのアイコンが上から以下の順番に並びます。
  1. File Tree
  2. Git Graph
  3. Diff View
  4. Input History
  5. Error Log
- 今後新しいサイドバー機能（Viewer）を追加する際、開発者が `ViewerSystem.tsx` を見たときに、どこに `import` を記述すればよいかが明確になり、エラーログが常に最下部に保たれるようになります。

---

## 5. 実行方法
本計画の内容に基づき、`myT-x/frontend/src/components/viewer/ViewerSystem.tsx` の import 部分を修正してください。
