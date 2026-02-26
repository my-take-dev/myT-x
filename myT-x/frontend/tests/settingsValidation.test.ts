import {describe, expect, it} from "vitest";

import {
    isUnsafeWorktreeCopyPath,
    normalizeRelativePath,
    validateDefaultSessionDir,
    validateViewerShortcuts,
    validateWorktreeCopyPathSettings,
} from "../src/components/settings/settingsValidation";

describe("normalizeRelativePath", () => {
    it("collapses nested dot segments", () => {
        expect(normalizeRelativePath("a//b/./c")).toBe("a/b/c");
    });

    it("keeps unresolved traversal at the start", () => {
        expect(normalizeRelativePath("./././../x")).toBe("../x");
        expect(normalizeRelativePath("foo/bar/../../..")).toBe("..");
    });

    it("returns dot when path collapses to root", () => {
        expect(normalizeRelativePath("foo/..")).toBe(".");
    });
});

describe("isUnsafeWorktreeCopyPath", () => {
    it("accepts safe relative paths", () => {
        expect(isUnsafeWorktreeCopyPath("config/app.yaml")).toBe(false);
        expect(isUnsafeWorktreeCopyPath("config\\app.yaml")).toBe(false);
        expect(isUnsafeWorktreeCopyPath("  config/app.yaml  ")).toBe(false);
    });

    it("rejects traversal and absolute paths", () => {
        expect(isUnsafeWorktreeCopyPath(".")).toBe(true);
        expect(isUnsafeWorktreeCopyPath("..")).toBe(true);
        expect(isUnsafeWorktreeCopyPath("../secret.txt")).toBe(true);
        expect(isUnsafeWorktreeCopyPath("foo/bar/../../..")).toBe(true);
        expect(isUnsafeWorktreeCopyPath("/etc/passwd")).toBe(true);
        expect(isUnsafeWorktreeCopyPath("C:\\Windows\\System32")).toBe(true);
        expect(isUnsafeWorktreeCopyPath("\\\\server\\share\\folder")).toBe(true);
    });
});

describe("validateWorktreeCopyPathSettings", () => {
    it("reports per-item and aggregate errors for unsafe paths", () => {
        const errors = validateWorktreeCopyPathSettings(
            ["foo/bar/../../..", "valid/file.txt"],
            ["./././../x", "foo/..", "valid-dir"],
        );

        expect(errors).toHaveProperty("wt_copy_files_0");
        expect(errors).toHaveProperty("wt_copy_files");
        expect(errors).toHaveProperty("wt_copy_dirs_0");
        expect(errors).toHaveProperty("wt_copy_dirs_1");
        expect(errors).toHaveProperty("wt_copy_dirs");
    });

    it("returns no errors for valid relative paths", () => {
        const errors = validateWorktreeCopyPathSettings(
            ["env/.env.local", "config/app.yaml"],
            ["vendor/assets", "templates/email"],
        );
        expect(errors).toEqual({});
    });
});

describe("validateViewerShortcuts", () => {
    it("accepts defaults when no custom shortcuts are provided", () => {
        expect(validateViewerShortcuts({})).toEqual({});
    });

    it("rejects shortcuts without modifier keys", () => {
        const errors = validateViewerShortcuts({
            "file-tree": "f",
        });
        expect(errors).toHaveProperty("viewer_shortcut_file-tree");
    });

    it("allows function keys without modifier keys", () => {
        const errors = validateViewerShortcuts({
            "file-tree": "F12",
        });
        expect(errors).not.toHaveProperty("viewer_shortcut_file-tree");
    });

    it("rejects duplicate custom shortcuts (case-insensitive)", () => {
        const errors = validateViewerShortcuts({
            "file-tree": "Ctrl+Shift+Q",
            "git-graph": "ctrl+shift+q",
        });
        expect(errors).toHaveProperty("viewer_shortcut_file-tree");
        expect(errors).toHaveProperty("viewer_shortcut_git-graph");
    });

    it("rejects custom shortcut that conflicts with another view default", () => {
        const errors = validateViewerShortcuts({
            "file-tree": "Ctrl+Shift+G", // conflicts with git-graph default
        });
        expect(errors).toHaveProperty("viewer_shortcut_file-tree");
        expect(errors).toHaveProperty("viewer_shortcut_git-graph");
    });

    it("normalizes modifier ordering before duplicate checks", () => {
        const errors = validateViewerShortcuts({
            "git-graph": "Shift+Ctrl+E", // conflicts with file-tree default Ctrl+Shift+E
        });
        expect(errors).toHaveProperty("viewer_shortcut_file-tree");
        expect(errors).toHaveProperty("viewer_shortcut_git-graph");
    });

    it("rejects conflict with global hotkey", () => {
        const errors = validateViewerShortcuts(
            {
                "diff": "Ctrl+Shift+F12",
            },
            "Ctrl+Shift+F12",
        );
        expect(errors).toHaveProperty("viewer_shortcut_diff");
    });
});

describe("validateDefaultSessionDir", () => {
    it("accepts empty value", () => {
        expect(validateDefaultSessionDir("")).toEqual({});
    });

    it("accepts absolute paths", () => {
        expect(validateDefaultSessionDir("C:\\Users\\tester\\project")).toEqual({});
        expect(validateDefaultSessionDir("\\\\server\\share\\project")).toEqual({});
    });

    it("accepts tilde and env-var prefixes", () => {
        expect(validateDefaultSessionDir("~/project")).toEqual({});
        expect(validateDefaultSessionDir("$HOME/project")).toEqual({});
        expect(validateDefaultSessionDir("%USERPROFILE%\\project")).toEqual({});
    });

    it("accepts POSIX absolute paths", () => {
        expect(validateDefaultSessionDir("/home/tester/project")).toEqual({});
        expect(validateDefaultSessionDir("/usr/local/bin")).toEqual({});
    });

    it("rejects single-backslash rooted paths", () => {
        expect(validateDefaultSessionDir("\\windows\\system32")).toEqual({
            default_session_dir: "絶対パスを指定してください",
        });
    });

    it("rejects relative paths", () => {
        expect(validateDefaultSessionDir("relative/path")).toEqual({
            default_session_dir: "絶対パスを指定してください",
        });
    });
});
