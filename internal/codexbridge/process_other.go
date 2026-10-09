//go:build !windows

package codexbridge

import "os/exec"

func configureCommand(_ *exec.Cmd) {}
