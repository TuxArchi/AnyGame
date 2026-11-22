// SPDX-License-Identifier: GPL-2.0-only

//go:build !windows

package diagnose

import (
	"os/exec"
	"syscall"
)

func setProcessGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcess(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	pid := command.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = command.Process.Kill()
}
