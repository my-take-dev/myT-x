export function createRetriableAsyncSingleton<T>(
    factory: () => Promise<T>,
): () => Promise<T> {
    let promise: Promise<T> | null = null;

    return () => {
        if (promise === null) {
            promise = Promise.resolve()
                .then(factory)
                .catch((err: unknown) => {
                    promise = null;
                    throw err;
                });
        }
        return promise;
    };
}
