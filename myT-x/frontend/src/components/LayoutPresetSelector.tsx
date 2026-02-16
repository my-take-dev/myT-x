import { api } from "../api";

interface LayoutPresetSelectorProps {
  sessionName: string;
  paneCount: number;
}

interface PresetDef {
  id: string;
  label: string;
  minPanes: number;
  svgPath: string;
}

const presets: PresetDef[] = [
  {
    id: "even-horizontal",
    label: "横均等",
    minPanes: 2,
    svgPath: "M1 1h12v12H1zM7 1v12",
  },
  {
    id: "even-vertical",
    label: "縦均等",
    minPanes: 2,
    svgPath: "M1 1h12v12H1zM1 7h12",
  },
  {
    id: "main-vertical",
    label: "左メイン",
    minPanes: 3,
    svgPath: "M1 1h12v12H1zM8.5 1v12M8.5 7h4.5",
  },
  {
    id: "main-horizontal",
    label: "上メイン",
    minPanes: 3,
    svgPath: "M1 1h12v12H1zM1 8.5h12M7 8.5v4.5",
  },
  {
    id: "tiled",
    label: "タイル",
    minPanes: 4,
    svgPath: "M1 1h12v12H1zM7 1v12M1 7h12",
  },
];

export function LayoutPresetSelector({ sessionName, paneCount }: LayoutPresetSelectorProps) {
  if (paneCount < 2) return null;

  return (
    <div className="layout-preset-bar">
      {presets
        .filter((p) => paneCount >= p.minPanes)
        .map((p) => (
          <button
            key={p.id}
            type="button"
            className="terminal-toolbar-btn layout-preset-btn"
            title={p.label}
            aria-label={`Layout: ${p.label}`}
            onClick={() => {
              void api.ApplyLayoutPreset(sessionName, p.id).catch((err) => {
                console.warn("[layout-preset] ApplyLayoutPreset failed", err);
              });
            }}
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.2">
              <path d={p.svgPath} />
            </svg>
          </button>
        ))}
    </div>
  );
}
