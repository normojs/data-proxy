//go:build !windows

package dpagent

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

func startRemoteShellPTY(cmd *exec.Cmd, cols int, rows int) (*os.File, error) {
	return pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

func resizeRemoteShellPTY(tty *os.File, cols int, rows int) error {
	return pty.Setsize(tty, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}
