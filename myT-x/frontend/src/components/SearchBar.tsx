import { useEffect, useRef, useState } from "react";
import type { SearchAddon } from "@xterm/addon-search";

interface SearchBarProps {
  open: boolean;
  onClose: () => void;
  searchAddon: SearchAddon | null;
}

export function SearchBar({ open, onClose, searchAddon }: SearchBarProps) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [query, setQuery] = useState("");

  useEffect(() => {
    if (open) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [open]);

  if (!open) return null;

  const findNext = () => {
    if (query && searchAddon) {
      searchAddon.findNext(query, { caseSensitive: false, regex: false });
    }
  };

  const findPrev = () => {
    if (query && searchAddon) {
      searchAddon.findPrevious(query, { caseSensitive: false, regex: false });
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      searchAddon?.clearDecorations();
      onClose();
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      if (e.shiftKey) {
        findPrev();
      } else {
        findNext();
      }
    }
  };

  return (
    <div className="terminal-search-bar">
      <input
        ref={inputRef}
        className="terminal-search-input"
        type="text"
        placeholder="Search..."
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          if (e.target.value && searchAddon) {
            searchAddon.findNext(e.target.value, { caseSensitive: false, regex: false });
          }
        }}
        onKeyDown={handleKeyDown}
      />
      <button
        type="button"
        className="terminal-search-btn"
        title="Previous (Shift+Enter)"
        onClick={findPrev}
      >
        &#x25B2;
      </button>
      <button
        type="button"
        className="terminal-search-btn"
        title="Next (Enter)"
        onClick={findNext}
      >
        &#x25BC;
      </button>
      <button
        type="button"
        className="terminal-search-btn terminal-search-btn-close"
        title="Close (Esc)"
        onClick={() => {
          searchAddon?.clearDecorations();
          onClose();
        }}
      >
        &times;
      </button>
    </div>
  );
}
