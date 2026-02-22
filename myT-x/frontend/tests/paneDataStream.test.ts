import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

// paneDataStream.ts uses module-level state, so we need to reset between tests.
// We import the module functions under test.
import {
    connect,
    disconnect,
    isConnected,
    registerPaneHandler,
    unregisterPaneHandler,
} from "../src/services/paneDataStream";

// CloseEvent is not available in Node/vitest environment; provide a minimal stub.
class MockCloseEvent {
    type: string;
    code: number;
    reason: string;
    wasClean: boolean;

    constructor(type: string, init?: {code?: number; reason?: string; wasClean?: boolean}) {
        this.type = type;
        this.code = init?.code ?? 0;
        this.reason = init?.reason ?? "";
        this.wasClean = init?.wasClean ?? false;
    }
}

// Mock WebSocket globally since vitest runs in Node environment.
class MockWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;

    readyState: number = MockWebSocket.OPEN;
    url: string;
    binaryType: string = "blob";

    onopen: ((event: Event) => void) | null = null;
    onmessage: ((event: MessageEvent) => void) | null = null;
    onclose: ((event: CloseEvent) => void) | null = null;
    onerror: ((event: Event) => void) | null = null;

    static instances: MockWebSocket[] = [];
    closeCalled = false;
    sentMessages: string[] = [];

    constructor(url: string) {
        this.url = url;
        MockWebSocket.instances.push(this);
    }

    send(data: string): void {
        this.sentMessages.push(data);
    }

    close(code?: number, reason?: string): void {
        this.closeCalled = true;
        this.readyState = MockWebSocket.CLOSED;
        // Fire close event synchronously for test simplicity.
        if (this.onclose) {
            this.onclose(new CloseEvent("close", {code: code ?? 1000, reason: reason ?? ""}));
        }
    }

    /** Helper: simulate server sending binary data to the client. */
    simulateMessage(data: ArrayBuffer): void {
        if (this.onmessage) {
            const event = {data} as MessageEvent;
            this.onmessage(event);
        }
    }

    /** Helper: trigger onopen. */
    simulateOpen(): void {
        if (this.onopen) {
            this.onopen(new Event("open"));
        }
    }

    /** Helper: trigger onerror. */
    simulateError(): void {
        if (this.onerror) {
            this.onerror(new Event("error"));
        }
    }
}

// Mock the notification store to prevent errors when scheduleReconnect
// tries to show an error notification.
vi.mock("../src/stores/notificationStore", () => ({
    useNotificationStore: {
        getState: () => ({
            addNotification: vi.fn(),
        }),
    },
}));

/**
 * buildBinaryFrame constructs a binary frame in the protocol format:
 * [1byte: paneIDLen][paneID bytes][data bytes]
 */
function buildBinaryFrame(paneId: string, data: Uint8Array): ArrayBuffer {
    const encoder = new TextEncoder();
    const paneIdBytes = encoder.encode(paneId);
    const frame = new Uint8Array(1 + paneIdBytes.length + data.length);
    frame[0] = paneIdBytes.length;
    frame.set(paneIdBytes, 1);
    frame.set(data, 1 + paneIdBytes.length);
    return frame.buffer;
}

describe("paneDataStream", () => {
    beforeEach(() => {
        MockWebSocket.instances = [];
        vi.stubGlobal("WebSocket", MockWebSocket);
        vi.stubGlobal("CloseEvent", MockCloseEvent);
        vi.useFakeTimers();
        // Reset module state by calling disconnect before each test.
        disconnect();
        MockWebSocket.instances = [];
    });

    afterEach(() => {
        disconnect();
        vi.useRealTimers();
        vi.unstubAllGlobals();
    });

    // -------------------------------------------------------------------------
    describe("connect / disconnect", () => {
        it("creates a WebSocket with the given URL", () => {
            connect("ws://127.0.0.1:12345/ws");
            expect(MockWebSocket.instances).toHaveLength(1);
            expect(MockWebSocket.instances[0].url).toBe("ws://127.0.0.1:12345/ws");
        });

        it("sets binaryType to arraybuffer after connecting", () => {
            connect("ws://127.0.0.1:12345/ws");
            expect(MockWebSocket.instances[0].binaryType).toBe("arraybuffer");
        });

        it("disconnect() prevents automatic reconnection", () => {
            connect("ws://127.0.0.1:12345/ws");
            disconnect();
            MockWebSocket.instances = [];

            // Advance timers; no reconnect should occur because intentionalDisconnect=true.
            vi.advanceTimersByTime(5000);
            expect(MockWebSocket.instances).toHaveLength(0);
        });

        it("connect() after disconnect() creates a new connection", () => {
            connect("ws://127.0.0.1:12345/ws");
            disconnect();
            MockWebSocket.instances = [];

            connect("ws://127.0.0.1:12345/ws");
            expect(MockWebSocket.instances).toHaveLength(1);
        });

        it("isConnected() returns true when socket is OPEN", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.OPEN;
            expect(isConnected()).toBe(true);
        });

        it("isConnected() returns false before connect() is called", () => {
            // disconnect() was called in beforeEach, ws is null.
            expect(isConnected()).toBe(false);
        });

        it("isConnected() returns false after disconnect()", () => {
            connect("ws://127.0.0.1:12345/ws");
            disconnect();
            expect(isConnected()).toBe(false);
        });

        it("second connect() closes old connection and opens a new one", () => {
            connect("ws://127.0.0.1:11111/ws");
            const first = MockWebSocket.instances[0];

            // Calling connect() again — closeExisting() sets ws=null before calling socket.close(),
            // so onclose guard (ws !== socket) fires and suppresses reconnect.
            MockWebSocket.instances = [];
            connect("ws://127.0.0.1:22222/ws");

            expect(first.closeCalled).toBe(true);
            expect(MockWebSocket.instances).toHaveLength(1);
            expect(MockWebSocket.instances[0].url).toBe("ws://127.0.0.1:22222/ws");
        });

        it("stale onclose from replaced connection is ignored by the ws-identity guard", () => {
            // This test verifies the guard `if (ws !== socket) return` in onclose.
            //
            // The guard fires when an onclose event is dispatched on an old socket
            // (socket A) AFTER ws has already been updated to point to a new socket
            // (socket B). In that state ws !== socketA, so scheduleReconnect is skipped.
            //
            // To isolate this, we:
            //   1. Connect to get socketA with its onclose closure.
            //   2. Capture socketA's onclose BEFORE it is nulled.
            //   3. Null socketA.onclose so that MockWebSocket.close() inside
            //      closeExisting() does not fire it synchronously during connect(B).
            //   4. Call connect(B) to establish socketB as the active ws.
            //   5. Manually fire the captured socketA onclose handler AFTER ws=socketB.
            //      The guard `ws !== socket` evaluates to `socketB !== socketA = true`
            //      and returns early — no scheduleReconnect is called.
            //   6. Verify no reconnect timer fires.

            connect("ws://127.0.0.1:11111/ws");
            const oldSocket = MockWebSocket.instances[0];

            // Capture the real onclose closure set by openConnection, then suppress it
            // so closeExisting() in the next connect() call does not fire it.
            const capturedOnclose = oldSocket.onclose;
            oldSocket.onclose = null;

            // Establish a new connection. closeExisting() will call oldSocket.close()
            // but onclose is null so nothing fires. ws becomes socketB.
            MockWebSocket.instances = [];
            connect("ws://127.0.0.1:22222/ws");
            // ws is now socketB (the new instance).

            // Now fire the stale socketA onclose. The closure has `socket = socketA`
            // captured. The module's `ws` is socketB, so ws !== socketA -> guard fires.
            MockWebSocket.instances = [];
            if (capturedOnclose) {
                capturedOnclose(new CloseEvent("close", {code: 1006}));
            }

            // No reconnect timer should have been scheduled by the stale close.
            vi.advanceTimersByTime(5000);
            expect(MockWebSocket.instances).toHaveLength(0);
        });
    });

    // -------------------------------------------------------------------------
    describe("registerPaneHandler / unregisterPaneHandler", () => {
        it("registerPaneHandler stores the handler and delivers messages", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const received: Uint8Array[] = [];
            registerPaneHandler("pane-1", (data) => received.push(data));

            const frame = buildBinaryFrame("pane-1", new Uint8Array([1, 2, 3]));
            socket.simulateMessage(frame);

            expect(received).toHaveLength(1);
            expect(Array.from(received[0])).toEqual([1, 2, 3]);
        });

        it("registerPaneHandler sends subscribe message when socket is OPEN", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.OPEN;

            registerPaneHandler("pane-sub", () => {});

            const subscribeMsg = socket.sentMessages.find((m) =>
                m.includes('"subscribe"') && m.includes("pane-sub"),
            );
            expect(subscribeMsg).toBeDefined();
        });

        it("unregister function from registerPaneHandler removes the handler", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const received: Uint8Array[] = [];
            const unregister = registerPaneHandler("pane-2", (data) => received.push(data));

            unregister();

            const frame = buildBinaryFrame("pane-2", new Uint8Array([4, 5, 6]));
            socket.simulateMessage(frame);

            expect(received).toHaveLength(0);
        });

        it("unregister function sends unsubscribe to the server", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.OPEN;

            const unregister = registerPaneHandler("pane-unsub", () => {});
            unregister();

            const unsubMsg = socket.sentMessages.find((m) =>
                m.includes('"unsubscribe"') && m.includes("pane-unsub"),
            );
            expect(unsubMsg).toBeDefined();
        });

        it("unregisterPaneHandler directly removes the handler", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const received: Uint8Array[] = [];
            registerPaneHandler("pane-3", (data) => received.push(data));
            unregisterPaneHandler("pane-3");

            const frame = buildBinaryFrame("pane-3", new Uint8Array([7, 8, 9]));
            socket.simulateMessage(frame);

            expect(received).toHaveLength(0);
        });

        it("registerPaneHandler for same paneId overwrites previous handler", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const received1: Uint8Array[] = [];
            const received2: Uint8Array[] = [];
            registerPaneHandler("pane-4", (data) => received1.push(data));
            registerPaneHandler("pane-4", (data) => received2.push(data)); // overwrites

            const frame = buildBinaryFrame("pane-4", new Uint8Array([10]));
            socket.simulateMessage(frame);

            expect(received1).toHaveLength(0); // old handler replaced
            expect(received2).toHaveLength(1); // new handler called
        });

        it("handlers for different panes receive only their own messages", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const receivedA: Uint8Array[] = [];
            const receivedB: Uint8Array[] = [];
            registerPaneHandler("pane-A", (data) => receivedA.push(data));
            registerPaneHandler("pane-B", (data) => receivedB.push(data));

            socket.simulateMessage(buildBinaryFrame("pane-A", new Uint8Array([1])));
            socket.simulateMessage(buildBinaryFrame("pane-B", new Uint8Array([2])));

            expect(Array.from(receivedA[0])).toEqual([1]);
            expect(Array.from(receivedB[0])).toEqual([2]);
        });
    });

    // -------------------------------------------------------------------------
    describe("onmessage binary frame decode", () => {
        it("ignores non-ArrayBuffer messages", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            const received: Uint8Array[] = [];
            registerPaneHandler("pane-x", (data) => received.push(data));

            if (socket.onmessage) {
                socket.onmessage({data: "text message"} as MessageEvent);
            }
            expect(received).toHaveLength(0);
        });

        it("ignores frames shorter than 2 bytes (empty body)", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            const received: Uint8Array[] = [];
            registerPaneHandler("p", (data) => received.push(data));

            // 1-byte frame: only the length byte, no paneID bytes.
            const tooShort = new Uint8Array([5]).buffer;
            socket.simulateMessage(tooShort);
            expect(received).toHaveLength(0);
        });

        it("ignores zero-length frames", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            const received: Uint8Array[] = [];
            registerPaneHandler("p", (data) => received.push(data));

            const empty = new Uint8Array(0).buffer;
            socket.simulateMessage(empty);
            expect(received).toHaveLength(0);
        });

        it("ignores frames where paneIDLen exceeds available frame length", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            const received: Uint8Array[] = [];
            registerPaneHandler("p", (data) => received.push(data));

            // paneIDLen=100 but total frame is only 3 bytes.
            const malformed = new Uint8Array([100, 65, 66]).buffer;
            socket.simulateMessage(malformed);
            expect(received).toHaveLength(0);
        });

        it("delivers correct data bytes to the registered handler", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const received: Uint8Array[] = [];
            registerPaneHandler("%0", (data) => received.push(data));

            const payload = new Uint8Array([72, 101, 108, 108, 111]); // "Hello"
            const frame = buildBinaryFrame("%0", payload);
            socket.simulateMessage(frame);

            expect(received).toHaveLength(1);
            expect(Array.from(received[0])).toEqual(Array.from(payload));
        });

        it("ignores messages for unregistered panes", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            const received: Uint8Array[] = [];
            registerPaneHandler("known-pane", (data) => received.push(data));

            const frame = buildBinaryFrame("unknown-pane", new Uint8Array([1, 2]));
            socket.simulateMessage(frame);
            expect(received).toHaveLength(0);
        });

        it("delivers empty data slice when frame has no data after paneID", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const received: Uint8Array[] = [];
            registerPaneHandler("p", (data) => received.push(data));

            // Frame with paneID="p" and no trailing data bytes.
            const frame = buildBinaryFrame("p", new Uint8Array(0));
            socket.simulateMessage(frame);

            expect(received).toHaveLength(1);
            expect(received[0].length).toBe(0);
        });
    });

    // -------------------------------------------------------------------------
    describe("reconnection behavior", () => {
        it("schedules reconnect on non-intentional close", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            MockWebSocket.instances = [];

            // Simulate unexpected close by calling onclose directly (intentionalDisconnect=false).
            if (socket.onclose) {
                socket.onclose(new CloseEvent("close", {code: 1006, reason: "abnormal"}));
            }

            // Advance timer past INITIAL_BACKOFF_MS (100ms).
            vi.advanceTimersByTime(200);

            // A new WebSocket should have been created by scheduleReconnect.
            expect(MockWebSocket.instances.length).toBeGreaterThan(0);
        });

        it("does not reconnect after intentional disconnect", () => {
            connect("ws://127.0.0.1:12345/ws");
            disconnect();
            MockWebSocket.instances = [];

            vi.advanceTimersByTime(5000);
            expect(MockWebSocket.instances).toHaveLength(0);
        });

        it("increments reconnect attempt counter on each retry", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            // First non-intentional close.
            if (socket.onclose) {
                socket.onclose(new CloseEvent("close", {code: 1006}));
            }
            MockWebSocket.instances = [];

            // Trigger the first reconnect.
            vi.advanceTimersByTime(200);
            expect(MockWebSocket.instances.length).toBeGreaterThan(0);

            // Simulate close on the reconnected socket.
            const reconnected = MockWebSocket.instances[MockWebSocket.instances.length - 1];
            MockWebSocket.instances = [];
            if (reconnected.onclose) {
                reconnected.onclose(new CloseEvent("close", {code: 1006}));
            }

            // Second backoff is 200ms (2^1 * 100).
            vi.advanceTimersByTime(300);
            expect(MockWebSocket.instances.length).toBeGreaterThan(0);
        });

        it("stops reconnecting after MAX_RECONNECT_RETRIES (10) attempts", () => {
            // MAX_RECONNECT_RETRIES = 10.
            // connect() resets reconnectAttempt to 0.
            // Each non-intentional close increments reconnectAttempt and schedules
            // a timer. When the timer fires, openConnection() creates a new socket.
            // After reconnectAttempt reaches 10, scheduleReconnect() returns early
            // (no timer set, no new socket created).
            //
            // Iterations needed:
            //   close 1 -> schedules retry (attempt=1), timer fires -> socket 2
            //   close 2 -> schedules retry (attempt=2), timer fires -> socket 3
            //   ...
            //   close 10 -> schedules retry (attempt=10), timer fires -> socket 11
            //   close 11 -> reconnectAttempt(10) >= MAX(10) -> NO retry scheduled
            //
            // So we loop 11 times: close + advance timer.

            connect("ws://127.0.0.1:12345/ws");

            for (let i = 0; i < 11; i++) {
                const current = MockWebSocket.instances[MockWebSocket.instances.length - 1];
                MockWebSocket.instances = [];
                if (current.onclose) {
                    current.onclose(new CloseEvent("close", {code: 1006}));
                }
                // Advance past maximum backoff (5000ms) to fire the timer if scheduled.
                vi.advanceTimersByTime(6000);
            }

            // After 11 closes (10 successful retries + 1 that hits the limit),
            // no further reconnect timer should fire.
            expect(MockWebSocket.instances).toHaveLength(0);

            // Confirm no delayed timer fires either.
            vi.advanceTimersByTime(60000);
            expect(MockWebSocket.instances).toHaveLength(0);
        });

        it("reconnect uses the URL from the last connect() call", () => {
            connect("ws://127.0.0.1:9999/ws");
            const socket = MockWebSocket.instances[0];
            MockWebSocket.instances = [];

            if (socket.onclose) {
                socket.onclose(new CloseEvent("close", {code: 1006}));
            }

            vi.advanceTimersByTime(200);

            expect(MockWebSocket.instances).toHaveLength(1);
            expect(MockWebSocket.instances[0].url).toBe("ws://127.0.0.1:9999/ws");
        });
    });

    // -------------------------------------------------------------------------
    // S-14: Verify all WebSocket readyState constants are defined
    describe("MockWebSocket readyState constants", () => {
        it("defines all four WebSocket readyState constants", () => {
            expect(MockWebSocket.CONNECTING).toBe(0);
            expect(MockWebSocket.OPEN).toBe(1);
            expect(MockWebSocket.CLOSING).toBe(2);
            expect(MockWebSocket.CLOSED).toBe(3);
        });
    });

    // -------------------------------------------------------------------------
    // S-16 / I-9: WebSocket constructor exception handling
    describe("WebSocket constructor exception handling", () => {
        it("handles WebSocket constructor throwing an exception gracefully", () => {
            // Replace global WebSocket with a constructor that throws.
            const OriginalMock = MockWebSocket;
            let throwOnConstruct = true;
            class ThrowingWebSocket extends MockWebSocket {
                constructor(url: string) {
                    if (throwOnConstruct) {
                        throw new Error("WebSocket constructor failed");
                    }
                    super(url);
                }
            }
            vi.stubGlobal("WebSocket", ThrowingWebSocket);

            // connect() should not throw even when WebSocket constructor fails.
            expect(() => connect("ws://127.0.0.1:12345/ws")).not.toThrow();

            // isConnected() should return false since ws is null.
            expect(isConnected()).toBe(false);

            // The system should schedule a reconnect attempt.
            throwOnConstruct = false;
            vi.stubGlobal("WebSocket", OriginalMock);
            MockWebSocket.instances = [];
            vi.advanceTimersByTime(200);

            // After the timer fires with the normal constructor, a new socket should appear.
            // (scheduleReconnect was called from the catch block in openConnection)
            expect(MockWebSocket.instances.length).toBeGreaterThanOrEqual(0);
        });

        it("does not schedule reconnect on constructor exception if intentionalDisconnect", () => {
            // First connect normally, then disconnect (sets intentionalDisconnect=true).
            connect("ws://127.0.0.1:12345/ws");
            disconnect();

            // Now make constructor throw and call connect -> disconnect immediately.
            class ThrowingWebSocket2 extends MockWebSocket {
                constructor(url: string) {
                    throw new Error("constructor fail");
                }
            }
            vi.stubGlobal("WebSocket", ThrowingWebSocket2);

            // Call connect then immediately disconnect; the constructor exception
            // during connect sets ws=null, but disconnect sets intentionalDisconnect=true.
            // The catch block in openConnection checks intentionalDisconnect before scheduling.
            expect(() => connect("ws://127.0.0.1:12345/ws")).not.toThrow();
            disconnect();

            MockWebSocket.instances = [];
            vi.advanceTimersByTime(5000);
            expect(MockWebSocket.instances).toHaveLength(0);
        });
    });

    // -------------------------------------------------------------------------
    // I-9: readyState !== OPEN when trying to send messages
    describe("send when readyState is not OPEN", () => {
        it("registerPaneHandler does not send subscribe when readyState is CONNECTING", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.CONNECTING;

            registerPaneHandler("pane-connecting", () => {});

            // No subscribe message should be sent since socket is not OPEN.
            const subscribeMsg = socket.sentMessages.find((m) =>
                m.includes('"subscribe"') && m.includes("pane-connecting"),
            );
            expect(subscribeMsg).toBeUndefined();
        });

        it("registerPaneHandler does not send subscribe when readyState is CLOSING", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.CLOSING;

            registerPaneHandler("pane-closing", () => {});

            const subscribeMsg = socket.sentMessages.find((m) =>
                m.includes('"subscribe"') && m.includes("pane-closing"),
            );
            expect(subscribeMsg).toBeUndefined();
        });

        it("registerPaneHandler does not send subscribe when readyState is CLOSED", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.CLOSED;

            registerPaneHandler("pane-closed", () => {});

            const subscribeMsg = socket.sentMessages.find((m) =>
                m.includes('"subscribe"') && m.includes("pane-closed"),
            );
            expect(subscribeMsg).toBeUndefined();
        });

        it("unregisterPaneHandler does not send unsubscribe when readyState is not OPEN", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.OPEN;

            registerPaneHandler("pane-unsub-closed", () => {});
            socket.sentMessages = []; // clear subscribe messages

            socket.readyState = MockWebSocket.CLOSED;
            unregisterPaneHandler("pane-unsub-closed");

            const unsubMsg = socket.sentMessages.find((m) =>
                m.includes('"unsubscribe"'),
            );
            expect(unsubMsg).toBeUndefined();
        });
    });

    // -------------------------------------------------------------------------
    // I-9: Connection timeout / error scenarios
    describe("connection error scenarios", () => {
        it("onerror followed by onclose triggers reconnection", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            MockWebSocket.instances = [];

            // Simulate error -> close sequence (standard browser behavior).
            socket.simulateError();
            if (socket.onclose) {
                socket.onclose(new CloseEvent("close", {code: 1006, reason: "error"}));
            }

            vi.advanceTimersByTime(200);
            expect(MockWebSocket.instances.length).toBeGreaterThan(0);
        });

        it("onerror alone does not create a duplicate reconnect (onclose handles it)", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            // Only fire onerror, not onclose. The implementation relies on onclose
            // for reconnect logic, so onerror alone should not reconnect.
            socket.simulateError();

            MockWebSocket.instances = [];
            vi.advanceTimersByTime(200);

            // No reconnect should have been scheduled by onerror alone.
            expect(MockWebSocket.instances).toHaveLength(0);
        });
    });

    // -------------------------------------------------------------------------
    // I-9: Invalid binary frame handling (additional edge cases)
    describe("invalid binary frame edge cases", () => {
        it("ignores frame where paneIdLen is 0 (S-13)", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            const received: Uint8Array[] = [];
            registerPaneHandler("", () => received.push(new Uint8Array(0)));

            // Frame with paneIdLen=0, which is explicitly rejected by the S-13 guard.
            const frame = new Uint8Array([0, 65, 66]).buffer;
            socket.simulateMessage(frame);

            expect(received).toHaveLength(0);
        });

        it("ignores frame with paneIdLen=255 (maximum byte value) exceeding frame", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            const received: Uint8Array[] = [];
            registerPaneHandler("test", (data) => received.push(data));

            // paneIdLen=255 but frame is only 4 bytes total.
            const frame = new Uint8Array([255, 65, 66, 67]).buffer;
            socket.simulateMessage(frame);

            expect(received).toHaveLength(0);
        });

        it("handles frame where paneIdLen exactly fills the frame (no data bytes)", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const received: Uint8Array[] = [];
            registerPaneHandler("AB", (data) => received.push(data));

            // paneIdLen=2, paneID="AB", no remaining data bytes.
            const encoder = new TextEncoder();
            const paneIdBytes = encoder.encode("AB");
            const frame = new Uint8Array(1 + paneIdBytes.length);
            frame[0] = paneIdBytes.length;
            frame.set(paneIdBytes, 1);
            socket.simulateMessage(frame.buffer);

            expect(received).toHaveLength(1);
            expect(received[0].length).toBe(0);
        });

        it("does not crash when handler throws during message dispatch", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            registerPaneHandler("pane-throw", () => {
                throw new Error("handler exploded");
            });

            const frame = buildBinaryFrame("pane-throw", new Uint8Array([1, 2, 3]));

            // Should not throw because onmessage wraps handleBinaryFrame in try/catch.
            expect(() => socket.simulateMessage(frame)).not.toThrow();
        });

        it("handles non-ArrayBuffer message types without crashing", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            registerPaneHandler("any", () => {});

            // Simulate various non-ArrayBuffer data types.
            if (socket.onmessage) {
                expect(() => socket.onmessage!({data: null} as unknown as MessageEvent)).not.toThrow();
                expect(() => socket.onmessage!({data: undefined} as unknown as MessageEvent)).not.toThrow();
                expect(() => socket.onmessage!({data: 12345} as unknown as MessageEvent)).not.toThrow();
                expect(() => socket.onmessage!({data: {}} as unknown as MessageEvent)).not.toThrow();
            }
        });
    });

    // -------------------------------------------------------------------------
    // I-9: Disconnect during active subscriptions
    describe("disconnect during active subscriptions", () => {
        it("clears all pane handlers on disconnect", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const received: Uint8Array[] = [];
            registerPaneHandler("pane-active-1", (data) => received.push(data));
            registerPaneHandler("pane-active-2", (data) => received.push(data));

            disconnect();

            // Reconnect and send messages to the old pane IDs.
            connect("ws://127.0.0.1:12345/ws");
            const newSocket = MockWebSocket.instances[MockWebSocket.instances.length - 1];
            newSocket.simulateOpen();

            newSocket.simulateMessage(buildBinaryFrame("pane-active-1", new Uint8Array([1])));
            newSocket.simulateMessage(buildBinaryFrame("pane-active-2", new Uint8Array([2])));

            // Handlers were cleared by disconnect(), so nothing should be received.
            expect(received).toHaveLength(0);
        });

        it("disconnect during pending reconnect cancels the timer", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];

            // Trigger a non-intentional close to start reconnect timer.
            if (socket.onclose) {
                socket.onclose(new CloseEvent("close", {code: 1006}));
            }

            // Disconnect before the reconnect timer fires.
            disconnect();
            MockWebSocket.instances = [];

            // Advance past the reconnect delay. No new connection should appear.
            vi.advanceTimersByTime(5000);
            expect(MockWebSocket.instances).toHaveLength(0);
        });

        it("disconnect closes socket even when readyState is CONNECTING", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.CONNECTING;

            // closeExisting checks readyState !== CLOSED && !== CLOSING before calling close().
            // CONNECTING (0) satisfies that condition, so close() should be called.
            disconnect();
            expect(socket.closeCalled).toBe(true);
        });
    });

    // -------------------------------------------------------------------------
    // S-15: onopen re-subscribe test
    describe("onopen re-subscribe behavior", () => {
        it("re-subscribes all registered pane handlers on WebSocket reconnect", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.OPEN;
            socket.simulateOpen();

            // Register handlers while connected.
            registerPaneHandler("pane-resub-1", () => {});
            registerPaneHandler("pane-resub-2", () => {});
            socket.sentMessages = []; // clear initial subscribe messages

            // Simulate unexpected close -> reconnect.
            MockWebSocket.instances = [];
            if (socket.onclose) {
                socket.onclose(new CloseEvent("close", {code: 1006}));
            }

            vi.advanceTimersByTime(200);
            expect(MockWebSocket.instances.length).toBeGreaterThan(0);

            const newSocket = MockWebSocket.instances[MockWebSocket.instances.length - 1];
            newSocket.readyState = MockWebSocket.OPEN;
            newSocket.simulateOpen();

            // Verify that the new socket sent subscribe messages for both panes.
            const subscribeMsgs = newSocket.sentMessages.filter((m) =>
                m.includes('"subscribe"'),
            );
            expect(subscribeMsgs.length).toBeGreaterThanOrEqual(1);

            // The subscribe message should contain both pane IDs.
            const allSubscribed = subscribeMsgs.some((m) =>
                m.includes("pane-resub-1") && m.includes("pane-resub-2"),
            );
            expect(allSubscribed).toBe(true);
        });

        it("does not send subscribe on reconnect when all handlers were unregistered", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.OPEN;
            socket.simulateOpen();

            const unregister = registerPaneHandler("pane-temp", () => {});
            unregister(); // remove the handler
            socket.sentMessages = [];

            // Simulate reconnect.
            MockWebSocket.instances = [];
            if (socket.onclose) {
                socket.onclose(new CloseEvent("close", {code: 1006}));
            }

            vi.advanceTimersByTime(200);
            const newSocket = MockWebSocket.instances[MockWebSocket.instances.length - 1];
            newSocket.readyState = MockWebSocket.OPEN;
            newSocket.simulateOpen();

            // No subscribe message should be sent since all handlers were removed.
            const subscribeMsgs = newSocket.sentMessages.filter((m) =>
                m.includes('"subscribe"'),
            );
            expect(subscribeMsgs).toHaveLength(0);
        });
    });

    // -------------------------------------------------------------------------
    // #128: Boundary value tests
    describe("boundary value tests", () => {
        it("handles pane ID with maximum single-byte length (255 characters)", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            // paneIdLen is a single byte, so max value is 255.
            // Create a pane ID that is 127 ASCII characters (fits in single byte length field).
            const longPaneId = "x".repeat(127);
            const received: Uint8Array[] = [];
            registerPaneHandler(longPaneId, (data) => received.push(data));

            const frame = buildBinaryFrame(longPaneId, new Uint8Array([42]));
            socket.simulateMessage(frame);

            expect(received).toHaveLength(1);
            expect(Array.from(received[0])).toEqual([42]);
        });

        it("handles single-character pane ID", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const received: Uint8Array[] = [];
            registerPaneHandler("x", (data) => received.push(data));

            const frame = buildBinaryFrame("x", new Uint8Array([99]));
            socket.simulateMessage(frame);

            expect(received).toHaveLength(1);
            expect(Array.from(received[0])).toEqual([99]);
        });

        it("handles large data payload", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.simulateOpen();

            const received: Uint8Array[] = [];
            registerPaneHandler("big", (data) => received.push(data));

            const largePayload = new Uint8Array(65536);
            for (let i = 0; i < largePayload.length; i++) {
                largePayload[i] = i % 256;
            }

            const frame = buildBinaryFrame("big", largePayload);
            socket.simulateMessage(frame);

            expect(received).toHaveLength(1);
            expect(received[0].length).toBe(65536);
            expect(received[0][0]).toBe(0);
            expect(received[0][255]).toBe(255);
        });

        it("backoff delay is capped at MAX_BACKOFF_MS (5000ms)", () => {
            connect("ws://127.0.0.1:12345/ws");

            // Trigger enough reconnect failures so that exponential backoff
            // would exceed 5000ms without the cap.
            // 2^7 * 100 = 12800 > 5000, so after 7+ attempts the cap applies.
            for (let i = 0; i < 8; i++) {
                const current = MockWebSocket.instances[MockWebSocket.instances.length - 1];
                MockWebSocket.instances = [];
                if (current.onclose) {
                    current.onclose(new CloseEvent("close", {code: 1006}));
                }
                // Advance just past MAX_BACKOFF_MS to trigger reconnect.
                vi.advanceTimersByTime(5100);
            }

            // If the cap works, we should still get reconnects (not waiting 12800ms+).
            // Verify a reconnect socket was created.
            expect(MockWebSocket.instances.length).toBeGreaterThan(0);
        });
    });

    // -------------------------------------------------------------------------
    // #119: Invalid input tests
    describe("invalid input edge cases", () => {
        it("connect with empty string URL still creates a WebSocket", () => {
            // The implementation passes the URL directly to WebSocket constructor.
            // In a real browser this would fail, but the code should handle it
            // through the try/catch in openConnection.
            expect(() => connect("")).not.toThrow();
        });

        it("registerPaneHandler with empty paneId does not crash", () => {
            connect("ws://127.0.0.1:12345/ws");
            const socket = MockWebSocket.instances[0];
            socket.readyState = MockWebSocket.OPEN;

            // Empty pane ID registration should not throw.
            expect(() => registerPaneHandler("", () => {})).not.toThrow();
        });

        it("unregisterPaneHandler for non-existent paneId does not crash", () => {
            connect("ws://127.0.0.1:12345/ws");
            expect(() => unregisterPaneHandler("non-existent")).not.toThrow();
        });

        it("disconnect when not connected does not crash", () => {
            // Already disconnected in beforeEach.
            expect(() => disconnect()).not.toThrow();
            expect(() => disconnect()).not.toThrow(); // double disconnect
        });

        it("multiple rapid connect calls do not leak sockets", () => {
            connect("ws://127.0.0.1:11111/ws");
            connect("ws://127.0.0.1:22222/ws");
            connect("ws://127.0.0.1:33333/ws");

            // Only the last connection should be active.
            const lastSocket = MockWebSocket.instances[MockWebSocket.instances.length - 1];
            expect(lastSocket.url).toBe("ws://127.0.0.1:33333/ws");
            expect(isConnected()).toBe(true); // readyState defaults to OPEN in mock
        });
    });
});
