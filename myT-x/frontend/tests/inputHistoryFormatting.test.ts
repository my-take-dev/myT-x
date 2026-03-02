import {describe, expect, it} from "vitest";
import {formatInputForDisplay} from "../src/components/viewer/views/input-history/useInputHistory";
import {formatTimestamp} from "../src/utils/timestampUtils";

describe("input history formatting", () => {
    it("formats compact timestamps", () => {
        expect(formatTimestamp("20260301112233")).toBe("2026-03-01 11:22:33");
    });

    it("keeps non-compact timestamps unchanged", () => {
        expect(formatTimestamp("2026-03-01T11:22:33Z")).toBe("2026-03-01T11:22:33Z");
    });

    it("renders control characters in caret notation", () => {
        expect(formatInputForDisplay("\x01hello\x7f\n")).toBe("^Ahello^?^J");
    });
});
