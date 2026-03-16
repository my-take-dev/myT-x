import {useEffect, useRef, useState} from "react";
import {useI18n} from "../../../../i18n";
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

export function OrchestratorDetailPanel({representativeMCP, activeSession}: OrchestratorDetailPanelProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const normalizedSession = activeSession?.trim() ?? "";
    if (normalizedSession === "") {
        return (
            <div className="mcp-detail-empty">
                {tr("viewer.orchDetail.selectSession", "アクティブなセッションを選択してください。", "Select an active session.")}
            </div>
        );
    }

    const bridgeRecommendation = buildOrchMcpLaunchRecommendation(
        resolveBridgeCommand(representativeMCP),
    );
    const cliExamples = bridgeRecommendation == null ? [] : buildCliExamples(bridgeRecommendation, orchMcpConfigServerName);
    const orchToolDescriptions = [
        {
            name: "register_agent",
            desc: tr(
                "viewer.orchDetail.tool.registerAgent",
                "エージェント名とペインIDの登録",
                "Register an agent name and pane ID",
            ),
        },
        {
            name: "list_agents",
            desc: tr(
                "viewer.orchDetail.tool.listAgents",
                "登録済みエージェント一覧の取得",
                "Get the list of registered agents",
            ),
        },
        {
            name: "send_task",
            desc: tr(
                "viewer.orchDetail.tool.sendTask",
                "他エージェントへのタスク送信",
                "Send a task to another agent",
            ),
        },
        {
            name: "get_my_tasks",
            desc: tr(
                "viewer.orchDetail.tool.getMyTasks",
                "自分宛タスクの確認",
                "Check tasks assigned to yourself",
            ),
        },
        {
            name: "send_response",
            desc: tr(
                "viewer.orchDetail.tool.sendResponse",
                "タスクへの返信（完了記録）",
                "Reply to a task (record completion)",
            ),
        },
        {
            name: "check_tasks",
            desc: tr(
                "viewer.orchDetail.tool.checkTasks",
                "全タスクの状態監視",
                "Monitor the status of all tasks",
            ),
        },
        {
            name: "capture_pane",
            desc: tr(
                "viewer.orchDetail.tool.capturePane",
                "他ペインの表示内容取得",
                "Capture output from another pane",
            ),
        },
    ];

    return (
        <div className="mcp-detail-panel">
            <div className="mcp-detail-left">
                <h3 className="mcp-detail-title">Agent Orchestrator</h3>
                <p className="mcp-detail-description">
                    {tr(
                        "viewer.orchDetail.description",
                        "tmux上の複数AIエージェント間でタスク送信・返信・状態管理を行うMCPサーバーです。各AIツールのMCP設定にブリッジコマンドを追加するだけで利用を開始できます。",
                        "An MCP server that sends tasks, receives responses, and tracks task state across multiple AI agents on tmux. You can start using it by adding the bridge command to each AI tool's MCP configuration.",
                    )}
                </p>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.orchDetail.section.tools", "提供ツール", "Provided tools")}
                    </h4>
                    <ul className="orch-tool-list">
                        {orchToolDescriptions.map((tool) => (
                            <li key={tool.name}>
                                <code>{tool.name}</code> — {tool.desc}
                            </li>
                        ))}
                    </ul>
                </div>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.orchDetail.section.recommendedFlow", "推奨フロー", "Recommended flow")}
                    </h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>register_agent → list_agents → send_task → get_my_tasks → send_response → check_tasks</code>
                    </pre>
                </div>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.orchDetail.section.behavior", "動作仕様", "Behavior")}
                    </h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>
                            {tr(
                                "viewer.orchDetail.behaviorBody",
                                `各クライアント接続ごとに独立したランタイムが起動します。
SQLite共有ストレージでエージェント情報とタスク状態を同期します。
ペインIDは毎回現在値を使用します。`,
                                `An isolated runtime starts for each client connection.
Agent information and task state are synchronized through shared SQLite storage.
Pane IDs always use the current value at execution time.`,
                            )}
                        </code>
                    </pre>
                </div>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.orchDetail.section.bridgeTemplate", "ブリッジコマンドテンプレート", "Bridge command template")}
                    </h4>
                    {bridgeRecommendation == null ? (
                        <p className="mcp-detail-description">
                            {tr(
                                "viewer.orchDetail.bridgeMetadataMissing",
                                "現在のスナップショットにブリッジコマンドのメタデータがありません。更新ボタンで再読み込みしてください。",
                                "No bridge-command metadata was found in the current snapshot. Refresh to reload it.",
                            )}
                        </p>
                    ) : (
                        <pre className="mcp-detail-usage-pre">
                            <code>{bridgeRecommendation.commandPreview}</code>
                        </pre>
                    )}
                </div>
            </div>

            <div className="mcp-detail-right">
                <OrchCliExamplePanel examples={cliExamples} bridgeReady={bridgeRecommendation != null}/>
            </div>
        </div>
    );
}

function OrchCliExamplePanel({examples, bridgeReady}: {examples: CliExample[]; bridgeReady: boolean}) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);
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
                <h4 className="mcp-detail-section-title mcp-cli-example-title">
                    {tr("viewer.orchDetail.clientConfig", "クライアント設定", "Client configuration")}
                </h4>
                {bridgeReady ? (
                    <div className="mcp-connection-hint">
                        {tr(
                            "viewer.orchDetail.clientConfigHintReady",
                            "以下のスニペットをそのままMCP設定に追加してください。",
                            "Add the snippet below directly to your MCP configuration.",
                        )}
                    </div>
                ) : (
                    <div className="mcp-connection-hint">
                        {tr(
                            "viewer.orchDetail.clientConfigHintNotReady",
                            "スニペットをコピーする前に、ビューを更新してブリッジコマンドのメタデータを読み込んでください。",
                            "Refresh the view to load bridge-command metadata before copying snippets.",
                        )}
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
                            title={tr(
                                "viewer.orchDetail.copyConfigTitle",
                                `${example.title} 設定をコピー`,
                                `Copy ${example.title} config`,
                            )}
                            aria-label={tr(
                                "viewer.orchDetail.copyConfigAria",
                                `${example.title} 設定をコピー`,
                                `Copy ${example.title} config`,
                            )}
                        >
                            {copiedKey === example.id
                                ? tr("viewer.orchDetail.copied", "コピー済", "Copied")
                                : tr("viewer.orchDetail.copy", "コピー", "Copy")}
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
