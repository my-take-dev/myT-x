# claude-mem フック問題 診断・修正計画

## Context

claude-mem@thedotmack プラグイン（v10.2.3）が今日インストールされたが、
旧ワーカープロセス（v10.2.1）がポート37777を占有し続けているため、
全てのツールコール後の PostToolUse フックが失敗し続けている。

ユーザーが報告した「No tools needed for suggestion」は hooks によるブロックではなく、
Claude Code (AI) がツールなしで提案しようとした際のメッセージと考えられる。

## 発見した問題

### 根本原因: ポート37777の競合

| 項目 | 内容 |
|------|------|
| 旧ワーカー | PID 17816, `bun` プロセス |
| 起動時刻 | 2026-02-17 08:20:44（昨日） |
| バージョン | 10.2.1（現プラグイン: 10.2.3） |
| 状態 | ポート37777でLISTENING中 |

### 発生経緯

1. 09:20:25 - claude-mem v10.2.3 インストール完了
2. 09:20:26 - バージョン不一致検出 → 自動再起動試行
3. 再起動失敗（旧プロセスが生き残った）
4. 13:03以降 - 全 PostToolUse フックでエラー連発:
   ```
   Worker failed to start Failed to start server. Is port 37777 in use?
   ```

### 影響範囲

- claude-mem の `PostToolUse` フック（`matcher: "*"`）が全ツールコール後に実行
- 毎回ワーカー起動を試みて失敗 → タイムアウト後にエラーで終了
- メモリの記録・学習が全て機能していない状態

## 修正手順

### Step 1: 旧 bun ワーカーの終了

```powershell
# PID 17816（旧 claude-mem worker v10.2.1）を終了
Stop-Process -Id 17816 -Force
```

### Step 2: ポート解放確認

```powershell
netstat -ano | findstr ':37777'
# 出力が空になることを確認
```

### Step 3: claude-mem ワーカーの再起動

```powershell
# 新しいセッションを開始 or Claude Code を再起動すると
# SessionStart フックが自動的に新ワーカー（v10.2.3）を起動する
```

### Step 4: 動作確認

```powershell
# ログで正常起動を確認
Get-Content 'C:\Users\mytakedev\.claude-mem\logs\claude-mem-2026-02-18.log' -Tail 20
# [INFO] Worker available {"workerUrl":"http://127.0.0.1:37777"} が出ればOK
```

## 「No tools needed for suggestion」について

- **hooks によるブロックではない**（フックスクリプトにこの文字列は存在しない）
- Claude Code (AI) がレビュー修正の提案を行う際、ツール使用なしで応答しようとした際の
  AI 自身のメッセージと考えられる
- グローバル設定で `"defaultMode": "plan"` が設定されているため、
  デフォルトで plan mode になり、write 系ツールがブロックされる点は注意

## 補足: plan mode の defaultMode について

`C:\Users\mytakedev\.claude\settings.json` に `"defaultMode": "plan"` が設定されている。
全セッションが plan mode で開始するため、実装フェーズで明示的に plan mode を
解除する必要がある。これが意図した設定か確認推奨。

## 検証方法

修正後、Claude Code で任意のツールコールを実行し、
ログに `Worker available` が出力されること、
エラーが消えることを確認する。
