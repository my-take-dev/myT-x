import {beforeEach, describe, expect, it} from "vitest";
import {useCanvasStore} from "./canvasStore";

/** ストアを初期状態にリセットする（replace=true でアクションも含め完全上書き）。 */
function resetStore(): void {
    useCanvasStore.setState(
        {
            ...useCanvasStore.getState(),
            mode: "simple",
            activeSessionName: null,
            nodePositions: {},
            nodeSizes: {},
            taskEdgeMap: {},
            agentMap: {},
            processStatusMap: {},
            sessionDataMap: {},
        },
        true,
    );
}

beforeEach(() => {
    resetStore();
});

describe("resetForSession", () => {
    it("同一セッションなら no-op", () => {
        const store = useCanvasStore.getState();
        store.resetForSession("session-A");

        // サイズを設定
        useCanvasStore.getState().setNodeSize("%0", {width: 600, height: 400});

        // 同一セッションで resetForSession → サイズが維持される
        useCanvasStore.getState().resetForSession("session-A");

        const state = useCanvasStore.getState();
        expect(state.nodeSizes["%0"]).toEqual({width: 600, height: 400});
        expect(state.activeSessionName).toBe("session-A");
    });

    it("セッション切替時に現セッションを保存し対象を復元する", () => {
        const store = useCanvasStore.getState();

        // セッションAにデータを設定
        store.resetForSession("session-A");
        useCanvasStore.getState().setNodeSize("%0", {width: 500, height: 300});
        useCanvasStore.getState().setNodePosition("%0", {x: 100, y: 200});

        // セッションBに切替 → Aのデータが保存される
        useCanvasStore.getState().resetForSession("session-B");
        const afterSwitch = useCanvasStore.getState();
        expect(afterSwitch.activeSessionName).toBe("session-B");
        expect(afterSwitch.nodeSizes).toEqual({});
        expect(afterSwitch.nodePositions).toEqual({});
        // Aのデータが sessionDataMap に保存されている
        expect(afterSwitch.sessionDataMap["session-A"]).toBeDefined();
        expect(afterSwitch.sessionDataMap["session-A"].nodeSizes["%0"]).toEqual({width: 500, height: 300});

        // セッションBにデータを設定
        useCanvasStore.getState().setNodeSize("%1", {width: 700, height: 500});

        // セッションAに戻る → Aのデータが復元される
        useCanvasStore.getState().resetForSession("session-A");
        const restored = useCanvasStore.getState();
        expect(restored.activeSessionName).toBe("session-A");
        expect(restored.nodeSizes["%0"]).toEqual({width: 500, height: 300});
        expect(restored.nodePositions["%0"]).toEqual({x: 100, y: 200});

        // Bのデータも sessionDataMap に保存されている
        expect(restored.sessionDataMap["session-B"]).toBeDefined();
        expect(restored.sessionDataMap["session-B"].nodeSizes["%1"]).toEqual({width: 700, height: 500});

        // 復元済みのAは sessionDataMap から除去されている（二重保持防止）
        expect(restored.sessionDataMap["session-A"]).toBeUndefined();
    });

    it("初回セッション設定（activeSessionName=null）では保存をスキップする", () => {
        useCanvasStore.getState().resetForSession("session-A");
        const state = useCanvasStore.getState();
        expect(state.activeSessionName).toBe("session-A");
        expect(Object.keys(state.sessionDataMap)).toHaveLength(0);
    });
});

describe("clearSessionData", () => {
    it("非アクティブセッションのデータを削除する", () => {
        const store = useCanvasStore.getState();

        // セッションAにデータを作成
        store.resetForSession("session-A");
        useCanvasStore.getState().setNodeSize("%0", {width: 500, height: 300});

        // セッションBに切替（Aが sessionDataMap に保存される）
        useCanvasStore.getState().resetForSession("session-B");
        expect(useCanvasStore.getState().sessionDataMap["session-A"]).toBeDefined();

        // セッションAのデータを削除
        useCanvasStore.getState().clearSessionData("session-A");
        expect(useCanvasStore.getState().sessionDataMap["session-A"]).toBeUndefined();
        // アクティブセッション（B）のフラット状態は影響なし
        expect(useCanvasStore.getState().activeSessionName).toBe("session-B");
    });

    it("アクティブセッションのデータを削除するとフラット状態もクリアする", () => {
        const store = useCanvasStore.getState();

        store.resetForSession("session-A");
        useCanvasStore.getState().setNodeSize("%0", {width: 500, height: 300});
        useCanvasStore.getState().setNodePosition("%0", {x: 100, y: 200});

        // アクティブセッション自身を削除
        useCanvasStore.getState().clearSessionData("session-A");
        const state = useCanvasStore.getState();
        expect(state.activeSessionName).toBeNull();
        expect(state.nodeSizes).toEqual({});
        expect(state.nodePositions).toEqual({});
        expect(state.taskEdgeMap).toEqual({});
        expect(state.sessionDataMap["session-A"]).toBeUndefined();
    });
});

describe("セッション間の参照独立性", () => {
    it("新規セッション初期化で他セッションのデータが混入しない", () => {
        // セッションAを初期化（空データで開始）
        useCanvasStore.getState().resetForSession("session-A");
        // セッションBに切替
        useCanvasStore.getState().resetForSession("session-B");
        // セッションBにデータを追加
        useCanvasStore.getState().setNodePosition("%0", {x: 50, y: 60});

        // セッションCに切替（新規、保存データなし）
        useCanvasStore.getState().resetForSession("session-C");
        const state = useCanvasStore.getState();
        // Cのフラット状態にBのデータが混入していないこと
        expect(state.nodePositions).toEqual({});
        expect(state.nodeSizes).toEqual({});
    });
});

describe("モード切替によるサイズ維持", () => {
    it("Simple↔Canvas切替でnodeSizesが維持される", () => {
        const store = useCanvasStore.getState();

        store.resetForSession("session-A");
        useCanvasStore.getState().setNodeSize("%0", {width: 600, height: 400});

        // Canvas → Simple
        useCanvasStore.getState().setMode("simple");
        expect(useCanvasStore.getState().nodeSizes["%0"]).toEqual({width: 600, height: 400});

        // Simple → Canvas
        useCanvasStore.getState().setMode("canvas");
        expect(useCanvasStore.getState().nodeSizes["%0"]).toEqual({width: 600, height: 400});
    });
});
