import {describe, expect, it} from "vitest";
import type {FileNode} from "./fileTreeTypes";
import {classifyDocument, filterDocumentTree, isDocumentFile} from "./documentFilter";

function makeFile(path: string): FileNode {
    const segments = path.split("/");
    return {
        name: segments[segments.length - 1] ?? path,
        path,
        isDir: false,
        hasChildren: false,
        size: 128,
    };
}

function makeDir(path: string, children?: readonly FileNode[], hasChildren: boolean = Boolean(children?.length)): FileNode {
    const segments = path.split("/");
    return {
        name: segments[segments.length - 1] ?? path,
        path,
        isDir: true,
        hasChildren,
        children,
    };
}

describe("isDocumentFile", () => {
    it.each([
        ["README.md", true],
        ["README.MD", true],
        ["archive.db", true],
        ["diagram.mmd", true],
        ["design.drawio", true],
        ["design.drawio.svg", true],
        ["design.drawio.xml", true],
        ["cache.sqlite", true],
        ["cache.sqlite3", true],
        ["openapi.yaml", true],
        ["openapi.yml", true],
        ["openapi.json", true],
        ["notes.txt", false],
        ["Makefile", false],
        ["archive.tar.gz", false],
        ["folder.md", false],
    ])("classifies %s as supported=%s", (name, expected) => {
        const node: FileNode = expected && name !== "folder.md"
            ? makeFile(name)
            : {
                name,
                path: name,
                isDir: name === "folder.md",
                hasChildren: false,
            };
        expect(isDocumentFile(node)).toBe(expected);
    });
});

describe("filterDocumentTree", () => {
    it("keeps supported files and removes unsupported siblings", () => {
        const filtered = filterDocumentTree([
            makeFile("README.md"),
            makeFile("notes.txt"),
        ]);

        expect(filtered.map((node) => node.path)).toEqual(["README.md"]);
    });

    it("keeps loaded parent directories that contain supported descendants", () => {
        const filtered = filterDocumentTree([
            makeDir("docs", [
                makeDir("docs/spec", [
                    makeFile("docs/spec/guide.md"),
                    makeFile("docs/spec/guide.txt"),
                ]),
            ]),
        ]);

        expect(filtered).toEqual([
            makeDir("docs", [
                makeDir("docs/spec", [
                    makeFile("docs/spec/guide.md"),
                ]),
            ]),
        ]);
    });

    it("removes loaded directories that become empty after recursive filtering", () => {
        const filtered = filterDocumentTree([
            makeDir("docs", [
                makeDir("docs/notes", [
                    makeFile("docs/notes/todo.txt"),
                ]),
            ]),
        ]);

        expect(filtered).toEqual([]);
    });

    it("keeps unexplored directories with children so lazy loading can continue", () => {
        const filtered = filterDocumentTree([
            makeDir("docs", undefined, true),
        ]);

        expect(filtered.map((node) => node.path)).toEqual(["docs"]);
    });

    it("removes empty directories with no known children", () => {
        const filtered = filterDocumentTree([
            makeDir("empty", [], false),
        ]);

        expect(filtered).toEqual([]);
    });
});

describe("classifyDocument", () => {
    it.each([
        ["README.md", "# title", "markdown"],
        ["README.MD", "# title", "markdown"],
        ["archive.db", "", "sqlite"],
        ["diagram.mmd", "graph TD;", "mermaid"],
        ["design.drawio", "<mxfile/>", "drawio-xml"],
        ["design.drawio.xml", "<mxfile/>", "drawio-xml"],
        ["design.drawio.svg", "<svg/>", "drawio-svg"],
        ["cache.sqlite", "", "sqlite"],
        ["cache.sqlite3", "", "sqlite"],
        ["spec.yaml", "openapi: 3.0.3", "swagger"],
        ["spec.yml", "\n  swagger: \"2.0\"", "swagger"],
        ["spec.json", "{\"openapi\":\"3.1.0\"}", "swagger"],
        ["spec.json", "{\"swagger\":\"2.0\"}", "swagger"],
        ["spec.json", "{\"info\":{},\"openapi\":\"3.1.0\"}", "swagger"],
        ["spec.yaml", "\uFEFFopenapi: 3.1.0", "swagger"],
        ["spec.yaml", "---\nopenapi: 3.0.3", "swagger"],
        ["config.yaml", "name: test", "yaml-json-raw"],
        ["config.yml", "swagger_version: 2.0", "yaml-json-raw"],
        ["config.json", "{\"name\":\"plain-json\"}", "yaml-json-raw"],
        ["config.json", "{\"info\":{\"openapi\":\"3.1.0\"}}", "yaml-json-raw"],
        ["config.json", "{\"items\":[{\"swagger\":\"2.0\"}]}", "yaml-json-raw"],
        ["config.json", "{\"name\":\"contains openapi: 3.1.0 in a value\"}", "yaml-json-raw"],
        ["config.yaml", "# comment\ninfo:\n  title: demo", "yaml-json-raw"],
        ["config.json", `${" ".repeat(1025)}{"openapi":"3.1.0"}`, "yaml-json-raw"],
    ])("classifies %s as %s", (name, content, expected) => {
        expect(classifyDocument(name, content)).toBe(expected);
    });
});
