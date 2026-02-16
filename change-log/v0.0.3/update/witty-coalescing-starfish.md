# Plan: ドラッグ＆ドロップ時のパス表示を最適化（`\\` → `\`）

## Context

ターミナルペインにファイルをドラッグ＆ドロップすると、パスが `C:\\Users\\test\\file.txt` のように二重バックスラッシュで表示される。
原因は `quotePathForShell` 関数がUnix/bash向けのエスケープ（`\` → `\\`）を行っているため。
本アプリはWindows専用であり、PowerShell/cmd.exeではバックスラッシュはエスケープ文字ではなくパス区切り文字なので、エスケープ不要。

## 修正対象ファイル

- `myT-x/frontend/src/hooks/useFileDrop.ts` (7行目)

## 修正内容

### `quotePathForShell` 関数の修正

**現在のコード:**
```typescript
function quotePathForShell(path: string): string {
  const escaped = path.replace(/[\\"$`]/g, "\\$&");
  return `"${escaped}"`;
}
```

**修正後:**
```typescript
function quotePathForShell(path: string): string {
  // Windows: \ はパス区切り文字でありエスケープ不要
  // Windowsファイル名に " は使用不可のためエスケープ不要
  return `"${path}"`;
}
```

**理由:**
- `\` — Windowsパス区切り文字。エスケープ不要
- `"` — Windowsファイル名に使用不可。エスケープ不要
- `$` / `` ` `` — PowerShellで特殊文字だが、OSから取得するファイルパスにこれらが含まれることは実質的に無い（ネットワーク共有の `$` は末尾のみで問題にならない）
- ドラッグ＆ドロップのパスはOS提供の正規パスであり、特殊文字の心配は不要

## 検証方法

1. `npm run dev` でアプリを起動
2. ターミナルペインにファイルをドラッグ＆ドロップ
3. パスが `"C:\Users\test\file.txt"` のように単一バックスラッシュで表示されることを確認
4. スペースを含むパスのファイルもドロップして正常にクォートされることを確認
