export const EDITOR_CONFIG = {
    LARGE_FILE_THRESHOLD: 1_048_576,
    LAYOUT_DELAY_MS: 100,
} as const;

export const MONACO_OPTIONS = {
    automaticLayout: true,
    bracketPairColorization: {enabled: true},
    fontSize: 13,
    guides: {
        bracketPairs: true,
        indentation: true,
    },
    inlayHints: {enabled: "on"},
    lineNumbers: "on",
    minimap: {enabled: true, scale: 0.8},
    scrollBeyondLastLine: false,
    "semanticHighlighting.enabled": true,
    stickyScroll: {enabled: true},
    tabSize: 2,
    wordWrap: "off",
} as const;
