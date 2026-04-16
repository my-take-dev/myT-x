import {describe, expect, it} from "vitest";
import {
    buildAbsoluteEditorPath,
    joinRelativePath,
    parentDirOf,
    remapDescendantPath,
    toWindowsPath,
} from "../src/components/viewer/views/editor/editorPathUtils";

describe("editorPathUtils", () => {
    it("returns the parent directory or an empty string for root-level files", () => {
        expect(parentDirOf("src/app.ts")).toBe("src");
        expect(parentDirOf("README.md")).toBe("");
    });

    it("joins relative paths without duplicating separators", () => {
        expect(joinRelativePath("", "app.ts")).toBe("app.ts");
        expect(joinRelativePath("src", "app.ts")).toBe("src/app.ts");
    });

    it("remaps renamed descendants", () => {
        expect(remapDescendantPath("src/nested/file.ts", "src", "app")).toBe("app/nested/file.ts");
        expect(remapDescendantPath("src", "src", "app")).toBe("app");
        expect(remapDescendantPath("src-other/file.ts", "src", "app")).toBe("src-other/file.ts");
    });

    it("normalizes editor paths for Windows shells", () => {
        expect(toWindowsPath("src/nested/file.ts")).toBe("src\\nested\\file.ts");
        expect(buildAbsoluteEditorPath("", "src/nested/file.ts")).toBe("src\\nested\\file.ts");
        expect(buildAbsoluteEditorPath("C:/repo/", "")).toBe("C:\\repo");
        expect(buildAbsoluteEditorPath("C:/repo/", "\\src\\nested\\file.ts")).toBe("C:\\repo\\src\\nested\\file.ts");
    });
});
