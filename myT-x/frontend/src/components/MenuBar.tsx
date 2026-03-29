import {useI18n} from "../i18n";
import {useTmuxStore} from "../stores/tmuxStore";

interface MenuBarProps {
  onOpenSettings: () => void;
}

export function MenuBar({ onOpenSettings }: MenuBarProps) {
  const {language, setLanguage, t} = useI18n();
  const triggerImeReset = useTmuxStore((s) => s.triggerImeReset);

  return (
    <nav className="menu-bar">
      <div className="menu-bar-group">
        <div className="menu-bar-item">
          <button className="menu-bar-trigger" onClick={onOpenSettings}>
            {t("menu.settings", "設定")}
          </button>
        </div>
      </div>
      <div className="menu-bar-group menu-bar-group--end">
        <div className="menu-bar-item">
          <button
            type="button"
            className="menu-bar-trigger menu-bar-ime-reset"
            title={t("menu.imeReset.title", "IME リセット (入力変換の修復)")}
            aria-label={t("menu.imeReset.aria", "IME リセット")}
            onClick={triggerImeReset}
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5">
              <text x="1" y="10" fill="currentColor" stroke="none" fontSize="9" fontFamily="sans-serif">あ</text>
              <path d="M11 3.5a3 3 0 1 1-1.5-1.2" strokeLinecap="round"/>
              <polyline points="9.5,1.2 9.5,2.8 11,2.8" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </button>
        </div>
        <div className="menu-bar-item menu-bar-language">
          <span className="menu-bar-label">{t("menu.language", "Language")}</span>
          <div className="menu-bar-segmented" role="group" aria-label={t("menu.language", "Language")}>
            <button
              type="button"
              className={`menu-bar-trigger menu-bar-segment ${language === "ja" ? "active" : ""}`}
              onClick={() => setLanguage("ja")}
              aria-pressed={language === "ja"}
            >
              {t("menu.language.japanese", "日本語")}
            </button>
            <button
              type="button"
              className={`menu-bar-trigger menu-bar-segment ${language === "en" ? "active" : ""}`}
              onClick={() => setLanguage("en")}
              aria-pressed={language === "en"}
            >
              {t("menu.language.english", "English")}
            </button>
          </div>
        </div>
      </div>
    </nav>
  );
}
