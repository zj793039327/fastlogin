package runner

import (
	"os"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// attachSize copies the current terminal's size onto the PTY master.
func attachSize(ptmx *os.File) {
	fd := int(os.Stdin.Fd())
	if w, h, err := term.GetSize(fd); err == nil {
		_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(h), Cols: uint16(w)})
	}
}
