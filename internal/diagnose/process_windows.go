// SPDX-License-Identifier: GPL-2.0-only

//go:build windows

package diagnose

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNewProcessGroup = 0x00000200

func setProcessGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup,
		HideWindow:    true,
	}
}

func killProcess(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	killer := exec.Command("taskkill", "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F")
	killer.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = killer.Run()
	_ = command.Process.Kill()
}
