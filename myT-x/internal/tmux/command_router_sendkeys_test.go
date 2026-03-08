package tmux

import (
	"errors"
	"testing"
	"time"
)

func TestWriteSendKeysPayloadWithDelayEmptyPayload(t *testing.T) {
	writer := &sendKeysWriteSpy{}
	sleep := &sendKeysSleepSpy{}

	err := writeSendKeysPayloadWithDelay(writer, nil, 10*time.Millisecond, sleep.Sleep)
	if err != nil {
		t.Fatalf("writeSendKeysPayloadWithDelay() error = %v, want nil", err)
	}
	if len(writer.writes) != 0 {
		t.Fatalf("write count = %d, want 0", len(writer.writes))
	}
	if len(sleep.calls) != 0 {
		t.Fatalf("sleep count = %d, want 0", len(sleep.calls))
	}
}

func TestWriteSendKeysPayloadWithDelayNoSplit(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "plain text",
			payload: []byte("echo ok"),
		},
		{
			name:    "single enter",
			payload: []byte{'\r'},
		},
		{
			name:    "only enter keys",
			payload: []byte{'\r', '\r'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &sendKeysWriteSpy{}
			sleep := &sendKeysSleepSpy{}

			err := writeSendKeysPayloadWithDelay(writer, tt.payload, 10*time.Millisecond, sleep.Sleep)
			if err != nil {
				t.Fatalf("writeSendKeysPayloadWithDelay() error = %v, want nil", err)
			}
			if len(writer.writes) != 1 {
				t.Fatalf("write count = %d, want 1", len(writer.writes))
			}
			if string(writer.writes[0]) != string(tt.payload) {
				t.Fatalf("write payload = %q, want %q", writer.writes[0], tt.payload)
			}
			if len(sleep.calls) != 0 {
				t.Fatalf("sleep count = %d, want 0", len(sleep.calls))
			}
		})
	}
}

func TestWriteSendKeysPayloadWithDelaySplitTrailingSubmit(t *testing.T) {
	writer := &sendKeysWriteSpy{}
	sleep := &sendKeysSleepSpy{}
	delay := 12 * time.Millisecond

	err := writeSendKeysPayloadWithDelay(writer, []byte("echo ok\r"), delay, sleep.Sleep)
	if err != nil {
		t.Fatalf("writeSendKeysPayloadWithDelay() error = %v, want nil", err)
	}
	if len(writer.writes) != 2 {
		t.Fatalf("write count = %d, want 2", len(writer.writes))
	}
	if got, want := string(writer.writes[0]), "echo ok"; got != want {
		t.Fatalf("first write = %q, want %q", got, want)
	}
	if got, want := string(writer.writes[1]), "\r"; got != want {
		t.Fatalf("second write = %q, want %q", got, want)
	}
	if len(sleep.calls) != 1 {
		t.Fatalf("sleep count = %d, want 1", len(sleep.calls))
	}
	if sleep.calls[0] != delay {
		t.Fatalf("sleep delay = %v, want %v", sleep.calls[0], delay)
	}
}

func TestWriteSendKeysPayloadWithDelayReturnsFirstWriteError(t *testing.T) {
	wantErr := errors.New("first write failed")
	writer := &sendKeysWriteSpy{
		failAtCall: 1,
		failErr:    wantErr,
	}
	sleep := &sendKeysSleepSpy{}

	err := writeSendKeysPayloadWithDelay(writer, []byte("echo ok\r"), 10*time.Millisecond, sleep.Sleep)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if len(writer.writes) != 1 {
		t.Fatalf("write count = %d, want 1", len(writer.writes))
	}
	if len(sleep.calls) != 0 {
		t.Fatalf("sleep count = %d, want 0", len(sleep.calls))
	}
}

func TestWriteSendKeysPayloadWithDelayReturnsSecondWriteError(t *testing.T) {
	wantErr := errors.New("second write failed")
	writer := &sendKeysWriteSpy{
		failAtCall: 2,
		failErr:    wantErr,
	}
	sleep := &sendKeysSleepSpy{}

	err := writeSendKeysPayloadWithDelay(writer, []byte("echo ok\r"), 10*time.Millisecond, sleep.Sleep)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if len(writer.writes) != 2 {
		t.Fatalf("write count = %d, want 2", len(writer.writes))
	}
	if len(sleep.calls) != 1 {
		t.Fatalf("sleep count = %d, want 1", len(sleep.calls))
	}
}

func TestWriteSendKeysPayloadWithDelayNilWriter(t *testing.T) {
	err := writeSendKeysPayloadWithDelay(nil, []byte("echo ok\r"), 10*time.Millisecond, nil)
	if !errors.Is(err, errNilSendKeysWriter) {
		t.Fatalf("error = %v, want %v", err, errNilSendKeysWriter)
	}
}

// --- Typewriter mode tests ---

func TestWriteSendKeysPayloadTypewriterBasic(t *testing.T) {
	writer := &sendKeysWriteSpy{}
	sleep := &sendKeysSleepSpy{}
	charDelay := 3 * time.Millisecond
	submitDelay := 60 * time.Millisecond

	// "hello\r" → 5 single-byte writes + 4 charDelays + 1 submitDelay + 1 "\r" write
	err := writeSendKeysPayloadTypewriterWithDelay(writer, []byte("hello\r"),
		submitDelay, charDelay, sleep.Sleep)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}

	// 5 text bytes + 1 submit byte = 6 writes
	if got := len(writer.writes); got != 6 {
		t.Fatalf("write count = %d, want 6", got)
	}
	// Verify each text byte
	for i, expected := range []byte("hello") {
		if got := writer.writes[i]; len(got) != 1 || got[0] != expected {
			t.Fatalf("write[%d] = %q, want %q", i, got, []byte{expected})
		}
	}
	// Verify submit byte
	if got := string(writer.writes[5]); got != "\r" {
		t.Fatalf("submit write = %q, want %q", got, "\r")
	}

	// 4 charDelays between text bytes + 1 submitDelay before "\r" = 5 sleeps
	if got := len(sleep.calls); got != 5 {
		t.Fatalf("sleep count = %d, want 5", got)
	}
	for i := range 4 {
		if sleep.calls[i] != charDelay {
			t.Fatalf("sleep[%d] = %v, want %v", i, sleep.calls[i], charDelay)
		}
	}
	if sleep.calls[4] != submitDelay {
		t.Fatalf("sleep[4] = %v, want %v", sleep.calls[4], submitDelay)
	}
}

func TestWriteSendKeysPayloadTypewriterShortPayload(t *testing.T) {
	writer := &sendKeysWriteSpy{}
	sleep := &sendKeysSleepSpy{}

	// Single char "x" is below threshold (2) → bulk write, no charDelay
	err := writeSendKeysPayloadTypewriterWithDelay(writer, []byte("x"),
		60*time.Millisecond, 3*time.Millisecond, sleep.Sleep)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if got := len(writer.writes); got != 1 {
		t.Fatalf("write count = %d, want 1", got)
	}
	if got := string(writer.writes[0]); got != "x" {
		t.Fatalf("write = %q, want %q", got, "x")
	}
	if got := len(sleep.calls); got != 0 {
		t.Fatalf("sleep count = %d, want 0", got)
	}
}

func TestWriteSendKeysPayloadTypewriterThresholdBoundary(t *testing.T) {
	writer := &sendKeysWriteSpy{}
	sleep := &sendKeysSleepSpy{}

	// "ab" is exactly typewriterThreshold (2) → typewriter mode activates:
	// 2 single-byte writes + 1 charDelay between them, no submitDelay.
	err := writeSendKeysPayloadTypewriterWithDelay(writer, []byte("ab"),
		60*time.Millisecond, 3*time.Millisecond, sleep.Sleep)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if got := len(writer.writes); got != 2 {
		t.Fatalf("write count = %d, want 2", got)
	}
	if got := string(writer.writes[0]); got != "a" {
		t.Fatalf("write[0] = %q, want %q", got, "a")
	}
	if got := string(writer.writes[1]); got != "b" {
		t.Fatalf("write[1] = %q, want %q", got, "b")
	}
	// 1 charDelay between the 2 bytes, no submitDelay
	if got := len(sleep.calls); got != 1 {
		t.Fatalf("sleep count = %d, want 1", got)
	}
	if sleep.calls[0] != 3*time.Millisecond {
		t.Fatalf("sleep[0] = %v, want %v", sleep.calls[0], 3*time.Millisecond)
	}
}

func TestWriteSendKeysPayloadTypewriterNoSubmit(t *testing.T) {
	writer := &sendKeysWriteSpy{}
	sleep := &sendKeysSleepSpy{}

	// "hello" without trailing \r → 5 single-byte writes + 4 charDelays, no submitDelay
	err := writeSendKeysPayloadTypewriterWithDelay(writer, []byte("hello"),
		60*time.Millisecond, 3*time.Millisecond, sleep.Sleep)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if got := len(writer.writes); got != 5 {
		t.Fatalf("write count = %d, want 5", got)
	}
	if got := len(sleep.calls); got != 4 {
		t.Fatalf("sleep count = %d, want 4", got)
	}
}

func TestWriteSendKeysPayloadTypewriterEmpty(t *testing.T) {
	writer := &sendKeysWriteSpy{}
	sleep := &sendKeysSleepSpy{}

	err := writeSendKeysPayloadTypewriterWithDelay(writer, nil,
		60*time.Millisecond, 3*time.Millisecond, sleep.Sleep)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if len(writer.writes) != 0 {
		t.Fatalf("write count = %d, want 0", len(writer.writes))
	}
}

func TestWriteSendKeysPayloadTypewriterNilWriter(t *testing.T) {
	// Provide a no-op sleepFn to avoid nil-panic risk if check order changes.
	err := writeSendKeysPayloadTypewriterWithDelay(nil, []byte("hello\r"),
		60*time.Millisecond, 3*time.Millisecond, func(time.Duration) {})
	if !errors.Is(err, errNilSendKeysWriter) {
		t.Fatalf("error = %v, want %v", err, errNilSendKeysWriter)
	}
}

func TestWriteSendKeysPayloadTypewriterWriteError(t *testing.T) {
	wantErr := errors.New("write failed at byte 3")
	writer := &sendKeysWriteSpy{
		failAtCall: 3, // fail on the 3rd single-byte write
		failErr:    wantErr,
	}
	sleep := &sendKeysSleepSpy{}

	err := writeSendKeysPayloadTypewriterWithDelay(writer, []byte("hello\r"),
		60*time.Millisecond, 3*time.Millisecond, sleep.Sleep)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	// Should have attempted 3 writes (3rd failed)
	if got := len(writer.writes); got != 3 {
		t.Fatalf("write count = %d, want 3", got)
	}
	// 2 charDelays between first 3 bytes
	if got := len(sleep.calls); got != 2 {
		t.Fatalf("sleep count = %d, want 2", got)
	}
}

// --- CRLF mode tests ---

func TestTransformSubmitCRLF(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    string
		wantLen int
	}{
		{
			name:    "trailing CR becomes CRLF",
			input:   []byte("hello\r"),
			want:    "hello\r\n",
			wantLen: 7,
		},
		{
			name:    "no trailing CR unchanged",
			input:   []byte("hello"),
			want:    "hello",
			wantLen: 5,
		},
		{
			name:    "empty payload",
			input:   []byte{},
			want:    "",
			wantLen: 0,
		},
		{
			name:    "nil payload",
			input:   nil,
			want:    "",
			wantLen: 0,
		},
		{
			name:    "CR only becomes CRLF",
			input:   []byte{'\r'},
			want:    "\r\n",
			wantLen: 2,
		},
		{
			name:    "multiple CRs only last transformed",
			input:   []byte{'\r', '\r'},
			want:    "\r\r\n",
			wantLen: 3,
		},
		{
			name:    "trailing LF unchanged",
			input:   []byte("hello\n"),
			want:    "hello\n",
			wantLen: 6,
		},
		{
			name:    "already CRLF adds extra LF",
			input:   []byte("hello\r\n\r"),
			want:    "hello\r\n\r\n",
			wantLen: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transformSubmitCRLF(tt.input)
			if string(got) != tt.want {
				t.Fatalf("transformSubmitCRLF(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestWriteSendKeysPayloadCRLFBasic(t *testing.T) {
	writer := &sendKeysWriteSpy{}
	sleep := &sendKeysSleepSpy{}
	charDelay := 3 * time.Millisecond
	submitDelay := 60 * time.Millisecond

	// "hi\r" → text part "hi" via typewriter + submitDelay + "\r\n" bulk write
	// Writes: "h" (3ms) "i" (60ms submitDelay) "\r\n"
	err := writeSendKeysPayloadCRLFWithDelay(writer, []byte("hi\r"),
		submitDelay, charDelay, sleep.Sleep)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}

	// 2 text bytes (h, i) + 1 submit write (\r\n) = 3 writes
	if got := len(writer.writes); got != 3 {
		t.Fatalf("write count = %d, want 3", got)
	}
	if got := string(writer.writes[0]); got != "h" {
		t.Fatalf("write[0] = %q, want %q", got, "h")
	}
	if got := string(writer.writes[1]); got != "i" {
		t.Fatalf("write[1] = %q, want %q", got, "i")
	}
	if got := string(writer.writes[2]); got != "\r\n" {
		t.Fatalf("write[2] = %q, want %q", got, "\r\n")
	}

	// 1 charDelay between h and i + 1 submitDelay before \r\n = 2 sleeps
	if got := len(sleep.calls); got != 2 {
		t.Fatalf("sleep count = %d, want 2", got)
	}
	if sleep.calls[0] != charDelay {
		t.Fatalf("sleep[0] = %v, want %v", sleep.calls[0], charDelay)
	}
	if sleep.calls[1] != submitDelay {
		t.Fatalf("sleep[1] = %v, want %v", sleep.calls[1], submitDelay)
	}
}

func TestWriteSendKeysPayloadCRLFNoTrailingCR(t *testing.T) {
	writer := &sendKeysWriteSpy{}
	sleep := &sendKeysSleepSpy{}

	// "hello" has no trailing \r → no CRLF transform, just typewriter mode
	err := writeSendKeysPayloadCRLFWithDelay(writer, []byte("hello"),
		60*time.Millisecond, 3*time.Millisecond, sleep.Sleep)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	// 5 bytes → 5 single-byte writes
	if got := len(writer.writes); got != 5 {
		t.Fatalf("write count = %d, want 5", got)
	}
}

func TestWriteSendKeysPayloadCRLFNilWriter(t *testing.T) {
	err := writeSendKeysPayloadCRLFWithDelay(nil, []byte("hello\r"),
		60*time.Millisecond, 3*time.Millisecond, func(time.Duration) {})
	if !errors.Is(err, errNilSendKeysWriter) {
		t.Fatalf("error = %v, want %v", err, errNilSendKeysWriter)
	}
}

func TestWriteSendKeysPayloadCRLFEmpty(t *testing.T) {
	writer := &sendKeysWriteSpy{}
	sleep := &sendKeysSleepSpy{}

	err := writeSendKeysPayloadCRLFWithDelay(writer, nil,
		60*time.Millisecond, 3*time.Millisecond, sleep.Sleep)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if len(writer.writes) != 0 {
		t.Fatalf("write count = %d, want 0", len(writer.writes))
	}
}

type sendKeysWriteSpy struct {
	writes     [][]byte
	failAtCall int
	failErr    error
}

func (s *sendKeysWriteSpy) Write(payload []byte) (int, error) {
	copyPayload := append([]byte(nil), payload...)
	s.writes = append(s.writes, copyPayload)

	if s.failAtCall > 0 && len(s.writes) == s.failAtCall {
		return 0, s.failErr
	}

	return len(payload), nil
}

type sendKeysSleepSpy struct {
	calls []time.Duration
}

func (s *sendKeysSleepSpy) Sleep(delay time.Duration) {
	s.calls = append(s.calls, delay)
}
