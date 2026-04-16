import {describe, expect, it} from "vitest";
import {buildAbsoluteEditorPath, remapDescendantPath} from "./editorPathUtils";

describe("editorPathUtils", () => {
    it("remaps descendant paths after normalizing separators", () => {
        expect(remapDescendantPath("src\\nested\\file.txt", "src", "app")).toBe("app/nested/file.txt");
        expect(remapDescendantPath("src-other\\file.txt", "src", "app")).toBe("src-other/file.txt");
    });

    it("builds absolute Windows paths from mixed separators", () => {
        expect(buildAbsoluteEditorPath("C:/worktree\\project/", "\\src/file.ts")).toBe("C:\\worktree\\project\\src\\file.ts");
    });
});
