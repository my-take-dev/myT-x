# File Tree ビュー: パスコピー + テキストコピー機能追加

## Context

File Tree ビューでファイルを閲覧した際、パスやファイル内容をコピーして他の場所（Claude Code への指示など）に
貼り付けたいケースが頻繁に発生する。現在の FileContentViewer にはコピー機能が一切ないため、
パスコピーボタンと、ターミナルと同等のテキスト選択コピー機能を追加する。

---

## 変更対象ファイル

| ファイル | 変更内容 |
|---------|---------|
| `frontend/src/components/viewer/views/file-tree/FileContentViewer.tsx` | パスコピーボタン追加 + Ctrl+C / 選択コピー機能追加 |
| `frontend/src/styles/viewer.css` | コピーボタン用CSS追加 |

---

## 実装詳細

### 1. パスコピーボタン（ヘッダー部）

**場所**: `.file-content-header` 内の `.file-content-path` の隣

**動作**:
- ボタンクリック → `ClipboardSetText(content.path)` でパスをクリップボードにコピー
- コピー成功時: ボタンアイコンを一時的に変更（コピーアイコン → チェックマーク、1.5秒後に戻る）
- 既存パターン再利用: `ClipboardSetText` from `wailsjs/runtime/runtime`（ターミナルと同じ）

```tsx
// FileContentViewer.tsx のヘッダー部
<div className="file-content-header">
  <span className="file-content-path">{content.path}</span>
  <button
    className="file-content-copy-path-btn"
    onClick={handleCopyPath}
    title="Copy path"
  >
    {pathCopied ? "\u2713" : "\uD83D\uDCCB"}  // チェック or クリップボードアイコン
  </button>
  <span className="file-content-size">...</span>
</div>
```

### 2. テキスト選択コピー（Ctrl+C）

**場所**: `.file-content-body` にキーボードイベントハンドラを追加

**動作**:
- `.file-content-body` にフォーカス管理（`tabIndex={0}` + ref）
- `Ctrl+C` 押下時: `window.getSelection()?.toString()` で選択テキストを取得
- 選択テキストがあれば `ClipboardSetText(text)` → ブラウザデフォルト阻止
- 選択テキストがなければ何もしない（デフォルト動作に任せる）
- ターミナルの `useTerminalEvents.ts:189-202` と同じ Smart Ctrl+C パターン

### 3. 選択自動コピー（マウスで範囲選択後）

**動作**:
- `.file-content-body` の `mouseup` イベントで `window.getSelection()?.toString()` を取得
- テキストがあれば 100ms debounce 後に `ClipboardSetText(text)`
- ターミナルの `useTerminalEvents.ts:119-133` と同じ copy-on-select パターン

### 4. CSS追加

```css
.file-content-copy-path-btn {
  /* viewer-header-btn と同系統の小さめボタン */
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--fg-dim);
  cursor: pointer;
  padding: 2px 6px;
  font-size: 0.78rem;
  transition: color 0.15s, background 0.15s;
}

.file-content-copy-path-btn:hover {
  color: var(--fg-main);
  background: var(--accent-06);
}

.file-content-copy-path-btn.copied {
  color: rgba(61, 228, 183, 0.9); /* 成功色 = session-running 色 */
}
```

---

## 再利用する既存コード

| パターン | ファイル | 用途 |
|---------|---------|------|
| `ClipboardSetText` | `wailsjs/runtime/runtime` | クリップボード書き込み |
| Smart Ctrl+C | `hooks/useTerminalEvents.ts:189-202` | 選択テキストのコピーパターン |
| Copy-on-Select | `hooks/useTerminalEvents.ts:119-133` | 100ms debounce 自動コピー |
| エラーハンドリング | `hooks/useTerminalEvents.ts` | `.catch()` + `[DEBUG-copy]` ログ |

---

## 検証方法

1. **パスコピーボタン**: File Tree でファイル選択 → コピーボタンクリック → テキストエディタに貼り付けてパスが出ること
2. **Ctrl+C コピー**: ファイル内容の一部をマウスで選択 → Ctrl+C → 貼り付けて内容が出ること
3. **選択自動コピー**: ファイル内容の一部をマウスで選択（mouseup時点でコピー） → 貼り付けて内容が出ること
4. **ビルド確認**: `cd myT-x/frontend && npm run build`
