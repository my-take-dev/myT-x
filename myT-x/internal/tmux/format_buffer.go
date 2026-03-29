// format_buffer.go — Buffer formatting: paste buffer variable expansion for list-buffers output.
package tmux

import (
	"log/slog"
	"strconv"
	"strings"
)

// defaultBufferListFormat is the default format for list-buffers output.
const defaultBufferListFormat = "#{buffer_name}: #{buffer_size} bytes: \"#{buffer_sample}\""

// formatBufferLine formats a paste buffer for list-buffers output.
// Uses expandBufferFormat which supports nested #{...} and comparison operators.
func formatBufferLine(buf *PasteBuffer, customFormat string) string {
	format := strings.TrimSpace(customFormat)
	if format == "" {
		format = defaultBufferListFormat
	}
	return expandBufferFormat(format, buf)
}

// expandBufferFormat expands #{...} placeholders for buffer variables.
// Uses the same manual brace-matching approach as expandFormatNested to correctly handle
// nested #{...} inside comparison operators like #{==:#{buffer_name},foo}.
func expandBufferFormat(format string, buf *PasteBuffer) string {
	var out strings.Builder
	out.Grow(len(format))
	i := 0
	for i < len(format) {
		if i+1 < len(format) && format[i] == '#' && format[i+1] == '{' {
			inner, end := extractNestedBraces(format, i+2)
			if end < 0 {
				slog.Debug("[DEBUG-FORMAT] expandBufferFormat: unclosed brace in format",
					"snippet", format[i:])
				out.WriteString(format[i:])
				break
			}
			out.WriteString(resolveBufferFormatExpr(inner, buf))
			i = end + 1
		} else {
			out.WriteByte(format[i])
			i++
		}
	}
	return out.String()
}

// resolveBufferFormatExpr resolves a single format expression for buffer context.
// Handles comparison operators (==, !=) and plain buffer variable names.
func resolveBufferFormatExpr(expr string, buf *PasteBuffer) string {
	// Handle comparison operators: ==:lhs,rhs and !=:lhs,rhs
	var op string
	var rest string
	if strings.HasPrefix(expr, "==:") {
		op = "=="
		rest = expr[3:]
	} else if strings.HasPrefix(expr, "!=:") {
		op = "!="
		rest = expr[3:]
	} else {
		return lookupBufferVariable(expr, buf)
	}

	commaIdx := findTopLevelComma(rest)
	if commaIdx < 0 {
		slog.Debug("[DEBUG-FORMAT] malformed buffer comparison expr: missing comma", "op", op, "expr", expr)
		return ""
	}
	lhs := expandBufferFormat(rest[:commaIdx], buf)
	rhs := expandBufferFormat(rest[commaIdx+1:], buf)

	switch op {
	case "==":
		if lhs == rhs {
			return "1"
		}
		return "0"
	case "!=":
		if lhs != rhs {
			return "1"
		}
		return "0"
	default:
		return ""
	}
}

func lookupBufferVariable(name string, buf *PasteBuffer) string {
	if buf == nil {
		switch name {
		case "buffer_name":
			return ""
		case "buffer_size":
			return "0"
		case "buffer_sample":
			return ""
		default:
			return ""
		}
	}
	switch name {
	case "buffer_name":
		return buf.Name
	case "buffer_size":
		return strconv.Itoa(len(buf.Data))
	case "buffer_sample":
		sample := string(buf.Data)
		if len(sample) > 50 {
			sample = sample[:50]
		}
		// Replace newlines with spaces for single-line display.
		sample = strings.ReplaceAll(sample, "\n", " ")
		sample = strings.ReplaceAll(sample, "\r", "")
		return sample
	default:
		return ""
	}
}
