declare module "wavedrom" {
    interface WavedromApi {
        renderAny(index: number, source: object, lane: unknown): Element;
        waveSkin: Record<string, unknown>;
    }

    const wavedrom: WavedromApi;
    export default wavedrom;
}
