import {describe, expect, it} from "vitest";
import {MONACO_OPTIONS} from "./editorConstants";

describe("MONACO_OPTIONS", () => {
    it("enables the expected reader-focused editor options", () => {
        expect(MONACO_OPTIONS.stickyScroll).toEqual({enabled: true});
        expect(MONACO_OPTIONS.guides).toEqual({bracketPairs: true, indentation: true});
        expect(MONACO_OPTIONS["semanticHighlighting.enabled"]).toBe(true);
        expect(MONACO_OPTIONS.inlayHints).toEqual({enabled: "on"});
    });
});
