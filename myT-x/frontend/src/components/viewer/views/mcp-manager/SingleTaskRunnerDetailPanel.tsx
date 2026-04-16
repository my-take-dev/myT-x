import {useMemo} from "react";
import {useI18n} from "../../../../i18n";
import type {MCPSnapshot} from "../../../../types/mcp";
import {
    buildCliExamples,
    buildStrMcpLaunchRecommendation,
    type CliExample,
    type CliExampleID,
    resolveBridgeCommand,
    strMcpConfigServerName,
} from "./mcpConfigSnippets";
import {useClipboardCopyFeedback} from "./useClipboardCopyFeedback";

interface SingleTaskRunnerDetailPanelProps {
    representativeMCP: MCPSnapshot | null;
    activeSession: string | null;
}

export function SingleTaskRunnerDetailPanel({representativeMCP, activeSession}: SingleTaskRunnerDetailPanelProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const normalizedSession = activeSession?.trim() ?? "";
    const strToolDescriptions = useMemo(() => [
        {
            name: "enqueue_task",
            desc: t(
                "viewer.strDetail.tool.enqueueTask",
                language === "ja" ? "タスクキューへの1〜20件の追加" : "Queue 1-20 tasks for a target pane",
            ),
        },
        {
            name: "complete_task",
            desc: t(
                "viewer.strDetail.tool.completeTask",
                language === "ja" ? "アクティブタスクの完了通知、次タスクへ進行" : "Mark active task as completed, trigger next",
            ),
        },
        {
            name: "fail_task",
            desc: t(
                "viewer.strDetail.tool.failTask",
                language === "ja" ? "アクティブタスクの失敗記録、キュー停止" : "Mark active task as failed, stop queue",
            ),
        },
        {
            name: "list_queue",
            desc: t(
                "viewer.strDetail.tool.listQueue",
                language === "ja" ? "キュースナップショットの取得" : "Return current queue snapshot",
            ),
        },
        {
            name: "cancel_task",
            desc: t(
                "viewer.strDetail.tool.cancelTask",
                language === "ja" ? "保留/アクティブタスクのキャンセル" : "Cancel pending or active tasks",
            ),
        },
        {
            name: "help",
            desc: t(
                "viewer.strDetail.tool.help",
                language === "ja" ? "使用方法ヘルプ" : "Usage help",
            ),
        },
    ], [language, t]);

    if (normalizedSession === "") {
        return (
            <div className="mcp-detail-empty">
                {tr("viewer.strDetail.selectSession", "アクティブなセッションを選択してください。", "Select an active session.")}
            </div>
        );
    }

    const bridgeRecommendation = buildStrMcpLaunchRecommendation(
        resolveBridgeCommand(representativeMCP),
    );
    const cliExamples = bridgeRecommendation == null ? [] : buildCliExamples(bridgeRecommendation, strMcpConfigServerName);

    return (
        <div className="mcp-detail-panel">
            <div className="mcp-detail-left">
                <h3 className="mcp-detail-title">Single Task Runner</h3>
                <p className="mcp-detail-description">
                    {tr(
                        "viewer.strDetail.description",
                        "1つのペインに対してタスクを順次実行する軽量MCPサーバーです。エージェントの事前登録は不要で、チャネルベースの即座完了通知によりDBポーリングなしで動作します。",
                        "A lightweight MCP server that executes queued tasks sequentially on a single pane. No agent pre-registration is needed. Channel-based instant completion signals eliminate DB polling.",
                    )}
                </p>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.strDetail.section.tools", "提供ツール", "Provided tools")}
                    </h4>
                    <ul className="orch-tool-list">
                        {strToolDescriptions.map((tool) => (
                            <li key={tool.name}>
                                <code>{tool.name}</code> — {tool.desc}
                            </li>
                        ))}
                    </ul>
                </div>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.strDetail.section.recommendedFlow", "推奨フロー", "Recommended flow")}
                    </h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>enqueue_task → UI start queue → complete_task / fail_task / cancel_task</code>
                    </pre>
                </div>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.strDetail.section.behavior", "動作仕様", "Behavior")}
                    </h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>
                            {tr(
                                "viewer.strDetail.behaviorBody",
                                `セッション単位で独立した状態を管理します。
データベースは使用せず、Goチャネルによる即座完了通知で次タスクへ進行します。
キューの開始はUIまたはWails APIから行います。
complete_task / fail_task / cancel_task の結果に応じてキューが更新されます。`,
                                `State is managed independently per session.
No database is used — Go channels provide instant completion signals to advance the queue.
Start the queue from the UI or the Wails API.
The active task is resolved through complete_task, fail_task, or cancel_task, and the queue updates based on that result.`,
                            )}
                        </code>
                    </pre>
                </div>

                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.strDetail.section.bridgeTemplate", "ブリッジコマンドテンプレート", "Bridge command template")}
                    </h4>
                    {bridgeRecommendation == null ? (
                        <p className="mcp-detail-description">
                            {tr(
                                "viewer.strDetail.bridgeMetadataMissing",
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
                <StrCliExamplePanel examples={cliExamples} bridgeReady={bridgeRecommendation != null}/>
            </div>
        </div>
    );
}

function StrCliExamplePanel({examples, bridgeReady}: { examples: CliExample[]; bridgeReady: boolean }) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);
    const {copiedKey, handleCopy} = useClipboardCopyFeedback<CliExampleID>("str-detail");

    return (
        <div className="mcp-connection-info">
            <div className="mcp-connection-section">
                <h4 className="mcp-detail-section-title mcp-cli-example-title">
                    {tr("viewer.strDetail.clientConfig", "クライアント設定", "Client configuration")}
                </h4>
                {bridgeReady ? (
                    <div className="mcp-connection-hint">
                        {tr(
                            "viewer.strDetail.clientConfigHintReady",
                            "以下のスニペットをそのままMCP設定に追加してください。",
                            "Add the snippet below directly to your MCP configuration.",
                        )}
                    </div>
                ) : (
                    <div className="mcp-connection-hint">
                        {tr(
                            "viewer.strDetail.clientConfigHintNotReady",
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
                                "viewer.strDetail.copyConfigTitle",
                                `${example.title} 設定をコピー`,
                                `Copy ${example.title} config`,
                            )}
                            aria-label={tr(
                                "viewer.strDetail.copyConfigAria",
                                `${example.title} 設定をコピー`,
                                `Copy ${example.title} config`,
                            )}
                        >
                            {copiedKey === example.id
                                ? tr("viewer.strDetail.copied", "コピー済", "Copied")
                                : tr("viewer.strDetail.copy", "コピー", "Copy")}
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
