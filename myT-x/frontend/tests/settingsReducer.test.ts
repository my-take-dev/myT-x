import {describe, expect, it} from "vitest";
import {INITIAL_FORM, formReducer} from "../src/components/settings/settingsReducer";

describe("settingsReducer INITIAL_FORM guard", () => {
  it("keeps expected top-level form keys", () => {
    const keys = Object.keys(INITIAL_FORM).sort();
    expect(keys).toEqual([
      "activeCategory",
      "agentFrom",
      "agentTo",
      "allowedShells",
      "claudeEnvDefaultEnabled",
      "claudeEnvEntries",
      "defaultSessionDir",
      "effortLevel",
      "error",
      "globalHotkey",
      "keys",
      "loadFailed",
      "loading",
      "minOverrideNameLen",
      "overrides",
      "paneEnvDefaultEnabled",
      "paneEnvEntries",
      "prefix",
      "quakeMode",
      "saving",
      "shell",
      "validationErrors",
      "viewerShortcuts",
      "wtCopyDirs",
      "wtCopyFiles",
      "wtEnabled",
      "wtForceCleanup",
      "wtSetupScripts",
    ].sort());
  });

  it("RESET_FOR_LOAD preserves schema and sets loading", () => {
    const reset = formReducer(INITIAL_FORM, {type: "RESET_FOR_LOAD"});
    expect(Object.keys(reset).sort()).toEqual(Object.keys(INITIAL_FORM).sort());
    expect(reset.loading).toBe(true);
  });
});
