import {describe, expect, it} from "vitest";
import {sanitizeHref} from "../src/utils/sanitizeHref";

describe("sanitizeHref", () => {
    describe("returns null for empty / falsy inputs", () => {
        it.each([
            [undefined, "undefined"],
            ["", "empty string"],
            ["   ", "whitespace only"],
            ["\t\n\r", "control characters only"],
            ["\0", "null byte only"],
        ])("returns null for %s (%s)", (input, _desc) => {
            expect(sanitizeHref(input as string | undefined)).toBeNull();
        });
    });

    describe("allows fragment-only links", () => {
        it.each([
            "#",
            "#top",
            "#section-1",
            "#my-anchor",
        ])("allows %s", (input) => {
            expect(sanitizeHref(input)).toBe(input);
        });
    });

    describe("allows relative paths", () => {
        it.each([
            ["/absolute-path", "/absolute-path"],
            ["/path/to/file.html", "/path/to/file.html"],
            ["./relative", "./relative"],
            ["./path/to/file", "./path/to/file"],
            ["../parent", "../parent"],
            ["../parent/file.md", "../parent/file.md"],
        ])("allows %s", (input, expected) => {
            expect(sanitizeHref(input)).toBe(expected);
        });
    });

    describe("rejects protocol-relative URLs (//)", () => {
        it.each([
            "//evil.com",
            "//evil.com/payload",
            "//localhost/test",
            "///triple-slash",
            "///",
        ])("rejects %s", (input) => {
            expect(sanitizeHref(input)).toBeNull();
        });
    });

    describe("allows safe schemes", () => {
        it.each([
            ["http://example.com", "http"],
            ["https://example.com", "https"],
            ["https://example.com/path?q=1#anchor", "https with query and fragment"],
            ["mailto:user@example.com", "mailto"],
            ["tel:+1234567890", "tel"],
        ])("allows %s (%s)", (input) => {
            expect(sanitizeHref(input)).toBe(input);
        });
    });

    describe("rejects javascript: scheme (XSS prevention)", () => {
        it.each([
            "javascript:alert(1)",
            "javascript:alert('xss')",
            "JavaScript:alert(1)",
            "JAVASCRIPT:ALERT(1)",
            "jAvAsCrIpT:alert(1)",
        ])("rejects %s", (input) => {
            expect(sanitizeHref(input)).toBeNull();
        });

        it("rejects javascript: with control character obfuscation", () => {
            // Control chars (\r\n\t\0) are stripped before evaluation
            expect(sanitizeHref("java\tscript:alert(1)")).toBeNull();
            expect(sanitizeHref("java\nscript:alert(1)")).toBeNull();
            expect(sanitizeHref("java\rscript:alert(1)")).toBeNull();
            expect(sanitizeHref("java\0script:alert(1)")).toBeNull();
            expect(sanitizeHref("j\ta\tv\ta\tscript:alert(1)")).toBeNull();
        });

        it("rejects javascript: with zero-width and line-separator obfuscation", () => {
            expect(sanitizeHref("javascript\u200B:alert(1)")).toBeNull();
            expect(sanitizeHref("java\u200Cscript:alert(1)")).toBeNull();
            expect(sanitizeHref("java\u200Dscript:alert(1)")).toBeNull();
            expect(sanitizeHref("java\u200Escript:alert(1)")).toBeNull();
            expect(sanitizeHref("java\u200Fscript:alert(1)")).toBeNull();
            expect(sanitizeHref("java\uFEFFscript:alert(1)")).toBeNull();
            expect(sanitizeHref("java\u2028script:alert(1)")).toBeNull();
            expect(sanitizeHref("java\u2029script:alert(1)")).toBeNull();
            expect(sanitizeHref("java\u202Escript:alert(1)")).toBeNull();
            expect(sanitizeHref("java\u2066script:alert(1)")).toBeNull();
        });
    });

    describe("rejects data: scheme", () => {
        it.each([
            "data:text/html,<script>alert(1)</script>",
            "data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==",
            "data:image/svg+xml,<svg onload=alert(1)>",
            "DATA:text/html,test",
        ])("rejects %s", (input) => {
            expect(sanitizeHref(input)).toBeNull();
        });
    });

    describe("rejects vbscript: and other dangerous schemes", () => {
        it.each([
            "vbscript:MsgBox(1)",
            "VBScript:MsgBox(1)",
            "file:///etc/passwd",
            "ftp://evil.com/payload",
            "blob:http://example.com/uuid",
        ])("rejects %s", (input) => {
            expect(sanitizeHref(input)).toBeNull();
        });
    });

    describe("control character sanitization", () => {
        it("strips \\r\\n\\t\\0 before evaluation", () => {
            // Tabs and newlines within a valid URL are stripped
            expect(sanitizeHref("https://exa\tmple.com")).toBe("https://example.com");
            expect(sanitizeHref("https://exa\nmple.com")).toBe("https://example.com");
            expect(sanitizeHref("https://exa\rmple.com")).toBe("https://example.com");
            expect(sanitizeHref("https://exa\0mple.com")).toBe("https://example.com");
        });

        it("strips leading/trailing whitespace via trim()", () => {
            expect(sanitizeHref("  https://example.com  ")).toBe("https://example.com");
            expect(sanitizeHref("\thttps://example.com\n")).toBe("https://example.com");
        });

        it("rejects protocol-relative URL after separator stripping", () => {
            expect(sanitizeHref("\t//evil.com")).toBeNull();
            expect(sanitizeHref("\n//evil.com")).toBeNull();
            expect(sanitizeHref("\u200B//evil.com")).toBeNull();
        });
    });

    describe("scheme-less strings treated as relative", () => {
        it.each([
            "simple-text",
            "some/path",
            "file.html",
            "path/to/page.md",
        ])("allows scheme-less string %s", (input) => {
            expect(sanitizeHref(input)).toBe(input);
        });
    });

    describe("malformed URL handling", () => {
        it("rejects invalid URL with scheme that fails URL parsing", () => {
            // A string that matches SCHEME_PATTERN but fails new URL() parsing
            expect(sanitizeHref("not-a-real-scheme:///")).toBeNull();
            expect(sanitizeHref("https://")).toBeNull();
        });

        it("keeps malformed percent-encoded relative paths as relative", () => {
            expect(sanitizeHref("docs/%E0%A4%A.md")).toBe("docs/%E0%A4%A.md");
        });
    });

    describe("percent-encoded scheme bypass", () => {
        it.each([
            ["%6aavascript:alert(1)", "%6a = j"],
            ["%6Aavascript:alert(1)", "%6A = j (uppercase hex)"],
            ["j%61vascript:alert(1)", "%61 = a"],
            ["java%73cript:alert(1)", "%73 = s"],
            ["%6a%61%76%61script:alert(1)", "multiple encoded chars"],
        ])("rejects %s (%s)", (input) => {
            expect(sanitizeHref(input)).toBeNull();
        });

        it("rejects percent-encoded protocol-relative URLs", () => {
            expect(sanitizeHref("%2F%2Fevil.com/path")).toBeNull();
            expect(sanitizeHref("%2f%2fevil.com/path")).toBeNull();
        });
    });

    describe("Bidi control character stripping", () => {
        it("strips LRE (U+202A) from URLs", () => {
            expect(sanitizeHref("https://example.com/\u202Apath")).toBe("https://example.com/path");
        });

        it("strips PDI (U+2069) from URLs", () => {
            expect(sanitizeHref("https://example.com/\u2069path")).toBe("https://example.com/path");
        });

        it("strips multiple Bidi control characters from the same URL", () => {
            expect(sanitizeHref("https://\u202Aexample\u202E.com/\u2066path\u2069")).toBe("https://example.com/path");
        });
    });

    describe("soft hyphen stripping", () => {
        it("strips soft hyphen (\\u00AD) before evaluation", () => {
            expect(sanitizeHref("java\u00ADscript:alert(1)")).toBeNull();
        });
    });
});
