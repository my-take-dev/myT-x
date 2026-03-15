import {useCanvasStore} from "../../stores/canvasStore";
import {useI18n} from "../../i18n";

export function CanvasModeToggle() {
    const {language, t} = useI18n();
    const mode = useCanvasStore((s) => s.mode);
    const setMode = useCanvasStore((s) => s.setMode);

    return (
        <button
            type="button"
            className={`terminal-toolbar-btn canvas-mode-toggle ${mode === "canvas" ? "canvas-active" : ""}`}
            title={
                language === "en"
                    ? (mode === "canvas" ? "Switch to Simple mode" : "Switch to Canvas mode")
                    : t(
                        mode === "canvas"
                            ? "canvas.toggle.toSimple"
                            : "canvas.toggle.toCanvas",
                        mode === "canvas" ? "シンプルモードに切替" : "キャンバスモードに切替",
                    )
            }
            onClick={() => setMode(mode === "canvas" ? "simple" : "canvas")}
        >
            {mode === "canvas" ? (
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.4">
                    <rect x="1" y="1" width="5" height="5" rx="0.5"/>
                    <rect x="8" y="1" width="5" height="5" rx="0.5"/>
                    <rect x="1" y="8" width="5" height="5" rx="0.5"/>
                    <rect x="8" y="8" width="5" height="5" rx="0.5"/>
                </svg>
            ) : (
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.4">
                    <rect x="1" y="2" width="4" height="4" rx="0.5"/>
                    <rect x="9" y="8" width="4" height="4" rx="0.5"/>
                    <path d="M5 4 L9 10"/>
                    <circle cx="3" cy="4" r="0.5" fill="currentColor"/>
                    <circle cx="11" cy="10" r="0.5" fill="currentColor"/>
                </svg>
            )}
            <span className="canvas-mode-label">
                {mode === "canvas" ? "Simple" : "Canvas"}
            </span>
        </button>
    );
}
