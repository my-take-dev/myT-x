import type {Ref} from "react";
import {useI18n} from "../i18n";
import {useTmuxStore} from "../stores/tmuxStore";
import {QUICK_SEARCH_DIALOG_ID, QUICK_SEARCH_SHORTCUT_DISPLAY} from "./quickSearchShared";

export interface MenuBarProps {
  onOpenSettings: () => void;
  onOpenQuickSearch: () => void;
  isQuickSearchOpen: boolean;
  quickSearchTriggerRef?: Ref<HTMLButtonElement>;
}

export function MenuBar({
  onOpenSettings,
  onOpenQuickSearch,
  isQuickSearchOpen,
  quickSearchTriggerRef,
}: MenuBarProps) {
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
      <div className="menu-bar-center">
        <button
          type="button"
          className="menu-bar-search-trigger"
          onClick={onOpenQuickSearch}
          ref={quickSearchTriggerRef}
          aria-haspopup="dialog"
          aria-expanded={isQuickSearchOpen}
          aria-controls={QUICK_SEARCH_DIALOG_ID}
          aria-label={t("menu.search.aria", "コマンドパレットを開く")}
        >
          <svg
            width="13"
            height="13"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
            focusable="false"
          >
            <circle cx="11" cy="11" r="7"/>
            <line x1="16.5" y1="16.5" x2="21" y2="21"/>
          </svg>
          <span className="menu-bar-search-placeholder" aria-hidden="true">
            {t("menu.search", "検索... ({shortcut})", {shortcut: QUICK_SEARCH_SHORTCUT_DISPLAY})}
          </span>
        </button>
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
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true" focusable="false">
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
