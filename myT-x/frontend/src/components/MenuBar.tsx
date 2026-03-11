import {useI18n} from "../i18n";

interface MenuBarProps {
  onOpenSettings: () => void;
}

export function MenuBar({ onOpenSettings }: MenuBarProps) {
  const {language, setLanguage, t} = useI18n();

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
