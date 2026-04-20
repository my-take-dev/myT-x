import {describe, expect, it, vi} from "vitest";
import {createRetriableAsyncSingleton} from "./createRetriableAsyncSingleton";

describe("createRetriableAsyncSingleton", () => {
    it("shares the same pending promise across concurrent callers", async () => {
        let resolveValue!: (value: string) => void;
        const pending = new Promise<string>((resolve) => {
            resolveValue = resolve;
        });
        const factory = vi.fn<() => Promise<string>>(() => pending);
        const load = createRetriableAsyncSingleton(factory);

        const first = load();
        const second = load();
        await Promise.resolve();

        expect(factory).toHaveBeenCalledTimes(1);
        resolveValue("ok");

        await expect(first).resolves.toBe("ok");
        await expect(second).resolves.toBe("ok");
    });

    it("retries after an asynchronous rejection", async () => {
        const factory = vi.fn<() => Promise<string>>()
            .mockRejectedValueOnce(new Error("temporary failure"))
            .mockResolvedValueOnce("recovered");
        const load = createRetriableAsyncSingleton(factory);

        await expect(load()).rejects.toThrow("temporary failure");
        await expect(load()).resolves.toBe("recovered");
        expect(factory).toHaveBeenCalledTimes(2);
    });

    it("retries after a synchronous throw", async () => {
        const factory = vi.fn<() => Promise<string>>()
            .mockImplementationOnce(() => {
                throw new Error("sync failure");
            })
            .mockResolvedValueOnce("recovered");
        const load = createRetriableAsyncSingleton(factory);

        await expect(load()).rejects.toThrow("sync failure");
        await expect(load()).resolves.toBe("recovered");
        expect(factory).toHaveBeenCalledTimes(2);
    });
});
