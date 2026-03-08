import {useEffect, useRef, useState} from "react";
import type {MCPStatus, MCPSnapshot} from "../../../../types/mcp";
import {writeClipboardText} from "../../../../utils/clipboardUtils";
import {notifyClipboardFailure} from "../../../../utils/notifyUtils";
import {
    buildCliExamples,
    buildLspMcpLaunchRecommendation,
    type CliExample,
    type CliExampleID,
    lspMcpConfigServerName,
    lspMcpNamePlaceholder,
    resolveBridgeCommand,
} from "./mcpConfigSnippets";

interface McpDetailPanelProps {
    representativeMCP: MCPSnapshot | null;
    activeSession: string | null;
    aggregateStatus: MCPStatus | null;
    totalLspCount: number;
}

export function McpDetailPanel({
    representativeMCP,
    activeSession,
    aggregateStatus,
    totalLspCount,
}: McpDetailPanelProps) {
    const normalizedSession = activeSession?.trim() ?? "";
    if (normalizedSession === "") {
        return (
            <div className="mcp-detail-empty">
                アクティブなセッションを選択して、LSP-MCP起動例を生成してください。
            </div>
        );
    }
    if (aggregateStatus == null) {
        return (
            <div className="mcp-detail-empty">
                このセッションで利用可能なLSP-MCPプロファイルはありません。
            </div>
        );
    }

    const bridgeRecommendation = buildLspMcpLaunchRecommendation(resolveBridgeCommand(representativeMCP), normalizedSession);
    const cliExamples = bridgeRecommendation == null ? [] : buildCliExamples(bridgeRecommendation);
    const statusDetail = describeAggregateStatus(aggregateStatus);

    return (
        <div className="mcp-detail-panel">
            <div className="mcp-detail-left">
                <h3 className="mcp-detail-title">LSP-MCP</h3>
                <div className="mcp-connection-status-row">
                    <span className={`mcp-status-dot ${aggregateStatus}`} title={aggregateStatus}/>
                    <span className="mcp-connection-label">{aggregateStatus}</span>
                    <span className="mcp-connection-hint">{normalizedSession}</span>
                </div>
                <p className="mcp-detail-description">
                    myT-xはセッションごとにLSP-MCPを提供し、CLIクライアント接続時に要求された言語サーバーを起動します。
                </p>
                <p className="mcp-detail-description">
                    対象のLSPは <code>--mcp {lspMcpNamePlaceholder}</code> で指定します。ブリッジが要求されたLSPをオンデマンドで解決・起動するため、言語ごとの手動トグルは不要です。
                </p>
                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">セッション動作</h4>
                    <p className="mcp-detail-description">
                        ステータス概要: <code>{statusDetail}</code>
                    </p>
                    <p className="mcp-detail-description">
                        登録済みビルトインプロファイル数: <code>{String(totalLspCount)}</code>
                    </p>
                    <p className="mcp-detail-description">
                        設定例のクライアント設定キー: <code>{lspMcpConfigServerName}</code>
                    </p>
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
                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">備考</h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>
{`${lspMcpNamePlaceholder} を起動したいサーバー名に置き換えてください（例: gopls, pyright-langserver, rust-analyzer）。
Named PipeエンドポイントはmyT-x内部で解決されます。
1つのクライアントで複数の言語サーバーを使用する場合は、設定エントリを複製し、それぞれに固有の設定キーを付けてください。
プレビューの引用符は説明用です。お使いのシェルに合わせて調整してください。`}
                        </code>
                    </pre>
                </div>
                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">前提条件</h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>
{`言語サーバーの実行ファイルがシステムのPATH環境変数に登録されている必要があります。
myT-xはPATHから対象の言語サーバーコマンド（例: gopls, pyright-langserver）を検索して起動します。
言語サーバーが見つからない場合は、該当するコマンドをPATHに追加してからmyT-xを再起動してください。`}
                        </code>
                    </pre>
                </div>
            </div>

            <div className="mcp-detail-right">
                <McpCliExamplePanel examples={cliExamples} bridgeReady={bridgeRecommendation != null} />
            </div>
        </div>
    );
}

function describeAggregateStatus(status: MCPStatus): string {
    switch (status) {
        case "running":
            return "少なくとも1つのLSP-MCPが実行中です";
        case "starting":
            return "LSP-MCPを起動中です";
        case "error":
            return "直近のLSP-MCP起動でエラーが発生しました";
        case "stopped":
            return "クライアント接続時に要求されたLSP-MCPが起動されます";
        default: {
            const _: never = status;
            void _;
            return "クライアント接続時に要求されたLSP-MCPが起動されます";
        }
    }
}

function McpCliExamplePanel({examples, bridgeReady}: {examples: CliExample[]; bridgeReady: boolean}) {
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
            console.warn("[mcp-detail] clipboard write failed", err);
        });
    }

    return (
        <div className="mcp-connection-info">
            <div className="mcp-connection-section">
                <h4 className="mcp-detail-section-title mcp-cli-example-title">クライアント設定</h4>
                {bridgeReady ? (
                    <div className="mcp-connection-hint">
                        スニペットを使用する前に <code>{lspMcpNamePlaceholder}</code> を置き換えてください。
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
