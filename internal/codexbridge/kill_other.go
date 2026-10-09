//go:build !windows

package codexbridge

import (
	"os/exec"
	"syscall"
)

func killProcessTree(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		// configureCommand starts Codex in its own process group, so descendants
		// spawned by the CLI are canceled together with their parent.
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			_ = cmd.Process.Kill()
		}
	}
}
