import {describe, expect, it} from "vitest";
import {sanitizeCssColor} from "../src/utils/cssUtils";

describe("sanitizeCssColor", () => {
    it("returns inherit for undefined", () => {
        expect(sanitizeCssColor(undefined)).toBe("inherit");
    });

    it("returns inherit for empty string", () => {
        expect(sanitizeCssColor("")).toBe("inherit");
    });

    describe("valid hex colors", () => {
        it.each([
            ["#fff", "#fff"],
            ["#FFF", "#FFF"],
            ["#ff00ff", "#ff00ff"],
            ["#FF00FF", "#FF00FF"],
            ["#ff00ff80", "#ff00ff80"],
            ["#abc", "#abc"],
            ["#aabb", "#aabb"],
            ["#aabbcc", "#aabbcc"],
            ["#aabbccdd", "#aabbccdd"],
        ])("accepts %s", (input, expected) => {
            expect(sanitizeCssColor(input)).toBe(expected);
        });
    });

    describe("valid CSS function colors", () => {
        it.each([
            "rgb(255, 0, 0)",
            "rgb(255,0,0)",
            "rgb(100% 50% 0%)",
            "rgba(255, 0, 0, 0.5)",
            "rgba(255, 0, 0, 50%)",
            "hsl(120, 100%, 50%)",
            "hsla(120, 100%, 50%, 0.5)",
            "rgb(255 0 0 / 50%)",
        ])("accepts %s", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });
    });

    describe("valid CSS Color Level 4 functions", () => {
        it.each([
            "oklch(0.7 0.15 180)",
            "lch(50% 30 270)",
            "lab(50% 30 -20)",
            "oklab(0.5 0.1 -0.1)",
            "hwb(120 10% 20%)",
        ])("accepts %s", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });
    });

    describe("valid CSS color() function", () => {
        it.each([
            "color(display-p3 1 0.5 0)",
            "color(sRGB 1 0.5 0)",
            "color(display-p3 1 0.5 0 / 0.8)",
            "color(display-p3 1.5e-1 0.5 0)",
        ])("accepts %s", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });

        // T-10: Mixed-case color space names (tests case-insensitive `i` flag on color() regex)
        it.each([
            "color(Display-P3 1 0.5 0)",
            "color(SRGB 1 0.5 0)",
        ])("accepts mixed-case colorspace %s", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });

        it("accepts color() with multiple spaces between values", () => {
            expect(sanitizeCssColor("color(display-p3  1  0.5  0)")).toBe("color(display-p3  1  0.5  0)");
        });

        // S-07: color() with negative values — CSS color() allows negative channel values
        // for some color spaces (e.g., xyz-d65). Matches lch/oklch/lab pattern consistency.
        it.each([
            "color(xyz-d65 -0.5 0.2 0.1)",
            "color(display-p3 -0.1 0.5 0)",
            "color(sRGB -0.5 -0.3 0.1)",
        ])("accepts color() with negative values %s", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });
    });

    // T-11: Negative values in CSS Color Level 4 functions.
    // CSS Color Level 4 allows negative values in lch/oklch/lab axes.
    // The current regex supports this via escaped `\-` in the character class.
    describe("valid CSS Color Level 4 with negative values", () => {
        it.each([
            "lch(50% -20 300)",
            "oklch(0.5 -0.1 300)",
            "lab(50% -20 30)",
        ])("accepts %s", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });
    });

    describe("valid CSS Color Level 4 with alpha", () => {
        it.each([
            "oklch(0.5 0.1 240 / 80%)",
        ])("accepts %s", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });
    });

    describe("valid hwb function", () => {
        it.each([
            "hwb(120 10% 20%)",
        ])("accepts %s", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });
    });

    describe("rejects invalid Level 4 colors", () => {
        it.each([
            "oklch()",
            "color()",
        ])("rejects %s → inherit", (input) => {
            expect(sanitizeCssColor(input)).toBe("inherit");
        });
    });

    // T-04: Boundary value tests for color() numeric parsing.
    // The color() regex uses [\d.]+ for numeric values, which accepts some technically
    // invalid CSS but is intentionally permissive — the function's purpose is CSS injection
    // prevention, not color correctness (see NOTE in sanitizeCssColor source).
    describe("color() function numeric boundary values", () => {
        it.each([
            ["color(display-p3 .5 .5 .5)", "leading dot numbers are valid CSS"],
            ["color(display-p3 0.0 0.0 0.0)", "zero values with decimal point"],
            ["color(display-p3 1 1 1)", "integer-only values"],
        ])("accepts %s (%s)", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });

        // These are not valid CSS color values, but the regex allows them because
        // [\d.]+ does not enforce numeric format. This is documented behavior —
        // sanitizeCssColor prevents injection, not semantic validity.
        it.each([
            ["color(display-p3 0. 0 0)", "trailing dot — CSS invalid but regex allows"],
            ["color(display-p3 1.2.3 0 0)", "multiple dots — CSS invalid but regex allows"],
            ["color(display-p3 ... 0 0)", "dots only — CSS invalid but regex allows"],
        ])("accepts %s (%s) — known permissive behavior", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });
    });

    describe("rejects invalid color() function inputs", () => {
        it.each([
            ["color(p3)", "no numeric values after colorspace"],
            ["color(p3 ); background: red;", "injection attempt"],
            ["color(p3 calc(1))", "nested parens"],
        ])("rejects %s (%s) → inherit", (input) => {
            expect(sanitizeCssColor(input)).toBe("inherit");
        });
    });

    describe("rejects scientific notation in non-color() functions (not valid CSS color syntax)", () => {
        it.each([
            "oklch(0.5 0.1e+2 240)",
            "lab(50% 3e-1 -2e+1)",
            "hsl(1e2, 50%, 50%)",
        ])("rejects %s → inherit", (input) => {
            expect(sanitizeCssColor(input)).toBe("inherit");
        });
    });

    // Regression tests for non-color() function sanitizer strictness.
    describe("rejects invalid punctuation and notation in non-color() functions", () => {
        it.each([
            ["hsl(1e+2, 50%, 50%)", "scientific notation with + in non-color() function"],
            ["oklch(0.5 0.2+0.1 120)", "+ between numeric values"],
            ["rgb(255&0&0)", "& character"],
            ["hsl(120' 100%' 50%)", "' character"],
            ["rgb(255*0*0)", "* character"],
        ])("rejects %s (%s) → inherit", (input) => {
            expect(sanitizeCssColor(input)).toBe("inherit");
        });
    });

    describe("accepts valid values with literal hyphen (negative numbers)", () => {
        it.each([
            "hsl(-30, 50%, 50%)",
            "lab(50% -20 -30)",
            "oklch(0.5 -0.1 -180)",
        ])("accepts %s", (input) => {
            expect(sanitizeCssColor(input)).toBe(input);
        });
    });

    describe("rejects nonsensical function arguments", () => {
        it.each([
            "hsl(e+-)",
            "rgb(e)",
            "oklch(+)",
        ])("rejects %s → inherit", (input) => {
            expect(sanitizeCssColor(input)).toBe("inherit");
        });
    });

    describe("rejects invalid / injection attempts", () => {
        it.each([
            "red",
            "blue",
            "transparent",
            "#gg0000",
            "#12345",
            "#1234567",
            "#1234567890",
            "url(javascript:alert(1))",
            "expression(alert(1))",
            "rgb(255, 0, 0); background: red",
            "<script>alert(1)</script>",
            "javascript:alert(1)",
            "var(--accent)",
            "calc(100px)",
            "rgb()",
            "rgb(a, b, c)",
        ])("rejects %s → inherit", (input) => {
            expect(sanitizeCssColor(input)).toBe("inherit");
        });
    });

    describe("rejects uppercase non-color() function names", () => {
        it.each([
            "RGB(255, 0, 0)",
            "HSL(120, 100%, 50%)",
            "OKLCH(0.7 0.15 180)",
        ])("rejects %s", (input) => {
            expect(sanitizeCssColor(input)).toBe("inherit");
        });
    });
});
