// SPDX-License-Identifier: GPL-2.0-only

//go:build windows

package relink

import (
	"os/exec"
	"syscall"
)

func hideWindow(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
