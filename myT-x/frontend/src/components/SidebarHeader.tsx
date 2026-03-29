import {useI18n} from "../i18n";

interface SidebarHeaderProps {
    readonly onNewSession: () => void;
}

export function SidebarHeader({onNewSession}: SidebarHeaderProps) {
    const {language, t} = useI18n();
    return (
        <>
            <div className="sidebar-header">
                <h1>myT-x</h1>
                <p>
                    {language === "en"
                        ? "Terminal Multiplexer"
                        : t("sidebar.subtitle", "ターミナルマルチプレクサ")}
                </p>
            </div>

            <div className="sidebar-actions">
                <button
                    type="button"
                    className="primary"
                    onClick={onNewSession}
                >
                    {language === "en"
                        ? "+ New Session"
                        : t("sidebar.action.newSession", "+ 新規セッション")}
                </button>
            </div>
        </>
    );
}
