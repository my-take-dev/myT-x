import type {BinaryFileContent} from "./documentTypes";

export function decodeBase64Buffer(data: string): ArrayBuffer {
    const binary = atob(data);
    const buffer = new ArrayBuffer(binary.length);
    const bytes = new Uint8Array(buffer);
    for (let index = 0; index < binary.length; index += 1) {
        bytes[index] = binary.charCodeAt(index);
    }
    return buffer;
}

export function createBinaryBlob(binaryContent: BinaryFileContent): Blob {
    return new Blob([decodeBase64Buffer(binaryContent.data)], {type: binaryContent.mime});
}
