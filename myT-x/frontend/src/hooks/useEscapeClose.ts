import { useEffect, useRef } from "react";

function shouldIgnoreEscape(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) {
    return false;
  }
  return target.closest('[data-escape-close-disabled="true"]') !== null;
}

/**
 * Registers a keydown listener that calls onClose when Escape is pressed.
 * The listener is only active while `active` is true.
 */
export function useEscapeClose(active: boolean, onClose: () => void): void {
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    if (!active) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape" || e.defaultPrevented || shouldIgnoreEscape(e.target)) {
        return;
      }
      onCloseRef.current();
    };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [active]);
}
