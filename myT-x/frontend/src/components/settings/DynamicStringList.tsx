import { useLayoutEffect, useRef, useState } from "react";

interface DynamicStringListProps {
  items: string[];
  onChange: (items: string[]) => void;
  placeholder?: string;
  addLabel?: string;
}

function sameItems(left: string[] | null, right: string[]): boolean {
  if (!left || left.length !== right.length) {
    return false;
  }
  for (let i = 0; i < left.length; i++) {
    if (left[i] !== right[i]) {
      return false;
    }
  }
  return true;
}

export function DynamicStringList({ items, onChange, placeholder, addLabel }: DynamicStringListProps) {
  const nextRowIDRef = useRef(items.length);
  const prevItemsRef = useRef(items);
  const expectedNextItemsRef = useRef<string[] | null>(null);
  const [rowIDs, setRowIDs] = useState<string[]>(() =>
    Array.from({ length: items.length }, (_, index) => `dsl-${index}`),
  );

  useLayoutEffect(() => {
    if (prevItemsRef.current === items) {
      return;
    }
    setRowIDs((currentRowIDs) => {
      const wasInternalUpdate = sameItems(expectedNextItemsRef.current, items);
      if (wasInternalUpdate) {
        expectedNextItemsRef.current = null;
      }
      if (!wasInternalUpdate) {
        // Parent replaced the list externally (e.g. LOAD_CONFIG).
        return Array.from({ length: items.length }, () => `dsl-${nextRowIDRef.current++}`);
      }
      if (currentRowIDs.length > items.length) {
        return currentRowIDs.slice(0, items.length);
      }
      if (currentRowIDs.length < items.length) {
        const next = [...currentRowIDs];
        while (next.length < items.length) {
          next.push(`dsl-${nextRowIDRef.current++}`);
        }
        return next;
      }
      return currentRowIDs;
    });
    prevItemsRef.current = items;
  }, [items]);

  return (
    <div className="dynamic-list">
      {items.map((item, index) => (
        <div key={rowIDs[index] ?? `dsl-pending-${index}`} className="dynamic-list-row">
          <input
            className="form-input"
            value={item}
            aria-label={placeholder ? `${placeholder} ${index + 1}` : `Item ${index + 1}`}
            onChange={(e) => {
              const next = [...items];
              next[index] = e.target.value;
              expectedNextItemsRef.current = next;
              onChange(next);
            }}
            placeholder={placeholder}
          />
          <button
            type="button"
            className="dynamic-list-remove"
            onClick={() => {
              const next = items.filter((_, i) => i !== index);
              expectedNextItemsRef.current = next;
              onChange(next);
            }}
            title="削除"
          >
            &times;
          </button>
        </div>
      ))}
      <button
        type="button"
        className="modal-btn dynamic-list-add"
        onClick={() => {
          const next = [...items, ""];
          expectedNextItemsRef.current = next;
          onChange(next);
        }}
      >
        + {addLabel || "追加"}
      </button>
    </div>
  );
}
