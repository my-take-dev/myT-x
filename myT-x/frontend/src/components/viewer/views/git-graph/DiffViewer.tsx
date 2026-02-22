import { useMemo, useState } from "react";

interface DiffViewerProps {
  diff: string;
}

interface DiffFile {
  header: string;
  hunks: DiffHunk[];
}

interface DiffHunk {
  header: string;
  lines: DiffLine[];
}

interface DiffLine {
  type: "context" | "added" | "removed" | "hunk-header";
  content: string;
  oldLineNum?: number;
  newLineNum?: number;
}

function parseDiff(raw: string): DiffFile[] {
  const files: DiffFile[] = [];
  const lines = raw.split("\n");
  let currentFile: DiffFile | null = null;
  let currentHunk: DiffHunk | null = null;
  let oldLine = 0;
  let newLine = 0;

  for (const line of lines) {
    if (line.startsWith("diff --git")) {
      currentFile = { header: line, hunks: [] };
      files.push(currentFile);
      currentHunk = null;
      continue;
    }

    if (!currentFile) continue;

    // Skip diff metadata lines.
    if (line.startsWith("index ") || line.startsWith("---") || line.startsWith("+++") ||
        line.startsWith("new file") || line.startsWith("deleted file") ||
        line.startsWith("similarity") || line.startsWith("rename")) {
      continue;
    }

    // Hunk header.
    if (line.startsWith("@@")) {
      const match = line.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
      if (!match) continue;
      oldLine = parseInt(match[1], 10);
      newLine = parseInt(match[2], 10);
      currentHunk = { header: line, lines: [] };
      currentFile.hunks.push(currentHunk);
      continue;
    }

    if (!currentHunk) continue;

    if (line.startsWith("+")) {
      currentHunk.lines.push({
        type: "added",
        content: line.substring(1),
        newLineNum: newLine++,
      });
    } else if (line.startsWith("-")) {
      currentHunk.lines.push({
        type: "removed",
        content: line.substring(1),
        oldLineNum: oldLine++,
      });
    } else {
      // Context line (or empty).
      currentHunk.lines.push({
        type: "context",
        content: line.startsWith(" ") ? line.substring(1) : line,
        oldLineNum: oldLine++,
        newLineNum: newLine++,
      });
    }
  }

  return files;
}

function DiffFileSection({ file }: { file: DiffFile }) {
  const [collapsed, setCollapsed] = useState(false);

  // Extract file path from header.
  const filePath = file.header.replace(/^diff --git a\/(.+?) b\/.+$/, "$1");

  return (
    <div>
      <div className="diff-file-header" onClick={() => setCollapsed(!collapsed)}>
        <span>{collapsed ? "\u25B6" : "\u25BC"} {filePath}</span>
      </div>
      {!collapsed && file.hunks.map((hunk, hi) => (
        <div key={hi}>
          <div className="diff-hunk-header">{hunk.header}</div>
          {hunk.lines.map((line, li) => (
            <div key={li} className={`diff-line ${line.type}`}>
              <span className="diff-line-number">
                {line.oldLineNum ?? ""}
              </span>
              <span className="diff-line-number">
                {line.newLineNum ?? ""}
              </span>
              <span className="diff-line-content">{line.content}</span>
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

export function DiffViewer({ diff }: DiffViewerProps) {
  const files = useMemo(() => parseDiff(diff), [diff]);

  if (files.length === 0) {
    return <div className="viewer-message">No diff available</div>;
  }

  return (
    <div className="diff-viewer">
      {files.map((file, i) => (
        <DiffFileSection key={i} file={file} />
      ))}
    </div>
  );
}
