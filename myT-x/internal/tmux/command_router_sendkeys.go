package tmux

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
)

const sendKeysSubmitDelay = 60 * time.Millisecond

const (
	// typewriterCharDelay is the delay between each byte in typewriter mode.
	// This simulates manual typing to prevent burst-mode input issues in
	// interactive TUIs (e.g. Copilot CLI / Inquirer-based prompts).
	typewriterCharDelay = 3 * time.Millisecond

	// typewriterThreshold is the minimum text-part length to activate per-byte
	// delays. Payloads shorter than this threshold (i.e. single byte) are
	// written in a single bulk call because per-byte delays add no value for
	// a single character. At exactly threshold (2 bytes), typewriter mode
	// activates and each byte is written individually with charDelay between them.
	typewriterThreshold = 2
)

var errNilSendKeysWriter = errors.New("send-keys writer is nil")

// writeSendKeysPayload writes translated send-keys bytes to the target writer.
// When payload contains command text followed by a trailing submit '\r', it
// splits into two writes with a small delay to avoid paste-style submission in
// interactive TUIs.
func writeSendKeysPayload(writer io.Writer, payload []byte) error {
	return writeSendKeysPayloadWithDelay(writer, payload, sendKeysSubmitDelay, time.Sleep)
}

func writeSendKeysPayloadWithDelay(writer io.Writer, payload []byte, delay time.Duration, sleepFn func(time.Duration)) error {
	if len(payload) == 0 {
		return nil
	}
	if writer == nil {
		return errNilSendKeysWriter
	}

	splitIndex, shouldSplit := splitTrailingSubmit(payload)
	slog.Debug("[DEBUG-SENDKEYS-SPLIT]",
		"payloadLen", len(payload),
		"splitIndex", splitIndex,
		"shouldSplit", shouldSplit,
		"lastByte", fmt.Sprintf("%02x", payload[len(payload)-1]),
	)
	if !shouldSplit {
		slog.Debug("[DEBUG-SENDKEYS-WRITE] bulk write (no split)",
			"hex", fmt.Sprintf("%x", payload),
			"len", len(payload),
		)
		_, err := writer.Write(payload)
		return err
	}

	slog.Debug("[DEBUG-SENDKEYS-WRITE] text part",
		"hex", fmt.Sprintf("%x", payload[:splitIndex]),
		"len", splitIndex,
	)
	if _, err := writer.Write(payload[:splitIndex]); err != nil {
		return err
	}
	if delay > 0 && sleepFn != nil {
		sleepFn(delay)
	}
	slog.Debug("[DEBUG-SENDKEYS-WRITE] submit part after delay",
		"hex", fmt.Sprintf("%x", payload[splitIndex:]),
		"delayMs", delay.Milliseconds(),
	)
	_, err := writer.Write(payload[splitIndex:])
	return err
}

// writeSendKeysPayloadTypewriter writes payload one byte at a time with
// micro-delays to simulate manual typing. This prevents burst-mode input
// issues in interactive TUIs where readline treats bulk writes differently
// from individual keystrokes.
func writeSendKeysPayloadTypewriter(writer io.Writer, payload []byte) error {
	return writeSendKeysPayloadTypewriterWithDelay(writer, payload,
		sendKeysSubmitDelay, typewriterCharDelay, time.Sleep)
}

func writeSendKeysPayloadTypewriterWithDelay(
	writer io.Writer,
	payload []byte,
	submitDelay time.Duration,
	charDelay time.Duration,
	sleepFn func(time.Duration),
) error {
	if len(payload) == 0 {
		return nil
	}
	if writer == nil {
		return errNilSendKeysWriter
	}

	splitIndex, shouldSplit := splitTrailingSubmit(payload)

	textPart := payload
	var submitPart []byte
	if shouldSplit {
		textPart = payload[:splitIndex]
		submitPart = payload[splitIndex:]
	}

	// Write text part: one byte at a time with charDelay for payloads
	// above the threshold; single bulk write otherwise.
	if len(textPart) >= typewriterThreshold && charDelay > 0 {
		slog.Debug("[DEBUG-SENDKEYS-TYPEWRITER] per-byte mode",
			"textLen", len(textPart),
			"charDelayMs", charDelay.Milliseconds(),
		)
		for i := 0; i < len(textPart); i++ {
			slog.Debug("[DEBUG-SENDKEYS-TYPEWRITER] byte",
				"index", i,
				"hex", fmt.Sprintf("%02x", textPart[i]),
			)
			if _, err := writer.Write(textPart[i : i+1]); err != nil {
				return err
			}
			if i < len(textPart)-1 && sleepFn != nil {
				sleepFn(charDelay)
			}
		}
	} else if len(textPart) > 0 {
		slog.Debug("[DEBUG-SENDKEYS-TYPEWRITER] bulk text write",
			"hex", fmt.Sprintf("%x", textPart),
			"len", len(textPart),
		)
		if _, err := writer.Write(textPart); err != nil {
			return err
		}
	}

	// Write submit part after submitDelay.
	if shouldSplit && len(submitPart) > 0 {
		if submitDelay > 0 && sleepFn != nil {
			sleepFn(submitDelay)
		}
		slog.Debug("[DEBUG-SENDKEYS-TYPEWRITER] submit part",
			"hex", fmt.Sprintf("%x", submitPart),
			"submitDelayMs", submitDelay.Milliseconds(),
		)
		if _, err := writer.Write(submitPart); err != nil {
			return err
		}
	}

	return nil
}

// writeSendKeysPayloadCRLF transforms trailing \r to \r\n then writes via
// typewriter mode. This addresses ConPTY on Windows where the input pipe may
// require CRLF to generate a proper KEY_EVENT_RECORD for Enter.
func writeSendKeysPayloadCRLF(writer io.Writer, payload []byte) error {
	return writeSendKeysPayloadCRLFWithDelay(writer, payload,
		sendKeysSubmitDelay, typewriterCharDelay, time.Sleep)
}

func writeSendKeysPayloadCRLFWithDelay(
	writer io.Writer,
	payload []byte,
	submitDelay time.Duration,
	charDelay time.Duration,
	sleepFn func(time.Duration),
) error {
	if len(payload) == 0 {
		return nil
	}
	if writer == nil {
		return errNilSendKeysWriter
	}

	// Check if the original payload has a trailing \r (Enter) to split.
	_, hasSubmit := splitTrailingSubmit(payload)

	if !hasSubmit {
		// No trailing Enter: just use typewriter mode as-is.
		slog.Debug("[DEBUG-SENDKEYS-CRLF] no trailing CR, typewriter passthrough",
			"hex", fmt.Sprintf("%x", payload),
		)
		return writeSendKeysPayloadTypewriterWithDelay(writer, payload,
			submitDelay, charDelay, sleepFn)
	}

	// Split: text part is everything before the trailing \r, submit part is \r\n.
	textPart := payload[:len(payload)-1]
	submitPart := []byte{'\r', '\n'}
	slog.Debug("[DEBUG-SENDKEYS-CRLF] split with CRLF submit",
		"textHex", fmt.Sprintf("%x", textPart),
		"submitHex", fmt.Sprintf("%x", submitPart),
	)

	// Write text part via typewriter mode (no trailing submit).
	if err := writeSendKeysPayloadTypewriterWithDelay(writer, textPart,
		submitDelay, charDelay, sleepFn); err != nil {
		return err
	}

	// Submit delay then write \r\n.
	if submitDelay > 0 && sleepFn != nil {
		sleepFn(submitDelay)
	}
	slog.Debug("[DEBUG-SENDKEYS-CRLF] writing CRLF submit after delay",
		"submitDelayMs", submitDelay.Milliseconds(),
	)
	_, err := writer.Write(submitPart)
	return err
}

// transformSubmitCRLF replaces a trailing \r with \r\n in the payload.
// If there is no trailing \r, returns the payload unchanged.
func transformSubmitCRLF(payload []byte) []byte {
	if len(payload) == 0 || payload[len(payload)-1] != '\r' {
		return payload
	}
	result := make([]byte, len(payload)+1)
	copy(result, payload)
	result[len(payload)] = '\n'
	return result
}

func splitTrailingSubmit(payload []byte) (int, bool) {
	if len(payload) <= 1 {
		return 0, false
	}
	if payload[len(payload)-1] != '\r' {
		return 0, false
	}

	prefix := payload[:len(payload)-1]
	for _, b := range prefix {
		if b != '\r' {
			return len(payload) - 1, true
		}
	}

	return 0, false
}
