package terminal

import (
	"testing"
)

func TestStartSmoke(t *testing.T) {
	term, err := Start(Config{
		Columns: 120,
		Rows:    40,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer term.Close()
}

func TestNormalizePipeInput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "no carriage return",
			in:   "echo hello",
			out:  "echo hello",
		},
		{
			name: "single carriage return",
			in:   "cmd\r",
			out:  "cmd\r\n",
		},
		{
			name: "already crlf",
			in:   "cmd\r\n",
			out:  "cmd\r\n",
		},
		{
			name: "multiple carriage returns",
			in:   "a\rb\r",
			out:  "a\r\nb\r\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(normalizePipeInput([]byte(tc.in)))
			if got != tc.out {
				t.Fatalf("normalizePipeInput(%q) = %q, want %q", tc.in, got, tc.out)
			}
		})
	}
}

type captureWriteCloser struct {
	data []byte
}

func (c *captureWriteCloser) Write(p []byte) (int, error) {
	c.data = append(c.data, p...)
	return len(p), nil
}

func (c *captureWriteCloser) Close() error { return nil }

func TestWritePipeModeConvertsCRToCRLF(t *testing.T) {
	writer := &captureWriteCloser{}
	term := &Terminal{stdin: writer}

	if _, err := term.Write([]byte("cmd\r")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := string(writer.data); got != "cmd\r\n" {
		t.Fatalf("pipe input = %q, want %q", got, "cmd\\r\\n")
	}
}
