import {isImeTransitionalEvent} from "./ime";

/**
 * Returns true when xterm.js should continue handling the keyboard event via
 * attachCustomKeyEventHandler. xterm's contract is "true = process in xterm,
 * false = stop terminal processing", so IME composition/transitional events are
 * delegated to xterm's CompositionHelper pipeline.
 */
export function shouldLetXtermHandleImeEvent(event: KeyboardEvent, isComposing: boolean): boolean {
    return isComposing || isImeTransitionalEvent(event);
}
