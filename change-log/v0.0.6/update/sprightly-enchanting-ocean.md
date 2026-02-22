# File Tree View: Path Copy Button & Content Copy Features

## Context

File Tree ビューでファイル内容を閲覧する際、パスやコード内容をクリップボードにコピーする手段がない。
Claude Code 等への指示で「このファイルの○○行を修正して」と伝える際にパスのコピーが頻繁に必要になる。
ターミナルペインには既に Ctrl+C コピー・マウス選択コピーが実装されているため、同じパターンを File Content Viewer にも適用する。

## 変更対象ファイル

| File | 変更内容 |
|------|----------|
| `myT-x/frontend/src/components/viewer/views/file-tree/FileContentViewer.tsx` | 3機能追加 |
| `myT-x/frontend/src/styles/viewer.css` | コピーボタン・テキスト選択スタイル追加 |

## 実装内容

### 1. パスコピーボタン (FileContentViewer ヘッダー)

`.file-content-header` 内の `file-content-path` と `file-content-size` の間にコピーボタンを配置。

- クリップボードアイコン SVG (14x14) → クリック → チェックマークに 1.5s 切替
- `ClipboardSetText(content.path)` で Wails ランタイム経由コピー
- `useState<boolean>` で copied 状態管理、ファイル切替時にリセット

### 2. Ctrl+C コピー (ファイル本文エリア)

- `.file-content-body` に `ref` + `tabIndex={0}` を付与（キーボードイベント受信用）
- `keydown` リスナーで `Ctrl+C` を検知
- `window.getSelection().toString()` でブラウザ選択テキストを取得 → `ClipboardSetText` でコピー
- 選択なしの場合は何もしない（ターミナルと違い SIGINT 不要）

### 3. マウス選択時自動コピー (Copy-on-Select)

ターミナルの `useTerminalEvents.ts` と同一パターン：

- `document.addEventListener("selectionchange", ...)` でリスニング
- `el.contains(selection.anchorNode)` で `.file-content-body` 内の選択のみに限定
- 100ms デバウンスで `ClipboardSetText` 呼び出し
- アンマウント時にタイマークリーンアップ

### CSS 追加

- `.file-content-copy-path` : 24x24 ボタン、`viewer-header-btn` 同等スタイル
- `.file-content-body` : `user-select: text; cursor: text;`
- `.file-content-body:focus` : `outline: none;`
- `.file-content-body ::selection` : テーマ色でハイライト

## 再利用する既存パターン

| パターン | 参照元 |
|---------|--------|
| `ClipboardSetText` + DEV-only error logging | `src/hooks/useTerminalEvents.ts:126` |
| 100ms debounce copy-on-select | `src/hooks/useTerminalEvents.ts:119-133` |
| SVG アイコン (stroke/currentColor) | `src/components/viewer/icons/FileTreeIcon.tsx` |
| ボタンスタイル | `styles/viewer.css` `.viewer-header-btn` |
| Import path | `../../../../../wailsjs/runtime/runtime` |

## 実装手順

1. `viewer.css` にスタイル追加
2. `FileContentViewer.tsx` に import・state・ref・3機能のハンドラ・JSX 変更を実装
3. `frontend-design` スキル参照でボタンの視覚品質を確認
4. `defensive-coding-checklist` 走査
5. `self-review` 実施

## 検証方法

1. File Tree ビューでファイルを選択
2. ヘッダーのコピーボタンクリック → アイコンがチェックマークに変化 → テキストエディタに貼り付けてパスが正しいことを確認
3. ファイル本文をマウスドラッグで選択 → 別の場所に貼り付けて選択テキストがコピーされていることを確認
4. ファイル本文をクリック → テキスト選択 → Ctrl+C → 貼り付けて確認
5. 行番号が選択に含まれないことを確認
6. 別ファイルに切替 → コピーボタンがリセットされることを確認
