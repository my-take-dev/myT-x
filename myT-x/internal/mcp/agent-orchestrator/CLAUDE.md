# Agent Orchestrator Guidelines

**厳守製品仕様**
製品固有仕様に対する以下を厳守する事

- `send_task`: 全エージェントが送信可能（orchestrator限定にしない）
- `capture_pane`: 全エージェントが利用可能（orchestrator限定にしない）
- `list_all_tasks`: 全エージェントが利用可能（orchestrator限定にしない）
- `register_agent`: 誰でも登録・更新が可能（自ペイン限定にしない）。
- エージェント間通信: 各エージェントが必要に応じて直接相談・通信できる（orchestrator経由を強制しない）

## 認可モデル（Authorization Model）

本サブパッケージの認可判定は以下のモデルに従う。**厳格化する場合は上記仕様の変更と `/ACCEPTED_DESIGN_DECISIONS.md` の AD-001〜004 エントリ更新を同一 PR で実施すること。**

- **Trusted caller**: pipe bridge 経由で接続した caller（`resolveCaller` が TMUX_PANE 解決不能時に `domain.Agent{Name: "_trusted"}` を返す）。
  - **任意の assignee 名で task / response 操作が可能**（`authorizeAssigneeCaller` が `IsTrustedCaller` を無条件許可する）。
  - これは「エージェント間通信は直接可」仕様の帰結であり、pipe bridge / TMUX_PANE 不明環境を機能させるために必要。
- **Non-trusted caller**: TMUX_PANE 解決済みの通常 caller。自分の `pane_id` / `agent_name` と一致する assignee のみ許可。
- **`logTrustedCallerInstanceHint` (= 旧 `recoverTrustedCallerByInstance`)**: MCP instance ID → registered agent 対応付けを**デバッグログに残すための observability-only 関数**。返り値を authorization 判定に用いてはならない。この関数名に `recover` が含まれる旧名は「認可に効かせる」と誤読されやすいため rename 済み。
- **`register_agent` の pane 限定なし**: 他エージェントの代理登録も許容（orchestrator 的運用）。ただし空 PaneID 登録は validation で弾く（認可と入力 validation は別論点）。

関連: `/ACCEPTED_DESIGN_DECISIONS.md` AD-001 / AD-002 / AD-004。



