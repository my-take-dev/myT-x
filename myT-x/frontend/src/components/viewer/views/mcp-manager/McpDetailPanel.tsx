import {useI18n} from "../../../../i18n";
import type {MCPSnapshot} from "../../../../types/mcp";
import {
    buildCliExamples,
    buildLspMcpLaunchRecommendation,
    type CliExample,
    type CliExampleID,
    lspMcpConfigServerName,
    lspMcpNamePlaceholder,
    resolveBridgeCommand,
} from "./mcpConfigSnippets";
import {useClipboardCopyFeedback} from "./useClipboardCopyFeedback";

interface McpDetailPanelProps {
    representativeMCP: MCPSnapshot | null;
    activeSession: string | null;
    totalLspCount: number;
}

export function McpDetailPanel({
    representativeMCP,
    activeSession,
    totalLspCount,
}: McpDetailPanelProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const normalizedSession = activeSession?.trim() ?? "";
    if (normalizedSession === "") {
        return (
            <div className="mcp-detail-empty">
                {tr(
                    "viewer.mcpDetail.selectSession",
                    "アクティブなセッションを選択して、LSP-MCP起動例を生成してください。",
                    "Select an active session to generate LSP-MCP launch examples.",
                )}
            </div>
        );
    }
    if (totalLspCount === 0) {
        return (
            <div className="mcp-detail-empty">
                {tr(
                    "viewer.mcpDetail.noProfiles",
                    "このセッションで利用可能なLSP-MCPプロファイルはありません。",
                    "No LSP-MCP profiles are available in this session.",
                )}
            </div>
        );
    }

    const bridgeRecommendation = buildLspMcpLaunchRecommendation(resolveBridgeCommand(representativeMCP));
    const cliExamples = bridgeRecommendation == null ? [] : buildCliExamples(bridgeRecommendation);

    return (
        <div className="mcp-detail-panel">
            <div className="mcp-detail-left">
                <h3 className="mcp-detail-title">LSP-MCP</h3>
                <p className="mcp-detail-description">
                    {tr(
                        "viewer.mcpDetail.description.runtime",
                        "myT-xはセッションごとにLSP-MCPを提供し、CLIクライアント接続時に要求された言語サーバーを起動します。",
                        "myT-x provides LSP-MCP per session and launches requested language servers when a CLI client connects.",
                    )}
                </p>
                <p className="mcp-detail-description">
                    {tr(
                        "viewer.mcpDetail.description.target",
                        `対象のLSPは --mcp ${lspMcpNamePlaceholder} で指定します。ブリッジが要求されたLSPをオンデマンドで解決・起動するため、言語ごとの手動トグルは不要です。`,
                        `Specify the target LSP with --mcp ${lspMcpNamePlaceholder}. The bridge resolves and starts requested LSPs on demand, so manual per-language toggles are unnecessary.`,
                    )}
                </p>
                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.mcpDetail.section.sessionBehavior", "セッション動作", "Session behavior")}
                    </h4>
                    <p className="mcp-detail-description">
                        {tr(
                            "viewer.mcpDetail.registeredBuiltinProfiles",
                            "登録済みビルトインプロファイル数",
                            "Registered built-in profile count",
                        )}: <code>{String(totalLspCount)}</code>
                    </p>
                    <p className="mcp-detail-description">
                        {tr("viewer.mcpDetail.clientConfigKey", "設定例のクライアント設定キー", "Client config key in examples")}: <code>{lspMcpConfigServerName}</code>
                    </p>
                </div>
                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.mcpDetail.section.bridgeTemplate", "ブリッジコマンドテンプレート", "Bridge command template")}
                    </h4>
                    {bridgeRecommendation == null ? (
                        <p className="mcp-detail-description">
                            {tr(
                                "viewer.mcpDetail.bridgeMetadataMissing",
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
                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.mcpDetail.section.notes", "備考", "Notes")}
                    </h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>
                            {tr(
                                "viewer.mcpDetail.notesBody",
                                `${lspMcpNamePlaceholder} を起動したいサーバー名に置き換えてください（例: gopls, pyright-langserver, rust-analyzer）。
Named PipeエンドポイントはmyT-x内部で解決されます。
1つのクライアントで複数の言語サーバーを使用する場合は、設定エントリを複製し、それぞれに固有の設定キーを付けてください。
プレビューの引用符は説明用です。お使いのシェルに合わせて調整してください。`,
                                `Replace ${lspMcpNamePlaceholder} with the server name you want to launch (for example: gopls, pyright-langserver, rust-analyzer).
Named Pipe endpoints are resolved inside myT-x.
If one client uses multiple language servers, duplicate the config entry and assign a unique key to each.
Quoted arguments in the preview are illustrative. Adjust them for your shell.`,
                            )}
                        </code>
                    </pre>
                </div>
                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.mcpDetail.section.prerequisites", "前提条件", "Prerequisites")}
                    </h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>
                            {tr(
                                "viewer.mcpDetail.prerequisitesBody",
                                `言語サーバーの実行ファイルがシステムのPATH環境変数に登録されている必要があります。
myT-xはPATHから対象の言語サーバーコマンド（例: gopls, pyright-langserver）を検索して起動します。
言語サーバーが見つからない場合は、該当するコマンドをPATHに追加してからmyT-xを再起動してください。`,
                                `Language-server executables must be available in your system PATH.
myT-x searches PATH and launches the requested language server command (for example: gopls, pyright-langserver).
If a language server is not found, add the command to PATH and restart myT-x.`,
                            )}
                        </code>
                    </pre>
                </div>
            </div>

            <div className="mcp-detail-right">
                <McpCliExamplePanel examples={cliExamples} bridgeReady={bridgeRecommendation != null}/>
            </div>
        </div>
    );
}

function McpCliExamplePanel({examples, bridgeReady}: {examples: CliExample[]; bridgeReady: boolean}) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);
    const {copiedKey, handleCopy} = useClipboardCopyFeedback<CliExampleID>("mcp-detail");

    return (
        <div className="mcp-connection-info">
            <div className="mcp-connection-section">
                <h4 className="mcp-detail-section-title mcp-cli-example-title">
                    {tr("viewer.mcpDetail.clientConfig", "クライアント設定", "Client configuration")}
                </h4>
                {bridgeReady ? (
                    <div className="mcp-connection-hint">
                        {tr(
                            "viewer.mcpDetail.clientConfigHintReady",
                            `スニペットを使用する前に ${lspMcpNamePlaceholder} を置き換えてください。`,
                            `Replace ${lspMcpNamePlaceholder} before using the snippet.`,
                        )}
                    </div>
                ) : (
                    <div className="mcp-connection-hint">
                        {tr(
                            "viewer.mcpDetail.clientConfigHintNotReady",
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
                                "viewer.mcpDetail.copyConfigTitle",
                                `${example.title} 設定をコピー`,
                                `Copy ${example.title} config`,
                            )}
                            aria-label={tr(
                                "viewer.mcpDetail.copyConfigAria",
                                `${example.title} 設定をコピー`,
                                `Copy ${example.title} config`,
                            )}
                        >
                            {copiedKey === example.id
                                ? tr("viewer.mcpDetail.copied", "コピー済", "Copied")
                                : tr("viewer.mcpDetail.copy", "コピー", "Copy")}
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
