import {useEffect, useRef, useState} from "react";
import type {MCPSnapshot} from "../../../../types/mcp";
import {writeClipboardText} from "../../../../utils/clipboardUtils";
import {notifyClipboardFailure} from "../../../../utils/notifyUtils";
import {
    buildCliExamples,
    buildOrchMcpLaunchRecommendation,
    type CliExample,
    type CliExampleID,
    orchMcpConfigServerName,
    resolveBridgeCommand,
} from "./mcpConfigSnippets";

interface OrchestratorDetailPanelProps {
    representativeMCP: MCPSnapshot | null;
    activeSession: string | null;
}

const orchToolDescriptions = [
    {name: "register_agent", desc: "エージェント名とペインIDの登録"},
    {name: "list_agents", desc: "登録済みエージェント一覧の取得"},
    {name: "send_task", desc: "他エージェントへのタスク送信"},
    {name: "get_my_tasks", desc: "自分宛タスクの確認"},
    {name: "send_response", desc: "タスクへの返信（完了記録）"},
    {name: "check_tasks", desc: "全タスクの状態監視"},
    {name: "capture_pane", desc: "他ペインの表示内容取得"},
];

export function OrchestratorDetailPanel({representativeMCP, activeSession}: OrchestratorDetailPanelProps) {
    const normalizedSession = activeSession?.trim() ?? "";
    if (normalizedSession === "") {
        return (
            <div className="mcp-detail-empty">
                アクティブなセッションを選択してください。
            </div>
        );
    }

    const bridgeRecommendation = buildOrchMcpLaunchRecommendation(
        resolveBridgeCommand(representativeMCP),
        normalizedSession,
    );
    const cliExamples = bridgeRecommendation == null ? [] : buildCliExamples(bridgeRecommendation, orchMcpConfigServerName);

    return (
        <div className="mcp-detail-panel">
            <div className="mcp-detail-left">
                <h3 className="mcp-detail-title">Agent Orchestrator</h3>
                <p className="mcp-detail-description">
                    tmux上の複数AIエージェント間でタスク送信・返信・状態管理を行うMCPサーバーです。各AIツールのMCP設定にブリッジコマンドを追加するだけで利用を開始できます。
                </p>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">提供ツール</h4>
                    <ul className="orch-tool-list">
                        {orchToolDescriptions.map((tool) => (
                            <li key={tool.name}>
                                <code>{tool.name}</code> — {tool.desc}
                            </li>
                        ))}
                    </ul>
                </div>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">推奨フロー</h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>register_agent → list_agents → send_task → get_my_tasks → send_response → check_tasks</code>
                    </pre>
                </div>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">動作仕様</h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>
{`各クライアント接続ごとに独立したランタイムが起動します。
SQLite共有ストレージでエージェント情報とタスク状態を同期します。
ペインIDは毎回現在値を使用します。`}
                        </code>
                    </pre>
                </div>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">ブリッジコマンドテンプレート</h4>
                    {bridgeRecommendation == null ? (
                        <p className="mcp-detail-description">
                            現在のスナップショットにブリッジコマンドのメタデータがありません。更新ボタンで再読み込みしてください。
                        </p>
                    ) : (
                        <pre className="mcp-detail-usage-pre">
                            <code>{bridgeRecommendation.commandPreview}</code>
                        </pre>
                    )}
                </div>
            </div>

            <div className="mcp-detail-right">
                <OrchCliExamplePanel examples={cliExamples} bridgeReady={bridgeRecommendation != null} />
            </div>
        </div>
    );
}

function OrchCliExamplePanel({examples, bridgeReady}: {examples: CliExample[]; bridgeReady: boolean}) {
    const [copiedKey, setCopiedKey] = useState<CliExampleID | null>(null);
    const isMountedRef = useRef(true);
    const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    useEffect(() => {
        isMountedRef.current = true;
        return () => {
            isMountedRef.current = false;
            if (timerRef.current != null) {
                clearTimeout(timerRef.current);
                timerRef.current = null;
            }
        };
    }, []);

    function handleCopy(value: string, key: CliExampleID) {
        void writeClipboardText(value).then(() => {
            if (!isMountedRef.current) {
                return;
            }
            setCopiedKey(key);
            if (timerRef.current != null) {
                clearTimeout(timerRef.current);
            }
            timerRef.current = setTimeout(() => setCopiedKey(null), 2000);
        }).catch((err: unknown) => {
            if (!isMountedRef.current) {
                return;
            }
            notifyClipboardFailure();
            console.warn("[orch-detail] clipboard write failed", err);
        });
    }

    return (
        <div className="mcp-connection-info">
            <div className="mcp-connection-section">
                <h4 className="mcp-detail-section-title mcp-cli-example-title">クライアント設定</h4>
                {bridgeReady ? (
                    <div className="mcp-connection-hint">
                        以下のスニペットをそのままMCP設定に追加してください。
                    </div>
                ) : (
                    <div className="mcp-connection-hint">
                        スニペットをコピーする前に、ビューを更新してブリッジコマンドのメタデータを読み込んでください。
                    </div>
                )}
            </div>
            {examples.map((example) => (
                <div className="mcp-connection-section" key={example.id}>
                    <div className="mcp-pipe-path-row">
                        <h4 className="mcp-detail-section-title mcp-cli-example-title">{example.title}</h4>
                        <button
                            className="mcp-copy-btn"
                            onClick={() => handleCopy(example.snippet, example.id)}
                            title={`${example.title} 設定をコピー`}
                            aria-label={`${example.title} 設定をコピー`}
                        >
                            {copiedKey === example.id ? "コピー済" : "コピー"}
                        </button>
                    </div>
                    <div className="mcp-connection-hint">{example.configPath}</div>
                    <pre className="mcp-detail-usage-pre">
                        <code>{example.snippet}</code>
                    </pre>
                </div>
            ))}
        </div>
    );
}
