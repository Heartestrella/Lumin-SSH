//go:build windows

package codexbridge

import (
	"os/exec"
	"strconv"
)

func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	configureCommand(kill)
	_ = kill.Run()
}
