package jsonrpc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Framing は使用された stdio フレーミング形式を表す。
type Framing int

const (
	FramingUnknown Framing = iota
	FramingContentLength
	FramingLineJSON
)

const MaxFrameBytes int64 = 4 << 20

var ErrFrameTooLarge = fmt.Errorf("json-rpc frame exceeds %d bytes", MaxFrameBytes)

// ReadMessage は Content-Length または行区切り JSON 形式で 1 件の JSON-RPC メッセージを読み込む。
func ReadMessage(reader *bufio.Reader) ([]byte, error) {
	payload, _, err := ReadMessageWithFraming(reader)
	return payload, err
}

// ReadMessageWithFraming は 1 件の JSON-RPC メッセージを読み込み、使用されたフレーミングを返す。
func ReadMessageWithFraming(reader *bufio.Reader) ([]byte, Framing, error) {
	for {
		limited := &io.LimitedReader{R: reader, N: MaxFrameBytes + 1}
		line, err := readLineFromLimited(limited)
		if err != nil {
			return nil, FramingUnknown, err
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}

		linePayload := strings.TrimSpace(trimmed)
		if strings.HasPrefix(linePayload, "{") || strings.HasPrefix(linePayload, "[") {
			return []byte(linePayload), FramingLineJSON, nil
		}

		contentLength, err := parseContentLengthHeader(trimmed)
		if err != nil {
			return nil, FramingUnknown, err
		}

		for {
			line, err = readLineFromLimited(limited)
			if err != nil {
				return nil, FramingUnknown, err
			}

			trimmed = strings.TrimRight(line, "\r\n")
			if trimmed == "" {
				break
			}

			n, err := parseContentLengthHeader(trimmed)
			if err != nil {
				return nil, FramingUnknown, err
			}
			if n >= 0 {
				contentLength = n
			}
		}

		if contentLength < 0 {
			return nil, FramingUnknown, fmt.Errorf("missing Content-Length header")
		}
		if int64(contentLength) > MaxFrameBytes {
			return nil, FramingUnknown, ErrFrameTooLarge
		}

		payload := make([]byte, contentLength)
		if _, err := io.ReadFull(limited, payload); err != nil {
			if errors.Is(err, io.EOF) && limited.N == 0 {
				return nil, FramingUnknown, ErrFrameTooLarge
			}
			return nil, FramingUnknown, err
		}

		return payload, FramingContentLength, nil
	}
}

func readLineFromLimited(reader *io.LimitedReader) (string, error) {
	var b strings.Builder
	var buf [1]byte

	for {
		n, err := reader.Read(buf[:])
		if n > 0 {
			b.WriteByte(buf[0])
			if buf[0] == '\n' {
				return b.String(), nil
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) && reader.N == 0 {
				return "", ErrFrameTooLarge
			}
			return "", err
		}
	}
}

func parseContentLengthHeader(line string) (int, error) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return -1, nil
	}

	headerName := strings.TrimSpace(parts[0])
	headerValue := strings.TrimSpace(parts[1])
	if !strings.EqualFold(headerName, "Content-Length") {
		return -1, nil
	}

	n, err := strconv.Atoi(headerValue)
	if err != nil || n < 0 {
		return -1, fmt.Errorf("invalid Content-Length header: %q", headerValue)
	}
	return n, nil
}

// WriteMessage は Content-Length 形式で 1 件の JSON-RPC メッセージを書き込む。
func WriteMessage(writer io.Writer, payload []byte) error {
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}

// WriteJSON は値をマーシャルして 1 件のフレーム付きメッセージを書き込む。
func WriteJSON(writer io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return WriteMessage(writer, payload)
}

// WriteJSONLine は値をマーシャルして 1 件の行区切り JSON-RPC メッセージを書き込む。
func WriteJSONLine(writer io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := writer.Write(payload); err != nil {
		return err
	}
	_, err = writer.Write([]byte("\n"))
	return err
}
