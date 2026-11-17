//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

// ownProcessGroup lets a cancelled command take the processes it started
// with it: MATLAB's launcher is a script that starts MATLAB itself.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
