package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"fastlogin/internal/config"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// CommandRunner execs an arbitrary shell command under a PTY.
type CommandRunner struct{}

func (r *CommandRunner) Run(ctx context.Context, e config.Entry) error {
	if e.Command == "" {
		return fmt.Errorf("entry %q has empty command", e.Name)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", e.Command)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start command %q: %w", e.Command, err)
	}
	defer ptmx.Close()
	attachSize(ptmx)

	rawRestore := makeRawStdin()
	if rawRestore != nil {
		defer rawRestore()
	}

	go io.Copy(ptmx, os.Stdin)
	go io.Copy(os.Stdout, ptmx)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command %q exited: %w", e.Command, err)
	}
	return nil
}

// makeRawStdin puts the terminal in raw mode and returns a restore func, or nil
// if stdin is not a terminal.
func makeRawStdin() func() {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return nil
	}
	return func() { _ = term.Restore(fd, old) }
}
