import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it} from "vitest";
import type {ValidationRules} from "../src/types/tmux";
import {type AnchorPosition, CHAT_RATIO_DEFAULT, CHAT_RATIO_MAX, CHAT_RATIO_MIN, clampChatRatio, useChatResize} from "../src/components/useChatResize";

interface ResizeProbeProps {
    chatOverlayPercentage: number;
    validationRules?: ValidationRules | null;
}

type ResizeProbeState = ReturnType<typeof useChatResize>;

let currentState: ResizeProbeState | null = null;

function ResizeProbe({chatOverlayPercentage, validationRules}: ResizeProbeProps) {
    currentState = useChatResize({chatOverlayPercentage, validationRules});
    return null;
}

function getCurrentState(): ResizeProbeState {
    expect(currentState).not.toBeNull();
    return currentState as ResizeProbeState;
}

describe("useChatResize", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        currentState = null;
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("normalizes chat ratios from percentage inputs", () => {
        const cases = [
            {input: 0, expected: CHAT_RATIO_DEFAULT},
            {input: 10, expected: CHAT_RATIO_MIN},
            {input: 40, expected: CHAT_RATIO_DEFAULT},
            {input: 70, expected: CHAT_RATIO_MAX},
            {input: 90, expected: CHAT_RATIO_MAX},
        ];

        for (const testCase of cases) {
            act(() => {
                root.render(<ResizeProbe chatOverlayPercentage={testCase.input}/>);
            });
            expect(getCurrentState().chatRatio).toBe(testCase.expected);
        }
    });

    it("uses backend validation rules when provided", () => {
        const validationRules: ValidationRules = {
            min_override_name_len: 5,
            min_pre_exec_reset_delay: 0,
            max_pre_exec_reset_delay: 60,
            min_pre_exec_idle_timeout: 0,
            max_pre_exec_idle_timeout: 600,
            max_message_templates: 50,
            max_template_name_len: 80,
            max_template_message_len: 4000,
            min_single_task_runner_clear_delay: 0,
            max_single_task_runner_clear_delay: 300,
            min_chat_overlay_percentage: 20,
            max_chat_overlay_percentage: 80,
            default_chat_overlay_percentage: 50,
        };

        act(() => {
            root.render(<ResizeProbe chatOverlayPercentage={10} validationRules={validationRules}/>);
        });
        expect(getCurrentState().chatRatio).toBe(0.2);

        act(() => {
            getCurrentState().setChatRatio(0.95);
        });
        expect(getCurrentState().chatRatio).toBe(0.8);

        expect(clampChatRatio(NaN, validationRules)).toBe(0.5);
    });

    it("halves the current ratio and restores the previous full size", () => {
        act(() => {
            root.render(<ResizeProbe chatOverlayPercentage={70}/>);
        });

        act(() => {
            getCurrentState().toggleHalfSize();
        });
        expect(getCurrentState().chatRatio).toBe(CHAT_RATIO_MAX / 2);
        expect(getCurrentState().isHalfSize).toBe(true);

        act(() => {
            getCurrentState().toggleHalfSize();
        });
        expect(getCurrentState().chatRatio).toBe(CHAT_RATIO_MAX);
        expect(getCurrentState().isHalfSize).toBe(false);
    });

    it("switches anchor orientation and clears half-size mode", () => {
        act(() => {
            root.render(<ResizeProbe chatOverlayPercentage={40}/>);
        });

        act(() => {
            getCurrentState().toggleHalfSize();
        });
        expect(getCurrentState().isHalfSize).toBe(true);

        const anchors: AnchorPosition[] = ["left", "top", "right", "bottom"];
        for (const anchor of anchors) {
            act(() => {
                getCurrentState().changeAnchor(anchor);
            });
            expect(getCurrentState().anchor).toBe(anchor);
            expect(getCurrentState().isHalfSize).toBe(false);
            expect(getCurrentState().isHorizontal).toBe(anchor === "left" || anchor === "right");
        }
    });

    it("restores fullChatRatio when changing anchor during half-size mode", () => {
        act(() => {
            root.render(<ResizeProbe chatOverlayPercentage={40}/>);
        });

        const originalRatio = getCurrentState().chatRatio;
        expect(originalRatio).toBe(CHAT_RATIO_DEFAULT);

        act(() => {
            getCurrentState().toggleHalfSize();
        });
        expect(getCurrentState().chatRatio).toBe(CHAT_RATIO_DEFAULT / 2);
        expect(getCurrentState().isHalfSize).toBe(true);

        act(() => {
            getCurrentState().changeAnchor("left");
        });
        expect(getCurrentState().chatRatio).toBe(originalRatio);
        expect(getCurrentState().isHalfSize).toBe(false);
    });

    it("clamps setChatRatio to valid range and clears half-size", () => {
        act(() => {
            root.render(<ResizeProbe chatOverlayPercentage={40}/>);
        });

        act(() => {
            getCurrentState().toggleHalfSize();
        });
        expect(getCurrentState().isHalfSize).toBe(true);

        act(() => {
            getCurrentState().setChatRatio(0.05);
        });
        expect(getCurrentState().chatRatio).toBe(CHAT_RATIO_MIN);
        expect(getCurrentState().isHalfSize).toBe(false);

        act(() => {
            getCurrentState().setChatRatio(0.99);
        });
        expect(getCurrentState().chatRatio).toBe(CHAT_RATIO_MAX);

        act(() => {
            getCurrentState().setChatRatio(NaN);
        });
        expect(getCurrentState().chatRatio).toBe(CHAT_RATIO_DEFAULT);
    });

    it("resetChatRatio restores the default ratio", () => {
        act(() => {
            root.render(<ResizeProbe chatOverlayPercentage={60}/>);
        });
        const defaultRatio = getCurrentState().chatRatio;
        expect(defaultRatio).toBe(0.6);

        act(() => {
            getCurrentState().setChatRatio(0.3);
        });
        expect(getCurrentState().chatRatio).toBe(0.3);

        act(() => {
            getCurrentState().resetChatRatio();
        });
        expect(getCurrentState().chatRatio).toBe(defaultRatio);
        expect(getCurrentState().isHalfSize).toBe(false);
    });

    it("panelStyle switches between height and width based on anchor", () => {
        act(() => {
            root.render(<ResizeProbe chatOverlayPercentage={40}/>);
        });

        expect(getCurrentState().panelStyle).toEqual({height: "40%"});

        act(() => {
            getCurrentState().changeAnchor("left");
        });
        expect(getCurrentState().panelStyle).toEqual({width: "40%"});

        act(() => {
            getCurrentState().changeAnchor("right");
        });
        expect(getCurrentState().panelStyle).toEqual({width: "40%"});

        act(() => {
            getCurrentState().changeAnchor("top");
        });
        expect(getCurrentState().panelStyle).toEqual({height: "40%"});
    });
});

describe("clampChatRatio", () => {
    it.each([
        {input: 0, expected: CHAT_RATIO_MIN},
        {input: 0.5, expected: 0.5},
        {input: 0.01, expected: CHAT_RATIO_MIN},
        {input: 0.99, expected: CHAT_RATIO_MAX},
        {input: NaN, expected: CHAT_RATIO_DEFAULT},
        {input: Infinity, expected: CHAT_RATIO_DEFAULT},
        {input: -Infinity, expected: CHAT_RATIO_DEFAULT},
        {input: -1, expected: CHAT_RATIO_MIN},
    ])("clamps $input to $expected", ({input, expected}) => {
        expect(clampChatRatio(input)).toBe(expected);
    });
});
