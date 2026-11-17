//go:build windows

package thumbnail

import "os/exec"

// Windows has no SIGSTOP/SIGCONT. The gate in Throttler.Wait keeps new
// generation from starting while paused; an already running ffmpeg command
// is short-lived and finishes on its own.
func pauseProcess(cmd *exec.Cmd) error  { return nil }
func resumeProcess(cmd *exec.Cmd) error { return nil }
