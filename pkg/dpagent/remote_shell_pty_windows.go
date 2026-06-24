//go:build windows

package dpagent

import (
	"errors"
	"os"
	"os/exec"
)

func startRemoteShellPTY(_ *exec.Cmd, _ int, _ int) (*os.File, error) {
	return nil, errors.New("PTY shell is not supported on Windows yet; use remote_shell_open without pty=true")
}

func resizeRemoteShellPTY(_ *os.File, _ int, _ int) error {
	return errors.New("PTY shell is not supported on Windows yet")
}
