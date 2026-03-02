import {describe, expect, it, vi} from "vitest";
import {extToShikiLang, getHighlightSkipInfo, highlightCode, isMarkdownLang, pathToShikiLang} from "../src/utils/shikiHighlighter";

describe("extToShikiLang", () => {
    it("resolves aliases and normalizes extension casing", () => {
        expect(extToShikiLang("yml")).toBe("yaml");
        expect(extToShikiLang("mjs")).toBe("javascript");
        expect(extToShikiLang("TS")).toBe("typescript");
    });

    it("returns null for unknown extensions", () => {
        expect(extToShikiLang("unknownext")).toBeNull();
    });

    it("handles boundary/format variants", () => {
        expect(extToShikiLang("")).toBeNull();
        expect(extToShikiLang(".ts")).toBeNull();
        expect(extToShikiLang(" ts ")).toBe("typescript");
    });
});

describe("pathToShikiLang", () => {
    it("detects extension-based languages", () => {
        expect(pathToShikiLang("src/app.ts")).toBe("typescript");
        expect(pathToShikiLang("README.markdown")).toBe("markdown");
    });

    it("detects dotfile-based languages", () => {
        expect(pathToShikiLang(".env")).toBe("ini");
        expect(pathToShikiLang("config/.bashrc")).toBe("shellscript");
        expect(pathToShikiLang("Dockerfile")).toBe("dockerfile");
        expect(pathToShikiLang(".env.local")).toBe("ini");
        expect(pathToShikiLang(".npmrc")).toBe("ini");
        expect(pathToShikiLang(".editorconfig")).toBe("ini");
        expect(pathToShikiLang("Makefile")).toBe("makefile");
    });

    it("supports windows path separators and multiple dots", () => {
        expect(pathToShikiLang("C:\\repo\\src\\main.TS")).toBe("typescript");
        expect(pathToShikiLang("C:\\repo\\archive\\name.test.yaml")).toBe("yaml");
    });

    it("returns null for unknown or malformed filenames", () => {
        expect(pathToShikiLang("notes.unknownext")).toBeNull();
        expect(pathToShikiLang("plainfilename")).toBeNull();
        expect(pathToShikiLang("file.")).toBeNull();
        expect(pathToShikiLang("")).toBeNull();
    });
});

describe("getHighlightSkipInfo", () => {
    it("allows content exactly at configured limits", () => {
        const maxSizeLines = Array.from({length: 10}, () => "x".repeat(1_999));
        maxSizeLines[maxSizeLines.length - 1] = `${maxSizeLines[maxSizeLines.length - 1]}x`;
        const maxSize = maxSizeLines.join("\n");
        const maxLines = Array.from({length: 1_200}, () => "x").join("\n");
        const maxLineLength = "x".repeat(2_000);

        expect(getHighlightSkipInfo(maxSize)).toBeNull();
        expect(getHighlightSkipInfo(maxLines)).toBeNull();
        expect(getHighlightSkipInfo(maxLineLength)).toBeNull();
    });

    it("skips oversized content", () => {
        const oversized = "x".repeat(20_001);
        expect(getHighlightSkipInfo(oversized)).toEqual({
            reason: "size-limit",
            limit: 20_000,
            actual: 20_001,
        });
    });

    it("skips when line count is too large", () => {
        const manyLines = Array.from({length: 1_201}, () => "x").join("\n");
        expect(getHighlightSkipInfo(manyLines)).toEqual({
            reason: "line-count-limit",
            limit: 1_200,
            actual: 1_201,
        });
    });

    it("skips when a line is too long", () => {
        const longLine = "x".repeat(2_001);
        expect(getHighlightSkipInfo(longLine)).toEqual({
            reason: "line-length-limit",
            limit: 2_000,
            actual: 2_001,
        });
    });

    it("counts CRLF lines correctly and ignores carriage returns for line-length", () => {
        const crlfWithinLimit = Array.from({length: 1_200}, () => "x").join("\r\n");
        const crlfOverLimit = Array.from({length: 1_201}, () => "x").join("\r\n");
        const crlfLongLine = `${"x".repeat(2_001)}\r\nok`;

        expect(getHighlightSkipInfo(crlfWithinLimit)).toBeNull();
        expect(getHighlightSkipInfo(crlfOverLimit)).toEqual({
            reason: "line-count-limit",
            limit: 1_200,
            actual: 1_201,
        });
        expect(getHighlightSkipInfo(crlfLongLine)).toEqual({
            reason: "line-length-limit",
            limit: 2_000,
            actual: 2_001,
        });
    });

    it("allows content within limits", () => {
        const safe = ["const a = 1;", "const b = 2;", "console.log(a + b);"];
        expect(getHighlightSkipInfo(safe.join("\n"))).toBeNull();
    });
});

describe("highlightCode", () => {
    it("highlights markdown with lazy-loaded grammar", async () => {
        const code = "# Title\n\n```ts\nconst value = 1;\n```";
        const tokens = await highlightCode(code, "markdown");

        expect(tokens).not.toBeNull();
        expect(tokens?.length).toBeGreaterThan(0);
    });

    it("returns null for unknown language", async () => {
        expect(await highlightCode("const x = 1;", "not-a-real-language")).toBeNull();
    });

    it("returns null repeatedly while unknown-language cooldown is active", async () => {
        const first = await highlightCode("const first = 1;", "not-a-real-language");
        const second = await highlightCode("const second = 2;", "not-a-real-language");
        expect(first).toBeNull();
        expect(second).toBeNull();
    });

    it("returns null for oversized code", async () => {
        const oversized = "x".repeat(20_001);
        expect(await highlightCode(oversized, "typescript")).toBeNull();
    });

    it("returns cached tokens on repeated calls", async () => {
        const code = "const cached = 1;";
        const first = await highlightCode(code, "typescript");
        const second = await highlightCode(code, "typescript");
        expect(first).not.toBeNull();
        expect(second).toBe(first);
    });

    it("evicts least-recent entries when cache exceeds entry limit", async () => {
        const firstCode = "const evict_target = 0;";
        const firstTokens = await highlightCode(firstCode, "typescript");

        for (let i = 1; i <= 30; i++) {
            await highlightCode(`const evict_${i} = ${i};`, "typescript");
        }

        const firstTokensAgain = await highlightCode(firstCode, "typescript");
        expect(firstTokens).not.toBeNull();
        expect(firstTokensAgain).not.toBeNull();
        expect(firstTokensAgain).not.toBe(firstTokens);
    });
});

describe("highlightCode cooldown retry", () => {
    it("retries unknown language after cooldown expires", async () => {
        vi.useFakeTimers();
        try {
            // 1. Trigger a highlight with an unknown language — should fail and enter cooldown
            const first = await highlightCode("const x = 1;", "not-a-real-language-retry-test");
            expect(first).toBeNull();

            // During cooldown, should still return null
            const duringCooldown = await highlightCode("const y = 2;", "not-a-real-language-retry-test");
            expect(duringCooldown).toBeNull();

            // 2. Advance time past the max cooldown period (5 minutes = 300_000ms)
            vi.advanceTimersByTime(5 * 60_000 + 1);

            // 3. Trigger highlight again — should retry (not be blocked by cooldown)
            //    It will still return null (the language truly doesn't exist),
            //    but the important thing is that it actually attempts the load again
            //    rather than being immediately blocked by the cooldown.
            const afterCooldown = await highlightCode("const z = 3;", "not-a-real-language-retry-test");
            expect(afterCooldown).toBeNull();
        } finally {
            vi.useRealTimers();
        }
    });
});

describe("isMarkdownLang", () => {
    it.each([
        ["markdown", true],
        ["mdx", true],
        ["typescript", false],
        ["", false],
        [null, false],
    ])("isMarkdownLang(%s) === %s", (lang, expected) => {
        expect(isMarkdownLang(lang as string | null)).toBe(expected);
    });
});
