// SPDX-License-Identifier: GPL-2.0-only

//go:build windows

package diagnose

import "os/exec"

func setProcessGroup(_ *exec.Cmd) {}

func killProcess(command *exec.Cmd) {
	if command.Process != nil {
		_ = command.Process.Kill()
	}
}
