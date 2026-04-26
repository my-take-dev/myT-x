import {beforeEach, describe, expect, it} from "vitest";
import type {DiffReviewComment} from "./diffReviewStore";
import {useDiffReviewStore} from "./diffReviewStore";

const ALPHA_SESSION_KEY = "session:1";
const BETA_SESSION_KEY = "session:2";

function resetStore(): void {
    useDiffReviewStore.setState(
        {
            ...useDiffReviewStore.getState(),
            comments: [],
            drafts: {},
            activeCommentLineKey: null,
        },
        true,
    );
}

beforeEach(() => {
    resetStore();
});

function createComment(overrides: Partial<Omit<DiffReviewComment, "id">> = {}): Omit<DiffReviewComment, "id"> {
    return {
        sessionKey: ALPHA_SESSION_KEY,
        filePath: "a.ts",
        startLineNum: 1,
        startLineType: "added",
        endLineNum: 1,
        endLineType: "added",
        lineContent: "const a = 1;",
        commentText: "first",
        ...overrides,
    };
}

describe("diffReviewStore", () => {
    it("removes only the requested comments after send completion", () => {
        const store = useDiffReviewStore.getState();

        store.addComment(createComment());
        store.addComment(createComment({
            filePath: "b.ts",
            startLineNum: 2,
            startLineType: "context",
            endLineNum: 2,
            endLineType: "context",
            lineContent: "return value;",
            commentText: "second",
        }));

        const [firstComment, secondComment] = useDiffReviewStore.getState().comments;
        expect(firstComment).toBeDefined();
        expect(secondComment).toBeDefined();

        useDiffReviewStore.getState().removeComments([firstComment!.id]);

        const remaining = useDiffReviewStore.getState().comments;
        expect(remaining).toHaveLength(1);
        expect(remaining[0]?.id).toBe(secondComment!.id);
    });

    it("removes only matching IDs within the requested session", () => {
        const store = useDiffReviewStore.getState();

        store.addComment(createComment({commentText: "alpha first"}));
        store.addComment(createComment({commentText: "alpha second"}));
        store.addComment(createComment({
            sessionKey: BETA_SESSION_KEY,
            filePath: "b.ts",
            startLineNum: 2,
            startLineType: "context",
            endLineNum: 2,
            endLineType: "context",
            lineContent: "return value;",
            commentText: "beta comment",
        }));

        const [alphaFirst, alphaSecond, betaComment] = useDiffReviewStore.getState().comments;
        expect(alphaFirst).toBeDefined();
        expect(alphaSecond).toBeDefined();
        expect(betaComment).toBeDefined();

        store.removeCommentsForSession([alphaFirst!.id, betaComment!.id], ALPHA_SESSION_KEY);

        const remaining = useDiffReviewStore.getState().comments;
        expect(remaining).toHaveLength(2);
        expect(remaining.map((comment) => comment.commentText)).toEqual(["alpha second", "beta comment"]);
    });

    it("preserves drafts when comments are cleared for the active session", () => {
        const store = useDiffReviewStore.getState();

        store.setActiveCommentLineKey("src/app.ts:row-1");
        store.setDraft("src/app.ts:row-1", "draft text");
        store.addComment(createComment({
            filePath: "src/app.ts",
            startLineNum: 10,
            startLineType: "context",
            endLineNum: 12,
            endLineType: "context",
            lineContent: "return nil\nreturn err",
            commentText: "stored comment",
        }));

        store.clearCommentsForSession(ALPHA_SESSION_KEY);

        const state = useDiffReviewStore.getState();
        expect(state.comments).toEqual([]);
        expect(state.drafts["src/app.ts:row-1"]).toBe("draft text");
        expect(state.activeCommentLineKey).toBe("src/app.ts:row-1");

        state.clearDraft("src/app.ts:row-1");
        expect(useDiffReviewStore.getState().drafts["src/app.ts:row-1"]).toBeUndefined();
    });

    it("clears only comments that belong to the active session", () => {
        const store = useDiffReviewStore.getState();

        store.addComment(createComment({commentText: "alpha comment"}));
        store.addComment(createComment({
            sessionKey: BETA_SESSION_KEY,
            filePath: "b.ts",
            startLineNum: 2,
            startLineType: "context",
            endLineNum: 2,
            endLineType: "context",
            lineContent: "return value;",
            commentText: "beta comment",
        }));

        store.clearCommentsForSession(ALPHA_SESSION_KEY);

        const remaining = useDiffReviewStore.getState().comments;
        expect(remaining).toHaveLength(1);
        expect(remaining[0]?.sessionKey).toBe(BETA_SESSION_KEY);
    });

    it("clears drafts and active line state for the requested session", () => {
        const store = useDiffReviewStore.getState();

        store.setActiveCommentLineKey("session:1::src/app.ts\u001fhunk:10:12:0");
        store.setDraft("session:1::src/app.ts\u001fhunk:10:12:0", "alpha draft");
        store.setDraft("session:2::src/app.ts\u001fhunk:10:12:0", "beta draft");

        store.clearTransientStateForSession(ALPHA_SESSION_KEY);

        const state = useDiffReviewStore.getState();
        expect(state.activeCommentLineKey).toBeNull();
        expect(state.drafts["session:1::src/app.ts\u001fhunk:10:12:0"]).toBeUndefined();
        expect(state.drafts["session:2::src/app.ts\u001fhunk:10:12:0"]).toBe("beta draft");
    });

    it("clears comments and drafts together when a session is destroyed", () => {
        const store = useDiffReviewStore.getState();

        store.addComment(createComment({commentText: "alpha"}));
        store.addComment(createComment({sessionKey: BETA_SESSION_KEY, commentText: "beta"}));
        store.setActiveCommentLineKey("session:1::src/app.ts\u001fhunk:10:12:0");
        store.setDraft("session:1::src/app.ts\u001fhunk:10:12:0", "alpha draft");
        store.setDraft("session:2::src/app.ts\u001fhunk:10:12:0", "beta draft");

        store.clearSessionState(ALPHA_SESSION_KEY);

        const state = useDiffReviewStore.getState();
        expect(state.comments).toHaveLength(1);
        expect(state.comments[0]?.commentText).toBe("beta");
        expect(state.activeCommentLineKey).toBeNull();
        expect(state.drafts["session:1::src/app.ts\u001fhunk:10:12:0"]).toBeUndefined();
        expect(state.drafts["session:2::src/app.ts\u001fhunk:10:12:0"]).toBe("beta draft");
    });

    it("ignores invalid comments at the store boundary", () => {
        const store = useDiffReviewStore.getState();

        store.addComment(createComment({sessionKey: "", commentText: "orphan"}));
        store.addComment(createComment({filePath: "   ", commentText: "missing file"}));
        store.addComment(createComment({commentText: "   "}));

        expect(useDiffReviewStore.getState().comments).toEqual([]);
    });
});
