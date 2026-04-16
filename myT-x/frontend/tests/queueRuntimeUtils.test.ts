import {describe, expect, it} from "vitest";
import {
    normalizeGenerationId,
    stoppedMessage,
} from "../src/components/viewer/views/shared/queueRuntimeUtils";

describe("queueRuntimeUtils", () => {
    it("normalizes generation ids", () => {
        expect(normalizeGenerationId(undefined)).toBeNull();
        expect(normalizeGenerationId(null)).toBeNull();
        expect(normalizeGenerationId("")).toBeNull();
        expect(normalizeGenerationId("   ")).toBeNull();
        expect(normalizeGenerationId(" gen-1 ")).toBe("gen-1");
    });

    it("extracts non-empty stopped reasons", () => {
        expect(stoppedMessage(" worker stopped ")).toBe("worker stopped");
        expect(stoppedMessage("   ")).toBeNull();
        expect(stoppedMessage({reason: " timeout "})).toBe("timeout");
        expect(stoppedMessage({reason: ""})).toBeNull();
        expect(stoppedMessage({})).toBeNull();
        expect(stoppedMessage(42)).toBeNull();
    });
});
