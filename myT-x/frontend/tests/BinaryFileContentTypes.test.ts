import {describe, expect, it} from "vitest";
import type {BinaryFileContent} from "../src/components/viewer/views/file-tree/documentTypes";
import {devpanel} from "../wailsjs/go/models";

describe("BinaryFileContent field alignment", () => {
    it("manual frontend type matches the generated Wails model fields", () => {
        const manualShape: BinaryFileContent = {
            path: "",
            data: "",
            mime: "",
        };
        const generatedShape = new devpanel.BinaryFileContent({
            path: "",
            data: "",
            mime: "",
        });

        expect(Object.keys(manualShape).sort()).toEqual(Object.keys(generatedShape).sort());
    });
});
