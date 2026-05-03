// Extend this list only for sequences whose sole effect is scrollback purge.
// Invariant: no sequence may be a strict prefix of another sequence. The
// streaming matcher keeps incomplete suffixes pending, so prefix overlap would
// make a shorter complete sequence ambiguous with a longer incomplete one.
// RIS (ESC c), DECSTR (ESC [ ! p), and alternate-screen controls are preserved
// because they reset broader terminal state and may represent explicit user or
// application behavior rather than a scrollback-only clear request.
const SCROLLBACK_PURGE_SEQUENCES = ["\x1b[?3J", "\x1b[3J"] as const;

interface ReplaySanitizeResult {
    readonly output: string;
    readonly pendingPrefix: string;
}

const replayBoundaries = new Map<string, string>();
const panesWithLiveOutput = new Set<string>();

function assertNoStrictPrefixOverlap(sequences: readonly string[]): void {
    for (const sequence of sequences) {
        for (const candidate of sequences) {
            if (sequence !== candidate && candidate.startsWith(sequence)) {
                throw new Error("scrollback purge sequences must not have strict prefix overlap");
            }
        }
    }
}

assertNoStrictPrefixOverlap(SCROLLBACK_PURGE_SEQUENCES);

function startsWithAt(input: string, sequence: string, index: number): boolean {
    if (index + sequence.length > input.length) {
        return false;
    }
    for (let offset = 0; offset < sequence.length; offset++) {
        if (input.charCodeAt(index + offset) !== sequence.charCodeAt(offset)) {
            return false;
        }
    }
    return true;
}

function pendingPurgePrefixAt(input: string, index: number): string {
    const remainingLength = input.length - index;
    for (const sequence of SCROLLBACK_PURGE_SEQUENCES) {
        if (remainingLength > sequence.length) {
            continue;
        }
        let matches = true;
        for (let offset = 0; offset < remainingLength; offset++) {
            if (input.charCodeAt(index + offset) !== sequence.charCodeAt(offset)) {
                matches = false;
                break;
            }
        }
        if (matches) {
            return input.slice(index);
        }
    }
    return "";
}

export class TerminalOutputFilter {
    private pending = "";

    sanitize(chunk: string): string {
        if (chunk.length === 0) {
            return "";
        }

        const input = this.pending + chunk;
        this.pending = "";

        const parts: string[] = [];
        let index = 0;
        let emitStart = 0;
        while (index < input.length) {
            const matchedSequence = SCROLLBACK_PURGE_SEQUENCES.find((sequence) => {
                return startsWithAt(input, sequence, index);
            });
            if (matchedSequence !== undefined) {
                parts.push(input.slice(emitStart, index));
                index += matchedSequence.length;
                emitStart = index;
                continue;
            }

            const pendingPrefix = pendingPurgePrefixAt(input, index);
            if (pendingPrefix.length > 0) {
                parts.push(input.slice(emitStart, index));
                this.pending = pendingPrefix;
                break;
            }

            index++;
        }
        if (this.pending.length === 0) {
            parts.push(input.slice(emitStart));
        }

        return parts.join("");
    }

    flush(): string {
        const pending = this.pending;
        this.pending = "";
        return pending;
    }

    reset(): void {
        this.pending = "";
    }
}

export function sanitizeTerminalReplay(chunk: string): ReplaySanitizeResult {
    const filter = new TerminalOutputFilter();
    const output = filter.sanitize(chunk);
    return {output, pendingPrefix: filter.flush()};
}

export function canWritePaneReplay(paneId: string): boolean {
    return !panesWithLiveOutput.has(paneId);
}

export function setPaneReplayBoundaryPrefix(paneId: string, pendingPrefix: string): boolean {
    if (pendingPrefix.length === 0) {
        replayBoundaries.delete(paneId);
        return true;
    }
    if (panesWithLiveOutput.has(paneId)) {
        return false;
    }
    replayBoundaries.set(paneId, pendingPrefix);
    return true;
}

export function applyPaneReplayBoundary(paneId: string, liveChunk: string): string {
    if (liveChunk.length === 0) {
        return liveChunk;
    }
    panesWithLiveOutput.add(paneId);
    const pendingPrefix = replayBoundaries.get(paneId);
    if (pendingPrefix === undefined) {
        return liveChunk;
    }
    replayBoundaries.delete(paneId);

    const combined = pendingPrefix + liveChunk;
    const matchedSequence = SCROLLBACK_PURGE_SEQUENCES.find((sequence) => {
        return startsWithAt(combined, sequence, 0);
    });
    if (matchedSequence !== undefined) {
        return combined.slice(matchedSequence.length);
    }
    return combined;
}

export function clearPaneReplayBoundary(paneId: string): void {
    replayBoundaries.delete(paneId);
    panesWithLiveOutput.delete(paneId);
}
