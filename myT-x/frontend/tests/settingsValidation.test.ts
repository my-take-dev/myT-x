import { describe, expect, it } from "vitest";

import {
  isUnsafeWorktreeCopyPath,
  normalizeRelativePath,
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
