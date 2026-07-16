import {act} from "react";

export function setFieldValue(element: HTMLInputElement, value: string): void {
    const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value");
    const setValue = descriptor?.set;
    if (!setValue) {
        throw new Error("value setter is unavailable");
    }

    act(() => {
        setValue.call(element, value);
        element.dispatchEvent(new Event("input", {bubbles: true}));
        element.dispatchEvent(new Event("change", {bubbles: true}));
    });
}
