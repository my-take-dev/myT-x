//go:build !windows

package terminal

import (
	"os"

	"github.com/creack/pty"
)

func resizePtmx(ptmx *os.File, cols, rows int) error {
	return pty.Setsize(ptmx, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
}
