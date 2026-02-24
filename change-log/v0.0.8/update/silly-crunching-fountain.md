# Plan: Sidebar Icon Order Fix

## Context

`plans/v0.0.8/sidebar-icon-order-plan.md` の対応でimportの順序は正しくなったが、
`input-history/index.ts` に `position: "bottom"` が設定されているため、
ActivityStrip のフィルタリングにより error-log と同じ「下部グループ」に分類されてしまっている。

**現在の表示順:**
file-tree → git-graph → diff → [spacer] → input-history → error-log

**期待する表示順:**
file-tree → git-graph → diff → input-history → [spacer] → error-log

また、今後も同様の問題が発生しないよう、`position: "bottom"` は error-log 専用である旨をコードで明示する。

## 修正方針

### 仕組みの確認

- `viewerRegistry.ts`: `ViewPlugin` の `position` フィールド（"top" | "bottom"）
- `ActivityStrip.tsx`: `topViews` / `bottomViews` に分けて描画。bottomViews の上に spacer を挿入
- `ViewerSystem.tsx`: import 順序がアイコン並び順を決定。コメントでルールを記載済み
- **唯一の fix 箇所**: `input-history/index.ts` の `position: "bottom"` を削除するだけ

### Step 1: input-history の position を修正

**ファイル:** `myT-x/frontend/src/components/viewer/views/input-history/index.ts`

`position: "bottom"` を削除する（省略時は "top" にデフォルト）。

### Step 2: ViewerSystem.tsx のコメント強化

**ファイル:** `myT-x/frontend/src/components/viewer/ViewerSystem.tsx`

既存のコメントに以下を追記し、今後の誤用を防ぐ：
- `position: "bottom"` は error-log 専用である旨
- 新しいビューは position を省略（= "top"）するルール

### Step 3: viewerRegistry.ts にルールコメントを追加

**ファイル:** `myT-x/frontend/src/components/viewer/viewerRegistry.ts`

`ViewPlugin` interface の `position` フィールドに JSDoc コメントで使用ルールを明記する。

## 修正対象ファイル

| ファイル | 変更内容 |
|--------|---------|
| `myT-x/frontend/src/components/viewer/views/input-history/index.ts` | `position: "bottom"` を削除 |
| `myT-x/frontend/src/components/viewer/ViewerSystem.tsx` | コメント強化（position ルールの明記） |
| `myT-x/frontend/src/components/viewer/viewerRegistry.ts` | ViewPlugin.position の JSDoc コメント追加 |

## 検証方法

1. アプリをビルドして起動
2. 右サイドバーのアイコン順序を確認:
   - 上から: File Tree → Git Graph → Diff → Input History
   - spacer
   - 下部固定: Error Log
3. input-history のバッジ（unread count）が正常に動作することを確認
4. error-log が依然として最下部に固定されていることを確認
