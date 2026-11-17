package core

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNoWindow = 0x08000000

// ownProcessGroup lets a cancelled command take the processes it started
// with it (matlab.exe is a launcher), and keeps console windows from
// flashing up beside the app.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	cmd.Cancel = func() error {
		kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
		kill.Run()
		return cmd.Process.Kill()
	}
}
