import {beforeEach, describe, expect, it} from "vitest";
import {useTmuxStore} from "../src/stores/tmuxStore";

describe("IME recovery signal", () => {
    beforeEach(() => {
        useTmuxStore.setState({imeResetSignal: 0});
    });

    it("imeResetSignal starts at 0", () => {
        expect(useTmuxStore.getState().imeResetSignal).toBe(0);
    });

    it("triggerImeReset increments signal by 1", () => {
        useTmuxStore.getState().triggerImeReset();
        expect(useTmuxStore.getState().imeResetSignal).toBe(1);
    });

    it("multiple triggers increment monotonically", () => {
        const trigger = useTmuxStore.getState().triggerImeReset;
        trigger();
        trigger();
        trigger();
        expect(useTmuxStore.getState().imeResetSignal).toBe(3);
    });

    it("store subscription fires on signal change", () => {
        const signals: number[] = [];
        const unsub = useTmuxStore.subscribe((state) => {
            signals.push(state.imeResetSignal);
        });
        useTmuxStore.getState().triggerImeReset();
        useTmuxStore.getState().triggerImeReset();
        unsub();
        expect(signals).toEqual([1, 2]);
    });

    it("subscription with last-signal tracking detects changes", () => {
        let lastSignal = useTmuxStore.getState().imeResetSignal;
        let resetCount = 0;
        const unsub = useTmuxStore.subscribe((state) => {
            if (state.imeResetSignal !== lastSignal) {
                lastSignal = state.imeResetSignal;
                resetCount++;
            }
        });

        // Unrelated store update should not trigger reset
        useTmuxStore.getState().setPrefixMode(true);
        expect(resetCount).toBe(0);

        // IME reset trigger should fire
        useTmuxStore.getState().triggerImeReset();
        expect(resetCount).toBe(1);

        unsub();
    });
});
