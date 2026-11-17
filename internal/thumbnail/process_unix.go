//go:build !windows

package thumbnail

import (
	"os/exec"
	"syscall"
)

func pauseProcess(cmd *exec.Cmd) error {
	return cmd.Process.Signal(syscall.SIGSTOP)
}

func resumeProcess(cmd *exec.Cmd) error {
	return cmd.Process.Signal(syscall.SIGCONT)
}
