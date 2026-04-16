package jsonrpc

import (
	"bufio"
	"bytes"
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
const maxHeaderBytes int64 = 16 << 10
const maxHeaderLineBytes int64 = 4096

var ErrFrameTooLarge = fmt.Errorf("json-rpc frame exceeds %d bytes", MaxFrameBytes)

// ReadMessage は Content-Length または行区切り JSON 形式で 1 件の JSON-RPC メッセージを読み込む。
func ReadMessage(reader *bufio.Reader) ([]byte, error) {
	payload, _, err := ReadMessageWithFraming(reader)
	return payload, err
}

// ReadMessageWithFraming は 1 件の JSON-RPC メッセージを読み込み、使用されたフレーミングを返す。
func ReadMessageWithFraming(reader *bufio.Reader) ([]byte, Framing, error) {
	for {
		line, used, err := readLineWithLimit(reader, MaxFrameBytes+1)
		if err != nil {
			return nil, FramingUnknown, err
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}

		// 一部の MCP stdio クライアントが使う行区切り JSON-RPC をサポート。
		linePayload := strings.TrimSpace(trimmed)
		if strings.HasPrefix(linePayload, "{") || strings.HasPrefix(linePayload, "[") {
			if int64(len(linePayload)) > MaxFrameBytes {
				return nil, FramingUnknown, ErrFrameTooLarge
			}
			return []byte(linePayload), FramingLineJSON, nil
		}
		if used > maxHeaderLineBytes || used > maxHeaderBytes {
			return nil, FramingUnknown, ErrFrameTooLarge
		}

		contentLength, err := parseContentLengthHeader(trimmed)
		if err != nil {
			return nil, FramingUnknown, err
		}

		headerBytesRemaining := maxHeaderBytes - used
		for {
			if headerBytesRemaining <= 0 {
				return nil, FramingUnknown, ErrFrameTooLarge
			}
			line, used, err = readLineWithLimit(reader, minInt64(maxHeaderLineBytes, headerBytesRemaining))
			if err != nil {
				return nil, FramingUnknown, err
			}
			headerBytesRemaining -= used

			trimmed = strings.TrimRight(line, "\r\n")
			// Accept both CRLF and LF-only header delimiters.
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
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, FramingUnknown, err
		}

		return payload, FramingContentLength, nil
	}
}

func readLineWithLimit(reader *bufio.Reader, maxBytes int64) (string, int64, error) {
	var builder bytes.Buffer
	var used int64
	for {
		chunk, err := reader.ReadSlice('\n')
		if int64(len(chunk)) > maxBytes-used {
			return "", 0, ErrFrameTooLarge
		}
		if len(chunk) > 0 {
			used += int64(len(chunk))
			builder.Write(chunk)
		}
		switch {
		case err == nil:
			return builder.String(), used, nil
		case errors.Is(err, bufio.ErrBufferFull):
			if used >= maxBytes {
				return "", 0, ErrFrameTooLarge
			}
			continue
		case errors.Is(err, io.EOF):
			if used == 0 {
				return "", 0, io.EOF
			}
			return "", 0, io.ErrUnexpectedEOF
		default:
			return "", 0, err
		}
	}
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// parseContentLengthHeader は Content-Length ヘッダー行を解析する。
// Content-Length 以外のヘッダー行、またはヘッダー形式でない行は (-1, nil) を返す。
// Content-Length の値が不正（非数値や負の数）の場合は (-1, error) を返す。
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
