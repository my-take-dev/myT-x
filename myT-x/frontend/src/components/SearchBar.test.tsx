import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {SearchAddon} from "@xterm/addon-search";
import {SearchBar} from "./SearchBar";

interface SearchResultsEvent {
    readonly resultCount: number;
    readonly resultIndex: number;
}

type SearchResultsListener = (event: SearchResultsEvent) => void;

function keyboardEvent(init: KeyboardEventInit & { keyCode?: number } = {}): KeyboardEvent {
    const event = new KeyboardEvent("keydown", init);
    if (typeof init.keyCode === "number") {
        Object.defineProperty(event, "keyCode", {value: init.keyCode});
    }
    return event;
}

function setInputValue(input: HTMLInputElement, value: string): void {
    const valueSetter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
    valueSetter?.call(input, value);
    input.dispatchEvent(new Event("input", {bubbles: true}));
}

function createSearchAddonMock() {
    let listener: SearchResultsListener | null = null;
    const clearDecorations = vi.fn();
    const findNext = vi.fn();
    const findPrevious = vi.fn();
    const onDidChangeResults = vi.fn((nextListener: SearchResultsListener) => {
        listener = nextListener;
        return {
            dispose: vi.fn(() => {
                if (listener === nextListener) {
                    listener = null;
                }
            }),
        };
    });

    return {
        addon: {
            clearDecorations,
            findNext,
            findPrevious,
            onDidChangeResults,
        } as unknown as SearchAddon,
        clearDecorations,
        findNext,
        findPrevious,
        emitResults(event: SearchResultsEvent) {
            listener?.(event);
        },
    };
}

describe("SearchBar", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        consoleWarnSpy.mockRestore();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("uses incremental decorated search and renders result counts", async () => {
        const searchAddon = createSearchAddonMock();

        await act(async () => {
            root.render(<SearchBar open onClose={() => {}} searchAddon={searchAddon.addon}/>);
        });

        const input = container.querySelector("input");
        expect(input).not.toBeNull();

        await act(async () => {
            setInputValue(input as HTMLInputElement, "error");
        });

        expect(searchAddon.findNext).toHaveBeenCalledWith("error", expect.objectContaining({
            incremental: true,
            regex: false,
            wholeWord: false,
            decorations: expect.objectContaining({
                matchOverviewRuler: expect.any(String),
                activeMatchColorOverviewRuler: expect.any(String),
            }),
        }));

        await act(async () => {
            searchAddon.emitResults({resultCount: 17, resultIndex: 2});
        });

        expect(container.querySelector(".terminal-search-results")?.textContent).toBe("3 / 17");
    });

    it("shows an overflow-safe label when the match limit is exceeded", async () => {
        const searchAddon = createSearchAddonMock();

        await act(async () => {
            root.render(<SearchBar open onClose={() => {}} searchAddon={searchAddon.addon}/>);
        });

        await act(async () => {
            searchAddon.emitResults({resultCount: 1000, resultIndex: -1});
        });

        const results = container.querySelector(".terminal-search-results");
        expect(results?.textContent).toBe("- / 1000+");
        expect(results?.getAttribute("title")).toBe("検索結果（上限超過）");
    });

    it("resets stale query state when reopened", async () => {
        const searchAddon = createSearchAddonMock();

        await act(async () => {
            root.render(<SearchBar open onClose={() => {}} searchAddon={searchAddon.addon}/>);
        });

        const input = container.querySelector("input") as HTMLInputElement | null;
        expect(input).not.toBeNull();

        await act(async () => {
            setInputValue(input!, "warn");
        });
        await act(async () => {
            searchAddon.emitResults({resultCount: 5, resultIndex: 1});
        });

        await act(async () => {
            root.render(<SearchBar open={false} onClose={() => {}} searchAddon={searchAddon.addon}/>);
        });
        await act(async () => {
            root.render(<SearchBar open onClose={() => {}} searchAddon={searchAddon.addon}/>);
        });

        const reopenedInput = container.querySelector("input") as HTMLInputElement | null;
        expect(reopenedInput?.value).toBe("");
        expect(container.querySelector(".terminal-search-results")?.textContent).toBe("0 / 0");
        expect(searchAddon.clearDecorations).toHaveBeenCalledTimes(2);
    });

    it("swallows disposed addon errors during search interactions", async () => {
        const searchAddon = createSearchAddonMock();
        searchAddon.findNext.mockImplementation(() => {
            throw new Error("disposed");
        });

        await act(async () => {
            root.render(<SearchBar open onClose={() => {}} searchAddon={searchAddon.addon}/>);
        });

        const input = container.querySelector("input") as HTMLInputElement | null;
        expect(input).not.toBeNull();

        await act(async () => {
            setInputValue(input!, "panic");
        });

        expect(consoleWarnSpy).toHaveBeenCalledWith(
            "[DEBUG-search] searchAddon operation failed (possibly disposed)",
            expect.any(Error),
        );
    });

    it("does not trigger Enter navigation while IME composition is transitional", async () => {
        const searchAddon = createSearchAddonMock();

        await act(async () => {
            root.render(<SearchBar open onClose={() => {}} searchAddon={searchAddon.addon}/>);
        });

        const input = container.querySelector("input") as HTMLInputElement | null;
        expect(input).not.toBeNull();

        await act(async () => {
            setInputValue(input!, "日本語");
        });
        searchAddon.findNext.mockClear();

        await act(async () => {
            input?.dispatchEvent(keyboardEvent({key: "Enter", isComposing: true, bubbles: true, cancelable: true}));
        });

        expect(searchAddon.findNext).not.toHaveBeenCalled();
        expect(searchAddon.findPrevious).not.toHaveBeenCalled();
    });

    it("does not close on Escape while IME conversion is transitional", async () => {
        const searchAddon = createSearchAddonMock();
        const onClose = vi.fn();

        await act(async () => {
            root.render(<SearchBar open onClose={onClose} searchAddon={searchAddon.addon}/>);
        });

        const input = container.querySelector("input") as HTMLInputElement | null;
        expect(input).not.toBeNull();
        searchAddon.clearDecorations.mockClear();

        await act(async () => {
            input?.dispatchEvent(keyboardEvent({key: "Escape", keyCode: 229, bubbles: true, cancelable: true}));
        });

        expect(onClose).not.toHaveBeenCalled();
        expect(searchAddon.clearDecorations).not.toHaveBeenCalled();
    });
});
